// Package gitx asks git what the extension needs: which commit and branch
// the folder is on, how often files change, and how they differ from
// HEAD. It only reads; nothing here commits, stashes or checks out.
package gitx

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func git(ctx context.Context, dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	// Reading must not take git's index lock from under the developer.
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C")
	hide(cmd)
	return cmd.Output()
}

// Head is where the folder is.
type Head struct {
	Commit string `json:"commit"`
	Branch string `json:"branch"` // empty when HEAD is detached
}

// HeadOf is the folder's HEAD; zero when it is not in a repository.
func HeadOf(ctx context.Context, dir string) Head {
	out, err := git(ctx, dir, "rev-parse", "HEAD", "--abbrev-ref", "HEAD")
	if err != nil {
		return Head{}
	}
	lines := strings.Fields(string(out))
	h := Head{}
	if len(lines) > 0 {
		h.Commit = lines[0]
	}
	if len(lines) > 1 && lines[1] != "HEAD" {
		h.Branch = lines[1]
	}
	return h
}

// Top is the repository's top folder, or dir when it is not in one.
func Top(ctx context.Context, dir string) string {
	out, err := git(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return dir
	}
	return filepath.Clean(strings.TrimSpace(string(out)))
}

// Churn is how many lines of each file changed in the commits since
// since, by absolute path: where the code moves most.
func Churn(ctx context.Context, dir string, since time.Duration) map[string]int {
	out, err := git(ctx, dir, "log", "--since="+strconv.Itoa(int(since.Hours()/24))+".days", "--numstat", "--format=", "--no-renames", "--", ".")
	if err != nil {
		return map[string]int{}
	}
	top := Top(ctx, dir)
	return numstat(out, top)
}

func numstat(out []byte, top string) map[string]int {
	churn := map[string]int{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		parts := strings.SplitN(sc.Text(), "\t", 3)
		if len(parts) != 3 {
			continue
		}
		add, _ := strconv.Atoi(parts[0]) // "-" for binary files reads as 0
		del, _ := strconv.Atoi(parts[1])
		churn[filepath.Join(top, filepath.FromSlash(parts[2]))] += add + del
	}
	return churn
}

// FileChange is how one file differs from HEAD.
type FileChange struct {
	Path    string `json:"path"` // absolute
	Status  string `json:"status"`
	Added   int    `json:"added"`
	Deleted int    `json:"deleted"`
}

// Changes are the files that differ from HEAD, untracked ones included.
func Changes(ctx context.Context, dir string) ([]FileChange, error) {
	top := Top(ctx, dir)
	out, err := git(ctx, dir, "diff", "HEAD", "--numstat", "--no-renames", "--", ".")
	if err != nil {
		return nil, err
	}
	var list []FileChange
	for path, n := range numstatPairs(out, top) {
		st := "M"
		if _, err := os.Stat(path); err != nil {
			st = "D"
		}
		list = append(list, FileChange{Path: path, Status: st, Added: n[0], Deleted: n[1]})
	}
	if un, err := git(ctx, dir, "ls-files", "--others", "--exclude-standard", "--full-name", "--", "."); err == nil {
		for _, rel := range strings.Split(strings.TrimSpace(string(un)), "\n") {
			if rel == "" {
				continue
			}
			path := filepath.Join(top, filepath.FromSlash(rel))
			lines := 0
			if b, err := os.ReadFile(path); err == nil {
				lines = bytes.Count(b, []byte("\n"))
			}
			list = append(list, FileChange{Path: path, Status: "A", Added: lines})
		}
	}
	return list, nil
}

func numstatPairs(out []byte, top string) map[string][2]int {
	m := map[string][2]int{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		parts := strings.SplitN(sc.Text(), "\t", 3)
		if len(parts) != 3 {
			continue
		}
		add, _ := strconv.Atoi(parts[0])
		del, _ := strconv.Atoi(parts[1])
		m[filepath.Join(top, filepath.FromSlash(parts[2]))] = [2]int{add, del}
	}
	return m
}
