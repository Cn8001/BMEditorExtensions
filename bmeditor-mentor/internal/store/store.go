// Package store keeps what the extension remembers about one repository
// in a SQLite file on this machine: the search index over its symbols,
// embeddings when they are on, which changes the developer has reviewed,
// checkpoints and recorded decisions. Nothing in it leaves the machine.
package store

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// DB is one repository's store.
type DB struct{ sql *sql.DB }

var migrations = []string{
	`CREATE VIRTUAL TABLE fts USING fts5(key UNINDEXED, name, fields, doc, sig, pkg, context, tokenize = 'porter unicode61');
	 CREATE TABLE meta (k TEXT PRIMARY KEY, v TEXT NOT NULL);
	 CREATE TABLE reviews (key TEXT PRIMARY KEY, hash TEXT NOT NULL, note TEXT NOT NULL, at INTEGER NOT NULL);
	 CREATE TABLE embeddings (key TEXT PRIMARY KEY, model TEXT NOT NULL, hash TEXT NOT NULL, vec BLOB NOT NULL);
	 CREATE TABLE checkpoints (id INTEGER PRIMARY KEY, created INTEGER NOT NULL, status TEXT NOT NULL, data TEXT NOT NULL);
	 CREATE TABLE decisions (id INTEGER PRIMARY KEY, created INTEGER NOT NULL, claim TEXT NOT NULL, rationale TEXT NOT NULL, source TEXT NOT NULL, status TEXT NOT NULL, keys TEXT NOT NULL);`,
}

// Open opens the store at path, making it and its folder when missing.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // one writer; reads are quick
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		db.Close()
		return nil, err
	}
	for i := version; i < len(migrations); i++ {
		if _, err := db.Exec(migrations[i]); err != nil {
			db.Close()
			return nil, err
		}
		if _, err := db.Exec(`PRAGMA user_version = ` + strconv.Itoa(i+1)); err != nil {
			db.Close()
			return nil, err
		}
	}
	return &DB{sql: db}, nil
}

// Close closes the file.
func (d *DB) Close() error { return d.sql.Close() }

// Doc is one symbol as search sees it: each column already split into
// words, so the tokenizer finds "identity" in UserIdentity.
type Doc struct {
	Key, Name, Fields, Doc, Sig, Pkg, Context string
}

