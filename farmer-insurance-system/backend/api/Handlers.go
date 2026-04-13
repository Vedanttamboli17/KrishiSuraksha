package api

import (
	"fmt"
	"github.com/Vedanttamboli17/krushi-suraksha-backend/models" 
	"github.com/Vedanttamboli17/krushi-suraksha-backend/services" 
	"strconv"
	"github.com/gofiber/fiber/v2"
	"encoding/json"
	"path/filepath"
	"os"
	"bytes"
	"io"
	"os/exec"
	"log"
	"strings"
	"math/rand"
    "time"
	"golang.org/x/crypto/bcrypt" 
	"sync"
)

// FabricHandler holds the FabricService instance
type FabricHandler struct {
	Fabric *services.FabricService
	Firebase *services.FirebaseService
	AadharDB map[string]models.AadharRecord
	OtpStore map[string]string
	UserDB      map[string]UserRecord 
	UserDBMutex *sync.RWMutex
	PolicyDB map[string][]Policy
	SaveUserDB func()
	CropCalendar map[string][]int
}

type BankDetails struct {
	AccountHolderName string `json:"accountHolderName"`
	AccountNumber     string `json:"accountNumber"`
	IFSCCode          string `json:"ifscCode"`
	BankName          string `json:"bankName"`
}

type UserRecord struct {
	FarmerID     string `json:"farmerID"`
	Name         string `json:"name"`
	Address      string `json:"address"`
	Mobile       string `json:"mobile"`
	PasswordHash string `json:"passwordHash"`
	BankDetails  *BankDetails `json:"bankDetails"`
}

type Policy struct {
	PolicyID            string `json:"policyID"`
	PolicyName          string `json:"policyName"`
	Coverage            string `json:"coverage"`
	MinThresholdPercent int    `json:"min_threshold_percent"`
	Description         string `json:"description"`
}

type PolicySummary struct {
	PolicyID     string `json:"policyID"`
	PolicyName   string `json:"policyName"`
	PendingCount int    `json:"pendingCount"`
}

type policyFileStructure map[string][]struct {
	PolicyID   string `json:"policyID"`
	PolicyName string `json:"policyName"`
}

// ClaimsStatsResponse defines the JSON structure for our new analytics endpoint
type ClaimsStatsResponse struct {
	Pending  int `json:"pending"`
	Approved int `json:"approved"`
	Rejected int `json:"rejected"`
}

// NewFabricHandler creates a new handler with the Fabric service
func NewFabricHandler(fab *services.FabricService, fb *services.FirebaseService, aadharDB map[string]models.AadharRecord, otpStore map[string]string, userDB map[string]UserRecord,userDBMutex *sync.RWMutex, saveDBFunc func(), policyDB map[string][]Policy, cropCalendar map[string][]int,) *FabricHandler {
	return &FabricHandler{
		Fabric:   fab,
		Firebase: fb,
		AadharDB: aadharDB,
		OtpStore: otpStore,
		UserDB:      userDB,
		UserDBMutex: userDBMutex,
		SaveUserDB:  saveDBFunc,
		PolicyDB:    policyDB,
		CropCalendar: cropCalendar,
	}
}

func (h *FabricHandler) RegisterFarmer(c *fiber.Ctx) error {
	fmt.Println("Received request to /registerFarmer")

	req := new(models.RegisterFarmerRequest)
	if err := c.BodyParser(req); err != nil {
		fmt.Printf("Error parsing request body: %v\n", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot parse JSON",
			"details": err.Error(),
		})
	}

	if req.FarmerID == "" || req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing farmerID or name in request",
		})
	}

	fmt.Printf("Attempting to register farmer: ID=%s, Name=%s\n", req.FarmerID, req.Name)

	_, err := h.Fabric.Contract.SubmitTransaction("RegisterFarmer", req.FarmerID, req.Name)
	if err != nil {
		fmt.Printf("Error submitting transaction to Fabric: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to register farmer on the blockchain",
			"details": err.Error(), 
		})
	}

	fmt.Printf("Successfully registered farmer %s\n", req.FarmerID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message":  "Farmer registered successfully",
		"farmerID": req.FarmerID,
	})
}

func (h *FabricHandler) AddFarm(c *fiber.Ctx) error {
	fmt.Println("Received request to /addFarm")

	req := new(models.AddFarmRequest) 
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}

	if req.FarmID == "" || req.OwnerFarmerID == "" || req.Location == "" || req.LandRecordHash == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing required fields: farmID, ownerFarmerID, location, or landRecordHash"})
	}

	fmt.Printf("Submitting to Fabric: FarmID=%s, Owner=%s, LandRecordHash(URL)=%s\n", req.FarmID, req.OwnerFarmerID, req.LandRecordHash)
	
	_, err := h.Fabric.Contract.SubmitTransaction(
		"AddFarm",
		req.FarmID,
		req.OwnerFarmerID,
		req.Location,
		req.CropType,
		req.LandRecordHash, 
	)
	
	if err != nil {
		fmt.Printf("Error submitting AddFarm transaction to Fabric: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to add farm on the blockchain",
			"details": err.Error(),
		})
	}

	fmt.Printf("Successfully added farm %s for farmer %s\n", req.FarmID, req.OwnerFarmerID)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Farm added successfully and document URL recorded",
		"farmID":  req.FarmID,
	})
}

