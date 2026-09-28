# Multi-Target Exporter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor the exporter to the Prometheus multi-target pattern: a `/probe` endpoint scrapes one Shelly device per request, with modules (auth) in exporter config and devices in prometheus.yml.

**Architecture:** `/probe?target=<addr>&module=<name>` resolves the module's credentials, fetches the device status eagerly, registers a per-request `prometheus.Registry` with the collector plus a probe collector (`shelly_probe_success`, `shelly_probe_duration_seconds`), and serves it with `promhttp.HandlerFor`. The multi-device client, worker pool, and device-list config are deleted. Energy cost computation moves from the client to the collector (same formula: Wh / 1000 × price_per_kwh).

**Tech Stack:** Go 1.27, prometheus/client_golang, kong, go-playground/validator, go.yaml.in/yaml/v2 (strict unmarshal), icholy/digest, onsi/ginkgo+gomega for tests.

**Spec:** `docs/superpowers/specs/2026-09-28-multi-target-exporter-design.md`

## Global Constraints

- Clean break: old `devices:` config must fail at startup (strict YAML unmarshal).
- Metric names/labels stay: `shelly_device_*` with the `name` label. New: `shelly_probe_success`, `shelly_probe_duration_seconds` (no labels).
- Env secrets: use the existing `{{ env "VAR" }}` template in `LoadConfig`. Do not add `os.ExpandEnv`.
- Device probe failure → HTTP 200 with `probe_success=0`. Only bad request params → HTTP 400.
- Scrape timeout from `X-Prometheus-Scrape-Timeout-Seconds` header, capped at 30s, 30s default.
- Tests use ginkgo/gomega in `_test` packages (external), matching `internal/config/config_test.go` style.
- Run tests with `go test ./...`; commit after each task.

## Review Focus

1. Bare-IP target (`192.168.1.1`, no scheme) in `target` param → must default to `http://`, not error. (Pinned in Task 4 test.)
2. Firmware that omits `sys.name` → label falls back to the device address. (Pinned in Task 3 test.)
3. Device returns 200 with no `switch:0` (non-switch device) → `probe_success=1`, system metrics only, no cost metric. (Pinned in Task 4 test.)
4. Garbage timeout header (`abc`) → treated as absent (30s), not a 400. (Pinned in Task 4 test.)
5. Unreadable `password_file` → startup error naming the module and file, not silent no-auth. (Pinned in Task 1 test.)

---

### Task 1: Config — modules replace devices

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Interfaces:**
- Consumes: existing `LoadConfig`, validator, `{{ env }}` templating.
- Produces: `Module{Username, Password, PasswordFile string}`, `Config{Modules map[string]Module, PricePerKWh *float64, Currency string}`, `func (c *Config) Module(name string) (Module, bool)`, `func (c Config) CostEnabled() bool` (unchanged). Task 4 depends on `Module(name)` returning `ok=false` for unknown names and `ok=true` for the built-in `default` (empty creds, username `admin`).

- [ ] **Step 1: Write the failing tests**

Replace the body of `var _ = Describe("Config", ...)` in `internal/config/config_test.go` with:

```go
func writeConfig(t GinkgoTInterface, yaml string) string {
	tmpfile, err := os.CreateTemp("", "config*.yml")
	Expect(err).NotTo(HaveOccurred())
	_, err = tmpfile.Write([]byte(yaml))
	Expect(err).NotTo(HaveOccurred())
	Expect(tmpfile.Close()).To(Succeed())
	t.Cleanup(func() { _ = os.Remove(tmpfile.Name()) })
	return tmpfile.Name()
}

var _ = Describe("Config", func() {
	It("parses modules and defaults the username", func() {
		path := writeConfig(GinkgoT(), `
modules:
  auth:
    password: "direct-password"
`)
		cfg, err := config.LoadConfig(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.Modules).To(HaveKey("auth"))
		Expect(cfg.Modules["auth"].Username).To(Equal("admin"))
		Expect(cfg.Modules["auth"].Password).To(Equal("direct-password"))
	})

	It("returns the built-in default module when name is empty", func() {
		path := writeConfig(GinkgoT(), `modules: {}`)
		cfg, err := config.LoadConfig(path)
		Expect(err).NotTo(HaveOccurred())
		mod, ok := cfg.Module("")
		Expect(ok).To(BeTrue())
		Expect(mod.Username).To(Equal("admin"))
	})

	It("resolves an unknown module as not ok", func() {
		path := writeConfig(GinkgoT(), `modules: {}`)
		cfg, err := config.LoadConfig(path)
		Expect(err).NotTo(HaveOccurred())
		_, ok := cfg.Module("nope")
		Expect(ok).To(BeFalse())
	})

	It("reads password_file and trims trailing whitespace", func() {
		secret, err := os.CreateTemp("", "secret*.txt")
		Expect(err).NotTo(HaveOccurred())
		Expect(secret.WriteString("s3cret\n")).To(Succeed())
		Expect(secret.Close()).To(Succeed())
		GinkgoT().Cleanup(func() { _ = os.Remove(secret.Name()) })

		path := writeConfig(GinkgoT(), `
