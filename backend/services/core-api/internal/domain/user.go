package domain

import "time"

// Role distinguishes a tenant's admin (manages users) from its vendor staff
// (operate the app on the market floor).
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleVendor Role = "vendor"
)

// User is one person who can log in to the app, scoped to a single tenant.
// Authentication itself is handled by Zitadel; ZitadelSubjectID links this
// record to the identity Zitadel issues tokens for.
type User struct {
	ID               string    `json:"id"`
	TenantID         string    `json:"tenant_id"`
	ZitadelSubjectID string    `json:"zitadel_subject_id"`
	DisplayName      string    `json:"display_name"`
	Email            string    `json:"email"`
	Role             Role      `json:"role"`
	CreatedAt        time.Time `json:"created_at"`
}
