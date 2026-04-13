# -----------------------------------------------------------------
# FINAL SCRIPT: satellite_data.py
# -----------------------------------------------------------------
# This script correctly:
# 1. Uses the Process API.
# 2. Uses your auth keys.
# 3. Uses the full polygon.
# 4. Requests an image (TIFF).
# 5. READS the image using rasterio to get REAL data.
# 6. Prints the final JSON for Go.
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
            print(f"❌ Authentication failed: {response.status_code}\n{response.text}", file=sys.stderr)
            return None
    except Exception as e:
        print(f"❌ Error getting access token: {e}", file=sys.stderr)
        return None


def get_satellite_data(polygon_json_string):
    """
    Get satellite data (as a GeoTIFF image) using the Process API and a polygon.
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

    end_date = datetime.now()
    start_date = end_date - timedelta(days=30)

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
        print("🌱 Connecting to Sentinel Hub Process API...", file=sys.stderr)

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
    Read the image bytes with rasterio, extract NDVI and mask,
    and calculate statistics.
    """
    try:
        print("🔍 Processing REAL satellite data...", file=sys.stderr)

        # Use rasterio MemoryFile to open bytes
        with MemoryFile(image_bytes) as memfile:
            with memfile.open() as dataset:
                # dataset.count should be 2 (ndvi, mask)
                ndvi_band = dataset.read(1).astype("float32")
                mask_band = dataset.read(2).astype("uint8")

        # Filter NDVI using mask == 1 (valid data)
        ndvi_values = ndvi_band[mask_band == 1]

        if ndvi_values.size == 0:
            print("❌ No valid data pixels found (all pixels masked, possibly 100% cloud).", file=sys.stderr)
            return None

        total_pixels = int(ndvi_values.size)

        # Stats
        ndvi_mean = float(np.mean(ndvi_values))
        ndvi_min = float(np.min(ndvi_values))
        ndvi_max = float(np.max(ndvi_values))
        ndvi_std = float(np.std(ndvi_values))

        # Health categories
        healthy_pixels = int(np.sum(ndvi_values > 0.5))
        stressed_pixels = int(np.sum((ndvi_values > 0.2) & (ndvi_values <= 0.5)))
        barren_pixels = int(np.sum(ndvi_values <= 0.2))

        healthy_percent = float(healthy_pixels / total_pixels) * 100.0
        stressed_percent = float(stressed_pixels / total_pixels) * 100.0
        barren_percent = float(barren_pixels / total_pixels) * 100.0

        results = {
            "metadata": {
                "satellite": "Sentinel-2 L2A",
                "processing_time": datetime.now().isoformat(),
                "valid_pixel_count": total_pixels,
            },
            "vegetation_indices": {
                "ndvi_mean": ndvi_mean,
                "ndvi_min": ndvi_min,
                "ndvi_max": ndvi_max,
                "ndvi_std": ndvi_std,
            },
            "vegetation_health": {
                "healthy": healthy_percent,
                "stressed": stressed_percent,
                "barren": barren_percent,
            },
        }

        print("✅ REAL satellite data processing complete", file=sys.stderr)
        return results

    except Exception as e:
        print(f"❌ Error processing REAL satellite data: {e}", file=sys.stderr)
        return None


def calculate_crop_damage(sentinel2_data, claim_id):
    """Calculate crop damage assessment from REAL Sentinel-2 data"""
    if sentinel2_data is None:
        return None

    try:
        ndvi_mean = sentinel2_data["vegetation_indices"]["ndvi_mean"]
        health = sentinel2_data["vegetation_health"]

        damage_from_stress = health["stressed"] * 0.5  # 50% damage weight
        damage_from_barren = health["barren"] * 0.9   # 90% damage weight

        total_damage = min(100, damage_from_stress + damage_from_barren)

        timestamp = str(time.time())
        data_hash = hashlib.sha256(f"{claim_id}_{ndvi_mean}_{total_damage}_{timestamp}".encode()).hexdigest()

        return {
            "damagePercentage": int(total_damage),
            "ndviValue": float(ndvi_mean),
            "satelliteDataHash": data_hash,
            "analysisReport": sentinel2_data,
        }

    except Exception as e:
        print(f"❌ Error in crop damage assessment: {e}", file=sys.stderr)
        return None


def main(claim_id, polygon_json):
    """
    Main analysis function called by Go.
    """
    print(f"🚀 INITIATING ANALYSIS for ClaimID: {claim_id}", file=sys.stderr)

    # 1. Get Sentinel-2 image data
    sentinel2_image_bytes = get_satellite_data(polygon_json)

    if sentinel2_image_bytes is None:
        print(f"❌ FAILED: Could not retrieve Sentinel-2 data.", file=sys.stderr)
        return None

    # 2. Process the raw image bytes to get statistics
    sentinel2_data = process_satellite_data(sentinel2_image_bytes)

    if sentinel2_data is None:
        print(f"❌ FAILED: Could not process Sentinel-2 data.", file=sys.stderr)
        return None

    # 3. Calculate final damage assessment
    final_assessment = calculate_crop_damage(sentinel2_data, claim_id)
    if final_assessment is None:
        print(f"❌ FAILED: Could not calculate final damage.", file=sys.stderr)
        return None

    # 4. Print the final JSON to STDOUT (Go reads this)
    print(json.dumps(final_assessment, indent=2))
    print(f"✅ ANALYSIS COMPLETE for ClaimID: {claim_id}", file=sys.stderr)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Process satellite data for a farm.")
    parser.add_argument("claim_id", type=str, help="The claim ID")
    parser.add_argument("polygon_json", type=str, help="The JSON string of the farm boundary polygon")

    args = parser.parse_args()

    main(args.claim_id, args.polygon_json)