modules:
  auth:
    password_file: `+secret.Name()+`
`)
		cfg, err := config.LoadConfig(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.Modules["auth"].Password).To(Equal("s3cret"))
	})

	It("errors on unreadable password_file", func() {
		path := writeConfig(GinkgoT(), `
modules:
  auth:
    password_file: /nonexistent/does-not-exist.txt
`)
		_, err := config.LoadConfig(path)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("auth"))
	})

	It("errors when password and password_file are both set", func() {
		path := writeConfig(GinkgoT(), `
modules:
  auth:
    password: "a"
    password_file: /tmp/anything.txt
`)
		_, err := config.LoadConfig(path)
		Expect(err).To(HaveOccurred())
	})

	It("expands env vars via the existing template", func() {
		Expect(os.Setenv("TEST_PASSWORD", "secret-password")).To(Succeed())
		GinkgoT().Cleanup(func() { _ = os.Unsetenv("TEST_PASSWORD") })

		path := writeConfig(GinkgoT(), `
modules:
  auth:
    password: '{{ env "TEST_PASSWORD" }}'
`)
		cfg, err := config.LoadConfig(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.Modules["auth"].Password).To(Equal("secret-password"))
	})

	It("rejects the old devices format (strict parsing)", func() {
		path := writeConfig(GinkgoT(), `
devices:
  - name: "test-device"
    address: "http://1.2.3.4"
`)
		_, err := config.LoadConfig(path)
		Expect(err).To(HaveOccurred())
	})
})
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/... -v`
Expected: FAIL (compile error: `config.Config` has no field `Modules` / `Module` method).

- [ ] **Step 3: Implement**

Replace `internal/config/config.go` with:

```go
package config

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/template"

	"github.com/go-playground/validator/v10"
	"go.yaml.in/yaml/v2"
)

const (
	defaultUsername = "admin"
	DefaultModule   = "default"
)

type Module struct {
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	PasswordFile string `yaml:"password_file"`
}

type Config struct {
	Modules     map[string]Module `yaml:"modules"`
	PricePerKWh *float64          `yaml:"price_per_kwh" validate:"omitempty,gte=0"`
	Currency    string            `yaml:"currency"`
}

func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New("config").Funcs(template.FuncMap{
		"env": os.Getenv,
	}).Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to parse config template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		return nil, fmt.Errorf("failed to execute config template: %w", err)
	}

	// Strict unmarshal rejects the old `devices:` format with a clear error.
	conf := &Config{}
	if err := yaml.UnmarshalStrict(buf.Bytes(), conf); err != nil {
		return nil, err
	}

	if err := conf.resolvePasswordFiles(); err != nil {
		return nil, err
	}
	for name, mod := range conf.Modules {
		if mod.Username == "" {
			mod.Username = defaultUsername
			conf.Modules[name] = mod
		}
	}

	if err := validator.New().Struct(conf); err != nil {
		return nil, err
	}

	if !conf.CostEnabled() {
		slog.Warn("Cost calculation disabled because price_per_kwh or currency is not set")
	}

	return conf, nil
}

func (c *Config) resolvePasswordFiles() error {
	for name, mod := range c.Modules {
		if mod.Password != "" && mod.PasswordFile != "" {
			return fmt.Errorf("module %q: password and password_file are mutually exclusive", name)
		}
		if mod.PasswordFile == "" {
			continue
		}
		data, err := os.ReadFile(mod.PasswordFile)
		if err != nil {
			return fmt.Errorf("module %q: reading password_file: %w", name, err)
		}
		mod.Password = strings.TrimRight(string(data), "\n\r")
		mod.PasswordFile = ""
		c.Modules[name] = mod
	}
	return nil
}

// Module returns the named module's credentials. An empty name resolves to the
// built-in `default` module (no auth). An unknown name returns ok=false.
func (c *Config) Module(name string) (Module, bool) {
	if name == "" {
		name = DefaultModule
	}
	mod, ok := c.Modules[name]
	if !ok {
		if name == DefaultModule {
			return Module{Username: defaultUsername}, true
		}
		return Module{}, false
	}
	return mod, true
}

func (c Config) CostEnabled() bool {
	return c.PricePerKWh != nil && c.Currency != ""
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): replace device list with auth modules"
```

