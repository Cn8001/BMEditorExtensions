package dotnet

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

var ErrRunning = errors.New("dotnet is running already")

// Line is one line dotnet wrote, on stdout or stderr.
type Line struct {
	Stream string `json:"stream"`
	Text   string `json:"text"`
}

// Runner runs one dotnet command at a time, so a build cannot write the
// files a running program holds.
type Runner struct {
	mu   sync.Mutex
	cmd  *exec.Cmd
	what string
}

// Start runs bin with args in dir, in the background: each line goes to
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
		return fmt.Errorf("start dotnet %s: %w", what, err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("start dotnet %s: %w", what, err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start dotnet %s: %w", what, err)
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

// Stop ends the command and what it started: dotnet run starts the program
// as a child, which outlives a killed dotnet on Windows.
func (r *Runner) Stop() {
	r.mu.Lock()
	cmd := r.cmd
	r.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		kill(cmd)
	}
}
