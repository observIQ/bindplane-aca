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

BindPlane on Azure Container Apps consists of multiple microservices working together to provide a comprehensive observability platform. For detailed information about the architecture, component relationships, and system design, see the [Architecture Documentation](docs/architecture.md).

## Prerequisites Setup

Before using this tool, ensure you have:

1. **Bindplane Enterprise License** - Valid license key
2. **Azure Container Apps Environment** - Already created and accessible
3. **Azure Database for PostgreSQL** - Deployed and configured (see [Bindplane's PostgreSQL setup documentation](https://docs.bindplane.com/deployment/virtual-machine/postgresql))
4. **Azure Storage Account** - For persistent volume storage
5. **Azure CLI** - Installed and authenticated for deployment

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
  -resource-group "your-resource-group"
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
| `storage-account-key` | Access key for the Azure Storage Account |
| `storage-account-name` | Name of Azure Storage Account for persistent volumes |

### Optional Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `output-dir` | `out` | Directory where generated files will be written |
| `templates-dir` | `templates` | Directory containing the template files |


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
- `nats.yaml` - NATS message bus cluster
- `prometheus.yaml` - Metrics collection and storage
- `transform-agent.yaml` - Data transformation service
- `secrets.yaml` - Kubernetes secrets and persistent volumes
- `deploy.sh` - Automated deployment script

### Manual Deployment

Deploy the components in order:

```bash
cd out/

# 1. Deploy secrets and storage
az containerapp apply -f secrets.yaml --resource-group your-resource-group

# 2. Deploy Prometheus
az containerapp apply -f prometheus.yaml --resource-group your-resource-group

# 3. Deploy Transform Agent
az containerapp apply -f transform-agent.yaml --resource-group your-resource-group

# 4. Deploy NATS cluster
az containerapp apply -f nats.yaml --resource-group your-resource-group

# 5. Deploy Jobs component
az containerapp apply -f jobs.yaml --resource-group your-resource-group

# 6. Deploy main Bindplane application
az containerapp apply -f bindplane.yaml --resource-group your-resource-group
```

# Community

The Bindplane Azure Container Apps deployment tool is an open source project. If you'd like to contribute, take a look at our [contribution guidelines](/docs/CONTRIBUTING.md) and [developer guide](/docs/development.md). We look forward to building with you.

# How can we help?

If you need any additional help feel free to file a GitHub issue or reach out to us at support@bindplane.com.
