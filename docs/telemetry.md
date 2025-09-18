# Telemetry and Monitoring (Azure Container Apps)

Bindplane exposes internal operational metrics so you can monitor server health and behavior. For the full list of metrics and configuration approaches, see Bindplane’s official Monitoring documentation: [Monitoring](https://docs.bindplane.com/configuration/bindplane/monitoring).

On Azure Container Apps, this repository enables monitoring via environment variables (not by editing a config file). The `bindplane` and `jobs` containers are pre-configured to expose Prometheus-compatible metrics at `/metrics`:

```yaml
- name: BINDPLANE_METRICS_TYPE
  value: prometheus
- name: BINDPLANE_METRICS_PROMETHEUS_ENDPOINT
  value: /metrics
- name: BINDPLANE_LOGGING_LEVEL
  value: debug
```

Notes:
- Metrics are served at the HTTP endpoint `/metrics` in Prometheus format and can be scraped by your platform of choice.
- Logging level is configurable via `BINDPLANE_LOGGING_LEVEL`.
  - Use `debug` for new deployments and troubleshooting.
  - Use `info` or `warn` for production systems operating normally.

If you prefer OTLP metrics export or other options, refer to Bindplane’s official guide: [Monitoring](https://docs.bindplane.com/configuration/bindplane/monitoring).