func (h *FabricHandler) SubmitClaim(c *fiber.Ctx) error {
	req := new(models.SubmitClaimRequest) 
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}

	// Step 1: Submit the claim to the ledger 
	log.Printf("Submitting new claim %s for farm %s", req.ClaimID, req.FarmID)
	_, err := h.Fabric.Contract.SubmitTransaction("SubmitClaim", req.ClaimID, req.FarmID, req.Reason, req.Description, req.PolicyID, req.PolicyName, req.DamageDate, req.EvidenceHashes,)
	if err != nil {
		fmt.Printf("Error submitting SubmitClaim transaction: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to submit claim",
			"details": err.Error(),
		})
	}

	// Step 2: Fetch the farm's polygon data 
	log.Printf("Fetching farm %s to get polygon location...", req.FarmID)
	farmJSON, err := h.Fabric.Contract.EvaluateTransaction("GetFarm", req.FarmID)
	if err != nil {
		log.Printf("Error fetching farm: %v\n", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Failed to find farm on ledger"})
	}

	var farm models.Farm
	if err := json.Unmarshal(farmJSON, &farm); err != nil {
		log.Printf("Error parsing farm JSON: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to parse farm data"})
	}

	if farm.Location == "" {
		log.Printf("Farm %s has no location data. Aborting analysis.", req.FarmID)
		// Return success, but log the error. The claim will stay in "Processing"
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"message": "Claim submitted, but farm has no location data. Analysis skipped.",
		})
	}

	// Step 3: Trigger the Python script asynchronously We run this in a "goroutine" (go func()...) so that it runs in the background. This allows us to send the "Success" response to the farmer *immediately*. The farmer doesn't have to wait for the satellite analysis.
	go func(claimID string, polygonJSON string, damageDate string, reason string) {
		log.Printf("Starting Python analysis (with Fraud Check) for claim: %s", claimID)

		scriptPath, _ := filepath.Abs("python-scripts/satellite_data.py")
		
		// We call the script and pass the claimID as an argument
		cmd := exec.Command("python3", scriptPath, claimID, polygonJSON, damageDate, reason)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		
		err := cmd.Run() 
		
		log.Printf("[Python Log for %s]:\n%s", claimID, stderr.String())

		if err != nil {
			log.Printf("PYTHON SCRIPT FAILED for claim %s: %v", claimID, err)
			
			_, err = h.Fabric.Contract.SubmitTransaction("UpdateClaimStatusFailed", claimID, "Python script failed")
			if err != nil {
				 log.Printf("Failed to set claim %s status to Failed: %v", claimID, err)
			}
			return
		}
		
		// Step 4: Python script finished, get its stdout JSON 
		log.Printf("Python analysis complete for claim: %s", claimID)
		
		type AnalysisReport struct {
			models.AnalysisResult
			VerificationFlags []string `json:"verificationFlags"`
		}
		var analysisResult AnalysisReport

		if err := json.Unmarshal(stdout.Bytes(), &analysisResult); err != nil {
			log.Printf("Failed to parse Python JSON output for claim %s: %v", claimID, err)
			log.Printf("Python output that failed parsing: %s", stdout.String())
			
			// Update claim to "FailedAnalysis"
			_, err = h.Fabric.Contract.SubmitTransaction("UpdateClaimStatusFailed", claimID, "Failed to parse Python output")
			if err != nil {
				 log.Printf("Failed to set claim %s status to Failed: %v", claimID, err)
			}
			return
		}
		
		// Step 5: Call UpdateClaimWithSatelliteData chaincode function 
		log.Printf("Updating chaincode with analysis for claim: %s", claimID)

		verificationFlagsJSON, err := json.Marshal(analysisResult.VerificationFlags)
		if err != nil {
			log.Printf("Failed to marshal verification flags for claim %s: %v", claimID, err)
            // Send an empty list as a fallback
            verificationFlagsJSON = []byte("[]") 
		}
		_, err = h.Fabric.Contract.SubmitTransaction(
			"UpdateClaimWithSatelliteData",
			claimID,
			analysisResult.SatelliteDataHash,
			fmt.Sprintf("%d", analysisResult.DamagePercentage),
			fmt.Sprintf("%f", analysisResult.NdviValue),
			string(verificationFlagsJSON), 
		)
		if err != nil {
			log.Printf("Failed to update chaincode with analysis for claim %s: %v", claimID, err)
			return
		}
		
		log.Printf("Successfully updated chaincode for claim %s", claimID)

		// The claim is updated. Now, let's query it to see if it was auto-approved.
		claimJSON, err := h.Fabric.Contract.EvaluateTransaction("GetClaim", claimID)
		if err != nil {
			log.Printf("AutoPay: Failed to re-query claim %s to check status: %v", claimID, err)
			return 
		}

		var updatedClaim models.Claim 
		if err := json.Unmarshal(claimJSON, &updatedClaim); err != nil {
			log.Printf("AutoPay: Failed to parse claim JSON for %s: %v", claimID, err)
			return
		}

		// Check for the "ApprovedAuto" status
		if updatedClaim.Status == "ApprovedAuto" {
			log.Printf("AutoPay: Claim %s was AUTO-APPROVED. Processing mock payment...", claimID)

			// This is where real policy logic would go. We'll use a mock calculation.
			// (Mock Farm Value * Damage Percentage)
			// NOTE: This is a simplified calculation for the project.
			mockFarmValue := 50000 // Mock value, in a real system, this would be on the Farm asset
			payoutAmount := int(float64(updatedClaim.DamagePercentage) / 100.0 * float64(mockFarmValue))

			// Convert payout to string for the chaincode function
			payoutAmountStr := strconv.Itoa(payoutAmount)

			// Call the new MarkAsPaid function
			_, err = h.Fabric.Contract.SubmitTransaction("MarkAsPaid", claimID, payoutAmountStr)
			if err != nil {
				log.Printf("AutoPay: FAILED to mark claim %s as Paid: %v", claimID, err)
			} else {
				log.Printf("AutoPay: Successfully processed payment for %s and marked as 'Paid'. Mock Payout: %d", claimID, payoutAmount)
			}
		}
	}(req.ClaimID, farm.Location, req.DamageDate, req.Reason)

	// --- Step 3: Return success to the farmer immediately ---
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Claim submitted successfully, now processing satellite data",
		"claimID": req.ClaimID,
	})
}

