# Bindplane Azure Container Apps Deployment

A Go-based templating tool for generating Bindplane deployment files for Azure Container Apps (ACA). This tool processes YAML templates and generates deployment-ready files with user-supplied configuration values.

## Prerequisites

Before deploying Bindplane to Azure Container Apps, ensure you have:

- Azure CLI installed and configured
- Appropriate Azure permissions to create resources
- A PostgreSQL database (Azure Database for PostgreSQL recommended)
- An Azure Storage Account for persistent volumes

## Deployment Order

This guide follows a logical deployment order where all prerequisites are set up before creating the Container Apps Environment:

1. **Authentication & Networking Setup** - Create VNet and subnets
2. **Azure Service Bus Setup** - Create namespace and topic
3. **Container Apps Environment** - Create environment with managed identity support
4. **Storage Configuration** - Set up Azure Files and private endpoints
5. **Bindplane Deployment** - Generate and deploy Container Apps

## Step 1: Authentication & Networking Setup

### Create Resource Group and VNet

For production deployments, it's recommended to use a VNet-injected Container Apps environment for better security and network isolation.

```bash
# Set your variables
RESOURCE_GROUP="your-resource-group"
LOCATION="eastus"
VNET_NAME="bindplane-vnet"
SUBNET_NAME="bindplane-subnet"
ENV_NAME="bindplane-env"

# Create resource group
az group create --name "$RESOURCE_GROUP" --location "$LOCATION"

# Create VNet
az network vnet create \
  --resource-group "$RESOURCE_GROUP" \
  --name "$VNET_NAME" \
  --address-prefix 10.0.0.0/16 \
  --location "$LOCATION"

# Create subnet for Container Apps
az network vnet subnet create \
  --resource-group "$RESOURCE_GROUP" \
  --vnet-name "$VNET_NAME" \
  --name "$SUBNET_NAME" \
  --address-prefix 10.0.1.0/24 \
  --delegations Microsoft.App/environments

# Create subnet for private endpoints (for storage)
az network vnet subnet create \
  --resource-group "$RESOURCE_GROUP" \
  --vnet-name "$VNET_NAME" \
  --name "private-endpoints-subnet" \
  --address-prefix 10.0.2.0/24
```

**Important Notes:**
- The Container Apps subnet must have at least `/24` CIDR (256 IPs)
- The private endpoints subnet is used for Azure Storage private endpoints
- Both subnets are created now but will be used in later steps

## Step 2: Azure Service Bus Setup

Bindplane uses Azure Service Bus for event messaging between components. You need to create a Service Bus namespace and topic before deploying Bindplane.

### 2.1 Create Azure Service Bus namespace

```bash
# Set your variables
SERVICE_BUS_NAMESPACE="bindplane-sb-namespace"

# Create Service Bus namespace
az servicebus namespace create \
  --resource-group "$RESOURCE_GROUP" \
  --name "$SERVICE_BUS_NAMESPACE" \
  --location "$LOCATION" \
  --sku Standard
```

### 2.2 Create Service Bus topic

```bash
# Set topic name
TOPIC_NAME="bindplane-events"

# Create Service Bus topic
az servicebus topic create \
  --resource-group "$RESOURCE_GROUP" \
  --namespace-name "$SERVICE_BUS_NAMESPACE" \
  --name "$TOPIC_NAME"
```

### 2.3 Get connection string and subscription ID

```bash
# Get the connection string for the Service Bus namespace
SERVICE_BUS_CONNECTION=$(az servicebus namespace authorization-rule keys list \
  --resource-group "$RESOURCE_GROUP" \
  --namespace-name "$SERVICE_BUS_NAMESPACE" \
  --name RootManageSharedAccessKey \
  --query primaryConnectionString \
  --output tsv)

# Get your Azure subscription ID
SUBSCRIPTION_ID=$(az account show --query id --output tsv)

echo "Service Bus Connection String: $SERVICE_BUS_CONNECTION"
echo "Subscription ID: $SUBSCRIPTION_ID"
```

