package game

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFind_ReadsTheManifestAtTheTop(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"Golf.ascdiiproj": `{"name": "Golf", "start": "scenes/main.scene.json", "display": {"mode": "window"}}`})

	// Act
	p, err := Find(t.Context(), dir)
	_, errNone := Find(t.Context(), t.TempDir())

	// Assert
	if err != nil || p.Manifest.Name != "Golf" || p.Manifest.Display.Mode != "window" {
		t.Fatalf("got %+v, %v", p, err)
	}
	if !errors.Is(errNone, ErrNoProject) {
		t.Errorf("a folder without a manifest: %v", errNone)
	}
}

func TestRead_ListsAssetsAndWhichObjectsUseAScript(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"scripts/hero.lua":         "function update(self, dt) end",
		"scripts/hero.lua.meta":    `{"format": 1, "guid": "g-hero"}`,
		"scripts/gate.lua":         "",
		"scenes/main.scene.json":   `{"objects": [{"name": "world", "children": [{"name": "hero", "components": [{"kind": "script", "props": {"file": "g-hero", "speed": 6.5}}]}]}]}`,
		"prefabs/door.prefab.json": `{"root": {"name": "door", "components": [{"kind": "script", "props": {"file": "scripts/gate.lua"}}]}}`,
		"build/linux/game.lua":     "",
		"tools/gen/main.go":        "",
		"README.md":                "",
		"textures/grass.png":       "",
		"textures/grass.png.meta":  "",
		"textures/ui/icon.png":     "",
		"models/tree.obj":          "",
		"models/tree.mtl":          "",
		"models/notes.md":          "",
	})

	// Act
	idx, err := Read(t.Context(), dir)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Assets) != 7 {
		t.Fatalf("assets %+v, want two scripts, the scene, the prefab, two textures and the model", idx.Assets)
	}
	if !slices.Equal(idx.AssetOnly, []string{"textures", "textures/ui"}) {
		t.Errorf("asset only folders %v, want textures and textures/ui: models has a note", idx.AssetOnly)
	}
	if u := idx.Uses["scripts/hero.lua"]; len(u) != 1 || u[0].Object != "hero" || u[0].In != "scenes/main.scene.json" || u[0].Values["speed"] != 6.5 {
		t.Errorf("hero.lua used by %+v", u)
	}
	if u := idx.Uses["scripts/gate.lua"]; len(u) != 1 || u[0].Object != "door" {
		t.Errorf("gate.lua used by %+v", u)
	}
}

func TestCheck_PointsAtASyntaxError(t *testing.T) {
	// Arrange
	src := "function update(self, dt)\n  if dt > 1 then\n    log(dt)\nend\n"

	// Act
	ok := Check("scripts/a.lua", "local x = 1\nreturn x\n")
	bad := Check("scripts/b.lua", src)

	// Assert
	if len(ok) != 0 {
		t.Errorf("a valid script: %+v", ok)
	}
	if len(bad) != 1 || bad[0].File != "scripts/b.lua" || bad[0].Line < 4 || bad[0].Message == "" {
		t.Errorf("got %+v, want one error at the end", bad)
	}
}

func TestLogProblem_ReadsAFailedScriptFromTheLog(t *testing.T) {
	// Arrange
	line := "error scripts/door.lua: scripts/door.lua:2: cannot perform add operation between nil and number"

	// Act
	p, ok := LogProblem(line)
	_, other := LogProblem("info mod:welcome: hero starts at 83 88")

	// Assert
	if !ok || p.File != "scripts/door.lua" || p.Line != 2 || p.Message != "cannot perform add operation between nil and number" {
		t.Fatalf("got %+v, %v", p, ok)
	}
	if other {
		t.Error("an info line read as a problem")
	}
}
