package auth

import (
	"context"
	"errors"
)

// Provider is an outside identity provider a user signs in with.
type Provider interface {
	Name() string
	Exchange(ctx context.Context, code string) (UserIdentity, error)
}

// SteamProvider signs users in with Steam.
type SteamProvider struct{ Key string }

func (SteamProvider) Name() string { return "steam" }

func (p SteamProvider) Exchange(ctx context.Context, code string) (UserIdentity, error) {
	if code == "" {
		return UserIdentity{}, errors.New("steam: no code")
	}
	return UserIdentity{Provider: "steam", ProviderUserID: code}, nil
}

// GoogleProvider signs users in with Google.
type GoogleProvider struct{ ClientID string }

func (GoogleProvider) Name() string { return "google" }

func (p GoogleProvider) Exchange(ctx context.Context, code string) (UserIdentity, error) {
	return UserIdentity{Provider: "google", ProviderUserID: code}, nil
}
