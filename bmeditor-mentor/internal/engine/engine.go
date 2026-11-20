// Package engine keeps one folder's index up to date in the background:
// it reads the code when the folder opens, watches its files, reads again
// what changed, and keeps the search documents and embeddings in step.
// Readers get the newest finished index; while one is being read, they
// are told the answers may be stale.
package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"bmeditor-mentor/internal/embed"
	"bmeditor-mentor/internal/gitx"
	"bmeditor-mentor/internal/golang"
	"bmeditor-mentor/internal/index"
	"bmeditor-mentor/internal/review"
	"bmeditor-mentor/internal/search"
	"bmeditor-mentor/internal/store"
)

// Settings are the extension's settings the engine reads.
type Settings struct {
	BuildTags   string
	Excludes    []string
	Embeddings  string // "" or "none", or "ollama"
	OllamaURL   string
	OllamaModel string
}

// SettingsOf reads them from the extension's settings, a project's
// winning.
func SettingsOf(get func(string) string) Settings {
	var ex []string
	for _, f := range strings.FieldsFunc(get("excludes"), func(r rune) bool { return r == ',' || r == '\n' || r == ';' }) {
		if f = strings.TrimSpace(f); f != "" {
			ex = append(ex, f)
		}
	}
	return Settings{BuildTags: get("buildTags"), Excludes: ex, Embeddings: get("embeddings"),
		OllamaURL: get("ollamaUrl"), OllamaModel: get("ollamaModel")}
}

func (s Settings) code() string {
	return s.BuildTags + "|" + strings.Join(s.Excludes, ",")
}

func (s Settings) semantic() bool { return s.Embeddings == "ollama" && s.OllamaModel != "" }

// Status is what the window shows about the index.
type Status struct {
	Root      string         `json:"root"`
	State     string         `json:"state"` // indexing, ready, stale, error
	Detail    string         `json:"detail,omitempty"`
	Packages  int            `json:"packages"`
	Symbols   int            `json:"symbols"`
	Files     int            `json:"files"`
	Languages map[string]int `json:"languages"` // packages by language
	Built     time.Time      `json:"built,omitzero"`
	Took      int64          `json:"tookMs"`
	Head      gitx.Head      `json:"head"`
	Errors    []string       `json:"errors"`
	Semantic  string         `json:"semantic"` // off, building, on, or why it failed
}

// Engine is one folder's index and store.
type Engine struct {
	Root   string
	DB     *store.DB
	Review *review.Book

	notify func(method string, params any)

	mu       sync.Mutex
	settings Settings
	sources  []index.Source
	status   Status
	vecs     map[string]store.Vector
	model    string

	ix    atomic.Pointer[index.Index]
	ready chan struct{}
	once  sync.Once
	kick  chan bool // true for a full read
	stop  context.CancelFunc
	done  chan struct{}
}

// DataDir is where the extension keeps its files on this machine: the
// folder the editor names, else the user's cache.
func DataDir() string {
	if d := os.Getenv("BMEDITOR_DATA"); d != "" {
		return filepath.Join(d, "extension-data", "mentor")
	}
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "bmeditor-mentor")
	}
	return filepath.Join(os.TempDir(), "bmeditor-mentor")
}

// Hash names a folder's files in the data folder.
func Hash(root string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(root)))
	return hex.EncodeToString(sum[:])[:16]
}

