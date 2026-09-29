# Local Development Guide

Spin up the exporter, Prometheus, and Grafana with pre-provisioned dashboards:

```bash
export SHELLY_PASSWORD=your-device-password   # for the "auth" module
docker compose -f development/compose.yml up -d --build
```

Before starting, set your device address in `prometheus.yml` (file binds don't
expand env vars).

- Exporter: http://localhost:8080 (probe: `/probe?target=<device-ip>`)
- Prometheus: http://localhost:9091
- Grafana: http://localhost:3001 (dashboards pre-provisioned in the "Shelly" folder)

## Config files

| File             | Mounted into                                            |
| ---------------- | ------------------------------------------------------- |
| `shelly.yml`     | exporter config (`/config.yml` in `shellydeviceexporter`) |
| `prometheus.yml` | Prometheus scrape config                                 |
| `grafana.yml`    | Grafana datasource + dashboard provisioning              |

Grafana only reloads provisioned dashboards on startup: `docker compose -f development/compose.yml restart grafana1` after editing a dashboard JSON.

## Shelly API Reference

- [Authentication](https://shelly-api-docs.shelly.cloud/gen2/General/Authentication/)

For debugging purposes, you can directly access your Shelly devices via curl:

```bash
# Get device info
curl 'http://YOUR_SHELLY_IP/shelly'

# Get device status using Digest Auth
curl --digest -u admin:"YOUR_PASSWORD" 'http://YOUR_SHELLY_IP/rpc/Shelly.GetStatus'
```
