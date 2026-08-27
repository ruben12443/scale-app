package auth

import (
	"errors"
	"net/http"
	"strings"

	"scale-app/backend/services/core-api/internal/domain"
	"scale-app/backend/services/core-api/internal/storage"
)

// Middleware verifies the bearer token on incoming requests, looks up the
// corresponding local User by its Rauthy subject, and attaches it to the
// request context for downstream handlers (see UserFromContext).
func Middleware(verifier TokenVerifier, users storage.UserRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}

			subject, err := verifier.Verify(r.Context(), token)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			user, err := users.GetByRauthySubject(r.Context(), subject)
			if err != nil {
				if errors.Is(err, storage.ErrNotFound) {
					http.Error(w, "no local account for this identity", http.StatusForbidden)
					return
				}
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}

			next.ServeHTTP(w, r.WithContext(ContextWithUser(r.Context(), user)))
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(h, prefix))
	if token == "" {
		return "", false
	}
	return token, true
}

// RequireRole wraps next so it only runs if the request's authenticated user
// (attached by Middleware) has the given role.
func RequireRole(role domain.Role, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		if user.Role != role {
			http.Error(w, "insufficient role", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
