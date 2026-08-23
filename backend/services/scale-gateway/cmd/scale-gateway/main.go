// Command scale-gateway runs the local scale-gateway service: it loads a
// config file describing the scales at this location, connects to each one,
// and exposes an HTTP API the mobile app uses to send a price to a scale and
// read back the resulting transaction.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"scale-app/backend/services/scale-gateway/internal/gateway"
)

func main() {
	configPath := flag.String("config", "config.json", "path to the gateway config file")
	flag.Parse()

	cfg, err := gateway.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("scale-gateway: %v", err)
	}

	entries, err := gateway.BuildEntries(cfg)
	if err != nil {
		log.Fatalf("scale-gateway: %v", err)
	}

	srv := gateway.NewServer(entries)
	srv.ConnectAll(context.Background())

	log.Printf("scale-gateway: listening on %s with %d configured scale(s)", cfg.ListenAddress, len(entries))
	if err := http.ListenAndServe(cfg.ListenAddress, srv); err != nil {
		log.Fatalf("scale-gateway: %v", err)
	}
}
