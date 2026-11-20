// Package auth signs users in through outside identity providers.
package auth

import "time"

// UserIdentity links a user to their account at an outside identity
// provider, such as Steam or Google.
type UserIdentity struct {
	UserID         int64
	Provider       string
	ProviderUserID string
	LinkedAt       time.Time
}