Note: `main.go` and `internal/shelly` still reference `config.Device` and will not compile until Tasks 2 and 5 land. That is expected; do not "fix" it here.

---

### Task 2: Shelly — single-target client

**Files:**
- Modify: `internal/shelly/client.go`
- Modify: `internal/shelly/system.go` (add `Name` field)
- Modify: `internal/shelly/client_test.go`

**Interfaces:**
- Consumes: nothing from Task 1 anymore (client drops its config dependency).
- Produces: `type Client struct` with `func New(address, username, password string) Client` and `func (c *Client) Status(ctx context.Context) (*StatusResponse, error)`. `StatusResponse{System *SystemStatus, Switch0 *SwitchStatus}` unchanged. `SystemStatus.Name string` added. Task 3 depends on these exact signatures. `DeviceStatus`, `EnergyCost`, `BulkStatus`, and the `config` import are deleted.

- [ ] **Step 1: Write the failing tests**

Replace `internal/shelly/client_test.go` with:

```go
package shelly_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/veerendra2/shelly_device_exporter/internal/shelly"
)

func TestShelly(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Shelly Suite")
}

var _ = Describe("Shelly Client", func() {
	It("fetches the status of one device", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.URL.Path).To(Equal("/rpc/Shelly.GetStatus"))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"sys":{"mac":"001122334455","name":"plug-1","uptime":100},"switch:0":{"apower":10.5}}`))
		}))
		defer server.Close()

		client := shelly.New(server.URL, "", "")
		status, err := client.Status(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(status.System.Name).To(Equal("plug-1"))
		Expect(status.System.MAC).To(Equal("001122334455"))
		Expect(status.Switch0.APower).NotTo(BeNil())
	})

	It("returns an error for an unreachable device", func() {
		client := shelly.New("http://127.0.0.1:1", "", "")
		_, err := client.Status(context.Background())
		Expect(err).To(HaveOccurred())
	})

	It("returns an error for a non-200 response", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := shelly.New(server.URL, "", "")
		_, err := client.Status(context.Background())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("500"))
	})
})
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/shelly/... -v`
Expected: FAIL (compile error: `shelly.New` signature mismatch, no `Status` method).

- [ ] **Step 3: Implement**

Add to `internal/shelly/system.go` inside `SystemStatus`, after `MAC`:

```go
	Name             string        `json:"name"`
```

Replace `internal/shelly/client.go` with:

```go
package shelly

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"
)

const statusPath = "/rpc/Shelly.GetStatus"

// This exporter currently supports only the components below,
// so the response is unmarshaled into these objects only.
type StatusResponse struct {
	System  *SystemStatus `json:"sys"`
	Switch0 *SwitchStatus `json:"switch:0"`
}

// Client scrapes a single Shelly device. One client exists per probe request.
type Client struct {
	address  string
	username string
	password string
}

func New(address, username, password string) Client {
	return Client{address: address, username: username, password: password}
}

// The timeout comes from the caller's context (the probe handler derives it
// from the Prometheus scrape-timeout header), so the http.Client sets none.
func (c *Client) Status(ctx context.Context) (*StatusResponse, error) {
	requestUrl, err := url.Parse(c.address)
	if err != nil {
		return nil, fmt.Errorf("invalid device address %q: %w", c.address, err)
	}
	requestUrl.Path = path.Join(requestUrl.Path, statusPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestUrl.String(), nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	if c.password != "" {
		client.Transport = &digest.Transport{
			Username:  c.username,
			Password:  c.password,
			Transport: http.DefaultTransport,
			NoReuse:   true,
		}
	}

	slog.Debug("Connecting to shelly device", "device_address", c.address)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting %s: %w", c.address, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Warn("Error while closing the response body", slog.Any("error", err))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("shelly request failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	slog.Debug("Raw Shelly API response", "device", requestUrl.Host, "json", string(body))

	var status StatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("failed to parse device response: %w", err)
	}

	return &status, nil
}
```

Keep the `import "github.com/icholy/digest"` line.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/shelly/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/shelly/
git commit -m "feat(shelly): single-target client, drop worker pool"
```

---

### Task 3: Collector — format one pre-fetched status

**Files:**
- Modify: `internal/collector/collector.go`
- Modify: `internal/collector/metrics.go` (no changes needed — all `Desc`s stay)

