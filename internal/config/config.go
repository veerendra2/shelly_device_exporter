package config

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"text/template"

	"github.com/go-playground/validator/v10"
	"go.yaml.in/yaml/v2"
)

const defaultUsername = "admin"

type Device struct {
	Name     string `yaml:"name" validate:"required"`
	Address  string `yaml:"address" validate:"required,url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type Config struct {
	Devices     []Device `yaml:"devices" validate:"required,min=1,dive"`
	PricePerKWh *float64 `yaml:"price_per_kwh" validate:"omitempty,gte=0"`
	Currency    string   `yaml:"currency"`
}

func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New("config").Funcs(template.FuncMap{
		"env": os.Getenv,
	}).Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to parse config template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		return nil, fmt.Errorf("failed to execute config template: %w", err)
	}

	conf := &Config{}
	if err := yaml.Unmarshal(buf.Bytes(), conf); err != nil {
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
	for i := range c.Devices {
		if c.Devices[i].Username == "" {
			c.Devices[i].Username = defaultUsername
		}
	}
}

func (c Config) CostEnabled() bool {
	return c.PricePerKWh != nil && c.Currency != ""
}
