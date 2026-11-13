package unity

import (
	"os"
	"path/filepath"
	"testing"
)

func unityProject(t *testing.T) Workspace {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Assets", "Scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Assets", "Scripts.meta"), []byte("guid: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return Workspace{Root: root, Kind: "unity"}
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func TestRenameAsset_TakesItsMetaAlong(t *testing.T) {
	// Arrange
	w := unityProject(t)
	script := filepath.Join(w.Root, "Assets", "Scripts", "Player.cs")
	_ = os.WriteFile(script, []byte("class Player {}"), 0o644)
	_ = os.WriteFile(script+".meta", []byte("guid: abc\n"), 0o644)

	// Act
	to, err := RenameAsset(w, script, "Hero.cs")

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if to != filepath.Join(w.Root, "Assets", "Scripts", "Hero.cs") || !exists(to) || !exists(to+".meta") {
		t.Fatalf("renamed to %s; the script and its .meta should be there", to)
	}
	if exists(script) || exists(script+".meta") {
		t.Error("the old names should be gone")
	}
}

func TestRenameAsset_RefusesANameThatLeavesTheFolder(t *testing.T) {
	// Arrange
	w := unityProject(t)
	script := filepath.Join(w.Root, "Assets", "Scripts", "Player.cs")
	_ = os.WriteFile(script, nil, 0o644)

	// Act
	_, err := RenameAsset(w, script, "../Player.cs")

	// Assert
	if err == nil || !exists(script) {
		t.Fatalf("err = %v; the script should stay where it is", err)
	}
}

func TestDeleteAsset_DeletesAFolderAndItsMeta(t *testing.T) {
	// Arrange
	w := unityProject(t)
	dir := filepath.Join(w.Root, "Assets", "Scripts")
	_ = os.WriteFile(filepath.Join(dir, "Player.cs"), nil, 0o644)

	// Act
	err := DeleteAsset(w, dir)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if exists(dir) || exists(dir+".meta") {
		t.Error("the folder and its .meta should be gone")
	}
	if err := DeleteAsset(w, w.Root); err == nil || !exists(w.Root) {
		t.Errorf("deleting the open folder itself: err = %v, want refused", err)
	}
}

func TestNewFolder_WritesAMetaWhereUnityImports(t *testing.T) {
	// Arrange
	w := unityProject(t)
	scripts := filepath.Join(w.Root, "Assets", "Scripts")

	// Act
	path, err := NewFolder(w, scripts, "AI", true)
	outside, err2 := NewFolder(w, w.Root, "Tools", true)

	// Assert
	if err != nil || err2 != nil {
		t.Fatal(err, err2)
	}
	if !exists(path) || !exists(path+".meta") {
		t.Error("a folder in Assets should get its .meta")
	}
	if !exists(outside) || exists(outside+".meta") {
		t.Error("a folder Unity does not import should get none")
	}
}
