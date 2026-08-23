// Package driver defines the ScaleDriver abstraction: a protocol-agnostic
// contract for sending a price to a scale and reading back an approved
// transaction. Concrete implementations (Dialog 02/04 over raw TCP today, RIK
// in the future) plug in behind this interface so the rest of the system never
// depends on a specific vendor protocol.
package driver

import (
	"context"
	"fmt"

	"scale-app/backend/services/scale-gateway/internal/protocol"
)

// ScaleDriver sends a price-per-kg to a scale and waits for the resulting
// transaction (weight, computed total, and the scale's own status/approval
// data).
type ScaleDriver interface {
	// Connect establishes the underlying connection to the scale. It must be
	// called before SendPriceAndAwaitTransaction.
	Connect(ctx context.Context) error
	// Close releases the underlying connection. It is safe to call multiple
	// times and safe to call on a driver that was never connected.
	Close() error
	// SendPriceAndAwaitTransaction sets pricePerKgCents on the scale and
	// returns the resulting transaction once the scale reports it.
	SendPriceAndAwaitTransaction(ctx context.Context, pricePerKgCents int) (protocol.TransactionResult, error)
}

// Kind identifies which underlying implementation a driver uses.
type Kind string

const (
	// KindDialogRawTCP speaks Bizerba's Dialog 02/04 protocol over a raw TCP
	// socket, typically reaching the scale via a serial-to-Ethernet adapter.
	KindDialogRawTCP Kind = "dialog_raw_tcp"
	// KindRIK is reserved for a future driver built on Bizerba's Retail
	// Integrators Kit. Not implemented yet.
	KindRIK Kind = "rik"
)

// Config describes how to construct and connect a single scale's driver.
type Config struct {
	// ScaleID identifies the scale this config applies to.
	ScaleID string
	// Kind selects which driver implementation to construct.
	Kind Kind
	// DialogVariant selects the Dialog message format. Only used when Kind is
	// KindDialogRawTCP.
	DialogVariant DialogVariant
	// Address is the "host:port" of the scale's TCP endpoint (e.g. the
	// serial-to-Ethernet adapter). Only used when Kind is KindDialogRawTCP.
	Address string
}

// New constructs the ScaleDriver described by cfg. The driver is not yet
// connected; call Connect before use.
func New(cfg Config) (ScaleDriver, error) {
	switch cfg.Kind {
	case KindDialogRawTCP:
		return NewDialogTCPDriver(cfg.Address, cfg.DialogVariant)
	case KindRIK:
		return nil, fmt.Errorf("driver: kind %q (RIK) is not implemented yet", cfg.Kind)
	default:
		return nil, fmt.Errorf("driver: unknown kind %q", cfg.Kind)
	}
}
