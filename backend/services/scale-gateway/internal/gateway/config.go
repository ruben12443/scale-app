package gateway

import (
	"encoding/json"
	"fmt"
	"os"

	"scale-app/backend/services/scale-gateway/internal/driver"
)

// ScaleConfig describes one scale's connection details, as read from the
// gateway's JSON config file.
type ScaleConfig struct {
	ID            string               `json:"id"`
	Kind          driver.Kind          `json:"kind"`
	DialogVariant driver.DialogVariant `json:"dialog_variant,omitempty"`
	Address       string               `json:"address,omitempty"`
}

// Config is the top-level shape of the gateway's config file.
type Config struct {
	// ListenAddress is the "host:port" the gateway's own HTTP API listens on,
	// e.g. ":8080".
	ListenAddress string        `json:"listen_address"`
	Scales        []ScaleConfig `json:"scales"`
}

// LoadConfig reads and validates a gateway config file.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("gateway: read config %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("gateway: parse config %s: %w", path, err)
	}

	if cfg.ListenAddress == "" {
		return Config{}, fmt.Errorf("gateway: config %s: listen_address is required", path)
	}

	seen := make(map[string]bool, len(cfg.Scales))
	for _, s := range cfg.Scales {
		if s.ID == "" {
			return Config{}, fmt.Errorf("gateway: config %s: a scale entry is missing an id", path)
		}
		if seen[s.ID] {
			return Config{}, fmt.Errorf("gateway: config %s: duplicate scale id %q", path, s.ID)
		}
		seen[s.ID] = true

		switch s.Kind {
		case driver.KindDialogRawTCP:
			if s.Address == "" {
				return Config{}, fmt.Errorf("gateway: config %s: scale %q: address is required for kind %q", path, s.ID, s.Kind)
			}
			if s.DialogVariant != driver.DialogVariant02 && s.DialogVariant != driver.DialogVariant04 {
				return Config{}, fmt.Errorf("gateway: config %s: scale %q: dialog_variant must be \"02\" or \"04\"", path, s.ID)
			}
		case driver.KindRIK:
			// Accepted at config level so a deployment can be prepared ahead of
			// the driver's implementation; BuildEntries will fail loudly if one
			// is actually instantiated.
		default:
			return Config{}, fmt.Errorf("gateway: config %s: scale %q: unknown kind %q", path, s.ID, s.Kind)
		}
	}

	return cfg, nil
}

// toDriverConfig converts a ScaleConfig into the driver package's Config.
func (s ScaleConfig) toDriverConfig() driver.Config {
	return driver.Config{
		ScaleID:       s.ID,
		Kind:          s.Kind,
		DialogVariant: s.DialogVariant,
		Address:       s.Address,
	}
}
