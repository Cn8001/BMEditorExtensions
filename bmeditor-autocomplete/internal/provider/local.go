package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"bmeditor-autocomplete/internal/download"
	"bmeditor-autocomplete/internal/fim"
)

// The llama.cpp build used: pinned, so that what runs is what was tried.
const llamaBuild = "b12283"

// Model is a code model llama.cpp fills in the middle with.
type Model struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	URL   string `json:"url"`
	File  string `json:"file"`
	Size  int64  `json:"size"`
	SHA   string `json:"sha256"`
	// Downloaded is filled in by Here, for the menu that picks one.
	Downloaded bool `json:"downloaded"`
}

func hf(id, label, repo, file string, size int64, sha string) Model {
	return Model{ID: id, Label: label, URL: "https://huggingface.co/" + repo + "/resolve/main/" + file, File: file, Size: size, SHA: sha}
}

// Models are Qwen2.5-Coder's base models (they fill in the middle), as
// ggml-org publishes them for llama.cpp, Q8_0.
var Models = []Model{
	hf("0.5b", "Qwen2.5-Coder 0.5B: fast on any CPU, 0.5 GB", "ggml-org/Qwen2.5-Coder-0.5B-Q8_0-GGUF", "qwen2.5-coder-0.5b-q8_0.gguf", 531068128, "d0f8cd6c49bab52a0abdbe47948518b1f4d9b1a8a2a6825099cea31cea10ac56"),
	hf("1.5b", "Qwen2.5-Coder 1.5B: better, wants a recent CPU, 1.6 GB", "ggml-org/Qwen2.5-Coder-1.5B-Q8_0-GGUF", "qwen2.5-coder-1.5b-q8_0.gguf", 1646573056, "29871c94d15727a6e243f79a37113d4ae625a6215b5e800bf41a23af2da32832"),
	hf("3b", "Qwen2.5-Coder 3B: best, slow without a GPU, 3.3 GB", "ggml-org/Qwen2.5-Coder-3B-Q8_0-GGUF", "qwen2.5-coder-3b-q8_0.gguf", 3285476160, "a522a906e299ed34db738b9626b2cd0da9e446c14674468a22fc2eae3dbd344d"),
}

// DefaultModel is the smallest: fast everywhere, and half a gigabyte.
const DefaultModel = "0.5b"

// Home is where downloads go: the user's cache, never the extension.
func Home() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "bmeditor-autocomplete"), nil
}

// Here is Models, each saying whether it is downloaded.
func Here() []Model {
	home, err := Home()
	out := make([]Model, len(Models))
	for i, m := range Models {
		m.Downloaded = err == nil && isFile(filepath.Join(home, "models", m.File))
		out[i] = m
	}
	return out
}

// RemoveDownloads deletes the downloaded models and llama.cpp.
func RemoveDownloads() error {
	home, err := Home()
	if err != nil {
		return err
	}
	return os.RemoveAll(home)
}

// local runs llama.cpp's llama-server on a model and asks its /infill.
type local struct {
	home    string
	model   Model // URL "" for a .gguf file of one's own
	path    string
	threads int
	gpu     bool

	mu     sync.Mutex
	cmd    *exec.Cmd
	url    string
	idle   *time.Timer
	exited chan struct{}
	stderr *tail
}

// IdleStop is how long the server runs without a request before it stops,
// giving its memory back.
var IdleStop = 10 * time.Minute

func newLocal(c Config) (*local, error) {
	home, err := Home()
	if err != nil {
		return nil, err
	}
	p := &local{home: home, threads: c.Threads, gpu: c.GPU}
	id := strings.TrimSpace(c.LocalModel)
	if id == "" {
		id = DefaultModel
	}
	if strings.EqualFold(id, "custom") {
		id = strings.TrimSpace(c.ModelFile)
		if id == "" {
			return nil, errors.New("no model file: Settings › Autocomplete › Model file takes a .gguf or its URL")
		}
	}
	for _, m := range Models {
		if strings.EqualFold(m.ID, id) {
			p.model = m
			p.path = filepath.Join(home, "models", m.File)
			return p, nil
		}
	}
	switch {
	case strings.HasPrefix(id, "http://") || strings.HasPrefix(id, "https://"):
		// A model of one's own, by URL: downloaded once, unchecked.
		name := path.Base(strings.SplitN(id, "?", 2)[0])
		if !strings.HasSuffix(strings.ToLower(name), ".gguf") {
			return nil, fmt.Errorf("%s is not a .gguf file", id)
		}
		p.model = Model{ID: id, Label: name, URL: id, File: name}
		p.path = filepath.Join(home, "models", name)
	case strings.HasSuffix(strings.ToLower(id), ".gguf"):
		p.model = Model{ID: id, Label: filepath.Base(id), File: filepath.Base(id)}
		p.path = id
	default:
		return nil, fmt.Errorf("unknown model %q: 0.5b, 1.5b, 3b, or a .gguf file or its URL", id)
	}
	return p, nil
}

