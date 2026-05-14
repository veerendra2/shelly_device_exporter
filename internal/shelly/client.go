package shelly

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/icholy/digest"
	"github.com/veerendra2/shelly-plug-exporter/internal/config"
)

const statusPath = "/rpc/Shelly.GetStatus"

type Client struct {
	devices       []config.Device
	pricePerKWh   *float64
	currency      string
	maxConcurrent int
	httpClient    http.Client
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

	client := http.DefaultClient
	if password != "" {
		client = &http.Client{
			Transport: &digest.Transport{
				Username:  username,
				Password:  password,
				Transport: http.DefaultTransport,
			},
		}
	}

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

	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return &status, err
	}

	return &status, nil
}

func (c *Client) BulkStatus(ctx context.Context) (*DeviceStatus, error) {
	for _, device := range c.devices {
		fmt.Println(device)
		status, err := doRequest(ctx, device.Address, device.Username, device.Password)
		if err != nil {
			slog.Warn("Failed connect device", "error", err)
			os.Exit(1)
		}
		data, _ := json.MarshalIndent(status, "", "  ")
		fmt.Println(string(data))
	}

	return &DeviceStatus{}, nil
}

func New(cfg config.Config) (Client, error) {
	return Client{
		devices:       cfg.Devices,
		pricePerKWh:   cfg.PricePerKWh,
		maxConcurrent: cfg.MaxConcurrentDeviceConnections,
		currency:      cfg.Currency,
		httpClient:    http.Client{},
	}, nil
}
