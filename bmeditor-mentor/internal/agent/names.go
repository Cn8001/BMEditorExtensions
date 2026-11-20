package agent

import (
	"regexp"
	"sort"
	"strings"

	"bmeditor-mentor/internal/index"
)

var codeName = regexp.MustCompile("`([A-Za-z_][A-Za-z0-9_.]*)(\\(\\))?`|\\b([A-Z][a-z0-9]+[A-Z][A-Za-z0-9]*(\\.[A-Za-z_][A-Za-z0-9]*)?)\\b|\\b([A-Za-z_][A-Za-z0-9]*\\.[A-Z][A-Za-z0-9]*)\\b")

// unknownNames are the code names in what the agent wrote, in backticks
// or CamelCase, that the code does not have: what it says it did is
// checked, not shown as fact.
func unknownNames(ix *index.Index, text string) []string {
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
