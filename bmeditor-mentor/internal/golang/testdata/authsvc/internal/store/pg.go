// Package store keeps the service's data in Postgres.
package store

import (
	"context"
	"database/sql"

	"example.com/authsvc/internal/auth"
)

// PGStore keeps identities in Postgres.
type PGStore struct{ db *sql.DB }

// Open connects to the database at url.
func Open(url string) (*PGStore, error) {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, err
	}
	return &PGStore{db: db}, nil
}

func (s *PGStore) FindIdentity(ctx context.Context, provider, providerUserID string) (*auth.UserIdentity, error) {
	row := s.db.QueryRowContext(ctx, `SELECT user_id FROM identities WHERE provider = $1 AND provider_user_id = $2`, provider, providerUserID)
	id := &auth.UserIdentity{Provider: provider, ProviderUserID: providerUserID}
	if err := row.Scan(&id.UserID); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return id, nil
}

func (s *PGStore) SaveIdentity(ctx context.Context, id *auth.UserIdentity) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO identities VALUES ($1, $2, $3)`, id.UserID, id.Provider, id.ProviderUserID)
	return err
}
