// Package engine finds the ASCDII Engine's ascdii program and runs it: a
// game in its window or in the engine's editor, and builds, with their
// output line by line.
package engine

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var (
	ErrNotFound = errors.New("the ascdii program was not found")
	ErrRunning  = errors.New("the game is running already")
)

func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// Locate is the ascdii program the setting names, as the program or the
// folder it is in, or else the one on the PATH.
func Locate(setting string) (string, error) {
	if setting = strings.TrimSpace(setting); setting != "" {
		if strings.HasPrefix(setting, "~") {
			if home, err := os.UserHomeDir(); err == nil {
				setting = filepath.Join(home, setting[1:])
			}
		}
		if fi, err := os.Stat(setting); err == nil && fi.IsDir() {
			setting = filepath.Join(setting, exe("ascdii"))
		}
		if fi, err := os.Stat(setting); err != nil || fi.IsDir() {
			return "", fmt.Errorf("%w at %s", ErrNotFound, setting)
		}
		return setting, nil
	}
	p, err := exec.LookPath("ascdii")
	if err != nil {
		return "", fmt.Errorf("%w on the PATH: set it in Settings", ErrNotFound)
	}
	return p, nil
}

// Version is what ascdii version prints, such as "ASCDII Engine 0.0.3".
func Version(ctx context.Context, bin string) (string, error) {
	out, err := exec.CommandContext(ctx, bin, "version").Output()
	if err != nil {
		return "", fmt.Errorf("ascdii version: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Line is one line a program wrote, on stdout or stderr.
type Line struct {
	Stream string `json:"stream"`
	Text   string `json:"text"`
}

// Runner runs one game at a time.
type Runner struct {
	mu   sync.Mutex
	cmd  *exec.Cmd
	what string
}

// Start runs ascdii with args in dir, in the background: each line goes to
// line, and done gets the end.
func (r *Runner) Start(bin, dir, what string, args []string, line func(Line), done func(error)) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cmd != nil {
		return fmt.Errorf("%w: %s", ErrRunning, r.what)
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	group(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("start %s: %w", what, err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("start %s: %w", what, err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", what, err)
	}
	r.cmd, r.what = cmd, what
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); lines(stdout, "stdout", line) }()
	go func() { defer wg.Done(); lines(stderr, "stderr", line) }()
	go func() {
		wg.Wait()
		err := cmd.Wait()
		r.mu.Lock()
		if r.cmd == cmd {
			r.cmd, r.what = nil, ""
		}
		r.mu.Unlock()
		done(err)
	}()
	return nil
}

func lines(rd io.Reader, stream string, line func(Line)) {
	sc := bufio.NewScanner(rd)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line(Line{Stream: stream, Text: strings.TrimRight(sc.Text(), "\r")})
	}
}

// Running is what runs, or "" when nothing does.
func (r *Runner) Running() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.what
}

// Stop ends the game and the programs it started, such as the window
// player ascdii play hands a windowed game to.
func (r *Runner) Stop() {
	r.mu.Lock()
	cmd := r.cmd
	r.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		kill(cmd)
	}
}
