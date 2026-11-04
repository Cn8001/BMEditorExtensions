// Command vue-ls starts Vue's language server for BMEditor. The server has
// to be told where a TypeScript with its JavaScript API is, which differs
// per machine and project, so vue-ls finds one and adds it to the editor's
// initialize request; every other message passes through as it is.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "vue-ls:", err)
		os.Exit(1)
	}
}

func run() error {
	server := exec.Command("vue-language-server", "--stdio")
	server.Stdout, server.Stderr = os.Stdout, os.Stderr
	in, err := server.StdinPipe()
	if err != nil {
		return err
	}
	if err := server.Start(); err != nil {
		return fmt.Errorf("start vue-language-server (npm install -g @vue/language-server@2 typescript): %w", err)
	}
	r := bufio.NewReader(os.Stdin)
	first, err := readFrame(r)
	if err != nil {
		return err
	}
	if err := writeFrame(in, withTypeScript(first, tsdk(candidates(rootOf(first))))); err != nil {
		return err
	}
	go func() {
		_, _ = io.Copy(in, r)
		in.Close()
	}()
	return server.Wait()
}

// candidates are the TypeScript lib folders to try, best first: the
// project's own, as VS Code uses; the global one; then one another global
// package carries. TypeScript 7 has no JavaScript API, so a project or a
// global install on it is passed over for the next.
func candidates(root string) []string {
	var dirs []string
	if root != "" {
		dirs = append(dirs, filepath.Join(root, "node_modules", "typescript", "lib"))
	}
	if out, err := exec.Command("npm", "root", "-g").Output(); err == nil {
		global := strings.TrimSpace(string(out))
		dirs = append(dirs, filepath.Join(global, "typescript", "lib"))
		nested, _ := filepath.Glob(filepath.Join(global, "*", "node_modules", "typescript", "lib"))
		scoped, _ := filepath.Glob(filepath.Join(global, "@*", "*", "node_modules", "typescript", "lib"))
		dirs = append(append(dirs, nested...), scoped...)
	}
	return dirs
}

// tsdk is the first folder with TypeScript's JavaScript API in it.
func tsdk(dirs []string) string {
	for _, d := range dirs {
		for _, f := range []string{"typescript.js", "tsserverlibrary.js"} {
			if _, err := os.Stat(filepath.Join(d, f)); err == nil {
				return d
			}
		}
	}
	return ""
}

// rootOf is the folder an initialize request opens, from its rootUri.
func rootOf(body []byte) string {
	var req struct {
		Params struct {
			RootURI string `json:"rootUri"`
		} `json:"params"`
	}
	if json.Unmarshal(body, &req) != nil {
		return ""
	}
	u, err := url.Parse(req.Params.RootURI)
	if err != nil || u.Scheme != "file" {
		return ""
	}
	// file:///C:/x is C:/x on Windows; elsewhere the path is as it is.
	return filepath.FromSlash(windowsPath(u.Path))
}

func windowsPath(p string) string {
	if len(p) > 2 && p[0] == '/' && p[2] == ':' {
		return p[1:]
	}
	return p
}

// withTypeScript adds tsdk to an initialize request, and has the server
// handle TypeScript itself (hybridMode off) rather than through a tsserver
// plugin the editor does not run.
func withTypeScript(body []byte, tsdk string) []byte {
	var req map[string]any
	if json.Unmarshal(body, &req) != nil || req["method"] != "initialize" {
		return body
	}
	params, _ := req["params"].(map[string]any)
	if params == nil {
		params = map[string]any{}
		req["params"] = params
	}
	opts, _ := params["initializationOptions"].(map[string]any)
	if opts == nil {
		opts = map[string]any{}
	}
	opts["typescript"] = map[string]any{"tsdk": tsdk}
	opts["vue"] = map[string]any{"hybridMode": false}
	params["initializationOptions"] = opts
	out, err := json.Marshal(req)
	if err != nil {
		return body
	}
	return out
}

func readFrame(r *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if v, ok := strings.CutPrefix(line, "Content-Length:"); ok {
			length, _ = strconv.Atoi(strings.TrimSpace(v))
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("a message without Content-Length")
	}
	body := make([]byte, length)
	_, err := io.ReadFull(r, body)
	return body, err
}

func writeFrame(w io.Writer, body []byte) error {
	_, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return err
}
