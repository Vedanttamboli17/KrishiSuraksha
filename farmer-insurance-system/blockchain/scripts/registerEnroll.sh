#!/bin/bash

# Function to create crypto material for the Farmers Organization
function createFarmersOrg {
  echo "Enrolling the CA admin for FarmersOrg"
  mkdir -p ${PWD}/../blockchain/organizations/peerOrganizations/farmers.example.com/

  export FABRIC_CA_CLIENT_HOME=${PWD}/../blockchain/organizations/peerOrganizations/farmers.example.com/

  fabric-ca-client enroll -u https://admin:adminpw@localhost:7054 --caname ca-farmers --tls.certfiles "${PWD}/organizations/fabric-ca/farmers/tls-cert.pem"

  echo 'NodeOUs:
  Enable: true
  ClientOUIdentifier:
    Certificate: cacerts/localhost-7054-ca-farmers.pem
    OrganizationalUnitIdentifier: client
  PeerOUIdentifier:
    Certificate: cacerts/localhost-7054-ca-farmers.pem
    OrganizationalUnitIdentifier: peer
  AdminOUIdentifier:
    Certificate: cacerts/localhost-7054-ca-farmers.pem
    OrganizationalUnitIdentifier: admin
  OrdererOUIdentifier:
    Certificate: cacerts/localhost-7054-ca-farmers.pem
    OrganizationalUnitIdentifier: orderer' > "${PWD}/organizations/peerOrganizations/farmers.example.com/msp/config.yaml"

  echo "Registering peer0 for FarmersOrg"
  fabric-ca-client register --caname ca-farmers --id.name peer0 --id.secret peer0pw --id.type peer --tls.certfiles "${PWD}/organizations/fabric-ca/farmers/tls-cert.pem"

  echo "Registering user for FarmersOrg"
  fabric-ca-client register --caname ca-farmers --id.name user1 --id.secret user1pw --id.type client --tls.certfiles "${PWD}/organizations/fabric-ca/farmers/tls-cert.pem"

  echo "Registering the org admin for FarmersOrg"
  fabric-ca-client register --caname ca-farmers --id.name farmersadmin --id.secret farmersadminpw --id.type admin --tls.certfiles "${PWD}/organizations/fabric-ca/farmers/tls-cert.pem"

  echo "Generating the peer0 msp"
  fabric-ca-client enroll -u https://peer0:peer0pw@localhost:7054 --caname ca-farmers -M "${PWD}/organizations/peerOrganizations/farmers.example.com/peers/peer0.farmers.example.com/msp" --csr.hosts peer0.farmers.example.com --tls.certfiles "${PWD}/organizations/fabric-ca/farmers/tls-cert.pem"
  cp "${PWD}/organizations/peerOrganizations/farmers.example.com/msp/config.yaml" "${PWD}/organizations/peerOrganizations/farmers.example.com/peers/peer0.farmers.example.com/msp/config.yaml"

  echo "Generating the peer0-tls certificates"
  fabric-ca-client enroll -u https://peer0:peer0pw@localhost:7054 --caname ca-farmers -M "${PWD}/organizations/peerOrganizations/farmers.example.com/peers/peer0.farmers.example.com/tls" --enrollment.profile tls --csr.hosts peer0.farmers.example.com --csr.hosts localhost --tls.certfiles "${PWD}/organizations/fabric-ca/farmers/tls-cert.pem"
  
  cp "${PWD}/organizations/peerOrganizations/farmers.example.com/peers/peer0.farmers.example.com/tls/tlscacerts/"* "${PWD}/organizations/peerOrganizations/farmers.example.com/peers/peer0.farmers.example.com/tls/ca.crt"
  cp "${PWD}/organizations/peerOrganizations/farmers.example.com/peers/peer0.farmers.example.com/tls/signcerts/"* "${PWD}/organizations/peerOrganizations/farmers.example.com/peers/peer0.farmers.example.com/tls/server.crt"
  cp "${PWD}/organizations/peerOrganizations/farmers.example.com/peers/peer0.farmers.example.com/tls/keystore/"* "${PWD}/organizations/peerOrganizations/farmers.example.com/peers/peer0.farmers.example.com/tls/server.key"

  mkdir -p "${PWD}/organizations/peerOrganizations/farmers.example.com/msp/tlscacerts"
  cp "${PWD}/organizations/peerOrganizations/farmers.example.com/peers/peer0.farmers.example.com/tls/tlscacerts/"* "${PWD}/organizations/peerOrganizations/farmers.example.com/msp/tlscacerts/ca.crt"

  mkdir -p "${PWD}/organizations/peerOrganizations/farmers.example.com/tlsca"
  cp "${PWD}/organizations/peerOrganizations/farmers.example.com/peers/peer0.farmers.example.com/tls/tlscacerts/"* "${PWD}/organizations/peerOrganizations/farmers.example.com/tlsca/tlsca.farmers.example.com-cert.pem"

  mkdir -p "${PWD}/organizations/peerOrganizations/farmers.example.com/ca"
  cp "${PWD}/organizations/peerOrganizations/farmers.example.com/msp/cacerts/"* "${PWD}/organizations/peerOrganizations/farmers.example.com/ca/ca.farmers.example.com-cert.pem"

  echo "Generating the user msp"
  fabric-ca-client enroll -u https://user1:user1pw@localhost:7054 --caname ca-farmers -M "${PWD}/organizations/peerOrganizations/farmers.example.com/users/User1@farmers.example.com/msp" --tls.certfiles "${PWD}/organizations/fabric-ca/farmers/tls-cert.pem"
  cp "${PWD}/organizations/peerOrganizations/farmers.example.com/msp/config.yaml" "${PWD}/organizations/peerOrganizations/farmers.example.com/users/User1@farmers.example.com/msp/config.yaml"

  echo "Generating the org admin msp"
  fabric-ca-client enroll -u https://farmersadmin:farmersadminpw@localhost:7054 --caname ca-farmers -M "${PWD}/organizations/peerOrganizations/farmers.example.com/users/Admin@farmers.example.com/msp" --tls.certfiles "${PWD}/organizations/fabric-ca/farmers/tls-cert.pem"
  cp "${PWD}/organizations/peerOrganizations/farmers.example.com/msp/config.yaml" "${PWD}/organizations/peerOrganizations/farmers.example.com/users/Admin@farmers.example.com/msp/config.yaml"
}