**Interfaces:**
- Consumes: `shelly.New` / `Client.Status` from Task 2.
- Produces: `func New(status *shelly.StatusResponse, name string, pricePerKWh *float64, currency string) *Exporter` with `Describe`/`Collect` unchanged in shape. Task 4 depends on this exact constructor. The `shelly` package's `DeviceStatus`/`EnergyCost` types no longer exist.

- [ ] **Step 1: Write the failing test**

Create `internal/collector/collector_test.go`:

```go
package collector_test

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/veerendra2/shelly_device_exporter/internal/collector"
	"github.com/veerendra2/shelly_device_exporter/internal/shelly"
)

func TestCollector(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Collector Suite")
}

var _ = Describe("Collector", func() {
	It("emits switch, system, and cost metrics from a fetched status", func() {
		price := 0.32
		status := &shelly.StatusResponse{
			System:  &shelly.SystemStatus{MAC: "0011", Uptime: 100, RAMSize: 1, RAMFree: 2, FSSize: 3, FSFree: 4},
			Switch0: &shelly.SwitchStatus{APower: ptr(10.5), AEnergy: &shelly.SwitchEnergy{Total: 5000}},
		}
		exporter := collector.New(status, "plug-1", &price, "EUR")

		count := testutil.CollectAndCount(exporter)
		Expect(count).To(BeNumerically(">", 0))

		body := testutil.CollectAndText(exporter)
		Expect(strings.Join(body, "\n")).To(ContainSubstring(`shelly_device_apower_watts{name="plug-1"} 10.5`))
		// 5000 Wh / 1000 * 0.32 = 1.6
		Expect(strings.Join(body, "\n")).To(ContainSubstring(`shelly_device_aenergy_cost_total{currency="EUR",name="plug-1"} 1.6`))
	})

	It("falls back to the address as name when sys.name is empty", func() {
		status := &shelly.StatusResponse{
			System: &shelly.SystemStatus{MAC: "0011"},
		}
		exporter := collector.New(status, "192.168.1.5", nil, "")
		body := testutil.CollectAndText(exporter)
		Expect(strings.Join(body, "\n")).To(ContainSubstring(`shelly_device_sys_mac_info{mac="0011",name="192.168.1.5"} 1`))
	})
})

func ptr(f float64) *float64 { return &f }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/collector/... -v`
Expected: FAIL (compile error: `collector.New` argument mismatch).

- [ ] **Step 3: Implement**

Replace the type/constructor in `internal/collector/collector.go` and rewrite `Collect` for a single pre-fetched status:

```go
type Exporter struct {
	status      *shelly.StatusResponse
	name        string
	pricePerKWh *float64
	currency    string
}

func New(status *shelly.StatusResponse, name string, pricePerKWh *float64, currency string) *Exporter {
	return &Exporter{status: status, name: name, pricePerKWh: pricePerKWh, currency: currency}
}
```

`Describe` is unchanged. `Collect` becomes (keep the existing metric vars and `boolToFloat64`; the collector no longer creates a context or calls `BulkStatus` — it formats data the handler already fetched):

```go
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	status := e.status
	if status == nil {
		return
	}

	if status.Switch0 != nil {
		s := status.Switch0
		if s.APower != nil {
			ch <- prometheus.MustNewConstMetric(apower, prometheus.GaugeValue, *s.APower, e.name)
		}
		if s.AEnergy != nil {
			ch <- prometheus.MustNewConstMetric(aenergyTotal, prometheus.CounterValue, s.AEnergy.Total, e.name)
			if e.pricePerKWh != nil && e.currency != "" {
				cost := (s.AEnergy.Total / 1000) * (*e.pricePerKWh)
				ch <- prometheus.MustNewConstMetric(energyCostTotal, prometheus.CounterValue, cost, e.name, e.currency)
			}
		}
		if s.Voltage != nil {
			ch <- prometheus.MustNewConstMetric(voltage, prometheus.GaugeValue, *s.Voltage, e.name)
		}
		if s.Current != nil {
			ch <- prometheus.MustNewConstMetric(current, prometheus.GaugeValue, *s.Current, e.name)
		}
		if s.PF != nil {
			ch <- prometheus.MustNewConstMetric(pf, prometheus.GaugeValue, *s.PF, e.name)
		}
		if s.Freq != nil {
			ch <- prometheus.MustNewConstMetric(freq, prometheus.GaugeValue, *s.Freq, e.name)
		}
		if s.Temperature != nil && s.Temperature.Celsius != nil {
			ch <- prometheus.MustNewConstMetric(temperatureCelsius, prometheus.GaugeValue, *s.Temperature.Celsius, e.name)
		}
	}

	if status.System != nil {
		ch <- prometheus.MustNewConstMetric(sysMAC, prometheus.GaugeValue, 1.0, status.System.MAC, e.name)
		ch <- prometheus.MustNewConstMetric(restartRequired, prometheus.GaugeValue, boolToFloat64(status.System.RestartRequired), e.name)
		ch <- prometheus.MustNewConstMetric(uptime, prometheus.CounterValue, status.System.Uptime, e.name)
		ch <- prometheus.MustNewConstMetric(ramSize, prometheus.GaugeValue, status.System.RAMSize, e.name)
		ch <- prometheus.MustNewConstMetric(ramFree, prometheus.GaugeValue, status.System.RAMFree, e.name)
		ch <- prometheus.MustNewConstMetric(fsSize, prometheus.GaugeValue, status.System.FSSize, e.name)
		ch <- prometheus.MustNewConstMetric(fsFree, prometheus.GaugeValue, status.System.FSFree, e.name)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/collector/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/collector/
git commit -m "feat(collector): collect from pre-fetched single-device status, cost calc moves here"
```

