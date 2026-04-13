import time
import random
import argparse # For reading command-line arguments (like --claimID)
import requests # For calling our Go backend API
import json
import sys

# This is the (simulated) "Satellite Analysis" script.

def simulate_satellite_analysis():
    """
    Simulates a time-consuming satellite data pull and analysis.
    Returns (damage_percentage, ndvi_value, satellite_hash)
    """
    print("Python: Starting simulated satellite data analysis...")
    
    # Simulate a 2 to 5 second delay
    delay = random.randint(2, 5)
    print(f"Python: Simulating {delay} second data pull...")
    time.sleep(delay)
    
    # Simulate damage based on NDVI (Normalized Difference Vegetation Index)
    # Healthy plant: 0.6 to 0.9. Stressed/Damaged: 0.1 to 0.3
    ndvi_value = round(random.uniform(0.1, 0.9), 2)
    
    damage_percentage = 0
    if ndvi_value < 0.2:
        damage_percentage = random.randint(80, 100) # Total loss
    elif ndvi_value < 0.4:
        damage_percentage = random.randint(40, 79) # High damage
    else:
        damage_percentage = random.randint(0, 19) # Low/No damage
        
    # A mock hash of the data we "pulled"
    satellite_hash = f"fake_satellite_data_hash_{random.randint(10000, 99999)}"
    
    print(f"Python: Analysis complete. NDVI: {ndvi_value}, Damage: {damage_percentage}%")
    return damage_percentage, ndvi_value, satellite_hash

def call_backend_api(claim_id, damage_percentage, ndvi_value, satellite_hash):
    """
    Calls back to the Go backend's /update-claim-satellite endpoint
    to submit the analysis results.
    """
    # This is the URL of your Go backend
    api_url = "http://localhost:3000/updateClaimWithSatelliteData"
    
    payload = {
        "claimID": claim_id,
        "satelliteDataHash": satellite_hash,
        "damagePercentage": damage_percentage,
        "ndviValue": ndvi_value
    }
    
    headers = {'Content-Type': 'application/json'}
    
    print(f"Python: Calling backend API at {api_url} with payload: {json.dumps(payload)}")
    
    try:
        response = requests.post(api_url, data=json.dumps(payload), headers=headers, timeout=10)
        
        if response.status_code == 200:
            print("Python: Successfully updated claim on the backend.")
            print(f"Backend response: {response.json()}")
        else:
            print(f"Python: Error calling backend. Status: {response.status_code}, Response: {response.text}")
            
    except requests.exceptions.RequestException as e:
        print(f"Python: Failed to connect to backend API: {e}")
        sys.exit(1) # Exit with an error code

def main():
    # 1. Parse the --claimID argument sent from the Go backend
    parser = argparse.ArgumentParser(description="Farmer Insurance Satellite Analysis Script")
    parser.add_argument("--claimID", type=str, required=True, help="The Claim ID to analyze")
    args = parser.parse_args()
    
    claim_id = args.claimID
    print(f"Python: Received job for Claim ID: {claim_id}")
    
    # 2. Run the simulated analysis
    damage_percentage, ndvi_value, satellite_hash = simulate_satellite_analysis()
    
    # 3. Call back to the Go backend with the results
    call_backend_api(claim_id, damage_percentage, ndvi_value, satellite_hash)

if __name__ == "__main__":
    main()