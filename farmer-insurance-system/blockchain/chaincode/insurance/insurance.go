package main

import (
	"encoding/json"
	"fmt"
	"bytes"
	"log"
	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

// SmartContract provides functions for managing farmer insurance
type SmartContract struct {
	contractapi.Contract
}

// Asset Types to differentiate assets in the ledger
const (
	farmerAssetType = "farmer"
	farmAssetType   = "farm"
	claimAssetType  = "claim"
)

// Farmer describes a farmer registered on the platform
type Farmer struct {
	DocType  string `json:"docType"` // "farmer"
	FarmerID string `json:"farmerID"`// Primary Key
	Name     string `json:"name"`
	FarmIDs  []string `json:"farmIDs"` // List of farms they own
}

// Farm describes a piece of land insured
type Farm struct {
	DocType        string `json:"docType"` // "farm"
	FarmID         string `json:"farmID"`  // Primary Key
	OwnerFarmerID  string `json:"ownerFarmerID"`
	Location       string `json:"location"`
	CropType       string `json:"cropType"`
	LandRecordHash string `json:"landRecordHash"` // Hash of the 7-12 document
	ActiveClaimID  string `json:"activeClaimID"`  // A farm can only have one active claim
	Status         string `json:"status"`
}

// Claim represents an insurance claim
type Claim struct {
	DocType            string  `json:"docType"` 
	ClaimID            string  `json:"claimID"` 
	FarmID             string  `json:"farmID"`
	FarmerID           string  `json:"farmerID"`
	SubmitTimestamp    string  `json:"submitTimestamp"` 
	Reason             string  `json:"reason"`
	Status             string  `json:"status"` 

	Description        string   `json:"description"` 
	PolicyID           string   `json:"policyID"`    
	PolicyName         string   `json:"policyName"`

	DamageDate         string  `json:"damageDate"` //"2025-10-31"
	EvidenceHashes   []string  `json:"evidenceHashes"`

	// Fields to be populated after satellite analysis
	SatelliteDataHash  string  `json:"satelliteDataHash"` // Hash from Python script
	AnalysisTimestamp  string  `json:"analysisTimestamp"` // Timestamp when analysis was done
	DamagePercentage   int     `json:"damagePercentage"`  // Calculated by analysis
	NDVIValue          float64 `json:"ndviValue"`         // Example satellite data metric

	VerificationFlags []string `json:"verificationFlags"`

	// Fields related to approval/rejection
	PayoutAmount       int     `json:"payoutAmount"`     // Set upon approval
	DecisionTimestamp  string  `json:"decisionTimestamp"` // Timestamp of approval/rejection
	DecisionNotes      string  `json:"decisionNotes"`    // Reason for rejection or manual approval notes
	AssignedAuditor    string  `json:"assignedAuditor"`  // For manual review ("FlaggedForReview")
}

// --- Farmer Functions ---

func (s *SmartContract) RegisterFarmer(ctx contractapi.TransactionContextInterface, farmerID string, name string) error {
	exists, err := s.AssetExists(ctx, farmerID)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("the farmer %s already exists", farmerID)
	}

	farmer := Farmer{
		DocType:  farmerAssetType,
		FarmerID: farmerID,
		Name:     name,
		FarmIDs:  make([]string, 0), // Initialize empty list
	}
	farmerJSON, err := json.Marshal(farmer)
	if err != nil {
		return err
	}

	return ctx.GetStub().PutState(farmerID, farmerJSON)
}

// AddFarm registers a new farm and links it to an existing farmer.
func (s *SmartContract) AddFarm(ctx contractapi.TransactionContextInterface, farmID string, farmerID string, location string, cropType string, landRecordHash string) error {
	// 1. Check if farm already exists
	farmExists, err := s.AssetExists(ctx, farmID)
	if err != nil {
		return err
	}
	if farmExists {
		return fmt.Errorf("the farm %s already exists", farmID)
	}

	// 2. Get the farmer and check if they exist
	farmer, err := s.GetFarmer(ctx, farmerID)
	if err != nil {
		return err // GetFarmer will return a "not found" error
	}

	// 3. Create the new farm asset
	farm := Farm{
		DocType:        farmAssetType,
		FarmID:         farmID,
		OwnerFarmerID:  farmerID,
		Location:       location,
		CropType:       cropType,
		LandRecordHash: landRecordHash,
		ActiveClaimID:  "", // No active claim initially
		Status:         "PendingVerification",
	}
	farmJSON, err := json.Marshal(farm)
	if err != nil {
		return err
	}

	// 4. Update the farmer's list of farms
	farmer.FarmIDs = append(farmer.FarmIDs, farmID)
	farmerJSON, err := json.Marshal(farmer)
	if err != nil {
		return err
	}
	
	// 5. Write both assets to the ledger
	if err = ctx.GetStub().PutState(farmID, farmJSON); err != nil {
		return err
	}
	return ctx.GetStub().PutState(farmerID, farmerJSON)
}