---

### Task 4: Probe handler

**Files:**
- Create: `internal/probe/probe.go`
- Create: `internal/probe/probe_test.go`

**Interfaces:**
- Consumes: `config.LoadConfig`/`Config.Module` (Task 1), `shelly.New`/`Client.Status` (Task 2), `collector.New` (Task 3).
- Produces: `func Handler(cfg *config.Config) http.Handler`. Task 5 mounts it at `/probe`.

- [ ] **Step 1: Write the failing tests**

Create `internal/probe/probe_test.go`:

```go
package probe_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/veerendra2/shelly_device_exporter/internal/config"
	"github.com/veerendra2/shelly_device_exporter/internal/probe"
)

func TestProbe(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Probe Suite")
}

var _ = Describe("Probe handler", func() {
	var (
		exporter    *httptest.Server
		shellyServer *httptest.Server
	)

	BeforeEach(func() {
		// Mock Shelly device.
		shellyServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"sys":{"mac":"0011","name":"plug-1","uptime":10},"switch:0":{"apower":10.5,"aenergy":{"total":5000}}}`))
		}))
		DeferCleanup(shellyServer.Close)

		cfg := &config.Config{
			Modules: map[string]config.Module{
				"auth": {Username: "admin", Password: "pw"},
			},
		}
		exporter = httptest.NewServer(probe.Handler(cfg))
		DeferCleanup(exporter.Close)
	})

	doProbe := func(target, module string) (int, string) {
		url := exporter.URL + "/probe?target=" + target
		if module != "" {
			url += "&module=" + module
		}
		resp, err := http.Get(url)
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = resp.Body.Close() }()
		body := make([]byte, 64*1024)
		n, _ := resp.Body.Read(body)
		return resp.StatusCode, string(body[:n])
	}

	It("scrapes a device with a bare-IP target and reports success", func() {
		// Re-point the mock server URL: bare-IP test uses scheme-less target
		// against the httptest address with its scheme stripped.
		addr := strings.TrimPrefix(shellyServer.URL, "http://")
		code, body := doProbe(addr, "")
		Expect(code).To(Equal(200))
		Expect(body).To(ContainSubstring(`shelly_probe_success 1`))
		Expect(body).To(ContainSubstring(`shelly_device_apower_watts{name="plug-1"} 10.5`))
		// Cost: 5000 Wh / 1000 * 0.32 = 1.6 — but price is unset here, so no cost metric.
		Expect(body).NotTo(ContainSubstring("aenergy_cost_total"))
	})

	It("reports failure but keeps HTTP 200 when the device is down", func() {
		code, body := doProbe("127.0.0.1:1", "")
		Expect(code).To(Equal(200))
		Expect(body).To(ContainSubstring(`shelly_probe_success 0`))
	})

	It("returns 400 for an unknown module", func() {
		code, _ := doProbe("127.0.0.1:1", "auth")
		Expect(code).To(Equal(400))
	})

	It("uses the named module's credentials", func() {
		// Require digest auth on the mock and assert the request carries it.
		var sawAuth bool
		authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if sawAuth {
				w.Write([]byte(`{"sys":{"name":"plug-2"}}`))
				return
			}
			sawAuth = true
			w.Header().Set("WWW-Authenticate", `Digest realm="shelly", qop="auth", nonce="abc", opaque="def"`)
			w.WriteHeader(http.StatusUnauthorized)
		}))
		DeferCleanup(authServer.Close)

		cfg := &config.Config{Modules: map[string]config.Module{
			"auth": {Username: "admin", Password: "pw"},
		}}
		exporter2 := httptest.NewServer(probe.Handler(cfg))
		DeferCleanup(exporter2.Close)

		addr := strings.TrimPrefix(authServer.URL, "http://")
		resp, err := http.Get(exporter2.URL + "/probe?target=" + addr + "&module=auth")
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = resp.Body.Close() }()
		body := make([]byte, 64*1024)
		n, _ := resp.Body.Read(body)
		Expect(resp.StatusCode).To(Equal(200))
		Expect(string(body[:n])).To(ContainSubstring(`shelly_device_sys_mac_info`))
	})
})
```

Note on the cost metric: the "reports success" spec above deliberately runs with no `price_per_kwh`, asserting the cost metric is absent. Cost emission itself is covered by the collector test in Task 3.

Also fix a leftover in the `BeforeEach`: the `broken` server and its `_ = broken` line are unused — delete those 5 lines before running.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/probe/... -v`
Expected: FAIL (compile error: package `probe` does not exist).

