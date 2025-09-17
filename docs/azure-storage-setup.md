# Azure Storage Account Setup for Bindplane ACA

This document provides detailed instructions for setting up an Azure Storage Account required for Bindplane deployment on Azure Container Apps.

## Why Azure Storage Account is Required

Bindplane on Azure Container Apps requires persistent storage for:

- **Configuration data**: Storing Bindplane configuration files and settings
- **Logs and metrics**: Temporary storage for log files and metric data before processing
- **Agent packages**: Storing custom agent packages and configurations
- **Backup data**: Storing database backups and recovery files
- **Shared volumes**: Providing shared storage between different Bindplane components

The Azure Storage Account provides reliable, scalable, and highly available storage that can be mounted as persistent volumes in your Azure Container Apps.

## Prerequisites

Before creating an Azure Storage Account, ensure you have:

1. **Azure CLI installed** - Follow the [official installation guide](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli)
2. **Azure subscription** - An active Azure subscription with appropriate permissions
3. **Resource Group** - An existing resource group where the storage account will be created

## Creating an Azure Storage Account

1. **Login to Azure CLI**:
   ```bash
   az login
   ```

2. **Set your subscription** (if you have multiple):
   ```bash
   az account set --subscription "your-subscription-id"
   ```

3. **Create the storage account**:
   ```bash
   # Replace these variables with your actual values
   RESOURCE_GROUP="your-resource-group"
   STORAGE_ACCOUNT="yourstorageaccount"  # Must be globally unique, 3-24 characters, lowercase letters and numbers only
   LOCATION="eastus"  # Choose your preferred Azure region
   
   az storage account create \
     --name $STORAGE_ACCOUNT \
     --resource-group $RESOURCE_GROUP \
     --location $LOCATION \
     --sku Standard_ZRS \
     --kind StorageV2 \
     --access-tier Hot \
     --https-only true \
     --min-tls-version TLS1_2
   ```

4. **Verify the storage account was created**:
   ```bash
   az storage account show --name $STORAGE_ACCOUNT --resource-group $RESOURCE_GROUP
   ```

## Retrieving the Storage Account Access Key

The Bindplane ACA tool requires the storage account access key as a raw string value. Get the primary access key with this command:

```bash
az storage account keys list \
  --resource-group $RESOURCE_GROUP \
  --account-name $STORAGE_ACCOUNT \
  --query '[0].value' \
  --output tsv
```

This will output just the key value that you can use directly with the `-storage-account-key` parameter. The tool will automatically handle base64 encoding when generating the deployment templates.

## Verify Azure File share and attach environment storage

1) Create the Azure File share used by Prometheus (name must match templates):

```bash
STORAGE_ACCOUNT="<your storage account>"
STORAGE_KEY="<your storage key>"

az storage share create --account-name "$STORAGE_ACCOUNT" --account-key "$STORAGE_KEY" --name prometheus-data
```

2) Attach that share to the Container Apps environment as environment storage (name must be `prometheus-pv`):

```bash
ENV_NAME="<your ACA environment name>"
RESOURCE_GROUP="<your resource group>"

az containerapp env storage set \
  --name "$ENV_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --storage-name prometheus-pv \
  --azure-file-account-name "$STORAGE_ACCOUNT" \
  --azure-file-account-key "$STORAGE_KEY" \
  --azure-file-share-name prometheus-data \
  --access-mode ReadWrite

# Verify
az storage share show --name prometheus-data --account-name "$STORAGE_ACCOUNT" --account-key "$STORAGE_KEY" -o table
az containerapp env storage list --name "$ENV_NAME" --resource-group "$RESOURCE_GROUP" -o table
```

## Network security for Azure Files

Azure Container Apps must be able to reach Azure Files over SMB (TCP 445). This deployment requires a private endpoint for secure connectivity.

### Private endpoint configuration

This deployment requires a VNet-injected Container Apps environment. First, determine your Container Apps environment's VNet details, then create a private endpoint for the File service.

#### Step 1: Get your Container Apps environment VNet information

```bash
# Get the VNet details from your existing Container Apps environment
ENV_NAME="<your ACA environment name>"
RESOURCE_GROUP="<your resource group>"

# Get VNet information from the Container Apps environment
VNET_ID=$(az containerapp env show \
  --name "$ENV_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --query "properties.vnetConfiguration.infrastructureSubnetId" \
  --output tsv)

# Extract VNet resource group and name from the subnet ID
VNET_RG=$(echo "$VNET_ID" | cut -d'/' -f5)
VNET_NAME=$(echo "$VNET_ID" | cut -d'/' -f9)
SUBNET_NAME=$(echo "$VNET_ID" | cut -d'/' -f11)

echo "VNet Resource Group: $VNET_RG"
echo "VNet Name: $VNET_NAME" 
echo "Subnet Name: $SUBNET_NAME"
```

