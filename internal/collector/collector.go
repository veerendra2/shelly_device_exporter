package collector

import "github.com/veerendra2/shelly-plug-exporter/internal/config"

type Exporter struct {
	config *config.Config
}

func New(cfg config.Config) (*Exporter, error) {
	return &Exporter{
		config: &cfg,
	}, nil
}
