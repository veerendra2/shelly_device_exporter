# AGENTS.md

This file provides guidance to Claude Code (claude.ai/code) and other coding
agents when working with code in this repository.

## Commands

```bash
task all   # format, lint, security, and test — run before every commit
```

Or individually:

```bash
task fmt    # gofmt
task lint   # golangci-lint
task vet    # go vet
task test   # go test ./...
go test ./internal/probe/...          # one package
go test ./internal/config/... -run TestConfig -v   # closer: pick a spec via ginkgo's -focus
task run                              # run the exporter locally
```

A commit that fails any of these is not done. Fix lint issues in the same commit
as the change that caused them.

## Documentation

When writing or editing user-facing docs (README, guides, wiki), use the
`humanizer` skill if available. If it is not installed, suggest installing it
first: https://github.com/blader/humanizer

## Architecture

Prometheus multi-target exporter (blackbox convention). The exporter holds no
device list: Prometheus passes each device per request via
`/probe?target=<addr>&module=<name>`, and one scrape never shares state with
another.

Request flow (`main.go` → `internal/`):

1. `internal/config` — parses `config.yml`: auth modules
   (`password`/`password_file`, mutually exclusive, `{{ env "VAR" }}`
   templating), global `price_per_kwh`/`currency`, strict YAML unmarshal
   (unknown fields fail startup). A built-in `default` module (username admin,
   no password) always exists.
2. `internal/probe` — the HTTP handler. Derives the device deadline from
   `X-Prometheus-Scrape-Timeout-Seconds` (capped at 30s, 1s margin reserved),
   scrapes the device, then serves metrics from a **per-request**
   `prometheus.NewRegistry()`. Device failure is still HTTP 200 with
   `shelly_probe_success=0`; only bad params (missing target, unknown module)
   are 400.
3. `internal/shelly` — one client per probe. `Status()` hits `Shelly.GetStatus`;
   digest auth via `github.com/icholy/digest`.
4. `internal/collector` — stateless formatter: converts a pre-fetched
   `*shelly.StatusResponse` into metrics. Cost is computed in-collector as
   `(aenergy_total Wh / 1000) × price_per_kwh`. Metrics are registered under
   the `shelly_device_` prefix; names are stable.

Key invariants when changing this code:

- **Timeouts**: `main.go` WriteTimeout (35s) must exceed the probe's 30s cap, or
  `shelly_probe_success 0` never reaches Prometheus for slow devices.
- **Keep `/probe` side-effect free and per-request**: nothing global except
  loaded config; `collector.Exporter` is constructed fresh each probe.
- **`shelly_probe_success`**: keep 200-on-device-failure semantics; Prometheus
  alerts and the dashboards key on it.
- **Dashboards** (`assets/*.json`, Grafana schema v2, Prometheus +
  VictoriaMetrics variants) label everything by `name`; renaming metrics or
  changing the `name`/`currency` labels is a breaking change for both.
- The exporter emits no `name` label. Prometheus supplies it from
  `static_configs` labels — do not reintroduce a device name lookup
  (`Shelly.GetConfig`) without a reason; it costs an extra request per probe.

## Local development

`development/` has a docker compose stack (exporter + Prometheus + Grafana with
provisioned dashboards) and a README with manual curl recipes. See its README.