func (h *FabricHandler) ApproveClaim(c *fiber.Ctx) error {
	fmt.Println("Received request to /approveClaim")

	req := new(models.ApproveClaimRequest)
	if err := c.BodyParser(req); err != nil {
		fmt.Printf("Error parsing request body: %v\n", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON", "details": err.Error()})
	}

	if req.ClaimID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing claimID in request"})
	}
	// Payout amount can be 0, so no check needed unless you have specific business logic

	fmt.Printf("Attempting to approve claim: ClaimID=%s, Payout=%d\n", req.ClaimID, req.PayoutAmount)

	payoutAmountStr := strconv.Itoa(req.PayoutAmount)

	// Args: claimID, payoutAmount
	_, err := h.Fabric.Contract.SubmitTransaction("ApproveClaim", req.ClaimID, payoutAmountStr)
	if err != nil {
		fmt.Printf("Error submitting ApproveClaim transaction to Fabric: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to approve claim on the blockchain",
			"details": err.Error(),
		})
	}

	fmt.Printf("Successfully approved claim %s\n", req.ClaimID)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Claim approved successfully",
		"claimID": req.ClaimID,
		"status":  "Approved",
	})
}

func (h *FabricHandler) RejectClaim(c *fiber.Ctx) error {
	fmt.Println("Received request to /rejectClaim")

	req := new(models.RejectClaimRequest)
	if err := c.BodyParser(req); err != nil {
		fmt.Printf("Error parsing request body: %v\n", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON", "details": err.Error()})
	}

	if req.ClaimID == "" || req.ReasonForRejection == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing claimID or reasonForRejection in request"})
	}

	fmt.Printf("Attempting to reject claim: ClaimID=%s, Reason=%s\n", req.ClaimID, req.ReasonForRejection)

	_, err := h.Fabric.Contract.SubmitTransaction("RejectClaim", req.ClaimID, req.ReasonForRejection)
	if err != nil {
		fmt.Printf("Error submitting RejectClaim transaction to Fabric: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to reject claim on the blockchain",
			"details": err.Error(),
		})
	}

	fmt.Printf("Successfully rejected claim %s\n", req.ClaimID)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Claim rejected successfully",
		"claimID": req.ClaimID,
		"status":  "Rejected",
	})
}

func (h *FabricHandler) VerifyFarm(c *fiber.Ctx) error {
    fmt.Println("Received request to /verifyFarm")

    // 1. Get FarmID from URL parameter (e.g., /verifyFarm/FARM003)
    farmID := c.Params("farmID") // Assuming we'll use a URL parameter
    if farmID == "" {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing farmID in URL path"})
    }

    fmt.Printf("Attempting to verify farm: FarmID=%s\n", farmID)

    // Args: farmID
    _, err := h.Fabric.Contract.SubmitTransaction("VerifyFarm", farmID)
    if err != nil {
        fmt.Printf("Error submitting VerifyFarm transaction to Fabric: %v\n", err)
        // Check for specific chaincode errors (like farm not pending)
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error":   "Failed to verify farm on the blockchain",
            "details": err.Error(),
        })
    }

    fmt.Printf("Successfully verified farm %s\n", farmID)

    return c.Status(fiber.StatusOK).JSON(fiber.Map{
        "message": "Farm verified successfully",
        "farmID":  farmID,
        "status":  "Verified",
    })
}

func (h *FabricHandler) UpdateClaimWithSatelliteData(c *fiber.Ctx) error {
	fmt.Println("Received request to /updateClaimWithSatelliteData")

	req := new(models.UpdateClaimWithSatelliteDataRequest)
	if err := c.BodyParser(req); err != nil {
		fmt.Printf("Error parsing request body: %v\n", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON", "details": err.Error()})
	}

	if req.ClaimID == "" || req.SatelliteDataHash == "" {
		// Allow 0 for DamagePercentage and NDVIValue
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing required fields in request (claimID, satelliteDataHash)"})
	}

	fmt.Printf("Attempting to update claim %s with satellite data\n", req.ClaimID)

	// Convert numeric types to strings for chaincode args
	damagePercentageStr := strconv.Itoa(req.DamagePercentage)
	// Use fmt.Sprintf for float conversion to handle precision if needed
	ndviValueStr := fmt.Sprintf("%f", req.NDVIValue)

	// Args: claimID, satelliteDataHash, damagePercentage, ndviValue
	_, err := h.Fabric.Contract.SubmitTransaction("UpdateClaimWithSatelliteData", req.ClaimID, req.SatelliteDataHash, damagePercentageStr, ndviValueStr)
	if err != nil {
		fmt.Printf("Error submitting UpdateClaimWithSatelliteData transaction: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to update claim with satellite data",
			"details": err.Error(),
		})
	}

	fmt.Printf("Successfully updated claim %s with satellite data\n", req.ClaimID)

	// 3. (Optional) Query the updated claim status to return it
	// Note: It's often better to just return success and let the client query separately if needed.
	// For simplicity here, we just return a success message.
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Claim updated successfully with satellite data",
		"claimID": req.ClaimID,
	})
}

