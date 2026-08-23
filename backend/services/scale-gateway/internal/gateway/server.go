// Package gateway wires configured scale drivers to a small local HTTP API
// that the rest of the system (mobile app, backend services) uses to send a
// price to a scale and read back the resulting transaction, without knowing
// which protocol variant a given scale actually speaks.
package gateway

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"scale-app/backend/services/scale-gateway/internal/driver"
)

type scaleState struct {
	id   string
	kind driver.Kind
	drv  driver.ScaleDriver

	// txMu serializes transactions against one scale: a scale can only run one
	// weigh operation at a time.
	txMu sync.Mutex

	mu        sync.Mutex
	connected bool
	lastError string
}

// Server exposes configured scales over HTTP.
type Server struct {
	mux    *http.ServeMux
	scales map[string]*scaleState
}

// NewServer builds a Server for the given scale entries. Drivers are not
// connected automatically; call ConnectAll.
func NewServer(entries []ScaleEntry) *Server {
	s := &Server{
		mux:    http.NewServeMux(),
		scales: make(map[string]*scaleState, len(entries)),
	}
	for _, e := range entries {
		s.scales[e.ID] = &scaleState{id: e.ID, kind: e.Kind, drv: e.Driver}
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /scales", s.handleListScales)
	s.mux.HandleFunc("POST /scales/{id}/transactions", s.handleSendTransaction)
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// ConnectAll attempts to connect every configured scale's driver. Failures are
// recorded per scale (visible via GET /scales) rather than causing ConnectAll
// itself to fail, so one unreachable scale doesn't block the gateway from
// serving the others.
func (s *Server) ConnectAll(ctx context.Context) {
	for _, st := range s.scales {
		st.connect(ctx)
	}
}

func (st *scaleState) connect(ctx context.Context) {
	err := st.drv.Connect(ctx)
	st.mu.Lock()
	defer st.mu.Unlock()
	st.connected = err == nil
	if err != nil {
		st.lastError = err.Error()
		log.Printf("gateway: scale %q: connect failed: %v", st.id, err)
	} else {
		st.lastError = ""
	}
}

type scaleStatus struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Connected bool   `json:"connected"`
	LastError string `json:"last_error,omitempty"`
}

func (s *Server) handleListScales(w http.ResponseWriter, r *http.Request) {
	statuses := make([]scaleStatus, 0, len(s.scales))
	for _, st := range s.scales {
		st.mu.Lock()
		statuses = append(statuses, scaleStatus{
			ID:        st.id,
			Kind:      string(st.kind),
			Connected: st.connected,
			LastError: st.lastError,
		})
		st.mu.Unlock()
	}
	writeJSON(w, http.StatusOK, statuses)
}

type sendTransactionRequest struct {
	PricePerKgCents int `json:"price_per_kg_cents"`
}

type sendTransactionResponse struct {
	ScaleID     string `json:"scale_id"`
	StatusCode  string `json:"status_code"`
	WeightGrams int    `json:"weight_grams"`
	PriceCents  int    `json:"price_cents"`
}

// transactionTimeout bounds how long a single weigh operation is allowed to
// take end-to-end, independent of the driver's own I/O timeout.
const transactionTimeout = 10 * time.Second

func (s *Server) handleSendTransaction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	st, ok := s.scales[id]
	if !ok {
		writeError(w, http.StatusNotFound, "unknown scale id")
		return
	}

	var req sendTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.PricePerKgCents < 0 {
		writeError(w, http.StatusBadRequest, "price_per_kg_cents must not be negative")
		return
	}

	// Only one transaction may be in flight against a given scale at a time.
	st.txMu.Lock()
	defer st.txMu.Unlock()

	ctx, cancel := context.WithTimeout(r.Context(), transactionTimeout)
	defer cancel()

	result, err := st.drv.SendPriceAndAwaitTransaction(ctx, req.PricePerKgCents)
	if err != nil {
		st.mu.Lock()
		st.connected = false
		st.lastError = err.Error()
		st.mu.Unlock()
		writeError(w, http.StatusBadGateway, "scale transaction failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, sendTransactionResponse{
		ScaleID:     id,
		StatusCode:  result.StatusCode,
		WeightGrams: result.WeightGrams,
		PriceCents:  result.PriceCents,
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}