### 2.4 Grant Service Bus permissions to the Container Apps environment identity

We'll use a system-assigned managed identity on the Container Apps environment. After creating the environment and assigning the identity in Step 3, grant it Service Bus permissions:

```bash
# Get the Service Bus namespace resource ID
SERVICE_BUS_ID="/subscriptions/$SUBSCRIPTION_ID/resourceGroups/$RESOURCE_GROUP/providers/Microsoft.ServiceBus/namespaces/$SERVICE_BUS_NAMESPACE"

# Get the environment principal ID (after identity is assigned in Step 3)
ENV_PRINCIPAL_ID=$(az containerapp env show \
  --name "$ENV_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --query "identity.principalId" \
  --output tsv)

# Assign Service Bus Data Owner role to the environment identity
az role assignment create \
  --assignee "$ENV_PRINCIPAL_ID" \
  --role "Azure Service Bus Data Owner" \
  --scope "$SERVICE_BUS_ID"
```

**Important Notes:**
- The Container Apps environment should be created with managed identity enabled for automatic authentication
- Bindplane will automatically create its own subscriptions when connecting to the topic
- The Service Bus namespace must be in the same subscription as your Container Apps environment
- Use the Standard SKU for Service Bus to support topics (Basic SKU only supports queues)
- The managed identity needs "Azure Service Bus Data Owner" role to create subscriptions and manage topics
- **Critical**: The Container Apps Environment must be created with managed identity support enabled

## Step 3: Container Apps Environment Setup

Now create the Container Apps Environment and assign a system-assigned managed identity:

```bash
# Create Container Apps environment with VNet integration and managed identity
az containerapp env create \
  --name "$ENV_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --location "$LOCATION" \
  --infrastructure-subnet-resource-id "/subscriptions/$SUBSCRIPTION_ID/resourceGroups/$RESOURCE_GROUP/providers/Microsoft.Network/virtualNetworks/$VNET_NAME/subnets/$SUBNET_NAME" \
  --internal-only false \
  --enable-workload-profiles

# Assign a system-assigned managed identity to the environment
az containerapp env identity assign \
  --name "$ENV_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --system-assigned

# Verify the environment was created with VNet integration
az containerapp env show \
  --name "$ENV_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --query "properties.vnetConfiguration.infrastructureSubnetId"

# Get the environment ID for later use
ACA_ENVIRONMENT_ID=$(az containerapp env show \
  --name "$ENV_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --query "id" \
  --output tsv)

echo "Container Apps Environment ID: $ACA_ENVIRONMENT_ID"
```

**Important Notes:**
- The `--infrastructure-subnet-resource-id` parameter enables VNet integration
- The `--internal-only false` parameter allows both internal and external traffic
- Assign a system-assigned managed identity using `az containerapp env identity assign`
- The subnet must have at least `/24` CIDR (256 IPs) for Container Apps

## Step 4: Storage Configuration

### 4.1 Create Azure Storage Account and File Shares

```bash
# Set your storage account name (must be globally unique)
STORAGE_ACCOUNT="your-unique-storage-account-name"

# Create storage account
az storage account create \
  --resource-group "$RESOURCE_GROUP" \
  --name "$STORAGE_ACCOUNT" \
  --location "$LOCATION" \
  --sku Standard_LRS

# Get storage account key
STORAGE_KEY=$(az storage account keys list \
  --resource-group "$RESOURCE_GROUP" \
  --account-name "$STORAGE_ACCOUNT" \
  --query "[0].value" \
  --output tsv)

# Create file share for Prometheus data
az storage share create \
  --name prometheus-data \
  --account-name "$STORAGE_ACCOUNT" \
  --account-key "$STORAGE_KEY"

echo "Storage Account: $STORAGE_ACCOUNT"
echo "Storage Key: $STORAGE_KEY"
```

### 4.2 Attach Storage to Container Apps Environment