// Open opens root's store under dataDir and starts reading its code.
func Open(root, dataDir string, set Settings, notify func(string, any)) (*Engine, error) {
	db, err := store.Open(filepath.Join(dataDir, Hash(root), "mentor.db"))
	if err != nil {
		return nil, err
	}
	if notify == nil {
		notify = func(string, any) {}
	}
	book, err := review.Open(context.Background(), db)
	if err != nil {
		db.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	e := &Engine{Root: root, DB: db, Review: book, notify: notify, settings: set, ready: make(chan struct{}),
		kick: make(chan bool, 1), stop: cancel, done: make(chan struct{})}
	e.sources = e.makeSources(set)
	e.status = Status{Root: root, State: "indexing", Languages: map[string]int{}, Errors: []string{}, Semantic: "off"}
	go e.run(ctx)
	return e, nil
}

func (e *Engine) makeSources(set Settings) []index.Source {
	return []index.Source{&golang.Builder{Root: e.Root, Opt: golang.Options{BuildTags: set.BuildTags, Excludes: set.Excludes}}}
}

// Close stops the engine and closes its store.
func (e *Engine) Close() {
	e.stop()
	<-e.done
	e.DB.Close()
}

// Index is the newest index, waiting for the first one when there is
// none yet.
// An index without packages is an error that says why.
func (e *Engine) Index(ctx context.Context) (*index.Index, error) {
	ix := e.ix.Load()
	if ix == nil {
		select {
		case <-e.ready:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		ix = e.ix.Load()
	}
	if ix != nil && len(ix.Packages) > 0 {
		return ix, nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	msg := e.status.Detail
	if msg == "" {
		msg = "The code could not be read."
	}
	return nil, errors.New(strings.Join(append([]string{msg}, e.status.Errors...), " "))
}

// Peek is the newest index, nil when there is none yet.
func (e *Engine) Peek() *index.Index { return e.ix.Load() }

// Status is the index's state now.
func (e *Engine) Status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	st := e.status
	st.Errors = append([]string{}, st.Errors...)
	return st
}

// Reindex reads everything again.
func (e *Engine) Reindex() { e.poke(true) }

// Apply takes new settings: a change to what is read reads everything
// again, a change to embeddings makes them again.
func (e *Engine) Apply(set Settings) {
	e.mu.Lock()
	old := e.settings
	e.settings = set
	if old.code() != set.code() {
		e.sources = e.makeSources(set)
	}
	e.mu.Unlock()
	if old.code() != set.code() {
		e.poke(true)
	} else if old.Embeddings != set.Embeddings || old.OllamaModel != set.OllamaModel || old.OllamaURL != set.OllamaURL {
		if ix := e.Peek(); ix != nil {
			go e.embedAll(context.Background(), ix)
		}
	}
}

func (e *Engine) poke(full bool) {
	select {
	case e.kick <- full:
	default:
		if full { // a full read queued replaces a partial one
			select {
			case <-e.kick:
			default:
			}
			select {
			case e.kick <- true:
			default:
			}
		}
	}
}

func (e *Engine) run(ctx context.Context) {
	defer close(e.done)
	e.build(ctx, true, nil)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case full := <-e.kick:
			e.build(ctx, full, nil)
		case <-tick.C:
			e.mu.Lock()
			srcs := e.sources
			e.mu.Unlock()
			var dirs []string
			full := false
			for _, s := range srcs {
				d, f := s.Changed()
				dirs = append(dirs, d...)
				full = full || f
			}
			if full || len(dirs) > 0 {
				e.setState("stale", "Files changed; reading them again.")
				// Let a burst of saves, a checkout or an agent's edits settle.
				select {
				case <-time.After(700 * time.Millisecond):
				case <-ctx.Done():
					return
				}
				e.build(ctx, full, dirs)
			}
		}
	}
}

func (e *Engine) setState(state, detail string) {
	e.mu.Lock()
	e.status.State, e.status.Detail = state, detail
	st := e.status
	e.mu.Unlock()
	e.notify("mentor/index", st)
}

// build reads all the code, or only dirs, and puts the new index in place.
func (e *Engine) build(ctx context.Context, full bool, dirs []string) {
	start := time.Now()
	if full || e.Peek() == nil {
		e.setState("indexing", "Reading the code.")
	}
	e.mu.Lock()
	srcs := e.sources
	e.mu.Unlock()
	var errs []string
	for _, s := range srcs {
		var err error
		if full || e.Peek() == nil {
			err = s.Full(ctx)
		} else {
			err = s.Update(ctx, dirs)
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			errs = append(errs, err.Error())
		}
	}
	ix := index.Merge(e.Root, srcs...)
	if err := e.DB.ReplaceDocs(ctx, search.Docs(ix)); err != nil && ctx.Err() == nil {
		errs = append(errs, "search: "+err.Error())
	}
	head := gitx.HeadOf(ctx, e.Root)
	e.mu.Lock()
	// The index and its status change together: a reader never sees one
	// without the other.
	e.ix.Store(ix)
	st := &e.status
	st.State, st.Detail = "ready", ""
	st.Packages, st.Symbols, st.Files = len(ix.Packages), len(ix.Symbols), len(ix.ByFile)
	st.Languages = map[string]int{}
	for _, p := range ix.Packages {
		st.Languages[p.Language]++
	}
	st.Built, st.Took, st.Head = ix.Built, time.Since(start).Milliseconds(), head
	// Never null: the view reads it as a list.
	st.Errors = append(append([]string{}, errs...), ix.Errors...)
	if len(st.Errors) > 30 {
		st.Errors = st.Errors[:30]
	}
	if len(ix.Packages) == 0 {
		st.State = "error"
		st.Detail = "No code here the extension can read."
		if len(ix.Modules) == 0 {
			st.Detail = "No go.mod in this folder or the folders under it."
		}
	}
	semantic := e.settings.semantic()
	stCopy := *st
	e.mu.Unlock()
	e.once.Do(func() { close(e.ready) })
	e.notify("mentor/index", stCopy)
	if semantic {
		go e.embedAll(ctx, ix)
	}
}

