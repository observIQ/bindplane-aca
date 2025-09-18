# Bindplane Azure Container Apps Architecture

This document describes the architecture of Bindplane when deployed on Azure Container Apps (ACA), detailing the components, their relationships, and how they work together to provide a scalable observability platform.

## Architecture Overview

Bindplane on Azure Container Apps follows a hybrid architecture with five main components. Rather than traditional microservices, most components utilize the same Bindplane container image (`ghcr.io/observiq/bindplane-ee`) configured with different operational modes to perform specialized functions. This approach provides the benefits of service separation while maintaining consistency and simplifying deployment. The deployment is designed to be cloud-native, leveraging Azure Container Apps' scaling and orchestration features.

### Architecture Pattern

The core Bindplane components (Bindplane main application, Jobs, and NATS cluster) all run the same container image but are configured with different `BINDPLANE_MODE` values to specialize their behavior:

- **Single Codebase, Multiple Roles**: All Bindplane components share the same binary but operate in different modes
- **Configuration-Driven Specialization**: Each component's role is determined by environment variables rather than separate codebases
- **Hybrid Service Model**: Combines the operational benefits of microservices (independent scaling, deployment) with the maintenance benefits of a monolithic codebase

```
┌───────────────────────────────────────────────────────────────┐
│           Azure Container Apps Environment                    │
│                                                               │
│  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐          │
│  │  Bindplane  │   │    Jobs     │   │ Transform   │          │
│  │    (1-N)    │◄─►│     (1)     │◄─►│  Agent (2)  │          │
│  └─────────────┘   └─────────────┘   └─────────────┘          │
│          │                    │                    │          │
│          └──────────────┬─────┼─────┬──────────────┘          │
│                         │     │     │                         │
│                  ┌─────▼─────▼─────▼─────┐    ┌─────────────┐ │
│                  │         NATS          │    │  Prometheus │ │
│                  │       Cluster (3)     │    │     (1)     │ │
│                  └───────────────────────┘    └─────────────┘ │
└───────────────────────────────────────────────────────────────┘
           │
  ┌────────▼────────┐    ┌─────────────┐
  │   PostgreSQL    │    │   Azure     │
  │    Database     │    │  Storage    │
  │   (External)    │    │ (External)  │
  └─────────────────┘    └─────────────┘
```

## Core Components

### 1. Bindplane (Main Application)

**Purpose**: The primary Bindplane application server that provides the web UI, API, and OpAMP
capabilities.

**Architecture Details**:
- **Container Image**: `ghcr.io/observiq/bindplane-ee`
- **Mode**: `node` - Performs all server duties but does not perform database migrations or scheduled tasks
- **Replicas**: 1-3 (horizontally scalable)
- **Ingress**: External HTTPS ingress for user access
- **Dependencies**: PostgreSQL, NATS, Prometheus, Transform Agent, Jobs
- **Ports**: 
  - 3001: Main HTTP/HTTPS interface
  - Health check on `/health` endpoint

**Key Responsibilities**:
- Web-based management interface
- REST API for configuration and management
- OpAMP server for agent management
- Agent configuration distribution
- User authentication and authorization
- Operates NATS client for messaging with the NATS cluster

### 2. Jobs Component

**Purpose**: Handles background processing tasks and long-running operations. Creates the initial database schema for new deployments and manages database migrations during upgrades.

**Architecture Details**:
- **Container Image**: `ghcr.io/observiq/bindplane-ee`
- **Mode**: `all` - Performs all duties
- **Replicas**: 1 (single instance for job coordination)
- **Ingress**: Internal only
- **Dependencies**: NATS, PostgreSQL, Prometheus, Transform Agent
- **Ports**: Health check on `/health` endpoint

**Key Responsibilities**:
- Scheduled jobs (disconnected agent cleanup, interval based tasks)
- Background task processing
- Resource seeding
- Database initialization for new deployments
- Database schema migrations during upgrades
- Operates NATS client for messaging with the NATS cluster

### 3. Transform Agent