```bash
# Attach the file share to the Container Apps environment
az containerapp env storage set \
  --name "$ENV_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --storage-name prometheus-pv \
  --azure-file-account-name "$STORAGE_ACCOUNT" \
  --azure-file-account-key "$STORAGE_KEY" \
  --azure-file-share-name prometheus-data \
  --access-mode ReadWrite

# Verify storage is attached
az containerapp env storage list \
  --name "$ENV_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --output table
```

### 4.3 Private Endpoint Configuration (Recommended)

For detailed step-by-step instructions on setting up the private endpoint, see the [Azure Storage Account Setup Guide](docs/azure-storage-setup.md). The guide includes:

- Automatic VNet discovery from your Container Apps environment
- Creating a dedicated subnet for private endpoints
- Setting up private DNS resolution
- Disabling public network access for security

## Step 5: Bindplane Deployment

### 5.1 Installation

```bash
git clone https://github.com/observiq/bindplane-aca.git
cd bindplane-aca
go build -o bindplane-aca main.go
```

### 5.2 Generate Deployment Files

Now that all prerequisites are set up, generate the deployment files:

```bash
# Set your PostgreSQL and Bindplane configuration
POSTGRES_HOST="your-postgres.postgres.database.azure.com"
POSTGRES_USERNAME="your_postgres_user"
POSTGRES_DATABASE="bindplane"
POSTGRES_PASSWORD="your-postgres-password"
BINDPLANE_LICENSE="your-bindplane-license-key"
SESSION_SECRET="$(uuidgen)"  # Generate a random session secret

# Generate deployment files
./bindplane-aca \
  -aca-environment-id "$ACA_ENVIRONMENT_ID" \
  -postgres-host "$POSTGRES_HOST" \
  -postgres-username "$POSTGRES_USERNAME" \
  -postgres-database "$POSTGRES_DATABASE" \
  -license "$BINDPLANE_LICENSE" \
  -postgres-password "$POSTGRES_PASSWORD" \
  -storage-account-name "$STORAGE_ACCOUNT" \
  -storage-account-key "$STORAGE_KEY" \
  -resource-group "$RESOURCE_GROUP" \
  -session-secret "$SESSION_SECRET" \
  -azure-connection-string "$SERVICE_BUS_CONNECTION" \
  -azure-topic "$TOPIC_NAME" \
  -azure-subscription-id "$SUBSCRIPTION_ID" \
  -azure-resource-group "$RESOURCE_GROUP" \
  -azure-namespace "$SERVICE_BUS_NAMESPACE" \
  -bindplane-tag "1.94.3"
```

### 5.3 Deploy Container Apps

```bash
# Deploy all Container Apps
chmod +x out/deploy.sh
./out/deploy.sh
```

## Architecture

Bindplane on Azure Container Apps consists of multiple microservices working together to provide a comprehensive observability platform. For detailed information about the architecture, component relationships, and system design, see the [Architecture Documentation](docs/architecture.md). For telemetry and monitoring details, see the [Telemetry Guide](docs/telemetry.md).

## Required Images

Ensure your environment can pull the following container images (tags derive from your `-bindplane-tag` value):

- `ghcr.io/observiq/bindplane-ee:<BindplaneTag>`
  - Used by `bindplane` and `bindplane-jobs` apps
- `ghcr.io/observiq/bindplane-transform-agent:<BindplaneTag>-bindplane`
  - Used by `bindplane-transform-agent`
- `ghcr.io/observiq/bindplane-prometheus:<BindplaneTag>`
  - Used by `bindplane-prometheus`

Note: `<BindplaneTag>` is supplied via the `-bindplane-tag` flag (default `1.94.3`).

## Usage

The tool requires several configuration parameters to generate the deployment files:

```bash
./bindplane-aca \
  -aca-environment-id "your-aca-environment-id" \
  -postgres-host "your-postgres.postgres.database.azure.com" \
  -postgres-username "your_postgres_user" \
  -postgres-database "bindplane" \
  -license "your-bindplane-license-key" \
  -postgres-password "your-postgres-password" \
  -storage-account-name "$STORAGE_ACCOUNT" \
  -storage-account-key "your-storage-key" \
  -resource-group "your-resource-group" \
  -session-secret "<random-uuid-or-strong-secret>" \
  -azure-connection-string "your-service-bus-connection-string" \
  -azure-topic "bindplane-events" \
  -azure-subscription-id "your-subscription-id" \
  -azure-resource-group "your-resource-group" \
  -azure-namespace "your-service-bus-namespace" \
  -bindplane-tag "1.94.3"
```

