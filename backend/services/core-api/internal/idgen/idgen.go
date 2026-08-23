// Package idgen generates opaque unique IDs for domain entities.
package idgen

import "github.com/google/uuid"

// New returns a new random unique ID.
func New() string {
	return uuid.NewString()
}
