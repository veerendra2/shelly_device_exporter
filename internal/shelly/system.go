package shelly

// SystemStatus is the response payload for Sys.GetStatus.
//
// Source: https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Sys/#status
type SystemStatus struct {
	MAC              string        `json:"mac"`
	RestartRequired  bool          `json:"restart_required"`
	Time             *string       `json:"time"`
	Unixtime         *float64      `json:"unixtime"`
	LastSyncTS       *float64      `json:"last_sync_ts"`
	Uptime           float64       `json:"uptime"`
	RAMSize          float64       `json:"ram_size"`
	RAMFree          float64       `json:"ram_free"`
	FSSize           float64       `json:"fs_size"`
	FSFree           float64       `json:"fs_free"`
	ConfigRevision   float64       `json:"cfg_rev"`
	KVSRevision      float64       `json:"kvs_rev"`
	ScheduleRevision *float64      `json:"schedule_rev,omitempty"`
	WebhookRevision  *float64      `json:"webhook_rev,omitempty"`
	KNXRevision      *float64      `json:"knx_rev,omitempty"`
	BTRelayRevision  *float64      `json:"btrelay_rev,omitempty"`
	BTHCRevision     *float64      `json:"bthc_rev,omitempty"`
	AvailableUpdates SystemUpdates `json:"available_updates"`
	WakeupReason     *WakeupReason `json:"wakeup_reason,omitempty"`
	UTCOffset        *float64      `json:"utc_offset,omitempty"`
}

// SystemUpdates contains available firmware update versions.
type SystemUpdates struct {
	Beta   *SystemUpdate `json:"beta,omitempty"`
	Stable *SystemUpdate `json:"stable,omitempty"`
}

// SystemUpdate contains a firmware update version.
type SystemUpdate struct {
	Version string `json:"version"`
}

// WakeupReason contains boot type and cause for battery-operated devices.
type WakeupReason struct {
	Boot         string   `json:"boot"`
	Cause        string   `json:"cause"`
	WakeupPeriod *float64 `json:"wakeup_period,omitempty"`
}
