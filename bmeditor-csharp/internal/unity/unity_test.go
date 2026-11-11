package unity

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const (
	gGame  = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	gTools = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	gCore  = "cccccccccccccccccccccccccccccccc"
	gDup   = "dddddddddddddddddddddddddddddddd"
)

// project makes a small Unity project: Game and Core reference each other,
// Game references the Tools package by GUID, and the .meta files have each
// kind of problem CheckMeta finds.
func project(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"ProjectSettings/ProjectVersion.txt":                     "m_EditorVersion: 2022.3.67f2\nm_EditorVersionWithRevision: 2022.3.67f2 (abc)\n",
		"ProjectSettings/EditorSettings.asset":                   "EditorSettings:\n  m_ProjectGenerationRootNamespace: Golf\n",
		"Assets/Game.meta":                                       MetaText("", true, "e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1"),
		"Assets/Game/Game.asmdef":                                `{"name":"Game","rootNamespace":"Golf.Game","references":["Core","GUID:` + gTools + `","Unity.TextMeshPro","GUID:0123456789abcdef0123456789abcdef"]}`,
		"Assets/Game/Game.asmdef.meta":                           "fileFormatVersion: 2\nguid: " + gGame + "\n",
		"Assets/Game/Sub.meta":                                   MetaText("", true, "e2e2e2e2e2e2e2e2e2e2e2e2e2e2e2e2"),
		"Assets/Game/Sub/x.txt":                                  "",
		"Assets/Game/Sub/x.txt.meta":                             "fileFormatVersion: 2\nguid: e3e3e3e3e3e3e3e3e3e3e3e3e3e3e3e3\n",
		"Assets/Core/Core.asmdef":                                "\ufeff" + `{"name":"Core","references":["Game"],"autoReferenced":false}`,
		"Assets/Core/Core.asmdef.meta":                           "fileFormatVersion: 2\nguid: " + gCore + "\n",
		"Assets/Old.cs.meta":                                     MetaText("Old.cs", false, "f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1"),
		"Assets/New.cs":                                          "",
		"Assets/Moved.cs.meta":                                   MetaText("Moved.cs", false, "f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2"),
		"Assets/Game/Moved.cs":                                   "",
		"Assets/Dup1.txt":                                        "",
		"Assets/Dup1.txt.meta":                                   "fileFormatVersion: 2\nguid: " + gDup + "\n",
		"Assets/Dup2.txt":                                        "",
		"Assets/Dup2.txt.meta":                                   "fileFormatVersion: 2\nguid: " + gDup + "\n",
		"Assets/.hidden":                                         "",
		"Assets/Samples~/Sample.cs":                              "",
		"Assets/Samples~/Hidden.asmdef":                          `{"name":"Hidden"}`,
		"Packages/manifest.json":                                 `{"dependencies":{"com.unity.textmeshpro":"3.0.6"}}`,
		"Packages/com.test.tools/package.json":                   `{"name":"com.test.tools","displayName":"Tools","version":"1.0.0"}`,
		"Packages/com.test.tools/package.json.meta":              "fileFormatVersion: 2\nguid: f3f3f3f3f3f3f3f3f3f3f3f3f3f3f3f3\n",
		"Packages/com.test.tools/Runtime.meta":                   MetaText("", true, "f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4"),
		"Packages/com.test.tools/Runtime/Tools.asmdef":           `{"name":"Test.Tools","rootNamespace":"Test.Tools"}`,
		"Packages/com.test.tools/Runtime/Tools.asmdef.meta":      "fileFormatVersion: 2\nguid: " + gTools + "\n",
		"Library/PackageCache/com.unity.x@1/X.asmdef":            `{"name":"Unity.X"}`,
		"Library/PackageCache/com.unity.x@1/Runtime/ignored.txt": "",
	}
	for rel, text := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "Assets", "Core"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func rel(t *testing.T, root string, paths []string) []string {
	t.Helper()
	out := []string{}
	for _, p := range paths {
		r, err := filepath.Rel(root, p)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, filepath.ToSlash(r))
	}
	sort.Strings(out)
	return out
}

func TestDetect(t *testing.T) {
	root := project(t)
	w := Detect(root)
	if w.Kind != "unity" || w.UnityVersion != "2022.3.67f2" || w.RootNamespace != "Golf" {
		t.Errorf("workspace = %+v", w)
	}
	if len(w.Packages) != 1 || w.Packages[0].Name != "com.test.tools" {
		t.Errorf("packages = %+v, want com.test.tools", w.Packages)
	}
	if !IsUPMName("com.company.thing") || IsUPMName("MyPackage") || IsUPMName("thing") {
		t.Error("IsUPMName")
	}
}

