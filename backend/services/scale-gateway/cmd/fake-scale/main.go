// Command fake-scale simulates a Dialog-speaking scale over TCP, standing
// in for a real one (and the serial-to-Ethernet adapter in front of it) so
// the rest of the system can be tested locally without physical hardware.
// It answers every set-price request with a fixed weight and the total
// price that implies, exactly as a real scale would after settling.
package main

import (
	"flag"
	"log"
	"net"

	"scale-app/backend/services/scale-gateway/internal/fakescale"
	"scale-app/backend/services/scale-gateway/internal/protocol"
)

func main() {
	addr := flag.String("addr", ":9999", "address to listen on, simulating a serial-to-Ethernet adapter")
	variant := flag.String("variant", "02", `dialog variant to speak: "02" or "04"`)
	weightGrams := flag.Int("weight-grams", 1250, "canned weight to report on every transaction, in grams")
	statusCode := flag.String("status", "", `canned status code to report (defaults to "1" for variant 02, "100" for variant 04)`)
	flag.Parse()

	var codec protocol.Codec
	var defaultStatus string
	switch *variant {
	case "02":
		codec = protocol.Dialog02Codec{}
		defaultStatus = "1"
	case "04":
		codec = protocol.Dialog04Codec{}
		defaultStatus = "100"
	default:
		log.Fatalf("fake-scale: unknown -variant %q, want \"02\" or \"04\"", *variant)
	}
	status := *statusCode
	if status == "" {
		status = defaultStatus
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("fake-scale: listen: %v", err)
	}
	log.Printf("fake-scale: speaking dialog%s on %s (weight=%dg, status=%q)", *variant, *addr, *weightGrams, status)

	srv := fakescale.New(fakescale.Config{
		Codec:       codec,
		StatusCode:  status,
		WeightGrams: *weightGrams,
		Logf:        log.Printf,
	})
	if err := srv.Serve(ln); err != nil {
		log.Fatalf("fake-scale: serve: %v", err)
	}
}