// --- Claim Functions ---

// SubmitClaim creates a new insurance claim for a farm.
// Farmer provides only the reason; satellite data is processed later.
func (s *SmartContract) SubmitClaim(ctx contractapi.TransactionContextInterface, claimID string, farmID string, reason string, description string, policyID string, policyName string, damageDate string, evidenceHashesJSON string) error {
	// 1. Check if claim already exists
	claimExists, err := s.AssetExists(ctx, claimID)
	if err != nil {
		return err
	}
	if claimExists {
		return fmt.Errorf("the claim %s already exists", claimID)
	}

	// 2. Get the farm and check if it exists and is verified
	farm, err := s.GetFarm(ctx, farmID)
	if err != nil {
		return err 
	}
	if farm.Status != "Verified" {
		return fmt.Errorf("farm %s is not verified, cannot submit claim (current status: %s)", farmID, farm.Status)
	}

	// 3. Check if the farm already has an active claim
	if farm.ActiveClaimID != "" {
		return fmt.Errorf("farm %s already has an active claim: %s", farmID, farm.ActiveClaimID)
	}

	// 4. Get transaction timestamp
	txTimestamp, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return fmt.Errorf("failed to get transaction timestamp: %w", err)
	}
	submitTimeStr := txTimestamp.String() // Convert timestamp to string

	var evidenceHashes []string
	if err := json.Unmarshal([]byte(evidenceHashesJSON), &evidenceHashes); err != nil {
		return fmt.Errorf("failed to parse evidenceHashes JSON: %w", err)
	}

	// 5. Create the new claim with initial status
	claim := Claim{
		DocType:            claimAssetType,
		ClaimID:            claimID,
		FarmID:             farmID,
		FarmerID:           farm.OwnerFarmerID,
		SubmitTimestamp:    submitTimeStr, 
		Reason:             reason,
		Status:             "ProcessingSatelliteData", 

		Description:       description,
		PolicyID:          policyID,
		PolicyName:        policyName,

		DamageDate:         damageDate,
		EvidenceHashes:     evidenceHashes,
		// Initialize satellite/analysis fields
		SatelliteDataHash:  "",
		AnalysisTimestamp:  "",
		DamagePercentage:   0,
		NDVIValue:          0.0,

		// Initialize decision fields
		PayoutAmount:       0,
		DecisionTimestamp:  "",
		DecisionNotes:      "",
		AssignedAuditor:    "",
	}
	claimJSON, err := json.Marshal(claim)
	if err != nil {
		return err
	}

	// 6. Update the farm to link to this new active claim
	farm.ActiveClaimID = claimID
	farmJSON, err := json.Marshal(farm)
	if err != nil {
		return err
	}

	// 7. Write both assets to the ledger
	if err = ctx.GetStub().PutState(claimID, claimJSON); err != nil {
		return err
	}
	return ctx.GetStub().PutState(farmID, farmJSON)
}

// ApproveClaim is called by the InsuranceOrg to approve a claim and issue a payout.
// This function would be protected by an Access Control List in a real system.
func (s *SmartContract) ApproveClaim(ctx contractapi.TransactionContextInterface, claimID string, payoutAmount int) error {
	claim, err := s.GetClaim(ctx, claimID)
	if err != nil {
		return err
	}
	
	if claim.Status != "FlaggedForReview" {
		return fmt.Errorf("claim %s is not in 'Flagged for review' state, cannot approve", claimID)
	}

	farm, err := s.GetFarm(ctx, claim.FarmID)
	if err != nil {
		return err
	}
	
	// Update claim
	claim.Status = "Approved"
	claim.PayoutAmount = payoutAmount
	claimJSON, err := json.Marshal(claim)
	if err != nil {
		return err
	}

	// Update farm (claim is no longer active)
	farm.ActiveClaimID = ""
	farmJSON, err := json.Marshal(farm)
	if err != nil {
		return err
	}

	// Write both assets
	if err = ctx.GetStub().PutState(claimID, claimJSON); err != nil {
		return err
	}
	return ctx.GetStub().PutState(farm.FarmID, farmJSON)
}

