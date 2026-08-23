package gateway

import (
	"fmt"

	"scale-app/backend/services/scale-gateway/internal/driver"
)

// ScaleEntry pairs a scale's driver with the metadata the HTTP API reports
// about it.
type ScaleEntry struct {
	ID     string
	Kind   driver.Kind
	Driver driver.ScaleDriver
}

// BuildEntries constructs a ScaleDriver for every configured scale.
func BuildEntries(cfg Config) ([]ScaleEntry, error) {
	entries := make([]ScaleEntry, 0, len(cfg.Scales))
	for _, s := range cfg.Scales {
		drv, err := driver.New(s.toDriverConfig())
		if err != nil {
			return nil, fmt.Errorf("gateway: build driver for scale %q: %w", s.ID, err)
		}
		entries = append(entries, ScaleEntry{ID: s.ID, Kind: s.Kind, Driver: drv})
	}
	return entries, nil
}
