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

## Storage Performance Considerations

For optimal Prometheus performance, create your storage account with these specifications:

1. **Performance tier**: **Standard** (required for Azure File shares)
2. **Redundancy**: **ZRS (Zone Redundant Storage)** - Recommended for production workloads to ensure high availability
3. **Access tier**: **Hot** - Prometheus requires frequent read/write access for time-series data

The deployment will provision approximately **125Gi total storage**:
- **120Gi** for Prometheus time-series data (2-day retention)
- **5Gi** for NATS message persistence

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