// RejectClaim is called by the InsuranceOrg (or Auditor) to reject a claim.
func (s *SmartContract) RejectClaim(ctx contractapi.TransactionContextInterface, claimID string, reasonForRejection string) error {
	claim, err := s.GetClaim(ctx, claimID)
	if err != nil {
		return err
	}

	if claim.Status != "FlaggedForReview" {
		return fmt.Errorf("claim %s is not in 'Flagged for review' state, cannot reject", claimID)
	}

	farm, err := s.GetFarm(ctx, claim.FarmID)
	if err != nil {
		return err
	}

	// Update claim
	claim.Status = "Rejected"
	claim.Reason = reasonForRejection // Overwrite farmer's reason with rejection reason
	claimJSON, err := json.Marshal(claim)
	if err != nil {
		return err
	}

	// Update farm (claim is no longer active)
	farm.ActiveClaimID = ""
	farmJSON, err := json.Marshal(farm)
	if err != nil {
		return err
	}

	// Write both assets
	if err = ctx.GetStub().PutState(claimID, claimJSON); err != nil {
		return err
	}
	return ctx.GetStub().PutState(farm.FarmID, farmJSON)
}

// MarkAsPaid is called by the backend after a payout is processed.
// It sets the claim status to "Paid" and clears the farm's ActiveClaimID.
func (s *SmartContract) MarkAsPaid(ctx contractapi.TransactionContextInterface, claimID string, payoutAmount int) error {
	claim, err := s.GetClaim(ctx, claimID)
	if err != nil {
		return err
	}

	if claim.Status != "ApprovedAuto" && claim.Status != "Approved" {
		return fmt.Errorf("claim %s is not in an 'Approved' or 'ApprovedAuto' state, cannot mark as paid (current status: %s)", claimID, claim.Status)
	}

	farm, err := s.GetFarm(ctx, claim.FarmID)
	if err != nil {
		return fmt.Errorf("failed to get farm %s to clear active claim: %w", claim.FarmID, err)
	}
	
	txTimestamp, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return fmt.Errorf("failed to get transaction timestamp: %w", err)
	}
	decisionTimeStr := txTimestamp.String()

	farm.ActiveClaimID = "" 

	claim.Status = "Paid"
	claim.PayoutAmount = payoutAmount
	claim.DecisionTimestamp = decisionTimeStr
	claim.DecisionNotes = "Payment processed successfully."

	farmJSON, err := json.Marshal(farm)
	if err != nil {
		return fmt.Errorf("failed to marshal farm %s while marking claim as paid: %w", farm.FarmID, err)
	}
	claimJSON, err := json.Marshal(claim)
	if err != nil {
		return fmt.Errorf("failed to marshal claim %s while marking as paid: %w", claimID, err)
	}

	if err = ctx.GetStub().PutState(farm.FarmID, farmJSON); err != nil {
		return fmt.Errorf("failed to update farm %s state while marking claim as paid: %w", farm.FarmID, err)
	}
	return ctx.GetStub().PutState(claimID, claimJSON)
}

// VerifyFarm is called by the GovernmentOrg (or authorized entity)
// to mark a farm as verified after checking documents.
// This function would typically be protected by Access Control Lists (ACLs).
func (s *SmartContract) VerifyFarm(ctx contractapi.TransactionContextInterface, farmID string) error {
    // 1. Get the farm
    farm, err := s.GetFarm(ctx, farmID)
    if err != nil {
        return err // Returns error if farm not found
    }

    if farm.Status != "PendingVerification" {
        return fmt.Errorf("farm %s is not in 'PendingVerification' state, cannot verify (current status: %s)", farmID, farm.Status)
    }

    farm.Status = "Verified"

    // 4. Marshal back to JSON
    farmJSON, err := json.Marshal(farm)
    if err != nil {
        return fmt.Errorf("failed to marshal farm: %w", err)
    }

    // 5. Put the updated state back to the ledger
    return ctx.GetStub().PutState(farmID, farmJSON)
}

