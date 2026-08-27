// Package memory provides an in-memory storage.Store, used for fast unit
// tests and local development without a database.
package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"scale-app/backend/services/core-api/internal/domain"
	"scale-app/backend/services/core-api/internal/storage"
)

// Store is an in-memory implementation of storage.Store. It is safe for
// concurrent use.
type Store struct {
	mu sync.Mutex

	tenants      map[string]domain.Tenant
	users        map[string]domain.User
	products     map[string]domain.Product
	transactions map[string]domain.Transaction
	receipts     map[string]domain.Receipt
	payments     map[string]domain.Payment

	nextReceiptNumber map[string]int // tenantID -> next number
}

// New returns an empty in-memory store.
func New() *Store {
	return &Store{
		tenants:           make(map[string]domain.Tenant),
		users:             make(map[string]domain.User),
		products:          make(map[string]domain.Product),
		transactions:      make(map[string]domain.Transaction),
		receipts:          make(map[string]domain.Receipt),
		payments:          make(map[string]domain.Payment),
		nextReceiptNumber: make(map[string]int),
	}
}

func (s *Store) Tenants() storage.TenantRepository           { return (*tenantRepo)(s) }
func (s *Store) Users() storage.UserRepository               { return (*userRepo)(s) }
func (s *Store) Products() storage.ProductRepository         { return (*productRepo)(s) }
func (s *Store) Transactions() storage.TransactionRepository { return (*transactionRepo)(s) }
func (s *Store) Receipts() storage.ReceiptRepository         { return (*receiptRepo)(s) }
func (s *Store) Payments() storage.PaymentRepository         { return (*paymentRepo)(s) }

type tenantRepo Store

func (r *tenantRepo) Create(ctx context.Context, t *domain.Tenant) error {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tenants[t.ID] = *t
	return nil
}

func (r *tenantRepo) Get(ctx context.Context, id string) (*domain.Tenant, error) {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tenants[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return &t, nil
}

func (r *tenantRepo) List(ctx context.Context) ([]*domain.Tenant, error) {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*domain.Tenant, 0, len(s.tenants))
	for _, t := range s.tenants {
		t := t
		out = append(out, &t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

type userRepo Store

func (r *userRepo) Create(ctx context.Context, u *domain.User) error {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[u.ID] = *u
	return nil
}

func (r *userRepo) Get(ctx context.Context, id string) (*domain.User, error) {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return &u, nil
}

func (r *userRepo) GetByRauthySubject(ctx context.Context, subject string) (*domain.User, error) {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.users {
		if u.RauthySubjectID == subject {
			u := u
			return &u, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (r *userRepo) ListByTenant(ctx context.Context, tenantID string) ([]*domain.User, error) {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*domain.User, 0)
	for _, u := range s.users {
		if u.TenantID == tenantID {
			u := u
			out = append(out, &u)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *userRepo) Delete(ctx context.Context, id string) error {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[id]; !ok {
		return storage.ErrNotFound
	}
	delete(s.users, id)
	return nil
}

type productRepo Store

func (r *productRepo) Create(ctx context.Context, p *domain.Product) error {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.products[p.ID] = *p
	return nil
}

func (r *productRepo) Get(ctx context.Context, id string) (*domain.Product, error) {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.products[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return &p, nil
}

func (r *productRepo) ListByTenant(ctx context.Context, tenantID string) ([]*domain.Product, error) {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*domain.Product, 0)
	for _, p := range s.products {
		if p.TenantID == tenantID {
			p := p
			out = append(out, &p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *productRepo) Update(ctx context.Context, p *domain.Product) error {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.products[p.ID]; !ok {
		return storage.ErrNotFound
	}
	s.products[p.ID] = *p
	return nil
}

func (r *productRepo) Delete(ctx context.Context, id string) error {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.products[id]; !ok {
		return storage.ErrNotFound
	}
	delete(s.products, id)
	return nil
}

type transactionRepo Store

func (r *transactionRepo) Create(ctx context.Context, t *domain.Transaction) error {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transactions[t.ID] = *t
	return nil
}

func (r *transactionRepo) Get(ctx context.Context, id string) (*domain.Transaction, error) {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.transactions[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return &t, nil
}

func (r *transactionRepo) ListByTenant(ctx context.Context, tenantID string) ([]*domain.Transaction, error) {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*domain.Transaction, 0)
	for _, t := range s.transactions {
		if t.TenantID == tenantID {
			t := t
			out = append(out, &t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

type receiptRepo Store

func (r *receiptRepo) Create(ctx context.Context, rc *domain.Receipt) error {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.receipts[rc.ID] = cloneReceipt(rc)
	return nil
}

func (r *receiptRepo) Get(ctx context.Context, id string) (*domain.Receipt, error) {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	rc, ok := s.receipts[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	out := cloneReceipt(&rc)
	return &out, nil
}

func (r *receiptRepo) Update(ctx context.Context, rc *domain.Receipt) error {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.receipts[rc.ID]; !ok {
		return storage.ErrNotFound
	}
	s.receipts[rc.ID] = cloneReceipt(rc)
	return nil
}

func (r *receiptRepo) ListOpenByUser(ctx context.Context, userID string) ([]*domain.Receipt, error) {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*domain.Receipt, 0)
	for _, rc := range s.receipts {
		if rc.UserID == userID && rc.Status == domain.ReceiptStatusDraft {
			cp := cloneReceipt(&rc)
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (r *receiptRepo) NextReceiptNumber(ctx context.Context, tenantID string) (int, error) {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.nextReceiptNumber[tenantID] + 1
	s.nextReceiptNumber[tenantID] = next
	return next, nil
}

// cloneReceipt copies a Receipt including its TransactionIDs slice, so
// callers can't mutate the store's internal state through a returned/stored
// pointer's backing array.
func cloneReceipt(rc *domain.Receipt) domain.Receipt {
	out := *rc
	out.TransactionIDs = append([]string(nil), rc.TransactionIDs...)
	return out
}

type paymentRepo Store

func (r *paymentRepo) Create(ctx context.Context, p *domain.Payment) error {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.payments[p.ID] = *p
	return nil
}

func (r *paymentRepo) Get(ctx context.Context, id string) (*domain.Payment, error) {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.payments[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return &p, nil
}

func (r *paymentRepo) GetByStripePaymentIntentID(ctx context.Context, intentID string) (*domain.Payment, error) {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.payments {
		if p.StripePaymentIntentID == intentID {
			p := p
			return &p, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (r *paymentRepo) UpdateStatus(ctx context.Context, id string, status domain.PaymentStatus) error {
	s := (*Store)(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.payments[id]
	if !ok {
		return storage.ErrNotFound
	}
	p.Status = status
	p.UpdatedAt = time.Now().UTC()
	s.payments[id] = p
	return nil
}