### Required Parameters

| Parameter | Description |
|-----------|-------------|
| `aca-environment-id` | Azure Container Apps Environment ID |
| `license` | Bindplane Enterprise license key |
| `postgres-database` | PostgreSQL database name |
| `postgres-host` | PostgreSQL hostname |
| `postgres-password` | PostgreSQL password |
| `postgres-username` | Username for PostgreSQL database access |
| `resource-group` | Azure Resource Group name for generating deployment commands |
| `session-secret` | Secret used for Bindplane sessions (authentication cookies/session) |
| `storage-account-key` | Access key for the Azure Storage Account |
| `storage-account-name` | Name of Azure Storage Account for persistent volumes |
| `azure-connection-string` | Azure Service Bus connection string |
| `azure-topic` | Azure Service Bus topic name |
| `azure-subscription-id` | Azure subscription ID |
| `azure-resource-group` | Azure resource group name |
| `azure-namespace` | Azure Service Bus namespace |
| `managed-identity-id` | User-assigned managed identity ID |
| `azure-client-id` | Azure managed identity client ID |

### Optional Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `bindplane-remote-url` | `http://localhost:3001` | Bindplane remote URL for external access |
| `bindplane-tag` | `1.94.3` | Bindplane image tag |
| `output-dir` | `out` | Output directory for generated files |
| `postgres-ssl-mode` | `disabled` | PostgreSQL SSL mode: disabled, require, verify-ca, or verify-full |
| `templates-dir` | `templates` | Templates directory |

### Example Usage

```bash
# Set your configuration values
export ACA_ENV_ID="your-environment-id"
export POSTGRES_HOST="mypostgres.postgres.database.azure.com"
export POSTGRES_USER="bindplane_user"
export POSTGRES_DB="bindplane"
export BINDPLANE_LICENSE="your-license-key"
export POSTGRES_PASSWORD="secure-password"
export STORAGE_ACCOUNT="mystorageaccount"
export STORAGE_KEY="your-storage-key"
export RESOURCE_GROUP="bindplane-rg"
export SERVICE_BUS_CONNECTION="your-service-bus-connection-string"
export SERVICE_BUS_TOPIC="bindplane-events"
export SUBSCRIPTION_ID="your-subscription-id"
export SERVICE_BUS_NAMESPACE="your-service-bus-namespace"
# System-assigned identity is used; no identity IDs are required

# Generate deployment files
./bindplane-aca \
  -aca-environment-id "$ACA_ENV_ID" \
  -postgres-host "$POSTGRES_HOST" \
  -postgres-username "$POSTGRES_USER" \
  -postgres-database "$POSTGRES_DB" \
  -license "$BINDPLANE_LICENSE" \
  -postgres-password "$POSTGRES_PASSWORD" \
  -storage-account-name "$STORAGE_ACCOUNT" \
  -storage-account-key "$STORAGE_KEY" \
  -resource-group "$RESOURCE_GROUP" \
  -session-secret "$SESSION_SECRET" \
  -azure-connection-string "$SERVICE_BUS_CONNECTION" \
  -azure-topic "$SERVICE_BUS_TOPIC" \
  -azure-subscription-id "$SUBSCRIPTION_ID" \
  -azure-resource-group "$RESOURCE_GROUP" \
  -azure-namespace "$SERVICE_BUS_NAMESPACE"
```

## Deployment

After running the tool, you'll find generated files in the `out/` directory:

- `bindplane.yaml` - Main Bindplane application
- `jobs.yaml` - Bindplane jobs component
- `prometheus.yaml` - Prometheus monitoring
- `transform-agent.yaml` - Transform agent
- `deploy.sh` - Deployment script

### Deploy using the generated script

