// Package postgres provides the production storage.Store backed by
// PostgreSQL, using pgx (pure Go, no cgo required).
package postgres

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"scale-app/backend/services/core-api/internal/storage"
)

// pgxTx is the subset of pgx.Tx used for writing inside a transaction.
// Satisfied by pgx.Tx.
type pgxTx interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// pgxQuerier is the subset used for read-only queries, satisfied by both
// *pgxpool.Pool and pgx.Tx, so read helpers work inside or outside a
// transaction.
type pgxQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

//go:embed schema.sql
var schemaSQL string

// Store is a Postgres-backed implementation of storage.Store.
type Store struct {
	pool *pgxpool.Pool
}

// Open connects to Postgres at connString. Callers should call Migrate once
// before using the store against a fresh database.
func Open(ctx context.Context, connString string) (*Store, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("postgres: open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close releases the underlying connection pool.
func (s *Store) Close() { s.pool.Close() }

// Migrate applies the embedded schema. It is idempotent.
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, schemaSQL); err != nil {
		return fmt.Errorf("postgres: migrate: %w", err)
	}
	return nil
}

func (s *Store) Tenants() storage.TenantRepository           { return (*tenantRepo)(s) }
func (s *Store) Users() storage.UserRepository               { return (*userRepo)(s) }
func (s *Store) Products() storage.ProductRepository         { return (*productRepo)(s) }
func (s *Store) Transactions() storage.TransactionRepository { return (*transactionRepo)(s) }
func (s *Store) Receipts() storage.ReceiptRepository         { return (*receiptRepo)(s) }

func noRowsToNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return storage.ErrNotFound
	}
	return err
}
