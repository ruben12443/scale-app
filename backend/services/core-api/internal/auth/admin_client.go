package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// AdminClient manages user identities in the external identity provider
// (Rauthy), separate from the local User record core-api keeps for
// tenant/role scoping.
type AdminClient interface {
	// CreateVendorUser creates a new user in Rauthy and returns its subject
	// (user) ID. Rauthy has no organization/tenant concept of its own (see
	// domain.Tenant's doc comment) so, unlike the Zitadel admin client this
	// replaced, there's no org/tenant scoping parameter to pass — core-api's
	// own User record is solely responsible for that.
	CreateVendorUser(ctx context.Context, email, displayName string) (subjectID string, err error)
	// DeleteUser deletes the user with the given subject ID.
	DeleteUser(ctx context.Context, subjectID string) error
}

// RauthyAdminClient implements AdminClient against Rauthy's admin User API.
//
// Endpoint shapes follow Rauthy's OpenAPI spec (POST /users, DELETE
// /users/{id}) — fetched from a live instance
// (https://iam.sebadob.dev/auth/v1/docs/openapi.json) while building this,
// not guessed. Authentication uses Rauthy's API Key scheme (`Authorization:
// API-Key <name>$<secret>`), not a bearer token — API keys are how Rauthy
// expects automated/service callers to authenticate, since a normal user
// session additionally needs CSRF handling meant for browsers. See the root
// docker-compose.yml's rauthy service and rauthy-bootstrap/api_keys.json for
// how the key this client uses is provisioned.
//
// This has not been exercised against a real Rauthy instance — no live
// instance or credentials were available while building it — so treat it as
// unverified until checked against one.
type RauthyAdminClient struct {
	// BaseURL is the Rauthy instance's API base, e.g.
	// "http://rauthy:8080/auth/v1".
	BaseURL string
	// APIKey authenticates as a bootstrapped Rauthy API key with `Users`
	// create/delete access, formatted as "<name>$<secret>".
	APIKey string
	// HTTPClient is used for requests; http.DefaultClient if nil.
	HTTPClient *http.Client
}

func (r *RauthyAdminClient) httpClient() *http.Client {
	if r.HTTPClient != nil {
		return r.HTTPClient
	}
	return http.DefaultClient
}

// newUserRequest mirrors Rauthy's NewUserRequest. roles/groups are left
// empty: role authorization is entirely core-api's own concern (see
// domain.User's doc comment), not something Rauthy needs to know about.
type newUserRequest struct {
	Email      string   `json:"email"`
	GivenName  string   `json:"given_name,omitempty"`
	FamilyName string   `json:"family_name,omitempty"`
	Language   string   `json:"language"`
	Roles      []string `json:"roles"`
}

type userResponse struct {
	ID string `json:"id"`
}

// CreateVendorUser creates the user with no password set. Rauthy's own
// first-login flow takes it from there: the new user gets a password-reset
// email to set their own password, the same way it works for a user created
// through Rauthy's Admin UI (see the "First Start" section of its docs) —
// there's no Zitadel-style "temporary password the admin sees" step to
// replicate, since Rauthy's user-creation API has no password field at all.
func (r *RauthyAdminClient) CreateVendorUser(ctx context.Context, email, displayName string) (string, error) {
	reqBody := newUserRequest{
		Email:      email,
		GivenName:  displayName,
		FamilyName: displayName,
		Language:   "en",
		Roles:      []string{},
	}

	var respBody userResponse
	if err := r.doJSON(ctx, http.MethodPost, "/users", reqBody, &respBody); err != nil {
		return "", fmt.Errorf("auth: create rauthy user: %w", err)
	}
	return respBody.ID, nil
}

func (r *RauthyAdminClient) DeleteUser(ctx context.Context, subjectID string) error {
	if err := r.doJSON(ctx, http.MethodDelete, "/users/"+subjectID, nil, nil); err != nil {
		return fmt.Errorf("auth: delete rauthy user: %w", err)
	}
	return nil
}

func (r *RauthyAdminClient) doJSON(ctx context.Context, method, path string, reqBody, respBody any) error {
	var bodyReader io.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, r.BaseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "API-Key "+r.APIKey)
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := r.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(data))
	}

	if respBody != nil {
		if err := json.NewDecoder(resp.Body).Decode(respBody); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
