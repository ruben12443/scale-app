package fakescale_test

import (
	"context"
	"net"
	"testing"

	"scale-app/backend/services/scale-gateway/internal/driver"
	"scale-app/backend/services/scale-gateway/internal/fakescale"
	"scale-app/backend/services/scale-gateway/internal/protocol"
)

// startServer runs a fakescale.Server on a random local port and returns
// its address. The listener is closed (and Serve's goroutine unblocked)
// via t.Cleanup.
func startServer(t *testing.T, codec protocol.Codec, status string, weightGrams int) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	srv := fakescale.New(fakescale.Config{Codec: codec, StatusCode: status, WeightGrams: weightGrams})
	go srv.Serve(ln) //nolint:errcheck // error is expected once the listener is closed in cleanup

	return ln.Addr().String()
}

// These tests drive the REAL driver.DialogTCPDriver (the same code
// scale-gateway uses against actual hardware) against fakescale.Server, so
// the simulator is verified against production client code, not just its
// own codec in isolation.

func TestFakeScaleAgainstRealDialog02Driver(t *testing.T) {
	addr := startServer(t, protocol.Dialog02Codec{}, "1", 1250)

	drv, err := driver.NewDialogTCPDriver(addr, driver.DialogVariant02)
	if err != nil {
		t.Fatalf("NewDialogTCPDriver returned error: %v", err)
	}
	if err := drv.Connect(context.Background()); err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	defer drv.Close()

	result, err := drv.SendPriceAndAwaitTransaction(context.Background(), 499)
	if err != nil {
		t.Fatalf("SendPriceAndAwaitTransaction returned error: %v", err)
	}
	if result.StatusCode != "1" || result.WeightGrams != 1250 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if want := 499 * 1250 / 1000; result.PriceCents != want {
		t.Fatalf("PriceCents = %d, want %d", result.PriceCents, want)
	}

	// A second transaction over the SAME persistent connection, mirroring
	// how the real driver is actually used (connect once, many
	// transactions across the connection's lifetime).
	result2, err := drv.SendPriceAndAwaitTransaction(context.Background(), 199)
	if err != nil {
		t.Fatalf("second SendPriceAndAwaitTransaction returned error: %v", err)
	}
	if want := 199 * 1250 / 1000; result2.PriceCents != want {
		t.Fatalf("second PriceCents = %d, want %d", result2.PriceCents, want)
	}
}

func TestFakeScaleAgainstRealDialog04Driver(t *testing.T) {
	addr := startServer(t, protocol.Dialog04Codec{}, "100", 2000)

	drv, err := driver.NewDialogTCPDriver(addr, driver.DialogVariant04)
	if err != nil {
		t.Fatalf("NewDialogTCPDriver returned error: %v", err)
	}
	if err := drv.Connect(context.Background()); err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	defer drv.Close()

	result, err := drv.SendPriceAndAwaitTransaction(context.Background(), 899)
	if err != nil {
		t.Fatalf("SendPriceAndAwaitTransaction returned error: %v", err)
	}
	if result.StatusCode != "100" || result.WeightGrams != 2000 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if want := 899 * 2000 / 1000; result.PriceCents != want {
		t.Fatalf("PriceCents = %d, want %d", result.PriceCents, want)
	}
}

func TestFakeScaleRejectsUnparsableRequestByClosingConnection(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	srv := fakescale.New(fakescale.Config{Codec: protocol.Dialog02Codec{}, StatusCode: "1", WeightGrams: 1000})
	go srv.Serve(ln) //nolint:errcheck

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// A Dialog04-shaped request sent to a server configured for Dialog02:
	// wrong payload length, so DecodeSetPriceRequest fails server-side.
	if _, err := conn.Write(protocol.BuildFrame([]byte("0001499"))); err != nil {
		t.Fatalf("write: %v", err)
	}

	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("expected the connection to be closed after an unparsable request, got a byte instead")
	}
}
