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
   STORAGE_ACCOUNT_NAME="yourstorageaccount"  # Must be globally unique, 3-24 characters, lowercase letters and numbers only
   LOCATION="eastus"  # Choose your preferred Azure region
   
   az storage account create \
     --name $STORAGE_ACCOUNT_NAME \
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
   az storage account show --name $STORAGE_ACCOUNT_NAME --resource-group $RESOURCE_GROUP
   ```

## Retrieving the Storage Account Access Key

The Bindplane ACA tool requires the storage account access key as a raw string value. Get the primary access key with this command:

```bash
az storage account keys list \
  --resource-group $RESOURCE_GROUP \
  --account-name $STORAGE_ACCOUNT_NAME \
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

Azure Container Apps must be able to reach Azure Files over SMB (TCP 445). Choose one:

### Option A: Public networking (quick start)

```bash
# Ensure the storage account allows public access
az storage account update \
  --name "$STORAGE_ACCOUNT" \
  --resource-group "$RESOURCE_GROUP" \
  --public-network-access Enabled

# If using selected networks, permit ACA outbound IPs
OUT_IPS=$(az containerapp env show \
  --name "$ENV_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --query "properties.outboundIpAddresses" -o tsv)

for ip in $OUT_IPS; do
  az storage account network-rule add \
    --resource-group "$RESOURCE_GROUP" \
    --account-name "$STORAGE_ACCOUNT" \
    --ip-address "$ip"
done
```

Note: Outbound IPs can change when the environment scales or updates.

### Option B: Private endpoint (recommended)

Use a VNet-injected ACA environment. Create a private endpoint for the File service and configure Private DNS:

```bash
VNET_RG="<vnet resource group>"
VNET_NAME="<vnet name>"
SUBNET_NAME="<subnet for private endpoints>"
PE_NAME="${STORAGE_ACCOUNT}-file-pe"
PE_RG="$RESOURCE_GROUP"

STG_ID=$(az storage account show -n "$STORAGE_ACCOUNT" -g "$RESOURCE_GROUP" --query id -o tsv)
az network private-endpoint create \
  --name "$PE_NAME" \
  --resource-group "$PE_RG" \
  --vnet-name "$VNET_NAME" \
  --subnet "$SUBNET_NAME" \
  --private-connection-resource-id "$STG_ID" \
  --group-ids file \
  --connection-name "${STORAGE_ACCOUNT}-file-conn"

az network private-dns zone create -g "$PE_RG" -n privatelink.file.core.windows.net || true
az network private-dns link vnet create \
  -g "$PE_RG" -n "${VNET_NAME}-file-link" \
  -z privatelink.file.core.windows.net \
  -v "$VNET_NAME" --registration-enabled false || true

NIC_ID=$(az network private-endpoint show -g "$PE_RG" -n "$PE_NAME" --query "networkInterfaces[0].id" -o tsv)
PE_IP=$(az network nic show --ids "$NIC_ID" --query "ipConfigurations[0].privateIpAddress" -o tsv)
az network private-dns record-set a create -g "$PE_RG" -z privatelink.file.core.windows.net -n "$STORAGE_ACCOUNT" || true
az network private-dns record-set a add-record -g "$PE_RG" -z privatelink.file.core.windows.net -n "$STORAGE_ACCOUNT" -a "$PE_IP" || true

# (Optional) Disable public network access afterwards
az storage account update \
  --name "$STORAGE_ACCOUNT" \
  --resource-group "$RESOURCE_GROUP" \
  --public-network-access Disabled
```

If your organization blocks outbound TCP 445, you must use this private endpoint approach.

## Storage Performance Considerations

For optimal Prometheus performance, create your storage account with these specifications:

1. **Performance tier**: **Standard** (required for Azure File shares)
2. **Redundancy**: **ZRS (Zone Redundant Storage)** - Recommended for production workloads to ensure high availability
3. **Access tier**: **Hot** - Prometheus requires frequent read/write access for time-series data

The deployment will provision approximately **120Gi** for Prometheus time-series data (2-day retention). NATS uses `EmptyDir` and does not require Azure Files.

Update your storage account creation command to use ZRS for better availability:

```bash
az storage account create \
  --name $STORAGE_ACCOUNT_NAME \
  --resource-group $RESOURCE_GROUP \
  --location $LOCATION \
  --sku Standard_ZRS \
  --kind StorageV2 \
  --access-tier Hot \
  --https-only true \
  --min-tls-version TLS1_2
```

## Troubleshooting Common Issues

### Storage Account Name Already Exists
Storage account names must be globally unique across all of Azure. If you get this error, try a different name.

### Insufficient Permissions
Ensure your account has the following permissions:
- `Storage Account Contributor` role
- `Storage Blob Data Contributor` role (if using blob storage)

### Access Key Not Working
- Verify you're using the correct key
- Check if the storage account has firewall rules that might be blocking access
- Ensure the key hasn't been regenerated

## Additional Resources

- [Azure Storage Account Documentation](https://docs.microsoft.com/en-us/azure/storage/common/storage-account-overview)
- [Azure Storage Security Guide](https://docs.microsoft.com/en-us/azure/storage/common/storage-security-guide)
- [Azure Storage Performance Guide](https://docs.microsoft.com/en-us/azure/storage/common/storage-performance-checklist)
