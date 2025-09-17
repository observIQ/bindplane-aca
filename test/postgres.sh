#!/bin/bash

set -e

# Script to deploy PostgreSQL to Azure
# Usage: ./postgres.sh <password>
# Requires LOCATION and RESOURCE_GROUP environment variables

# Check if password argument is provided
if [ $# -eq 0 ]; then
    echo "Error: Password argument is required"
    echo "Usage: $0 <password>"
    exit 1
fi

PASSWORD="$1"

# Check if required environment variables are set
if [ -z "$LOCATION" ]; then
    echo "Error: LOCATION environment variable is not set"
    exit 1
fi

if [ -z "$RESOURCE_GROUP" ]; then
    echo "Error: RESOURCE_GROUP environment variable is not set"
    exit 1
fi

# PostgreSQL configuration
SERVER_NAME="bindplane-postgres-$(date +%s)"
ADMIN_USERNAME="bindplane"
SKU_NAME="GP_Gen5_2"
STORAGE_SIZE="128"
VERSION="13"

echo "Deploying PostgreSQL to Azure..."
echo "Location: $LOCATION"
echo "Resource Group: $RESOURCE_GROUP"
echo "Server Name: $SERVER_NAME"
echo "Admin Username: $ADMIN_USERNAME"

# Create PostgreSQL server
echo "Creating PostgreSQL server..."
az postgres server create \
    --resource-group "$RESOURCE_GROUP" \
    --name "$SERVER_NAME" \
    --location "$LOCATION" \
    --admin-user "$ADMIN_USERNAME" \
    --admin-password "$PASSWORD" \
    --sku-name "$SKU_NAME" \
    --storage-size "${STORAGE_SIZE}GB" \
    --version "$VERSION" \
    --public-network-access Enabled

# Configure firewall rule to allow access from any IP
echo "Configuring firewall rule for public access..."
az postgres server firewall-rule create \
    --resource-group "$RESOURCE_GROUP" \
    --server "$SERVER_NAME" \
    --name "AllowAllIPs" \
    --start-ip-address "0.0.0.0" \
    --end-ip-address "255.255.255.255"

# Enable SSL enforcement (optional but recommended)
echo "Configuring SSL enforcement..."
az postgres server configuration set \
    --resource-group "$RESOURCE_GROUP" \
    --server "$SERVER_NAME" \
    --name "require_secure_transport" \
    --value "on"

# Get server details
echo "Getting server details..."
FQDN=$(az postgres server show \
    --resource-group "$RESOURCE_GROUP" \
    --name "$SERVER_NAME" \
    --query "fullyQualifiedDomainName" \
    --output tsv)

echo ""
echo "PostgreSQL deployment completed successfully!"
echo "=============================================="
echo "Server Name: $SERVER_NAME"
echo "FQDN: $FQDN"
echo "Admin Username: $ADMIN_USERNAME"
echo "Location: $LOCATION"
echo "Resource Group: $RESOURCE_GROUP"
echo ""
echo "Connection string example:"
echo "postgresql://$ADMIN_USERNAME:$PASSWORD@$FQDN:5432/postgres?sslmode=require"
echo ""
echo "To connect using psql:"
echo "psql \"host=$FQDN port=5432 dbname=postgres user=$ADMIN_USERNAME password=$PASSWORD sslmode=require\""