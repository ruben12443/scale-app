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

	// holderID/holderName/claimExpiresAt track which vendor currently has this
	// scale claimed, so two vendors can never be sending prices to the same
	// physical scale at once. holderID empty (or claimExpiresAt in the past)
	// means unclaimed.
	holderID       string
	holderName     string
	claimExpiresAt time.Time
}

// claimTTL bounds how long a scale claim survives without being renewed by
// its holder (a claim, or a transaction from the same holder_id). It's a
// server-side backstop, deliberately longer than the mobile app's own 7s
// on-screen-inactivity timeout: it only matters if a client vanishes without
// releasing (crash, lost network, the app failing to notice a locked
// screen).
const claimTTL = 20 * time.Second

// activeHolderLocked returns the current holder id and whether the claim is
// still active as of now. Callers must hold st.mu.
func (st *scaleState) activeHolderLocked(now time.Time) (holderID string, active bool) {
	if st.holderID == "" || now.After(st.claimExpiresAt) {
		return "", false
	}
	return st.holderID, true
}

// Server exposes configured scales over HTTP.
type Server struct {
	mux    *http.ServeMux
	scales map[string]*scaleState

	// now is overridden in tests to make claim expiry deterministic.
	now func() time.Time
}

// NewServer builds a Server for the given scale entries. Drivers are not
// connected automatically; call ConnectAll.
func NewServer(entries []ScaleEntry) *Server {
	s := &Server{
		mux:    http.NewServeMux(),
		scales: make(map[string]*scaleState, len(entries)),
		now:    time.Now,
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
	s.mux.HandleFunc("POST /scales/{id}/claim", s.handleClaimScale)
	s.mux.HandleFunc("POST /scales/{id}/release", s.handleReleaseScale)
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
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Connected  bool   `json:"connected"`
	LastError  string `json:"last_error,omitempty"`
	HeldByID   string `json:"held_by_id,omitempty"`
	HeldByName string `json:"held_by_name,omitempty"`
}

func (s *Server) handleListScales(w http.ResponseWriter, r *http.Request) {
	now := s.now()
	statuses := make([]scaleStatus, 0, len(s.scales))
	for _, st := range s.scales {
		st.mu.Lock()
		holder, active := st.activeHolderLocked(now)
		status := scaleStatus{
			ID:        st.id,
			Kind:      string(st.kind),
			Connected: st.connected,
			LastError: st.lastError,
		}
		if active {
			status.HeldByID = holder
			status.HeldByName = st.holderName
		}
		st.mu.Unlock()
		statuses = append(statuses, status)
	}
	writeJSON(w, http.StatusOK, statuses)
}

type claimRequest struct {
	HolderID   string `json:"holder_id"`
	HolderName string `json:"holder_name,omitempty"`
}

type claimResponse struct {
	ScaleID    string    `json:"scale_id"`
	HolderID   string    `json:"holder_id"`
	HolderName string    `json:"holder_name,omitempty"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type claimConflictResponse struct {
	Error      string `json:"error"`
	HeldByID   string `json:"held_by_id"`
	HeldByName string `json:"held_by_name,omitempty"`
}

// handleClaimScale grants the requesting vendor exclusive use of a scale, so
// a second vendor's app can't also start weighing against it. Re-claiming
// with the same holder_id (a renewal, e.g. from the mobile app's periodic
// keep-alive) always succeeds and extends the claim.
func (s *Server) handleClaimScale(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	st, ok := s.scales[id]
	if !ok {
		writeError(w, http.StatusNotFound, "unknown scale id")
		return
	}

	var req claimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.HolderID == "" {
		writeError(w, http.StatusBadRequest, "holder_id is required")
		return
	}

	now := s.now()
	st.mu.Lock()
	defer st.mu.Unlock()

	if holder, active := st.activeHolderLocked(now); active && holder != req.HolderID {
		display := st.holderName
		if display == "" {
			display = holder
		}
		writeJSON(w, http.StatusConflict, claimConflictResponse{
			Error:      "scale is in use by another vendor",
			HeldByID:   holder,
			HeldByName: display,
		})
		return
	}

	st.holderID = req.HolderID
	st.holderName = req.HolderName
	st.claimExpiresAt = now.Add(claimTTL)

	writeJSON(w, http.StatusOK, claimResponse{
		ScaleID:    id,
		HolderID:   st.holderID,
		HolderName: st.holderName,
		ExpiresAt:  st.claimExpiresAt,
	})
}

type releaseRequest struct {
	HolderID string `json:"holder_id"`
}

type releaseResponse struct {
	Released bool `json:"released"`
}

// handleReleaseScale gives up a claim. It's idempotent and never errors on a
// mismatched holder: releasing is only ever a safety/cleanup action (product
// added, inactivity timeout, screen lock, navigating away), so a stale or
// already-lost claim is a silent no-op rather than a failure the caller has
// to handle.
func (s *Server) handleReleaseScale(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	st, ok := s.scales[id]
	if !ok {
		writeError(w, http.StatusNotFound, "unknown scale id")
		return
	}

	var req releaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	now := s.now()
	st.mu.Lock()
	defer st.mu.Unlock()

	_, active := st.activeHolderLocked(now)
	released := !active || st.holderID == req.HolderID
	if released {
		st.holderID = ""
		st.holderName = ""
		st.claimExpiresAt = time.Time{}
	}

	writeJSON(w, http.StatusOK, releaseResponse{Released: released})
}

type sendTransactionRequest struct {
	PricePerKgCents int    `json:"price_per_kg_cents"`
	HolderID        string `json:"holder_id"`
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
	if req.HolderID == "" {
		writeError(w, http.StatusBadRequest, "holder_id is required")
		return
	}

	// A transaction only proceeds for whoever currently holds the scale's
	// claim (see handleClaimScale) — this is the actual enforcement point
	// that keeps two vendors from weighing on the same scale at once. It also
	// renews the claim, so an actively-selling vendor's claim never expires
	// mid-sale purely from the claimTTL backstop.
	now := s.now()
	st.mu.Lock()
	if holder, active := st.activeHolderLocked(now); active && holder != req.HolderID {
		st.mu.Unlock()
		writeError(w, http.StatusConflict, "scale is claimed by another vendor")
		return
	}
	st.holderID = req.HolderID
	st.claimExpiresAt = now.Add(claimTTL)
	st.mu.Unlock()

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
