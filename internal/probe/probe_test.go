package probe_test

import (
	"io"
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
		exporter     *httptest.Server
		shellyServer *httptest.Server
	)

	BeforeEach(func() {
		// Mock Shelly device: GetStatus for metrics, GetConfig for the name.
		shellyServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			switch r.URL.Path {
			case "/rpc/Shelly.GetConfig":
				_, _ = w.Write([]byte(`{"sys":{"device":{"name":"plug-1"}}}`))
			default:
				_, _ = w.Write([]byte(`{"sys":{"mac":"0011","uptime":10},"switch:0":{"apower":10.5,"aenergy":{"total":5000}}}`))
			}
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
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}

	It("scrapes a device with a bare-IP target and reports success", func() {
		addr := strings.TrimPrefix(shellyServer.URL, "http://")
		code, body := doProbe(addr, "")
		Expect(code).To(Equal(200))
		Expect(body).To(ContainSubstring(`shelly_probe_success 1`))
		Expect(body).To(ContainSubstring(`shelly_device_apower_watts{name="plug-1"} 10.5`))
		// price is unset in this config, so no cost metric
		Expect(body).NotTo(ContainSubstring("aenergy_cost_total"))
	})

	It("reports failure but keeps HTTP 200 when the device is down", func() {
		code, body := doProbe("127.0.0.1:1", "")
		Expect(code).To(Equal(200))
		Expect(body).To(ContainSubstring(`shelly_probe_success 0`))
	})

	It("falls back to the bare target address when the device has no name", func() {
		unnamed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"sys":{"mac":"0011"}}`))
		}))
		DeferCleanup(unnamed.Close)

		code, body := doProbe(strings.TrimPrefix(unnamed.URL, "http://"), "")
		Expect(code).To(Equal(200))
		// no scheme in the fallback name, matching the instance label
		Expect(body).To(ContainSubstring(`{name="` + strings.TrimPrefix(unnamed.URL, "http://") + `"}`))
	})

	It("returns 400 for a missing target", func() {
		code, _ := doProbe("", "")
		Expect(code).To(Equal(400))
	})

	It("returns 400 for an unknown module", func() {
		code, _ := doProbe("127.0.0.1:1", "nope")
		Expect(code).To(Equal(400))
	})

	It("treats a garbage timeout header as absent, not an error", func() {
		addr := strings.TrimPrefix(shellyServer.URL, "http://")
		req, err := http.NewRequest(http.MethodGet, exporter.URL+"/probe?target="+addr, nil)
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set("X-Prometheus-Scrape-Timeout-Seconds", "abc")
		resp2, err := http.DefaultClient.Do(req)
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = resp2.Body.Close() }()
		body, _ := io.ReadAll(resp2.Body)
		Expect(resp2.StatusCode).To(Equal(200))
		Expect(string(body)).To(ContainSubstring(`shelly_probe_success 1`))
	})

	It("uses the named module's credentials", func() {
		var sawAuth bool
		authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if sawAuth {
				_, _ = w.Write([]byte(`{"sys":{"device":{"name":"plug-2"}}}`))
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
		body, _ := io.ReadAll(resp.Body)
		Expect(resp.StatusCode).To(Equal(200))
		Expect(string(body)).To(ContainSubstring(`shelly_device_sys_mac_info`))
	})
})
