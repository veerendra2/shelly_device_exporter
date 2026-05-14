package config

import (
	"log/slog"
	"os"

	"github.com/go-playground/validator/v10"
	"go.yaml.in/yaml/v2"
)

const defaultMaxConcurrentDeviceConnections = 4
const defaultUsername = "admin"

type Device struct {
	Name     string `yaml:"name" validate:"required"`
	Address  string `yaml:"address" validate:"required,url"`
	Username string `yaml:"username"`
	Password string `yaml:"password" validate:"required"`
}

type Config struct {
	Devices                        []Device `yaml:"devices" validate:"required,min=1,dive"`
	PricePerKWh                    *float64 `yaml:"price_per_kwh" validate:"omitempty,gte=0"`
	Currency                       string   `yaml:"currency"`
	MaxConcurrentDeviceConnections int      `yaml:"max_concurrent_device_connections" validate:"omitempty,gte=0"`
}

func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	conf := &Config{}
	err = yaml.Unmarshal(data, conf)
	if err != nil {
		return nil, err
	}

	conf.setDefaults()

	if err := validator.New().Struct(conf); err != nil {
		return nil, err
	}

	if !conf.CostEnabled() {
		slog.Warn("Cost calculation disabled because price_per_kwh or currency is not set")
	}

	return conf, nil
}

func (c *Config) setDefaults() {
	if c.MaxConcurrentDeviceConnections == 0 {
		c.MaxConcurrentDeviceConnections = defaultMaxConcurrentDeviceConnections
	}

	for i := range c.Devices {
		if c.Devices[i].Username == "" {
			c.Devices[i].Username = defaultUsername
		}
	}
}

func (c Config) CostEnabled() bool {
	return c.PricePerKWh != nil && c.Currency != ""
}