// engineDir keeps the GPU build apart, so that switching downloads it
// rather than running the other.
func (p *local) engineDir() string {
	if p.gpu {
		return filepath.Join(p.home, "llama.cpp", llamaBuild+"-gpu")
	}
	return filepath.Join(p.home, "llama.cpp", llamaBuild)
}

// server finds llama-server in the unpacked build.
func (p *local) server() string {
	name := "llama-server"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	found := ""
	_ = filepath.WalkDir(p.engineDir(), func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == name {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func (p *local) Ready() (bool, string) {
	if p.server() == "" {
		return false, "llama.cpp is not downloaded: Autocomplete: Get Model Ready downloads it"
	}
	if !isFile(p.path) {
		if p.model.URL == "" {
			return false, "no model file at " + p.path
		}
		return false, p.model.Label + " is not downloaded: Autocomplete: Get Model Ready downloads it"
	}
	return true, ""
}

// Prepare downloads llama.cpp and the model when they are not here, and
// starts the server.
func (p *local) Prepare(ctx context.Context, progress func(string)) error {
	if p.server() == "" {
		if err := p.downloadEngine(ctx, progress); err != nil {
			return err
		}
	}
	if !isFile(p.path) {
		if p.model.URL == "" {
			return fmt.Errorf("no model file at %s", p.path)
		}
		progress(fmt.Sprintf("Downloading %s from %s", p.model.Label, p.model.URL))
		if err := download.Fetch(ctx, p.model.URL, p.path, p.model.Size, p.model.SHA, bar(progress, p.model.File)); err != nil {
			return fmt.Errorf("download %s: %w", p.model.File, err)
		}
	}
	on := "CPU"
	if p.gpu {
		on = "GPU"
	}
	progress(fmt.Sprintf("Starting llama.cpp on %s (%s)", filepath.Base(p.path), on))
	if _, err := p.start(ctx); err != nil {
		return err
	}
	progress("Ready: " + p.model.Label)
	return nil
}

// bar reports a download every tenth of the way.
func bar(progress func(string), name string) download.Progress {
	last := -1
	return func(done, total int64) {
		if total <= 0 {
			return
		}
		pct := int(done * 100 / total)
		if pct/10 != last/10 || pct == 100 && last != 100 {
			last = pct
			progress(fmt.Sprintf("%s: %d%% of %.0f MB", name, pct, float64(total)/1e6))
		}
	}
}

// engine is a llama.cpp build to download, as its release publishes it.
type engine struct {
	Name string
	Size int64
	SHA  string
}

// engines are the pinned build's CPU builds, by system, with the sizes and
// SHA-256 sums GitHub publishes: fixed here so that the download needs no
// call to GitHub's API, which has a rate limit and was one more host to
// reach.
var engines = map[string]engine{
	"windows/amd64": {"win-cpu-x64.zip", 25058174, "dafb19ac757bfb7cfc1f91c6e26703264df861c4331e3ef6dc9543ae8d010c5e"},
	"windows/arm64": {"win-cpu-arm64.zip", 13409429, "a9e50ed274c6f31a8549297c2c571ebaabfe27b15bccd3a1b3115fe8ca5aca8f"},
	"linux/amd64":   {"ubuntu-x64.tar.gz", 23283834, "5f24a62b84a37c186efd1e3e1d77bfad3a1772d9aa35bfcc9e1572723d0c4faf"},
	"linux/arm64":   {"ubuntu-arm64.tar.gz", 18926106, "2e44d8517120793b0227d28012a794d114a7d4ad7c3bead4a62b6f3f78b469e7"},
	"darwin/arm64":  {"macos-arm64.tar.gz", 16870708, "bbc2abe2001293687540fb274bf4114b27b56af1fc166ee76730f64996d68b00"},
	"darwin/amd64":  {"macos-x64.tar.gz", 12613034, "7179905c572cd11e9eadf9669058b49846cc9519f3d8d10497cad19fbf9eb8b0"},
}

// gpuEngines are the pinned build's Vulkan builds: one download for
// NVIDIA, AMD and Intel, where CUDA's needs 400 MB more of runtime. Without
// a Vulkan driver they run on the CPU. macOS builds have Metal already.
var gpuEngines = map[string]engine{
	"windows/amd64": {"win-vulkan-x64.zip", 38022362, "58c3426f7473bc08135c83333259366521fc940e4ad7bc6ad2b26755cc42cb03"},
	"windows/arm64": {"win-vulkan-arm64.zip", 25954885, "78c3fe2ce5705663156e296d49ba8b734926cae424f016c61e66b28098054e17"},
	"linux/amd64":   {"ubuntu-vulkan-x64.tar.gz", 36106160, "34138a7d99f2cc4b2be8b6978a656eb77240e742ba0870a58d418024c6e2e8b8"},
	"linux/arm64":   {"ubuntu-vulkan-arm64.tar.gz", 29383025, "3b8af7db1538b7995bbec32c4b3d739addf1f39f888ca668dd8a3478b106a5cf"},
}

// engineFor is the build for this system (a variable for tests).
var engineFor = func(gpu bool) (engine, error) {
	sys := runtime.GOOS + "/" + runtime.GOARCH
	e, ok := gpuEngines[sys]
	if !gpu || !ok {
		e, ok = engines[sys]
	}
	if !ok {
		return engine{}, fmt.Errorf("llama.cpp has no build for %s/%s: run Ollama and choose it instead", runtime.GOOS, runtime.GOARCH)
	}
	e.Name = "llama-" + llamaBuild + "-bin-" + e.Name
	return e, nil
}

// releases is where the pinned build's files are (a variable for tests).
var releases = "https://github.com/ggml-org/llama.cpp/releases/download/" + llamaBuild + "/"

func (p *local) downloadEngine(ctx context.Context, progress func(string)) error {
	a, err := engineFor(p.gpu)
	if err != nil {
		return err
	}
	archive := filepath.Join(p.home, "llama.cpp", a.Name)
	progress(fmt.Sprintf("Downloading llama.cpp %s (%.0f MB)", llamaBuild, float64(a.Size)/1e6))
	if err := download.Fetch(ctx, releases+a.Name, archive, a.Size, a.SHA, bar(progress, a.Name)); err != nil {
		return err
	}
	dir := p.engineDir()
	_ = os.RemoveAll(dir)
	if err := download.Extract(archive, dir); err != nil {
		return err
	}
	_ = os.Remove(archive)
	if p.server() == "" {
		return fmt.Errorf("%s has no llama-server", a.Name)
	}
	return nil
}

// tail keeps the last lines a process wrote, to say why it stopped.
type tail struct {
	mu    sync.Mutex
	lines []string
}

func (t *tail) Write(b []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, l := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		t.lines = append(t.lines, l)
	}
	if len(t.lines) > 20 {
		t.lines = t.lines[len(t.lines)-20:]
	}
	return len(b), nil
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.Join(t.lines, "\n")
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// start runs llama-server unless it runs, and waits for its model to load.
func (p *local) start(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd != nil {
		select {
		case <-p.exited:
			p.cmd = nil
		default:
			p.touch()
			return p.url, nil
		}
	}
	if ok, why := p.Ready(); !ok {
		return "", errors.New(why)
	}
	port, err := freePort()
	if err != nil {
		return "", err
	}
	args := []string{"-m", p.path, "--host", "127.0.0.1", "--port", strconv.Itoa(port),
		"-c", "8192", "-b", "1024", "-ub", "1024", "--cache-reuse", "256"}
	if p.threads > 0 {
		args = append(args, "-t", strconv.Itoa(p.threads))
	}
	if p.gpu {
		args = append(args, "-ngl", "99") // every layer: these models fit any GPU
	}
	cmd := exec.Command(p.server(), args...)
	cmd.Dir = filepath.Dir(p.server()) // its DLLs are beside it
	p.stderr = &tail{}
	cmd.Stdout, cmd.Stderr = p.stderr, p.stderr
	hide(cmd)
	if err := cmd.Start(); err != nil {
		return "", err
	}
	bind(cmd)
	exited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(exited)
	}()
	url := "http://127.0.0.1:" + strconv.Itoa(port)
	deadline := time.After(3 * time.Minute)
	for {
		select {
		case <-exited:
			return "", fmt.Errorf("llama.cpp stopped:\n%s", p.stderr)
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			return "", ctx.Err()
		case <-deadline:
			_ = cmd.Process.Kill()
			return "", errors.New("llama.cpp did not load the model in 3 minutes")
		case <-time.After(250 * time.Millisecond):
		}
		res, err := http.Get(url + "/health")
		if err != nil {
			continue
		}
		res.Body.Close()
		if res.StatusCode == http.StatusOK {
			break
		}
	}
	p.cmd, p.url, p.exited = cmd, url, exited
	p.touch()
	return url, nil
}

// touch puts off the idle stop. p.mu is held.
func (p *local) touch() {
	if p.idle != nil {
		p.idle.Stop()
	}
	p.idle = time.AfterFunc(IdleStop, p.Close)
}

func (p *local) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.idle != nil {
		p.idle.Stop()
	}
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
		<-p.exited
	}
	p.cmd = nil
}

func (p *local) Complete(ctx context.Context, r fim.Request) (string, error) {
	// Loading must outlive the request: the next keystroke cancels it, and
	// a cancelled load kills llama-server before it is ever ready.
	url, err := p.start(context.WithoutCancel(ctx))
	if err != nil {
		return "", err
	}
	return infill(ctx, url, r)
}

// infill asks a llama-server at url to fill in the middle.
func infill(ctx context.Context, url string, r fim.Request) (string, error) {
	extra := []map[string]string{}
	for _, e := range r.Extra {
		extra = append(extra, map[string]string{"filename": e.Path, "text": e.Text})
	}
	var out struct {
		Content string `json:"content"`
	}
	err := postJSON(ctx, url+"/infill", map[string]any{
		"input_prefix":     r.Prefix,
		"input_suffix":     r.Suffix,
		"input_extra":      extra,
		"n_predict":        128,
		"temperature":      0.2,
		"cache_prompt":     true,
		"t_max_predict_ms": 2500,
		"stop":             []string{"\n\n\n"},
	}, &out)
	if errors.Is(err, io.EOF) {
		err = nil
	}
	return out.Content, err
}