func TestScanAssemblies(t *testing.T) {
	root := project(t)
	as := ScanAssemblies(root)
	var names []string
	for _, a := range as.Assemblies {
		names = append(names, a.Name)
	}
	// Samples~ is left out; Library/PackageCache comes last, external.
	if want := []string{"Core", "Game", "Test.Tools", "Unity.X"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("assemblies = %v, want %v", names, want)
	}
	game := as.Assemblies[1]
	var status []string
	for _, r := range game.References {
		status = append(status, r.Name+":"+r.Status)
	}
	want := []string{"Core:ok", "Test.Tools:ok", "Unity.TextMeshPro:external", ":unresolved"}
	if !reflect.DeepEqual(status, want) {
		t.Errorf("Game's references = %v, want %v", status, want)
	}
	if game.GUID != gGame || game.RootNamespace != "Golf.Game" || !game.AutoReferenced {
		t.Errorf("Game = %+v", game)
	}
	if as.Assemblies[0].AutoReferenced || as.Assemblies[0].Error != "" {
		t.Errorf("Core (with a BOM, autoReferenced false) = %+v", as.Assemblies[0])
	}
	if !as.Assemblies[3].External {
		t.Error("Unity.X should be external")
	}
	if want := [][]string{{"Core", "Game"}}; !reflect.DeepEqual(as.Cycles, want) {
		t.Errorf("cycles = %v, want %v", as.Cycles, want)
	}
	if len(as.Edges) != 3 {
		t.Errorf("edges = %v, want Game→Core, Game→Tools, Core→Game", as.Edges)
	}
}

func TestCheckMeta(t *testing.T) {
	root := project(t)
	r := CheckMeta(AssetRoots(Detect(root)))
	if got, want := rel(t, root, r.Missing), []string{"Assets/Core", "Assets/Game/Moved.cs", "Assets/New.cs"}; !reflect.DeepEqual(got, want) {
		t.Errorf("missing = %v, want %v", got, want)
	}
	if got, want := rel(t, root, r.Orphans), []string{"Assets/Moved.cs.meta", "Assets/Old.cs.meta"}; !reflect.DeepEqual(got, want) {
		t.Errorf("orphans = %v, want %v", got, want)
	}
	if len(r.Duplicates) != 1 || r.Duplicates[0].GUID != gDup || len(r.Duplicates[0].Metas) != 2 {
		t.Errorf("duplicates = %+v", r.Duplicates)
	}
	var pairs []string
	for _, p := range r.Renames {
		pairs = append(pairs, p.Reason+" "+rel(t, root, []string{p.Meta})[0]+" → "+rel(t, root, []string{p.To})[0])
	}
	want := []string{"moved Assets/Moved.cs.meta → Assets/Game/Moved.cs", "renamed Assets/Old.cs.meta → Assets/New.cs"}
	if !reflect.DeepEqual(pairs, want) {
		t.Errorf("renames = %v, want %v", pairs, want)
	}
}

func TestApplyMeta(t *testing.T) {
	root := project(t)
	roots := AssetRoots(Detect(root))
	assets := filepath.Join(root, "Assets")
	failed := ApplyMeta(roots, []MetaFix{
		{Kind: "move", Path: filepath.Join(assets, "Old.cs.meta"), To: filepath.Join(assets, "New.cs")},
		{Kind: "create", Path: filepath.Join(assets, "Core")},
		{Kind: "delete", Path: filepath.Join(assets, "Moved.cs.meta")},
		{Kind: "regen", Path: filepath.Join(assets, "Dup2.txt.meta")},
		{Kind: "delete", Path: filepath.Join(assets, "Dup1.txt.meta")},                // its asset is there
		{Kind: "create", Path: filepath.Join(root, "ProjectSettings", "Thing.asset")}, // not an asset folder
	})
	if len(failed) != 2 || failed[4] == "" || failed[5] == "" {
		t.Errorf("failed = %v, want the last two only", failed)
	}
	if MetaGUID(filepath.Join(assets, "New.cs.meta")) != "f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1" {
		t.Error("the moved .meta should keep its GUID")
	}
	if !metaHas(filepath.Join(assets, "Core.meta"), "folderAsset: yes") {
		t.Error("a folder's new .meta should say folderAsset")
	}
	if g := MetaGUID(filepath.Join(assets, "Dup2.txt.meta")); g == gDup || len(g) != 32 {
		t.Errorf("regen GUID = %q", g)
	}
	r := CheckMeta(roots)
	if len(r.Duplicates) != 0 || len(r.Orphans) != 0 || len(r.Missing) != 1 {
		t.Errorf("after the fixes: %+v, want only Game/Moved.cs missing", r)
	}
}