# Function to create crypto material for the Insurance Organization
function createInsuranceOrg {
  echo "Enrolling the CA admin for InsuranceOrg"
  mkdir -p ${PWD}/../blockchain/organizations/peerOrganizations/insurance.example.com/

  export FABRIC_CA_CLIENT_HOME=${PWD}/../blockchain/organizations/peerOrganizations/insurance.example.com/

  fabric-ca-client enroll -u https://admin:adminpw@localhost:8054 --caname ca-insurance --tls.certfiles "${PWD}/organizations/fabric-ca/insurance/tls-cert.pem"

  echo 'NodeOUs:
  Enable: true
  ClientOUIdentifier:
    Certificate: cacerts/localhost-8054-ca-insurance.pem
    OrganizationalUnitIdentifier: client
  PeerOUIdentifier:
    Certificate: cacerts/localhost-8054-ca-insurance.pem
    OrganizationalUnitIdentifier: peer
  AdminOUIdentifier:
    Certificate: cacerts/localhost-8054-ca-insurance.pem
    OrganizationalUnitIdentifier: admin
  OrdererOUIdentifier:
    Certificate: cacerts/localhost-8054-ca-insurance.pem
    OrganizationalUnitIdentifier: orderer' > "${PWD}/organizations/peerOrganizations/insurance.example.com/msp/config.yaml"

  echo "Registering peer0 for InsuranceOrg"
  fabric-ca-client register --caname ca-insurance --id.name peer0 --id.secret peer0pw --id.type peer --tls.certfiles "${PWD}/organizations/fabric-ca/insurance/tls-cert.pem"
  
  echo "Registering user for InsuranceOrg"
  fabric-ca-client register --caname ca-insurance --id.name user1 --id.secret user1pw --id.type client --tls.certfiles "${PWD}/organizations/fabric-ca/insurance/tls-cert.pem"
  
  echo "Registering the org admin for InsuranceOrg"
  fabric-ca-client register --caname ca-insurance --id.name insuranceadmin --id.secret insuranceadminpw --id.type admin --tls.certfiles "${PWD}/organizations/fabric-ca/insurance/tls-cert.pem"
  
  echo "Generating the peer0 msp"
  fabric-ca-client enroll -u https://peer0:peer0pw@localhost:8054 --caname ca-insurance -M "${PWD}/organizations/peerOrganizations/insurance.example.com/peers/peer0.insurance.example.com/msp" --csr.hosts peer0.insurance.example.com --tls.certfiles "${PWD}/organizations/fabric-ca/insurance/tls-cert.pem"
  cp "${PWD}/organizations/peerOrganizations/insurance.example.com/msp/config.yaml" "${PWD}/organizations/peerOrganizations/insurance.example.com/peers/peer0.insurance.example.com/msp/config.yaml"

  echo "Generating the peer0-tls certificates"
  fabric-ca-client enroll -u https://peer0:peer0pw@localhost:8054 --caname ca-insurance -M "${PWD}/organizations/peerOrganizations/insurance.example.com/peers/peer0.insurance.example.com/tls" --enrollment.profile tls --csr.hosts peer0.insurance.example.com --csr.hosts localhost --tls.certfiles "${PWD}/organizations/fabric-ca/insurance/tls-cert.pem"
  
  cp "${PWD}/organizations/peerOrganizations/insurance.example.com/peers/peer0.insurance.example.com/tls/tlscacerts/"* "${PWD}/organizations/peerOrganizations/insurance.example.com/peers/peer0.insurance.example.com/tls/ca.crt"
  cp "${PWD}/organizations/peerOrganizations/insurance.example.com/peers/peer0.insurance.example.com/tls/signcerts/"* "${PWD}/organizations/peerOrganizations/insurance.example.com/peers/peer0.insurance.example.com/tls/server.crt"
  cp "${PWD}/organizations/peerOrganizations/insurance.example.com/peers/peer0.insurance.example.com/tls/keystore/"* "${PWD}/organizations/peerOrganizations/insurance.example.com/peers/peer0.insurance.example.com/tls/server.key"

  mkdir -p "${PWD}/organizations/peerOrganizations/insurance.example.com/msp/tlscacerts"
  cp "${PWD}/organizations/peerOrganizations/insurance.example.com/peers/peer0.insurance.example.com/tls/tlscacerts/"* "${PWD}/organizations/peerOrganizations/insurance.example.com/msp/tlscacerts/ca.crt"

  mkdir -p "${PWD}/organizations/peerOrganizations/insurance.example.com/tlsca"
  cp "${PWD}/organizations/peerOrganizations/insurance.example.com/peers/peer0.insurance.example.com/tls/tlscacerts/"* "${PWD}/organizations/peerOrganizations/insurance.example.com/tlsca/tlsca.insurance.example.com-cert.pem"

  mkdir -p "${PWD}/organizations/peerOrganizations/insurance.example.com/ca"
  cp "${PWD}/organizations/peerOrganizations/insurance.example.com/msp/cacerts/"* "${PWD}/organizations/peerOrganizations/insurance.example.com/ca/ca.insurance.example.com-cert.pem"

  echo "Generating the user msp"
  fabric-ca-client enroll -u https://user1:user1pw@localhost:8054 --caname ca-insurance -M "${PWD}/organizations/peerOrganizations/insurance.example.com/users/User1@insurance.example.com/msp" --tls.certfiles "${PWD}/organizations/fabric-ca/insurance/tls-cert.pem"
  cp "${PWD}/organizations/peerOrganizations/insurance.example.com/msp/config.yaml" "${PWD}/organizations/peerOrganizations/insurance.example.com/users/User1@insurance.example.com/msp/config.yaml"

  echo "Generating the org admin msp"
  fabric-ca-client enroll -u https://insuranceadmin:insuranceadminpw@localhost:8054 --caname ca-insurance -M "${PWD}/organizations/peerOrganizations/insurance.example.com/users/Admin@insurance.example.com/msp" --tls.certfiles "${PWD}/organizations/fabric-ca/insurance/tls-cert.pem"
  cp "${PWD}/organizations/peerOrganizations/insurance.example.com/msp/config.yaml" "${PWD}/organizations/peerOrganizations/insurance.example.com/users/Admin@insurance.example.com/msp/config.yaml"
}