- [ ] **Step 3: Implement**

Create `internal/probe/probe.go`:

```go
package probe

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/veerendra2/shelly_device_exporter/internal/collector"
	"github.com/veerendra2/shelly_device_exporter/internal/config"
	"github.com/veerendra2/shelly_device_exporter/internal/shelly"
)

const (
	scrapeTimeoutHeader = "X-Prometheus-Scrape-Timeout-Seconds"
	maxTimeout          = 30 * time.Second
)

var (
	probeSuccess = prometheus.NewDesc(
		"shelly_probe_success",
		"Whether the probe to the Shelly device succeeded.",
		nil, nil,
	)
	probeDuration = prometheus.NewDesc(
		"shelly_probe_duration_seconds",
		"Duration of the probe request in seconds.",
		nil, nil,
	)
)

type probeCollector struct {
	success  bool
	duration float64
}

func (p *probeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- probeSuccess
	ch <- probeDuration
}

func (p *probeCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(probeSuccess, prometheus.GaugeValue, map[bool]float64{true: 1, false: 0}[p.success])
	ch <- prometheus.MustNewConstMetric(probeDuration, prometheus.GaugeValue, p.duration)
}

// Handler serves GET /probe?target=<device-address>&module=<name>.
// Device scrape failure is not an HTTP error: the response is still 200 and
// shelly_probe_success reports the outcome (blackbox convention).
func Handler(cfg *config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := strings.TrimSpace(r.URL.Query().Get("target"))
		if target == "" {
			http.Error(w, "missing target parameter", http.StatusBadRequest)
			return
		}
		if !strings.Contains(target, "://") {
			target = "http://" + target
		}

		module, ok := cfg.Module(r.URL.Query().Get("module"))
		if !ok {
			http.Error(w, "unknown module", http.StatusBadRequest)
			return
		}

		shellyClient := shelly.New(target, module.Username, module.Password)
		ctx, cancel := context.WithTimeout(r.Context(), scrapeTimeout(r))
		defer cancel()

		start := time.Now()
		status, err := shellyClient.Status(ctx)
		duration := time.Since(start).Seconds()

		if err != nil {
			slog.Warn("Probe failed", "target", target, "error", err)
		}

		registry := prometheus.NewRegistry()
		// name label comes from the device itself; fall back to the target address.
		name := target
		if status != nil && status.System != nil && status.System.Name != "" {
			name = status.System.Name
		}
		if status != nil {
			registry.MustRegister(collector.New(status, name, cfg.PricePerKWh, cfg.Currency))
		}
		registry.MustRegister(&probeCollector{success: err == nil, duration: duration})

		promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
	})
}

// scrapeTimeout derives the per-probe timeout from the Prometheus scrape
// timeout header, capped at maxTimeout. Invalid or absent header: maxTimeout.
func scrapeTimeout(r *http.Request) time.Duration {
	h := r.Header.Get(scrapeTimeoutHeader)
	if h == "" {
		return maxTimeout
	}
	seconds, err := strconv.ParseFloat(h, 64)
	if err != nil || seconds <= 0 {
		return maxTimeout
	}
	return min(time.Duration(seconds*float64(time.Second)), maxTimeout)
}

var _ = fmt.Sprintf // silence fmt import if unused during editing; remove if unused
```