func TestGUIDIndex(t *testing.T) {
	root := project(t)
	ix := IndexGUIDs([]string{root})
	if got := ix.Find(strings.ToUpper(gTools)); filepath.Base(got) != "Tools.asmdef" {
		t.Errorf("Find(tools) = %q", got)
	}
	if ix.Find("ffff") != "" {
		t.Error("an unknown GUID should find nothing")
	}
}

func TestEditorLog(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(t.TempDir(), "Editor.log")
	write := func(s string, appendTo bool) {
		flag := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
		if appendTo {
			flag = os.O_CREATE | os.O_WRONLY | os.O_APPEND
		}
		f, err := os.OpenFile(path, flag, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if _, err := f.WriteString(s); err != nil {
			t.Fatal(err)
		}
	}
	write("COMMAND LINE ARGUMENTS:\r\nUnity.exe\r\n-projectpath\r\n"+root+"\r\n-useHub\r\n"+
		"Assets\\Game\\A.cs(10,5): error CS1002: ; expected\r\n"+
		"Assets\\Game\\A.cs(10,5): error CS1002: ; expected\r\n"+
		"Assets\\B.cs(3,1): warning CS0168: The variable 'e' is declared but never used\r\n"+
		"Assets\\B.cs(4,1): warning CS0219: half a li", false)
	l := &EditorLog{Path: path, Root: root}
	if _, changed, err := l.Poll(); err != nil || !changed {
		t.Fatalf("first poll: changed %v, %v", changed, err)
	}
	msgs := l.Messages()
	if len(msgs) != 2 || msgs[0].Severity != "error" || msgs[0].Line != 10 || msgs[0].Col != 5 || msgs[0].Code != "CS1002" {
		t.Fatalf("messages = %+v, want the error once and a warning", msgs)
	}
	if want := filepath.Join(root, "Assets", "Game", "A.cs"); msgs[0].File != want {
		t.Errorf("file = %q, want %q", msgs[0].File, want)
	}
	write("ne\r\n", true)
	if _, _, err := l.Poll(); err != nil || len(l.Messages()) != 3 {
		t.Errorf("a line completed later should count: %+v", l.Messages())
	}
	write("[ScriptCompilation] Requested script compilation because: changes\r\n", true)
	if _, changed, _ := l.Poll(); !changed || len(l.Messages()) != 0 {
		t.Errorf("a new compilation should clear the messages: %+v", l.Messages())
	}
	write("-projectpath\nC:\\Other\\Project\nAssets\\C.cs(1,1): error CS0001: x\n", false)
	if _, changed, _ := l.Poll(); !changed || l.Matches() || len(l.Messages()) != 0 {
		t.Errorf("another project's log: matches %v, messages %+v", l.Matches(), l.Messages())
	}
}

func TestScriptText(t *testing.T) {
	got, err := ScriptText("monobehaviour", "Mover", "Golf.Game")
	if err != nil {
		t.Fatal(err)
	}
	want := "using UnityEngine;\n\nnamespace Golf.Game\n{\n    public class Mover : MonoBehaviour\n    {\n        void Start()\n        {\n        }\n\n        void Update()\n        {\n        }\n    }\n}\n"
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	for _, k := range ScriptKinds {
		if _, err := ScriptText(k.ID, "Thing", ""); err != nil {
			t.Errorf("%s: %v", k.ID, err)
		}
	}
}

func TestNewScript(t *testing.T) {
	root := project(t)
	w := Detect(root)
	path, err := NewScript(w, filepath.Join(root, "Assets", "Game", "Sub"), "Ball", "class", true)
	if err != nil {
		t.Fatal(err)
	}
	text, _ := os.ReadFile(path)
	if !strings.Contains(string(text), "namespace Golf.Game") {
		t.Errorf("namespace should come from Game.asmdef:\n%s", text)
	}
	if MetaGUID(path+".meta") == "" {
		t.Error("a script in an imported folder should get a .meta")
	}
	if _, err := NewScript(w, filepath.Join(root, "Assets", "Game", "Sub"), "Ball", "class", true); err == nil {
		t.Error("an existing script should not be overwritten")
	}
	if _, err := NewScript(w, root, "1Bad", "class", true); err == nil {
		t.Error("a name that is not an identifier should be refused")
	}
	outside, err := NewScript(w, filepath.Join(root, "ProjectSettings"), "Plain", "class", true)
	if err != nil {
		t.Fatal(err)
	}
	if text, _ := os.ReadFile(outside); !strings.Contains(string(text), "namespace Golf") {
		t.Errorf("without an asmdef the project's namespace is used:\n%s", text)
	}
	if _, err := os.Stat(outside + ".meta"); err == nil {
		t.Error("ProjectSettings/ is not imported: no .meta")
	}
}
