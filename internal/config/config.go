package config

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/template"

	"github.com/go-playground/validator/v10"
	"go.yaml.in/yaml/v2"
)

const (
	defaultUsername = "admin"
	DefaultModule   = "default"
)

type Module struct {
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	PasswordFile string `yaml:"password_file"`
}

type Config struct {
	Modules     map[string]Module `yaml:"modules"`
	PricePerKWh *float64          `yaml:"price_per_kwh" validate:"omitempty,gte=0"`
	Currency    string            `yaml:"currency"`
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

	// Strict unmarshal rejects the old `devices:` format with a clear error.
	conf := &Config{}
	if err := yaml.UnmarshalStrict(buf.Bytes(), conf); err != nil {
		return nil, err
	}

	if err := conf.resolvePasswordFiles(); err != nil {
		return nil, err
	}
	for name, mod := range conf.Modules {
		if mod.Username == "" {
			mod.Username = defaultUsername
			conf.Modules[name] = mod
		}
	}

	if err := validator.New().Struct(conf); err != nil {
		return nil, err
	}

	if !conf.CostEnabled() {
		slog.Warn("Cost calculation disabled because price_per_kwh or currency is not set")
	}

	return conf, nil
}

func (c *Config) resolvePasswordFiles() error {
	for name, mod := range c.Modules {
		if mod.Password != "" && mod.PasswordFile != "" {
			return fmt.Errorf("module %q: password and password_file are mutually exclusive", name)
		}
		if mod.PasswordFile == "" {
			continue
		}
		data, err := os.ReadFile(mod.PasswordFile)
		if err != nil {
			return fmt.Errorf("module %q: reading password_file: %w", name, err)
		}
		mod.Password = strings.TrimRight(string(data), "\n\r")
		mod.PasswordFile = ""
		c.Modules[name] = mod
	}
	return nil
}

// Module returns the named module's credentials. An empty name resolves to the
// built-in `default` module (no auth). An unknown name returns ok=false.
func (c *Config) Module(name string) (Module, bool) {
	if name == "" {
		name = DefaultModule
	}
	mod, ok := c.Modules[name]
	if !ok {
		if name == DefaultModule {
			return Module{Username: defaultUsername}, true
		}
		return Module{}, false
	}
	return mod, true
}

func (c Config) CostEnabled() bool {
	return c.PricePerKWh != nil && c.Currency != ""
}