//---------------------EVALUATE REQUESTS----------------------

// queryChaincodeByID is a helper for read-only queries for a single asset.
func (h *FabricHandler) queryChaincodeByID(c *fiber.Ctx, funcName string, assetID string) error {
	fmt.Printf("Received query request for %s: %s\n", funcName, assetID)

	resultJSON, err := h.Fabric.Contract.EvaluateTransaction(funcName, assetID)
	if err != nil {
		fmt.Printf("Error evaluating %s transaction: %v\n", funcName, err)
		if strings.Contains(err.Error(), "does not exist") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Asset not found", "details": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   fmt.Sprintf("Failed to query %s", funcName),
			"details": err.Error(),
		})
	}
	// This handles the (nil, nil) "not found" case
	if resultJSON == nil || len(resultJSON) == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Asset not found"})
	}

	fmt.Printf("Successfully queried %s %s\n", funcName, assetID)
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return c.Send(resultJSON)
}

// queryChaincodeList is a helper for list-based "rich queries". It returns an empty array "[]" if no results are found. 
func (h *FabricHandler) queryChaincodeList(c *fiber.Ctx, funcName string, args ...string) error {
	fmt.Printf("Received list query request for %s with args: %v\n", funcName, args)

	resultJSON, err := h.Fabric.Contract.EvaluateTransaction(funcName, args...)
	if err != nil {
		fmt.Printf("Error evaluating %s transaction: %v\n", funcName, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   fmt.Sprintf("Failed to query %s", funcName),
			"details": err.Error(),
		})
	}
	// (nil, nil) for a list query should return an empty array
	if resultJSON == nil || len(resultJSON) == 0 {
		return c.Status(fiber.StatusOK).SendString("[]")
	}

	fmt.Printf("Successfully queried %s\n", funcName)
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return c.Send(resultJSON) // The chaincode function already returns a JSON array
}

// --- Simple "Get by ID" Handlers ---

// GetFarmer handles GET /api/farmer/:id
func (h *FabricHandler) GetFarmer(c *fiber.Ctx) error {
	assetID := c.Params("id")
	if assetID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "'id' URL parameter is required"})
	}
	return h.queryChaincodeByID(c, "GetFarmer", assetID)
}

// GetFarm handles GET /api/farm/:id
func (h *FabricHandler) GetFarm(c *fiber.Ctx) error {
	assetID := c.Params("id")
	if assetID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "'id' URL parameter is required"})
	}
	return h.queryChaincodeByID(c, "GetFarm", assetID)
}

// GetClaim handles GET /api/claim/:id
func (h *FabricHandler) GetClaim(c *fiber.Ctx) error {
	assetID := c.Params("claimID")
	if assetID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "'id' URL parameter is required"})
	}
	return h.queryChaincodeByID(c, "GetClaim", assetID)
}

// --- "Rich Query" List Handlers ---

// GetFarmsByFarmer handles GET /api/farms/by-farmer/:farmerID
func (h *FabricHandler) GetFarmsByFarmer(c *fiber.Ctx) error {
	farmerID := c.Params("farmerID")
	if farmerID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "farmerID URL parameter is required"})
	}
	return h.queryChaincodeList(c, "GetAllFarmsByOwner", farmerID)
}

// GetClaimsByFarmer handles GET /api/claims/by-farmer/:farmerID
func (h *FabricHandler) GetClaimsByFarmer(c *fiber.Ctx) error {
	farmerID := c.Params("farmerID")
	if farmerID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "farmerID URL parameter is required"})
	}
	return h.queryChaincodeList(c, "GetAllClaimsByFarmer", farmerID)
}

// GetClaimsByFarm handles GET /api/claims/by-farm/:farmID
func (h *FabricHandler) GetClaimsByFarm(c *fiber.Ctx) error {
	farmID := c.Params("farmID")
	if farmID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "farmID URL parameter is required"})
	}
	return h.queryChaincodeList(c, "GetAllClaimsByFarm", farmID)
}

// GetClaimsByStatus handles GET /api/claims/by-status/:status
// GetClaimsByStatus handles GET /claims/by-status/:status
// This is for the admin website to find pending claims.
func (h *FabricHandler) GetClaimsByStatus(c *fiber.Ctx) error {
	status := c.Params("status")
	if status == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "status URL parameter is required"})
	}

	fmt.Printf("Received list query request for GetClaimsByStatus with status: %s\n", status)
	
	// This function (GetClaimsByStatus) now returns ([]models.Claim, error)
	// The Go SDK will marshal it into a JSON array string (as []byte).
	claimData, err := h.Fabric.Contract.EvaluateTransaction("GetClaimsByStatus", status)
	if err != nil {
		// This is where your error is currently happening
		fmt.Printf("Error evaluating GetClaimsByStatus transaction: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to query claims by status",
			"details": err.Error(),
		})
	}
	
	if claimData == nil || len(claimData) == 0 {
		// Return an empty JSON array `[]`
		return c.SendString("[]")
	}
	
	fmt.Println("Successfully queried GetClaimsByStatus")
	// Send the raw JSON array string from the chaincode
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return c.Send(claimData) // Send the raw []byte
}

