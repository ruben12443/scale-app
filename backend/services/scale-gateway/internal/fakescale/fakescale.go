// Package fakescale simulates a Dialog-speaking scale over a real TCP
// listener, so the rest of the system (scale-gateway, core-api, the mobile
// app) can be exercised end to end without physical hardware. It answers
// every set-price request with a fixed weight and the total that implies,
// exactly as a real scale would after settling.
package fakescale

import (
	"net"

	"scale-app/backend/services/scale-gateway/internal/protocol"
)

// Config configures the canned response a Server hands back for every
// set-price request.
type Config struct {
	Codec       protocol.Codec
	StatusCode  string
	WeightGrams int
	// Logf receives one line per handled request/connection event. Defaults
	// to a no-op if nil.
	Logf func(format string, args ...any)
}

// Server simulates one scale.
type Server struct {
	cfg Config
}

// New returns a Server with the given configuration.
func New(cfg Config) *Server {
	if cfg.Logf == nil {
		cfg.Logf = func(string, ...any) {}
	}
	return &Server{cfg: cfg}
}

// Serve accepts connections on ln, handling each until the peer
// disconnects, until ln itself returns an error (typically because it was
// closed), which Serve then returns.
func (s *Server) Serve(ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	addr := conn.RemoteAddr()
	s.cfg.Logf("fake-scale: connection from %s", addr)

	for {
		frame, err := protocol.ReadFrame(conn)
		if err != nil {
			s.cfg.Logf("fake-scale: %s: read: %v", addr, err)
			return
		}

		pricePerKgCents, err := s.cfg.Codec.DecodeSetPriceRequest(frame)
		if err != nil {
			s.cfg.Logf("fake-scale: %s: decode request: %v", addr, err)
			return
		}

		totalCents := pricePerKgCents * s.cfg.WeightGrams / 1000
		resp, err := s.cfg.Codec.EncodeTransactionResponse(s.cfg.StatusCode, s.cfg.WeightGrams, totalCents)
		if err != nil {
			s.cfg.Logf("fake-scale: %s: encode response: %v", addr, err)
			return
		}

		if _, err := conn.Write(resp); err != nil {
			s.cfg.Logf("fake-scale: %s: write: %v", addr, err)
			return
		}
		s.cfg.Logf("fake-scale: %s: price=%d cents/kg -> weight=%dg total=%d cents",
			addr, pricePerKgCents, s.cfg.WeightGrams, totalCents)
	}
}
