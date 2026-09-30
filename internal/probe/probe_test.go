package probe_test

import (
	"fmt"
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
		// Mock Shelly device: GetStatus only — the exporter must not ask for
		// anything else, so an unexpected path fails the probe response.
		shellyServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/rpc/Shelly.GetStatus" {
				Fail("unexpected device request: " + r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"sys":{"mac":"0011","uptime":10},"switch:0":{"apower":10.5,"aenergy":{"total":5000}}}`))
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
		Expect(body).To(ContainSubstring(fmt.Sprintf(`shelly_probe_success{name=%q} 1`, addr)))
		Expect(body).To(ContainSubstring(fmt.Sprintf(`shelly_device_apower_watts{name=%q} 10.5`, addr)))
		// price is unset in this config, so no cost metric
		Expect(body).NotTo(ContainSubstring("aenergy_cost_total"))
	})

	It("reports failure but keeps HTTP 200 when the device is down", func() {
		code, body := doProbe("127.0.0.1:1", "")
		Expect(code).To(Equal(200))
		Expect(body).To(ContainSubstring(`shelly_probe_success{name="127.0.0.1:1"} 0`))
	})

	It("reports success for a device without a switch component", func() {
		nonSwitchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"sys":{"mac":"0022","uptime":10}}`))
		}))
		DeferCleanup(nonSwitchServer.Close)

		addr := strings.TrimPrefix(nonSwitchServer.URL, "http://")
		code, body := doProbe(addr, "")
		Expect(code).To(Equal(200))
		Expect(body).To(ContainSubstring(fmt.Sprintf(`shelly_probe_success{name=%q} 1`, addr)))
		Expect(body).To(ContainSubstring(fmt.Sprintf(`shelly_device_sys_mac_info{mac="0022",name=%q} 1`, addr)))
		Expect(body).NotTo(ContainSubstring("shelly_device_apower_watts"))
		Expect(body).NotTo(ContainSubstring("shelly_device_aenergy_cost_total"))
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
		Expect(string(body)).To(ContainSubstring(fmt.Sprintf(`shelly_probe_success{name=%q} 1`, addr)))
	})

	It("uses the named module's credentials", func() {
		var sawAuth bool
		authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if sawAuth {
				_, _ = w.Write([]byte(`{"sys":{"mac":"0022"}}`))
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