// func (h *FabricHandler) GetClaimsByStatus(c *fiber.Ctx) error {
// 	status := c.Params("status")
// 	if status == "" {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "status URL parameter is required"})
// 	}
// 	return h.queryChaincodeList(c, "GetClaimsByStatus", status)
// }

func (h *FabricHandler) GetFarmsByStatus(c *fiber.Ctx) error {
	status := c.Params("status")
	if status == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "status URL parameter is required"})
	}

	fmt.Printf("Received list query request for GetFarmsByStatus with status: %s\n", status)
	
	farmData, err := h.Fabric.Contract.EvaluateTransaction("GetFarmsByStatus", status)
	if err != nil {
		fmt.Printf("Error evaluating GetFarmsByStatus transaction: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to query farms by status",
			"details": err.Error(),
		})
	}
	
	if farmData == nil || len(farmData) == 0 {
		return c.SendString("[]")
	}
	
	fmt.Println("Successfully queried GetFarmsByStatus")
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return c.Send(farmData)
}

// GetFarmWithDetails handles GET /api/farm-details/:id.
func (h *FabricHandler) GetFarmWithDetails(c *fiber.Ctx) error {
	farmID := c.Params("farmID")
	if farmID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "farm 'id' URL parameter is required"})
	}

	// 1. Get the Farm data from the blockchain
	farmJSON, err := h.Fabric.Contract.EvaluateTransaction("GetFarm", farmID)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Farm not found", "details": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to query farm", "details": err.Error()})
	}
	if farmJSON == nil || len(farmJSON) == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Farm not found"})
	}

	var farm models.Farm // Assuming Farm struct is in your models package
	if err := json.Unmarshal(farmJSON, &farm); err != nil {
		fmt.Printf("Error unmarshaling farm JSON: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to parse farm data from ledger"})
	}

	fileURL, err := h.Firebase.GetFileURL(farm.LandRecordHash)
	if err != nil {
		fmt.Printf("Warning: Failed to get file URL from Firebase: %v\n", err)
		fileURL = ""
	}

	type FarmDetailsResponse struct {
		models.Farm
		LandRecordURL string `json:"landRecordURL"`
	}

	response := FarmDetailsResponse{
		Farm:          farm,
		LandRecordURL: fileURL,
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

// GetClaimSummaryByScheme calls the chaincode to get a grouped count of pending claims by policy
// func (h *FabricHandler) GetClaimSummaryByScheme(c *fiber.Ctx) error {
// 	log.Println("Received request for /claims/summary-by-scheme")

// 	// Call the new chaincode function. It takes no arguments.
// 	resultJSON, err := h.Fabric.Contract.EvaluateTransaction("GetClaimSummaryByScheme")
// 	if err != nil {
// 		log.Printf("Error evaluating GetClaimSummaryByScheme: %v", err)
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error":   "Failed to query claim summary",
// 			"details": err.Error(),
// 		})
// 	}
	
// 	// Return an empty list "[]" if no summary is found
// 	if resultJSON == nil || len(resultJSON) == 0 {
// 		return c.Status(fiber.StatusOK).SendString("[]")
// 	}

// 	log.Println("Successfully queried /claims/summary-by-scheme")
// 	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
// 	return c.Send(resultJSON)
// }

// GetClaimsByPolicyAndStatus calls the chaincode to get a filtered list of claims
func (h *FabricHandler) GetClaimsByPolicyAndStatus(c *fiber.Ctx) error {
	// Get params from the URL
	policyID := c.Params("policyID")
	status := c.Params("status")
	log.Printf("Received request for /claims/policy/%s/status/%s", policyID, status)

	// Call the new chaincode function with the params
	resultJSON, err := h.Fabric.Contract.EvaluateTransaction("GetClaimsByPolicyAndStatus", policyID, status)
	if err != nil {
		log.Printf("Error evaluating GetClaimsByPolicyAndStatus: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to query filtered claims",
			"details": err.Error(),
		})
	}

	// Return an empty list "[]" if no claims are found
	if resultJSON == nil || len(resultJSON) == 0 {
		return c.Status(fiber.StatusOK).SendString("[]")
	}

	log.Printf("Successfully queried /claims/policy/%s/status/%s", policyID, status)
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return c.Send(resultJSON)
}

