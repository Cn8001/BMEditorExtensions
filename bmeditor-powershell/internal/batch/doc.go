// Package batch reads a cmd batch file (.bat, .cmd) for its language
// server: its labels and the goto and call that jump to them, the
// variables it sets and uses, and the commands it runs.
package batch

import (
	"regexp"
	"sort"
	"strings"
)

// Span is a place in a document: a line and the columns [Start, End) on
// it, all from 0, columns in UTF-16 code units as the protocol counts them.
type Span struct {
	Line, Start, End int
}

type Label struct {
	Name string // as written
	Span Span   // the name, without the colon
}

// Jump is a goto or call to a label.
type Jump struct {
	Name string
	Call bool
	Span Span
}

// Var is a variable the file sets (set, set /a, set /p, a for's %%x).
type Var struct {
	Name string
	Span Span
	Line string // the line that sets it
}

// Use is a variable read: %name% or !name!.
type Use struct {
	Name string
	Span Span
}

type Doc struct {
	Lines  []string
	Labels []Label
	Jumps  []Jump
	Vars   []Var
	Uses   []Use
}

var (
	labelLine = regexp.MustCompile(`^\s*:([^\s:+=,;"][^\s+=,;"]*)`)
	comment   = regexp.MustCompile(`(?i)^\s*(@?rem(\s|$)|::)`)
	jump      = regexp.MustCompile(`(?i)(?:^|[\s&|(@])(goto|call)\s+(:?)([^\s&|()<>"]+)`)
	setVar    = regexp.MustCompile(`(?i)(?:^|[\s&|(@])set\s+(?:/[ap]\s+)?"?([^=\s"/+\-*%&|^][^=\s"+\-*/%&|^]*)\s*[+\-*/%&|^]?=`)
	forVar    = regexp.MustCompile(`(?i)(?:^|[\s&|(@])for\s+(?:/[dlfr]\s+(?:"[^"]*"\s+|\S+\s+)?)*%%([A-Za-z])\s`)
	useVar    = regexp.MustCompile(`%([A-Za-z_][\w.$#@\-]*?)(?::[^%]*)?%|!([A-Za-z_][\w.$#@\-]*?)(?::[^!]*)?!`)
)

