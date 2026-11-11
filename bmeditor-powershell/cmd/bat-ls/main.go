// Command bat-ls is a language server for cmd batch files (.bat, .cmd):
// completion of commands, labels and variables, hover with each command's
// syntax, go to a label, its references and rename, labels as the
// outline, and goto or call to a label that is not there as an error.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	"bmeditor-powershell/internal/batch"
	"bmeditor-powershell/internal/rpc"
)

type server struct {
	out  *rpc.Writer
	mu   sync.Mutex
	docs map[string]*batch.Doc
}

func main() {
	s := &server{out: rpc.NewWriter(os.Stdout), docs: map[string]*batch.Doc{}}
	r := bufio.NewReader(os.Stdin)
	for {
		body, err := rpc.Read(r)
		if err != nil {
			return
		}
		var m rpc.Message
		if err := json.Unmarshal(body, &m); err != nil {
			continue
		}
		if m.Method == "exit" {
			return
		}
		result, rerr := s.handle(m)
		if m.ID == nil || m.Method == "" {
			continue // a notification, or an answer to us
		}
		reply := rpc.Message{JSONRPC: "2.0", ID: m.ID}
		if rerr != nil {
			reply.Error = rerr
		} else {
			reply.Result = result
			if result == nil {
				reply.Result = json.RawMessage("null")
			}
		}
		_ = s.out.JSON(reply)
	}
}

type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type rng struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

func toRange(sp batch.Span) rng {
	return rng{Start: position{sp.Line, sp.Start}, End: position{sp.Line, sp.End}}
}

type location struct {
	URI   string `json:"uri"`
	Range rng    `json:"range"`
}

type docPos struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Position position `json:"position"`
}

func (s *server) doc(uri string) *batch.Doc {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.docs[uri]
}

