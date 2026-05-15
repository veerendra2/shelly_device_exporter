package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alecthomas/kong"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/veerendra2/gopackages/slogger"
	"github.com/veerendra2/gopackages/version"
	"github.com/veerendra2/shelly_plug_exporter/internal/collector"
	"github.com/veerendra2/shelly_plug_exporter/internal/config"
	"github.com/veerendra2/shelly_plug_exporter/internal/shelly"
)

const appName = "shelly_device_exporter"

var cli struct {
	Address    string           `env:"ADDRESS" default:":8080" help:"The address where the server should listen on."`
	ConfigFile string           `env:"CONFIG_FILE" default:"config.yml" help:"Configuration file path"`
	Log        slogger.Config   `embed:"" prefix:"log-" envprefix:"LOG_"`
	Version    kong.VersionFlag `name:"version" help:"Print version information and exit"`
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

	slog.Info("Version information", version.Info()...)
	slog.Info("Build context", version.BuildContext()...)

	slog.Info("Loading configuration", "file", cli.ConfigFile)
	cfg, err := config.LoadConfig(cli.ConfigFile)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	shellyClient, err := shelly.New(*cfg)
	if err != nil {
		slog.Error("Failed to create shelly client", "error", err)
		os.Exit(1)
	}

	exporter, err := collector.New(shellyClient)
	if err != nil {
		slog.Error("Failed to create exporter", "error", err)
		os.Exit(1)
	}

	prometheus.MustRegister(exporter)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if _, err = w.Write([]byte("<body>Metrics are available at <a href=\"/metrics\">/metrics</a></body>")); err != nil {
			slog.Warn("Failed to write", "error", err)
		}
	})
	http.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:              cli.Address,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server died unexpected.", slog.Any("error", err))
		}
		slog.Error("Server stopped.")
	}()

	// All components should be terminated gracefully. For that we are listen
	// for the SIGINT and SIGTERM signals and try to gracefully shutdown the
	// started components. This ensures that established connections or tasks
	// are not interrupted.
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	slog.Info("Listening", "address", cli.Address)
	slog.Debug("Start listining for SIGINT and SIGTERM signal.")
	<-done
	slog.Info("Shutdown started.")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("HTTP shutdown error: %v", err)
	}

	slog.Info("Shutdown done.")
}
