#!/bin/bash

# This is a collection of bash functions used by different scripts

export CORE_PEER_TLS_ENABLED=true
export ORDERER_CA=${PWD}/organizations/ordererOrganizations/example.com/msp/tlscacerts/tlsca.example.com-cert.pem
export ORDERER_ADDRESS=orderer.example.com:7050

# Set environment variables for the peer org
setGlobals() {
  local USING_ORG=$1
  echo "Using organization ${USING_ORG}"
  if [ "$USING_ORG" = "farmers" ]; then
    export CORE_PEER_LOCALMSPID="FarmersOrgMSP"
    export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/farmers.example.com/peers/peer0.farmers.example.com/tls/ca.crt
    export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/farmers.example.com/users/Admin@farmers.example.com/msp
    export CORE_PEER_ADDRESS=peer0.farmers.example.com:7051
  elif [ "$USING_ORG" = "insurance" ]; then
    export CORE_PEER_LOCALMSPID="InsuranceOrgMSP"
    export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/insurance.example.com/peers/peer0.insurance.example.com/tls/ca.crt
    export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/insurance.example.com/users/Admin@insurance.example.com/msp
    export CORE_PEER_ADDRESS=peer0.insurance.example.com:8051
  elif [ "$USING_ORG" = "government" ]; then
    export CORE_PEER_LOCALMSPID="GovernmentOrgMSP"
    export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/government.example.com/peers/peer0.government.example.com/tls/ca.crt
    export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/government.example.com/users/Admin@government.example.com/msp
    export CORE_PEER_ADDRESS=peer0.government.example.com:10051
  elif [ "$USING_ORG" = "dataprovider" ]; then
    export CORE_PEER_LOCALMSPID="DataProviderOrgMSP"
    export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/dataprovider.example.com/peers/peer0.dataprovider.example.com/tls/ca.crt
    export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/dataprovider.example.com/users/Admin@dataprovider.example.com/msp
    export CORE_PEER_ADDRESS=peer0.dataprovider.example.com:11051
  else
    echo "================== ORG UNKNOWN =================="
  fi
}

# Set environment variables for use in the CLI container
setGlobalsCLI() {
  setGlobals $1

  local USING_ORG=""
  if [ -z "$OVERRIDE_ORG" ]; then
    USING_ORG=$1
  else
    USING_ORG="${OVERRIDE_ORG}"
  fi
  if [ $USING_ORG = "farmers" ]; then
    export CORE_PEER_ADDRESS=peer0.farmers.example.com:7051
  elif [ $USING_ORG = "insurance" ]; then
    export CORE_PEER_ADDRESS=peer0.insurance.example.com:8051
  elif [ $USING_ORG = "government" ]; then
    export CORE_PEER_ADDRESS=peer0.government.example.com:10051
  elif [ $USING_ORG = "dataprovider" ]; then
    export CORE_PEER_ADDRESS=peer0.dataprovider.example.com:11051
  else
    echo "================== ORG UNKNOWN =================="
  fi
}

# Helper function to create the anchor peer update transaction
setAnchorPeer() {
  ORG=$1
  setGlobals $ORG
  
  # Get the peer host and port
  local HOST=""
  local PORT=""
  if [ "$ORG" = "farmers" ]; then
    HOST="peer0.farmers.example.com"
    PORT=7051
  elif [ "$ORG" = "insurance" ]; then
    HOST="peer0.insurance.example.com"
    PORT=8051
  elif [ "$ORG" = "government" ]; then
    HOST="peer0.government.example.com"
    PORT=10051
  elif [ "$ORG" = "dataprovider" ]; then
    HOST="peer0.dataprovider.example.com"
    PORT=11051
  else
    echo "================== ORG UNKNOWN for anchor peer =================="
    exit 1
  fi

  # Define file paths
  local CHANNEL_NAME="insurancechannel"
  local OUTPUT_DIR="organizations/channel-artifacts/"
  local CONFIG_BLOCK_FILE="${OUTPUT_DIR}/${CORE_PEER_LOCALMSPID}config_block.pb"
  local CONFIG_JSON_FILE="${OUTPUT_DIR}/${CORE_PEER_LOCALMSPID}config.json"
  local MODIFIED_CONFIG_JSON_FILE="${OUTPUT_DIR}/${CORE_PEER_LOCALMSPID}modified_config.json"
  local CONFIG_PROTO_FILE="${OUTPUT_DIR}/${CORE_PEER_LOCALMSPID}modified_config.pb"
  local UPDATE_IN_ENVELOPE_FILE="${OUTPUT_DIR}/${CORE_PEER_LOCALMSPID}anchors.tx"

  # 1. Fetch the latest configuration block
  peer channel fetch config $CONFIG_BLOCK_FILE -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com -c $CHANNEL_NAME --tls --cafile "$ORDERER_CA"
  
  # 2. Decode the block to JSON and extract the config
  configtxlator proto_decode --input $CONFIG_BLOCK_FILE --type common.Block | jq .data.data[0].payload.data.config > $CONFIG_JSON_FILE

  # 3. Add the anchor peer definition using jq
  # Note: The jq path is shorter now because we are modifying the config directly
  jq '.channel_group.groups.Application.groups.'${CORE_PEER_LOCALMSPID}'.values += {"AnchorPeers":{"mod_policy": "Admins","value":{"anchor_peers": [{"host": "'$HOST'","port": '$PORT'}]},"version": "0"}}' $CONFIG_JSON_FILE > $MODIFIED_CONFIG_JSON_FILE
  if [ $? -ne 0 ]; then
    echo "Failed to add anchor peer with jq!"
    exit 1
  fi

  # 4. Re-encode the original and modified config JSON to protobuf
  configtxlator proto_encode --input $CONFIG_JSON_FILE --type common.Config --output ${OUTPUT_DIR}/original_config.pb
  configtxlator proto_encode --input $MODIFIED_CONFIG_JSON_FILE --type common.Config --output $CONFIG_PROTO_FILE
  if [ $? -ne 0 ]; then
    echo "Failed to encode config protobufs!"
    exit 1
  fi

  # 5. Calculate the update delta
  configtxlator compute_update --channel_id $CHANNEL_NAME --original ${OUTPUT_DIR}/original_config.pb --updated $CONFIG_PROTO_FILE --output ${OUTPUT_DIR}/config_update.pb

  # 6. Decode the update delta to JSON and wrap it in an envelope
  configtxlator proto_decode --input ${OUTPUT_DIR}/config_update.pb --type common.ConfigUpdate | jq . > ${OUTPUT_DIR}/config_update.json
  echo '{"payload":{"header":{"channel_header":{"channel_id":"'$CHANNEL_NAME'", "type":2}},"data":{"config_update":'$(cat ${OUTPUT_DIR}/config_update.json)'}}}' | jq . > ${OUTPUT_DIR}/config_update_in_envelope.json
  configtxlator proto_encode --input ${OUTPUT_DIR}/config_update_in_envelope.json --type common.Envelope --output $UPDATE_IN_ENVELOPE_FILE

  # 7. Sign and submit the update
  peer channel update -f $UPDATE_IN_ENVELOPE_FILE -c $CHANNEL_NAME -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com --tls --cafile "$ORDERER_CA"
  echo "Anchor peer set for ${ORG}"
}

