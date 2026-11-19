package main

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"

	"bmeditor-csharp/bmext/host"
	"bmeditor-csharp/internal/dotnet"
)

// registerDotnet is the top bar's Run, Build and Test in a .NET project:
// dotnet's output goes to the window line by line, and its compiler
// messages to Problems.
func registerDotnet(s *host.Server, st *state) {
	for _, verb := range []string{"run", "build", "test"} {
		s.Handle("dotnet."+verb, handler(s, func(_ context.Context, root string, p struct{ Path string }) (any, error) {
			return nil, startDotnet(s, &st.dotnet, verb, root, p.Path)
		}))
	}
	s.Handle("dotnet.stop", func(context.Context, json.RawMessage) (any, error) {
		st.dotnet.Stop()
		return nil, nil
	})
}

func startDotnet(s *host.Server, r *dotnet.Runner, verb, root, target string) error {
	if target == "" {
		found := dotnet.Candidates(root, projectRelative(s, s.Context().Setting("solution")))
		if len(found) == 0 {
			return &host.Error{Code: "NO_SOLUTION", Message: "No solution or project in " + filepath.Base(root) + "."}
		}
		target = found[0]
	}
	bin, err := dotnet.Bin()
	if err != nil {
		return &host.Error{Code: "DOTNET_MISSING", Message: "dotnet was not found. Install the .NET SDK and restart BMEditor."}
	}
	args, err := dotnet.Args(verb, target)
	if errors.Is(err, dotnet.ErrNothingToRun) {
		return &host.Error{Code: "NOTHING_TO_RUN", Message: filepath.Base(target) + " has no project that builds a program (OutputType Exe)."}
	}
	if err != nil {
		return err
	}
	dir := filepath.Dir(target)
	line := func(l dotnet.Line) {
		_ = s.Notify("dotnet/output", l)
		if pr, ok := dotnet.ParseProblem(l.Text, dir); ok {
			_ = s.Notify("dotnet/problem", pr)
		}
	}
	done := func(err error) {
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		_ = s.Notify("dotnet/exit", map[string]string{"what": verb, "error": msg})
	}
	if err := r.Start(bin, dir, verb, args, line, done); err != nil {
		if errors.Is(err, dotnet.ErrRunning) {
			return &host.Error{Code: "RUNNING", Message: "dotnet " + r.Running() + " is running; stop it first."}
		}
		return err
	}
	_ = s.Notify("dotnet/started", map[string]string{"what": verb})
	return nil
}
