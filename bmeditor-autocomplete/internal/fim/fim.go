// Package fim is the request for a completion in the middle of a file (the
// text before and after the cursor, and snippets of other open files), cut
// to a size a small model takes quickly, and the cleaning of what the
// model answers into the text to show.
package fim

import (
	"strings"
)

// Request is a completion asked for at the cursor of a file.
type Request struct {
	Path     string    `json:"path"`
	Language string    `json:"language"`
	Prefix   string    `json:"prefix"` // the file up to the cursor
	Suffix   string    `json:"suffix"` // the file after the cursor
	Extra    []Snippet `json:"extra"`  // other open files, most recent first
}

type Snippet struct {
	Path string `json:"path"`
	Text string `json:"text"`
}

// Limits, in bytes (about four to a token): about 3k tokens before the
// cursor, 1k after, 2k of other files. A small model answers fast with this.
const (
	MaxPrefix = 12000
	MaxSuffix = 4000
	MaxExtra  = 8000
)

// Window cuts a request to the limits, at line starts so that no line is
// half there.
func Window(r Request) Request {
	if len(r.Prefix) > MaxPrefix {
		p := r.Prefix[len(r.Prefix)-MaxPrefix:]
		if i := strings.IndexByte(p, '\n'); i >= 0 {
			p = p[i+1:]
		}
		r.Prefix = p
	}
	if len(r.Suffix) > MaxSuffix {
		s := r.Suffix[:MaxSuffix]
		if i := strings.LastIndexByte(s, '\n'); i >= 0 {
			s = s[:i+1]
		}
		r.Suffix = s
	}
	left := MaxExtra
	var extra []Snippet
	for _, e := range r.Extra {
		if left <= 0 {
			break
		}
		if len(e.Text) > left {
			e.Text = e.Text[:left]
			if i := strings.LastIndexByte(e.Text, '\n'); i >= 0 {
				e.Text = e.Text[:i+1]
			}
		}
		left -= len(e.Text)
		extra = append(extra, e)
	}
	r.Extra = extra
	return r
}

// MaxLines is the most a suggestion shows.
const MaxLines = 16

// Clean turns what a model answered into the text to insert: without code
// fences, cut where it runs into the text after the cursor, at two blank
// lines, and at MaxLines. "" means nothing to suggest.
func Clean(out string, r Request) string {
	out = strings.ReplaceAll(out, "\r\n", "\n")
	out = stripFences(out)
	// Mid-line, nothing but the rest of the line.
	after := r.Suffix
	if i := strings.IndexByte(after, '\n'); i >= 0 {
		after = after[:i]
	}
	if strings.TrimSpace(after) != "" {
		if i := strings.IndexByte(out, '\n'); i >= 0 {
			out = out[:i]
		}
		if a := strings.TrimSpace(after); strings.HasSuffix(strings.TrimRight(out, " "), a) {
			out = strings.TrimRight(out, " ")
			out = out[:len(out)-len(a)]
		}
		return strings.TrimRight(out, " \t")
	}
	lines := strings.Split(out, "\n")
	// Stop where the answer repeats the next line of the file.
	next := ""
	for _, l := range strings.Split(r.Suffix, "\n") {
		if strings.TrimSpace(l) != "" {
			next = strings.TrimSpace(l)
			break
		}
	}
	keep := lines[:0]
	for i, l := range lines {
		if i > 0 && next != "" && strings.TrimSpace(l) == next {
			break
		}
		if i > 0 && strings.TrimSpace(l) == "" && i+1 < len(lines) && strings.TrimSpace(lines[i+1]) == "" {
			break // two blank lines: the block is over
		}
		keep = append(keep, l)
		if len(keep) == MaxLines {
			break
		}
	}
	out = strings.Join(keep, "\n")
	out = strings.TrimRight(out, " \t\n")
	if strings.TrimSpace(out) == "" {
		return ""
	}
	return out
}

func stripFences(s string) string {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "```") {
		return s
	}
	if i := strings.IndexByte(t, '\n'); i >= 0 {
		t = t[i+1:]
	} else {
		return ""
	}
	t = strings.TrimSuffix(strings.TrimRight(t, " \n"), "```")
	return strings.TrimRight(t, "\n")
}
