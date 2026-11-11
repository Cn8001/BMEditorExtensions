package fim

import (
	"strings"
	"testing"
)

func TestWindow_CutsAtLines(t *testing.T) {
	// Arrange
	prefix := strings.Repeat("line of code\n", 2000) + "func main() {"
	suffix := strings.Repeat("after\n", 2000)
	r := Request{Prefix: prefix, Suffix: suffix, Extra: []Snippet{{Path: "a.go", Text: strings.Repeat("x\n", 3000)}, {Path: "b.go", Text: strings.Repeat("y\n", 3000)}}}

	// Act
	w := Window(r)

	// Assert
	if len(w.Prefix) > MaxPrefix || !strings.HasPrefix(w.Prefix, "line of code\n") || !strings.HasSuffix(w.Prefix, "func main() {") {
		t.Errorf("prefix is %d bytes, starting %q", len(w.Prefix), w.Prefix[:20])
	}
	if len(w.Suffix) > MaxSuffix || !strings.HasSuffix(w.Suffix, "after\n") {
		t.Errorf("suffix is %d bytes", len(w.Suffix))
	}
	total := 0
	for _, e := range w.Extra {
		total += len(e.Text)
	}
	if total > MaxExtra || len(w.Extra) != 2 {
		t.Errorf("extra: %d snippets, %d bytes", len(w.Extra), total)
	}
}

func TestClean(t *testing.T) {
	for _, c := range []struct {
		name, out, prefix, suffix, want string
	}{
		{"plain", "fmt.Println(x)", "\tx := 1\n\t", "\n}\n", "fmt.Println(x)"},
		{"fences", "```go\nreturn nil\n```", "func f() error {\n\t", "\n}", "return nil"},
		{"mid-line takes the line only", "a, b)\nmore()", "f(", ")\n", "a, b"},
		{"stops at the next line of the file", "x++\n\ty++\n}\nfunc next() {", "for {\n\t", "\n}\nfunc next() {\n", "x++\n\ty++"},
		{"stops at two blank lines", "a()\n\n\nb()", "", "", "a()"},
		{"nothing", "  \n ", "x", "", ""},
	} {
		got := Clean(c.out, Request{Prefix: c.prefix, Suffix: c.suffix})
		if got != c.want {
			t.Errorf("%s: Clean = %q, want %q", c.name, got, c.want)
		}
	}
	long := strings.Repeat("x()\n", 40)
	if n := strings.Count(Clean(long, Request{}), "\n") + 1; n != MaxLines {
		t.Errorf("a long answer keeps %d lines, want %d", n, MaxLines)
	}
}