#!/bin/bash

# This script is to be sourced *inside* the CLI container

# export CORE_PEER_TLS_ENABLED=true
# export ORDERER_CA=/opt/gopath/src/github.com/hyperledger/fabric/peer/organizations/ordererOrganizations/example.com/msp/tlscacerts/tlsca.example.com-cert.pem

# # Set environment variables for the peer org
# setGlobals() {
#   local USING_ORG=$1
#   echo "Using organization ${USING_ORG}"

#   # Set the FABRIC_CFG_PATH to the default peer config inside the container
#   # This replaces the need for our broken core.yaml file
#   export FABRIC_CFG_PATH=/etc/hyperledger/fabric

#   if [ "$USING_ORG" = "farmers" ]; then
#     export CORE_PEER_LOCALMSPID="FarmersOrgMSP"
#     export CORE_PEER_TLS_ROOTCERT_FILE=/opt/gopath/src/github.com/hyperledger/fabric/peer/organizations/peerOrganizations/farmers.example.com/peers/peer0.farmers.example.com/tls/ca.crt
#     export CORE_PEER_MSPCONFIGPATH=/opt/gopath/src/github.com/hyperledger/fabric/peer/organizations/peerOrganizations/farmers.example.com/users/Admin@farmers.example.com/msp
#     # Crucial change: Use the container's network name, not localhost
#     export CORE_PEER_ADDRESS=peer0.farmers.example.com:7051

#   elif [ "$USING_ORG" = "insurance" ]; then
#     export CORE_PEER_LOCALMSPID="InsuranceOrgMSP"
#     export CORE_PEER_TLS_ROOTCERT_FILE=/opt/gopath/src/github.com/hyperledger/fabric/peer/organizations/peerOrganizations/insurance.example.com/peers/peer0.insurance.example.com/tls/ca.crt
#     export CORE_PEER_MSPCONFIGPATH=/opt/gopath/src/github.com/hyperledger/fabric/peer/organizations/peerOrganizations/insurance.example.com/users/Admin@insurance.example.com/msp
#     export CORE_PEER_ADDRESS=peer0.insurance.example.com:8051

#   elif [ "$USING_ORG" = "government" ]; then
#     export CORE_PEER_LOCALMSPID="GovernmentOrgMSP"
#     export CORE_PEER_TLS_ROOTCERT_FILE=/opt/gopath/src/github.com/hyperledger/fabric/peer/organizations/peerOrganizations/government.example.com/peers/peer0.government.example.com/tls/ca.crt
#     export CORE_PEER_MSPCONFIGPATH=/opt/gopath/src/github.com/hyperledger/fabric/peer/organizations/peerOrganizations/government.example.com/users/Admin@government.example.com/msp
#     export CORE_PEER_ADDRESS=peer0.government.example.com:10051

#   elif [ "$USING_ORG" = "dataprovider" ]; then
#     export CORE_PEER_LOCALMSPID="DataProviderOrgMSP"
#     export CORE_PEER_TLS_ROOTCERT_FILE=/opt/gopath/src/github.com/hyperledger/fabric/peer/organizations/peerOrganizations/dataprovider.example.com/peers/peer0.dataprovider.example.com/tls/ca.crt
#     export CORE_PEER_MSPCONFIGPATH=/opt/gopath/src/github.com/hyperledger/fabric/peer/organizations/peerOrganizations/dataprovider.example.com/users/Admin@dataprovider.example.com/msp
#     export CORE_PEER_ADDRESS=peer0.dataprovider.example.com:11051
#   else
#     echo "================== ORG UNKNOWN =================="
#   fi
# }