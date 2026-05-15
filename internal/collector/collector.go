package collector

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/veerendra2/shelly_plug_exporter/internal/shelly"
)

type Exporter struct {
	shellyClient shelly.Client
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, status := range e.shellyClient.BulkStatus(ctx) {
		// Collect Switch metrics
		if status.Switch != nil {
			if status.Switch.APower != nil {
				ch <- prometheus.MustNewConstMetric(apower, prometheus.GaugeValue, *status.Switch.APower, status.Name)
			}
			if status.Switch.AEnergy != nil {
				ch <- prometheus.MustNewConstMetric(aenergyTotal, prometheus.CounterValue, status.Switch.AEnergy.Total, status.Name)
			}
			if status.Switch.Voltage != nil {
				ch <- prometheus.MustNewConstMetric(voltage, prometheus.GaugeValue, *status.Switch.Voltage, status.Name)
			}
			if status.Switch.Current != nil {
				ch <- prometheus.MustNewConstMetric(current, prometheus.GaugeValue, *status.Switch.Current, status.Name)
			}
			if status.Switch.PF != nil {
				ch <- prometheus.MustNewConstMetric(pf, prometheus.GaugeValue, *status.Switch.PF, status.Name)
			}
			if status.Switch.Freq != nil {
				ch <- prometheus.MustNewConstMetric(freq, prometheus.GaugeValue, *status.Switch.Freq, status.Name)
			}
			if status.Switch.Temperature != nil && status.Switch.Temperature.Celsius != nil {
				ch <- prometheus.MustNewConstMetric(temperatureCelsius, prometheus.GaugeValue, *status.Switch.Temperature.Celsius, status.Name)
			}
		}

		// Collect Cost metrics
		if status.Cost != nil {
			ch <- prometheus.MustNewConstMetric(energyCostTotal, prometheus.CounterValue, status.Cost.Value, status.Name, status.Cost.Currency)
		}

		// Collect System metrics
		if status.System != nil {
			ch <- prometheus.MustNewConstMetric(sysMAC, prometheus.GaugeValue, 1.0, status.System.MAC, status.Name)
			ch <- prometheus.MustNewConstMetric(restartRequired, prometheus.GaugeValue, boolToFloat64(status.System.RestartRequired), status.Name)
			ch <- prometheus.MustNewConstMetric(uptime, prometheus.CounterValue, status.System.Uptime, status.Name)
			ch <- prometheus.MustNewConstMetric(ramSize, prometheus.GaugeValue, status.System.RAMSize, status.Name)
			ch <- prometheus.MustNewConstMetric(ramFree, prometheus.GaugeValue, status.System.RAMFree, status.Name)
			ch <- prometheus.MustNewConstMetric(fsSize, prometheus.GaugeValue, status.System.FSSize, status.Name)
			ch <- prometheus.MustNewConstMetric(fsFree, prometheus.GaugeValue, status.System.FSFree, status.Name)
		}
	}
}

func New(client shelly.Client) (*Exporter, error) {
	return &Exporter{
		shellyClient: client,
	}, nil
}

func boolToFloat64(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}
