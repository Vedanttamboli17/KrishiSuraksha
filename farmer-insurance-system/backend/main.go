package main

import (
	"fmt"
	"log"
	"os"
	"io"
	"encoding/json"
	"sync"
	"time"

	"github.com/Vedanttamboli17/krushi-suraksha-backend/api"
	"github.com/Vedanttamboli17/krushi-suraksha-backend/services"
	"github.com/Vedanttamboli17/krushi-suraksha-backend/models"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

// type AadharRecord struct {
// 	Aadhar  string `json:"aadhar"`
// 	Name    string `json:"name"`
// 	Address string `json:"address"`
// }

// type UserRecord struct {
// 	FarmerID     string `json:"farmerID"`
// 	Name         string `json:"name"`
// 	Address      string `json:"address"`
// 	Mobile       string `json:"mobile"` // Primary key
// 	PasswordHash string `json:"passwordHash"`
// }

var (
	userDB      map[string]api.UserRecord
	userDBMutex sync.RWMutex
)

func main() {
	// --- 1. Initialize Firebase Service (STUB) ---
	// This is the stub we discussed. It needs a real serviceAccountKey.json
	// path to be fully functional. For now, it returns a mock service.
	fb, err := services.NewFirebaseService("./serviceAccountKey.json", "fake-bucket-name.appspot.com")
	if err != nil {
		log.Panicf("Failed to initialize Firebase service: %v", err)
	}

	//Loading mock_aadhar_db file
	jsonFile, err := os.Open("mock_aadhar_db.json")
	if err != nil {
		log.Fatalf("Failed to open mock_aadhar_db.json: %v", err)
	}
	defer jsonFile.Close()

	policyFile, err := os.Open("policies.json")
	if err != nil {
		log.Fatalf("Failed to open policies.json: %v", err)
	}
	defer policyFile.Close()

	policyByteValue, _ := io.ReadAll(policyFile)
	var policyDB map[string][]api.Policy 

	if err := json.Unmarshal(policyByteValue, &policyDB); err != nil {
		log.Fatalf("Failed to parse policies.json: %v", err)
	}
	log.Println("Insurance Policy DB loaded successfully.")

	cropCalendarFile, err := os.Open("crop_calendar.json")
	if err != nil {
		log.Fatalf("Failed to open crop_calendar.json: %v", err)
	}
	defer cropCalendarFile.Close()

	cropCalendarByteValue, _ := io.ReadAll(cropCalendarFile)
	// We are defining this struct right here, but it will be passed to the handler
	var cropCalendar map[string][]int 
	if err := json.Unmarshal(cropCalendarByteValue, &cropCalendar); err != nil {
		log.Fatalf("Failed to parse crop_calendar.json: %v", err)
	}
	log.Println("Crop Calendar DB loaded successfully.")

	byteValue, _ := io.ReadAll(jsonFile)
	var aadharDB map[string]models.AadharRecord
	if err := json.Unmarshal(byteValue, &aadharDB); err != nil {
		log.Fatalf("Failed to parse mock_aadhar_db.json: %v", err)
	}
	log.Println("Mock Aadhar DB loaded successfully.")

	userDB = make(map[string]api.UserRecord) 
	userFile, err := os.ReadFile("users_db.json")
	if err == nil && len(userFile) > 0 {
		userDBMutex.Lock()
		if err := json.Unmarshal(userFile, &userDB); err != nil {
			log.Printf("Warning: Could not parse users_db.json: %v. Starting with empty DB.", err)
			userDB = make(map[string]api.UserRecord) 
		} else {
			log.Printf("User DB loaded successfully. Found %d users.", len(userDB))
		}
		userDBMutex.Unlock()
	} else if !os.IsNotExist(err) {
		log.Printf("Warning: Could not read users_db.json: %v. Starting with empty DB.", err)
	} else {
		log.Println("users_db.json not found. Starting with empty DB.")
	}

	//In-memory store for OTPs
	otpStore := make(map[string]string)

	// --- 2. Initialize Fabric Service ---
	fs, err := services.InitializeFabric()
	if err != nil {
		log.Fatalf("Failed to initialize Fabric service: %v", err)
	}
	defer fs.Close()

	app := fiber.New()
	app.Use(logger.New())
	app.Use(cors.New())

	fabricHandler := api.NewFabricHandler(fs, fb, aadharDB, otpStore, userDB, &userDBMutex, saveUserDB, policyDB, cropCalendar,)

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, Farmer Insurance Backend!")
	})
	// --- Write (Submit) Routes ---
	app.Post("/request-otp", fabricHandler.RequestOTP)
	app.Post("/verify-otp-and-register", fabricHandler.VerifyOTPAndRegister)
	app.Post("/login", fabricHandler.Login)
	app.Post("/registerFarmer", fabricHandler.RegisterFarmer)
	app.Post("/addFarm", fabricHandler.AddFarm)
	app.Post("/verifyFarm/:farmID", fabricHandler.VerifyFarm)
	app.Post("/submitClaim", fabricHandler.SubmitClaim)
	app.Post("/updateClaimWithSatelliteData", fabricHandler.UpdateClaimWithSatelliteData)
	app.Post("/approveClaim", fabricHandler.ApproveClaim)
	app.Post("/rejectClaim", fabricHandler.RejectClaim)
	app.Post("/bank-details", fabricHandler.UpdateBankDetails)
	app.Post("/loginGov", fabricHandler.LoginGov)

	// --- Read (Evaluate) Routes ---

	// "Get by ID" routes (using your URL params)
	app.Get("/farmer/:farmerID", fabricHandler.GetFarmer)
	app.Get("/farm/:farmID", fabricHandler.GetFarm)
	app.Get("/claim/:claimID", fabricHandler.GetClaim)
	app.Get("/weather", fabricHandler.GetWeather)
	app.Get("/bank-details", fabricHandler.GetBankDetails)

	// --- "Rich Query" list routes ---
	app.Get("/farms/by-farmer/:farmerID", fabricHandler.GetFarmsByFarmer)
	app.Get("/farms/by-status/:status", fabricHandler.GetFarmsByStatus)
	app.Get("/claims/by-farmer/:farmerID", fabricHandler.GetClaimsByFarmer)
	app.Get("/claims/by-farm/:farmID", fabricHandler.GetClaimsByFarm)
	app.Get("/claims/by-status/:status", fabricHandler.GetClaimsByStatus)
	app.Get("/farm-details/:farmID", fabricHandler.GetFarmWithDetails)
	app.Get("/claims/summary-by-scheme", fabricHandler.GetClaimSummaryByScheme)
	app.Get("/claims/policy/:policyID/status/:status", fabricHandler.GetClaimsByPolicyAndStatus)
	app.Get("/claims/stats", fabricHandler.GetClaimsStats)

	app.Get("/policy", fabricHandler.GetPolicy)


	// --- Start Proactive Monitoring Cron Job ---
	go func() {
		// Run the check once immediately on startup
		log.Println("ProactiveMonitor: Running initial check...")
		fabricHandler.RunProactiveMonitoring()

		// For testing, set this to a shorter duration, like "time.Minute * 10"
		ticker := time.NewTicker(time.Hour * 24)
		defer ticker.Stop()

		for range ticker.C {
			log.Println("ProactiveMonitor: Running scheduled check...")
			fabricHandler.RunProactiveMonitoring()
		}
	}()
	// --- END ---

	fmt.Println("Starting Fiber server on :3000...")
	log.Fatal(app.Listen(":3000"))
}

func saveUserDB() {
	userDBMutex.RLock() 
	data, err := json.MarshalIndent(userDB, "", "  ")
	userDBMutex.RUnlock()

	if err != nil {
		log.Printf("Error marshalling user DB: %v", err)
		return
	}

	userDBMutex.Lock() 
	defer userDBMutex.Unlock()
	err = os.WriteFile("users_db.json", data, 0644)
	if err != nil {
		log.Printf("Error writing user DB to file: %v", err)
	} else {
		log.Println("User DB saved to users_db.json")
	}
}