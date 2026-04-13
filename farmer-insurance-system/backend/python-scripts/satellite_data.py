# -----------------------------------------------------------------
# FINAL SCRIPT (V2 - WITH FRAUD CHECK): satellite_data.py
# -----------------------------------------------------------------
# This script now:
# 1. Performs "Before vs. After" NDVI analysis (as you described).
# 2. Performs Weather Cross-Verification (Fraud Check).
# 3. Takes claimID, polygon, damage_date, and reason as args.
# 4. Calls Sentinel Hub (NDVI) and Tomorrow.io (Weather).
# 5. Prints a final JSON report including a "verificationFlags" list.
# -----------------------------------------------------------------

import requests
import json
import numpy as np
from datetime import datetime, timedelta
import time
import hashlib
import sys
import argparse
import io
import rasterio
from rasterio.io import MemoryFile

# Configuration with your correct keys
config = {
    "sh_client_id": "3f2dc1c8-9e3b-4e88-bd2d-b40e2a38dfbe",
    "sh_client_secret": "Yg3Nsr3LS8GWhVUpEW2TKXVm0Ccl0QKf",
    "tomorrow_api_key": "VRe45t8MipOS1OhCGBg6ACwwUDu91b8k" 
}


def get_access_token():
    """Get access token from Sentinel Hub"""
    try:
        auth_url = "https://services.sentinel-hub.com/oauth/token"
        auth_data = {
            "grant_type": "client_credentials",
            "client_id": config["sh_client_id"],
            "client_secret": config["sh_client_secret"],
        }
        response = requests.post(auth_url, data=auth_data, timeout=10)
        if response.status_code == 200:
            return response.json()["access_token"]
        else:
            print(f"❌ SH Auth failed: {response.status_code}\n{response.text}", file=sys.stderr)
            return None
    except Exception as e:
        print(f"❌ Error getting SH access token: {e}", file=sys.stderr)
        return None

def get_polygon_centroid(polygon_json_string):
    """Calculates the centroid (average lat/lon) of the farm polygon."""
    try:
        polygon_data = json.loads(polygon_json_string)
        num_points = len(polygon_data)
        if num_points == 0:
            return None
        
        sum_lon = 0.0
        sum_lat = 0.0
        for point in polygon_data:
            sum_lon += point["longitude"]
            sum_lat += point["latitude"]
            
        return {"latitude": sum_lat / num_points, "longitude": sum_lon / num_points}
    except Exception as e:
        print(f"❌ Error parsing polygon for centroid: {e}", file=sys.stderr)
        return None

def verify_weather_event(reason, polygon_json_string, damage_date):
    """
    Calls Tomorrow.io to verify if weather data matches the farmer's claimed reason.
    Returns a list of "red flag" strings if fraud is suspected.
    """
    print(f"Verifying weather for: {reason} on {damage_date}", file=sys.stderr)
    flags = []
    
    # 1. Get the center of the farm for the weather API
    location = get_polygon_centroid(polygon_json_string)
    if location is None:
        flags.append("Could not parse farm location for weather verification.")
        return flags
        
    loc_str = f"{location['latitude']},{location['longitude']}"

    # 2. Define the exact time window for the damage date (full day)
    try:
        start_time_dt = datetime.strptime(damage_date, "%Y-%m-%d")
        end_time_dt = start_time_dt + timedelta(days=1) - timedelta(seconds=1)
        start_time_iso = start_time_dt.isoformat() + "Z"
        end_time_iso = end_time_dt.isoformat() + "Z"
    except Exception as e:
        print(f"❌ Error parsing damage_date: {e}", file=sys.stderr)
        flags.append(f"Invalid damage_date format: {damage_date}")
        return flags

    # 3. Define the API request payload
    url = "https://api.tomorrow.io/v4/timelines"
    payload = {
        "location": loc_str,
        "fields": ["precipitationTotal"], # We only care about rain for now
        "units": "metric",
        "timesteps": ["1d"], # Get one value for the whole day
        "startTime": start_time_iso,
        "endTime": end_time_iso,
        "apikey": config["tomorrow_api_key"]
    }
    
    try:
        # 4. Call the Tomorrow.io API
        response = requests.post(url, json=payload, timeout=20)
        
        if response.status_code != 200:
            print(f"❌ Weather API Error: {response.status_code}\n{response.text}", file=sys.stderr)
            flags.append(f"Weather API failed with status {response.status_code}.")
            return flags

        data = response.json()
        
        # 5. Parse the response
        intervals = data.get("data", {}).get("timelines", [{}])[0].get("intervals", [])
        if not intervals:
            raise Exception("No weather data intervals returned.")
            
        daily_values = intervals[0].get("values", {})
        precipitation_total = daily_values.get("precipitationTotal", 0.0)
        
        print(f"Weather Report: Total precipitation on {damage_date}: {precipitation_total} mm", file=sys.stderr)

        # 6. --- FRAUD LOGIC ---
        if reason == "Flood Damage" or reason == "Heavy Rainfall":
            if precipitation_total < 10.0: # Less than 10mm (1cm) of rain
                flags.append(f"Inconsistent weather: Farmer claimed '{reason}', but only {precipitation_total} mm of rain was recorded.")
        
        elif reason == "Hailstorm":
            if precipitation_total == 0.0: # Hail is precipitation. If 0, it's a red flag.
                flags.append(f"Inconsistent weather: Farmer claimed '{reason}', but 0.0 mm of precipitation was recorded.")
        

    except Exception as e:
        print(f"❌ Error during weather verification: {e}", file=sys.stderr)
        flags.append(f"Weather verification failed: {e}")

    return flags