#### Step 2: Create a dedicated subnet for private endpoints

```bash
# Create a new subnet specifically for private endpoints (if it doesn't exist)
PE_SUBNET_NAME="private-endpoints"
PE_SUBNET_CIDR="10.0.2.0/24"  # Adjust based on your VNet CIDR

# Check if subnet already exists
if ! az network vnet subnet show \
  --resource-group "$VNET_RG" \
  --vnet-name "$VNET_NAME" \
  --name "$PE_SUBNET_NAME" &>/dev/null; then
  
  echo "Creating private endpoint subnet..."
  az network vnet subnet create \
    --resource-group "$VNET_RG" \
    --vnet-name "$VNET_NAME" \
    --name "$PE_SUBNET_NAME" \
    --address-prefix "$PE_SUBNET_CIDR" \
    --disable-private-endpoint-network-policies false
else
  echo "Private endpoint subnet already exists"
fi
```

#### Step 3: Create the private endpoint for Azure Files

```bash
# Set variables for private endpoint creation
PE_NAME="${STORAGE_ACCOUNT}-file-pe"
PE_RG="$RESOURCE_GROUP"
LOCATION="eastus"  # Set your Azure region

# Get storage account resource ID
STG_ID=$(az storage account show -n "$STORAGE_ACCOUNT" -g "$RESOURCE_GROUP" --query id -o tsv)

# Create the private endpoint
# When VNet and private endpoint are in different resource groups, use the full subnet resource ID
SUBNET_RESOURCE_ID="/subscriptions/$(az account show --query id -o tsv)/resourceGroups/$VNET_RG/providers/Microsoft.Network/virtualNetworks/$VNET_NAME/subnets/$PE_SUBNET_NAME"

az network private-endpoint create \
  --name "$PE_NAME" \
  --location "$LOCATION" \
  --resource-group "$PE_RG" \
  --subnet "$SUBNET_RESOURCE_ID" \
  --private-connection-resource-id "$STG_ID" \
  --group-ids file \
  --connection-name "${STORAGE_ACCOUNT}-file-conn"
```

#### Step 4: Configure Private DNS

```bash
# Create private DNS zone for Azure Files
az network private-dns zone create \
  -g "$PE_RG" \
  -n privatelink.file.core.windows.net

# Link the DNS zone to your VNet
az network private-dns link vnet create \
  -g "$PE_RG" \
  -n "${VNET_NAME}-file-link" \
  -z privatelink.file.core.windows.net \
  -v "$VNET_NAME" \
  --registration-enabled false

# Get the private endpoint IP and create DNS record
# First, verify the private endpoint was created successfully
echo "Verifying private endpoint creation..."
az network private-endpoint show \
  -g "$PE_RG" \
  -n "$PE_NAME" \
  --query "provisioningState" -o tsv

# Get the network interface ID (the query path may vary)
NIC_ID=$(az network private-endpoint show \
  -g "$PE_RG" \
  -n "$PE_NAME" \
  --query "networkInterfaces[0].id" -o tsv)

# Extract NIC name and resource group from the NIC ID
NIC_RG=$(echo "$NIC_ID" | cut -d'/' -f5)
NIC_NAME=$(echo "$NIC_ID" | cut -d'/' -f9)

# Get the private IP address using resource group and name (more reliable than --ids)
PE_IP=$(az network nic show \
  --resource-group "$NIC_RG" \
  --name "$NIC_NAME" \
  --query "ipConfigurations[0].privateIPAddress" -o tsv)

# Create DNS A record
az network private-dns record-set a create \
  -g "$PE_RG" \
  -z privatelink.file.core.windows.net \
  -n "$STORAGE_ACCOUNT"

az network private-dns record-set a add-record \
  -g "$PE_RG" \
  -z privatelink.file.core.windows.net \
  -n "$STORAGE_ACCOUNT" \
  -a "$PE_IP"

echo "Private endpoint IP: $PE_IP"
echo "DNS record created for: $STORAGE_ACCOUNT.privatelink.file.core.windows.net"
```

#### Step 5: Disable public network access

```bash
# Disable public network access for security
az storage account update \
  --name "$STORAGE_ACCOUNT" \
  --resource-group "$RESOURCE_GROUP" \
  --public-network-access Disabled
```

This private endpoint approach ensures secure connectivity and is required for production deployments.

## Additional Resources

- [Azure Storage Account Documentation](https://docs.microsoft.com/en-us/azure/storage/common/storage-account-overview)
- [Azure Storage Security Guide](https://docs.microsoft.com/en-us/azure/storage/common/storage-security-guide)
- [Azure Storage Performance Guide](https://docs.microsoft.com/en-us/azure/storage/common/storage-performance-checklist)
