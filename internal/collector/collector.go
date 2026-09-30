package collector

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/veerendra2/shelly_device_exporter/internal/shelly"
)

type Exporter struct {
	status      *shelly.StatusResponse
	pricePerKWh *float64
	currency    string
}

func New(status *shelly.StatusResponse, pricePerKWh *float64, currency string) *Exporter {
	return &Exporter{status: status, pricePerKWh: pricePerKWh, currency: currency}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- apower
	ch <- aenergyTotal
	ch <- voltage
	ch <- current
	ch <- pf
	ch <- freq
	ch <- temperatureCelsius
	ch <- energyCostTotal
	ch <- sysMAC
	ch <- restartRequired
	ch <- uptime
	ch <- ramSize
	ch <- ramFree
	ch <- fsSize
	ch <- fsFree
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	status := e.status
	if status == nil {
		return
	}

	if status.Switch0 != nil {
		s := status.Switch0
		if s.APower != nil {
			ch <- prometheus.MustNewConstMetric(apower, prometheus.GaugeValue, *s.APower)
		}
		if s.AEnergy != nil {
			ch <- prometheus.MustNewConstMetric(aenergyTotal, prometheus.CounterValue, s.AEnergy.Total)
			if e.pricePerKWh != nil && e.currency != "" {
				cost := (s.AEnergy.Total / 1000) * (*e.pricePerKWh)
				ch <- prometheus.MustNewConstMetric(energyCostTotal, prometheus.CounterValue, cost, e.currency)
			}
		}
		if s.Voltage != nil {
			ch <- prometheus.MustNewConstMetric(voltage, prometheus.GaugeValue, *s.Voltage)
		}
		if s.Current != nil {
			ch <- prometheus.MustNewConstMetric(current, prometheus.GaugeValue, *s.Current)
		}
		if s.PF != nil {
			ch <- prometheus.MustNewConstMetric(pf, prometheus.GaugeValue, *s.PF)
		}
		if s.Freq != nil {
			ch <- prometheus.MustNewConstMetric(freq, prometheus.GaugeValue, *s.Freq)
		}
		if s.Temperature != nil && s.Temperature.Celsius != nil {
			ch <- prometheus.MustNewConstMetric(temperatureCelsius, prometheus.GaugeValue, *s.Temperature.Celsius)
		}
	}

	if status.System != nil {
		ch <- prometheus.MustNewConstMetric(sysMAC, prometheus.GaugeValue, 1.0, status.System.MAC)
		ch <- prometheus.MustNewConstMetric(restartRequired, prometheus.GaugeValue, boolToFloat64(status.System.RestartRequired))
		ch <- prometheus.MustNewConstMetric(uptime, prometheus.CounterValue, status.System.Uptime)
		ch <- prometheus.MustNewConstMetric(ramSize, prometheus.GaugeValue, status.System.RAMSize)
		ch <- prometheus.MustNewConstMetric(ramFree, prometheus.GaugeValue, status.System.RAMFree)
		ch <- prometheus.MustNewConstMetric(fsSize, prometheus.GaugeValue, status.System.FSSize)
		ch <- prometheus.MustNewConstMetric(fsFree, prometheus.GaugeValue, status.System.FSFree)
	}
}

func boolToFloat64(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}
