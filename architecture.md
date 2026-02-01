# Architecture (High-Level)

This repository deploys Bindplane components onto **Azure Container Apps (ACA)** using YAML templates in `templates/`.

At a high level:

- **Two externally-accessible endpoints** are expected:
  - `bindplane-ui`: user-facing web UI traffic
  - `bindplane`: collector-facing **OpAMP websocket** traffic (and may also serve UI)
- **Azure Service Bus** is the shared event bus between Bindplane services.
- **Postgres** is required and is **not** deployed by this repository.
- **Prometheus** stores throughput and agent health metrics (recommended to run externally, similar to Postgres).

## Components

### `bindplane-ui`

**Purpose**: Dedicated to user interface interaction; should scale with the number of concurrent users.

**Connectivity**:
- Receives **user web UI** traffic (external endpoint).
- Sends/receives events via **Azure Service Bus**.
- Connects to **Postgres**.
- Connects to **Prometheus**.
- Connects to `bindplane-transform-agent` for **Live Preview**.

### `bindplane`

**Purpose**: Dedicated to **OpAMP**; should scale with the number of connected collectors (and may also serve UI traffic).

**Connectivity**:
- Receives **OpAMP agent websocket** connections (external endpoint).
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
- Receives **Live Preview** requests from `bindplane-ui` (internal-only).
- Does not require Postgres, Service Bus, or Prometheus.

### `prometheus`

**Purpose**: TSDB storing:
- throughput metrics
- agent health metrics (CPU / memory)

**Connectivity**:
- Receives TSDB requests from `bindplane-ui` and `bindplane`.
- Recommended: operate Prometheus **externally from ACA** (similar to Postgres).

### `postgres` (required, external to this repo)

**Purpose**: Primary database for Bindplane configuration and operational state.

**Guidance**:
- Host Postgres in the **same region** (and **availability zone** if possible) as ACA for low latency.
- Use **private** connectivity (VPC/VNET) rather than a public IP address.

## `BINDPLANE_REMOTE_URL` (important)

All Bindplane services in this deployment may share the same `BINDPLANE_REMOTE_URL` value.

- **It must point to the external OpAMP endpoint of `bindplane`** (not `bindplane-ui`).
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
 |     bindplane-ui     |   |      bindplane       |   |   bindplane-jobs     |
 | (UI + event-driven)  |   | (OpAMP endpoint)     |   | (single-instance)    |
 +----------+-----------+   +----------+-----------+   +----------+-----------+
            |                        |                          |
            |                        |                          |
            +-----------+------------+------------+-------------+
                        |            |            |
                        |            |            |
                        v            v            v
     +-----------+  +-------------------+  +-----------------------+  +----------------------+
     | Postgres   |  | Azure Service Bus |  |      Prometheus       |  | bindplane-transform- |
     | (external) |  | (events/topic)    |  | (TSDB; external rec.) |  |        agent         |
     +-----------+  +-------------------+  +-----------------------+  +----------------------+
```

