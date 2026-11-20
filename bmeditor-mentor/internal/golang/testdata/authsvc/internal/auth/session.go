package auth

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Session is a signed-in user's active login, until it expires or is
// refreshed.
type Session struct {
	Token        string
	UserID       int64
	ExpiresAt    time.Time
	RefreshToken string
}

func newSession(userID int64) *Session {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return &Session{Token: hex.EncodeToString(b), UserID: userID, ExpiresAt: time.Now().Add(time.Hour)}
}

// Expired says whether the session can no longer be used.
func (s *Session) Expired(now time.Time) bool { return now.After(s.ExpiresAt) }
