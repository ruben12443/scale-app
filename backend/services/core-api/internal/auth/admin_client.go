package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// AdminClient manages user identities in the external identity provider
// (Zitadel), separate from the local User record core-api keeps for
// tenant/role scoping.
type AdminClient interface {
	// CreateVendorUser creates a new human user under the given Zitadel
	// organization and returns Zitadel's subject/user ID for it.
	CreateVendorUser(ctx context.Context, orgID, email, displayName string) (subjectID string, err error)
	// DeleteUser deletes the user with the given subject ID. Per Zitadel's
	// semantics this marks the user deleted and invalidates all of its
	// sessions and tokens.
	DeleteUser(ctx context.Context, subjectID string) error
}

// ZitadelAdminClient implements AdminClient against Zitadel's v2 User API.
//
// Endpoint shapes follow:
//   - https://zitadel.com/docs/apis/resources/user_service_v2/user-service-add-human-user
//   - https://zitadel.com/docs/reference/api/user/zitadel.user.v2.UserService.DeleteUser
//
// This has not been exercised against a real Zitadel instance — no live
// instance or credentials were available while building it — so treat it as
// unverified until checked against one.
type ZitadelAdminClient struct {
	// BaseURL is the Zitadel instance's custom domain, e.g.
	// "https://your-instance.zitadel.cloud".
	BaseURL string
	// BearerToken authenticates as a service account with user-management
	// permissions.
	BearerToken string
	// HTTPClient is used for requests; http.DefaultClient if nil.
	HTTPClient *http.Client
}

func (z *ZitadelAdminClient) httpClient() *http.Client {
	if z.HTTPClient != nil {
		return z.HTTPClient
	}
	return http.DefaultClient
}

type createHumanUserRequest struct {
	Username     string                      `json:"username"`
	Profile      createHumanUserProfile      `json:"profile"`
	Email        createHumanUserEmail        `json:"email"`
	Password     createHumanUserPassword     `json:"password"`
	Organization createHumanUserOrganization `json:"organization"`
}

type createHumanUserProfile struct {
	GivenName   string `json:"givenName"`
	FamilyName  string `json:"familyName"`
	DisplayName string `json:"displayName"`
}

type createHumanUserEmail struct {
	Email      string `json:"email"`
	IsVerified bool   `json:"isVerified"`
}

type createHumanUserPassword struct {
	Password       string `json:"password"`
	ChangeRequired bool   `json:"changeRequired"`
}

type createHumanUserOrganization struct {
	OrgID string `json:"orgId"`
}

type createHumanUserResponse struct {
	UserID string `json:"userId"`
}

// CreateVendorUser creates the user with a random temporary password that
// must be changed on first login, since the admin creating the account
// generally doesn't hand-pick a password for someone else.
func (z *ZitadelAdminClient) CreateVendorUser(ctx context.Context, orgID, email, displayName string) (string, error) {
	reqBody := createHumanUserRequest{
		Username: email,
		Profile: createHumanUserProfile{
			GivenName:   displayName,
			FamilyName:  displayName,
			DisplayName: displayName,
		},
		Email: createHumanUserEmail{Email: email, IsVerified: false},
		Password: createHumanUserPassword{
			Password:       generateTemporaryPassword(),
			ChangeRequired: true,
		},
		Organization: createHumanUserOrganization{OrgID: orgID},
	}

	var respBody createHumanUserResponse
	if err := z.doJSON(ctx, http.MethodPost, "/v2/users/human", reqBody, &respBody); err != nil {
		return "", fmt.Errorf("auth: create zitadel user: %w", err)
	}
	return respBody.UserID, nil
}

func (z *ZitadelAdminClient) DeleteUser(ctx context.Context, subjectID string) error {
	if err := z.doJSON(ctx, http.MethodDelete, "/v2/users/"+subjectID, nil, nil); err != nil {
		return fmt.Errorf("auth: delete zitadel user: %w", err)
	}
	return nil
}

func (z *ZitadelAdminClient) doJSON(ctx context.Context, method, path string, reqBody, respBody any) error {
	var bodyReader io.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, z.BaseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+z.BearerToken)
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := z.httpClient().Do(req)
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

// generateTemporaryPassword returns a random password combining
// cryptographically random bytes with a fixed suffix that guarantees
// uppercase, lowercase, digit, and symbol characters are present, since
// exact complexity requirements depend on the tenant's configured Zitadel
// password policy.
func generateTemporaryPassword() string {
	randomBytes := make([]byte, 18)
	_, _ = rand.Read(randomBytes) // crypto/rand.Read only fails on unrecoverable OS errors
	return base64.RawURLEncoding.EncodeToString(randomBytes) + "-Aa1!"
}
