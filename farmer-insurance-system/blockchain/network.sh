#!/bin/bash

# Load fabric-samples/bin into PATH
export PATH=${PWD}/../../../fabric-samples/bin:$PATH
export FABRIC_CFG_PATH=${PWD}/network/

# Import utils
. scripts/envVar.sh

# Function to bring down the network
function networkDown() {
  echo "----------- Tearing down the network ------------"
  docker-compose -f network/docker-compose-test-net.yaml -f network/compose-ca.yaml down --volumes --remove-orphans
  
  # remove organizations and system-genesis-block
  rm -rf organizations system-genesis-block
  echo "----------- Network torn down successfully ------------"
}

# Function to bring up the network
function networkUp() {
  # Check if the required images are present
  # If not, docker pull them
  for image in fabric-ca fabric-peer fabric-orderer fabric-tools; do
    if [[ "$(docker images -q hyperledger/$image:latest 2> /dev/null)" == "" ]]; then
      echo "----------- Pulling hyperledger/$image:latest ------------"
      docker pull hyperledger/$image:latest
    fi
  done

  # 1. Start Certificate Authorities
  echo "----------- Starting Certificate Authorities ------------"
  docker-compose -f network/compose-ca.yaml up -d
  sleep 5 # Give CAs some time to start

# Change ownership of the directories created by docker
  sudo chown -R $(id -u):$(id -g) organizations/

  echo "----------- Copying CA TLS certs to well-known location ------------"
  mkdir -p organizations/fabric-ca/farmers/
  cp organizations/fabric-ca/farmers/ca-cert.pem organizations/fabric-ca/farmers/tls-cert.pem

  mkdir -p organizations/fabric-ca/insurance/
  cp organizations/fabric-ca/insurance/ca-cert.pem organizations/fabric-ca/insurance/tls-cert.pem

  mkdir -p organizations/fabric-ca/government/
  cp organizations/fabric-ca/government/ca-cert.pem organizations/fabric-ca/government/tls-cert.pem

  mkdir -p organizations/fabric-ca/dataprovider/
  cp organizations/fabric-ca/dataprovider/ca-cert.pem organizations/fabric-ca/dataprovider/tls-cert.pem
  
  mkdir -p organizations/fabric-ca/ordererOrg/
  cp organizations/fabric-ca/ordererOrg/ca-cert.pem organizations/fabric-ca/ordererOrg/tls-cert.pem

  # 2. Create crypto material for organizations
  echo "----------- Creating Organization Identities ------------"
  . scripts/registerEnroll.sh

  createFarmersOrg
  createInsuranceOrg
  createGovernmentOrg
  createDataProviderOrg
  createOrdererOrg

  echo "--- Copying FarmersOrg Admin identity to backend ---"
  # Remove old identity if it exists
  rm -rf ../backend/identities/farmersAdmin
  # Recreate directory structure
  mkdir -p ../backend/identities/farmersAdmin
  # Copy the newly generated MSP
  cp -r organizations/peerOrganizations/farmers.example.com/users/Admin@farmers.example.com/msp ../backend/identities/farmersAdmin/
  echo "--- Backend identity copied ---"

  # 3. Generate Genesis Block for the ordering service
  echo "----------- Generating Genesis Block ------------"
  configtxgen -profile FourOrgsOrdererGenesis -channelID system-channel -outputBlock ./system-genesis-block/genesis.block
  if [ $? -ne 0 ]; then
    echo "Failed to generate genesis block!"
    exit 1
  fi

  # 4. Start the network nodes (peers and orderer)
  echo "----------- Starting Peers and Orderer ------------"
  docker-compose -f network/docker-compose-test-net.yaml up -d

  echo "----------- Network is up and running. Now creating channel. ------------"
}

# Function to create the application channel
function createChannel() {
  CHANNEL_NAME="insurancechannel" # You can change this name if you want

  sudo chown -R $(id -u):$(id -g) organizations/
  mkdir -p organizations/channel-artifacts/
  
  # Create channel genesis block
  echo "----------- Creating Channel Genesis Block for '$CHANNEL_NAME' ------------"
  configtxgen -profile FourOrgsChannel -outputCreateChannelTx ./organizations/channel-artifacts/${CHANNEL_NAME}.tx -channelID $CHANNEL_NAME
  if [ $? -ne 0 ]; then
    echo "Failed to generate channel configuration transaction!"
    exit 1
  fi

  # -----------------------------------------------------------------
  # THIS IS THE FIX: Unset the config path
  unset FABRIC_CFG_PATH
  # -----------------------------------------------------------------

  # Create the channel
  echo "----------- Creating Channel '$CHANNEL_NAME' ------------"
  setGlobals farmers # Use FarmersOrg peer to create the channel
  peer channel create -o orderer.example.com:7050 -c $CHANNEL_NAME --ordererTLSHostnameOverride orderer.example.com -f ./organizations/channel-artifacts/${CHANNEL_NAME}.tx --outputBlock ./organizations/channel-artifacts/${CHANNEL_NAME}.block --tls --cafile "$ORDERER_CA"
  
  # Join all peers to the channel
  echo "----------- Joining all peers to '$CHANNEL_NAME' ------------"
  setGlobals farmers
  peer channel join -b ./organizations/channel-artifacts/${CHANNEL_NAME}.block
  
  setGlobals insurance
  peer channel join -b ./organizations/channel-artifacts/${CHANNEL_NAME}.block

  setGlobals government
  peer channel join -b ./organizations/channel-artifacts/${CHANNEL_NAME}.block

  setGlobals dataprovider
  peer channel join -b ./organizations/channel-artifacts/${CHANNEL_NAME}.block

  # Set anchor peers for each org
  echo "----------- Setting Anchor Peers ------------"
  setAnchorPeer farmers
  setAnchorPeer insurance
  setAnchorPeer government
  setAnchorPeer dataprovider

  echo "----------- Channel '$CHANNEL_NAME' created and joined successfully! ------------"
}

