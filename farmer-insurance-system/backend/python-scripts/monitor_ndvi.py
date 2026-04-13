# -----------------------------------------------------------------
# NEW SCRIPT: monitor_ndvi.py
# -----------------------------------------------------------------
# This script is for the Proactive Monitor. It:
# 1. Takes a FarmID (for logging) and a polygon.
# 2. Fetches a "baseline" snapshot (most recent, 60-30 days ago).
# 3. Fetches a "current" snapshot (most recent, 7 days ago).
# 4. Calculates the NDVI drop between them.
# 5. Prints the final JSON for the Go cron job.
# -----------------------------------------------------------------

import requests
import json
import numpy as np
from datetime import datetime, timedelta
import hashlib
import time
import sys
import argparse
import io
import rasterio
from rasterio.io import MemoryFile

# Configuration with your correct keys
config = {
    "sh_client_id": "3f2dc1c8-9e3b-4e88-bd2d-b40e2a38dfbe",
    "sh_client_secret": "Yg3Nsr3LS8GWhVUpEW2TKXVm0Ccl0QKf",
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
            print(f"Authentication failed: {response.status_code}\n{response.text}", file=sys.stderr)
            return None
    except Exception as e:
        print(f"Error getting access token: {e}", file=sys.stderr)
        return None


def get_satellite_data_for_period(polygon_json_string, start_date, end_date):
    """
    Get satellite data (as a GeoTIFF image) using the Process API
    for a specific time window, taking the most recent clear image.
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
        print(f"Error parsing polygon JSON: {e}", file=sys.stderr)
        return None

    # Evalscript that returns two bands: NDVI and dataMask
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
                            "from": start_date.isoformat() + "Z",
                            "to": end_date.isoformat() + "Z",
                        },
                        "maxCloudCoverage": 20,
                        "mosaickingOrder": "mostRecent",
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
        print(f"[Monitor] Connecting to Sentinel Hub for period {start_date.date()} to {end_date.date()}...", file=sys.stderr)

        headers = {
            "Authorization": f"Bearer {access_token}",
            "Content-Type": "application/json",
            "Accept": "image/tiff",
        }

        url = "https://services.sentinel-hub.com/api/v1/process"
        response = requests.post(url, json=request_payload, headers=headers, timeout=120)

        if response.status_code == 200:
            print("[Monitor] Successfully received satellite TIFF data!", file=sys.stderr)
            return response.content
        else:
            print(f"[Monitor] API returned error: {response.status_code}", file=sys.stderr)
            print(f"[Monitor] Error details: {response.text[:500]}", file=sys.stderr)
            return None

    except Exception as e:
        print(f"[Monitor] Error fetching satellite data: {e}", file=sys.stderr)
        return None


def process_satellite_data(image_bytes):
    """
    Process the REAL satellite TIFF data.
    Read the image bytes with rasterio, extract NDVI and mask,
    and calculate statistics.
    """
    try:
        print("[Monitor] Processing REAL satellite data...", file=sys.stderr)

        with MemoryFile(image_bytes) as memfile:
            with memfile.open() as dataset:
                ndvi_band = dataset.read(1).astype("float32")
                mask_band = dataset.read(2).astype("uint8")

        ndvi_values = ndvi_band[mask_band == 1]

        if ndvi_values.size == 0:
            print("[Monitor] No valid data pixels found (all pixels masked, possibly 100% cloud).", file=sys.stderr)
            return {"vegetation_indices": {"ndvi_mean": 0.0}}

        ndvi_mean = float(np.mean(ndvi_values))

        results = {
            "metadata": {"valid_pixel_count": int(ndvi_values.size)},
            "vegetation_indices": {"ndvi_mean": ndvi_mean},
        }
        print(f"[Monitor] REAL satellite data processing complete. Mean NDVI: {ndvi_mean}", file=sys.stderr)
        return results

    except Exception as e:
        print(f"[Monitor] Error processing REAL satellite data: {e}", file=sys.stderr)
        return None


def calculate_ndvi_drop(baseline_data, current_data, farm_id):
    """
    Calculate the percentage drop in NDVI.
    """
    if baseline_data is None or current_data is None:
        return None

    try:
        baseline_ndvi = baseline_data["vegetation_indices"]["ndvi_mean"]
        current_ndvi = current_data["vegetation_indices"]["ndvi_mean"]

        print(f"[Monitor] Farm {farm_id} - Baseline NDVI: {baseline_ndvi:.4f}, Current NDVI: {current_ndvi:.4f}", file=sys.stderr)

        if baseline_ndvi < 0.1:
            print(f"[Monitor] Farm {farm_id} - Baseline NDVI < 0.1, no damage calculated.", file=sys.stderr)
            total_damage = 0.0
        else:
            percent_drop = ((baseline_ndvi - current_ndvi) / baseline_ndvi) * 100.0
            total_damage = max(0, min(100, percent_drop))

        timestamp = str(time.time())
        data_hash = hashlib.sha256(f"{farm_id}_{current_ndvi}_{total_damage}_{timestamp}".encode()).hexdigest()

        return {
            "damagePercentage": int(total_damage),
            "ndviValue": float(current_ndvi),
            "satelliteDataHash": data_hash,
            "analysisReport": {
                "baseline_ndvi_mean": baseline_ndvi,
                "current_ndvi_mean": current_ndvi
            },
        }

    except Exception as e:
        print(f"[Monitor] Error in NDVI drop calculation: {e}", file=sys.stderr)
        return None


def main(farm_id, polygon_json):
    """
    Main analysis function called by Go cron job.
    """
    print(f"[Monitor] INITIATING PROACTIVE CHECK for FarmID: {farm_id}", file=sys.stderr)

    today = datetime.now()

    # Current period: last 7 days
    current_start_date = today - timedelta(days=7)
    current_end_date = today

    # Baseline period: 60–30 days ago
    baseline_start_date = today - timedelta(days=60)
    baseline_end_date = today - timedelta(days=30)

    # 1. Get Baseline Data
    baseline_image_bytes = get_satellite_data_for_period(polygon_json, baseline_start_date, baseline_end_date)
    if baseline_image_bytes is None:
        print(f"[Monitor] FAILED: Could not retrieve Baseline data for farm {farm_id}.", file=sys.stderr)
        return None
    baseline_data = process_satellite_data(baseline_image_bytes)
    if baseline_data is None:
        print(f"[Monitor] FAILED: Could not process Baseline data for farm {farm_id}.", file=sys.stderr)
        return None

    # 2. Get Current Data
    current_image_bytes = get_satellite_data_for_period(polygon_json, current_start_date, current_end_date)
    if current_image_bytes is None:
        print(f"[Monitor] FAILED: Could not retrieve Current data for farm {farm_id}.", file=sys.stderr)
        return None
    current_data = process_satellite_data(current_image_bytes)
    if current_data is None:
        print(f"[Monitor] FAILED: Could not process Current data for farm {farm_id}.", file=sys.stderr)
        return None

    # 3. Calculate final damage assessment
    final_assessment = calculate_ndvi_drop(baseline_data, current_data, farm_id)
    if final_assessment is None:
        print(f"[Monitor] FAILED: Could not calculate final NDVI drop for {farm_id}.", file=sys.stderr)
        return None

    # 4. Print the final JSON to STDOUT (Go reads this)
    print(json.dumps(final_assessment, indent=2))
    print(f"[Monitor] ANALYSIS COMPLETE for FarmID: {farm_id}", file=sys.stderr)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Proactive monitoring of a farm's NDVI.")
    parser.add_argument("farm_id", type=str, help="The Farm ID for logging")
    parser.add_argument("polygon_json", type=str, help="The JSON string of the farm boundary polygon")

    args = parser.parse_args()
    main(args.farm_id, args.polygon_json)
