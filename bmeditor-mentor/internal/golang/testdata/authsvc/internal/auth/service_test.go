package auth

import (
	"context"
	"testing"
)

type memRepo struct{ ids []*UserIdentity }

func (m *memRepo) FindIdentity(_ context.Context, provider, id string) (*UserIdentity, error) {
	for _, i := range m.ids {
		if i.Provider == provider && i.ProviderUserID == id {
			return i, nil
		}
	}
	return nil, nil
}

func (m *memRepo) SaveIdentity(_ context.Context, id *UserIdentity) error {
	m.ids = append(m.ids, id)
	return nil
}

func TestLogin_LinksIdentity(t *testing.T) {
	repo := &memRepo{}
	s := NewService(repo, SteamProvider{})
	if _, err := s.Login(context.Background(), "steam", "42"); err != nil {
		t.Fatal(err)
	}
	if len(repo.ids) != 1 {
		t.Fatalf("ids = %d", len(repo.ids))
	}
}