# Function to create crypto material for the Government Organization
function createGovernmentOrg {
  echo "Enrolling the CA admin for GovernmentOrg"
  mkdir -p ${PWD}/../blockchain/organizations/peerOrganizations/government.example.com/

  export FABRIC_CA_CLIENT_HOME=${PWD}/../blockchain/organizations/peerOrganizations/government.example.com/

  fabric-ca-client enroll -u https://admin:adminpw@localhost:10054 --caname ca-government --tls.certfiles "${PWD}/organizations/fabric-ca/government/tls-cert.pem"

  echo 'NodeOUs:
  Enable: true
  ClientOUIdentifier:
    Certificate: cacerts/localhost-10054-ca-government.pem
    OrganizationalUnitIdentifier: client
  PeerOUIdentifier:
    Certificate: cacerts/localhost-10054-ca-government.pem
    OrganizationalUnitIdentifier: peer
  AdminOUIdentifier:
    Certificate: cacerts/localhost-10054-ca-government.pem
    OrganizationalUnitIdentifier: admin
  OrdererOUIdentifier:
    Certificate: cacerts/localhost-10054-ca-government.pem
    OrganizationalUnitIdentifier: orderer' > "${PWD}/organizations/peerOrganizations/government.example.com/msp/config.yaml"

  echo "Registering peer0 for GovernmentOrg"
  fabric-ca-client register --caname ca-government --id.name peer0 --id.secret peer0pw --id.type peer --tls.certfiles "${PWD}/organizations/fabric-ca/government/tls-cert.pem"
  
  echo "Registering user for GovernmentOrg"
  fabric-ca-client register --caname ca-government --id.name user1 --id.secret user1pw --id.type client --tls.certfiles "${PWD}/organizations/fabric-ca/government/tls-cert.pem"
  
  echo "Registering the org admin for GovernmentOrg"
  fabric-ca-client register --caname ca-government --id.name governmentadmin --id.secret governmentadminpw --id.type admin --tls.certfiles "${PWD}/organizations/fabric-ca/government/tls-cert.pem"
  
  echo "Generating the peer0 msp"
  fabric-ca-client enroll -u https://peer0:peer0pw@localhost:10054 --caname ca-government -M "${PWD}/organizations/peerOrganizations/government.example.com/peers/peer0.government.example.com/msp" --csr.hosts peer0.government.example.com --tls.certfiles "${PWD}/organizations/fabric-ca/government/tls-cert.pem"
  cp "${PWD}/organizations/peerOrganizations/government.example.com/msp/config.yaml" "${PWD}/organizations/peerOrganizations/government.example.com/peers/peer0.government.example.com/msp/config.yaml"

  echo "Generating the peer0-tls certificates"
  fabric-ca-client enroll -u https://peer0:peer0pw@localhost:10054 --caname ca-government -M "${PWD}/organizations/peerOrganizations/government.example.com/peers/peer0.government.example.com/tls" --enrollment.profile tls --csr.hosts peer0.government.example.com --csr.hosts localhost --tls.certfiles "${PWD}/organizations/fabric-ca/government/tls-cert.pem"
  
  cp "${PWD}/organizations/peerOrganizations/government.example.com/peers/peer0.government.example.com/tls/tlscacerts/"* "${PWD}/organizations/peerOrganizations/government.example.com/peers/peer0.government.example.com/tls/ca.crt"
  cp "${PWD}/organizations/peerOrganizations/government.example.com/peers/peer0.government.example.com/tls/signcerts/"* "${PWD}/organizations/peerOrganizations/government.example.com/peers/peer0.government.example.com/tls/server.crt"
  cp "${PWD}/organizations/peerOrganizations/government.example.com/peers/peer0.government.example.com/tls/keystore/"* "${PWD}/organizations/peerOrganizations/government.example.com/peers/peer0.government.example.com/tls/server.key"

  mkdir -p "${PWD}/organizations/peerOrganizations/government.example.com/msp/tlscacerts"
  cp "${PWD}/organizations/peerOrganizations/government.example.com/peers/peer0.government.example.com/tls/tlscacerts/"* "${PWD}/organizations/peerOrganizations/government.example.com/msp/tlscacerts/ca.crt"

  mkdir -p "${PWD}/organizations/peerOrganizations/government.example.com/tlsca"
  cp "${PWD}/organizations/peerOrganizations/government.example.com/peers/peer0.government.example.com/tls/tlscacerts/"* "${PWD}/organizations/peerOrganizations/government.example.com/tlsca/tlsca.government.example.com-cert.pem"

  mkdir -p "${PWD}/organizations/peerOrganizations/government.example.com/ca"
  cp "${PWD}/organizations/peerOrganizations/government.example.com/msp/cacerts/"* "${PWD}/organizations/peerOrganizations/government.example.com/ca/ca.government.example.com-cert.pem"

  echo "Generating the user msp"
  fabric-ca-client enroll -u https://user1:user1pw@localhost:10054 --caname ca-government -M "${PWD}/organizations/peerOrganizations/government.example.com/users/User1@government.example.com/msp" --tls.certfiles "${PWD}/organizations/fabric-ca/government/tls-cert.pem"
  cp "${PWD}/organizations/peerOrganizations/government.example.com/msp/config.yaml" "${PWD}/organizations/peerOrganizations/government.example.com/users/User1@government.example.com/msp/config.yaml"

  echo "Generating the org admin msp"
  fabric-ca-client enroll -u https://governmentadmin:governmentadminpw@localhost:10054 --caname ca-government -M "${PWD}/organizations/peerOrganizations/government.example.com/users/Admin@government.example.com/msp" --tls.certfiles "${PWD}/organizations/fabric-ca/government/tls-cert.pem"
  cp "${PWD}/organizations/peerOrganizations/government.example.com/msp/config.yaml" "${PWD}/organizations/peerOrganizations/government.example.com/users/Admin@government.example.com/msp/config.yaml"
}

