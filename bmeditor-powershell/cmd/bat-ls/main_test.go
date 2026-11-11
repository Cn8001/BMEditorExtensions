package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"bmeditor-powershell/internal/batch"
	"bmeditor-powershell/internal/rpc"
)

const script = "@echo off\r\nset \"target=Release\"\r\ncall :build %target%\r\ngoto :eof\r\n\r\nrem Builds one configuration.\r\n:build\r\necho %~1\r\ngoto :eof\r\n"

// call sends one request to a server and returns its answer.
func call(t *testing.T, s *server, method string, params any) any {
	t.Helper()
	raw, _ := json.Marshal(params)
	result, err := s.handle(rpc.Message{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: method, Params: raw})
	if err != nil {
		t.Fatalf("%s: %s", method, err.Message)
	}
	b, _ := json.Marshal(result)
	var v any
	_ = json.Unmarshal(b, &v)
	return v
}

func open(t *testing.T) (*server, *bytes.Buffer) {
	var out bytes.Buffer
	s := &server{out: rpc.NewWriter(&out), docs: map[string]*batch.Doc{}}
	call(t, s, "textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": "file:///b.cmd", "text": script}})
	return s, &out
}

func at(line, col int) map[string]any {
	return map[string]any{"textDocument": map[string]any{"uri": "file:///b.cmd"}, "position": map[string]any{"line": line, "character": col}}
}

func labels(items any) []string {
	var out []string
	for _, it := range items.(map[string]any)["items"].([]any) {
		out = append(out, it.(map[string]any)["label"].(string))
	}
	return out
}

func TestCompletion(t *testing.T) {
	s, _ := open(t)

	// After call :, the labels.
	s.update("file:///b.cmd", script+"call :")
	if got := labels(call(t, s, "textDocument/completion", at(9, 6))); strings.Join(got, ",") != "build" {
		t.Errorf("after call : = %v", got)
	}
	// After goto, the labels and :eof.
	s.update("file:///b.cmd", script+"goto b")
	if got := labels(call(t, s, "textDocument/completion", at(9, 6))); strings.Join(got, ",") != "build,eof" {
		t.Errorf("after goto = %v", got)
	}
	// After %, the variables set, then Windows', then the arguments.
	s.update("file:///b.cmd", script+"echo %ta")
	got := labels(call(t, s, "textDocument/completion", at(9, 8)))
	if got[0] != "target" || !contains(got, "USERPROFILE") || !contains(got, "%~dp0") {
		t.Errorf("after %% = %v", got)
	}
	// Elsewhere, the commands.
	s.update("file:///b.cmd", script+"rob")
	if got := labels(call(t, s, "textDocument/completion", at(9, 3))); !contains(got, "robocopy") || !contains(got, "setlocal") {
		t.Errorf("commands = %v", got)
	}
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func TestHoverDefinitionReferencesRename(t *testing.T) {
	s, out := open(t)
	if !strings.Contains(out.String(), `"diagnostics":[]`) {
		t.Errorf("a file without problems should have no diagnostics: %s", out)
	}

	h := call(t, s, "textDocument/hover", at(2, 7)).(map[string]any)
	if v := h["contents"].(map[string]any)["value"].(string); !strings.Contains(v, "line 7") || !strings.Contains(v, "Builds one configuration.") {
		t.Errorf("hover on the call = %q", v)
	}
	h = call(t, s, "textDocument/hover", at(0, 2)).(map[string]any)
	if v := h["contents"].(map[string]any)["value"].(string); !strings.Contains(v, "echo [on | off]") {
		t.Errorf("hover on echo = %q", v)
	}

	d := call(t, s, "textDocument/definition", at(2, 7)).(map[string]any)
	if d["range"].(map[string]any)["start"].(map[string]any)["line"].(float64) != 6 {
		t.Errorf("definition = %v", d)
	}
	refs := call(t, s, "textDocument/references", at(6, 2)).([]any)
	if len(refs) != 2 {
		t.Errorf("references = %v", refs)
	}
	edit := call(t, s, "textDocument/rename", map[string]any{"textDocument": map[string]any{"uri": "file:///b.cmd"}, "position": map[string]any{"line": 6, "character": 2}, "newName": ":compile"})
	edits := edit.(map[string]any)["changes"].(map[string]any)["file:///b.cmd"].([]any)
	if len(edits) != 2 || edits[0].(map[string]any)["newText"] != "compile" {
		t.Errorf("rename = %v", edits)
	}
	syms := call(t, s, "textDocument/documentSymbol", at(0, 0)).([]any)
	if len(syms) != 1 || syms[0].(map[string]any)["name"] != ":build" {
		t.Errorf("symbols = %v", syms)
	}
}

func TestDiagnostics_MissingLabel(t *testing.T) {
	s, out := open(t)
	out.Reset()
	s.update("file:///b.cmd", "goto nowhere\r\n")
	body, err := rpc.Read(bufio.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "No label :nowhere for this goto") {
		t.Errorf("diagnostics = %s", body)
	}
}
