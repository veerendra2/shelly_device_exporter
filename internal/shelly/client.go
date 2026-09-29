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

	"github.com/icholy/digest"
)

const (
	statusPath = "/rpc/Shelly.GetStatus"
	configPath = "/rpc/Shelly.GetConfig"
)

// This exporter currently supports only the components below,
// so the response is unmarshaled into these objects only.
type StatusResponse struct {
	System  *SystemStatus `json:"sys"`
	Switch0 *SwitchStatus `json:"switch:0"`
}

// The device name is not part of GetStatus; it lives in GetConfig
// under sys.device.name (see Shelly Gen2 API docs).
type ConfigResponse struct {
	System struct {
		Device struct {
			Name string `json:"name"`
		} `json:"device"`
	} `json:"sys"`
}

// Client scrapes a single Shelly device. One client exists per probe request.
type Client struct {
	address  string
	username string
	password string
}

func New(address, username, password string) Client {
	return Client{address: address, username: username, password: password}
}

// The timeout comes from the caller's context (the probe handler derives it
// from the Prometheus scrape-timeout header), so the http.Client sets none.
func (c *Client) get(ctx context.Context, rpcPath string) ([]byte, error) {
	requestUrl, err := url.Parse(c.address)
	if err != nil {
		return nil, fmt.Errorf("invalid device address %q: %w", c.address, err)
	}
	requestUrl.Path = path.Join(requestUrl.Path, rpcPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestUrl.String(), nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	if c.password != "" {
		client.Transport = &digest.Transport{
			Username:  c.username,
			Password:  c.password,
			Transport: http.DefaultTransport,
			NoReuse:   true,
		}
	}

	slog.Debug("Connecting to shelly device", "device_address", c.address)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting %s: %w", c.address, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Warn("Error while closing the response body", slog.Any("error", err))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("shelly request failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	slog.Debug("Raw Shelly API response", "device", requestUrl.Host, "json", string(body))
	return body, nil
}

func (c *Client) Status(ctx context.Context) (*StatusResponse, error) {
	body, err := c.get(ctx, statusPath)
	if err != nil {
		return nil, err
	}

	var status StatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("failed to parse device response: %w", err)
	}

	return &status, nil
}

// Name fetches the device name from Shelly.GetConfig. Best effort: the probe
// falls back to the target address when this fails, so errors are logged only.
func (c *Client) Name(ctx context.Context) string {
	body, err := c.get(ctx, configPath)
	if err != nil {
		slog.Warn("Failed to fetch device name", "device_address", c.address, "error", err)
		return ""
	}

	var cfg ConfigResponse
	if err := json.Unmarshal(body, &cfg); err != nil {
		slog.Warn("Failed to parse device config", "device_address", c.address, "error", err)
		return ""
	}
	return cfg.System.Device.Name
}
