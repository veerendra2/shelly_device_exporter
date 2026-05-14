package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/alecthomas/kong"
	"github.com/veerendra2/gopackages/slogger"
	"github.com/veerendra2/gopackages/version"
	"github.com/veerendra2/shelly-plug-exporter/internal/config"
	"github.com/veerendra2/shelly-plug-exporter/internal/shelly"
)

const appName = "shelly_device_exporter"

var cli struct {
	Address string           `env:"ADDRESS" default:":8080" help:"The address where the server should listen on."`
	Config  string           `env:"CONFIG_FILE" default:"config.yml" help:"Configuration file path"`
	Log     slogger.Config   `embed:"" prefix:"log." envprefix:"LOG_"`
	Version kong.VersionFlag `name:"version" help:"Print version information and exit"`
}

func main() {
	kongCtx := kong.Parse(&cli,
		kong.Name(appName),
		kong.Description("Prometheus exporter for Shelly Gen 2+ devices."),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
		kong.Vars{
			"version": version.Version,
		},
	)

	kongCtx.FatalIfErrorf(kongCtx.Error)

	slog.SetDefault(slogger.New(cli.Log))

	slog.Info("Starting shelly_device_exporter - A Prometheus exporter for Shelly Gen 2+ devices")
	slog.Info("Version information", version.Info()...)
	slog.Info("Build context", version.BuildContext()...)

	slog.Info("Loading configuration", "file", cli.Config)
	cfg, err := config.LoadConfig(cli.Config)
	if err != nil {
		slog.Error("Failed to load configuration", slog.Any("err", err))
		os.Exit(1)
	}
	fmt.Println(cfg)

	shellyClient, err := shelly.New(*cfg)
	if err != nil {
		slog.Error("Failed to create shelly client", "error", err)
		os.Exit(1)
	}

	_, err = shellyClient.BulkStatus(context.TODO())
	if err != nil {
		slog.Error("Failed to fetch status for devices", "error", err)
		os.Exit(1)
	}

}
