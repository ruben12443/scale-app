// Package storage defines the persistence contracts core-api depends on.
// Two implementations exist: memory (fast, used in unit tests) and postgres
// (production). Callers depend only on the interfaces here.
package storage

import (
	"context"
	"errors"

	"scale-app/backend/services/core-api/internal/domain"
)

// ErrNotFound is returned by Get-style methods when no matching record
// exists.
var ErrNotFound = errors.New("storage: not found")

type TenantRepository interface {
	Create(ctx context.Context, t *domain.Tenant) error
	Get(ctx context.Context, id string) (*domain.Tenant, error)
	List(ctx context.Context) ([]*domain.Tenant, error)
}

type UserRepository interface {
	Create(ctx context.Context, u *domain.User) error
	Get(ctx context.Context, id string) (*domain.User, error)
	GetByRauthySubject(ctx context.Context, subject string) (*domain.User, error)
	ListByTenant(ctx context.Context, tenantID string) ([]*domain.User, error)
	Delete(ctx context.Context, id string) error
}

type ProductRepository interface {
	Create(ctx context.Context, p *domain.Product) error
	Get(ctx context.Context, id string) (*domain.Product, error)
	ListByTenant(ctx context.Context, tenantID string) ([]*domain.Product, error)
	Update(ctx context.Context, p *domain.Product) error
	Delete(ctx context.Context, id string) error
}

type TransactionRepository interface {
	Create(ctx context.Context, t *domain.Transaction) error
	Get(ctx context.Context, id string) (*domain.Transaction, error)
	ListByTenant(ctx context.Context, tenantID string) ([]*domain.Transaction, error)
}

type ReceiptRepository interface {
	Create(ctx context.Context, r *domain.Receipt) error
	Get(ctx context.Context, id string) (*domain.Receipt, error)
	// Update persists changes to an existing receipt's status, number,
	// finalized-at timestamp, and line items.
	Update(ctx context.Context, r *domain.Receipt) error
	ListOpenByUser(ctx context.Context, userID string) ([]*domain.Receipt, error)
	// NextReceiptNumber atomically allocates the next sequential,
	// tenant-scoped receipt number, starting at 1.
	NextReceiptNumber(ctx context.Context, tenantID string) (int, error)
}

type PaymentRepository interface {
	Create(ctx context.Context, p *domain.Payment) error
	Get(ctx context.Context, id string) (*domain.Payment, error)
	GetByStripePaymentIntentID(ctx context.Context, intentID string) (*domain.Payment, error)
	// UpdateStatus updates a payment's status, e.g. from a Stripe webhook
	// event.
	UpdateStatus(ctx context.Context, id string, status domain.PaymentStatus) error
}

// Store aggregates every repository core-api depends on. Both the memory and
// postgres packages provide a type implementing it.
type Store interface {
	Tenants() TenantRepository
	Users() UserRepository
	Products() ProductRepository
	Transactions() TransactionRepository
	Receipts() ReceiptRepository
	Payments() PaymentRepository
}
