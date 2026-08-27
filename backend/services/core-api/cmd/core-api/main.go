// Command core-api runs the cloud backend: tenant/user management, the
// product/price catalog, transactions and draft/finalized receipts, backed
// by Postgres and Rauthy.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"scale-app/backend/services/core-api/internal/api"
	"scale-app/backend/services/core-api/internal/auth"
	"scale-app/backend/services/core-api/internal/payment"
	"scale-app/backend/services/core-api/internal/receipt"
	"scale-app/backend/services/core-api/internal/storage/postgres"
)

func main() {
	ctx := context.Background()

	dsn := requireEnv("DATABASE_URL")
	listenAddr := getEnv("LISTEN_ADDR", ":8081")
	issuerURL := requireEnv("RAUTHY_ISSUER_URL")
	// RAUTHY_DISCOVERY_URL defaults to issuerURL: they're only ever
	// different when core-api reaches Rauthy over an internal address
	// (e.g. a Docker Compose service name) that differs from Rauthy's
	// externally-configured issuer (e.g. localhost or a LAN IP for phone
	// testing) — see NewRauthyVerifier's doc comment.
	discoveryURL := getEnv("RAUTHY_DISCOVERY_URL", issuerURL)
	audience := requireEnv("RAUTHY_AUDIENCE")
	rauthyBaseURL := requireEnv("RAUTHY_BASE_URL")
	rauthyAPIKey := requireEnv("RAUTHY_API_KEY")
	smtpAddr := requireEnv("SMTP_ADDR")
	smtpFrom := requireEnv("SMTP_FROM")
	stripeSecretKey := requireEnv("STRIPE_SECRET_KEY")
	stripeWebhookSecret := requireEnv("STRIPE_WEBHOOK_SECRET")
	currency := getEnv("CURRENCY", "chf")

	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		log.Fatalf("core-api: %v", err)
	}
	if err := store.Migrate(ctx); err != nil {
		log.Fatalf("core-api: %v", err)
	}

	verifier, err := auth.NewRauthyVerifier(ctx, discoveryURL, issuerURL, audience)
	if err != nil {
		log.Fatalf("core-api: %v", err)
	}
	adminClient := &auth.RauthyAdminClient{BaseURL: rauthyBaseURL, APIKey: rauthyAPIKey}
	emailSender := &receipt.SMTPSender{Addr: smtpAddr, From: smtpFrom}
	processor := payment.NewStripeProcessor(stripeSecretKey)

	srv := api.NewServer(api.ServerConfig{
		Store:               store,
		Verifier:            verifier,
		Admin:               adminClient,
		EmailSender:         emailSender,
		PaymentProcessor:    processor,
		StripeWebhookSecret: stripeWebhookSecret,
		Currency:            currency,
	})

	log.Printf("core-api: listening on %s", listenAddr)
	if err := http.ListenAndServe(listenAddr, srv); err != nil {
		log.Fatalf("core-api: %v", err)
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("core-api: required environment variable %s is not set", key)
	}
	return v
}
