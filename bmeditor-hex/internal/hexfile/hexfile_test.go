package hexfile

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func file(t *testing.T, data []byte) (string, string) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "a.bin")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return root, path
}

func TestRead_ReadsAPageAndTheSize(t *testing.T) {
	// Arrange
	root, path := file(t, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

	// Act
	mid, err1 := Read(root, path, 3, 4)
	past, err2 := Read(root, path, 20, 4)

	// Assert
	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	if mid.Size != 10 || !bytes.Equal(mid.Data, []byte{3, 4, 5, 6}) {
		t.Errorf("middle = %v (size %d)", mid.Data, mid.Size)
	}
	if len(past.Data) != 0 {
		t.Errorf("past the end = %v, want nothing", past.Data)
	}
}

func TestWrite_OverwritesInPlace(t *testing.T) {
	// Arrange
	root, path := file(t, []byte{0, 1, 2, 3, 4, 5})

	// Act
	err := Write(root, path, []Change{{Offset: 1, Data: []byte{0xAA}}, {Offset: 4, Data: []byte{0xBB, 0xCC}}})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, []byte{0, 0xAA, 2, 3, 0xBB, 0xCC}) {
		t.Errorf("file = %x", got)
	}
}

func TestWrite_RefusesAChangePastTheEndAndWritesNothing(t *testing.T) {
	// Arrange
	root, path := file(t, []byte{0, 1, 2})

	// Act
	err := Write(root, path, []Change{{Offset: 0, Data: []byte{9}}, {Offset: 2, Data: []byte{7, 7}}})

	// Assert
	got, _ := os.ReadFile(path)
	if err == nil || !bytes.Equal(got, []byte{0, 1, 2}) {
		t.Fatalf("err = %v, file = %v; want refused and untouched", err, got)
	}
}

func TestRead_StaysInTheFolder(t *testing.T) {
	// Arrange
	root, _ := file(t, []byte{1})
	outside := filepath.Join(filepath.Dir(root), "secret.bin")
	_ = os.WriteFile(outside, []byte{1}, 0o644)

	// Act
	_, err := Read(root, outside, 0, 1)

	// Assert
	if err == nil {
		t.Fatal("a file outside the open folder was read")
	}
}
