package game

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

type Kind string

const (
	Scene    Kind = "scene"
	Prefab   Kind = "prefab"
	Script   Kind = "script"
	Model    Kind = "model"
	Material Kind = "material"
	Animator Kind = "animator"
	Sprite   Kind = "sprite"
	Texture  Kind = "texture"
	Audio    Kind = "audio"
)

// Classify sorts a file as the engine's asset catalog does.
func Classify(p string) (Kind, bool) {
	l := strings.ToLower(p)
	switch {
	case path.Base(l) == "pack.json":
		return "", false
	case strings.HasSuffix(l, ".prefab.json"):
		return Prefab, true
	case strings.HasSuffix(l, ".scene.json"):
		return Scene, true
	case strings.HasSuffix(l, ".lua"):
		return Script, true
	case strings.HasSuffix(l, ".obj"), strings.HasSuffix(l, ".fbx"):
		return Model, true
	case strings.HasSuffix(l, ".material.json"):
		return Material, true
	case strings.HasSuffix(l, ".animator.json"):
		return Animator, true
	case strings.HasSuffix(l, ".wav"), strings.HasSuffix(l, ".mp3"):
		return Audio, true
	case strings.HasSuffix(l, ".png"), strings.HasSuffix(l, ".jpg"), strings.HasSuffix(l, ".jpeg"), strings.HasSuffix(l, ".gif"):
		return Texture, true
	case strings.HasSuffix(l, ".sprite.json"), strings.HasSuffix(l, ".xp"):
		return Sprite, true
	}
	return "", false
}

type Asset struct {
	Path    string `json:"path"` // slash-separated, from the project's folder
	Kind    Kind   `json:"kind"`
	GUID    string `json:"guid,omitempty"`
	Package string `json:"package,omitempty"` // installed under packages/, read only
}

// Use is a scene or prefab object that has a script, and the values its
// Inspector gave the script's fields there.
type Use struct {
	Object string         `json:"object"`
	In     string         `json:"in"` // the scene or prefab, slash-separated
	Values map[string]any `json:"values,omitempty"`
}

type Index struct {
	Assets []Asset           `json:"assets"`
	Uses   map[string][]Use  `json:"uses"` // by script path
	Errors map[string]string `json:"errors,omitempty"`
	// AssetOnly are the folders that hold nothing but files the code view
	// leaves out, which it leaves out too rather than show them empty.
	AssetOnly []string `json:"assetOnly"`
}

// NotCode says a file is an asset no one edits as text, which a code view
// leaves out, as Rider's does: images, models, sounds, REXPaint art and the
// .meta files beside every asset.
func NotCode(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".obj", ".mtl", ".fbx", ".wav", ".mp3", ".ogg", ".xp", ".meta":
		return true
	}
	return false
}

// SkipDir leaves out what is not the game's content: build output, version
// control, and tools a project keeps beside its assets.
func SkipDir(rel, name string) bool {
	return strings.HasPrefix(name, ".") || name == "node_modules" || rel == "build" || rel == "tools"
}

// Read lists the project's assets and which objects use each script.
func Read(ctx context.Context, dir string) (Index, error) {
	idx := Index{Assets: []Asset{}, Uses: map[string][]Use{}, AssetOnly: []string{}}
	files, code := map[string]bool{}, map[string]bool{} // folders with files, and with code
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // an unreadable folder costs its files, not the list
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel != "." && SkipDir(rel, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		isCode := !NotCode(d.Name())
		for f := path.Dir(rel); f != "."; f = path.Dir(f) {
			files[f] = true
			code[f] = code[f] || isCode
		}
		kind, ok := Classify(rel)
		if !ok {
			return nil
		}
		a := Asset{Path: rel, Kind: kind, GUID: guidOf(p + ".meta")}
		if pkg, ok := strings.CutPrefix(rel, "packages/"); ok {
			a.Package, _, _ = strings.Cut(pkg, "/")
		}
		idx.Assets = append(idx.Assets, a)
		return nil
	})
	if err != nil {
		return idx, fmt.Errorf("list assets: %w", err)
	}
	slices.SortFunc(idx.Assets, func(a, b Asset) int { return strings.Compare(a.Path, b.Path) })
	for f := range files {
		if !code[f] {
			idx.AssetOnly = append(idx.AssetOnly, f)
		}
	}
	slices.Sort(idx.AssetOnly)

	scripts := map[string]string{} // a reference, GUID or path, to the script's path
	for _, a := range idx.Assets {
		if a.Kind == Script {
			scripts[a.Path] = a.Path
			if a.GUID != "" {
				scripts[a.GUID] = a.Path
			}
		}
	}
	for _, a := range idx.Assets {
		if a.Kind != Scene && a.Kind != Prefab {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(a.Path)))
		if err != nil {
			continue
		}
		var doc any
		if err := json.Unmarshal(raw, &doc); err != nil {
			if idx.Errors == nil {
				idx.Errors = map[string]string{}
			}
			idx.Errors[a.Path] = err.Error()
			continue
		}
		usesIn(doc, func(object, ref string, values map[string]any) {
			if s, ok := scripts[ref]; ok {
				idx.Uses[s] = append(idx.Uses[s], Use{Object: object, In: a.Path, Values: values})
			}
		})
	}
	return idx, nil
}

func guidOf(meta string) string {
	raw, err := os.ReadFile(meta)
	if err != nil {
		return ""
	}
	var m struct {
		GUID string `json:"guid"`
	}
	_ = json.Unmarshal(raw, &m)
	return m.GUID
}

// usesIn finds every object of a scene or prefab document with a script
// component, wherever the format nests it: objects, children, a prefab's
// root.
func usesIn(v any, found func(object, ref string, values map[string]any)) {
	switch v := v.(type) {
	case map[string]any:
		if comps, ok := v["components"].([]any); ok {
			name, _ := v["name"].(string)
			for _, c := range comps {
				c, _ := c.(map[string]any)
				if c["kind"] != "script" {
					continue
				}
				props, _ := c["props"].(map[string]any)
				if ref, ok := props["file"].(string); ok && ref != "" {
					values := map[string]any{}
					for k, v := range props {
						if k != "file" {
							values[k] = v
						}
					}
					found(name, ref, values)
				}
			}
		}
		for _, child := range v {
			usesIn(child, found)
		}
	case []any:
		for _, child := range v {
			usesIn(child, found)
		}
	}
}