func (h *FabricHandler) GetClaimsStats(c *fiber.Ctx) error {
	log.Println("Received request for /claims/stats")

	// We need to get counts for three categories: Pending, Approved, Rejected.
	// We'll query the chaincode for all statuses that fall into these categories.

	stats := ClaimsStatsResponse{
		Pending:  0,
		Approved: 0,
		Rejected: 0,
	}

	// Helper function to query chaincode and get the count
	get_count := func(status string) (int, error) {
		// We call GetClaimsByStatus, which returns a JSON string of an array of claims
		claimsJSON, err := h.Fabric.Contract.EvaluateTransaction("GetClaimsByStatus", status)
		if err != nil {
			// If no claims are found with this status, chaincode might return an error.
			// We check for "does not exist" and treat it as 0, not a fatal error.
			if strings.Contains(err.Error(), "does not exist") {
				return 0, nil
			}
			log.Printf("Error querying 'GetClaimsByStatus' for status '%s': %v", status, err)
			return 0, err // A real error
		}

		if claimsJSON == nil || len(claimsJSON) == 0 {
			return 0, nil // No claims found
		}

		// Unmarshal into a list of *anything* just to get the count
		var claimsList []interface{}
		if err := json.Unmarshal(claimsJSON, &claimsList); err != nil {
			log.Printf("Error unmarshaling claims list for status '%s': %v", status, err)
			return 0, err // JSON parsing error
		}

		return len(claimsList), nil
	}

	// --- 1. Pending Claims ---
	// "Pending" includes claims being processed and those flagged for review
	countProcessing, err1 := get_count("ProcessingSatelliteData")
	countFlagged, err2 := get_count("FlaggedForReview")
	if err1 != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to query 'ProcessingSatelliteData' claims stats"})
	}
	if err2 != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to query 'FlaggedForReview' claims stats"})
	}
	stats.Pending = countProcessing + countFlagged

	// --- 2. Approved Claims ---
	// "Approved" includes auto-approved, manually approved, and paid claims
	countApproved, err3 := get_count("Approved")       // Manual admin approval
	countApprovedAuto, err4 := get_count("ApprovedAuto") // New auto-approval
	countPaid, err5 := get_count("Paid")             // New paid status
	if err3 != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to query 'Approved' claims stats"})
	}
	if err4 != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to query 'ApprovedAuto' claims stats"})
	}
	if err5 != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to query 'Paid' claims stats"})
	}
	stats.Approved = countApproved + countApprovedAuto + countPaid

	// --- 3. Rejected Claims ---
	countRejected, err6 := get_count("Rejected")
	if err6 != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to query 'Rejected' claims stats"})
	}
	stats.Rejected = countRejected

	log.Printf("Returning claims stats: %+v", stats)
	return c.Status(fiber.StatusOK).JSON(stats)
}

// -------------- WEATHER DATA ----------------

func (h *FabricHandler) GetWeather(c *fiber.Ctx) error {
	// Get lat/lon from query parameters
	lat := c.Query("lat")
	lon := c.Query("lon")

	if lat == "" || lon == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "missing 'lat' or 'lon' query parameter",
		})
	}

	weatherData, err := services.GetWeatherForLocation(lat, lon)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(weatherData)
}

type RequestOTPPayload struct {
	Mobile string `json:"mobile"`
	Aadhar string `json:"aadhar"`
}

func (h *FabricHandler) RequestOTP(c *fiber.Ctx) error {
	var payload RequestOTPPayload
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request payload",
		})
	}

	// Check Aadhar DB
	record, exists := h.AadharDB[payload.Mobile]
	if !exists {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Mobile number not found in Aadhar records",
		})
	}

	// Verify Aadhar number
	if record.Aadhar != payload.Aadhar {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Aadhar number does not match mobile number",
		})
	}

	// Generate and "send" (log) OTP
	otp := fmt.Sprintf("%06d", rand.Intn(1000000)) // 6-digit OTP
	h.OtpStore[payload.Mobile] = otp

	log.Printf("✅ OTP for %s: %s\n", payload.Mobile, otp)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "OTP has been sent (check your Go terminal)",
	})
}

type VerifyOTPPayload struct {
	Mobile string `json:"mobile"`
	Aadhar string `json:"aadhar"`
	OTP    string `json:"otp"`
	Password string `json:"password"`
}

type RegistrationSuccessResponse struct {
	FarmerID string `json:"farmerID"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	Mobile   string `json:"mobile"`
	Token    string `json:"token"` 
}

func (h *FabricHandler) VerifyOTPAndRegister(c *fiber.Ctx) error {
	var payload VerifyOTPPayload
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request payload",
		})
	}

	if len(payload.Password) < 6 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Password must be at least 6 characters",
		})
	}

	storedOTP, exists := h.OtpStore[payload.Mobile]
	if !exists || storedOTP != payload.OTP {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid or expired OTP",
		})
	}

	delete(h.OtpStore, payload.Mobile)

	// Get Farmer Data from Aadhar record
	record := h.AadharDB[payload.Mobile]

	// Generate a new, unique FarmerID
	farmerID := fmt.Sprintf("FARMER_%d", time.Now().UnixNano())

	log.Printf("Registering farmer on blockchain: ID=%s, Name=%s", farmerID, record.Name)

	// Register on Blockchain (using your existing Fabric service)
	_, err := h.Fabric.Contract.SubmitTransaction("RegisterFarmer", farmerID, record.Name)
	if err != nil {
		fmt.Printf("Error submitting transaction to Fabric: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to register farmer on the blockchain",
			"details": err.Error(),
		})
	}
	log.Println("✅ Farmer registered on blockchain.")

	passwordHashBytes, err := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("❌ Error hashing password: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to process registration",
		})
	}
	passwordHash := string(passwordHashBytes)

	// --- 5. Save user to our User DB ---
	newUser := UserRecord{
		FarmerID:     farmerID,
		Name:         record.Name,
		Address:      record.Address,
		Mobile:       payload.Mobile,
		PasswordHash: passwordHash, // Store the hash
	}

	h.UserDBMutex.Lock() // Lock for writing
	h.UserDB[payload.Mobile] = newUser
	h.UserDBMutex.Unlock()

	// Asynchronously save the updated DB to file
	go h.SaveUserDB() 
	log.Printf("✅ User %s saved to local user DB.", payload.Mobile)

	// TODO: Save full profile (with password, etc.) to
	// external database (e.g., Firebase, Postgres, or another JSON)
	// Will do this when building the login_page.

	// 6. Return farmer data and token to Flutter
	response := RegistrationSuccessResponse{
		FarmerID: farmerID,
		Name:     record.Name,
		Address:  record.Address,
		Mobile:   payload.Mobile,
		Token:    fmt.Sprintf("mock_session_token_for_%s", farmerID), // Create a mock token
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

type LoginPayload struct {
	Mobile   string `json:"mobile"`
	Password string `json:"password"`
}

// LoginResponse includes user details and token
type LoginResponse struct {
	FarmerID string `json:"farmerID"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	Mobile   string `json:"mobile"`
	Token    string `json:"token"`
}

