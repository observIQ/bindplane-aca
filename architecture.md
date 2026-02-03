# Architecture (High-Level)

This repository deploys Bindplane components onto **Azure Container Apps (ACA)** using YAML templates in `templates/`.

At a high level:

- **One externally-accessible endpoint** is expected:
  - `bindplane`: serves the user-facing web UI and collector-facing **OpAMP websocket** traffic
- **Azure Service Bus** is the shared event bus between Bindplane services.
- **Postgres** is required and is **not** deployed by this repository.
- **Prometheus** stores throughput and agent health metrics (recommended to run externally, similar to Postgres).

## Components

### `bindplane`

**Purpose**: Serves the user-facing web UI and the collector-facing **OpAMP** endpoint; should scale with the number of connected collectors and concurrent users.

**Connectivity**:
- Receives **OpAMP agent websocket** connections (external endpoint).
- Receives **user web UI** traffic (external endpoint).
- Sends/receives events via **Azure Service Bus**.
- Connects to **Postgres**.
- Connects to **Prometheus**.
- Can connect to `bindplane-transform-agent` (all Bindplane services are capable, though UI traffic typically drives Live Preview).

### `bindplane-jobs`

**Purpose**: Single-instance jobs runner for periodic jobs, database migrations, and coordination tasks.

**Connectivity**:
- Does **not** receive HTTP connections from users or agents.
- Sends/receives events via **Azure Service Bus**.
- Connects to **Postgres**.
- Connects to **Prometheus**.
- Can connect to `bindplane-transform-agent` (all Bindplane services are capable, though UI traffic typically drives Live Preview).

### `bindplane-transform-agent`

**Purpose**: Powers Bindplane **Live Preview** by executing transformation pipelines.

**Connectivity**:
- Receives **Live Preview** requests from `bindplane` (internal-only).
- Does not require Postgres, Service Bus, or Prometheus.

### `prometheus`

**Purpose**: TSDB storing:
- throughput metrics
- agent health metrics (CPU / memory)

**Connectivity**:
- Receives TSDB requests from `bindplane` (and other Bindplane services as needed).
- Recommended: operate Prometheus **externally from ACA** (similar to Postgres).

### `postgres` (required, external to this repo)

**Purpose**: Primary database for Bindplane configuration and operational state.

**Guidance**:
- Host Postgres in the **same region** (and **availability zone** if possible) as ACA for low latency.
- Use **private** connectivity (VPC/VNET) rather than a public IP address.

## `BINDPLANE_REMOTE_URL` (important)

All Bindplane services in this deployment may share the same `BINDPLANE_REMOTE_URL` value.

- **It must point to the external OpAMP endpoint of `bindplane`**.
- The expected form is:
  - `wss://<bindplane_hostname>/v1/opamp`

This is typically set/updated as a follow-up step after the `bindplane` external hostname is known.

## Connectivity diagram (ASCII)

```
 +----------------------+                       +-------------------+
 |    Users (Web UI)    |                       | Collectors/Agents |
 +----------+-----------+                       | (OpAMP clients)   |
            |                                   +---------+---------+
            | HTTPS (external)                            |
            v                                             | WSS /v1/opamp (external)
 +----------------------+   +----------------------+   +----------------------+
 |      bindplane       |   |   bindplane-jobs     |
 | (UI + OpAMP endpoint)|   | (single-instance)    |
 +----------+-----------+   +----------+-----------+
            |                        |
            |                        |
            +-----------+------------+-------------+
                        |            |
                        |            |
                        v            v
     +-----------+  +-------------------+  +-----------------------+  +----------------------+
     | Postgres   |  | Azure Service Bus |  |      Prometheus       |  | bindplane-transform- |
     | (external) |  | (events/topic)    |  | (TSDB; external rec.) |  |        agent         |
     +-----------+  +-------------------+  +-----------------------+  +----------------------+
```

