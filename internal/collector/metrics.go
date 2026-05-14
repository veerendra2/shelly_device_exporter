package collector

import "github.com/prometheus/client_golang/prometheus"

const namespace = "shelly_device"

var (
	// ******* Switch (Power only) **********
	// https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Switch/#status
	apower = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "apower"),
		"Last measured instantaneous active power (in Watts) delivered to the attached load.",
		[]string{
			"name",
		}, nil,
	)
	voltage = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "voltage"),
		"Last measured voltage in Volts.",
		[]string{
			"name",
		}, nil,
	)
	current = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "current"),
		"Last measured current in Amperes.",
		[]string{
			"name",
		}, nil,
	)
	pf = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "pf"),
		"Last measured power factor.",
		[]string{
			"name",
		}, nil,
	)
	freq = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "freq"),
		"Last measured network frequency in Hz.",
		[]string{
			"name",
		}, nil,
	)
	temperatureCelsius = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "temperature_celsius"),
		"Temperature in Celsius.",
		[]string{
			"name",
		}, nil,
	)

	energyCostTotal = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "energy_cost_total"),
		"Total energy cost total.",
		[]string{
			"name",
		}, nil,
	)

	// ******** System ***********
	// https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Sys#status
	sysMAC = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "sys_mac"),
		"Mac address of the device.",
		[]string{
			"name",
		}, nil,
	)
	restartRequired = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "restart_required"),
		"True if restart is required, false otherwise.",
		[]string{
			"name",
		}, nil,
	)
	uptime = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "uptime"),
		"Time in seconds since last reboot.",
		[]string{
			"name",
		}, nil,
	)
	ramSize = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "ram_size"),
		"Total size of the RAM in the system in Bytes.",
		[]string{
			"name",
		}, nil,
	)
	ramFree = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "ram_free"),
		"Size of the free RAM in the system in Bytes.",
		[]string{
			"name",
		}, nil,
	)
	fsSize = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "fs_size"),
		"Total size of the file system in Bytes.",
		[]string{
			"name",
		}, nil,
	)
	fsFree = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "fs_free"),
		"Size of the free file system in Bytes.",
		[]string{
			"name",
		}, nil,
	)
)
