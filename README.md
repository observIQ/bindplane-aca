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

Bindplane on Azure Container Apps consists of multiple microservices working together to provide a comprehensive observability platform. For detailed information about the architecture, component relationships, and system design, see the [Architecture Documentation](docs/architecture.md).

## Prerequisites Setup

Before using this tool, ensure you have:

1. **Bindplane Enterprise License** - Valid license key
2. **Azure Container Apps Environment** - Already created and accessible
3. **Azure Database for PostgreSQL** - Deployed and configured (see [Bindplane's PostgreSQL setup documentation](https://docs.bindplane.com/deployment/virtual-machine/postgresql))
4. **Azure Storage Account** - For persistent volume storage (see [Azure Storage Account Setup Guide](docs/azure-storage-setup.md))
5. **Azure CLI** - Installed and authenticated for deployment


### Required Images

Ensure your environment can pull the following container images (tags derive from your `-bindplane-tag` value):

- `ghcr.io/observiq/bindplane-ee:<BindplaneTag>`
  - Used by `bindplane`, `bindplane-jobs`, `bindplane-jobs-migrate`, and all `bindplane-nats-*` apps
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

Azure Container Apps must be able to reach your Storage Account's Azure Files endpoint over SMB (TCP 445). Choose one approach:

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

2A) Public networking (simpler; production caution)

```bash
# Ensure public access is enabled on the storage account
az storage account update \
  --name "$STORAGE_ACCOUNT" \
  --resource-group "$RESOURCE_GROUP" \
  --public-network-access Enabled

# If using selected networks, allow ACA outbound IPs
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

2B) Private endpoint (recommended for production)

```bash
# Create a private endpoint for the File service and link Private DNS
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

# Optional: Disable public network access on the storage account after PE is configured
az storage account update \
  --name "$STORAGE_ACCOUNT" \
  --resource-group "$RESOURCE_GROUP" \
  --public-network-access Disabled
```

Note: If your organization blocks outbound TCP 445, you must use the private endpoint approach.

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
  -storage-account-name "yourstorageaccount" \
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
| `session-secret` |  | REQUIRED: Secret for Bindplane sessions |


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

# Community

The Bindplane Azure Container Apps deployment tool is an open source project. If you'd like to contribute, take a look at our [contribution guidelines](/docs/CONTRIBUTING.md) and [developer guide](/docs/development.md). We look forward to building with you.

# How can we help?

If you need any additional help feel free to file a GitHub issue or reach out to us at support@bindplane.com.