def get_satellite_data(polygon_json_string, start_date_iso, end_date_iso):
    """
    Get satellite data (as a GeoTIFF image) using the Process API and a polygon
    for a specific time window.
    """
    access_token = get_access_token()
    if not access_token:
        return None

    # Parse polygon JSON from Go
    try:
        polygon_data = json.loads(polygon_json_string)
        coordinates = [[p["longitude"], p["latitude"]] for p in polygon_data]
        if coordinates[0] != coordinates[-1]:
            coordinates.append(coordinates[0])

        geometry = {"type": "Polygon", "coordinates": [coordinates]}

    except Exception as e:
        print(f"❌ Error parsing polygon JSON: {e}", file=sys.stderr)
        return None

    evalscript = """
    //VERSION=3
    function setup() {
      return {
        input: [{ bands: ["B04", "B08", "dataMask"] }],
        output: { id: "default", bands: 2, sampleType: "FLOAT32" }
      };
    }
    function evaluatePixel(sample) {
      let ndvi = (sample.B08 - sample.B04) / (sample.B08 + sample.B04);
      return [ndvi, sample.dataMask];
    }
    """

    request_payload = {
        "input": {
            "bounds": {
                "geometry": geometry,
                "properties": {"crs": "http://www.opengis.net/def/crs/OGC/1.3/CRS84"},
            },
            "data": [
                {
                    "type": "sentinel-2-l2a",
                    "dataFilter": {
                        "timeRange": {
                            "from": start_date_iso + "Z",
                            "to": end_date_iso + "Z",
                        },
                        "maxCloudCoverage": 20,
                        "mosaickingOrder": "mostRecent", # Get the most recent clear image in window
                    },
                }
            ],
        },
        "output": {
            "responses": [
                {
                    "identifier": "default",
                    "format": {"type": "image/tiff"},
                }
            ]
        },
        "evalscript": evalscript,
    }

    try:
        print(f"Connecting to Sentinel Hub Process API for period {start_date_iso} to {end_date_iso}...", file=sys.stderr)
        headers = {
            "Authorization": f"Bearer {access_token}",
            "Content-Type": "application/json",
            "Accept": "image/tiff",
        }
        url = "https://services.sentinel-hub.com/api/v1/process"
        response = requests.post(url, json=request_payload, headers=headers, timeout=120)

        if response.status_code == 200:
            print("✅ Successfully received satellite TIFF data!", file=sys.stderr)
            return response.content
        else:
            print(f"❌ API returned error: {response.status_code}", file=sys.stderr)
            print(f"❌ Error details: {response.text[:500]}", file=sys.stderr)
            return None

    except Exception as e:
        print(f"❌ Error fetching satellite data: {e}", file=sys.stderr)
        return None


def process_satellite_data(image_bytes):
    """
    Process the REAL satellite TIFF data.
    """
    try:
        print("Processing REAL satellite data...", file=sys.stderr)
        with MemoryFile(image_bytes) as memfile:
            with memfile.open() as dataset:
                ndvi_band = dataset.read(1).astype("float32")
                mask_band = dataset.read(2).astype("uint8")

        # Filter NDVI using mask == 1 (valid data)
        ndvi_values = ndvi_band[mask_band == 1]

        if ndvi_values.size == 0:
            print("❌ No valid data pixels found (all pixels masked, possibly 100% cloud).", file=sys.stderr)
            # Return a special result with 0 NDVI
            return {"vegetation_indices": {"ndvi_mean": 0.0}}

        ndvi_mean = float(np.mean(ndvi_values))
        
        print(f"✅ REAL satellite data processing complete. Mean NDVI: {ndvi_mean}", file=sys.stderr)
        return {
            "metadata": {
                "valid_pixel_count": int(ndvi_values.size),
            },
            "vegetation_indices": {
                "ndvi_mean": ndvi_mean,
            }
        }
    except Exception as e:
        print(f"❌ Error processing REAL satellite data: {e}", file=sys.stderr)
        return None


