package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRead_NoSettingFindsTheVimrcInHome(t *testing.T) {
	// Arrange
	home := t.TempDir()
	write(t, filepath.Join(home, ".vim", "vimrc"), "inoremap jk <Esc>\n")

	// Act
	f, err := Read(context.Background(), Where{Home: home})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if f.Path != filepath.Join(home, ".vim", "vimrc") || f.Text != "inoremap jk <Esc>\n" {
		t.Fatalf("got %+v", f)
	}
}

func TestRead_NoSettingAndNoVimrcIsEmpty(t *testing.T) {
	// Arrange
	home := t.TempDir()

	// Act
	f, err := Read(context.Background(), Where{Home: home})

	// Assert
	if err != nil || f != (File{}) {
		t.Fatalf("got %+v, %v", f, err)
	}
}

func TestRead_SettingWinsOverHome(t *testing.T) {
	// Arrange
	home := t.TempDir()
	write(t, filepath.Join(home, ".vimrc"), "home")
	write(t, filepath.Join(home, "dots", "bm.vim"), "named")

	// Act
	f, err := Read(context.Background(), Where{Setting: "~/dots/bm.vim", Home: home})

	// Assert
	if err != nil || f.Text != "named" {
		t.Fatalf("got %+v, %v", f, err)
	}
}

func TestRead_RelativeSettingIsInTheProject(t *testing.T) {
	// Arrange
	project := t.TempDir()
	write(t, filepath.Join(project, ".vimrc"), "project")

	// Act
	f, err := Read(context.Background(), Where{Setting: ".vimrc", Base: project, Home: t.TempDir()})

	// Assert
	if err != nil || f.Text != "project" {
		t.Fatalf("got %+v, %v", f, err)
	}
}

func TestRead_MissingNamedVimrcIsNotFound(t *testing.T) {
	// Arrange
	home := t.TempDir()
	write(t, filepath.Join(home, ".vimrc"), "home")

	// Act
	_, err := Read(context.Background(), Where{Setting: filepath.Join(home, "gone.vim"), Home: home})

	// Assert
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}