# Function to create crypto material for the DataProvider Organization
function createDataProviderOrg {
  echo "Enrolling the CA admin for DataProviderOrg"
  mkdir -p ${PWD}/../blockchain/organizations/peerOrganizations/dataprovider.example.com/

  export FABRIC_CA_CLIENT_HOME=${PWD}/../blockchain/organizations/peerOrganizations/dataprovider.example.com/

  fabric-ca-client enroll -u https://admin:adminpw@localhost:11054 --caname ca-dataprovider --tls.certfiles "${PWD}/organizations/fabric-ca/dataprovider/tls-cert.pem"

  echo 'NodeOUs:
  Enable: true
  ClientOUIdentifier:
    Certificate: cacerts/localhost-11054-ca-dataprovider.pem
    OrganizationalUnitIdentifier: client
  PeerOUIdentifier:
    Certificate: cacerts/localhost-11054-ca-dataprovider.pem
    OrganizationalUnitIdentifier: peer
  AdminOUIdentifier:
    Certificate: cacerts/localhost-11054-ca-dataprovider.pem
    OrganizationalUnitIdentifier: admin
  OrdererOUIdentifier:
    Certificate: cacerts/localhost-11054-ca-dataprovider.pem
    OrganizationalUnitIdentifier: orderer' > "${PWD}/organizations/peerOrganizations/dataprovider.example.com/msp/config.yaml"

  echo "Registering peer0 for DataProviderOrg"
  fabric-ca-client register --caname ca-dataprovider --id.name peer0 --id.secret peer0pw --id.type peer --tls.certfiles "${PWD}/organizations/fabric-ca/dataprovider/tls-cert.pem"
  
  echo "Registering user for DataProviderOrg"
  fabric-ca-client register --caname ca-dataprovider --id.name user1 --id.secret user1pw --id.type client --tls.certfiles "${PWD}/organizations/fabric-ca/dataprovider/tls-cert.pem"
  
  echo "Registering the org admin for DataProviderOrg"
  fabric-ca-client register --caname ca-dataprovider --id.name dataprovideradmin --id.secret dataprovideradminpw --id.type admin --tls.certfiles "${PWD}/organizations/fabric-ca/dataprovider/tls-cert.pem"
  
  echo "Generating the peer0 msp"
  fabric-ca-client enroll -u https://peer0:peer0pw@localhost:11054 --caname ca-dataprovider -M "${PWD}/organizations/peerOrganizations/dataprovider.example.com/peers/peer0.dataprovider.example.com/msp" --csr.hosts peer0.dataprovider.example.com --tls.certfiles "${PWD}/organizations/fabric-ca/dataprovider/tls-cert.pem"
  cp "${PWD}/organizations/peerOrganizations/dataprovider.example.com/msp/config.yaml" "${PWD}/organizations/peerOrganizations/dataprovider.example.com/peers/peer0.dataprovider.example.com/msp/config.yaml"

  echo "Generating the peer0-tls certificates"
  fabric-ca-client enroll -u https://peer0:peer0pw@localhost:11054 --caname ca-dataprovider -M "${PWD}/organizations/peerOrganizations/dataprovider.example.com/peers/peer0.dataprovider.example.com/tls" --enrollment.profile tls --csr.hosts peer0.dataprovider.example.com --csr.hosts localhost --tls.certfiles "${PWD}/organizations/fabric-ca/dataprovider/tls-cert.pem"
  
  cp "${PWD}/organizations/peerOrganizations/dataprovider.example.com/peers/peer0.dataprovider.example.com/tls/tlscacerts/"* "${PWD}/organizations/peerOrganizations/dataprovider.example.com/peers/peer0.dataprovider.example.com/tls/ca.crt"
  cp "${PWD}/organizations/peerOrganizations/dataprovider.example.com/peers/peer0.dataprovider.example.com/tls/signcerts/"* "${PWD}/organizations/peerOrganizations/dataprovider.example.com/peers/peer0.dataprovider.example.com/tls/server.crt"
  cp "${PWD}/organizations/peerOrganizations/dataprovider.example.com/peers/peer0.dataprovider.example.com/tls/keystore/"* "${PWD}/organizations/peerOrganizations/dataprovider.example.com/peers/peer0.dataprovider.example.com/tls/server.key"

  mkdir -p "${PWD}/organizations/peerOrganizations/dataprovider.example.com/msp/tlscacerts"
  cp "${PWD}/organizations/peerOrganizations/dataprovider.example.com/peers/peer0.dataprovider.example.com/tls/tlscacerts/"* "${PWD}/organizations/peerOrganizations/dataprovider.example.com/msp/tlscacerts/ca.crt"

  mkdir -p "${PWD}/organizations/peerOrganizations/dataprovider.example.com/tlsca"
  cp "${PWD}/organizations/peerOrganizations/dataprovider.example.com/peers/peer0.dataprovider.example.com/tls/tlscacerts/"* "${PWD}/organizations/peerOrganizations/dataprovider.example.com/tlsca/tlsca.dataprovider.example.com-cert.pem"

  mkdir -p "${PWD}/organizations/peerOrganizations/dataprovider.example.com/ca"
  cp "${PWD}/organizations/peerOrganizations/dataprovider.example.com/msp/cacerts/"* "${PWD}/organizations/peerOrganizations/dataprovider.example.com/ca/ca.dataprovider.example.com-cert.pem"

  echo "Generating the user msp"
  fabric-ca-client enroll -u https://user1:user1pw@localhost:11054 --caname ca-dataprovider -M "${PWD}/organizations/peerOrganizations/dataprovider.example.com/users/User1@dataprovider.example.com/msp" --tls.certfiles "${PWD}/organizations/fabric-ca/dataprovider/tls-cert.pem"
  cp "${PWD}/organizations/peerOrganizations/dataprovider.example.com/msp/config.yaml" "${PWD}/organizations/peerOrganizations/dataprovider.example.com/users/User1@dataprovider.example.com/msp/config.yaml"

  echo "Generating the org admin msp"
  fabric-ca-client enroll -u https://dataprovideradmin:dataprovideradminpw@localhost:11054 --caname ca-dataprovider -M "${PWD}/organizations/peerOrganizations/dataprovider.example.com/users/Admin@dataprovider.example.com/msp" --tls.certfiles "${PWD}/organizations/fabric-ca/dataprovider/tls-cert.pem"
  cp "${PWD}/organizations/peerOrganizations/dataprovider.example.com/msp/config.yaml" "${PWD}/organizations/peerOrganizations/dataprovider.example.com/users/Admin@dataprovider.example.com/msp/config.yaml"
}