```bash
chmod +x out/deploy.sh
./out/deploy.sh
```

### Manual deployment

```bash
# Deploy in order to ensure proper dependencies
az containerapp create --name bindplane-transform-agent --resource-group "$RESOURCE_GROUP" --yaml out/transform-agent.yaml
az containerapp create --name bindplane-prometheus --resource-group "$RESOURCE_GROUP" --yaml out/prometheus.yaml
az containerapp create --name bindplane-jobs --resource-group "$RESOURCE_GROUP" --yaml out/jobs.yaml
az containerapp create --name bindplane --resource-group "$RESOURCE_GROUP" --yaml out/bindplane.yaml
```

## Next Steps

After successful deployment:

1. **Enable external ingress** on the main Bindplane App to expose port 3001 to the public internet
2. **Update the remote URL** template parameter to be the HTTPS Endpoint provided by Azure
   - Example: `https://bindplane.jollysand-c93c8cbe.eastus.azurecontainerapps.io`
3. **Redeploy** with the updated remote URL:
   ```bash
   ./bindplane-aca -bindplane-remote-url "https://your-bindplane-url.azurecontainerapps.io" # ... other parameters
   ./out/deploy.sh
   ```

## Troubleshooting

### Azure Service Bus Authentication Issues

If you encounter authentication errors with Azure Service Bus:

1. **Check Container App logs**:
```bash
az containerapp logs show \
  --name bindplane \
  --resource-group "$RESOURCE_GROUP" \
  --follow
```

2. **Verify managed identity configuration**:
```bash
az containerapp show \
  --name bindplane \
  --resource-group "$RESOURCE_GROUP" \
  --query "identity"
```

3. **Check Service Bus permissions**:
```bash
az role assignment list \
  --assignee "$ENV_PRINCIPAL_ID" \
  --scope "$SERVICE_BUS_ID" \
  --output table
```

4. **Common solutions**:
   - Ensure the Container Apps Environment has a system-assigned identity (`identity.type` is `SystemAssigned`)
   - Verify the environment identity has "Azure Service Bus Data Owner" role on the Service Bus namespace
   - Check that the Service Bus namespace and Container Apps are in the same subscription
   - If identity was just assigned, wait a minute and redeploy to allow RBAC propagation

### Managed Identity Not Found

If you're getting "connection refused" errors on `169.254.169.254:80`, this typically means:

1. **Container Apps Environment doesn't support managed identity**:
   ```bash
   # Check if your environment supports managed identity
   az containerapp env show --name "$ENV_NAME" --resource-group "$RESOURCE_GROUP" --query "identity"
   ```

2. **Managed identity not attached to environment**:
   ```bash
   # Check if the Container Apps Environment has identity
   az containerapp env show --name "$ENV_NAME" --resource-group "$RESOURCE_GROUP" --query "identity"
   
   # Assign if missing
   az containerapp env identity assign --name "$ENV_NAME" --resource-group "$RESOURCE_GROUP" --system-assigned
   ```

3. **No token from IMDS yet**:
   ```bash
   # Retry after a short delay; ensure network egress to IMDS (169.254.169.254) is not blocked
   sleep 60
   ```

### Environment Variables Not Required

The Azure SDK uses DefaultAzureCredential which automatically tries multiple authentication methods in order:
1. Environment variables (AZURE_TENANT_ID, AZURE_CLIENT_ID, AZURE_CLIENT_SECRET)
2. Workload Identity (for Kubernetes/AKS)
3. Managed Identity (for Azure Container Apps) ← **This is what we use**
4. Azure CLI credentials
5. Azure Developer CLI credentials

You do **NOT** need to set AZURE_TENANT_ID, AZURE_CLIENT_ID, or AZURE_CLIENT_SECRET when using system-assigned managed identity with Azure Container Apps.

# Community

The Bindplane Azure Container Apps deployment tool is an open source project. If you'd like to contribute, take a look at our [contribution guidelines](/docs/CONTRIBUTING.md) and [developer guide](/docs/development.md). We look forward to building with you.