// UpdateClaimWithSatelliteData is called by the backend after Python analysis.
// It adds satellite results and determines the next status.
func (s *SmartContract) UpdateClaimWithSatelliteData(ctx contractapi.TransactionContextInterface, claimID string, satelliteDataHash string, damagePercentage int, ndviValue float64, verificationFlagsJSON string,) error {
    // 1. Get the claim
    claim, err := s.GetClaim(ctx, claimID)
    if err != nil {
        return err 
    }

    // 2. Check current status
    if claim.Status != "ProcessingSatelliteData" {
        return fmt.Errorf("claim %s is not in 'ProcessingSatelliteData' state, cannot update (current status: %s)", claimID, claim.Status)
    }

    // 3. Get transaction timestamp for analysis time
    txTimestamp, err := ctx.GetStub().GetTxTimestamp()
    if err != nil {
        return fmt.Errorf("failed to get transaction timestamp: %w", err)
    }
    analysisTimeStr := txTimestamp.String()

    // 4. Update claim fields with satellite data
    claim.SatelliteDataHash = satelliteDataHash
    claim.AnalysisTimestamp = analysisTimeStr
    claim.DamagePercentage = damagePercentage
    claim.NDVIValue = ndviValue

	var verificationFlags []string
	if err := json.Unmarshal([]byte(verificationFlagsJSON), &verificationFlags); err != nil {
		// If flags are malformed, treat it as a red flag
		verificationFlags = []string{fmt.Sprintf("Failed to parse verification flags: %v", err)}
	}
	claim.VerificationFlags = verificationFlags

    // 5. Determine next status based on damage percentage (Example Logic)
    if damagePercentage < 30 {
		// Low/No damage, reject 
		claim.Status = "Rejected"
		claim.DecisionNotes = "Satellite analysis indicates damage percentage is below threshold."
		claim.DecisionTimestamp = analysisTimeStr

		// We must also clear the ActiveClaimID on the farm ATOMICALLY.
		farm, err := s.GetFarm(ctx, claim.FarmID)
		if err != nil {
			return fmt.Errorf("failed to get farm %s to clear active claim: %w", claim.FarmID, err)
		}
		
		farm.ActiveClaimID = ""
		farmJSON, err := json.Marshal(farm)
		if err != nil {
			return fmt.Errorf("failed to marshal farm %s while rejecting claim: %w", farm.FarmID, err)
		}
		
		err = ctx.GetStub().PutState(farm.FarmID, farmJSON)
		if err != nil {
			return fmt.Errorf("failed to update farm %s state while rejecting claim: %w", farm.FarmID, err)
		}

	} else if len(verificationFlags) > 0 {
		// FRAUD DETECTED: Damage is >= 30% but flags were found. Downgrade to manual review, even if damage is high. 
		claim.Status = "FlaggedForReview"
		claim.DecisionNotes = "Claim flagged for manual review due to inconsistent data: " + verificationFlags[0] // Show first flag

	} else if damagePercentage >= 60 {
		// NO FRAUD & HIGH DAMAGE: Auto-approve
		claim.Status = "ApprovedAuto"
		claim.DecisionNotes = "Claim auto-approved based on high satellite damage and successful verification."
		claim.DecisionTimestamp = analysisTimeStr

	} else {
		// NO FRAUD & MEDIUM DAMAGE (30-59%): Normal manual review
		claim.Status = "FlaggedForReview"
		claim.DecisionNotes = "Satellite analysis damage is between 30-59%, requires manual review."
	}

    claimJSON, err := json.Marshal(claim)
    if err != nil {
        return fmt.Errorf("failed to marshal updated claim: %w", err)
    }

    // 7. Put the updated state back to the ledger. This saves the claim and, if rejected, the farm update.
    return ctx.GetStub().PutState(claimID, claimJSON)
}

// UpdateClaimStatusFailed is a system-only function to mark a claim as failed during automated processing.
func (s *SmartContract) UpdateClaimStatusFailed(ctx contractapi.TransactionContextInterface, claimID string, notes string) error {
	claim, err := s.GetClaim(ctx, claimID)
	if err != nil {
		return err
	}

	if claim.Status != "ProcessingSatelliteData" {
		return fmt.Errorf("claim %s is not in 'ProcessingSatelliteData' state, cannot mark as failed", claimID)
	}

	farm, err := s.GetFarm(ctx, claim.FarmID)
	if err != nil {
		return err
	}

	claim.Status = "FailedAnalysis"
	claim.DecisionNotes = notes
	txTimestamp, _ := ctx.GetStub().GetTxTimestamp()
	claim.DecisionTimestamp = txTimestamp.String()

	farm.ActiveClaimID = ""

	claimJSON, err := json.Marshal(claim)
	if err != nil { return err }
	
	farmJSON, err := json.Marshal(farm)
	if err != nil { return err }

	if err = ctx.GetStub().PutState(claimID, claimJSON); err != nil {
		return err
	}
	return ctx.GetStub().PutState(farm.FarmID, farmJSON)
}