**Purpose**: Facilitates Bindplane [Live Preview](https://bindplane.com/docs/feature-guides/live-preview) by executing transformation pipelines for preview sessions.

**Architecture Details**:
- **Container Image**: `ghcr.io/observiq/bindplane-transform-agent`
- **Replicas**: 2 (horizontally scalable for processing load)
- **Ingress**: Internal only
- **Dependencies**: None (stateless service)
- **Ports**: 
  - 4568: Transform service endpoint
  - Health check on `/collector-version` endpoint

### 4. NATS Cluster

**Purpose**: Bindplane server processes that operate embedded [NATS](https://github.com/nats-io/nats-server) servers for message bus and event streaming platform functionality. Each Bindplane server instance publishes messages for other Bindplane servers to consume, enabling distributed coordination and data sharing across the cluster.

**Architecture Details**:
- **Container Image**: `ghcr.io/observiq/bindplane-ee`
- **Replicas**: 3 (clustered for high availability)
- **Ingress**: Internal only
- **Ports**:
  - 4222: Client connections
  - 6222: Cluster mesh communication
  - 8222: HTTP monitoring and health checks
  
  Note: NATS does not connect to PostgreSQL. It must start and be available before the Jobs component so that Jobs can initialize the database and run migrations while coordinating via NATS.

**Key Responsibilities**:
- Inter-service messaging and communication
- Event streaming and pub/sub messaging
- Cluster coordination and consensus
- Message distribution between Bindplane server instances

**Message Types**:
- **Agent Snapshots**: Live Preview and Recent Telemetry data sharing between server instances
- **Account Resource Seeding**: Resource initialization messages (always performed by the Jobs pod)
- **Configuration Updates**: Agent configuration changes distributed across the cluster
- **System Events**: Operational events and status updates between components

**Clustering Details**:
- Uses NATS clustering for high availability
- Cluster mesh topology for fault tolerance
- Each Bindplane server instance runs an embedded NATS server that participates in the cluster
- Uses `EmptyDir` volume for Bindplane scratch storage; no Azure Files required

### 5. Prometheus

**Purpose**: Internal metrics storage for Bindplane collector throughput and health metrics. This is not intended for consumption by external monitoring tools like Grafana and does not store Bindplane's operational metrics.

**Architecture Details**:
- **Container Image**: `ghcr.io/observiq/bindplane-prometheus:<BindplaneTag>`
- **Replicas**: 1 (single instance with persistent storage)
- **Ingress**: Internal only
- **Dependencies**: Azure File Storage (for metrics persistence)
- **Ports**: 
  - 9090: Prometheus server and API
  - Health check on port 9090

**Key Responsibilities**:
- Collector throughput metrics storage
- Collector health status metrics
- Internal Bindplane metrics analysis

**Scope Limitations**:
- Not designed for external monitoring tool integration
- Does not store Bindplane platform operational metrics
- Focused solely on collector-related metrics

## External Dependencies

### Azure Database for PostgreSQL

**Purpose**: Primary database for Bindplane configuration, metadata, and operational data.

**Integration**:
- Only the main Bindplane application and Jobs component connect to PostgreSQL
- NATS, Prometheus, and Transform Agent components do not connect to PostgreSQL
- Stores agent configurations, user data, and system state
- Configured with SSL connections for security
- Requires proper firewall rules to allow Container Apps access

### Azure Storage Account

**Purpose**: Provides persistent volume storage for stateful components.

**Storage Allocations**:
- **Prometheus**: 120GB Azure File Storage for metrics data retention

**Configuration**:
- Uses Azure File Storage shares mounted as persistent volumes
- Requires storage account access keys for authentication
- Supports concurrent access from multiple container instances

## Communication Patterns

### Internal Service Communication

All components communicate within the Azure Container Apps Environment using internal DNS names:

- **NATS**: `bindplane-nats:4222` (client), `bindplane-nats:6222` (cluster)
- **Prometheus**: `bindplane-prometheus:9090`
- **Transform Agent**: `bindplane-transform-agent:4568`
- **Jobs**: Internal HTTP APIs
- **Bindplane**: Internal HTTP APIs

### Message Flow

1. **Configuration Changes**: Bindplane → NATS → Jobs → Agent Updates
2. **Metrics Collection**: All Components → Prometheus
3. **Data Transformation**: Bindplane → Transform Agent → Processed Data
4. **Background Processing**: Bindplane → NATS → Jobs → Database Updates

## Scaling Considerations

### Horizontal Scaling

- **Bindplane**: Can scale from 1-3 replicas based on user load
- **Jobs**: Single instance (ensures job coordination)
- **Transform Agent**: Runs with 2 replicas and can be scaled independently for data processing load
- **NATS**: Fixed at 3 replicas (clustering requirement)
- **Prometheus**: Single instance with persistent storage

### Resource Allocation

Each component is configured with appropriate resource requests and limits:
- CPU and memory limits prevent resource contention
- Health checks ensure automatic restart of failed containers
- Readiness probes ensure traffic only routes to healthy instances

## High Availability

### Component Redundancy

- **NATS Cluster**: 3-node cluster provides fault tolerance
- **Bindplane**: Multiple replicas with load balancing
- **Database**: Azure Database for PostgreSQL provides built-in HA
- **Storage**: Azure Storage provides built-in redundancy

### Failure Scenarios

- **Single Container Failure**: Automatic restart and replacement
- **Node Failure**: Workload migration to healthy nodes
- **Network Partitions**: NATS cluster maintains consensus
- **Database Failure**: Azure PostgreSQL automatic failover

## Monitoring and Observability

### Health Checks

Each component implements health checks for monitoring:
- **HTTP Health Endpoints**: `/health`, `/healthz`, `/collector-version`
- **Readiness Probes**: Ensure components are ready to receive traffic
- **Liveness Probes**: Detect and restart failed containers

### Metrics Collection

- **Prometheus Integration**: All components expose metrics
- **Custom Metrics**: Application-specific performance indicators
- **Infrastructure Metrics**: Container and Azure resource metrics

## Data Flow Architecture

1. **Agent Configuration**: Bindplane generates configurations → stored in PostgreSQL → distributed via NATS
2. **Telemetry Ingestion**: Agents → Transform Agent → processed data → destination systems
3. **System Monitoring**: All components → Prometheus → metrics storage and alerting
4. **Background Processing**: Scheduled tasks → Jobs component → database updates

This architecture provides a robust, scalable foundation for observability data collection and management on Azure Container Apps.