func (h *FabricHandler) Login(c *fiber.Ctx) error {
	var payload LoginPayload
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request payload"})
	}

	// Find user in User DB
	h.UserDBMutex.RLock() // Read lock
	user, exists := h.UserDB[payload.Mobile]
	h.UserDBMutex.RUnlock()

	if !exists {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid mobile number or password"})
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(payload.Password))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid mobile number or password"})
	}

	log.Printf("Login successful for user %s", payload.Mobile)

	// Generate a token (mock for now)
	token := fmt.Sprintf("mock_session_token_for_%s", user.FarmerID)

	// Return user data and token
	response := LoginResponse{
		FarmerID: user.FarmerID,
		Name:     user.Name,
		Address:  user.Address,
		Mobile:   user.Mobile,
		Token:    token,
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

// Helper function to save user DB (needs access to userDB and userDBMutex)
// You might need to move this to main.go or pass the variables here.
// For simplicity, defining it here temporarily assuming access.
func saveUserDB() {
    // This needs access to the userDB map and userDBMutex from main.go
    // Ideally, pass them or use a shared state management approach.
    // Placeholder - implement actual saving logic as shown in main.go update
    log.Println("💾 [Handler Scope] Requesting saveUserDB...")
    // In a real app, you'd likely call a function passed during handler setup
    // For now, this is just a reminder that saving needs to happen.
}

// ------------------BANK DETAILS------------------
func (h *FabricHandler) GetBankDetails(c *fiber.Ctx) error {
	// TODO: In a real app, get mobile from JWT token
	// For now, we'll get it from a query param for simplicity.
	mobile := c.Query("mobile")
	if mobile == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "mobile query parameter is required"})
	}

	h.UserDBMutex.RLock()
	user, exists := h.UserDB[mobile]
	h.UserDBMutex.RUnlock()

	if !exists {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	if user.BankDetails == nil {
		// Return 200 OK with null, which is not an error
		return c.Status(fiber.StatusOK).JSON(nil)
	}

	return c.Status(fiber.StatusOK).JSON(user.BankDetails)
}

// UpdateBankDetails ---
func (h *FabricHandler) UpdateBankDetails(c *fiber.Ctx) error {
	mobile := c.Query("mobile") // TODO: Get from auth token
	if mobile == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "mobile query parameter is required"})
	}

	var payload BankDetails
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}

	if payload.AccountNumber == "" || payload.IFSCCode == "" || payload.AccountHolderName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing required bank details"})
	}

	h.UserDBMutex.Lock()
	user, exists := h.UserDB[mobile]
	if !exists {
		h.UserDBMutex.Unlock()
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	user.BankDetails = &payload
	h.UserDB[mobile] = user
	h.UserDBMutex.Unlock()

	go h.SaveUserDB()

	log.Printf("✅ Updated bank details for %s", mobile)
	return c.Status(fiber.StatusOK).JSON(payload)
}

// GetPolicy Handler
func (h *FabricHandler) GetPolicy(c *fiber.Ctx) error {
	policyType := c.Query("type")
	if policyType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "missing 'type' query parameter (e.g., 'Flood Damage', 'Drought')",
		})
	}

	// Look up the policy in our in-memory map
    // --- MODIFIED ---
	policies, ok := h.PolicyDB[policyType] // <-- Variable name changed to 'policies' (plural)
	if !ok {
		// If no policies are found, return an empty list
        // This is better for the app than an error
		return c.Status(fiber.StatusOK).JSON([]Policy{}) 
	}

	// Return the found list of policies
	return c.Status(fiber.StatusOK).JSON(policies)
    // --- END MODIFIED ---
}

// isHarvestSeason checks if the current month is in the harvest list for the given crop
func (h *FabricHandler) isHarvestSeason(cropType string) bool {
	// Get the current month (1-12)
	currentMonth := int(time.Now().Month())

	// Find the harvest months for this crop
	harvestMonths, ok := h.CropCalendar[cropType]
	if !ok {
		// If crop is not in our calendar, assume it's NOT harvest season
		// to be safe (i.e., we should monitor it).
		log.Printf("ProactiveMonitor: Crop type '%s' not found in crop_calendar.json. Proceeding with check.", cropType)
		return false
	}
	// Check if the current month is in the list
	for _, month := range harvestMonths {
		if month == currentMonth {
			return true 
		}
	}
	return false 
}

