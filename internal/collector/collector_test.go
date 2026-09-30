package collector_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/veerendra2/shelly_device_exporter/internal/collector"
	"github.com/veerendra2/shelly_device_exporter/internal/shelly"
)

func TestCollector(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Collector Suite")
}

func gather(exporter prometheus.Collector) map[string]*dto.MetricFamily {
	registry := prometheus.NewRegistry()
	registry.MustRegister(exporter)
	families, err := registry.Gather()
	Expect(err).NotTo(HaveOccurred())
	out := make(map[string]*dto.MetricFamily, len(families))
	for _, f := range families {
		out[f.GetName()] = f
	}
	return out
}

func labelValue(m *dto.Metric, name string) string {
	for _, l := range m.GetLabel() {
		if l.GetName() == name {
			return l.GetValue()
		}
	}
	return ""
}

var _ = Describe("Collector", func() {
	It("emits switch, system, and cost metrics from a fetched status", func() {
		price := 0.32
		status := &shelly.StatusResponse{
			System:  &shelly.SystemStatus{MAC: "0011", Uptime: 100, RAMSize: 1, RAMFree: 2, FSSize: 3, FSFree: 4},
			Switch0: &shelly.SwitchStatus{APower: new(10.5), AEnergy: &shelly.SwitchEnergy{Total: 5000}},
		}
		exporter := collector.New(status, "plug-1", &price, "EUR")

		families := gather(exporter)
		Expect(families).To(HaveKey("shelly_device_apower_watts"))
		Expect(families).To(HaveKey("shelly_device_aenergy_watt_hours_total"))
		Expect(families).To(HaveKey("shelly_device_aenergy_cost_total"))
		Expect(families).To(HaveKey("shelly_device_sys_mac_info"))
		Expect(families).To(HaveKey("shelly_device_uptime_seconds_total"))

		cost := families["shelly_device_aenergy_cost_total"].Metric[0]
		Expect(labelValue(cost, "name")).To(Equal("plug-1"))
		Expect(labelValue(cost, "currency")).To(Equal("EUR"))
		// 5000 Wh / 1000 * 0.32 = 1.6
		Expect(cost.GetCounter().GetValue()).To(Equal(1.6))

		power := families["shelly_device_apower_watts"].Metric[0]
		Expect(power.GetGauge().GetValue()).To(Equal(10.5))
		Expect(labelValue(power, "name")).To(Equal("plug-1"))
	})
})