func (s *server) handle(m rpc.Message) (any, *rpc.Error) {
	switch m.Method {
	case "initialize":
		return map[string]any{
			"capabilities": map[string]any{
				"textDocumentSync":          1, // whole documents
				"completionProvider":        map[string]any{"triggerCharacters": []string{"%", "!", ":", "~"}},
				"hoverProvider":             true,
				"definitionProvider":        true,
				"referencesProvider":        true,
				"documentSymbolProvider":    true,
				"renameProvider":            map[string]any{"prepareProvider": true},
				"documentHighlightProvider": true,
			},
			"serverInfo": map[string]any{"name": "bat-ls", "version": "0.1.0"},
		}, nil
	case "shutdown":
		return nil, nil
	case "textDocument/didOpen":
		var p struct {
			TextDocument struct {
				URI  string `json:"uri"`
				Text string `json:"text"`
			} `json:"textDocument"`
		}
		_ = json.Unmarshal(m.Params, &p)
		s.update(p.TextDocument.URI, p.TextDocument.Text)
	case "textDocument/didChange":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
		}
		_ = json.Unmarshal(m.Params, &p)
		if n := len(p.ContentChanges); n > 0 {
			s.update(p.TextDocument.URI, p.ContentChanges[n-1].Text)
		}
	case "textDocument/didClose":
		var p docPos
		_ = json.Unmarshal(m.Params, &p)
		s.mu.Lock()
		delete(s.docs, p.TextDocument.URI)
		s.mu.Unlock()
		_ = s.out.JSON(rpc.Message{JSONRPC: "2.0", Method: "textDocument/publishDiagnostics", Params: mustJSON(map[string]any{"uri": p.TextDocument.URI, "diagnostics": []any{}})})
	case "textDocument/completion":
		var p docPos
		_ = json.Unmarshal(m.Params, &p)
		if d := s.doc(p.TextDocument.URI); d != nil {
			return complete(d, p.Position), nil
		}
	case "textDocument/hover":
		var p docPos
		_ = json.Unmarshal(m.Params, &p)
		if d := s.doc(p.TextDocument.URI); d != nil {
			return hover(d, p.Position), nil
		}
	case "textDocument/definition":
		var p docPos
		_ = json.Unmarshal(m.Params, &p)
		if d := s.doc(p.TextDocument.URI); d != nil {
			return definition(d, p.TextDocument.URI, p.Position), nil
		}
	case "textDocument/references", "textDocument/documentHighlight":
		var p docPos
		_ = json.Unmarshal(m.Params, &p)
		d := s.doc(p.TextDocument.URI)
		if d == nil {
			return nil, nil
		}
		spans := references(d, p.Position)
		if m.Method == "textDocument/documentHighlight" {
			out := []map[string]any{}
			for _, sp := range spans {
				out = append(out, map[string]any{"range": toRange(sp)})
			}
			return out, nil
		}
		out := []location{}
		for _, sp := range spans {
			out = append(out, location{p.TextDocument.URI, toRange(sp)})
		}
		return out, nil
	case "textDocument/prepareRename":
		var p docPos
		_ = json.Unmarshal(m.Params, &p)
		d := s.doc(p.TextDocument.URI)
		if d == nil {
			return nil, nil
		}
		at := d.At(p.Position.Line, p.Position.Character)
		switch {
		case at.Label != nil:
			return map[string]any{"range": toRange(at.Label.Span), "placeholder": at.Label.Name}, nil
		case at.Jump != nil && !batch.Builtin(at.Jump.Name):
			return map[string]any{"range": toRange(at.Jump.Span), "placeholder": at.Jump.Name}, nil
		}
		return nil, &rpc.Error{Code: -32602, Message: "Only a label can be renamed"}
	case "textDocument/rename":
		var p struct {
			docPos
			NewName string `json:"newName"`
		}
		_ = json.Unmarshal(m.Params, &p)
		d := s.doc(p.TextDocument.URI)
		if d == nil {
			return nil, nil
		}
		name := strings.TrimPrefix(strings.TrimSpace(p.NewName), ":")
		if !labelName.MatchString(name) {
			return nil, &rpc.Error{Code: -32602, Message: fmt.Sprintf("%q is not a label name", p.NewName)}
		}
		edits := []map[string]any{}
		for _, sp := range references(d, p.Position) {
			edits = append(edits, map[string]any{"range": toRange(sp), "newText": name})
		}
		return map[string]any{"changes": map[string]any{p.TextDocument.URI: edits}}, nil
	case "textDocument/documentSymbol":
		var p docPos
		_ = json.Unmarshal(m.Params, &p)
		if d := s.doc(p.TextDocument.URI); d != nil {
			return symbols(d), nil
		}
	default:
		if m.ID != nil && m.Method != "" {
			return nil, &rpc.Error{Code: -32601, Message: "not supported: " + m.Method}
		}
	}
	return nil, nil
}

var labelName = regexp.MustCompile(`^[^\s:+=,;"%!&|<>()]+$`)

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func (s *server) update(uri, text string) {
	d := batch.Parse(text)
	s.mu.Lock()
	s.docs[uri] = d
	s.mu.Unlock()
	diags := []map[string]any{}
	for _, p := range d.Check() {
		diags = append(diags, map[string]any{"range": toRange(p.Span), "severity": p.Severity, "source": "cmd", "message": p.Message})
	}
	_ = s.out.JSON(rpc.Message{JSONRPC: "2.0", Method: "textDocument/publishDiagnostics", Params: mustJSON(map[string]any{"uri": uri, "diagnostics": diags})})
}

// LSP completion item kinds.
const (
	kindFunction = 3
	kindVariable = 6
	kindKeyword  = 14
	kindModule   = 9
	kindValue    = 12
	kindEvent    = 23
)

var (
	jumpBefore = regexp.MustCompile(`(?i)(?:^|[\s&|(@])(?:goto\s+:?|call\s+:)([^\s&|()<>"]*)$`)
	varBefore  = regexp.MustCompile(`([%!])([~\w.$#@\-()]*)$`)
	wordBefore = regexp.MustCompile(`[\w.\-]*$`)
)

type item struct {
	Label         string         `json:"label"`
	Kind          int            `json:"kind"`
	Detail        string         `json:"detail,omitempty"`
	Documentation map[string]any `json:"documentation,omitempty"`
	TextEdit      map[string]any `json:"textEdit,omitempty"`
	FilterText    string         `json:"filterText,omitempty"`
	SortText      string         `json:"sortText,omitempty"`
}

