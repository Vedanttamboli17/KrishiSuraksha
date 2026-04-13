package services

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/gateway"
)

// FabricService holds the gateway connection
type FabricService struct {
	Gateway *gateway.Gateway
	Network *gateway.Network
	Contract *gateway.Contract
}

// InitializeFabric sets up the connection to the Fabric network
func InitializeFabric() (*FabricService, error) {
	fmt.Println("Initializing Fabric connection...")

	// --- 1. Set up wallet ---
	// The wallet stores identities used to connect to the network.
	walletPath := "wallet" // Will be created in the backend directory
	wallet, err := gateway.NewFileSystemWallet(walletPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create wallet: %w", err)
	}

	// --- 2. Check if admin identity exists in wallet ---
	// We need to import the admin identity from the copied MSP files into the SDK's wallet format.
	identityLabel := "farmersAdmin"
	if !wallet.Exists(identityLabel) {
		fmt.Printf("Identity %s not found in wallet, attempting to import...\n", identityLabel)
		err = populateWallet(wallet, identityLabel)
		if err != nil {
			return nil, fmt.Errorf("failed to import admin identity into wallet: %w", err)
		}
		fmt.Printf("Successfully imported identity %s into wallet\n", identityLabel)
	} else {
		fmt.Printf("Identity %s already exists in wallet\n", identityLabel)
	}


	// --- 3. Set path to connection profile ---
	// Assumes the config file is in a 'config' subdirectory relative to where the app runs
	ccpPath := filepath.Join(".", "config", "connection-farmers.yaml") // Relative path

	// --- 4. Connect to the gateway ---
	// Uses the connection profile and the identity in the wallet to connect.
	gw, err := gateway.Connect(
		gateway.WithConfig(config.FromFile(filepath.Clean(ccpPath))),
		gateway.WithIdentity(wallet, identityLabel),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gateway: %w", err)
	}
	fmt.Println("Connected to Fabric gateway.")


	// --- 5. Get network and contract handles ---
	// Connect to the specific channel your chaincode is on.
	network, err := gw.GetNetwork("insurancechannel") // Your channel name
	if err != nil {
		gw.Close() // Clean up gateway connection on error
		return nil, fmt.Errorf("failed to get network: %w", err)
	}
	fmt.Println("Got network handle for 'insurancechannel'.")

	// Get a handle to the specific chaincode.
	contract := network.GetContract("insurance") // Your chaincode name
	fmt.Println("Got contract handle for 'insurance'.")


	// --- 6. Return the service struct ---
	// We store the gateway, network, and contract handles for later use.
	// The Gateway should be closed when the application shuts down.
	service := &FabricService{
		Gateway: gw,
		Network: network,
		Contract: contract,
	}

	fmt.Println("Fabric connection initialized successfully.")
	return service, nil
}

// populateWallet imports the MSP identity into the wallet
func populateWallet(wallet *gateway.Wallet, identityLabel string) error {
	// Assumes identity files are in identities/farmersAdmin/msp relative to app execution
	credPath := filepath.Join(".", "identities", "farmersAdmin", "msp")

	certPath := filepath.Join(credPath, "signcerts", "cert.pem") // The admin's public cert
	// check if cert path exists
	_, err := os.Stat(certPath)
	if err != nil {
		return fmt.Errorf("certificate file not found at %s: %w", certPath, err)
	}

	keyDir := filepath.Join(credPath, "keystore") // Directory containing the private key
	// The private key file has a name ending in "_sk"
	files, err := os.ReadDir(keyDir)
	if err != nil {
		return fmt.Errorf("failed to read keystore directory %s: %w", keyDir, err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no key files found in %s", keyDir)
	}
	// Assuming only one key file in the directory
	keyPath := filepath.Join(keyDir, files[0].Name())


	// Read the identity files
	cert, err := os.ReadFile(filepath.Clean(certPath))
	if err != nil {
		return fmt.Errorf("failed to read certificate file: %w", err)
	}

	key, err := os.ReadFile(filepath.Clean(keyPath))
	if err != nil {
		return fmt.Errorf("failed to read key file: %w", err)
	}

	// Create the identity object
	identity := gateway.NewX509Identity("FarmersOrgMSP", string(cert), string(key)) // Use your Org's MSP ID

	// Import the identity into the wallet
	err = wallet.Put(identityLabel, identity)
	if err != nil {
		return fmt.Errorf("failed to put identity into wallet: %w", err)
	}
	return nil
}

// Close terminates the gateway connection
func (fs *FabricService) Close() {
	if fs.Gateway != nil {
		fmt.Println("Closing Fabric Gateway connection...")
		fs.Gateway.Close()
	}
}