// RunProactiveMonitoring is the "cron job" that checks all verified farms
func (h *FabricHandler) RunProactiveMonitoring() {
	log.Println("ProactiveMonitor: Fetching all 'Verified' farms from ledger...")

	// 1. Get all verified farms
	farmsJSON, err := h.Fabric.Contract.EvaluateTransaction("GetFarmsByStatus", "Verified")
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			log.Println("ProactiveMonitor: No 'Verified' farms found. Job complete.")
			return
		}
		log.Printf("ProactiveMonitor: FAILED to query farms: %v", err)
		return
	}

	var farms []models.Farm
	if err := json.Unmarshal(farmsJSON, &farms); err != nil {
		log.Printf("ProactiveMonitor: FAILED to parse farms JSON: %v", err)
		return
	}

	log.Printf("ProactiveMonitor: Found %d 'Verified' farms. Starting checks...", len(farms))

	// 2. Loop through each farm and check it
	for _, farm := range farms {
		// 3. Check if it's harvest season for this farm's crop
		if h.isHarvestSeason(farm.CropType) {
			log.Printf("ProactiveMonitor: Skipping farm %s (%s). Reason: Harvest Season.", farm.FarmID, farm.CropType)
			continue // Skip this farm, it's harvest time
		}

		// 4. Not harvest season. Run the Python NDVI check.
		log.Printf("ProactiveMonitor: Checking farm %s (%s)...", farm.FarmID, farm.CropType)
		
        // We run this in a goroutine so that checks for many farms
        // can run in parallel and don't block each other.
		go func(f models.Farm) {
            // NOTE: This assumes a NEW script, "monitor_ndvi.py", 
            // designed to check current NDVI without a claimID.
			scriptPath, _ := filepath.Abs("python-scripts/monitor_ndvi.py")
            
            // This script takes FarmID (for logging) and Polygon
			cmd := exec.Command("python3", scriptPath, f.FarmID, f.Location)

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			log.Printf("[Python Monitor Log for %s]:\n%s", f.FarmID, stderr.String())

			if err != nil {
				log.Printf("ProactiveMonitor: Python script FAILED for farm %s: %v", f.FarmID, err)
				return
			}

			// 5. Parse the result from the monitor script
            // (Assuming it returns the same AnalysisResult struct)
			var analysisResult models.AnalysisResult
			if err := json.Unmarshal(stdout.Bytes(), &analysisResult); err != nil {
				log.Printf("ProactiveMonitor: Failed to parse Python JSON for farm %s: %v", f.FarmID, err)
				return
			}

			// 6. Check for damage and log an alert
			if analysisResult.DamagePercentage >= 30 {
                // --- THIS IS THE PROACTIVE ALERT ---
				log.Printf("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
				log.Printf("!!! PROACTIVE DAMAGE ALERT for Farm %s !!!", f.FarmID)
				log.Printf("!!! Owner: %s, Crop: %s", f.OwnerFarmerID, f.CropType)
				log.Printf("!!! Damage Detected: %d%% (Current NDVI: %f)", analysisResult.DamagePercentage, analysisResult.NdviValue)
				log.Printf("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
                // (In a real system, this would send a push notification)
			} else {
				log.Printf("ProactiveMonitor: Farm %s is healthy. (Damage: %d%%)", f.FarmID, analysisResult.DamagePercentage)
			}

		}(farm)
        
        // Add a small delay to avoid overwhelming the Python/Sentinel APIs
        time.Sleep(2 * time.Second)
	}
}

// ------------------GOVERNMENT WEBSITE------------------

func (h *FabricHandler) LoginGov(c *fiber.Ctx) error {
	var req models.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse request"})
	}

	jsonFile, err := os.Open("mock_government_employee_db.json")
	if err != nil {
		fmt.Println("Error opening mock DB file:", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Login service unavailable"})
	}
	defer jsonFile.Close()

	byteValue, _ := io.ReadAll(jsonFile)

	var employees []models.GovernmentEmployee
	if err := json.Unmarshal(byteValue, &employees); err != nil {
		fmt.Println("Error unmarshaling mock DB:", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Login service error"})
	}

	for _, user := range employees {
		if user.Email == req.Email {
			if user.Password == req.Password {
				return c.Status(fiber.StatusOK).JSON(fiber.Map{
					"message": "Login successful",
					"name":    user.Name,
					"email":   user.Email,
				})
			}
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid password"})
		}
	}

	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
}

func loadAllPolicies(filePath string) (map[string]string, error) {
	file, err := os.ReadFile(filePath) 
	if err != nil {
		return nil, fmt.Errorf("failed to read policy file: %w", err)
	}

	var policiesData policyFileStructure
	if err := json.Unmarshal(file, &policiesData); err != nil {
		return nil, fmt.Errorf("failed to parse policy file: %w", err)
	}

	uniquePolicies := make(map[string]string)
	
	for _, policies := range policiesData {
		for _, policy := range policies {
			uniquePolicies[policy.PolicyID] = policy.PolicyName
		}
	}

	return uniquePolicies, nil
}

func (h *FabricHandler) GetClaimSummaryByScheme(c *fiber.Ctx) error {
	allPolicies, err := loadAllPolicies("policies.json")
	if err != nil {
		fmt.Println("Error loading policies.json:", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to load policy data"})
	}

	fmt.Printf("Successfully loaded %d unique policies from policies.json\n", len(allPolicies))

	countsJSON, err := h.Fabric.Contract.EvaluateTransaction("GetFlaggedClaimCounts")
	if err != nil {
		fmt.Printf("Error evaluating GetFlaggedClaimCounts: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to query blockchain counts"})
	}

	actualCounts := make(map[string]int)
	if err := json.Unmarshal(countsJSON, &actualCounts); err != nil {
		fmt.Printf("Error unmarshaling chaincode counts: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to parse blockchain data"})
	}

	finalSummary := make([]PolicySummary, 0)
	for policyID, policyName := range allPolicies {
		count := actualCounts[policyID] 
		finalSummary = append(finalSummary, PolicySummary{
			PolicyID:     policyID,
			PolicyName:   policyName,
			PendingCount: count,
		})
	}
	return c.Status(fiber.StatusOK).JSON(finalSummary)
}