// Parse reads a document's text.
func Parse(text string) *Doc {
	d := &Doc{Lines: strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")}
	for i, line := range d.Lines {
		if comment.MatchString(line) {
			continue
		}
		if m := labelLine.FindStringSubmatchIndex(line); m != nil {
			d.Labels = append(d.Labels, Label{Name: line[m[2]:m[3]], Span: span(line, i, m[2], m[3])})
			continue
		}
		for _, m := range jump.FindAllStringSubmatchIndex(line, -1) {
			call := strings.EqualFold(line[m[2]:m[3]], "call")
			colon := m[5] > m[4]
			if call && !colon {
				continue // call of a program, not of a label
			}
			name := line[m[6]:m[7]]
			d.Jumps = append(d.Jumps, Jump{Name: name, Call: call, Span: span(line, i, m[6], m[7])})
		}
		for _, m := range setVar.FindAllStringSubmatchIndex(line, -1) {
			d.Vars = append(d.Vars, Var{Name: line[m[2]:m[3]], Span: span(line, i, m[2], m[3]), Line: strings.TrimSpace(line)})
		}
		for _, m := range forVar.FindAllStringSubmatchIndex(line, -1) {
			d.Vars = append(d.Vars, Var{Name: "%" + line[m[2]:m[3]], Span: span(line, i, m[2]-2, m[3]), Line: strings.TrimSpace(line)})
		}
		for _, m := range useVar.FindAllStringSubmatchIndex(line, -1) {
			s, e := m[2], m[3]
			if s < 0 {
				s, e = m[4], m[5]
			}
			if s > 1 && line[s-2] == '%' && line[s-1] == '%' {
				continue // %%x, a for's variable
			}
			d.Uses = append(d.Uses, Use{Name: line[s:e], Span: span(line, i, s, e)})
		}
	}
	return d
}

// span converts byte offsets on a line to UTF-16 columns.
func span(line string, n, start, end int) Span {
	return Span{Line: n, Start: utf16Col(line, start), End: utf16Col(line, end)}
}

func utf16Col(line string, byteOff int) int {
	col := 0
	for _, r := range line[:byteOff] {
		if r >= 0x10000 {
			col += 2
		} else {
			col++
		}
	}
	return col
}

// ByteOffset converts a UTF-16 column on a line to a byte offset.
func ByteOffset(line string, col int) int {
	n := 0
	for i, r := range line {
		if n >= col {
			return i
		}
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return len(line)
}

// Label finds a label by name; cmd ignores case.
func (d *Doc) Label(name string) (Label, bool) {
	for _, l := range d.Labels {
		if strings.EqualFold(l.Name, name) {
			return l, true
		}
	}
	return Label{}, false
}

// Var finds where a variable is first set.
func (d *Doc) Var(name string) (Var, bool) {
	for _, v := range d.Vars {
		if strings.EqualFold(v.Name, name) {
			return v, true
		}
	}
	return Var{}, false
}

// VarNames are the variables the file sets, each once, sorted.
func (d *Doc) VarNames() []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range d.Vars {
		k := strings.ToLower(v.Name)
		if !seen[k] && !strings.HasPrefix(v.Name, "%") {
			seen[k] = true
			out = append(out, v.Name)
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out
}

// Builtin labels: goto :eof ends the script or the called label.
func Builtin(name string) bool { return strings.EqualFold(name, "eof") }

// Problem is something wrong in the file.
type Problem struct {
	Span     Span
	Severity int // 1 error, 2 warning
	Message  string
}

// Check finds jumps to labels that are not there and labels written twice.
func (d *Doc) Check() []Problem {
	var out []Problem
	for _, j := range d.Jumps {
		if Builtin(j.Name) || strings.ContainsAny(j.Name, "%!") {
			continue // :eof, or a label in a variable
		}
		if _, ok := d.Label(j.Name); !ok {
			verb := "goto"
			if j.Call {
				verb = "call"
			}
			out = append(out, Problem{Span: j.Span, Severity: 1, Message: "No label :" + j.Name + " for this " + verb})
		}
	}
	seen := map[string]bool{}
	for _, l := range d.Labels {
		k := strings.ToLower(l.Name)
		if seen[k] {
			out = append(out, Problem{Span: l.Span, Severity: 2, Message: "Label :" + l.Name + " is already above: goto and call go to the first"})
		}
		seen[k] = true
	}
	return out
}

// At is what is at a position: a label, a jump, a variable use or set, or
// a word.
type At struct {
	Label *Label
	Jump  *Jump
	Use   *Use
	Var   *Var
	Word  string
}

func in(s Span, line, col int) bool { return s.Line == line && s.Start <= col && col <= s.End }

func (d *Doc) At(line, col int) At {
	for i := range d.Labels {
		if in(d.Labels[i].Span, line, col) {
			return At{Label: &d.Labels[i]}
		}
	}
	for i := range d.Jumps {
		if in(d.Jumps[i].Span, line, col) {
			return At{Jump: &d.Jumps[i]}
		}
	}
	for i := range d.Uses {
		if in(d.Uses[i].Span, line, col) {
			return At{Use: &d.Uses[i]}
		}
	}
	for i := range d.Vars {
		if in(d.Vars[i].Span, line, col) {
			return At{Var: &d.Vars[i]}
		}
	}
	if line < 0 || line >= len(d.Lines) {
		return At{}
	}
	text := d.Lines[line]
	b := ByteOffset(text, col)
	s, e := b, b
	isWord := func(c byte) bool {
		return c == '_' || c == '-' || c == '.' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
	}
	for s > 0 && isWord(text[s-1]) {
		s--
	}
	for e < len(text) && isWord(text[e]) {
		e++
	}
	return At{Word: text[s:e]}
}

// LabelRefs are the label's definition and every jump to it.
func (d *Doc) LabelRefs(name string) (def *Label, jumps []Jump) {
	if l, ok := d.Label(name); ok {
		def = &l
	}
	for _, j := range d.Jumps {
		if strings.EqualFold(j.Name, name) {
			jumps = append(jumps, j)
		}
	}
	return def, jumps
}
