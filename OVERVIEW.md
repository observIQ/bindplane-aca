# Overview

This document describes the configuration and instance sizing required for Bindplane to rapidly accept 10,000 to 30,000
new agents at once. Because Bindplane uses very little resources at idle, this test was conducted with 100,000 agents
connected before measuring the impact of 10,000-30,000 new agents.

This document is not exhaustive. It will not cover basic details, and instead will focus on the advanced options required
for the following services:
- Azure Service Bus
- Postgres
- Advanced Bindplane configuration options

## Notable Observations

- Container Apps [Premium Ingress](https://learn.microsoft.com/en-us/azure/container-apps/premium-ingress?pivots=azure-cli) was required to support rapid OpAMP connections.
- Azure Service Bus Premium tier was required as lower tiers suffered from throttling and message size limitation. 

# Azure Service Bus

Bindplane uses an **Azure Service Bus Topic** with `Premium` SKU for event messaging.

## Service Bus configuration

| Setting | Value |
| --- | --- |
| **Namespace SKU** | Premium |
| **Namespace capacity** | 1 |
| **Default message TTL** | 5 minutes |
| **Topic max size** | 10GB |
| **Max message size** | 10MB |

# Postgres

A Postgres instance with 8 CPU cores and 32GB is recommended for this scale. The database resources are over provisioned to ensure
the system can rapidly accept thousands of new agents.

Postgres should be equipped with an Azure SSD. During out testing, we used a Premium SSD LRS (P10) rated for 500 IOPS and 100 MB/s.
During the test, Postgres disk utilization was very low, never exceeding the rated IOPS or throughput.

It is critical that the Postgres instance be hosted in the same Azure availability zone and network as the Azure Container Apps
deployment. 

# Azure Container Apps

This section describes the **Azure Container Apps** deployments used to deploy Bindplane on Azure Container Apps. It does not cover
supplemental services such as Bindplane Transform Agent or Bindplane Prometheus, as these are not the focus of this sizing exercise.

## Ingress

This deployment uses **Premium Ingress**. The default Azure Container Apps ingress is not suitable when scaling OpAMP websocket connections.
The default solution should result in 500 status codes and "Envoy Overloaded" errors.

### Premium Ingress configuration

| Setting | Value |
| --- | --- |
| **Workload profile** | Dedicated workload profile (`ingress-d4`, type `D4`) |
| **Workload profile nodes** | min `4`, max `12` |
| **Ingress replicas** | min `4`, max `12` |
| **Termination grace period** | `1800s` |
| **Request idle timeout** | `30s` |

## Services

Two Bindplane services are deployed. The primary differentiator between the two are the modes (`BINDPLANE_MODE`)
they are configured to use.
- `bindplane` (Node): Services UI and OpAMP requests
- `jobs` (All): Does not receive ingress, handles periodic jobs

### `bindplane`

This is the primary container deployment. It services user interactions over the web interface and managed collector connections over OpAMP.

#### Scaling

Autoscaling is possible on this deployment, however, due to the nature of long running websockets, it is recommended to configure
the minimum replica count to the expected number of containers. Autoscaling should be used to handle unexpected bursts in traffic,
not as a mechanism to find the correct number of replicas.

When collectors connect to Bindplane, they perform the most expensive operations. Once the connection logic is finished, Bindplane will return to near idle resource utilization. This can cause auto-scalers to rapidly scale up and then rapidly scale down. The scale down will likely trigger a large number of collectors to reconnect. This process will cause significant churn and resource utilization, leading to a degraded experience.

#### Configuration

- **Ingress**: external `true`
- **Resources**: `cpu: 2.0`, `memory: 4Gi`
- **Scale**: `minReplicas: 8`, `maxReplicas: 16`

#### Environment options

The following environment variables are set in this manifest (descriptions intentionally left blank for you to fill in).

| Name | Value | Description |
| --- | --- | --- |
| `BINDPLANE_MODE` | `node` | Mode node disables periodic jobs. Jobs are handled by `jobs` deployment |
| `BINDPLANE_POSTGRES_MAX_CONNECTIONS` | `50` | The maximum number of Postgres connections for each instance. This value times the number of replicas shall not exceed the max number of connections configured on your Postgres instance. |
| `BINDPLANE_POSTGRES_MAX_IDLE_CONNECTIONS` | `15` | The number of connections Bindplane will keep open to Postgres, when under low load / idle. |
| `BINDPLANE_MAX_CONCURRENCY` | `15` | Maximum number of new OpAMP requests that can be accepted concurrently. Should never exceed half of `BINDPLANE_POSTGRES_MAX_CONNECTIONS`. |
| `BINDPLANE_AGENTS_MAX_SIMULTANEOUS_CONNECTIONS` | `15` | Maximum number of OpAMP requests that can be accepted concurrently before rate limiting is activated. Should be the same as Max concurrency unless directed by Bindplane support. |
| `BINDPLANE_AZURE_MAX_PAYLOAD_SIZE` | `10000000` | 10MB, should match the configured size limit on the Azure Service Bus Topic |

### `jobs`

This is the Bindplane Jobs deployment. It should not receive ingress, and it should always have a
single replica.

#### Scaling

This deployment should remain at **one replica**. It exists to run periodic jobs and should not be scaled out unless directed by Bindplane support.

#### Configuration

- **Ingress**: external `false`
- **Resources**: `cpu: 2`, `memory: 4Gi`
- **Scale**: `minReplicas: 1`, `maxReplicas: 1`

#### Environment options

Generally, the Jobs deployment will have the same configuration parameters as the Bindplane deployment. The primary
difference is that the Jobs deployment uses `BINDPLANE_MODE=all` instead of `node`.

The Jobs container connects to Postgres and Azure Service Bus the same as Bindplane Node, therefore it should use the same
Postgres and Azure configuration options.

| Name | Value | Description |
| --- | --- | --- |
| `BINDPLANE_MODE` | `all` | Mode all enables periodic jobs.
| `BINDPLANE_POSTGRES_MAX_CONNECTIONS` | `50` | The maximum number of Postgres connections for each instance. This value times the number of replicas shall not exceed the max number of connections configured on your Postgres instance. |
| `BINDPLANE_POSTGRES_MAX_IDLE_CONNECTIONS` | `15` | The number of connections Bindplane will keep open to Postgres, when under low load / idle. |
| `BINDPLANE_AZURE_MAX_PAYLOAD_SIZE` | `10000000` | 10MB, should match the configured size limit on the Azure Service Bus Topic |
