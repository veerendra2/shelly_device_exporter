# Local Development Guide

## Local Testing

Set your device credentials:

```bash
export SHELLY_PASSWORD=your-password
```

To start only shelly-device-exporter:

```bash
docker compose up shelly-device-exporter -d --build
```

To start full stack:

```bash
docker compose -f up -d --build
```

Endpoints:

| Endpoint              | App                    |
| --------------------- | ---------------------- |
| http://localhost:8081 | shelly-device-exporter |
| http://localhost:9091 | Prometheus             |
| http://localhost:3001 | Grafana                |

Probe the exporter directly:

```bash
# Probe a device (no auth)
curl 'http://localhost:8081/probe?target=192.168.1.6'

# With credentials (the "auth" module)
curl 'http://localhost:8081/probe?target=192.168.1.6&module=auth'

# Simulate the Prometheus timeout header
curl -H 'X-Prometheus-Scrape-Timeout-Seconds: 10' 'http://localhost:8081/probe?target=192.168.1.6'
```

## Shelly API Reference

- [Authentication](https://shelly-api-docs.shelly.cloud/gen2/General/Authentication/)

For debugging, you can talk to your Shelly devices directly with curl:

```bash
# Get device info
curl 'http://YOUR_SHELLY_IP/shelly'

# Get device status using Digest Auth
curl --digest -u admin:"YOUR_PASSWORD" 'http://YOUR_SHELLY_IP/rpc/Shelly.GetStatus'
```

### Build & Test

- Using [Taskfile](https://taskfile.dev/)

_Install Taskfile: [Installation Guide](https://taskfile.dev/docs/installation)_

```bash
# Available tasks
task --list
task: Available tasks for this project:
* all:                   Run comprehensive checks: format, lint, security and test
* build:                 Build the application binary for the current platform
* build-docker:          Build Docker image
* build-platforms:       Build the application binaries for multiple platforms and architectures
* fmt:                   Formats all Go source files
* install:               Install required tools and dependencies
* lint:                  Run static analysis and code linting using golangci-lint
* run:                   Runs the main application
* security:              Run security vulnerability scan
* test:                  Runs all tests in the project      (aliases: tests)
* vet:                   Examines Go source code and reports suspicious constructs
```

- Build with [goreleaser](https://goreleaser.com/)

_Install GoReleaser: [Installation Guide](https://goreleaser.com/install/)_

```bash
# Build locally
goreleaser release --snapshot --clean
```
