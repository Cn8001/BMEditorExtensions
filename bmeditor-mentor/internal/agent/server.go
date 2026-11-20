package agent

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"bmeditor-mentor/internal/gateproto"
)

// Server is where mentor-gate reaches the backend: a port on this
// machine only, behind a token only this user can read.
type Server struct {
	Dir   string // the registry folder
	token string
	port  int
	srv   *http.Server
	gate  func(root string) *Gate
}

// Listen starts the server. gate finds the gate of a folder, nil when the
// backend has none open for it.
func Listen(dir string, gate func(root string) *Gate) (*Server, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	s := &Server{Dir: dir, token: hex.EncodeToString(b), port: ln.Addr().(*net.TCPAddr).Port, gate: gate}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /hook", s.auth(s.hook))
	mux.HandleFunc("POST /tool", s.auth(s.tool))
	mux.HandleFunc("GET /ping", s.auth(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }))
	s.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = s.srv.Serve(ln) }()
	return s, nil
}

// Register tells the gate where to find the backend of root.
func (s *Server) Register(root string) error {
	return gateproto.Write(s.Dir, gateproto.Registry{Port: s.port, Token: s.token, Root: filepath.Clean(root), PID: os.Getpid()})
}

// Claim registers root for this backend unless another one that runs has
// it: with several editors open on a folder, the agent stays with one of
// them, and when that one goes, the next to ask takes the folder over.
func (s *Server) Claim(root string) error {
	if b, err := os.ReadFile(gateproto.RegistryPath(s.Dir, root)); err == nil {
		var r gateproto.Registry
		if json.Unmarshal(b, &r) == nil {
			if r.Port == s.port && r.PID == os.Getpid() {
				return nil // ours already
			}
			if r.PID > 0 && r.PID != os.Getpid() && gateproto.Alive(r.PID) {
				return nil // another editor's, which runs
			}
		}
	}
	return s.Register(root)
}

// Unregister removes root's registry, when it is this backend's.
func (s *Server) Unregister(root string) {
	path := gateproto.RegistryPath(s.Dir, root)
	if b, err := os.ReadFile(path); err == nil {
		var r gateproto.Registry
		if json.Unmarshal(b, &r) == nil && r.Port == s.port {
			_ = os.Remove(path)
		}
	}
}

// Close stops the server.
func (s *Server) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.srv.Shutdown(ctx)
}

func (s *Server) auth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("Authorization")
		// A page in a browser cannot send this header to another origin
		// without a preflight this server does not answer; refuse any
		// that says where it came from.
		if r.Header.Get("Origin") != "" || subtle.ConstantTimeCompare([]byte(got), []byte("Bearer "+s.token)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		h(w, r)
	}
}

func (s *Server) hook(w http.ResponseWriter, r *http.Request) {
	root := r.Header.Get("X-Mentor-Root")
	g := s.gate(root)
	var call HookCall
	body, _ := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if g == nil || json.Unmarshal(body, &call) != nil {
		writeJSON(w, gateproto.Decision{Allow: true})
		return
	}
	d := g.Hook(call)
	writeJSON(w, gateproto.Decision{Allow: d.Allow, Reason: d.Reason})
}

func (s *Server) tool(w http.ResponseWriter, r *http.Request) {
	var call gateproto.ToolCall
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&call); err != nil {
		writeJSON(w, gateproto.ToolResult{Text: "Bad request: " + err.Error(), IsError: true})
		return
	}
	g := s.gate(call.Root)
	if g == nil {
		writeJSON(w, gateproto.ToolResult{Text: "Mentor has no folder open here; carry on as the developer told you."})
		return
	}
	text, err := g.Tool(r.Context(), call.Tool, call.Args)
	if err != nil {
		writeJSON(w, gateproto.ToolResult{Text: err.Error(), IsError: true})
		return
	}
	writeJSON(w, gateproto.ToolResult{Text: text})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// Tool runs one of the MCP tools for the agent.
func (g *Gate) Tool(ctx context.Context, name string, args json.RawMessage) (string, error) {
	var a struct {
		Task    string     `json:"task"`
		Steps   []PlanStep `json:"steps"`
		Summary string     `json:"summary"`
		Why     string     `json:"why"`
		Made    string     `json:"made"`
		Know    string     `json:"know"`
		Fits    string     `json:"fits"`
		Answer  string     `json:"answer"`
	}
	if len(args) > 0 && string(args) != "null" {
		if err := json.Unmarshal(args, &a); err != nil {
			return "", errors.New("the arguments do not read: " + err.Error())
		}
	}
	g.mu.Lock()
	g.mcp = true // a tool that is called shows the server is there
	off := g.policy.Off
	g.mu.Unlock()
	if off {
		return "Mentor is off for the agent: work as the developer told you, without plans or checkpoints.", nil
	}
	switch name {
	case "mentor_plan":
		return g.Propose(a.Task, a.Steps), nil
	case "mentor_checkpoint":
		return g.OpenCheckpoint(ctx, a.Made, a.Know, a.Fits, a.Summary, a.Why), nil
	case "mentor_answer":
		return g.Answer(a.Answer), nil
	case "mentor_wait":
		// The developer may read a milestone for a while; it returns at once
		// when they answer or ask.
		return g.Wait(ctx, 10*time.Minute), nil
	case "mentor_status":
		return g.Status(), nil
	}
	return "", errors.New("no tool " + name)
}