// --- Helper & Query Functions ---

// ReadAsset returns any asset (Farmer, Farm, Claim) from the ledger given its ID.
func (s *SmartContract) ReadAsset(ctx contractapi.TransactionContextInterface, assetID string) ([]byte, error) {
	assetJSON, err := ctx.GetStub().GetState(assetID)
	if err != nil {
		return nil, fmt.Errorf("failed to read from world state: %v", err)
	}
	if assetJSON == nil {
		return nil, fmt.Errorf("the asset %s does not exist", assetID)
	}
	return assetJSON, nil
}

// GetFarmer is a helper to read and unmarshal a Farmer
func (s *SmartContract) GetFarmer(ctx contractapi.TransactionContextInterface, farmerID string) (*Farmer, error) {
	assetJSON, err := s.ReadAsset(ctx, farmerID)
	if err != nil {
		return nil, err
	}
	var farmer Farmer
	if err = json.Unmarshal(assetJSON, &farmer); err != nil {
		return nil, err
	}
	if farmer.DocType != farmerAssetType {
		return nil, fmt.Errorf("asset %s is not of type 'farmer'", farmerID)
	}
	return &farmer, nil
}

// GetFarm is a helper to read and unmarshal a Farm
func (s *SmartContract) GetFarm(ctx contractapi.TransactionContextInterface, farmID string) (*Farm, error) {
	assetJSON, err := s.ReadAsset(ctx, farmID)
	if err != nil {
		return nil, err
	}
	var farm Farm
	if err = json.Unmarshal(assetJSON, &farm); err != nil {
		return nil, err
	}
	if farm.DocType != farmAssetType {
		return nil, fmt.Errorf("asset %s is not of type 'farm'", farmID)
	}
	return &farm, nil
}

// GetClaim is a helper to read and unmarshal a Claim
func (s *SmartContract) GetClaim(ctx contractapi.TransactionContextInterface, claimID string) (*Claim, error) {
	assetJSON, err := s.ReadAsset(ctx, claimID)
	if err != nil {
		return nil, err
	}
	var claim Claim
	if err = json.Unmarshal(assetJSON, &claim); err != nil {
		return nil, err
	}
	if claim.DocType != claimAssetType {
		return nil, fmt.Errorf("asset %s is not of type 'claim'", claimID)
	}
	return &claim, nil
}

// AssetExists returns true when asset with given ID exists in world state
func (s *SmartContract) AssetExists(ctx contractapi.TransactionContextInterface, id string) (bool, error) {
	assetJSON, err := ctx.GetStub().GetState(id)
	if err != nil {
		return false, fmt.Errorf("failed to read from world state: %v", err)
	}
	return assetJSON != nil, nil
}

// --- Helper Function for Rich Queries ---

// constructQueryResponseFromIterator iterates through a state query iterator
// and constructs a JSON array of the results. This is essential for
// returning lists of farms or claims.
func constructQueryResponseFromIterator(resultsIterator shim.StateQueryIteratorInterface) ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteString("[")

	bArrayMemberAlreadyWritten := false
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}
		// Add a comma before array members, suppress it for the first array member
		if bArrayMemberAlreadyWritten {
			buffer.WriteString(",")
		}
		// Record is a JSON object, so we write as-is
		buffer.WriteString(string(queryResponse.Value))
		bArrayMemberAlreadyWritten = true
	}
	buffer.WriteString("]")

	return buffer.Bytes(), nil
}

// --- New Rich Query Functions ---