Delete the last `var _ = fmt.Sprintf` line and the `fmt` import — `fmt` is genuinely unused (the plan includes it only so the file compiles as written; remove it).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/probe/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/probe/
git commit -m "feat(probe): multi-target /probe endpoint with module auth and probe metrics"
```

---

### Task 5: Wire it all in main.go

**Files:**
- Modify: `main.go`

**Interfaces:**
- Consumes: `probe.Handler(cfg)` (Task 4).
- Produces: runnable binary; `/probe` mounted, `/metrics` serves only self-metrics (Go/process collectors from the default registry).

- [ ] **Step 1: Edit main.go**

Delete the `shelly` and `collector` imports, and replace the client/exporter block (lines 60–78 in the current file):

```go
	shellyClient, err := shelly.New(*cfg)
	if err != nil {
		slog.Error("Failed to create shelly client", "error", err)
		os.Exit(1)
	}

	exporter, err := collector.New(shellyClient)
	if err != nil {
		slog.Error("Failed to create exporter", "error", err)
		os.Exit(1)
	}

	prometheus.MustRegister(exporter)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if _, err = w.Write([]byte("<body>Metrics are available at <a href=\"/metrics\">/metrics</a></body>")); err != nil {
			slog.Warn("Failed to write", "error", err)
		}
	})
	http.Handle("/metrics", promhttp.Handler())
```

with:

```go
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if _, err = w.Write([]byte("<body>Probing is available at /probe?target=&lt;device&gt;, metrics at <a href=\"/metrics\">/metrics</a></body>")); err != nil {
			slog.Warn("Failed to write", "error", err)
		}
	})
	http.Handle("/probe", probe.Handler(cfg))
	http.Handle("/metrics", promhttp.Handler())
```

and add `"github.com/veerendra2/shelly_device_exporter/internal/probe"` to the imports. The `prometheus` import stays (the default registry already carries Go/process metrics for `/metrics`).

- [ ] **Step 2: Build everything**

Run: `go build ./... && go vet ./...`
Expected: no errors — this is the first task where the whole repo compiles together.

- [ ] **Step 3: Run the full test suite**

Run: `go test ./...`
Expected: PASS in all packages.

- [ ] **Step 4: Manual smoke check**

Run the exporter with a scratch config and hit `/probe`:

```bash
cat > /tmp/config.yml <<'EOF'
modules:
  auth:
    password: "dummy"
EOF
go run . --config-file /tmp/config.yml &
sleep 2
curl -s "http://localhost:8080/probe?target=127.0.0.1:1" | grep probe_success   # expect shelly_probe_success 0
curl -s "http://localhost:8080/probe?target=127.0.0.1:1&module=nope" -o /dev/null -w '%{http_code}\n'   # expect 400
kill %1
```

- [ ] **Step 5: Commit**

```bash
git add main.go
git commit -m "feat: mount /probe endpoint, exporter is now multi-target"
```

---

### Task 6: Docs, dashboards, and compose example

**Files:**
- Modify: `README.md`
- Modify: `compose-dev.yml`
- Modify: `assets/shelly-device-exporter-prometheus.json`
- Modify: `assets/shelly-device-exporter-victoriametrics.json`

**Interfaces:** none — documentation only.

- [ ] **Step 1: Update compose-dev.yml**

Replace the `prometheus-config` block (lines 55–65) with:

```yaml
configs:
  prometheus-config:
    content: |
      global:
        scrape_interval: 30s

      scrape_configs:
        - job_name: shelly_device_exporter
          metrics_path: /probe
          params:
            module: [default]
          static_configs:
            - targets:
                - 192.168.1.100   # example: replace with your Shelly IPs
          relabel_configs:
            - source_labels: [__address__]
              target_label: __param_target
            - source_labels: [__param_target]
              target_label: instance
            - target_label: __address__
              replacement: shellydeviceexporter:8080
```

- [ ] **Step 2: Update README.md**

Replace the config-related sections with the following content, keeping the existing badges, features table, and metric list. Under "Deployment":

1. Replace the current `config.yml` example with:

```yaml
# config.yml — exporter configuration
price_per_kwh: 0.32    # optional, enables cost metrics
currency: EUR          # optional, label for cost metric
modules:
  # no module defined: /probe falls back to the built-in "default" (no auth)
  # auth:
  #   username: admin              # defaults to "admin"
  #   password: ${SHELLY_PASSWORD} # or password_file: /etc/secrets/shelly.txt