func (e *Engine) provider() embed.Provider {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.settings.semantic() {
		return nil
	}
	return &embed.Ollama{URL: e.settings.OllamaURL, Name: e.settings.OllamaModel}
}

func (e *Engine) setSemantic(s string) {
	e.mu.Lock()
	e.status.Semantic = s
	st := e.status
	e.mu.Unlock()
	e.notify("mentor/index", st)
}

// embedText is what a symbol's vector is made from: what it is called,
// what it says it is, and what it holds.
func embedText(s *index.Symbol) string {
	var b strings.Builder
	b.WriteString(string(s.Kind) + " " + s.PkgName + "." + s.Name)
	if s.Recv != "" {
		b.WriteString(" of " + s.Recv)
	}
	b.WriteString("\n" + s.Sig)
	if s.Doc != "" {
		b.WriteString("\n" + index.FirstSentence(s.Doc))
	}
	for i, f := range s.Fields {
		if i == 12 {
			break
		}
		b.WriteString("\n" + f.Name + " " + f.Type)
	}
	return b.String()
}

// embedAll makes the vectors the index lacks: of symbols that are new or
// changed since theirs were made.
var embedLock sync.Mutex

func (e *Engine) embedAll(ctx context.Context, ix *index.Index) {
	p := e.provider()
	if p == nil {
		e.mu.Lock()
		e.vecs, e.model = nil, ""
		e.mu.Unlock()
		e.setSemantic("off")
		return
	}
	embedLock.Lock()
	defer embedLock.Unlock()
	have, err := e.DB.Embeddings(ctx, p.Model())
	if err != nil {
		e.setSemantic("failed: " + err.Error())
		return
	}
	var todo []*index.Symbol
	for _, s := range ix.Symbols {
		if s.Kind == index.KindField || s.Test {
			continue
		}
		if v, ok := have[s.Key]; !ok || v.Hash != s.Hash {
			todo = append(todo, s)
		}
	}
	sort.Slice(todo, func(i, j int) bool { return todo[i].Key < todo[j].Key })
	if len(todo) > 0 {
		e.setSemantic("building")
	}
	for i := 0; i < len(todo); i += 64 {
		batch := todo[i:min(i+64, len(todo))]
		texts := make([]string, len(batch))
		for j, s := range batch {
			texts[j] = embedText(s)
		}
		vecs, err := p.Embed(ctx, texts)
		if err != nil {
			e.setSemantic("failed: " + err.Error())
			return
		}
		out := map[string]store.Vector{}
		for j, s := range batch {
			out[s.Key] = store.Vector{Hash: s.Hash, Vec: vecs[j]}
			have[s.Key] = out[s.Key]
		}
		if err := e.DB.SaveEmbeddings(ctx, p.Model(), out); err != nil {
			e.setSemantic("failed: " + err.Error())
			return
		}
	}
	e.mu.Lock()
	e.vecs, e.model = have, p.Model()
	e.mu.Unlock()
	e.setSemantic("on")
}

// Search answers a query, with embeddings when they are on and ready.
func (e *Engine) Search(ctx context.Context, q string, opt search.Options) (*search.Result, error) {
	ix, err := e.Index(ctx)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	vecs := e.vecs
	e.mu.Unlock()
	if p := e.provider(); p != nil && len(vecs) > 0 {
		qctx, cancel := context.WithTimeout(ctx, 8*time.Second)
		if v, err := p.Embed(qctx, []string{q}); err == nil && len(v) == 1 {
			opt.QueryVec, opt.Vectors = v[0], vecs
		}
		cancel()
	}
	return search.Query(ctx, ix, e.DB, q, opt)
}
