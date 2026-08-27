package auth

import (
	"context"
	"fmt"
	"sync"
)

// FakeAdminClient is an in-memory AdminClient for tests, with no dependency
// on a live Rauthy instance.
type FakeAdminClient struct {
	mu      sync.Mutex
	nextID  int
	Created map[string]FakeCreatedUser
	Deleted map[string]bool
}

// FakeCreatedUser records the arguments a test passed to CreateVendorUser.
type FakeCreatedUser struct {
	Email       string
	DisplayName string
}

// NewFakeAdminClient returns an empty FakeAdminClient.
func NewFakeAdminClient() *FakeAdminClient {
	return &FakeAdminClient{Created: map[string]FakeCreatedUser{}, Deleted: map[string]bool{}}
}

func (f *FakeAdminClient) CreateVendorUser(ctx context.Context, email, displayName string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	id := fmt.Sprintf("fake-subject-%d", f.nextID)
	f.Created[id] = FakeCreatedUser{Email: email, DisplayName: displayName}
	return id, nil
}

func (f *FakeAdminClient) DeleteUser(ctx context.Context, subjectID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.Created[subjectID]; !ok {
		return fmt.Errorf("auth: fake admin client: unknown subject %q", subjectID)
	}
	f.Deleted[subjectID] = true
	return nil
}
