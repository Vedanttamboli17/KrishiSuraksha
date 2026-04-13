#!/bin/bash

# This script deploys the chaincode to the running network
# It must be run from the 'blockchain' directory

# --- 1. Set Environment Variables ---
export CHANNEL_NAME="insurancechannel"
export CC_NAME="insurance"
export CC_SRC_PATH="./chaincode/insurance"
export CC_VERSION="1.0"
export CC_SEQUENCE="1"

# Import utils
. scripts/envVar.sh

# --- 2. Package the Chaincode ---
echo "--- Packaging Chaincode ${CC_NAME} version ${CC_VERSION} ---"

# Tidy and vendor modules locally
echo "--- Tidying and vendoring Go modules at ${CC_SRC_PATH} ---"
(cd ${CC_SRC_PATH} && go mod tidy && go mod vendor)
if [ $? -ne 0 ]; then
  echo "!!! FAILED to tidy/vendor Go modules !!!"
  exit 1
fi

# Unset the path that confuses the 'peer' command
# This is the same fix that made your createChannel work
unset FABRIC_CFG_PATH

# Package the chaincode (using host 'peer' command)
peer lifecycle chaincode package ${CC_NAME}.tar.gz \
  --path ${CC_SRC_PATH} \
  --lang golang \
  --label ${CC_NAME}_${CC_VERSION}

if [ $? -ne 0 ]; then
  echo "!!! FAILED to package chaincode !!!"
  exit 1
fi
echo "--- Chaincode packaged successfully ---"

# --- 3. Install Chaincode on All Peers ---
echo "--- Installing Chaincode on all peers ---"

for ORG in farmers insurance government dataprovider; do
  echo "--- Installing on ${ORG} ---"
  setGlobals ${ORG}

  # Run install command and capture output/error
  INSTALL_OUTPUT=$(peer lifecycle chaincode install ${CC_NAME}.tar.gz 2>&1)
  INSTALL_EC=$? # Get the exit code

  # Check if installation succeeded (exit code 0) OR if it failed because it was already installed
  if [ $INSTALL_EC -eq 0 ]; then
    echo "Install successful on ${ORG}."
    # Extract and print Package ID from successful install output if needed
    echo "$INSTALL_OUTPUT" | grep "Chaincode code package identifier:"
  elif [[ "$INSTALL_OUTPUT" == *"chaincode already successfully installed"* ]]; then
    echo "Chaincode already installed on ${ORG}. Continuing..."
  else
    # Any other error is fatal
    echo "!!! FAILED to install chaincode on ${ORG} !!!"
    echo "$INSTALL_OUTPUT" # Print the actual error
    exit 1
  fi
done

echo "--- Chaincode installation check complete for all peers ---"

# --- 4. Query Installed & Get Package ID ---
echo "--- Querying installed chaincode to get Package ID ---"
setGlobals farmers

# Parse the package ID from the query output
QUERY_RESULT=$(peer lifecycle chaincode queryinstalled)
PACKAGE_ID=$(echo "$QUERY_RESULT" | grep "Package ID: ${CC_NAME}_${CC_VERSION}" | sed 's/Package ID: //;s/, Label:.*//')

if [ -z "$PACKAGE_ID" ]; then
  echo "!!! FAILED to get Package ID. Exiting. !!!"
  echo "Query Result was:"
  echo "$QUERY_RESULT"
  exit 1
fi

echo "--- Got Package ID: ${PACKAGE_ID} ---"

# --- 5. Approve Chaincode for All Orgs ---
echo "--- Approving chaincode definition for all orgs ---"

for ORG in farmers insurance government dataprovider; do
  echo "--- Approving for ${ORG} ---"
  setGlobals ${ORG}
  
  peer lifecycle chaincode approveformyorg \
    -o orderer.example.com:7050 \
    --channelID ${CHANNEL_NAME} \
    --name ${CC_NAME} \
    --version ${CC_VERSION} \
    --package-id ${PACKAGE_ID} \
    --sequence ${CC_SEQUENCE} \
    --tls --cafile "$ORDERER_CA"
    
  if [ $? -ne 0 ]; then
    echo "!!! FAILED to approve chaincode for ${ORG} !!!"
    exit 1
  fi
done

echo "--- Chaincode approved successfully by all orgs ---"

# --- 6. Check Commit Readiness ---
echo "--- Checking if chaincode is ready to be committed ---"
setGlobals farmers
peer lifecycle chaincode checkcommitreadiness \
  --channelID ${CHANNEL_NAME} \
  --name ${CC_NAME} \
  --version ${CC_VERSION} \
  --sequence ${CC_SEQUENCE} \
  --tls --cafile "$ORDERER_CA" \
  --output json

# --- 7. Commit the Chaincode Definition ---
echo "--- Committing the chaincode definition to ${CHANNEL_NAME} ---"

# Set peer connection parameters for all 4 orgs
PEER_CONN_PARMS=""
PEER_CONN_PARMS="${PEER_CONN_PARMS} --peerAddresses localhost:7051 --tlsRootCertFiles ${PWD}/organizations/peerOrganizations/farmers.example.com/peers/peer0.farmers.example.com/tls/ca.crt"
PEER_CONN_PARMS="${PEER_CONN_PARMS} --peerAddresses localhost:8051 --tlsRootCertFiles ${PWD}/organizations/peerOrganizations/insurance.example.com/peers/peer0.insurance.example.com/tls/ca.crt"
PEER_CONN_PARMS="${PEER_CONN_PARMS} --peerAddresses localhost:10051 --tlsRootCertFiles ${PWD}/organizations/peerOrganizations/government.example.com/peers/peer0.government.example.com/tls/ca.crt"
PEER_CONN_PARMS="${PEER_CONN_PARMS} --peerAddresses localhost:11051 --tlsRootCertFiles ${PWD}/organizations/peerOrganizations/dataprovider.example.com/peers/peer0.dataprovider.example.com/tls/ca.crt"

setGlobals farmers # Use one org to submit the transaction

peer lifecycle chaincode commit \
  -o orderer.example.com:7050 \
  --channelID ${CHANNEL_NAME} \
  --name ${CC_NAME} \
  --version ${CC_VERSION} \
  --sequence ${CC_SEQUENCE} \
  --tls --cafile "$ORDERER_CA" \
  ${PEER_CONN_PARMS}

if [ $? -ne 0 ]; then
  echo "!!! FAILED to commit chaincode definition !!!"
  exit 1
fi

echo "--- Chaincode definition committed successfully ---"

# --- 8. Query Committed ---
echo "--- Querying committed chaincode definition ---"
setGlobals farmers
peer lifecycle chaincode querycommitted \
  --channelID ${CHANNEL_NAME} \
  --name ${CC_NAME}

echo "--- Chaincode deployment complete ---"