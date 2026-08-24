package driver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"scale-app/backend/services/scale-gateway/internal/protocol"
)

// DialogVariant selects which Dialog message format a DialogTCPDriver speaks.
type DialogVariant string

const (
	DialogVariant02 DialogVariant = "02"
	DialogVariant04 DialogVariant = "04"
)

func codecFor(variant DialogVariant) (protocol.Codec, error) {
	switch variant {
	case DialogVariant02:
		return protocol.Dialog02Codec{}, nil
	case DialogVariant04:
		return protocol.Dialog04Codec{}, nil
	default:
		return nil, fmt.Errorf("driver: unknown dialog variant %q", variant)
	}
}

// DialogTCPDriver implements ScaleDriver by speaking Bizerba's Dialog 02/04
// protocol over a raw TCP connection, typically to a serial-to-Ethernet
// adapter wired to the scale.
type DialogTCPDriver struct {
	Address     string
	Variant     DialogVariant
	DialTimeout time.Duration
	IOTimeout   time.Duration

	codec protocol.Codec

	mu   sync.Mutex
	conn net.Conn
}

// NewDialogTCPDriver constructs a driver for the given address and Dialog
// variant. The connection is not established until Connect is called.
func NewDialogTCPDriver(address string, variant DialogVariant) (*DialogTCPDriver, error) {
	codec, err := codecFor(variant)
	if err != nil {
		return nil, err
	}
	return &DialogTCPDriver{
		Address:     address,
		Variant:     variant,
		DialTimeout: 5 * time.Second,
		IOTimeout:   5 * time.Second,
		codec:       codec,
	}, nil
}

// newDialogDriverForConn wires a DialogTCPDriver directly around an existing
// connection, bypassing Connect/net.Dial. It exists so tests can drive the
// protocol logic over an in-memory net.Pipe instead of a real socket.
func newDialogDriverForConn(conn net.Conn, variant DialogVariant) (*DialogTCPDriver, error) {
	d, err := NewDialogTCPDriver("", variant)
	if err != nil {
		return nil, err
	}
	d.conn = conn
	return d, nil
}

func (d *DialogTCPDriver) Connect(ctx context.Context) error {
	dialer := net.Dialer{Timeout: d.DialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", d.Address)
	if err != nil {
		return fmt.Errorf("driver: connect to scale at %s: %w", d.Address, err)
	}
	d.mu.Lock()
	d.conn = conn
	d.mu.Unlock()
	return nil
}

func (d *DialogTCPDriver) Close() error {
	d.mu.Lock()
	conn := d.conn
	d.conn = nil
	d.mu.Unlock()
	if conn == nil {
		return nil
	}
	return conn.Close()
}

func (d *DialogTCPDriver) SendPriceAndAwaitTransaction(ctx context.Context, pricePerKgCents int) (protocol.TransactionResult, error) {
	d.mu.Lock()
	conn := d.conn
	d.mu.Unlock()
	if conn == nil {
		return protocol.TransactionResult{}, errors.New("driver: not connected")
	}

	deadline := time.Now().Add(d.IOTimeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return protocol.TransactionResult{}, fmt.Errorf("driver: set deadline: %w", err)
	}

	requestFrame, err := d.codec.EncodeSetPrice(pricePerKgCents)
	if err != nil {
		return protocol.TransactionResult{}, fmt.Errorf("driver: encode price: %w", err)
	}
	if _, err := conn.Write(requestFrame); err != nil {
		return protocol.TransactionResult{}, fmt.Errorf("driver: write price frame: %w", err)
	}

	responseFrame, err := protocol.ReadFrame(conn)
	if err != nil {
		return protocol.TransactionResult{}, fmt.Errorf("driver: read transaction response: %w", err)
	}

	result, err := d.codec.DecodeTransactionResponse(responseFrame)
	if err != nil {
		return protocol.TransactionResult{}, fmt.Errorf("driver: decode transaction response: %w", err)
	}
	return result, nil
}
