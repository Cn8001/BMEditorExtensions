package unity

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MetaGUID is the guid a .meta file gives its asset, "" when it has none.
func MetaGUID(metaPath string) string {
	f, err := os.Open(metaPath)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for i := 0; sc.Scan() && i < 8; i++ {
		if v, ok := strings.CutPrefix(sc.Text(), "guid:"); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// NewGUID is a new random GUID as Unity writes one: 32 hex digits.
func NewGUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ignoredName says whether Unity imports nothing of this name: hidden
// (a leading dot), ending with ~, cvs, or a .tmp file.
func ignoredName(name string) bool {
	l := strings.ToLower(name)
	return strings.HasPrefix(name, ".") || strings.HasSuffix(name, "~") || l == "cvs" || strings.HasSuffix(l, ".tmp")
}

// AssetRoots are the folders whose contents Unity imports, so each entry
// in them needs a .meta: Assets/ and the packages of a Unity project, the
// package itself, or, for another folder, the folder (a checkout of part
// of Assets/, say) when it has .meta files at all.
func AssetRoots(w Workspace) []string {
	switch w.Kind {
	case "unity":
		roots := []string{filepath.Join(w.Root, "Assets")}
		for _, p := range w.Packages {
			roots = append(roots, p.Dir)
		}
		return roots
	case "upm":
		return []string{w.Root}
	}
	has := false
	_ = walk(w.Root, 2, func(path string, d fs.DirEntry) {
		if strings.HasSuffix(path, ".meta") {
			has = true
		}
	})
	if has {
		return []string{w.Root}
	}
	return nil
}

// MetaReport is what is wrong with the .meta files under some asset roots.
type MetaReport struct {
	Missing    []string    `json:"missing"`    // files and folders without a .meta
	Orphans    []string    `json:"orphans"`    // .meta files without their asset
	Duplicates []Duplicate `json:"duplicates"` // GUIDs more than one .meta has
	Renames    []Rename    `json:"renames"`    // an orphan .meta that likely belongs to a missing one
	Checked    int         `json:"checked"`
}

type Duplicate struct {
	GUID  string   `json:"guid"`
	Metas []string `json:"metas"`
}

// Rename pairs an orphan .meta with the asset it was likely made for
// before the asset was renamed or moved outside Unity.
type Rename struct {
	Meta   string `json:"meta"`
	To     string `json:"to"`
	Reason string `json:"reason"` // moved, renamed
}

// CheckMeta looks at every entry under roots.
func CheckMeta(roots []string) MetaReport {
	r := MetaReport{Missing: []string{}, Orphans: []string{}, Duplicates: []Duplicate{}, Renames: []Rename{}}
	guids := map[string][]string{}
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if path == root {
				return nil
			}
			name := d.Name()
			if strings.HasSuffix(name, ".meta") {
				asset := strings.TrimSuffix(path, ".meta")
				if _, err := os.Lstat(asset); err != nil || ignoredName(filepath.Base(asset)) {
					r.Orphans = append(r.Orphans, path)
				} else if g := MetaGUID(path); g != "" {
					guids[g] = append(guids[g], path)
				}
				return nil
			}
			if ignoredName(name) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			r.Checked++
			if _, err := os.Lstat(path + ".meta"); err != nil {
				r.Missing = append(r.Missing, path)
			}
			return nil
		})
	}
	for g, metas := range guids {
		if len(metas) > 1 {
			sort.Strings(metas)
			r.Duplicates = append(r.Duplicates, Duplicate{GUID: g, Metas: metas})
		}
	}
	sort.Slice(r.Duplicates, func(i, j int) bool { return r.Duplicates[i].Metas[0] < r.Duplicates[j].Metas[0] })
	r.Renames = pairRenames(r.Orphans, r.Missing)
	return r
}

// pairRenames matches an orphan .meta to a missing one when only one fits:
// the same name in another folder (moved), or else the only orphan and the
// only missing entry of one folder with the same extension (renamed).
func pairRenames(orphans, missing []string) []Rename {
	used := map[string]bool{}
	var out []Rename
	isFolderMeta := func(meta string) bool { return metaHas(meta, "folderAsset: yes") }
	kindOK := func(meta, asset string) bool {
		info, err := os.Stat(asset)
		return err == nil && info.IsDir() == isFolderMeta(meta)
	}
	byName := map[string][]string{}
	for _, m := range missing {
		byName[strings.ToLower(filepath.Base(m))] = append(byName[strings.ToLower(filepath.Base(m))], m)
	}
	for _, o := range orphans {
		asset := strings.TrimSuffix(o, ".meta")
		cands := byName[strings.ToLower(filepath.Base(asset))]
		if len(cands) == 1 && !used[cands[0]] && kindOK(o, cands[0]) {
			out = append(out, Rename{Meta: o, To: cands[0], Reason: "moved"})
			used[cands[0]], used[o] = true, true
		}
	}
	type group struct{ orphans, missing []string }
	dirs := map[string]*group{}
	key := func(p string) string { return filepath.Dir(p) + "|" + strings.ToLower(filepath.Ext(p)) }
	for _, o := range orphans {
		if !used[o] {
			k := key(strings.TrimSuffix(o, ".meta"))
			if dirs[k] == nil {
				dirs[k] = &group{}
			}
			dirs[k].orphans = append(dirs[k].orphans, o)
		}
	}
	for _, m := range missing {
		if !used[m] {
			if g := dirs[key(m)]; g != nil {
				g.missing = append(g.missing, m)
			}
		}
	}
	for _, g := range dirs {
		if len(g.orphans) == 1 && len(g.missing) == 1 && kindOK(g.orphans[0], g.missing[0]) {
			out = append(out, Rename{Meta: g.orphans[0], To: g.missing[0], Reason: "renamed"})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Meta < out[j].Meta })
	return out
}