def calculate_crop_damage(baseline_data, impact_data, claim_id):
    """Calculate crop damage assessment from "Before vs. After" REAL Sentinel-2 data"""
    if baseline_data is None or impact_data is None:
        return None

    try:
        baseline_ndvi = baseline_data["vegetation_indices"]["ndvi_mean"]
        impact_ndvi = impact_data["vegetation_indices"]["ndvi_mean"]

        print(f" Baseline NDVI: {baseline_ndvi:.4f}, Impact NDVI: {impact_ndvi:.4f}", file=sys.stderr)

        # If baseline NDVI is very low (e.g., already barren), any drop is irrelevant.
        if baseline_ndvi < 0.1:
            print("Baseline NDVI is < 0.1, no damage calculated.", file=sys.stderr)
            total_damage = 0.0
        else:
            # Calculate percentage drop
            percent_drop = ((baseline_ndvi - impact_ndvi) / baseline_ndvi) * 100.0
            # Clamp damage between 0 and 100
            total_damage = max(0, min(100, percent_drop))

        timestamp = str(time.time())
        data_hash = hashlib.sha256(f"{claim_id}_{impact_ndvi}_{total_damage}_{timestamp}".encode()).hexdigest()

        return {
            "damagePercentage": int(total_damage),
            "ndviValue": float(impact_ndvi),
            "satelliteDataHash": data_hash,
            "analysisReport": {
                "baseline_ndvi_mean": baseline_ndvi,
                "impact_ndvi_mean": impact_ndvi
            },
        }

    except Exception as e:
        print(f"❌ Error in crop damage assessment: {e}", file=sys.stderr)
        return None


def main(claim_id, polygon_json, damage_date, reason):
    """
    Main analysis function called by Go.
    """
    print(f"INITIATING ANALYSIS (with Fraud Check) for ClaimID: {claim_id}", file=sys.stderr)

    # 1. Define time periods for "Before vs. After"
    try:
        damage_dt = datetime.strptime(damage_date, "%Y-%m-%d")
        
        # Impact period: from damage date to today
        impact_start_dt = damage_dt
        impact_end_dt = datetime.now()
        
        # Baseline period: 30 days *before* the damage
        baseline_start_dt = damage_dt - timedelta(days=30)
        baseline_end_dt = damage_dt - timedelta(days=1) # Up to the day before damage
        
    except Exception as e:
        print(f"❌ FAILED: Invalid damage_date format '{damage_date}'. Error: {e}", file=sys.stderr)
        return None

    # 2. Get Baseline Sentinel-2 data
    baseline_image_bytes = get_satellite_data(polygon_json, baseline_start_dt.isoformat(), baseline_end_dt.isoformat())
    if baseline_image_bytes is None:
        print(f"❌ FAILED: Could not retrieve Baseline Sentinel-2 data.", file=sys.stderr)
        return None
    baseline_data = process_satellite_data(baseline_image_bytes)

    # 3. Get Impact Sentinel-2 data
    impact_image_bytes = get_satellite_data(polygon_json, impact_start_dt.isoformat(), impact_end_dt.isoformat())
    if impact_image_bytes is None:
        print(f"❌ FAILED: Could not retrieve Impact Sentinel-2 data.", file=sys.stderr)
        return None
    impact_data = process_satellite_data(impact_image_bytes)

    # 4. Calculate final damage assessment
    final_assessment = calculate_crop_damage(baseline_data, impact_data, claim_id)
    if final_assessment is None:
        print(f"❌ FAILED: Could not calculate final damage.", file=sys.stderr)
        return None

    verification_flags = verify_weather_event(reason, polygon_json, damage_date)
    final_assessment["verificationFlags"] = verification_flags # Add flags to final report

    # 6. Print the final JSON to STDOUT 
    print(json.dumps(final_assessment, indent=2))
    print(f"✅ ANALYSIS COMPLETE (with Fraud Check) for ClaimID: {claim_id}", file=sys.stderr)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Process satellite data for a farm.")
    parser.add_argument("claim_id", type=str, help="The claim ID")
    parser.add_argument("polygon_json", type=str, help="The JSON string of the farm boundary polygon")
    parser.add_argument("damage_date", type=str, help="Date of damage (YYYY-MM-DD)")
    parser.add_argument("reason", type=str, help="Farmer's claimed reason for damage (e.g., 'Flood Damage')")

    args = parser.parse_args()

    main(args.claim_id, args.polygon_json, args.damage_date, args.reason)