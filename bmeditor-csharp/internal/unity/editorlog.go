package unity

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Message is a compiler message Unity wrote to its Editor.log.
type Message struct {
	File     string `json:"file"` // absolute
	Rel      string `json:"rel"`  // as Unity wrote it, relative to the project
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	Severity string `json:"severity"` // error, warning
	Code     string `json:"code"`
	Message  string `json:"message"`
}

// A compiler message: Assets\Dir\File.cs(216,30): warning CS0168: text
var compileLine = regexp.MustCompile(`^(.+?\.cs)\((\d+),(\d+)\):\s*(error|warning)\s+([A-Z]+[0-9]+):\s*(.*)$`)

// compileStart begins a new compilation, after which the earlier messages
// no longer hold.
const compileStart = "Requested script compilation"

// EditorLog follows Unity's Editor.log for one project's compiler
// messages. The log belongs to the Unity Editor last started, whatever its
// project; Project says which one that is.
type EditorLog struct {
	Path    string
	Root    string // the project the messages are wanted for
	Project string // the project the log is Unity's for, from -projectpath

	offset   int64
	partial  string
	argsNext bool
	messages []Message
	seen     map[string]bool
}

// DefaultEditorLog is where Unity writes its log on Windows, macOS and
// Linux.
func DefaultEditorLog() string {
	if la := os.Getenv("LOCALAPPDATA"); la != "" {
		return filepath.Join(la, "Unity", "Editor", "Editor.log")
	}
	home, _ := os.UserHomeDir()
	if _, err := os.Stat(filepath.Join(home, "Library", "Logs")); err == nil {
		return filepath.Join(home, "Library", "Logs", "Unity", "Editor.log")
	}
	return filepath.Join(home, ".config", "unity3d", "Editor.log")
}

// Messages are the current compiler messages, for Root's files; none when
// the log is another project's.
func (l *EditorLog) Messages() []Message {
	if !l.Matches() {
		return []Message{}
	}
	return append([]Message{}, l.messages...)
}

// Matches says whether the log is Root's project's, or does not say.
func (l *EditorLog) Matches() bool {
	return l.Project == "" || SamePath(l.Project, l.Root)
}

// SamePath compares paths as Windows does: case and separators aside.
func SamePath(a, b string) bool {
	norm := func(p string) string {
		return strings.TrimSuffix(strings.ToLower(strings.ReplaceAll(filepath.Clean(p), `\`, "/")), "/")
	}
	return norm(a) == norm(b)
}

// Poll reads what the log gained since the last call. It returns the new
// lines and whether the messages changed. A log that got shorter is a new
// Unity session, read from its start.
func (l *EditorLog) Poll() (lines []string, changed bool, err error) {
	f, err := os.Open(l.Path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, false, err
	}
	if info.Size() < l.offset {
		l.reset()
		changed = true
	}
	if info.Size() == l.offset {
		return nil, changed, nil
	}
	const most = 16 << 20
	if info.Size()-l.offset > most {
		l.offset = info.Size() - most
		l.partial = ""
	}
	if _, err := f.Seek(l.offset, io.SeekStart); err != nil {
		return nil, changed, err
	}
	buf, err := io.ReadAll(io.LimitReader(f, info.Size()-l.offset))
	if err != nil {
		return nil, changed, err
	}
	l.offset += int64(len(buf))
	text := l.partial + string(buf)
	end := strings.LastIndexByte(text, '\n')
	if end < 0 {
		l.partial = text
		return nil, changed, nil
	}
	l.partial = text[end+1:]
	for _, line := range strings.Split(text[:end], "\n") {
		line = strings.TrimRight(line, "\r")
		lines = append(lines, line)
		if l.feed(line) {
			changed = true
		}
	}
	return lines, changed, nil
}

func (l *EditorLog) reset() {
	l.offset, l.partial, l.Project, l.argsNext = 0, "", "", false
	l.messages, l.seen = nil, nil
}

// feed takes one line, and says whether the messages changed.
func (l *EditorLog) feed(line string) bool {
	if l.argsNext {
		l.argsNext = false
		l.Project = strings.TrimSpace(line)
		return true
	}
	if strings.EqualFold(strings.TrimSpace(line), "-projectpath") {
		l.argsNext = true
		return false
	}
	if strings.Contains(line, compileStart) {
		had := len(l.messages) > 0
		l.messages, l.seen = nil, nil
		return had
	}
	m := compileLine.FindStringSubmatch(line)
	if m == nil {
		return false
	}
	if l.seen == nil {
		l.seen = map[string]bool{}
	}
	if l.seen[line] { // Unity writes some twice
		return false
	}
	l.seen[line] = true
	ln, _ := strconv.Atoi(m[2])
	col, _ := strconv.Atoi(m[3])
	rel := m[1]
	file := rel
	if !filepath.IsAbs(rel) && !strings.HasPrefix(rel, `\`) && !(len(rel) > 1 && rel[1] == ':') {
		file = filepath.Join(l.Root, filepath.FromSlash(strings.ReplaceAll(rel, `\`, "/")))
	}
	l.messages = append(l.messages, Message{File: file, Rel: rel, Line: ln, Col: col, Severity: m[4], Code: m[5], Message: m[6]})
	return true
}