# Parse command-line arguments
if [ "$1" == "up" ]; then
  networkUp
elif [ "$1" == "down" ]; then
  networkDown
elif [ "$1" == "createChannel" ]; then
  createChannel
else
  echo "Usage: ./network.sh [up|down|createChannel]"
  exit 1
fi

# function createChannel() {
#   CHANNEL_NAME="insurancechannel" # You can change this name if you want

#   # Create channel genesis block
#   echo "----------- Creating Channel Genesis Block for '$CHANNEL_NAME' ------------"
#   configtxgen -profile FourOrgsChannel -outputCreateChannelTx ./organizations/channel-artifacts/${CHANNEL_NAME}.tx -channelID $CHANNEL_NAME
#   if [ $? -ne 0 ]; then
#     echo "Failed to generate channel configuration transaction!"
#     exit 1
#   fi

#   # -----------------------------------------------------------------
#   # Unset the config path - let's keep this fix
#   unset FABRIC_CFG_PATH
#   # -----------------------------------------------------------------


#   # Create the channel
#   echo "----------- Creating Channel '$CHANNEL_NAME' ------------"
  
#   echo "--- DEBUG: CALLING SETGLOBALS ---"
#   setGlobals farmers # Use FarmersOrg peer to create the channel
#   echo "--- DEBUG: SETGLOBALS FINISHED ---"
  
#   echo ""
#   echo "--- DEBUG: CHECKING ENVIRONMENT ---"
#   echo "Which peer: $(which peer)"
#   echo "CORE_PEER_LOCALMSPID: $CORE_PEER_LOCALMSPID"
#   echo "CORE_PEER_ADDRESS: $CORE_PEER_ADDRESS"
#   echo "CORE_PEER_TLS_ROOTCERT_FILE: $CORE_PEER_TLS_ROOTCERT_FILE"
#   echo "CORE_PEER_MSPCONFIGPATH: $CORE_PEER_MSPCONFIGPATH"
#   echo "ORDERER_CA: $ORDERER_CA"
#   echo "--- DEBUG: CHECKING FILES ---"
#   echo "Listing MSP Keystore:"
#   ls -l $CORE_PEER_MSPCONFIGPATH/keystore/
#   echo "Listing TLS Root Cert:"
#   ls -l $CORE_PEER_TLS_ROOTCERT_FILE
#   echo "Listing Orderer CA Cert:"
#   ls -l $ORDERER_CA
#   echo "--- DEBUG: END OF CHECKS ---"
#   echo ""


#   peer channel create -o localhost:7050 -c $CHANNEL_NAME --ordererTLSHostnameOverride orderer.example.com -f ./organizations/channel-artifacts/${CHANNEL_NAME}.tx --outputBlock ./organizations/channel-artifacts/${CHANNEL_NAME}.block --tls --cafile "$ORDERER_CA"
  
#   # Join all peers to the channel
#   echo "----------- Joining all peers to '$CHANNEL_NAME' ------------"
#   setGlobals farmers
#   peer channel join -b ./organizations/channel-artifacts/${CHANNEL_NAME}.block
  
#   setGlobals insurance
#   peer channel join -b ./organizations/channel-artifacts/${CHANNEL_NAME}.block

#   setGlobals government
#   peer channel join -b ./organizations/channel-artifacts/${CHANNEL_NAME}.block

#   setGlobals dataprovider
#   peer channel join -b ./organizations/channel-artifacts/${CHANNEL_NAME}.block

#   # Set anchor peers for each org
#   echo "----------- Setting Anchor Peers ------------"
#   setAnchorPeer farmers
#   setAnchorPeer insurance
#   setAnchorPeer government
#   setAnchorPeer dataprovider

#   echo "----------- Channel '$CHANNEL_NAME' created and joined successfully! ------------"
# }