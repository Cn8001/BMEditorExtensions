// Package changes says what changed in the code, in the terms the code is
// made of: which declarations were added, changed or removed since a
// baseline (HEAD, or the moment an agent's step began), grouped into the
// units that change together, each with where it plugs into the rest,
// what it relies on, what it can break and which tests reach it. It reads
// the code before and after; it does not paraphrase the diff.
package changes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"bmeditor-mentor/internal/explain"
	"bmeditor-mentor/internal/gitx"
	"bmeditor-mentor/internal/golang"
	"bmeditor-mentor/internal/index"
)

// FileBase is one file as it was at the baseline: its content's hash,
// empty when it did not exist, and its declarations.
type FileBase struct {
	Hash  string        `json:"hash"`
	Decls []golang.Decl `json:"decls"`
}

// Baseline is the code to compare with. Files holds the files it knows;
// with All, a file it does not hold did not exist then, else it is taken
// as unchanged.
type Baseline struct {
	Label string              `json:"label"`
	At    time.Time           `json:"at"`
	Head  gitx.Head           `json:"head"`
	All   bool                `json:"all"`
	Files map[string]FileBase `json:"files"`
}

func contentHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:12])
}

// Snapshot is the code as it is now: every Go file of the index, with its
// declarations. An agent's step is compared with the snapshot taken when
// it began.
func Snapshot(ctx context.Context, ix *index.Index, label string) Baseline {
	b := Baseline{Label: label, At: time.Now(), Head: gitx.HeadOf(ctx, ix.Root), All: true, Files: map[string]FileBase{}}
	for file := range ix.ByFile {
		src, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		decls, _ := golang.Decls(pkgPathFor(ix, file, src), src)
		b.Files[file] = FileBase{Hash: contentHash(src), Decls: decls}
	}
	return b
}

// FromHead is HEAD as the baseline: the Go files that differ from it, as
// they were there, and every file git sees changed.
func FromHead(ctx context.Context, ix *index.Index) (Baseline, []gitx.FileChange, error) {
	head := gitx.HeadOf(ctx, ix.Root)
	b := Baseline{Label: "HEAD", At: time.Now(), Head: head, Files: map[string]FileBase{}}
	if head.Commit == "" {
		return b, nil, &NoGitError{}
	}
	files, err := gitx.Changes(ctx, ix.Root)
	if err != nil {
		return b, nil, err
	}
	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".go") {
			continue
		}
		fb := FileBase{}
		if f.Status != "A" {
			if src, err := gitx.Show(ctx, ix.Root, "HEAD", f.Path); err == nil {
				fb.Hash = contentHash(src)
				fb.Decls, _ = golang.Decls(pkgPathFor(ix, f.Path, src), src)
			}
		}
		b.Files[f.Path] = fb
	}
	return b, files, nil
}

// NoGitError says the folder is not in a git repository, so there is no
// HEAD to compare with.
type NoGitError struct{}

func (*NoGitError) Error() string {
	return "This folder is not in a git repository: changes are shown once an agent's step begins."
}

// pkgPathFor is the import path of the package a file is in, as the index
// knows it: from the file's symbols, else from its folder's package, with
// _test for an external test package.
func pkgPathFor(ix *index.Index, file string, src []byte) string {
	if syms := ix.ByFile[file]; len(syms) > 0 {
		return syms[0].Pkg
	}
	name := ""
	if f, err := parser.ParseFile(token.NewFileSet(), "", src, parser.PackageClauseOnly); err == nil {
		name = f.Name.Name
	}
	dir := filepath.Dir(file)
	for path, p := range ix.Packages {
		if p.Dir == dir && !strings.HasSuffix(path, "_test") {
			if strings.HasSuffix(name, "_test") {
				return path + "_test"
			}
			return path
		}
	}
	// A package the index does not know yet: its path from its module.
	best := index.Module{}
	for _, m := range ix.Modules {
		if (dir == m.Dir || strings.HasPrefix(dir, m.Dir+string(filepath.Separator))) && len(m.Dir) > len(best.Dir) {
			best = m
		}
	}
	if best.Dir == "" {
		return name
	}
	rel, _ := filepath.Rel(best.Dir, dir)
	path := best.Path
	if rel != "." {
		path += "/" + filepath.ToSlash(rel)
	}
	if strings.HasSuffix(name, "_test") {
		path += "_test"
	}
	return path
}

