# Multi-target exporter refactor

Issue: https://github.com/veerendra2/shelly_device_exporter/issues/29
Date: 2026-09-28

## Problem

The exporter lists every Shelly device in its own config and polls them on its own schedule. This follows the [multi-target exporter pattern](https://prometheus.io/docs/guides/multi-target-exporter/) incorrectly: the device list should live in prometheus.yml (or service discovery), Prometheus should drive scrape timing, and `instance` should be the device, not the exporter.

## Decision

Follow the multi-target pattern the way blackbox exporter does. Clean break: no support for the old config format.

- New `/probe` endpoint. Prometheus passes `target` (device address) and `module` as query params. The exporter scrapes one device per request and writes metrics to the response.
- The old device-list config and the internal worker pool are deleted. Prometheus owns scheduling and concurrency.
- Existing metric names and labels stay. `shelly_probe_success` and `shelly_probe_duration_seconds` are added, blackbox-style.
- Energy cost calculation is unchanged. It is a pure function of the scraped `aenergy_total` and the global `price_per_kwh` (internal/shelly/client.go:149), so it works in the stateless model as-is. Semantics stay "cost since device reboot".

## Config format

```yaml
price_per_kwh: 0.32
currency: EUR
modules:
  rack:
    username: admin
    password: ${SHELLY_RACK_PASSWORD}
```

- `modules` replaces `devices`. A module holds `username`, `password`, and optionally `password_file`. `password` and `password_file` together is a config error.
- `${ENV}` expansion applies to username, password, and password_file values (os.ExpandEnv).
- `password_file` is read at startup; its contents are trimmed of trailing whitespace.
- A built-in `default` module with no auth is used when the request has no `module` param. Users with one shared password define one named module; users with factory devices use the default.
- The YAML is parsed strictly, so the old `devices:` format fails at startup with a clear error.

Example Prometheus config:

```yaml
- job_name: shelly
  metrics_path: /probe
  params:
    module: [rack]
  static_configs:
    - targets: [192.168.1.100, 192.168.1.101]
  relabel_configs:
    - source_labels: [__address__]
      target_label: __param_target
    - source_labels: [__param_target]
      target_label: instance
    - target_label: __address__
      replacement: shelly-exporter:8080
```

For targets in different auth groups, one static_configs block per module with a `module` label relabeled into `__param_module`.

## Probe handler

New package `internal/probe`. `GET /probe?target=<addr>&module=<name>`:

1. Resolve the module. Missing target, or unknown module: HTTP 400 with a plain-text error, no metrics.
2. Timeout from the `X-Prometheus-Scrape-Timeout-Seconds` header, capped at 30 seconds; 30 if the header is missing or invalid.
3. Build one Shelly client for the target with the module's credentials, a fresh `prometheus.Registry`, and register the collector. Serve through `promhttp.HandlerFor`.
4. A small probe collector records `shelly_probe_success` (0 or 1) and `shelly_probe_duration_seconds` after the scrape attempt. Failed device scrape: `probe_success=0`, HTTP still 200, cause logged at WARN. This follows the blackbox convention so users can alert on probe failures separately from exporter health.
5. `/metrics` keeps serving exporter self-metrics only.

The name label now comes from the device's own status (`sys` in the GetStatus response) instead of the config file. Config `name` field is deleted.

## Code changes

- `internal/config`: `Device` becomes `Module`; add env expansion, `password_file` support, strict parsing, validation tests.
- `internal/shelly`: single-target client. `BulkStatus` and the worker pool are deleted; a `Status(ctx)` method returns one device's status. Digest auth comes from the module.
- `internal/collector`: constructor takes the client; collect logic otherwise unchanged.
- `internal/probe`: new handler package.
- `main.go`: register `/probe` and `/metrics`; keep existing flags.
- Docs: README config example, prometheus.yml snippet, compose-dev.yml, breaking-change note.

## Error handling

| Case | Result |
| ---- | ------ |
| Missing/unknown module, missing target | 400, no metrics |
| Device unreachable, auth failure, bad response | 200, `probe_success=0`, WARN log |
| Timeout header invalid or absent | cap of 30 seconds |
| Old config format | startup error |

## Testing

- Config: module parsing, env expansion, password/password_file conflict, strict rejection of old format, password_file reading.
- Probe: httptest with a fake Shelly RPC server. Success, device-down (probe_success=0, status 200), bad module/target (400), timeout header.
- Collector: adapt existing tests to the new constructor.

## Out of scope

Config hot-reload, RPC response caching, rate limiting, a status UI on the exporter, and metric redesign.
