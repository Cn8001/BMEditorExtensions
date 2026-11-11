package dotnet

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func rels(p *Project) []string {
	var out []string
	for _, f := range p.Files {
		out = append(out, f.Kind+":"+f.Rel)
	}
	return out
}

func TestLoad_UnitySolution(t *testing.T) {
	s, err := Load(filepath.Join("testdata", "unity", "unity.sln"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "unity" || len(s.Items) != 2 {
		t.Fatalf("solution %q with %d top items, want unity with 2", s.Name, len(s.Items))
	}
	folder, game := s.Items[0], s.Items[1]
	if folder.Kind != "folder" || folder.Name != "Packages" || len(folder.Children) != 1 || folder.Children[0].Name != "Game.Player" {
		t.Errorf("folder = %+v, want Packages holding Game.Player", folder)
	}
	if len(folder.Files) != 1 || filepath.Base(folder.Files[0]) != "README.md" {
		t.Errorf("solution items = %v, want README.md", folder.Files)
	}
	if game.Project == nil {
		t.Fatalf("Assembly-CSharp not read: %s", game.Error)
	}
	p := game.Project
	want := []string{"compile:Assets/Scripts/Game.cs", "compile:Assets/Scripts/Player/Mover.cs", "none:Assets/Scripts/Readme.txt"}
	if got := rels(p); !reflect.DeepEqual(got, want) {
		t.Errorf("files = %v, want %v", got, want)
	}
	if p.TargetFramework != "v4.7.1" || p.SDK != "" {
		t.Errorf("framework %q sdk %q, want v4.7.1 and no SDK", p.TargetFramework, p.SDK)
	}
	if len(p.References) != 1 || p.References[0].Name != "UnityEngine" || p.References[0].Path == "" {
		t.Errorf("references = %+v", p.References)
	}
	if len(p.ProjectRefs) != 1 || filepath.Base(p.ProjectRefs[0]) != "Game.Player.csproj" {
		t.Errorf("project references = %v", p.ProjectRefs)
	}
}

func TestLoad_SlnxAndSDKProjects(t *testing.T) {
	s, err := Load(filepath.Join("testdata", "sdk", "all.slnx"))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Items) != 2 || s.Items[0].Name != "src" || s.Items[1].Name != "Missing" {
		t.Fatalf("items = %+v, want src then Missing", s.Items)
	}
	if s.Items[1].Error == "" {
		t.Error("a missing project should carry an error")
	}
	src := s.Items[0]
	if len(src.Children) != 2 || src.Children[0].Name != "libs" || src.Children[1].Name != "App" {
		t.Fatalf("src holds %+v, want libs then App", src.Children)
	}
	app := src.Children[1].Project
	if app == nil {
		t.Fatalf("App not read: %s", src.Children[1].Error)
	}
	// Generated/ is removed, bin/ skipped, Nested/ is another project's.
	if got, want := rels(app), []string{"compile:Program.cs"}; !reflect.DeepEqual(got, want) {
		t.Errorf("files = %v, want %v", got, want)
	}
	if app.SDK != "Microsoft.NET.Sdk" || app.TargetFramework != "net8.0" || app.OutputType != "Exe" {
		t.Errorf("sdk %q framework %q output %q", app.SDK, app.TargetFramework, app.OutputType)
	}
	wantPkgs := []Reference{{Name: "Newtonsoft.Json", Version: "13.0.3"}, {Name: "NUnit", Version: "4.1.0"}}
	if !reflect.DeepEqual(app.PackageRefs, wantPkgs) {
		t.Errorf("packages = %+v, want %+v", app.PackageRefs, wantPkgs)
	}
	lib := src.Children[0].Children[0].Project
	if lib == nil || !reflect.DeepEqual(rels(lib), []string{"compile:Thing.cs"}) {
		t.Errorf("Lib = %+v", lib)
	}
}

func TestFind_PrefersTheFolderNamedSolution(t *testing.T) {
	got := Find(filepath.Join("testdata", "unity"))
	if len(got) != 1 || filepath.Base(got[0]) != "unity.sln" {
		t.Errorf("Find = %v", got)
	}
}

func TestGlobRegexp(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"Generated/**", "Generated/a/b.cs", true},
		{"**/*.cs", "a.cs", true},
		{"**/*.cs", "x/y/a.cs", true},
		{"src/*.cs", "src/x/a.cs", false},
		{`src\?.cs`, "src/a.cs", true},
	}
	for _, c := range cases {
		if got := globRegexp(c.pattern).MatchString(c.path); got != c.want {
			t.Errorf("%q ~ %q = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

func TestCandidates_PutTheProjectsChoiceFirst(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"B.sln", filepath.Base(root) + ".sln", "Tool.csproj", "Sub/Other.sln"} {
		p := filepath.Join(root, filepath.FromSlash(f))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, nil, 0o644)
	}
	_ = os.WriteFile(filepath.Join(root, "x.bmproj"), []byte(`{"name":"x","kind":"csharp","settings":{"csharp.solution":"Sub/Other.sln"}}`), 0o644)

	pref := ProjectSetting(root, "csharp.solution")
	got := []string{}
	for _, c := range Candidates(root, pref) {
		r, _ := filepath.Rel(root, c)
		got = append(got, filepath.ToSlash(r))
	}

	want := []string{"Sub/Other.sln", filepath.Base(root) + ".sln", "B.sln", "Tool.csproj"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("candidates = %v, want %v", got, want)
	}
	if Candidates(root, "missing.sln")[0] != filepath.Join(root, filepath.Base(root)+".sln") {
		t.Error("a missing choice is passed over")
	}
}