// GetAllFarmsByOwner finds all farms associated with a specific farmerID. This function now returns a slice of Farm structs, which the contract API will correctly marshal to a JSON array.
func (s *SmartContract) GetAllFarmsByOwner(ctx contractapi.TransactionContextInterface, farmerID string) ([]Farm, error) {
	 // The query string is a CouchDB JSON query
	queryString := fmt.Sprintf(`{"selector":{"docType":"%s","ownerFarmerID":"%s"}}`, farmAssetType, farmerID)

	resultsIterator, err := ctx.GetStub().GetQueryResult(queryString)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer resultsIterator.Close()

    // --- NEW LOGIC ---
    // Create an empty slice to hold the results
	var farms []Farm

	for resultsIterator.HasNext() {
	queryResponse, err := resultsIterator.Next()
	if err != nil {
 		return nil, err
 	}

    var farm Farm
    // Unmarshal the JSON from the iterator into a Farm struct
    if err := json.Unmarshal(queryResponse.Value, &farm); err != nil {
        return nil, fmt.Errorf("failed to unmarshal farm: %w", err)
    }

	farms = append(farms, farm)
    }

    // If the slice is nil (which happens if no farms are found),
    // we must return an *empty, non-nil slice* to ensure
    // the JSON marshals to `[]` and not `null`.
    if farms == nil {
        farms = make([]Farm, 0)
    }

	return farms, nil
}

// GetAllClaimsByFarmer finds all claims (across all their farms) associated with a specific farmerID.
func (s *SmartContract) GetAllClaimsByFarmer(ctx contractapi.TransactionContextInterface, farmerID string) ([]byte, error) {
	queryString := fmt.Sprintf(`{"selector":{"docType":"%s","farmerID":"%s"}}`, claimAssetType, farmerID)

	resultsIterator, err := ctx.GetStub().GetQueryResult(queryString)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer resultsIterator.Close()

	return constructQueryResponseFromIterator(resultsIterator)
}

// GetAllClaimsByFarm finds all claims (past and present) for a specific farm.
func (s *SmartContract) GetAllClaimsByFarm(ctx contractapi.TransactionContextInterface, farmID string) ([]byte, error) {
	queryString := fmt.Sprintf(`{"selector":{"docType":"%s","farmID":"%s"}}`, claimAssetType, farmID)

	resultsIterator, err := ctx.GetStub().GetQueryResult(queryString)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer resultsIterator.Close()

	return constructQueryResponseFromIterator(resultsIterator)
}

// GetClaimsByStatus finds all claims with a specific status.
// This function returns a Go slice, and Fabric handles marshaling.
func (s *SmartContract) GetClaimsByStatus(ctx contractapi.TransactionContextInterface, status string) ([]Claim, error) {
	queryString := fmt.Sprintf(`{"selector":{"docType":"%s","status":"%s"}}`, claimAssetType, status)

	resultsIterator, err := ctx.GetStub().GetQueryResult(queryString)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer resultsIterator.Close()

	var claims []Claim // 1. Create a slice of Claims
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}

		var claim Claim // 2. Unmarshal into a Claim struct
		if err := json.Unmarshal(queryResponse.Value, &claim); err != nil {
			return nil, fmt.Errorf("failed to unmarshal claim: %w", err)
		}
		claims = append(claims, claim) // 3. Append to the slice
	}

	if claims == nil {
		claims = make([]Claim, 0)
	}

	return claims, nil 
}

// func (s *SmartContract) GetClaimsByStatus(ctx contractapi.TransactionContextInterface, status string) ([]byte, error) {
// 	queryString := fmt.Sprintf(`{"selector":{"docType":"%s","status":"%s"}}`, claimAssetType, status)

// 	resultsIterator, err := ctx.GetStub().GetQueryResult(queryString)
// 	if err != nil {
// 		return nil, fmt.Errorf("query failed: %w", err)
// 	}
// 	defer resultsIterator.Close()

// 	return constructQueryResponseFromIterator(resultsIterator)
// }

// GetAllFarmsByStatus finds all farms with a specific status.
// This is a "rich query" that requires CouchDB.
func (s *SmartContract) GetFarmsByStatus(ctx contractapi.TransactionContextInterface, status string) ([]Farm, error) {
	queryString := fmt.Sprintf(`{"selector":{"docType":"%s","status":"%s"}}`, farmAssetType, status)

	resultsIterator, err := ctx.GetStub().GetQueryResult(queryString)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer resultsIterator.Close()

	var farms []Farm
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}

		var farm Farm
		if err := json.Unmarshal(queryResponse.Value, &farm); err != nil {
			return nil, fmt.Errorf("failed to unmarshal farm: %w", err)
		}
		farms = append(farms, farm)
	}

	if farms == nil {
		farms = make([]Farm, 0)
	}

	return farms, nil
}