func markdown(s string) map[string]any { return map[string]any{"kind": "markdown", "value": s} }

func complete(d *batch.Doc, pos position) map[string]any {
	items := []item{}
	if pos.Line >= len(d.Lines) {
		return map[string]any{"isIncomplete": false, "items": items}
	}
	text := d.Lines[pos.Line]
	before := text[:batch.ByteOffset(text, pos.Character)]
	edit := func(typed int, newText string) map[string]any {
		return map[string]any{"range": rng{Start: position{pos.Line, pos.Character - typed}, End: position{pos.Line, pos.Character}}, "newText": newText}
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(before)), "rem ") || strings.HasPrefix(strings.TrimSpace(before), "::") {
		return map[string]any{"isIncomplete": false, "items": items}
	}
	if m := jumpBefore.FindStringSubmatch(before); m != nil {
		typed := len([]rune(m[1]))
		for i, l := range d.Labels {
			items = append(items, item{Label: l.Name, Kind: kindFunction, Detail: fmt.Sprintf("label, line %d", l.Span.Line+1), TextEdit: edit(typed, l.Name), SortText: fmt.Sprintf("%04d", i)})
		}
		if strings.Contains(strings.ToLower(before), "goto") {
			items = append(items, item{Label: "eof", Kind: kindKeyword, Detail: "the end of the file, or of the call", TextEdit: edit(typed, "eof"), SortText: "9999"})
		}
		return map[string]any{"isIncomplete": false, "items": items}
	}
	if m := varBefore.FindStringSubmatch(before); m != nil && !strings.HasSuffix(before[:len(before)-len(m[0])], m[1]) {
		mark, typed := m[1], len([]rune(m[2]))
		for _, n := range d.VarNames() {
			v, _ := d.Var(n)
			items = append(items, item{Label: n, Kind: kindVariable, Detail: fmt.Sprintf("set on line %d", v.Span.Line+1), Documentation: markdown("```bat\n" + v.Line + "\n```"), TextEdit: edit(typed, n+mark), FilterText: n, SortText: "0" + strings.ToLower(n)})
		}
		for _, e := range batch.EnvVars {
			items = append(items, item{Label: e.Name, Kind: kindValue, Detail: "Windows", Documentation: markdown(e.Doc), TextEdit: edit(typed, e.Name+mark), FilterText: e.Name, SortText: "1" + strings.ToLower(e.Name)})
		}
		if mark == "%" {
			for _, a := range batch.Args {
				items = append(items, item{Label: "%" + a.Text, Kind: kindEvent, Detail: "argument", Documentation: markdown(a.Doc), TextEdit: edit(typed, a.Text), FilterText: a.Text, SortText: "2" + a.Text})
			}
		}
		return map[string]any{"isIncomplete": false, "items": items}
	}
	typed := len([]rune(wordBefore.FindString(before)))
	for _, c := range batch.Commands() {
		kind, detail := kindModule, "program"
		switch {
		case c.Internal:
			kind, detail = kindFunction, "cmd"
		case c.Keyword:
			kind, detail = kindKeyword, "keyword"
		}
		items = append(items, item{Label: c.Name, Kind: kind, Detail: detail, Documentation: markdown(commandDoc(c)), TextEdit: edit(typed, c.Name)})
	}
	return map[string]any{"isIncomplete": false, "items": items}
}

func commandDoc(c batch.Command) string {
	return "```bat\n" + c.Syntax + "\n```\n\n" + c.Doc
}

