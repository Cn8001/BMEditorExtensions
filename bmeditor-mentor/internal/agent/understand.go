package agent

import (
	"context"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"bmeditor-mentor/internal/changes"
	"bmeditor-mentor/internal/index"
)

// What a step is, before and after: its budget, what it said it would
// leave alone, and its tests.

// isTestFile says whether a file is a test's: tests do not count against a
// step's budget.
func isTestFile(rel string) bool {
	base := path.Base(rel)
	return strings.HasSuffix(base, "_test.go") || strings.Contains(rel, "testdata/") ||
		strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") || strings.HasPrefix(rel, "test/") || strings.Contains(rel, "/test/")
}

// budgetOf is how many files outside tests a step is expected to change:
// the ones it named, or two a package it named, and at least two.
func budgetOf(st Step) int {
	n := 0
	for _, f := range st.Files {
		if !isTestFile(f) {
			n++
		}
	}
	n += 2 * len(st.Packages)
	return max(n, 2)
}

// overBudget says whether a step that has changed n files outside tests
// has grown past its budget: by more than half of it, and by two at least.
func overBudget(n, budget int) bool {
	return n > budget+max(2, budget/2)
}

var pathLike = regexp.MustCompile(`[/*]|\.[a-z]{1,5}$`)

// avoided is the entry of avoid that rel falls under, if one does: a file,
// a folder, or a glob. Names that are not paths are checked at the
// checkpoint, against what changed.
func avoided(rel string, avoid []string) string {
	for _, a := range avoid {
		p := strings.TrimSuffix(strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(a)), "./"), "/")
		if p == "" || !pathLike.MatchString(p) && !strings.Contains(a, "/") {
			continue
		}
		if rel == p || strings.HasPrefix(rel, p+"/") {
			return a
		}
		if ok, _ := path.Match(p, rel); ok {
			return a
		}
		if ok, _ := path.Match(p, path.Base(rel)); ok && !strings.Contains(p, "/") {
			return a
		}
	}
	return ""
}

// avoidedNames are the names in avoid that the step changed anyway: a
// declaration of that name, or of that type's methods.
func avoidedNames(avoid []string, rep *changes.Report) []string {
	if rep == nil {
		return []string{}
	}
	out := []string{}
	for _, a := range avoid {
		a = strings.TrimSpace(a)
		if a == "" || pathLike.MatchString(a) {
			continue
		}
	next:
		for _, u := range rep.Units {
			for _, c := range u.Changes {
				if c.Name == a || c.Recv == a || c.Recv+"."+c.Name == a || strings.HasSuffix(c.Key, "."+a) {
					out = append(out, a)
					break next
				}
			}
		}
	}
	return out
}

// namesCheck are the names in uses the code does not have, and those in
// creates it already has.
func namesCheck(ix *index.Index, uses, creates []string) (unknown, exists []string) {
	unknown, exists = []string{}, []string{}
	if ix == nil {
		return
	}
	have := map[string]bool{}
	for _, s := range ix.Symbols {
		have[s.Name] = true
		have[s.PkgName+"."+s.Name] = true
		if s.Recv != "" {
			have[s.Recv+"."+s.Name] = true
		}
	}
	for _, u := range uses {
		if u = strings.Trim(strings.TrimSpace(u), "`()"); u != "" && !pathLike.MatchString(u) && !have[u] {
			unknown = append(unknown, u)
		}
	}
	for _, c := range creates {
		if c = strings.Trim(strings.TrimSpace(c), "`()"); c != "" && !pathLike.MatchString(c) && have[c] {
			exists = append(exists, c)
		}
	}
	return
}

// TestRun is the tests of what a step changed, as Mentor ran them.
type TestRun struct {
	State    string   `json:"state"` // running, passed, failed, none
	Packages []string `json:"packages"`
	Failed   []string `json:"failed"`           // the tests that failed
	Output   string   `json:"output,omitempty"` // the end of what go test printed, when it failed
	Took     int64    `json:"tookMs"`
}

// RunGoTests runs go test for dirs, folders from root, and says how it went.
func RunGoTests(ctx context.Context, root string, dirs []string) TestRun {
	run := TestRun{State: "none", Packages: dirs, Failed: []string{}}
	if len(dirs) == 0 {
		return run
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	args := []string{"test", "-count=1"}
	for _, d := range dirs {
		args = append(args, "./"+d)
	}
	start := time.Now()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	run.Took = time.Since(start).Milliseconds()
	if err == nil {
		run.State = "passed"
		return run
	}
	run.State = "failed"
	for _, line := range strings.Split(string(out), "\n") {
		if f, ok := strings.CutPrefix(strings.TrimSpace(line), "--- FAIL: "); ok {
			run.Failed = append(run.Failed, strings.Fields(f)[0])
		}
	}
	text := string(out)
	if len(text) > 4000 {
		text = "…" + text[len(text)-4000:]
	}
	run.Output = text
	return run
}

// goDirs are the folders of the Go files a step changed, tests included,
// that still hold Go files: what go test runs for it.
func goDirs(root string, files map[string]bool) []string {
	seen := map[string]bool{}
	out := []string{}
	for f := range files {
		if !strings.HasSuffix(f, ".go") {
			continue
		}
		d := path.Dir(f)
		if seen[d] {
			continue
		}
		seen[d] = true
		if m, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(d), "*.go")); len(m) > 0 {
			out = append(out, d)
		}
	}
	sort.Strings(out)
	return out
}
