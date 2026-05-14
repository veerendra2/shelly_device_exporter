package shelly

// SwitchStatus is the response payload for Switch.GetStatus.
//
// Source: https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Switch/#status
type SwitchStatus struct {
	ID             int                `json:"id"`
	Source         string             `json:"source"`
	Tag            *string            `json:"tag"`
	Output         bool               `json:"output"`
	TimerStartedAt *float64           `json:"timer_started_at,omitempty"`
	TimerDuration  *float64           `json:"timer_duration,omitempty"`
	APower         *float64           `json:"apower,omitempty"`
	Voltage        *float64           `json:"voltage,omitempty"`
	Current        *float64           `json:"current,omitempty"`
	PF             *float64           `json:"pf,omitempty"`
	Freq           *float64           `json:"freq,omitempty"`
	AEnergy        *SwitchEnergy      `json:"aenergy,omitempty"`
	RetAEnergy     *SwitchEnergy      `json:"ret_aenergy,omitempty"`
	Counts         *SwitchCounts      `json:"counts,omitempty"`
	Temperature    *SwitchTemperature `json:"temperature,omitempty"`
	Errors         []string           `json:"errors,omitempty"`
}

// SwitchEnergy contains active energy counter values.
type SwitchEnergy struct {
	Total    float64   `json:"total"`
	ByMinute []float64 `json:"by_minute,omitempty"`
	MinuteTS *int64    `json:"minute_ts,omitempty"`
}

// SwitchCounts contains switch lifecycle counters.
type SwitchCounts struct {
	OnTime            float64 `json:"on_time"`
	OnTimeResetTS     int64   `json:"on_time_rst_ts"`
	SwitchOn          float64 `json:"switch_on"`
	SwitchOnResetTS   int64   `json:"switch_on_rst_ts"`
	OnAboveThreshold  float64 `json:"on_above_thr"`
	OnAboveThrResetTS int64   `json:"on_above_thr_rst_ts"`
}

// SwitchTemperature contains switch temperature readings.
type SwitchTemperature struct {
	Celsius    *float64 `json:"tC"`
	Fahrenheit *float64 `json:"tF"`
}