# Function to create crypto material for the Orderer Organization
function createOrdererOrg {
  echo "Enrolling the CA admin for OrdererOrg"
  mkdir -p ${PWD}/../blockchain/organizations/ordererOrganizations/example.com

  export FABRIC_CA_CLIENT_HOME=${PWD}/../blockchain/organizations/ordererOrganizations/example.com

  fabric-ca-client enroll -u https://admin:adminpw@localhost:9054 --caname ca-orderer --tls.certfiles "${PWD}/organizations/fabric-ca/ordererOrg/tls-cert.pem"

  echo 'NodeOUs:
  Enable: true
  ClientOUIdentifier:
    Certificate: cacerts/localhost-9054-ca-orderer.pem
    OrganizationalUnitIdentifier: client
  PeerOUIdentifier:
    Certificate: cacerts/localhost-9054-ca-orderer.pem
    OrganizationalUnitIdentifier: peer
  AdminOUIdentifier:
    Certificate: cacerts/localhost-9054-ca-orderer.pem
    OrganizationalUnitIdentifier: admin
  OrdererOUIdentifier:
    Certificate: cacerts/localhost-9054-ca-orderer.pem
    OrganizationalUnitIdentifier: orderer' > "${PWD}/organizations/ordererOrganizations/example.com/msp/config.yaml"

  echo "Registering orderer"
  fabric-ca-client register --caname ca-orderer --id.name orderer --id.secret ordererpw --id.type orderer --tls.certfiles "${PWD}/organizations/fabric-ca/ordererOrg/tls-cert.pem"
  
  echo "Registering the orderer admin"
  fabric-ca-client register --caname ca-orderer --id.name ordererAdmin --id.secret ordererAdminpw --id.type admin --tls.certfiles "${PWD}/organizations/fabric-ca/ordererOrg/tls-cert.pem"

  echo "Generating the orderer msp"
  fabric-ca-client enroll -u https://orderer:ordererpw@localhost:9054 --caname ca-orderer -M "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp" --csr.hosts orderer.example.com --csr.hosts localhost --tls.certfiles "${PWD}/organizations/fabric-ca/ordererOrg/tls-cert.pem"
  cp "${PWD}/organizations/ordererOrganizations/example.com/msp/config.yaml" "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/config.yaml"

  echo "Generating the orderer-tls certificates"
  fabric-ca-client enroll -u https://orderer:ordererpw@localhost:9054 --caname ca-orderer -M "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/tls" --enrollment.profile tls --csr.hosts orderer.example.com --csr.hosts localhost --tls.certfiles "${PWD}/organizations/fabric-ca/ordererOrg/tls-cert.pem"

  cp "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/tls/tlscacerts/"* "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/tls/ca.crt"
  cp "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/tls/signcerts/"* "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/tls/server.crt"
  cp "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/tls/keystore/"* "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/tls/server.key"

  mkdir -p "${PWD}/organizations/ordererOrganizations/example.com/msp/tlscacerts"
  cp "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/tls/tlscacerts/"* "${PWD}/organizations/ordererOrganizations/example.com/msp/tlscacerts/tlsca.example.com-cert.pem"
  
  mkdir -p "${PWD}/organizations/ordererOrganizations/example.com/tlsca"
  cp "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/tls/tlscacerts/"* "${PWD}/organizations/ordererOrganizations/example.com/tlsca/tlsca.example.com-cert.pem"


  echo "Generating the admin msp"
  fabric-ca-client enroll -u https://ordererAdmin:ordererAdminpw@localhost:9054 --caname ca-orderer -M "${PWD}/organizations/ordererOrganizations/example.com/users/Admin@example.com/msp" --tls.certfiles "${PWD}/organizations/fabric-ca/ordererOrg/tls-cert.pem"
  cp "${PWD}/organizations/ordererOrganizations/example.com/msp/config.yaml" "${PWD}/organizations/ordererOrganizations/example.com/users/Admin@example.com/msp/config.yaml"
}