// Change is one declaration added, changed or removed.
type Change struct {
	explain.Ref
	Change       string `json:"change"` // added, modified, removed
	ShapeChanged bool   `json:"shapeChanged,omitempty"`
	OldShape     string `json:"oldShape,omitempty"`
	Shape        string `json:"shape,omitempty"`
	Review       string `json:"review,omitempty"` // unseen, reviewed or stale, filled in by the caller
	// Start and End are the lines it spans now; none when it was removed.
	Start int `json:"start,omitempty"`
	End   int `json:"end,omitempty"`
}

// Diff compares the code now, as the index and the files on disk have
// it, with base: the declarations of the files that differ.
func Diff(ix *index.Index, base Baseline) []Change {
	before := map[string]golang.Decl{}
	after := map[string]golang.Decl{}
	afterFile := map[string]string{}
	beforeFile := map[string]string{}
	files := map[string]bool{}
	for f := range base.Files {
		files[f] = true
	}
	if base.All {
		for f := range ix.ByFile {
			files[f] = true
		}
	}
	for f := range files {
		fb, known := base.Files[f]
		src, err := os.ReadFile(f)
		now := ""
		if err == nil {
			now = contentHash(src)
		}
		if known && fb.Hash == now {
			continue // unchanged
		}
		for _, d := range fb.Decls {
			before[d.Key] = d
			beforeFile[d.Key] = f
		}
		if err == nil {
			decls, _ := golang.Decls(pkgPathFor(ix, f, src), src)
			for _, d := range decls {
				after[d.Key] = d
				afterFile[d.Key] = f
			}
		}
	}
	var out []Change
	for k, d := range after {
		ref := refOf(ix, k, d, afterFile[k])
		old, was := before[k]
		switch {
		case !was:
			// A file that did not change cannot have lost it, so it is new.
			out = append(out, Change{Ref: ref, Change: "added", Shape: d.Shape, Start: d.Start, End: d.End})
		case old.Hash != d.Hash:
			out = append(out, Change{Ref: ref, Change: "modified", ShapeChanged: old.Shape != d.Shape && (old.Shape != "" || d.Shape != ""),
				OldShape: old.Shape, Shape: d.Shape, Start: d.Start, End: d.End})
		}
	}
	for k, d := range before {
		if _, ok := after[k]; ok {
			continue
		}
		out = append(out, Change{Ref: refOf(ix, k, d, beforeFile[k]), Change: "removed", OldShape: d.Shape})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}

func refOf(ix *index.Index, key string, d golang.Decl, file string) explain.Ref {
	if s := ix.Symbols[key]; s != nil && s.File == file {
		return explain.RefOf(s)
	}
	pkg := index.PackageOf(key)
	name := pkg
	if p := ix.Packages[pkg]; p != nil {
		name = p.Name
	} else if i := strings.LastIndex(pkg, "/"); i >= 0 {
		name = pkg[i+1:]
	}
	return explain.Ref{Key: key, Name: d.Name, Kind: string(d.Kind), Recv: d.Recv, Pkg: pkg, PkgName: name, File: file, Line: d.Line, Col: 1,
		Test: strings.HasSuffix(file, "_test.go")}
}

// FileChanges is one file that differs from the baseline, with the
// declarations that changed in it.
type FileChanges struct {
	gitx.FileChange
	Changes []Change `json:"changes"`
}

// ByFile lists the files keep holds, each with its changed declarations
// in the order of the file, by path.
func ByFile(list []Change, files []gitx.FileChange, keep func(path string) bool) []FileChanges {
	out := []FileChanges{}
	at := map[string]int{}
	for _, f := range files {
		if keep(f.Path) {
			at[f.Path] = len(out)
			out = append(out, FileChanges{FileChange: f, Changes: []Change{}})
		}
	}
	for _, c := range list {
		if i, ok := at[c.File]; ok {
			out[i].Changes = append(out[i].Changes, c)
		}
	}
	for i := range out {
		sort.SliceStable(out[i].Changes, func(a, b int) bool { return out[i].Changes[a].Line < out[i].Changes[b].Line })
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Path < out[b].Path })
	return out
}