func metaHas(meta, line string) bool {
	raw, err := os.ReadFile(meta)
	return err == nil && strings.Contains(string(raw), line)
}

// MetaFix is one change to .meta files.
type MetaFix struct {
	Kind string `json:"kind"`         // create, delete, move, regen
	Path string `json:"path"`         // create: the asset; delete, move, regen: the .meta
	To   string `json:"to,omitempty"` // move: the asset the .meta now belongs to
}

// ApplyMeta makes the changes, each only inside one of roots, and returns
// the error of each one that failed, by index.
func ApplyMeta(roots []string, fixes []MetaFix) map[int]string {
	failed := map[int]string{}
	inside := func(p string) bool {
		for _, r := range roots {
			if rel, err := filepath.Rel(r, p); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
				return true
			}
		}
		return false
	}
	for i, f := range fixes {
		var err error
		switch {
		case !inside(f.Path) || (f.To != "" && !inside(f.To)):
			err = errors.New("outside the project's assets")
		case f.Kind == "create":
			err = WriteMeta(f.Path)
		case f.Kind == "delete" && strings.HasSuffix(f.Path, ".meta"):
			if _, serr := os.Lstat(strings.TrimSuffix(f.Path, ".meta")); serr == nil {
				err = errors.New("its asset is there")
			} else {
				err = os.Remove(f.Path)
			}
		case f.Kind == "move" && strings.HasSuffix(f.Path, ".meta"):
			dst := f.To + ".meta"
			if _, serr := os.Lstat(dst); serr == nil {
				err = errors.New(filepath.Base(dst) + " is there already")
			} else {
				err = os.Rename(f.Path, dst)
			}
		case f.Kind == "regen" && strings.HasSuffix(f.Path, ".meta"):
			err = regenGUID(f.Path)
		default:
			err = fmt.Errorf("unknown fix %q", f.Kind)
		}
		if err != nil {
			failed[i] = err.Error()
		}
	}
	return failed
}

func regenGUID(meta string) error {
	raw, err := os.ReadFile(meta)
	if err != nil {
		return err
	}
	lines := strings.Split(string(raw), "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "guid:") {
			lines[i] = "guid: " + NewGUID()
			return os.WriteFile(meta, []byte(strings.Join(lines, "\n")), 0o644)
		}
	}
	return errors.New("no guid line")
}

// WriteMeta writes a new .meta for path with a new GUID, and the importer
// Unity uses for that kind of file; Unity fills in the rest on import. An
// existing .meta is left alone.
func WriteMeta(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	meta := path + ".meta"
	if _, err := os.Lstat(meta); err == nil {
		return errors.New(filepath.Base(meta) + " is there already")
	}
	return os.WriteFile(meta, []byte(MetaText(path, info.IsDir(), NewGUID())), 0o644)
}

// MetaText is the .meta Unity would write for a new asset.
func MetaText(path string, dir bool, guid string) string {
	head := "fileFormatVersion: 2\nguid: " + guid + "\n"
	tail := "  userData: \n  assetBundleName: \n  assetBundleVariant: \n"
	if dir {
		return head + "folderAsset: yes\nDefaultImporter:\n  externalObjects: {}\n" + tail
	}
	switch ext := strings.ToLower(filepath.Ext(path)); {
	case ext == ".cs":
		return head + "MonoImporter:\n  externalObjects: {}\n  serializedVersion: 2\n  defaultReferences: []\n  executionOrder: 0\n  icon: {instanceID: 0}\n" + tail
	case ext == ".asmdef":
		return head + "AssemblyDefinitionImporter:\n  externalObjects: {}\n" + tail
	case ext == ".asmref":
		return head + "AssemblyDefinitionReferenceImporter:\n  externalObjects: {}\n" + tail
	case strings.EqualFold(filepath.Base(path), "package.json"):
		return head + "PackageManifestImporter:\n  externalObjects: {}\n" + tail
	case strings.Contains(".txt.json.md.xml.html.htm.bytes.csv.yaml.yml.fnt", ext) && ext != "":
		return head + "TextScriptImporter:\n  externalObjects: {}\n" + tail
	}
	return head + "DefaultImporter:\n  externalObjects: {}\n" + tail
}

// GUIDIndex finds assets by GUID and GUIDs by asset under some folders.
type GUIDIndex struct {
	byGUID map[string]string
}

// IndexGUIDs reads the .meta files under roots. Library/PackageCache, for
// registry packages, is read only when it is one of roots.
func IndexGUIDs(roots []string) *GUIDIndex {
	ix := &GUIDIndex{byGUID: map[string]string{}}
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() && path != root && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			if !d.IsDir() && strings.HasSuffix(path, ".meta") {
				if g := MetaGUID(path); g != "" {
					if _, ok := ix.byGUID[g]; !ok {
						ix.byGUID[g] = strings.TrimSuffix(path, ".meta")
					}
				}
			}
			return nil
		})
	}
	return ix
}

// Find is the asset a GUID belongs to, "" when unknown.
func (ix *GUIDIndex) Find(guid string) string {
	return ix.byGUID[strings.ToLower(strings.TrimSpace(guid))]
}
