package auth

import (
	"context"
	"fmt"
)

// Service signs users in.
type Service struct {
	repo      Repository
	providers map[string]Provider
}

// NewService makes a Service that signs in with providers.
func NewService(repo Repository, providers ...Provider) *Service {
	s := &Service{repo: repo, providers: map[string]Provider{}}
	for _, p := range providers {
		s.providers[p.Name()] = p
	}
	return s
}

// Login signs a user in with provider's code and starts their session.
func (s *Service) Login(ctx context.Context, provider, code string) (*Session, error) {
	p, ok := s.providers[provider]
	if !ok {
		return nil, fmt.Errorf("no provider %q", provider)
	}
	ident, err := p.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}
	found, err := s.repo.FindIdentity(ctx, ident.Provider, ident.ProviderUserID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		if err := s.repo.SaveIdentity(ctx, &ident); err != nil {
			return nil, err
		}
		found = &ident
	}
	return newSession(found.UserID), nil
}
