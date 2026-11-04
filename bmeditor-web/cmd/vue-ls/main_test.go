package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWithTypeScript_AddsTsdkAndKeepsTheClientsOptions(t *testing.T) {
	// Arrange
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"rootUri":"file:///p","initializationOptions":{"keep":1}}}`)

	// Act
	out := withTypeScript(body, "/npm/typescript/lib")

	// Assert
	var got struct {
		Params struct {
			RootURI string `json:"rootUri"`
			Opts    struct {
				Keep       int `json:"keep"`
				TypeScript struct {
					Tsdk string `json:"tsdk"`
				} `json:"typescript"`
				Vue struct {
					HybridMode *bool `json:"hybridMode"`
				} `json:"vue"`
			} `json:"initializationOptions"`
		} `json:"params"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	p := got.Params
	if p.RootURI != "file:///p" || p.Opts.Keep != 1 || p.Opts.TypeScript.Tsdk != "/npm/typescript/lib" || p.Opts.Vue.HybridMode == nil || *p.Opts.Vue.HybridMode {
		t.Fatalf("got %s", out)
	}
}

func TestWithTypeScript_LeavesOtherMessagesAlone(t *testing.T) {
	// Arrange
	body := []byte(`{"jsonrpc":"2.0","method":"initialized","params":{}}`)

	// Act
	out := withTypeScript(body, "/x")

	// Assert
	if !bytes.Equal(out, body) {
		t.Fatalf("changed to %s", out)
	}
}

func TestFrames_RoundTrip(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	if err := writeFrame(&buf, []byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}

	// Act
	got, err := readFrame(bufio.NewReader(&buf))

	// Assert
	if err != nil || string(got) != `{"a":1}` {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestTsdk_PassesOverATypeScriptWithoutItsAPI(t *testing.T) {
	// Arrange
	ts7 := filepath.Join(t.TempDir(), "lib")
	ts5 := filepath.Join(t.TempDir(), "lib")
	for _, d := range []string{ts7, ts5} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(ts7, "tsc.js"), nil, 0o644)
	os.WriteFile(filepath.Join(ts5, "tsserverlibrary.js"), nil, 0o644)

	// Act
	got := tsdk([]string{filepath.Join(t.TempDir(), "missing"), ts7, ts5})

	// Assert
	if got != ts5 {
		t.Fatalf("tsdk = %q, want %q", got, ts5)
	}
}

func TestRootOf_ReadsTheFolderFromRootUri(t *testing.T) {
	// Arrange
	body := []byte(`{"method":"initialize","params":{"rootUri":"file:///home/me/my%20app"}}`)

	// Act
	got := rootOf(body)

	// Assert
	if got != filepath.FromSlash("/home/me/my app") {
		t.Fatalf("root = %q", got)
	}
}
