# Shelly Device Exporter

<p align="center">
  <img src="./assets/shell-logo.png" width="90"/>
  <img src="./assets/prometheus-logo.png" width="90"/>
  <br>
</p>
<p align="center">Prometheus exporter for Shelly Gen 2+ devices.</p>

<p align="center">
  <a href="https://github.com/veerendra2/shelly-plug-exporter/actions"><img src="https://github.com/veerendra2/shelly-plug-exporter/workflows/CI/badge.svg" alt="Build Status"></a>
  <a href="https://goreportcard.com/report/github.com/veerendra2/shelly-plug-exporter"><img src="https://goreportcard.com/badge/github.com/veerendra2/shelly-plug-exporter" alt="Go Report Card"></a>
  <a href="https://github.com/veerendra2/shelly-plug-exporter/releases"><img src="https://img.shields.io/github/v/release/veerendra2/shelly-plug-exporter" alt="Release"></a>
  <a href="https://github.com/veerendra2/shelly-plug-exporter/blob/main/LICENSE"><img src="https://img.shields.io/github/license/veerendra2/shelly-plug-exporter" alt="License"></a>
</p>

## Features

| Feature                     | Description                                                                                                              |
| :-------------------------- | :----------------------------------------------------------------------------------------------------------------------- |
| **Concurrent Scraping**     | Uses a configurable worker pool to fetch statuses from multiple devices simultaneously without overwhelming the network. |
| **Authentication**          | Supports Shelly's required Digest Authentication out of the box.                                                         |
| **Energy Cost Calculation** | Automatically calculates ongoing energy costs based on configurable `price_per_kwh` and `currency` fields.               |

## Device Compatibility

_Note: Compatible with all Gen 2+ devices (Plus, Pro, Mini) utilizing the standard Shelly RPC API._

| Device                                                                                     | Tested |
| ------------------------------------------------------------------------------------------ | ------ |
| [Shelly Plug M Gen3](https://shelly-api-docs.shelly.cloud/gen2/Devices/Gen3/ShellyPlugMG3) | ✅     |

## Exported Metrics

| Component                                                                        | Status |
| -------------------------------------------------------------------------------- | ------ |
| [Switch](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Switch) | ✅     |
| [System](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Sys)    | ✅     |

_See [Metrics](#metrics)_

## Usage

```bash
Usage: shelly_device_exporter [flags]

Prometheus exporter for Shelly Gen 2+ devices.

Flags:
  -h, --help                    Show context-sensitive help.
      --address=":8080"         The address where the server should listen on ($ADDRESS).
      --config="config.yml"     Configuration file path ($CONFIG_FILE)
      --log-format="console"    Set the output format of the logs. Must be "console" or "json" ($LOG_FORMAT).
      --log-level=INFO          Set the log level. Must be "DEBUG", "INFO", "WARN" or "ERROR" ($LOG_LEVEL).
      --log-add-source          Whether to add source file and line number to log records ($LOG_ADD_SOURCE).
      --version                 Print version information and exit
```

## Configuration

The exporter requires a configuration file to know which devices to poll and how to connect to them.

```yaml
---
# (Optional) Used to calculate the 'shelly_device_energy_cost_total' metric.
# Both fields must be set to enable cost calculation.
price_per_kwh: 0.10
currency: "EUR"

# (Optional) Controls the worker pool size for parallel requests. Default is 4.
max_concurrent_device_connections: 10

# List of Shelly devices to monitor
devices:
  - name: "home-plug"
    address: "http://SHELLY_DEVICE_IP"
    username: "admin" # Optional, defaults to "admin"
    password: "YOUR_PASSWORD"
```

## Prometheus Scrape Configuration

Add the following to your `prometheus.yml` scrape configurations to collect metrics from the exporter:

```yaml
scrape_configs:
  - job_name: "shelly_devices"
    # Adjust scrape interval based on your needs
    scrape_interval: 15s
    static_configs:
      - targets: ["localhost:8080"] # Replace with the exporter's address
```

## Docker

You can run the exporter easily using Docker. Make sure to mount your `config.yml` file into the container.

### Docker Run

```bash
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/config.yml:/app/config.yml:ro \
  ghcr.io/veerendra2/shelly-plug-exporter:latest
```

### Docker Compose

```yaml
version: "3.8"
services:
  shelly-exporter:
    image: ghcr.io/veerendra2/shelly-plug-exporter:latest
    ports:
      - "8080:8080"
    volumes:
      - ./config.yml:/app/config.yml:ro
    restart: unless-stopped
```

## Build & Test

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

## Shelly API Reference

For debugging purposes, you can directly access your Shelly devices via curl:

```bash
# Get device info
curl 'http://YOUR_SHELLY_IP/shelly'

# Get device status using Digest Auth
curl --digest -u admin:"YOUR_PASSWORD" 'http://YOUR_SHELLY_IP/rpc/Shelly.GetStatus'
```

---

## Metrics

```text
# HELP shelly_device_apower Last measured instantaneous active power (in Watts) delivered to the attached load.
# TYPE shelly_device_apower gauge
shelly_device_apower{name="home"} 15.5
# HELP shelly_device_current Last measured current in Amperes.
# TYPE shelly_device_current gauge
shelly_device_current{name="home"} 0.123
# HELP shelly_device_energy_cost_total Total energy cost total.
# TYPE shelly_device_energy_cost_total gauge
shelly_device_energy_cost_total{currency="EUR",name="home"} 0.9173950000000002
# HELP shelly_device_freq Last measured network frequency in Hz.
# TYPE shelly_device_freq gauge
shelly_device_freq{name="home"} 50
# HELP shelly_device_fs_free Size of the free file system in Bytes.
# TYPE shelly_device_fs_free gauge
shelly_device_fs_free{name="home"} 458752
# HELP shelly_device_fs_size Total size of the file system in Bytes.
# TYPE shelly_device_fs_size gauge
shelly_device_fs_size{name="home"} 917504
# HELP shelly_device_ram_free Size of the free RAM in the system in Bytes.
# TYPE shelly_device_ram_free gauge
shelly_device_ram_free{name="home"} 105692
# HELP shelly_device_ram_size Total size of the RAM in the system in Bytes.
# TYPE shelly_device_ram_size gauge
shelly_device_ram_size{name="home"} 260676
# HELP shelly_device_restart_required True if restart is required, false otherwise.
# TYPE shelly_device_restart_required gauge
shelly_device_restart_required{name="home"} 0
# HELP shelly_device_sys_mac Mac address of the device.
# TYPE shelly_device_sys_mac gauge
shelly_device_sys_mac{mac="0892724CCD80",name="home"} 0
# HELP shelly_device_temperature_celsius Temperature in Celsius.
# TYPE shelly_device_temperature_celsius gauge
shelly_device_temperature_celsius{name="home"} 38.5
# HELP shelly_device_uptime Time in seconds since last reboot.
# TYPE shelly_device_uptime gauge
shelly_device_uptime{name="home"} 1.987289e+06
# HELP shelly_device_voltage Last measured voltage in Volts.
# TYPE shelly_device_voltage gauge
shelly_device_voltage{name="home"} 237.2
```
