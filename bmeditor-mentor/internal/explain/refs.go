package explain

import (
	"sort"
	"strings"

	"bmeditor-mentor/internal/index"
)

// Every place a symbol is used, as an editor lists references: a page at a
// time, so a symbol used a million times answers as fast as one used once.
// Finding the places is the index's work, which is in memory; only the
// lines of the page asked for are read from disk.

// Use is one place a symbol is used.
type Use struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Col    int    `json:"col,omitempty"`
	Text   string `json:"text"`   // the line, its indentation left out
	Indent int    `json:"indent"` // how much indentation was left out, so Col still finds the name
	Name   string `json:"name"`   // the name used there: the symbol's, or its field's or method's
	In     string `json:"in,omitempty"`
	Test   bool   `json:"test,omitempty"`
}

// Uses is a page of the places a symbol is used.
type Uses struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Total  int    `json:"total"` // places, on every page
	Files  int    `json:"files"` // files they are in
	Offset int    `json:"offset"`
	Items  []Use  `json:"items"`
}

// RefsOf answers the places key is used, sorted by file and line, from
// offset, limit of them. A type is used through its fields and methods too.
// Nil when key is unknown.
func RefsOf(ix *index.Index, key string, offset, limit int) *Uses {
	s := ix.Symbols[key]
	if s == nil {
		return nil
	}
	type site struct {
		r index.Relation
		u *index.Symbol
	}
	var all []site
	seen := map[string]bool{}
	files := map[string]bool{}
	for _, r := range uses(ix, s) {
		switch r.Kind {
		case index.RelCalls, index.RelCallsDynamic, index.RelReferences, index.RelConstructs, index.RelEmbeds:
		default:
			continue
		}
		if r.File == "" || r.Line == 0 {
			continue
		}
		k := r.File + ":" + itoa(r.Line) + ":" + itoa(r.Col) + ":" + r.To
		if seen[k] {
			continue
		}
		seen[k] = true
		files[r.File] = true
		all = append(all, site{r: r, u: ix.Symbols[r.From]})
	}
	sort.Slice(all, func(i, j int) bool {
		a, b := all[i].r, all[j].r
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Col < b.Col
	})
	out := &Uses{Key: key, Name: display(s), Total: len(all), Files: len(files), Offset: offset, Items: []Use{}}
	if offset < 0 || offset >= len(all) {
		return out
	}
	if limit <= 0 {
		limit = 100
	}
	end := min(len(all), offset+limit)
	lines := map[string][]string{}
	for _, st := range all[offset:end] {
		r := st.r
		text := ""
		ls, ok := lines[r.File]
		if !ok {
			ls = ReadLines(r.File)
			lines[r.File] = ls
		}
		indent := 0
		if r.Line <= len(ls) {
			full := ls[r.Line-1]
			text = strings.TrimLeft(full, " \t")
			indent = len(full) - len(text)
			if len(text) > 240 {
				text = text[:240] + "…"
			}
		}
		name := s.Name
		if r.To != s.Key {
			if t := ix.Symbols[r.To]; t != nil {
				name = t.Name
			}
		}
		// The column points at the expression: auth.UserIdentity's at auth.
		// It moves on to the name, which is what is marked.
		col := r.Col
		if at := col - 1 - indent; col > 0 && at >= 0 && at < len(text) {
			if i := strings.Index(text[at:], name); i > 0 {
				col += i
			}
		}
		u := Use{File: r.File, Line: r.Line, Col: col, Text: text, Indent: indent, Name: name}
		if st.u != nil {
			u.In, u.Test = display(st.u), st.u.Test
		}
		out.Items = append(out.Items, u)
	}
	return out
}

// display is a symbol's name as code writes it: Recv.Name for a method.
func display(s *index.Symbol) string {
	if s.Recv != "" {
		return s.Recv + "." + s.Name
	}
	return s.Name
}