// SchemeSummary represents the data structure for the new dashboard
type SchemeSummary struct {
	PolicyID     string `json:"policyID"`
	PolicyName   string `json:"policyName"`
	PendingCount int    `json:"pendingCount"`
}

// GetClaimSummaryByScheme queries all "FlaggedForReview" claims and groups them by policy.
func (s *SmartContract) GetClaimSummaryByScheme(ctx contractapi.TransactionContextInterface) ([]SchemeSummary, error) {
	// 1. We only care about claims that need manual review
	queryString := fmt.Sprintf(`{"selector":{"docType":"%s","status":"%s"}}`, claimAssetType, "FlaggedForReview")

	resultsIterator, err := ctx.GetStub().GetQueryResult(queryString)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer resultsIterator.Close()

	// 2. Use a map to count claims per policy
	// map[policyID] -> SchemeSummary
	summaryMap := make(map[string]SchemeSummary)

	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}

		var claim Claim
		if err := json.Unmarshal(queryResponse.Value, &claim); err != nil {
			return nil, fmt.Errorf("failed to unmarshal claim: %w", err)
		}

		// 3. If a policyID exists, increment its count
		if claim.PolicyID != "" {
			summary, ok := summaryMap[claim.PolicyID]
			if !ok {
				// First time seeing this policy, create a new entry
				summary = SchemeSummary{
					PolicyID:     claim.PolicyID,
					PolicyName:   claim.PolicyName,
					PendingCount: 1,
				}
			} else {
				// Policy already seen, just increment the count
				summary.PendingCount++
			}
			summaryMap[claim.PolicyID] = summary
		}
	}

	// 4. Convert the map to a list for the final JSON array
	summaries := make([]SchemeSummary, 0, len(summaryMap))
	for _, summary := range summaryMap {
		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// GetClaimsByPolicyAndStatus finds claims matching both a policyID and a status
func (s *SmartContract) GetClaimsByPolicyAndStatus(ctx contractapi.TransactionContextInterface, policyID string, status string) ([]Claim, error) {
	// This CouchDB query finds documents matching both fields
	queryString := fmt.Sprintf(`{"selector":{"docType":"%s","policyID":"%s","status":"%s"}}`, claimAssetType, policyID, status)

	resultsIterator, err := ctx.GetStub().GetQueryResult(queryString)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer resultsIterator.Close()

	var claims []Claim
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}

		var claim Claim
		if err := json.Unmarshal(queryResponse.Value, &claim); err != nil {
			return nil, fmt.Errorf("failed to unmarshal claim: %w", err)
		}
		claims = append(claims, claim)
	}

	// Return an empty list "[]" instead of "null" if no claims are found
	if claims == nil {
		claims = make([]Claim, 0)
	}

	return claims, nil
}

// GetFlaggedClaimCounts performs an aggregation query to count all claims with status "FlaggedForReview" and groups them by policyID.
func (s *SmartContract) GetFlaggedClaimCounts(ctx contractapi.TransactionContextInterface) (string, error) {
	queryString := fmt.Sprintf(`{
		"selector": {
			"docType": "claim",
			"status": "FlaggedForReview"
		},
		"fields": ["policyID"]
	}`)

	resultsIterator, err := ctx.GetStub().GetQueryResult(queryString)
	if err != nil {
		return "", fmt.Errorf("query failed: %w", err)
	}
	defer resultsIterator.Close()

	// 2. Count the results in a map
	actualCounts := make(map[string]int)
	for resultsIterator.HasNext() {
		response, err := resultsIterator.Next()
		if err != nil {
			continue 
		}
		
		var result struct {
			PolicyID string `json:"policyID"`
		}
		
		if err := json.Unmarshal(response.Value, &result); err == nil {
			if result.PolicyID != "" {
				actualCounts[result.PolicyID]++
			}
		}
	}

	countsJSON, err := json.Marshal(actualCounts)
	if err != nil {
		return "", fmt.Errorf("failed to marshal counts: %w", err)
	}

	return string(countsJSON), nil
}

func main() {
	chaincode, err := contractapi.NewChaincode(&SmartContract{})
	if err != nil {
		log.Panicf("Error creating insurance chaincode: %v", err)
	}

	if err := chaincode.Start(); err != nil {
		log.Panicf("Error starting insurance chaincode: %v", err)
	}
}