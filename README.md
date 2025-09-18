# Bindplane Azure Container Apps Deployment

A Go-based templating tool for generating Bindplane deployment files for Azure Container Apps (ACA). This tool processes YAML templates and generates deployment-ready files with user-supplied configuration values.

## Quick Start

### Prerequisites

1. **Bindplane Enterprise License** - Bindplane on Azure Container Apps is supported by Bindplane Enterprise only and requires a valid Bindplane Enterprise license key
2. Install Go by following the [official installation guide](https://golang.org/doc/install)
3. Install Azure CLI by following the [official installation guide](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli)

### Installation

```bash
git clone https://github.com/observiq/bindplane-aca.git
cd bindplane-aca
go build -o bindplane-aca main.go
```

## Architecture

Bindplane on Azure Container Apps consists of multiple microservices working together to provide a comprehensive observability platform. For detailed information about the architecture, component relationships, and system design, see the [Architecture Documentation](docs/architecture.md). For telemetry and monitoring details, see the [Telemetry Guide](docs/telemetry.md).

## Prerequisites Setup

Before using this tool, ensure you have:

1. **Bindplane Enterprise License** - Valid license key
2. **Azure Container Apps Environment** - Already created and accessible (see [Creating a VNet-injected Container Apps Environment](#creating-a-vnet-injected-container-apps-environment))
3. **Azure Database for PostgreSQL** - Deployed and configured (see [Bindplane's PostgreSQL setup documentation](https://docs.bindplane.com/deployment/virtual-machine/postgresql))
4. **Azure Storage Account** - For persistent volume storage (see [Azure Storage Account Setup Guide](docs/azure-storage-setup.md))
5. **Azure CLI** - Installed and authenticated for deployment

### Creating a VNet-injected Container Apps Environment

This deployment requires a VNet-injected Container Apps environment to support private endpoints for Azure Files. Create your environment with the following commands:

```bash
# Set your variables
RESOURCE_GROUP="your-resource-group"
LOCATION="eastus"
ENV_NAME="your-aca-environment"
VNET_NAME="bindplane-vnet"
SUBNET_NAME="container-apps-subnet"
VNET_CIDR="10.0.0.0/16"
SUBNET_CIDR="10.0.1.0/24"

# Create resource group (if it doesn't exist)
az group create \
  --name "$RESOURCE_GROUP" \
  --location "$LOCATION"

# Create VNet
az network vnet create \
  --resource-group "$RESOURCE_GROUP" \
  --name "$VNET_NAME" \
  --location "$LOCATION" \
  --address-prefix "$VNET_CIDR"

# Create subnet for Container Apps
az network vnet subnet create \
  --resource-group "$RESOURCE_GROUP" \
  --vnet-name "$VNET_NAME" \
  --name "$SUBNET_NAME" \
  --address-prefix "$SUBNET_CIDR"

# Delegate the subnet to Microsoft.App/environments (required for Container Apps)
az network vnet subnet update \
  --resource-group "$RESOURCE_GROUP" \
  --vnet-name "$VNET_NAME" \
  --name "$SUBNET_NAME" \
  --delegations Microsoft.App/environments

# Create Container Apps environment with VNet integration
az containerapp env create \
  --name "$ENV_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --location "$LOCATION" \
  --infrastructure-subnet-resource-id "/subscriptions/$(az account show --query id -o tsv)/resourceGroups/$RESOURCE_GROUP/providers/Microsoft.Network/virtualNetworks/$VNET_NAME/subnets/$SUBNET_NAME" \
  --internal-only false

# Verify the environment was created with VNet integration
az containerapp env show \
  --name "$ENV_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --query "properties.vnetConfiguration.infrastructureSubnetId"
```

**Important Notes:**
- The `--infrastructure-subnet-resource-id` parameter enables VNet integration
- The `--internal-only false` parameter allows both internal and external traffic
- The subnet must have at least `/24` CIDR (256 IPs) for Container Apps
- After creation, you can use the VNet details in the [Azure Storage Account Setup Guide](docs/azure-storage-setup.md) for private endpoint configuration


### Required Images

Ensure your environment can pull the following container images (tags derive from your `-bindplane-tag` value):

- `ghcr.io/observiq/bindplane-ee:<BindplaneTag>`
  - Used by `bindplane`, `bindplane-jobs`, and all `bindplane-nats-*` apps
- `ghcr.io/observiq/bindplane-transform-agent:<BindplaneTag>-bindplane`
  - Used by `bindplane-transform-agent`
- `ghcr.io/observiq/bindplane-prometheus:<BindplaneTag>`
  - Used by `bindplane-prometheus`

Note: `<BindplaneTag>` is supplied via the `-bindplane-tag` flag (default `1.94.3`).

### Create and attach Azure File storage volumes (Prometheus, NATS)

Container Apps volumes that use Azure Files must be backed by file shares in your Storage Account and attached to the Container Apps environment as "environment storage" before apps can mount them.

1) Create the Azure File share (names below must match the templates):

```bash
STORAGE_ACCOUNT="<your storage account>"
STORAGE_KEY="<your storage key>"

# Create file share
az storage share create --account-name "$STORAGE_ACCOUNT" --account-key "$STORAGE_KEY" --name prometheus-data
```

2) Attach that share to the Container Apps environment as environment storage:

```bash
# Prometheus TSDB data
az containerapp env storage set \
  --name "$ENV_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --storage-name prometheus-pv \
  --azure-file-account-name "$STORAGE_ACCOUNT" \
  --azure-file-account-key "$STORAGE_KEY" \
  --azure-file-share-name prometheus-data \
  --access-mode ReadWrite
```

Note: NATS uses an `emptyDir` volume in Azure Container Apps and does not require persistent storage.

Once attached, the generated YAML in `templates/` references the above environment storage names via `template.volumes[].storageName` and mounts them via `volumeMounts[].volumeName`.

### Network security for Azure Files (required)

Azure Container Apps must be able to reach your Storage Account's Azure Files endpoint over SMB (TCP 445). This deployment requires a private endpoint for secure connectivity.

1) Verify the share and environment storage

```bash
# Share exists
az storage share show \
  --name prometheus-data \
  --account-name "$STORAGE_ACCOUNT" \
  --account-key "$STORAGE_KEY" -o table

# Environment storage attached (should list 'prometheus-pv')
az containerapp env storage list \
  --name "$ENV_NAME" \
  --resource-group "$RESOURCE_GROUP" -o table
```

2) Private endpoint configuration (required)

For detailed step-by-step instructions on setting up the private endpoint, see the [Azure Storage Account Setup Guide](docs/azure-storage-setup.md). The guide includes:

- Automatic VNet discovery from your Container Apps environment
- Creating a dedicated subnet for private endpoints
- Setting up private DNS resolution
- Disabling public network access for security

**Note:** This deployment requires a VNet-injected Container Apps environment. If you haven't created one yet, see the [Creating a VNet-injected Container Apps Environment](#creating-a-vnet-injected-container-apps-environment) section above.

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
  -bindplane-tag "1.94.3"
```

### Required Parameters

| Parameter | Description |
|-----------|-------------|
| `aca-environment-id` | Azure Container Apps Environment ID where the apps will be generated for |
| `license` | Your Bindplane Enterprise license key |
| `postgres-database` | Name of the PostgreSQL database |
| `postgres-host` | Hostname of your Azure Database for PostgreSQL server |
| `postgres-password` | Password for PostgreSQL database access |
| `postgres-username` | Username for PostgreSQL database access |
| `resource-group` | Azure Resource Group name for generating deployment commands |
| `session-secret` | Secret used for Bindplane sessions (authentication cookies/session) |
| `storage-account-key` | Access key for the Azure Storage Account |
| `storage-account-name` | Name of Azure Storage Account for persistent volumes |

### Optional Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `output-dir` | `out` | Directory where generated files will be written |
| `templates-dir` | `templates` | Directory containing the template files |
| `bindplane-tag` | `1.94.3` | Image tag for Bindplane components (also used to derive transform-agent and prometheus tags) |
| `bindplane-remote-url` | `http://localhost:3001` | Remote URL for Bindplane components to communicate with the main application |


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
  -resource-group "$RESOURCE_GROUP"
```

## Deployment

After running the tool, you'll find generated files in the `out/` directory:

- `bindplane.yaml` - Main Bindplane application
- `jobs.yaml` - Background job processing component
- `nats-0.yaml`, `nats-1.yaml`, `nats-2.yaml` - NATS message bus cluster (3 apps)
- `prometheus.yaml` - Metrics collection and storage
- `transform-agent.yaml` - Data transformation service
- `deploy.sh` - Automated deployment script

## Next Steps

After successfully deploying Bindplane to Azure Container Apps, you'll need to enable external access to the main Bindplane application to make it accessible from the public internet.

### Enable External Ingress

By default, the main Bindplane application is configured with `external: false` for security. To enable public access:

1. **Update the Bindplane Container App configuration** to enable external ingress:

```bash
# Enable external ingress for the main Bindplane app
az containerapp ingress enable \
  --name bindplane \
  --resource-group "$RESOURCE_GROUP" \
  --type external \
  --target-port 3001 \
  --transport http
```

2. **Get the HTTPS endpoint** provided by Azure Container Apps:

```bash
# Get the external URL for the Bindplane app
az containerapp show \
  --name bindplane \
  --resource-group "$RESOURCE_GROUP" \
  --query "properties.configuration.ingress.fqdn" \
  --output tsv
```

This will return a URL similar to: `https://bindplane.jollysand-c93c8cbe.eastus.azurecontainerapps.io`

3. **Update the remote URL configuration** by regenerating your deployment files with the new HTTPS endpoint:

```bash
# Regenerate deployment files with the public HTTPS endpoint
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
  -bindplane-remote-url "https://bindplane.jollysand-c93c8cbe.eastus.azurecontainerapps.io"
```

4. **Redeploy the updated configuration**:

```bash
# Update the Container Apps with the new configuration
az containerapp update --name bindplane --resource-group "$RESOURCE_GROUP" --yaml out/bindplane.yaml
az containerapp update --name bindplane-jobs --resource-group "$RESOURCE_GROUP" --yaml out/jobs.yaml
az containerapp update --name bindplane-nats-0 --resource-group "$RESOURCE_GROUP" --yaml out/nats-0.yaml
az containerapp update --name bindplane-nats-1 --resource-group "$RESOURCE_GROUP" --yaml out/nats-1.yaml
az containerapp update --name bindplane-nats-2 --resource-group "$RESOURCE_GROUP" --yaml out/nats-2.yaml
```

### Access Bindplane

Once the ingress is enabled and the configuration is updated, you can access Bindplane at the HTTPS endpoint provided by Azure Container Apps. The default credentials are:
- **Username**: `bpuser`
- **Password**: `bppass`

**Security Note**: Change the default credentials immediately after first login for production deployments.

# Community

The Bindplane Azure Container Apps deployment tool is an open source project. If you'd like to contribute, take a look at our [contribution guidelines](/docs/CONTRIBUTING.md) and [developer guide](/docs/development.md). We look forward to building with you.

# How can we help?

If you need any additional help feel free to file a GitHub issue or reach out to us at support@bindplane.com.
