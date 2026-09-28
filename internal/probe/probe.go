package probe

import (
	"context"
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
	success := 0.0
	if p.success {
		success = 1.0
	}
	ch <- prometheus.MustNewConstMetric(probeSuccess, prometheus.GaugeValue, success)
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
