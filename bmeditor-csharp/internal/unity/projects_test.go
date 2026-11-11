package unity

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"bmeditor-csharp/internal/dotnet"
)

// fakeUnity makes an editor's Data folder with a few of its assemblies.
func fakeUnity(t *testing.T) *Install {
	t.Helper()
	data := filepath.Join(t.TempDir(), "2022.3.67f2", "Editor", "Data")
	for _, rel := range []string{
		"Managed/UnityEngine/UnityEngine.CoreModule.dll",
		"Managed/UnityEngine/UnityEditor.CoreModule.dll",
		"UnityReferenceAssemblies/unity-4.8-api/mscorlib.dll",
		"UnityReferenceAssemblies/unity-4.8-api/Facades/netstandard.dll",
	} {
		p := filepath.Join(data, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return &Install{Version: "2022.3.67f2", Data: data}
}

func TestProjects_WritesOneProjectPerAssembly(t *testing.T) {
	// Arrange: the project of the other tests, with scripts.
	root := project(t)
	for rel, text := range map[string]string{
		"Assets/Game/Player.cs":                          "class Player {}",
		"Assets/Core/Rules.cs":                           "class Rules {}",
		"Assets/Loose.cs":                                "class Loose {}",
		"Assets/Tools/Editor/Menu.cs":                    "class Menu {}",
		"Assets/Samples~/Sample.cs":                      "class Hidden {}",
		"Packages/com.test.tools/Runtime/Tool.cs":        "class Tool {}",
		"Library/ScriptAssemblies/Unity.TextMeshPro.dll": "",
	} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w := Detect(root)
	out := filepath.Join(t.TempDir(), "projects")
	u := fakeUnity(t)

	// Act
	g, err := Projects(w, out, u)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Generated{}
	for _, p := range g.Projects {
		got[p.Name] = p
	}
	names := make([]string, 0, len(got))
	for n := range got {
		names = append(names, n)
	}
	slices.Sort(names)
	if want := []string{"Assembly-CSharp", "Assembly-CSharp-Editor", "Core", "Game", "Test.Tools"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("projects = %v, want %v", names, want)
	}
	base := func(paths []string) []string {
		var b []string
		for _, p := range paths {
			b = append(b, filepath.Base(p))
		}
		return b
	}
	if f := base(got["Game"].Files); !reflect.DeepEqual(f, []string{"Moved.cs", "Player.cs"}) {
		t.Errorf("Game's files = %v", f)
	}
	if f := base(got["Assembly-CSharp"].Files); !reflect.DeepEqual(f, []string{"Loose.cs", "New.cs"}) {
		t.Errorf("Assembly-CSharp's files = %v (Samples~ is left out, Editor/ goes to the editor's)", f)
	}
	if f := base(got["Assembly-CSharp-Editor"].Files); !reflect.DeepEqual(f, []string{"Menu.cs"}) {
		t.Errorf("Assembly-CSharp-Editor's files = %v", f)
	}
	if p := got["Game"]; !reflect.DeepEqual(p.Projects, []string{"Core", "Test.Tools"}) || len(p.DLLs) != 1 || filepath.Base(p.DLLs[0]) != "Unity.TextMeshPro.dll" {
		t.Errorf("Game references %v and %v", p.Projects, p.DLLs)
	}
	// Core is not auto-referenced.
	if p := got["Assembly-CSharp"]; !reflect.DeepEqual(p.Projects, []string{"Game", "Test.Tools"}) {
		t.Errorf("Assembly-CSharp references %v", p.Projects)
	}
	if p := got["Assembly-CSharp-Editor"]; !slices.Contains(p.Projects, "Assembly-CSharp") {
		t.Errorf("Assembly-CSharp-Editor references %v", p.Projects)
	}

	// The files are a solution and projects a .NET reader takes.
	s, err := dotnet.Load(g.Solution)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Items) != 5 {
		t.Fatalf("solution items = %d", len(s.Items))
	}
	gp, err := dotnet.ParseProject(got["Game"].Project)
	if err != nil {
		t.Fatal(err)
	}
	if len(gp.Files) != 3 || len(gp.ProjectRefs) != 2 || len(gp.References) != 5 {
		t.Errorf("Game.csproj: %d files, %d project refs, %d refs", len(gp.Files), len(gp.ProjectRefs), len(gp.References))
	}
	text, _ := os.ReadFile(got["Game"].Project)
	for _, want := range []string{"UNITY_2022_3_OR_NEWER", "UNITY_EDITOR", "<NoStdLib>true</NoStdLib>", "<RootNamespace>Golf.Game</RootNamespace>", "UnityEditor.CoreModule.dll"} {
		if !strings.Contains(string(text), want) {
			t.Errorf("Game.csproj lacks %s", want)
		}
	}

	// A second run with nothing changed leaves the files as they were.
	info, _ := os.Stat(got["Game"].Project)
	time.Sleep(20 * time.Millisecond)
	if _, err := Projects(w, out, u); err != nil {
		t.Fatal(err)
	}
	if again, _ := os.Stat(got["Game"].Project); !again.ModTime().Equal(info.ModTime()) {
		t.Error("an unchanged project should not be written again")
	}
}

func TestFindUnity_PicksTheVersionOrItsLine(t *testing.T) {
	installs := []Install{{Version: "6000.5.8f1"}, {Version: "2022.3.67f2"}, {Version: "2022.3.9f1"}}
	cases := map[string]string{
		"2022.3.67f2": "2022.3.67f2",
		"2022.3":      "2022.3.67f2",
		"2022.3.0f1":  "2022.3.67f2",
		"2021.3":      "6000.5.8f1",
		"":            "6000.5.8f1",
	}
	for want, got := range cases {
		if in, _ := FindUnity(installs, want); in.Version != got {
			t.Errorf("FindUnity(%q) = %s, want %s", want, in.Version, got)
		}
	}
	if compareVersions("2022.3.67f2", "2022.3.9f1") <= 0 {
		t.Error("67 comes after 9")
	}
}

func TestDefines(t *testing.T) {
	d := Defines("2022.3.67f2")
	for _, want := range []string{"UNITY_2022_3_67", "UNITY_2022_3", "UNITY_2022", "UNITY_5_3_OR_NEWER", "UNITY_2021_3_OR_NEWER", "UNITY_2022_3_OR_NEWER", "UNITY_EDITOR"} {
		if !slices.Contains(d, want) {
			t.Errorf("defines lack %s", want)
		}
	}
	if slices.Contains(d, "UNITY_2023_1_OR_NEWER") || slices.Contains(d, "UNITY_6000_0_OR_NEWER") {
		t.Error("defines name a later line")
	}
	if !slices.Contains(Defines("6000.5.8f1"), "UNITY_6000_5_OR_NEWER") || !slices.Contains(Defines("6000.5.8f1"), "UNITY_2023_4_OR_NEWER") {
		t.Error("Unity 6 defines")
	}
}
