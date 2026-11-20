// Package review keeps which symbols the developer has looked at as they
// are now. A mark is made on a symbol's hash, so when the agent (or
// anyone) changes it afterwards, it shows as stale until looked at again.
package review

import (
	"context"
	"sort"
	"sync"
	"time"

	"bmeditor-mentor/internal/index"
	"bmeditor-mentor/internal/store"
)

// The states of a symbol.
const (
	Unseen   = "unseen"
	Reviewed = "reviewed"
	Stale    = "stale" // reviewed, then changed
)

// Book is one repository's review marks.
type Book struct {
	db    *store.DB
	mu    sync.Mutex
	marks map[string]store.Review
}

// Open reads the marks kept in db.
func Open(ctx context.Context, db *store.DB) (*Book, error) {
	m, err := db.Reviews(ctx)
	if err != nil {
		return nil, err
	}
	return &Book{db: db, marks: m}, nil
}

// StateOf is key's state against the code in ix.
func (b *Book) StateOf(ix *index.Index, key string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state(ix, key)
}

func (b *Book) state(ix *index.Index, key string) string {
	m, ok := b.marks[key]
	if !ok {
		return Unseen
	}
	if s := ix.Symbols[key]; s != nil && s.Hash != m.Hash {
		return Stale
	}
	return Reviewed
}

// States are the states of keys.
func (b *Book) States(ix *index.Index, keys []string) map[string]string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		out[k] = b.state(ix, k)
	}
	return out
}

// Mark notes that the developer reviewed keys as they are in ix.
func (b *Book) Mark(ctx context.Context, ix *index.Index, keys []string, note string) error {
	now := time.Now()
	var rs []store.Review
	for _, k := range keys {
		if s := ix.Symbols[k]; s != nil {
			rs = append(rs, store.Review{Key: k, Hash: s.Hash, Note: note, At: now})
		}
	}
	if err := b.db.SaveReviews(ctx, rs); err != nil {
		return err
	}
	b.mu.Lock()
	for _, r := range rs {
		b.marks[r.Key] = r
	}
	b.mu.Unlock()
	return nil
}

// Unmark forgets the marks of keys.
func (b *Book) Unmark(ctx context.Context, keys []string) error {
	if err := b.db.ForgetReviews(ctx, keys); err != nil {
		return err
	}
	b.mu.Lock()
	for _, k := range keys {
		delete(b.marks, k)
	}
	b.mu.Unlock()
	return nil
}

// StaleKeys are the reviewed symbols that changed since.
func (b *Book) StaleKeys(ix *index.Index) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	for k := range b.marks {
		if b.state(ix, k) == Stale {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
