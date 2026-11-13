package game

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/yuin/gopher-lua/parse"
)

// Problem is an error in a script, 1-based, as the editor's Problems shows it.
type Problem struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// Check parses a script with the engine's own Lua (gopher-lua 5.1), so what
// the engine refuses to load shows before the game runs.
func Check(file, src string) []Problem {
	_, err := parse.Parse(strings.NewReader(src), file)
	if err == nil {
		return []Problem{}
	}
	p := Problem{File: file, Line: 1, Col: 1, Severity: "error", Message: strings.TrimSpace(err.Error())}
	var pe *parse.Error
	if errors.As(err, &pe) {
		p.Message = pe.Message
		if pe.Token != "" {
			p.Message += " near '" + pe.Token + "'"
		}
		if pe.Pos.Line > 0 {
			p.Line, p.Col = pe.Pos.Line, max(pe.Pos.Column, 1)
		} else {
			p.Line = strings.Count(src, "\n") + 1 // at the end of the file
		}
	}
	return []Problem{p}
}

// The engine logs "<level> <source>: <message>", and a failed script's
// message starts with its path and line: "scripts/door.lua:2: ...".
var logLine = regexp.MustCompile(`^(error|warn|warning)\s+\S+:\s+(.*?\.lua):(\d+):\s*(.*)$`)

// LogProblem reads a script error from a line the engine logged.
func LogProblem(line string) (Problem, bool) {
	m := logLine.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return Problem{}, false
	}
	n, _ := strconv.Atoi(m[3])
	severity := "error"
	if m[1] != "error" {
		severity = "warning"
	}
	return Problem{File: m[2], Line: n, Col: 1, Severity: severity, Message: m[4]}, true
}
