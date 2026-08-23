package driver

import (
	"context"
	"net"
	"testing"
	"time"

	"scale-app/backend/services/scale-gateway/internal/protocol"
)

// fakeScale reads exactly one frame written by the driver and, if
// respondWith is non-nil, writes it back as the scale's response. It runs on
// its own goroutine so the test can drive the driver from the other end of a
// net.Pipe without deadlocking (net.Pipe is unbuffered/synchronous).
func fakeScale(t *testing.T, conn net.Conn, respondWith []byte) <-chan []byte {
	t.Helper()
	received := make(chan []byte, 1)
	go func() {
		frame, err := readFrame(conn)
		if err != nil {
			received <- nil
			return
		}
		received <- frame
		if respondWith != nil {
			_, _ = conn.Write(respondWith)
		}
	}()
	return received
}

func TestDialogTCPDriverDialog02RoundTrip(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	wantResponse := protocol.BuildFrame([]byte("112501874"))
	received := fakeScale(t, serverConn, wantResponse)

	drv, err := newDialogDriverForConn(clientConn, DialogVariant02)
	if err != nil {
		t.Fatalf("newDialogDriverForConn returned error: %v", err)
	}

	result, err := drv.SendPriceAndAwaitTransaction(context.Background(), 1499)
	if err != nil {
		t.Fatalf("SendPriceAndAwaitTransaction returned error: %v", err)
	}

	wantRequest := protocol.BuildFrame([]byte("01499"))
	gotRequest := <-received
	if string(gotRequest) != string(wantRequest) {
		t.Fatalf("scale received % X, want % X", gotRequest, wantRequest)
	}

	if result.StatusCode != "1" || result.WeightGrams != 1250 || result.PriceCents != 1874 {
		t.Fatalf("SendPriceAndAwaitTransaction result = %+v, want status=1 weight=1250 price=1874", result)
	}
}

func TestDialogTCPDriverDialog04RoundTrip(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	wantResponse := protocol.BuildFrame([]byte("1001250001874"))
	received := fakeScale(t, serverConn, wantResponse)

	drv, err := newDialogDriverForConn(clientConn, DialogVariant04)
	if err != nil {
		t.Fatalf("newDialogDriverForConn returned error: %v", err)
	}

	result, err := drv.SendPriceAndAwaitTransaction(context.Background(), 1499)
	if err != nil {
		t.Fatalf("SendPriceAndAwaitTransaction returned error: %v", err)
	}

	wantRequest := protocol.BuildFrame([]byte("0001499"))
	gotRequest := <-received
	if string(gotRequest) != string(wantRequest) {
		t.Fatalf("scale received % X, want % X", gotRequest, wantRequest)
	}

	if result.StatusCode != "100" || result.WeightGrams != 1250 || result.PriceCents != 1874 {
		t.Fatalf("SendPriceAndAwaitTransaction result = %+v, want status=100 weight=1250 price=1874", result)
	}
}

func TestDialogTCPDriverNotConnected(t *testing.T) {
	drv, err := NewDialogTCPDriver("127.0.0.1:0", DialogVariant02)
	if err != nil {
		t.Fatalf("NewDialogTCPDriver returned error: %v", err)
	}
	_, err = drv.SendPriceAndAwaitTransaction(context.Background(), 1499)
	if err == nil {
		t.Fatal("expected error when sending without connecting first, got nil")
	}
}

func TestDialogTCPDriverMalformedResponse(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	badResponse := protocol.BuildFrame([]byte("112501874"))
	badResponse[len(badResponse)-1] ^= 0xFF // corrupt BCC
	fakeScale(t, serverConn, badResponse)

	drv, err := newDialogDriverForConn(clientConn, DialogVariant02)
	if err != nil {
		t.Fatalf("newDialogDriverForConn returned error: %v", err)
	}

	if _, err := drv.SendPriceAndAwaitTransaction(context.Background(), 1499); err == nil {
		t.Fatal("expected error for a response with a corrupted BCC, got nil")
	}
}

func TestDialogTCPDriverTimesOutWhenScaleDoesNotRespond(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	// Fake scale reads the request but never responds.
	fakeScale(t, serverConn, nil)

	drv, err := newDialogDriverForConn(clientConn, DialogVariant02)
	if err != nil {
		t.Fatalf("newDialogDriverForConn returned error: %v", err)
	}
	drv.IOTimeout = 50 * time.Millisecond

	start := time.Now()
	_, err = drv.SendPriceAndAwaitTransaction(context.Background(), 1499)
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("SendPriceAndAwaitTransaction took %v, expected it to time out quickly", elapsed)
	}
}

func TestDialogTCPDriverRespectsContextDeadline(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	fakeScale(t, serverConn, nil)

	drv, err := newDialogDriverForConn(clientConn, DialogVariant02)
	if err != nil {
		t.Fatalf("newDialogDriverForConn returned error: %v", err)
	}
	drv.IOTimeout = 10 * time.Second // long enough that only the context deadline matters

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = drv.SendPriceAndAwaitTransaction(ctx, 1499)
	if err == nil {
		t.Fatal("expected an error from the short context deadline, got nil")
	}
}

func TestCodecForUnknownVariant(t *testing.T) {
	if _, err := codecFor("99"); err == nil {
		t.Fatal("expected an error for an unknown dialog variant, got nil")
	}
}

func TestNewRejectsUnknownKind(t *testing.T) {
	_, err := New(Config{Kind: "carrier-pigeon"})
	if err == nil {
		t.Fatal("expected an error for an unknown driver kind, got nil")
	}
}

func TestNewRejectsRIKNotImplemented(t *testing.T) {
	_, err := New(Config{Kind: KindRIK})
	if err == nil {
		t.Fatal("expected an error since the RIK driver is not implemented yet, got nil")
	}
}

func TestNewBuildsDialogRawTCPDriver(t *testing.T) {
	drv, err := New(Config{Kind: KindDialogRawTCP, DialogVariant: DialogVariant02, Address: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if _, ok := drv.(*DialogTCPDriver); !ok {
		t.Fatalf("New returned %T, want *DialogTCPDriver", drv)
	}
}
