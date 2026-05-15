package shelly

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/icholy/digest"
	"github.com/veerendra2/shelly_plug_exporter/internal/config"
)

const (
	statusPath                     = "/rpc/Shelly.GetStatus"
	maxConcurrentDeviceConnections = 4
)

type Client struct {
	devices     []config.Device
	pricePerKWh *float64
	currency    string
	costEnabled bool
}

type DeviceStatus struct {
	Name    string
	Address string
	Switch  *SwitchStatus
	System  *SystemStatus
	Cost    *EnergyCost
	Err     error
}

type EnergyCost struct {
	Value    float64
	Currency string
}

// This exporter currently supports only the components below,
// so the response is unmarshaled into these objects only.
type StatusResponse struct {
	System  *SystemStatus `json:"sys"`
	Switch0 *SwitchStatus `json:"switch:0"`
}

func doRequest(ctx context.Context, addr string, username string, password string) (*StatusResponse, error) {
	var status StatusResponse
	requestUrl, err := url.Parse(addr)
	if err != nil {
		return &status, err
	}
	requestUrl.Path = path.Join(requestUrl.Path, statusPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestUrl.String(), nil)
	if err != nil {
		return &status, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	if password != "" {
		client.Transport = &digest.Transport{
			Username:  username,
			Password:  password,
			Transport: http.DefaultTransport,
		}
	}

	slog.Debug("Connecting to shelly device", "device_address", addr)
	resp, err := client.Do(req)
	if err != nil {
		return &status, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Warn("Error while closing the response body", slog.Any("err", err))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return &status, fmt.Errorf("shelly request failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &status, fmt.Errorf("failed to read response body: %w", err)
	}

	slog.Debug("Raw Shelly API response", "device", requestUrl.Host, "json", string(body))

	if err := json.Unmarshal(body, &status); err != nil {
		return &status, err
	}

	return &status, nil
}

func (c *Client) BulkStatus(ctx context.Context) []DeviceStatus {
	numDevices := len(c.devices)
	jobs := make(chan config.Device, numDevices)
	results := make(chan DeviceStatus, numDevices)

	// Determine the optimal number of workers
	numWorkers := min(numDevices, maxConcurrentDeviceConnections)
	slog.Debug("Spawning workers to connect shelly devices", "count", numWorkers)

	// Start workers
	for range numWorkers {
		go func() {
			for device := range jobs {
				select {
				case <-ctx.Done():
					// Context cancelled, send error to prevent deadlock in the collector loop
					results <- DeviceStatus{
						Name:    device.Name,
						Address: device.Address,
						Err:     ctx.Err(),
					}
					continue
				default:
				}

				status, err := doRequest(ctx, device.Address, device.Username, device.Password)
				if err != nil {
					results <- DeviceStatus{
						Name:    device.Name,
						Address: device.Address,
						Err:     err,
					}
					continue
				}

				deviceStatus := DeviceStatus{
					Name:    device.Name,
					Address: device.Address,
					Switch:  status.Switch0,
					System:  status.System,
				}

				if c.costEnabled && status.Switch0 != nil && status.Switch0.AEnergy != nil {
					deviceStatus.Cost = &EnergyCost{
						Value:    (status.Switch0.AEnergy.Total / 1000) * (*c.pricePerKWh),
						Currency: c.currency,
					}
				}

				results <- deviceStatus
			}
		}()
	}

	// Feed jobs
	for _, device := range c.devices {
		jobs <- device
	}
	close(jobs)

	// Collect and filter results
	var finalStatuses []DeviceStatus
	for range numDevices {
		res := <-results
		if res.Err != nil {
			slog.Warn("Failed to get status from device",
				"name", res.Name,
				"address", res.Address,
				"error", res.Err)
			continue
		}
		finalStatuses = append(finalStatuses, res)
	}

	return finalStatuses
}

func New(cfg config.Config) (Client, error) {
	return Client{
		devices:     cfg.Devices,
		pricePerKWh: cfg.PricePerKWh,
		currency:    cfg.Currency,
		costEnabled: cfg.CostEnabled(),
	}, nil
}
