package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"scale-app/backend/services/scale-gateway/internal/driver"
	"scale-app/backend/services/scale-gateway/internal/protocol"
)

// fakeDriver is a test double for driver.ScaleDriver that returns a canned
// result or error without touching any real network connection.
type fakeDriver struct {
	connectErr error
	result     protocol.TransactionResult
	txErr      error

	lastPricePerKgCents int
	connectCalls        int
	closeCalls          int
}

func (f *fakeDriver) Connect(ctx context.Context) error {
	f.connectCalls++
	return f.connectErr
}

func (f *fakeDriver) Close() error {
	f.closeCalls++
	return nil
}

func (f *fakeDriver) SendPriceAndAwaitTransaction(ctx context.Context, pricePerKgCents int) (protocol.TransactionResult, error) {
	f.lastPricePerKgCents = pricePerKgCents
	if f.txErr != nil {
		return protocol.TransactionResult{}, f.txErr
	}
	return f.result, nil
}

func TestHandleSendTransactionSuccess(t *testing.T) {
	fd := &fakeDriver{result: protocol.TransactionResult{StatusCode: "1", WeightGrams: 1250, PriceCents: 1874}}
	srv := NewServer([]ScaleEntry{{ID: "scale-1", Kind: driver.KindDialogRawTCP, Driver: fd}})

	body, _ := json.Marshal(sendTransactionRequest{PricePerKgCents: 1499})
	req := httptest.NewRequest(http.MethodPost, "/scales/scale-1/transactions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var resp sendTransactionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ScaleID != "scale-1" || resp.WeightGrams != 1250 || resp.PriceCents != 1874 || resp.StatusCode != "1" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fd.lastPricePerKgCents != 1499 {
		t.Fatalf("driver received price %d, want 1499", fd.lastPricePerKgCents)
	}
}

func TestHandleSendTransactionUnknownScale(t *testing.T) {
	srv := NewServer(nil)

	body, _ := json.Marshal(sendTransactionRequest{PricePerKgCents: 1499})
	req := httptest.NewRequest(http.MethodPost, "/scales/does-not-exist/transactions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleSendTransactionInvalidBody(t *testing.T) {
	fd := &fakeDriver{}
	srv := NewServer([]ScaleEntry{{ID: "scale-1", Driver: fd}})

	req := httptest.NewRequest(http.MethodPost, "/scales/scale-1/transactions", bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleSendTransactionNegativePrice(t *testing.T) {
	fd := &fakeDriver{}
	srv := NewServer([]ScaleEntry{{ID: "scale-1", Driver: fd}})

	body, _ := json.Marshal(sendTransactionRequest{PricePerKgCents: -5})
	req := httptest.NewRequest(http.MethodPost, "/scales/scale-1/transactions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleSendTransactionDriverError(t *testing.T) {
	fd := &fakeDriver{txErr: errors.New("scale unreachable")}
	srv := NewServer([]ScaleEntry{{ID: "scale-1", Driver: fd}})

	body, _ := json.Marshal(sendTransactionRequest{PricePerKgCents: 1499})
	req := httptest.NewRequest(http.MethodPost, "/scales/scale-1/transactions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadGateway, rec.Body.String())
	}
}

func TestHandleListScales(t *testing.T) {
	fdOK := &fakeDriver{}
	fdFail := &fakeDriver{connectErr: errors.New("connection refused")}
	srv := NewServer([]ScaleEntry{
		{ID: "scale-ok", Kind: driver.KindDialogRawTCP, Driver: fdOK},
		{ID: "scale-fail", Kind: driver.KindDialogRawTCP, Driver: fdFail},
	})
	srv.ConnectAll(context.Background())

	req := httptest.NewRequest(http.MethodGet, "/scales", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var statuses []scaleStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &statuses); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("got %d statuses, want 2", len(statuses))
	}
	byID := map[string]scaleStatus{}
	for _, s := range statuses {
		byID[s.ID] = s
	}
	if !byID["scale-ok"].Connected {
		t.Fatalf("scale-ok should report connected=true, got %+v", byID["scale-ok"])
	}
	if byID["scale-fail"].Connected {
		t.Fatalf("scale-fail should report connected=false, got %+v", byID["scale-fail"])
	}
	if byID["scale-fail"].LastError == "" {
		t.Fatalf("scale-fail should report a last_error, got %+v", byID["scale-fail"])
	}
}

func TestConnectAllCallsConnectOnEveryDriver(t *testing.T) {
	fd1 := &fakeDriver{}
	fd2 := &fakeDriver{}
	srv := NewServer([]ScaleEntry{{ID: "a", Driver: fd1}, {ID: "b", Driver: fd2}})
	srv.ConnectAll(context.Background())

	if fd1.connectCalls != 1 || fd2.connectCalls != 1 {
		t.Fatalf("expected Connect called once per driver, got fd1=%d fd2=%d", fd1.connectCalls, fd2.connectCalls)
	}
}