```

2. Add the Prometheus config example (this exact block, with the `auth` module):

```yaml
# prometheus.yml
scrape_configs:
  - job_name: shelly
    metrics_path: /probe
    params:
      module: [default]   # omit if your devices have no password
    static_configs:
      - targets: ["192.168.1.100", "192.168.1.101"]
      - targets: ["192.168.1.102"]
        labels: { module: auth }   # targets needing credentials
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [module]
        target_label: __param_module
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: shelly-exporter:8080   # host:port where the exporter runs
```

3. State that the device `name` label now comes from the device itself (set it in the Shelly app); one sentence.

- [ ] **Step 4: Add a probe status panel to both dashboard JSONs**

First rename the Prometheus dashboard file:

```bash
git mv assets/shelly-device-exporter-dashboard.json assets/shelly-device-exporter-prometheus.json
```

Both dashboards (`assets/shelly-device-exporter-prometheus.json` for Prometheus, `assets/shelly-device-exporter-victoriametrics.json` for VM) use the Grafana v2 `dashboard.grafana.app/v2` schema: panels live in `spec.elements` (keys `panel-1` … `panel-13`), wired into `spec.layout` via `ElementReference` items. The refactor adds `shelly_probe_success` and `shelly_probe_duration_seconds` (no labels; `instance` = device address), so add a "Device Status" panel that surfaces unreachable devices, which the old design never showed.

In each JSON:

1. In `spec.elements`, add a new element `panel-14`, mirroring the structure of the existing `panel-1` element but with these changes. For the Prometheus dashboard (`group: "prometheus"`):

```json
"panel-14": {
 "kind": "Panel",
 "spec": {
  "id": 14,
  "title": "Device Status",
  "description": "1 = probe succeeded, 0 = device unreachable or failed",
  "links": [],
  "data": {
   "kind": "QueryGroup",
   "spec": {
    "queries": [
     {
      "kind": "PanelQuery",
      "spec": {
       "query": {
        "kind": "DataQuery",
        "group": "prometheus",
        "version": "v0",
        "datasource": { "name": "${datasource}" },
        "spec": {
         "editorMode": "code",
         "expr": "shelly_probe_success",
         "legendFormat": "{{instance}}",
         "range": true
        }
       },
       "refId": "A",
       "hidden": false
      }
     }
    ],
    "transformations": [],
    "queryOptions": {}
   }
  },
  "vizConfig": {
   "kind": "VizConfig",
   "group": "timeseries",
   "version": "13.0.1",
   "spec": {
    "options": {
     "legend": { "calcs": [], "displayMode": "list", "placement": "bottom", "showLegend": true },
     "tooltip": { "hideZeros": false, "mode": "single", "sort": "none" }
    },
    "fieldConfig": {
     "defaults": {
      "unit": "short",
      "min": 0,
      "max": 1,
      "thresholds": {
       "mode": "absolute",
       "steps": [
        { "value": 0, "color": "red" },
        { "value": 1, "color": "green" }
       ]
      },
      "color": { "mode": "palette-classic" },
      "custom": { "drawStyle": "line", "fillOpacity": 0, "lineWidth": 2 }
     },
     "overrides": []
    }
   }
  }
 }
}
```

Copy any remaining fields (`annotations`, `transitions`, etc.) verbatim from `panel-1` so the element matches the file's schema version exactly. For the VM dashboard, the same element with `"group": "victoriametrics-metrics-datasource"` and `"expr": "shelly_probe_success{job=~\"$job\"}"`.

2. In `spec.layout`, add a `GridLayoutItem` at the start of the first row's `items` (shift nothing — the layout wraps; or place after the last item with `x`/`y` set to the next free slot):

```json
{
 "kind": "GridLayoutItem",
 "spec": {
  "x": 0, "y": 0, "width": 12, "height": 4,
  "element": { "kind": "ElementReference", "name": "panel-14" }
 }
}
```

3. Validate both files parse: `python3 -m json.tool assets/shelly-device-exporter-prometheus.json > /dev/null && python3 -m json.tool assets/shelly-device-exporter-victoriametrics.json > /dev/null`

- [ ] **Step 5: Commit**

```bash
git add README.md compose-dev.yml assets/shelly-device-exporter-prometheus.json assets/shelly-device-exporter-victoriametrics.json
git commit -m "docs: multi-target config examples, device status dashboard panel"
```

---

## Self-review notes

- Spec coverage: config modules (Task 1), single-target client + name from device (Task 2), collector/cost (Task 3), probe handler with timeout + probe metrics + 400 semantics (Task 4), main wiring + self-metrics (Task 5), docs (Task 6). Cost semantics verified in spec discussion: pure function of scraped Wh and global price; unchanged.
- Deviation from spec, already reflected in the spec file: env expansion reuses the existing `{{ env "VAR" }}` template instead of `os.ExpandEnv`.
- Type consistency: `Module(name string) (Module, bool)`, `New(address, username, password string) Client`, `Status(ctx) (*StatusResponse, error)`, `New(status, name, price, currency) *Exporter`, `Handler(cfg) http.Handler` — consistent across tasks.
