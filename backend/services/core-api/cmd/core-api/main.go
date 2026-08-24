// Command core-api runs the cloud backend: tenant/user management, the
// product/price catalog, transactions and draft/finalized receipts, backed
// by Postgres and Zitadel.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"scale-app/backend/services/core-api/internal/api"
	"scale-app/backend/services/core-api/internal/auth"
	"scale-app/backend/services/core-api/internal/receipt"
	"scale-app/backend/services/core-api/internal/storage/postgres"
)

func main() {
	ctx := context.Background()

	dsn := requireEnv("DATABASE_URL")
	listenAddr := getEnv("LISTEN_ADDR", ":8081")
	issuerURL := requireEnv("ZITADEL_ISSUER_URL")
	audience := requireEnv("ZITADEL_AUDIENCE")
	zitadelBaseURL := requireEnv("ZITADEL_BASE_URL")
	zitadelServiceToken := requireEnv("ZITADEL_SERVICE_TOKEN")
	smtpAddr := requireEnv("SMTP_ADDR")
	smtpFrom := requireEnv("SMTP_FROM")

	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		log.Fatalf("core-api: %v", err)
	}
	if err := store.Migrate(ctx); err != nil {
		log.Fatalf("core-api: %v", err)
	}

	verifier, err := auth.NewZitadelVerifier(ctx, issuerURL, audience)
	if err != nil {
		log.Fatalf("core-api: %v", err)
	}
	adminClient := &auth.ZitadelAdminClient{BaseURL: zitadelBaseURL, BearerToken: zitadelServiceToken}
	emailSender := &receipt.SMTPSender{Addr: smtpAddr, From: smtpFrom}

	srv := api.NewServer(store, verifier, adminClient, emailSender)

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
