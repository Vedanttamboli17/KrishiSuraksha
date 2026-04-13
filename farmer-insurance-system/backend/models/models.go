package models

import "encoding/json"

type AadharRecord struct {
	Aadhar  string `json:"aadhar"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

// RegisterFarmerRequest defines the structure for the /registerFarmer API request body
type RegisterFarmerRequest struct {
	FarmerID string `json:"farmerID" xml:"farmerID" form:"farmerID"`
	Name     string `json:"name" xml:"name" form:"name"`
}

type AddFarmRequest struct {
	FarmID         string `json:"farmID"`
	OwnerFarmerID  string `json:"ownerFarmerID"`
	Location       string `json:"location"`
	CropType       string `json:"cropType"`
	LandRecordHash string `json:"landRecordHash"`
}

type SubmitClaimRequest struct {
	ClaimID           string `json:"claimID"`
	FarmID            string `json:"farmID"`
	Reason            string `json:"reason"`
	DamageDate        string `json:"damageDate"`
	EvidenceHashes    string `json:"evidenceHashes"`
	DamagePercentage  int    `json:"damagePercentage"`
	SatelliteDataHash string `json:"satelliteDataHash"`
	Description 	  string `json:"description"`
	PolicyID    	  string `json:"policyID"`
	PolicyName  	  string `json:"policyName"`
}

type ApproveClaimRequest struct {
	ClaimID      string `json:"claimID"`
	PayoutAmount int    `json:"payoutAmount"`
}

type RejectClaimRequest struct {
	ClaimID            string `json:"claimID"`
	ReasonForRejection string `json:"reasonForRejection"`
}

type UpdateClaimWithSatelliteDataRequest struct {
	ClaimID           string  `json:"claimID"`
	SatelliteDataHash string  `json:"satelliteDataHash"`
	DamagePercentage  int     `json:"damagePercentage"`
	NDVIValue         float64 `json:"ndviValue"`
	// Add other fields from Python script if needed
}

type Farm struct {
	DocType        string `json:"docType"` // Should always be "farm"
	FarmID         string `json:"farmID"`
	OwnerFarmerID  string `json:"ownerFarmerID"`
	Location       string `json:"location"`
	CropType       string `json:"cropType"`
	LandRecordHash string `json:"landRecordHash"` // This is the path/ID used for Firebase
	ActiveClaimID  string `json:"activeClaimID"`
	Status         string `json:"status"` // e.g., "PendingVerification", "Verified", "Rejected"
}

type AnalysisResult struct {
	DamagePercentage  int     `json:"damagePercentage"`
	NdviValue         float64 `json:"ndviValue"`
	SatelliteDataHash string  `json:"satelliteDataHash"`
	AnalysisReport    json.RawMessage `json:"analysisReport"`
}

type Claim struct {
	DocType           string   `json:"docType"`
	ClaimID           string   `json:"claimID"`
	FarmID            string   `json:"farmID"`
	FarmerID          string   `json:"farmerID"`
	SubmitTimestamp   string   `json:"submitTimestamp"`
	Reason            string   `json:"reason"`
	Status            string   `json:"status"`
	Description       string   `json:"description"`
	PolicyID          string   `json:"policyID"`
	PolicyName        string   `json:"policyName"`
	DamageDate        string   `json:"damageDate"`
	EvidenceHashes    []string `json:"evidenceHashes"`
	SatelliteDataHash string   `json:"satelliteDataHash"`
	AnalysisTimestamp string   `json:"analysisTimestamp"`
	DamagePercentage  int      `json:"damagePercentage"`
	NDVIValue         float64  `json:"ndviValue"`
	VerificationFlags []string `json:"verificationFlags"`
	PayoutAmount      int      `json:"payoutAmount"`
	DecisionTimestamp string   `json:"decisionTimestamp"`
	DecisionNotes     string   `json:"decisionNotes"`
	AssignedAuditor   string   `json:"assignedAuditor"`
}

//-----------Government Official Login------------
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Add this struct to match the data in your mock_government_employee_db.json
type GovernmentEmployee struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}