func hover(d *batch.Doc, pos position) any {
	at := d.At(pos.Line, pos.Character)
	var text string
	var sp *batch.Span
	switch {
	case at.Jump != nil:
		sp = &at.Jump.Span
		if batch.Builtin(at.Jump.Name) {
			text = "`:eof`: the end of the file, or of the subroutine `call` started."
		} else if l, ok := d.Label(at.Jump.Name); ok {
			text = fmt.Sprintf("Label `:%s`, line %d", l.Name, l.Span.Line+1)
			if c := labelComment(d, l.Span.Line); c != "" {
				text += "\n\n" + c
			}
		} else {
			text = fmt.Sprintf("No label `:%s` in this file", at.Jump.Name)
		}
	case at.Label != nil:
		sp = &at.Label.Span
		_, jumps := d.LabelRefs(at.Label.Name)
		text = fmt.Sprintf("Label `:%s`, jumped to %d time(s)", at.Label.Name, len(jumps))
	case at.Use != nil:
		sp = &at.Use.Span
		if v, ok := d.Var(at.Use.Name); ok {
			text = fmt.Sprintf("Set on line %d:\n```bat\n%s\n```", v.Span.Line+1, v.Line)
		} else {
			for _, e := range batch.EnvVars {
				if strings.EqualFold(e.Name, at.Use.Name) {
					text = "`%" + e.Name + "%`: " + e.Doc
				}
			}
		}
	case at.Word != "":
		if c, ok := batch.Lookup(at.Word); ok {
			text = commandDoc(c)
		}
	}
	if text == "" {
		return nil
	}
	h := map[string]any{"contents": markdown(text)}
	if sp != nil {
		h["range"] = toRange(*sp)
	}
	return h
}

// labelComment is the rem or :: comment just above a label, which often
// says what it does.
func labelComment(d *batch.Doc, line int) string {
	var lines []string
	for i := line - 1; i >= 0; i-- {
		t := strings.TrimSpace(d.Lines[i])
		low := strings.ToLower(t)
		switch {
		case strings.HasPrefix(t, "::"):
			lines = append([]string{strings.TrimSpace(t[2:])}, lines...)
		case low == "rem" || strings.HasPrefix(low, "rem ") || strings.HasPrefix(low, "@rem "):
			lines = append([]string{strings.TrimSpace(t[strings.Index(low, "rem")+3:])}, lines...)
		default:
			return strings.Join(lines, "\n")
		}
	}
	return strings.Join(lines, "\n")
}

func definition(d *batch.Doc, uri string, pos position) any {
	at := d.At(pos.Line, pos.Character)
	switch {
	case at.Jump != nil:
		if l, ok := d.Label(at.Jump.Name); ok {
			return location{uri, toRange(l.Span)}
		}
	case at.Use != nil:
		if v, ok := d.Var(at.Use.Name); ok {
			return location{uri, toRange(v.Span)}
		}
	case at.Label != nil:
		return location{uri, toRange(at.Label.Span)}
	}
	return nil
}

// references are the spans of the label, or the variable, at pos: where
// it is defined and every use.
func references(d *batch.Doc, pos position) []batch.Span {
	at := d.At(pos.Line, pos.Character)
	name := ""
	switch {
	case at.Label != nil:
		name = at.Label.Name
	case at.Jump != nil && !batch.Builtin(at.Jump.Name):
		name = at.Jump.Name
	case at.Use != nil || at.Var != nil:
		v := ""
		if at.Use != nil {
			v = at.Use.Name
		} else {
			v = at.Var.Name
		}
		var out []batch.Span
		for _, x := range d.Vars {
			if strings.EqualFold(x.Name, v) {
				out = append(out, x.Span)
			}
		}
		for _, u := range d.Uses {
			if strings.EqualFold(u.Name, v) {
				out = append(out, u.Span)
			}
		}
		return out
	default:
		return nil
	}
	var out []batch.Span
	def, jumps := d.LabelRefs(name)
	if def != nil {
		out = append(out, def.Span)
	}
	for _, j := range jumps {
		out = append(out, j.Span)
	}
	return out
}

// symbols are the labels, each up to the next one, as the outline.
func symbols(d *batch.Doc) []map[string]any {
	out := []map[string]any{}
	for i, l := range d.Labels {
		end := len(d.Lines) - 1
		if i+1 < len(d.Labels) {
			end = d.Labels[i+1].Span.Line - 1
		}
		for end > l.Span.Line && strings.TrimSpace(d.Lines[end]) == "" {
			end--
		}
		full := rng{Start: position{l.Span.Line, 0}, End: position{end, len([]rune(d.Lines[end]))}}
		out = append(out, map[string]any{"name": ":" + l.Name, "kind": 12, "range": full, "selectionRange": toRange(l.Span)})
	}
	return out
}
