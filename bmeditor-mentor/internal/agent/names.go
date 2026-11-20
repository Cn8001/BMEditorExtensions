package agent

import (
	"context"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"bmeditor-mentor/internal/index"
)

var codeName = regexp.MustCompile("`([A-Za-z_][A-Za-z0-9_.]*)(\\(\\))?`|\\b([A-Z][a-z0-9]+[A-Z][A-Za-z0-9]*(\\.[A-Za-z_][A-Za-z0-9]*)?)\\b|\\b([A-Za-z_][A-Za-z0-9]*\\.[A-Z][A-Za-z0-9]*)\\b")

// unknownNames are the code names in what the agent wrote, in backticks
// or CamelCase, that the code does not have: what it says it did is
// checked, not shown as fact.
func unknownNames(ix *index.Index, text string, known func(string) bool) []string {
	have := map[string]bool{}
	for _, s := range ix.Symbols {
		have[s.Name] = true
		have[s.PkgName] = true
		if s.Recv != "" {
			have[s.Recv+"."+s.Name] = true
		}
		have[s.PkgName+"."+s.Name] = true
	}
	seen := map[string]bool{}
	out := []string{}
	for _, m := range codeName.FindAllStringSubmatch(text, -1) {
		name := m[1] + m[3] + m[5]
		if name == "" || seen[name] || have[name] {
			continue
		}
		seen[name] = true
		// A file, a path or the standard library is not what this checks.
		if strings.Contains(name, "/") || strings.HasSuffix(name, ".go") || stdlib(name) {
			continue
		}
		if i := strings.LastIndex(name, "."); i > 0 && (have[name[i+1:]] || have[name[:i]] && !isUpper(name[i+1:])) {
			continue
		}
		if known != nil && known(name) {
			continue
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func isUpper(s string) bool { return s != "" && s[0] >= 'A' && s[0] <= 'Z' }

func stdlib(name string) bool {
	pkg, _, ok := strings.Cut(name, ".")
	if !ok {
		return false
	}
	switch pkg {
	case "fmt", "errors", "context", "http", "json", "time", "strings", "strconv", "os", "io", "sql", "sync", "log", "slog", "bytes", "filepath", "sort", "url", "rand", "hex", "regexp", "testing", "t":
		return true
	}
	return false
}

// knownIn says whether a name is in the code beyond the Go index: a file of
// the project is named so (AgentCard for AgentCard.vue), or it is a word of
// one of texts, the files a step changed or named. Code in other languages
// is checked so, file by file.
func knownIn(files map[string]bool, texts []string) func(string) bool {
	return func(name string) bool {
		last := name
		if i := strings.LastIndex(name, "."); i >= 0 {
			last = name[i+1:]
		}
		if files[name] || files[last] {
			return true
		}
		for _, n := range []string{name, last} {
			if n == "" {
				continue
			}
			word := regexp.MustCompile(`(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(n) + `($|[^A-Za-z0-9_])`)
			for _, t := range texts {
				if word.MatchString(t) {
					return true
				}
			}
		}
		return false
	}
}

// fileNames are the names of the project's files, without their extension,
// as git lists them: what a name may stand for in code the index does not
// read.
func fileNames(ctx context.Context, root string) map[string]bool {
	out := map[string]bool{}
	cmd := exec.CommandContext(ctx, "git", "ls-files", "--cached", "--others", "--exclude-standard")
	cmd.Dir = root
	b, err := cmd.Output()
	if err != nil {
		return out
	}
	for _, f := range strings.Split(string(b), "\n") {
		if f = path.Base(strings.TrimSpace(f)); f != "" && f != "." {
			out[strings.TrimSuffix(f, path.Ext(f))] = true
		}
	}
	return out
}

// readTexts reads files, from root, up to a size each: what a step changed
// or named, for knownIn.
func readTexts(root string, rels []string) []string {
	var out []string
	for _, r := range rels {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(r)))
		if err != nil || len(b) > 1<<20 {
			continue
		}
		out = append(out, string(b))
	}
	return out
}
