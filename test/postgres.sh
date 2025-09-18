#!/bin/bash

set -e

# Script to deploy PostgreSQL (Flexible Server) to Azure
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
TIER="Burstable"
SKU_NAME="Standard_B2s"
STORAGE_SIZE="128"
VERSION="16"

echo "Deploying PostgreSQL Flexible Server to Azure..."
echo "Location: $LOCATION"
echo "Resource Group: $RESOURCE_GROUP"
echo "Server Name: $SERVER_NAME"
echo "Admin Username: $ADMIN_USERNAME"
echo "Selected PostgreSQL Version: $VERSION"

# Create PostgreSQL Flexible Server
echo "Creating PostgreSQL Flexible Server..."
az postgres flexible-server create \
    --resource-group "$RESOURCE_GROUP" \
    --name "$SERVER_NAME" \
    --location "$LOCATION" \
    --admin-user "$ADMIN_USERNAME" \
    --admin-password "$PASSWORD" \
    --tier "$TIER" \
    --sku-name "$SKU_NAME" \
    --storage-size "$STORAGE_SIZE" \
    --version "$VERSION" \
    --public-access all

# Get server details
echo "Getting server details..."
FQDN=$(az postgres flexible-server show \
    --resource-group "$RESOURCE_GROUP" \
    --name "$SERVER_NAME" \
    --query "fullyQualifiedDomainName" \
    --output tsv)

echo ""
echo "PostgreSQL Flexible Server deployment completed successfully!"
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
