package auth

import "context"

// Repository keeps identities.
type Repository interface {
	FindIdentity(ctx context.Context, provider, providerUserID string) (*UserIdentity, error)
	SaveIdentity(ctx context.Context, id *UserIdentity) error
}
