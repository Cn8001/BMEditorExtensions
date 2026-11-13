package unity

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// The Solution Explorer's file operations, done as Unity needs them done
// outside the editor: an asset and its .meta are renamed and deleted
// together, so its GUID, and every reference to it, stays.

func inside(w Workspace, path string) error {
	rel, err := filepath.Rel(w.Root, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s is outside the open folder", path)
	}
	return nil
}

// validName refuses a name that would leave its folder or that Windows
// cannot keep.
func validName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\:*?"<>|`) || strings.TrimRight(name, ". ") != name {
		return fmt.Errorf("%q is not a file name", name)
	}
	return nil
}

// NewFolder makes dir/name, and its .meta when meta is set and Unity
// imports dir.
func NewFolder(w Workspace, dir, name string, meta bool) (string, error) {
	if err := validName(name); err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	if err := inside(w, path); err != nil {
		return "", err
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return "", fmt.Errorf("%s is there already", name)
		}
		return "", err
	}
	if meta && importedFolder(w, dir) {
		if err := WriteMeta(path); err != nil {
			return path, err
		}
	}
	return path, nil
}

// RenameAsset gives a file or folder a new name in the same folder, and its
// .meta the matching one. It returns the new path.
func RenameAsset(w Workspace, path, name string) (string, error) {
	if err := validName(name); err != nil {
		return "", err
	}
	if err := inside(w, path); err != nil {
		return "", err
	}
	to := filepath.Join(filepath.Dir(path), name)
	// A change of case alone is the same file to Windows: that rename is fine.
	if _, err := os.Lstat(to); err == nil && !strings.EqualFold(to, path) {
		return "", fmt.Errorf("%s is there already", name)
	}
	if err := os.Rename(path, to); err != nil {
		return "", err
	}
	if _, err := os.Lstat(path + ".meta"); err == nil {
		if err := os.Rename(path+".meta", to+".meta"); err != nil {
			return to, fmt.Errorf("renamed, but not its .meta: %w", err)
		}
	}
	return to, nil
}

// DeleteAsset deletes a file or a folder with everything in it, and its
// .meta.
func DeleteAsset(w Workspace, path string) error {
	if err := inside(w, path); err != nil {
		return err
	}
	if _, err := os.Lstat(path); err != nil {
		return err
	}
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	if err := os.Remove(path + ".meta"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("deleted, but not its .meta: %w", err)
	}
	return nil
}
