package dotnet

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Problem is a compiler or MSBuild error or warning, as Problems shows it.
type Problem struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	Severity string `json:"severity"` // error or warning
	Message  string `json:"message"`
	Source   string `json:"source"`
}

// file(line,col[,endLine,endCol]): error CS1002: ; expected [project]
var diagnostic = regexp.MustCompile(`^\s*(.+?)\((\d+),(\d+)(?:,\d+,\d+)?\)\s*:\s*(error|warning)\s+([A-Za-z]+\d+)\s*:\s*(.*?)(?:\s+\[[^\]]*\])?\s*$`)

// ParseProblem reads one line of dotnet's output. A relative path is taken
// against dir, where dotnet ran.
func ParseProblem(line, dir string) (Problem, bool) {
	m := diagnostic.FindStringSubmatch(line)
	if m == nil {
		return Problem{}, false
	}
	ln, _ := strconv.Atoi(m[2])
	col, _ := strconv.Atoi(m[3])
	file := strings.TrimSpace(m[1])
	if !filepath.IsAbs(file) {
		file = filepath.Join(dir, file)
	}
	return Problem{File: file, Line: ln, Col: col, Severity: m[4], Message: m[5] + ": " + m[6], Source: "dotnet"}, true
}