// ReplaceDocs puts docs in place of every document searched.
func (d *DB) ReplaceDocs(ctx context.Context, docs []Doc) error {
	tx, err := d.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM fts`); err != nil {
		return err
	}
	st, err := tx.PrepareContext(ctx, `INSERT INTO fts (key, name, fields, doc, sig, pkg, context) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer st.Close()
	for _, doc := range docs {
		if _, err := st.ExecContext(ctx, doc.Key, doc.Name, doc.Fields, doc.Doc, doc.Sig, doc.Pkg, doc.Context); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Hit is a document that matched, with its BM25 score, higher better.
type Hit struct {
	Key   string
	Score float64
}

// Match finds the documents that have any of words, best first. Names
// weigh most, then fields, the doc comment, the signature, the package,
// and last the names of what uses it.
func (d *DB) Match(ctx context.Context, words []string, limit int) ([]Hit, error) {
	var terms []string
	for _, w := range words {
		w = strings.Map(func(r rune) rune {
			if r == '"' {
				return -1
			}
			return r
		}, w)
		if w != "" {
			terms = append(terms, `"`+w+`"`)
		}
	}
	if len(terms) == 0 {
		return nil, nil
	}
	rows, err := d.sql.QueryContext(ctx, `SELECT key, -bm25(fts, 10.0, 4.0, 3.0, 2.0, 2.0, 1.0) FROM fts WHERE fts MATCH ? ORDER BY 2 DESC LIMIT ?`,
		strings.Join(terms, " OR "), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hits []Hit
	for rows.Next() {
		var h Hit
		if err := rows.Scan(&h.Key, &h.Score); err != nil {
			return nil, err
		}
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

// Meta reads a value kept by name; empty when there is none.
func (d *DB) Meta(ctx context.Context, k string) string {
	var v string
	_ = d.sql.QueryRowContext(ctx, `SELECT v FROM meta WHERE k = ?`, k).Scan(&v)
	return v
}

// SetMeta keeps a value by name.
func (d *DB) SetMeta(ctx context.Context, k, v string) error {
	_, err := d.sql.ExecContext(ctx, `INSERT INTO meta (k, v) VALUES (?, ?) ON CONFLICT (k) DO UPDATE SET v = excluded.v`, k, v)
	return err
}

// Review is the developer's mark that they looked at a symbol as it was:
// Hash is its hash then, so a later change shows.
type Review struct {
	Key  string    `json:"key"`
	Hash string    `json:"hash"`
	Note string    `json:"note,omitempty"`
	At   time.Time `json:"at"`
}

// Reviews reads every review mark.
func (d *DB) Reviews(ctx context.Context) (map[string]Review, error) {
	rows, err := d.sql.QueryContext(ctx, `SELECT key, hash, note, at FROM reviews`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Review{}
	for rows.Next() {
		var r Review
		var at int64
		if err := rows.Scan(&r.Key, &r.Hash, &r.Note, &at); err != nil {
			return nil, err
		}
		r.At = time.UnixMilli(at)
		out[r.Key] = r
	}
	return out, rows.Err()
}

// SaveReviews writes review marks.
func (d *DB) SaveReviews(ctx context.Context, rs []Review) error {
	tx, err := d.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, r := range rs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO reviews (key, hash, note, at) VALUES (?, ?, ?, ?)
			ON CONFLICT (key) DO UPDATE SET hash = excluded.hash, note = excluded.note, at = excluded.at`,
			r.Key, r.Hash, r.Note, r.At.UnixMilli()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ForgetReviews removes the marks of keys.
func (d *DB) ForgetReviews(ctx context.Context, keys []string) error {
	for _, k := range keys {
		if _, err := d.sql.ExecContext(ctx, `DELETE FROM reviews WHERE key = ?`, k); err != nil {
			return err
		}
	}
	return nil
}

// Vector is an embedding with the hash of what it was made from.
type Vector struct {
	Hash string
	Vec  []float32
}

// Embeddings reads the vectors made with model.
func (d *DB) Embeddings(ctx context.Context, model string) (map[string]Vector, error) {
	rows, err := d.sql.QueryContext(ctx, `SELECT key, hash, vec FROM embeddings WHERE model = ?`, model)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Vector{}
	for rows.Next() {
		var key, hash string
		var blob []byte
		if err := rows.Scan(&key, &hash, &blob); err != nil {
			return nil, err
		}
		out[key] = Vector{Hash: hash, Vec: decodeVec(blob)}
	}
	return out, rows.Err()
}

// SaveEmbeddings writes vectors made with model, keyed by symbol.
func (d *DB) SaveEmbeddings(ctx context.Context, model string, vecs map[string]Vector) error {
	tx, err := d.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for k, v := range vecs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO embeddings (key, model, hash, vec) VALUES (?, ?, ?, ?)
			ON CONFLICT (key) DO UPDATE SET model = excluded.model, hash = excluded.hash, vec = excluded.vec`,
			k, model, v.Hash, encodeVec(v.Vec)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func encodeVec(v []float32) []byte {
	b := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[4*i:], math.Float32bits(f))
	}
	return b
}

func decodeVec(b []byte) []float32 {
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return v
}

// ErrNotFound is a record that is not there.
var ErrNotFound = errors.New("not found")

// SaveCheckpoint writes a checkpoint's data, as JSON, under id; id 0
// makes a new one and answers its id.
func (d *DB) SaveCheckpoint(ctx context.Context, id int64, status string, data any) (int64, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return 0, err
	}
	if id == 0 {
		res, err := d.sql.ExecContext(ctx, `INSERT INTO checkpoints (created, status, data) VALUES (?, ?, ?)`, time.Now().UnixMilli(), status, string(b))
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	}
	_, err = d.sql.ExecContext(ctx, `UPDATE checkpoints SET status = ?, data = ? WHERE id = ?`, status, string(b), id)
	return id, err
}

// Checkpoints reads the last n checkpoints' data, newest first.
func (d *DB) Checkpoints(ctx context.Context, n int) ([]json.RawMessage, error) {
	rows, err := d.sql.QueryContext(ctx, `SELECT data FROM checkpoints ORDER BY id DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []json.RawMessage
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, json.RawMessage(s))
	}
	return out, rows.Err()
}

// Decision is a claim about why the code is as it is, with where it came
// from: recorded when someone said so (the agent's plan, a commit, the
// developer), inferred when the extension worked it out, unknown when
// nobody knows.
type Decision struct {
	ID        int64     `json:"id"`
	Created   time.Time `json:"created"`
	Claim     string    `json:"claim"`
	Rationale string    `json:"rationale"`
	Source    string    `json:"source"`
	Status    string    `json:"status"`
	Keys      []string  `json:"keys"`
}

// AddDecision records a decision.
func (d *DB) AddDecision(ctx context.Context, dec Decision) (int64, error) {
	keys, _ := json.Marshal(dec.Keys)
	res, err := d.sql.ExecContext(ctx, `INSERT INTO decisions (created, claim, rationale, source, status, keys) VALUES (?, ?, ?, ?, ?, ?)`,
		time.Now().UnixMilli(), dec.Claim, dec.Rationale, dec.Source, dec.Status, string(keys))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Decisions reads the decisions that name key, or every one when key is
// empty, newest first.
func (d *DB) Decisions(ctx context.Context, key string) ([]Decision, error) {
	q := `SELECT id, created, claim, rationale, source, status, keys FROM decisions`
	var args []any
	if key != "" {
		q += ` WHERE keys LIKE ?`
		args = append(args, `%"`+key+`"%`)
	}
	rows, err := d.sql.QueryContext(ctx, q+` ORDER BY id DESC LIMIT 200`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Decision
	for rows.Next() {
		var dec Decision
		var created int64
		var keys string
		if err := rows.Scan(&dec.ID, &created, &dec.Claim, &dec.Rationale, &dec.Source, &dec.Status, &keys); err != nil {
			return nil, err
		}
		dec.Created = time.UnixMilli(created)
		_ = json.Unmarshal([]byte(keys), &dec.Keys)
		out = append(out, dec)
	}
	return out, rows.Err()
}

// VectorStore keeps embeddings. DB is the one used: the vectors sit in
// the same file as the rest, beside the hash of what each was made from.
// Another, such as a vector database a team shares, can stand in for it.
type VectorStore interface {
	Embeddings(ctx context.Context, model string) (map[string]Vector, error)
	SaveEmbeddings(ctx context.Context, model string, vecs map[string]Vector) error
}

var _ VectorStore = (*DB)(nil)
