package unity

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// A folder Unity has not written a solution for, a UPM package on its own
// above all, gives a C# language server nothing to work with: no project,
// no references, so no types and no completion. Projects writes the
// projects Unity would, from the assembly definitions, into a folder of
// its own (never the user's): one per .asmdef, referencing each other and
// the Unity Editor's assemblies, as Unity's own generated .csproj files do.

// Install is an installed Unity Editor.
type Install struct {
	Version string `json:"version"`
	// Data holds Managed/ and UnityReferenceAssemblies/: Editor/Data on
	// Windows and Linux, Unity.app/Contents on macOS.
	Data string `json:"data"`
}

// Installs lists the Unity Editors installed through Unity Hub, newest
// first. BMEDITOR_UNITY_EDITOR names another one's Data folder.
func Installs() []Install {
	var out []Install
	if d := os.Getenv("BMEDITOR_UNITY_EDITOR"); d != "" {
		out = append(out, Install{Version: "custom", Data: d})
	}
	seen := map[string]bool{}
	for _, hub := range hubFolders() {
		entries, _ := os.ReadDir(hub)
		for _, e := range entries {
			data := editorData(filepath.Join(hub, e.Name()))
			if !e.IsDir() || seen[data] || !isDir(filepath.Join(data, "Managed", "UnityEngine")) {
				continue
			}
			seen[data] = true
			out = append(out, Install{Version: e.Name(), Data: data})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return compareVersions(out[i].Version, out[j].Version) > 0 })
	return out
}

// hubFolders are where Unity Hub puts editors: its default folder and the
// one its settings name (secondaryInstallPath.json).
func hubFolders() []string {
	var dirs []string
	home, _ := os.UserHomeDir()
	config, _ := os.UserConfigDir()
	switch runtime.GOOS {
	case "windows":
		for _, pf := range []string{os.Getenv("ProgramFiles"), `C:\Program Files`} {
			if pf != "" {
				dirs = append(dirs, filepath.Join(pf, "Unity", "Hub", "Editor"))
			}
		}
	case "darwin":
		dirs = append(dirs, "/Applications/Unity/Hub/Editor")
	default:
		dirs = append(dirs, filepath.Join(home, "Unity", "Hub", "Editor"))
	}
	if raw, err := os.ReadFile(filepath.Join(config, "UnityHub", "secondaryInstallPath.json")); err == nil {
		var p string
		if json.Unmarshal(raw, &p) == nil && p != "" {
			dirs = append(dirs, p)
		}
	}
	return dirs
}

func editorData(dir string) string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(dir, "Unity.app", "Contents")
	}
	return filepath.Join(dir, "Editor", "Data")
}

// FindUnity is the editor for a version: the exact one (2022.3.67f2), else
// the newest of its line (2022.3), else the newest installed.
func FindUnity(installs []Install, want string) (Install, bool) {
	if len(installs) == 0 {
		return Install{}, false
	}
	for _, in := range installs {
		if in.Version == want {
			return in, true
		}
	}
	if parts := strings.SplitN(want, ".", 3); len(parts) >= 2 {
		line := parts[0] + "." + parts[1] + "."
		for _, in := range installs {
			if strings.HasPrefix(in.Version, line) {
				return in, true
			}
		}
	}
	return installs[0], true
}

var versionPart = regexp.MustCompile(`\d+`)

// compareVersions orders Unity versions: 2022.3.67f2 after 2022.3.9f1.
func compareVersions(a, b string) int {
	pa, pb := versionPart.FindAllString(a, -1), versionPart.FindAllString(b, -1)
	for i := 0; i < len(pa) && i < len(pb); i++ {
		x, _ := strconv.Atoi(pa[i])
		y, _ := strconv.Atoi(pb[i])
		if x != y {
			return x - y
		}
	}
	return len(pa) - len(pb)
}

// WantedVersion is the Unity version a folder is for: a project's
// ProjectVersion.txt, or a package's "unity" and "unityRelease".
func WantedVersion(w Workspace) string {
	if w.UnityVersion != "" {
		return w.UnityVersion
	}
	for _, p := range w.Packages {
		raw, err := os.ReadFile(p.Manifest)
		if err != nil {
			continue
		}
		var m struct{ Unity, UnityRelease string }
		if json.Unmarshal(trimBOM(raw), &m) == nil && m.Unity != "" {
			if m.UnityRelease != "" {
				return m.Unity + "." + m.UnityRelease
			}
			return m.Unity
		}
	}
	return ""
}

// Defines are the symbols Unity defines in the editor for a version.
func Defines(version string) []string {
	out := []string{"UNITY_EDITOR", "UNITY_EDITOR_64", "UNITY_64", "PLATFORM_ARCH_64", "ENABLE_MONO", "NET_4_6", "NET_UNITY_4_8", "CSHARP_7_3_OR_NEWER", "UNITY_INCLUDE_TESTS"}
	switch runtime.GOOS {
	case "windows":
		out = append(out, "UNITY_EDITOR_WIN", "UNITY_STANDALONE_WIN", "UNITY_STANDALONE", "PLATFORM_STANDALONE_WIN", "PLATFORM_STANDALONE")
	case "darwin":
		out = append(out, "UNITY_EDITOR_OSX", "UNITY_STANDALONE_OSX", "UNITY_STANDALONE", "PLATFORM_STANDALONE_OSX", "PLATFORM_STANDALONE")
	default:
		out = append(out, "UNITY_EDITOR_LINUX", "UNITY_STANDALONE_LINUX", "UNITY_STANDALONE", "PLATFORM_STANDALONE_LINUX", "PLATFORM_STANDALONE")
	}
	parts := versionPart.FindAllString(version, 3)
	if len(parts) < 2 {
		return out
	}
	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])
	if len(parts) == 3 {
		out = append(out, fmt.Sprintf("UNITY_%d_%d_%s", major, minor, parts[2]))
	}
	out = append(out, fmt.Sprintf("UNITY_%d_%d", major, minor), fmt.Sprintf("UNITY_%d", major))
	// Every line from 5.3 up to this one: UNITY_2021_3_OR_NEWER and so on.
	type line struct{ major, minor int }
	var lines []line
	for m := 3; m <= 6; m++ {
		lines = append(lines, line{5, m})
	}
	for y := 2017; y <= 2023; y++ {
		for m := 1; m <= 4; m++ {
			lines = append(lines, line{y, m})
		}
	}
	for v := 6000; v <= major; v++ {
		for m := 0; m <= 9; m++ {
			lines = append(lines, line{v, m})
		}
	}
	for _, l := range lines {
		if l.major < major || (l.major == major && l.minor <= minor) {
			out = append(out, fmt.Sprintf("UNITY_%d_%d_OR_NEWER", l.major, l.minor))
		}
	}
	return out
}

// Generated is a project Projects wrote for an assembly.
type Generated struct {
	Name     string   `json:"name"`
	Project  string   `json:"project"`            // the .csproj written
	Dir      string   `json:"dir"`                // the assembly's folder, where its .asmdef is
	Asmdef   string   `json:"asmdef"`             // "" for a default assembly (Assembly-CSharp)
	Files    []string `json:"files"`              // its scripts
	Projects []string `json:"projects"`           // the names of the assemblies here it references
	DLLs     []string `json:"dlls"`               // the assemblies it references as compiled files
	External bool     `json:"external,omitempty"` // a package's, not the folder's
}

// Generation is what Projects wrote.
type Generation struct {
	Solution string      `json:"solution"`
	Unity    *Install    `json:"unity,omitempty"`
	Projects []Generated `json:"projects"`
}

// Projects writes a solution and a project for each assembly of w into
// out, and returns them. It writes a file only when its text changes, so
// a language server watching them reloads only then.
func Projects(w Workspace, out string, unity *Install) (Generation, error) {
	g := Generation{Unity: unity}
	as := ScanAssemblies(w.Root)
	owners := map[string]string{} // folder → assembly name
	byName := map[string]Assembly{}
	for _, a := range as.Assemblies {
		if a.External || a.Error != "" {
			continue
		}
		owners[filepath.Dir(a.Path)] = a.Name
		byName[a.Name] = a
	}
	for _, r := range as.Asmrefs {
		if r.Target != "" {
			owners[filepath.Dir(r.Path)] = r.Target
		}
	}
	files := map[string][]string{}
	var dlls []string // the folder's own assemblies, plugins above all
	defaultFor := func(path string) string {
		if w.Kind == "unity" {
			if strings.Contains(filepath.ToSlash(path), "/Editor/") {
				return "Assembly-CSharp-Editor"
			}
			return "Assembly-CSharp"
		}
		return filepath.Base(w.Root) + ".Scripts"
	}
	for _, root := range scriptRoots(w) {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if path != root && (skipDir(d.Name()) || ignoredName(d.Name())) {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.EqualFold(filepath.Ext(path), ".dll") {
				dlls = append(dlls, path)
				return nil
			}
			if !strings.EqualFold(filepath.Ext(path), ".cs") {
				return nil
			}
			owner := ""
			for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
				if o, ok := owners[dir]; ok {
					owner = o
					break
				}
				if SamePath(dir, w.Root) || filepath.Dir(dir) == dir {
					break
				}
			}
			if owner == "" {
				owner = defaultFor(path)
			}
			files[owner] = append(files[owner], path)
			return nil
		})
	}
	names := make([]string, 0, len(files))
	for n := range byName {
		names = append(names, n)
	}
	for n := range files {
		if _, ok := byName[n]; !ok {
			names = append(names, n)
		}
	}
	sort.Strings(names)

	ext := newExternals(w, unity, as, dlls)
	var auto []string // the folder's .dll files every assembly gets
	for _, d := range dlls {
		if autoReferenced(d) {
			auto = append(auto, d)
		}
	}
	scriptAssemblies := filepath.Join(w.Root, "Library", "ScriptAssemblies")
	var outside []Assembly // assemblies of packages referenced, to write too
	taken := map[string]bool{}
	// references fills in what an assembly references: an assembly here,
	// one Unity compiled, or one of a package, and precompiled ones.
	references := func(p *Generated, a Assembly) {
		var names []string
		for _, r := range a.References {
			n := r.Name
			if n == "" && !strings.HasPrefix(r.Raw, "GUID:") {
				n = r.Raw
			}
			names = append(names, n)
		}
		precompiled := a.Precompiled
		if slices.Contains(a.OptionalUnity, "TestAssemblies") {
			names = append(names, "UnityEngine.TestRunner", "UnityEditor.TestRunner")
			precompiled = append(precompiled, "nunit.framework.dll")
		}
		for _, n := range names {
			if n == "" || n == p.Name || slices.Contains(p.Projects, n) {
				continue
			}
			if _, ok := byName[n]; ok {
				p.Projects = append(p.Projects, n)
			} else if dll := filepath.Join(scriptAssemblies, n+".dll"); isFile(dll) {
				p.DLLs = append(p.DLLs, dll)
			} else if b, ok := ext.assembly(n); ok {
				p.Projects = append(p.Projects, n)
				if !taken[n] {
					taken[n] = true
					outside = append(outside, b)
				}
			}
		}
		for _, f := range precompiled {
			if dll := ext.dll(f); dll != "" && !slices.Contains(p.DLLs, dll) {
				p.DLLs = append(p.DLLs, dll)
			}
		}
		if !a.OverrideReferences && !a.External {
			for _, d := range auto {
				if !slices.Contains(p.DLLs, d) {
					p.DLLs = append(p.DLLs, d)
				}
			}
		}
	}
	for _, name := range names {
		a, isAsm := byName[name]
		p := Generated{Name: name, Project: filepath.Join(out, name+".csproj"), Files: files[name], Projects: []string{}, DLLs: []string{}}
		sort.Strings(p.Files)
		if isAsm {
			p.Dir, p.Asmdef = filepath.Dir(a.Path), a.Path
			references(&p, a)
		} else {
			p.Dir = w.Root
			if w.Kind == "unity" {
				p.Dir = filepath.Join(w.Root, "Assets")
			}
			// A default assembly sees every auto-referenced one, but only
			// the editor's sees those for the Editor alone; neither sees
			// tests.
			editor := name == "Assembly-CSharp-Editor" || !strings.HasPrefix(name, "Assembly-CSharp")
			for _, n := range names {
				if b, ok := byName[n]; ok && b.AutoReferenced && !b.Test && (editor || !b.Editor) {
					p.Projects = append(p.Projects, n)
				}
			}
			if name == "Assembly-CSharp-Editor" && len(files["Assembly-CSharp"]) > 0 {
				p.Projects = append(p.Projects, "Assembly-CSharp")
			}
			references(&p, Assembly{Name: name})
		}
		if len(p.Files) == 0 && !isAsm {
			continue
		}
		g.Projects = append(g.Projects, p)
	}
	// Those of packages, and what they reference in turn.
	for i := 0; i < len(outside); i++ {
		a := outside[i]
		p := Generated{Name: a.Name, Project: filepath.Join(out, a.Name+".csproj"), Dir: filepath.Dir(a.Path), Asmdef: a.Path,
			Files: scripts(a), Projects: []string{}, DLLs: []string{}, External: true}
		references(&p, a)
		g.Projects = append(g.Projects, p)
	}
	outsideByName := map[string]Assembly{}
	for _, a := range outside {
		outsideByName[a.Name] = a
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return g, err
	}
	keep := map[string]bool{}
	for _, p := range g.Projects {
		a, ok := byName[p.Name]
		if !ok {
			a = outsideByName[p.Name]
		}
		text := projectText(p, a, unity, Defines(WantedVersion(w)), out)
		if err := writeIfChanged(p.Project, text); err != nil {
			return g, err
		}
		keep[filepath.Base(p.Project)] = true
	}
	// Projects of assemblies that are gone go too.
	old, _ := filepath.Glob(filepath.Join(out, "*.csproj"))
	for _, f := range old {
		if !keep[filepath.Base(f)] {
			_ = os.Remove(f)
		}
	}
	g.Solution = filepath.Join(out, filepath.Base(w.Root)+".sln")
	if err := writeIfChanged(g.Solution, solutionText(g.Projects)); err != nil {
		return g, err
	}
	return g, nil
}

// scriptRoots are the folders whose scripts Unity compiles: Assets and the
// packages of a project; the whole folder otherwise.
func scriptRoots(w Workspace) []string {
	if w.Kind != "unity" {
		return []string{w.Root}
	}
	roots := []string{filepath.Join(w.Root, "Assets")}
	for _, p := range w.Packages {
		roots = append(roots, p.Dir)
	}
	return roots
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func writeIfChanged(path, text string) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, []byte(text)) {
		return nil
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func projectGUID(name string) string {
	h := md5.Sum([]byte("bmeditor-csharp:" + name))
	s := hex.EncodeToString(h[:])
	return strings.ToUpper(s[0:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:32])
}

var xmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

func xmlEscape(s string) string { return xmlEscaper.Replace(s) }

// unityReferences are the editor's assemblies a project references, as
// Unity's own .csproj files list them: the .NET API Unity compiles against
// (unity-4.8-api and its facades) and every UnityEngine and UnityEditor
// module.
func unityReferences(u *Install) []string {
	if u == nil {
		return nil
	}
	var out []string
	api := filepath.Join(u.Data, "UnityReferenceAssemblies", "unity-4.8-api")
	for _, n := range []string{"mscorlib", "System", "System.Core", "System.Runtime.Serialization", "System.Xml", "System.Xml.Linq", "System.Numerics",
		"System.Numerics.Vectors", "System.Net.Http", "System.IO.Compression", "Microsoft.CSharp", "System.Data", "System.Data.DataSetExtensions",
		"System.Drawing", "System.IO.Compression.FileSystem", "System.ComponentModel.Composition", "System.Transactions"} {
		if p := filepath.Join(api, n+".dll"); isFile(p) {
			out = append(out, p)
		}
	}
	for _, dir := range []string{filepath.Join(api, "Facades"), filepath.Join(u.Data, "Managed", "UnityEngine")} {
		dlls, _ := filepath.Glob(filepath.Join(dir, "*.dll"))
		sort.Strings(dlls)
		out = append(out, dlls...)
	}
	return out
}

func projectText(p Generated, a Assembly, u *Install, defines []string, out string) string {
	var b bytes.Buffer
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }
	unsafe := "False"
	if a.AllowUnsafeCode {
		unsafe = "True"
	}
	w(`<?xml version="1.0" encoding="utf-8"?>
<!-- Written by the BMEditor C# extension for its language server, from %s.
     Not Unity's: Unity writes its own .csproj files next to the project. -->
<Project ToolsVersion="4.0" DefaultTargets="Build" xmlns="http://schemas.microsoft.com/developer/msbuild/2003">
  <PropertyGroup>
    <LangVersion>9.0</LangVersion>
    <_TargetFrameworkDirectories>non_empty_path_generated_by_unity.rider.package</_TargetFrameworkDirectories>
    <_FullFrameworkReferenceAssemblyPaths>non_empty_path_generated_by_unity.rider.package</_FullFrameworkReferenceAssemblyPaths>
    <DisableHandlePackageFileConflicts>true</DisableHandlePackageFileConflicts>
    <Configuration Condition=" '$(Configuration)' == '' ">Debug</Configuration>
    <Platform Condition=" '$(Platform)' == '' ">AnyCPU</Platform>
    <ProjectGuid>{%s}</ProjectGuid>
    <OutputType>Library</OutputType>
    <AssemblyName>%s</AssemblyName>
    <RootNamespace>%s</RootNamespace>
    <TargetFrameworkVersion>v4.7.1</TargetFrameworkVersion>
    <OutputPath>%s</OutputPath>
    <DefineConstants>%s</DefineConstants>
    <NoWarn>0169,0649</NoWarn>
    <AllowUnsafeBlocks>%s</AllowUnsafeBlocks>
    <NoConfig>true</NoConfig>
    <NoStdLib>true</NoStdLib>
    <AddAdditionalExplicitAssemblyReferences>false</AddAdditionalExplicitAssemblyReferences>
    <ImplicitlyExpandNETStandardFacades>false</ImplicitlyExpandNETStandardFacades>
    <ImplicitlyExpandDesignTimeFacades>false</ImplicitlyExpandDesignTimeFacades>
  </PropertyGroup>
  <ItemGroup>
`, xmlEscape(p.Dir), projectGUID(p.Name), xmlEscape(p.Name), xmlEscape(a.RootNamespace), xmlEscape(filepath.Join(out, "bin", p.Name)+string(filepath.Separator)), strings.Join(defines, ";"), unsafe)
	for _, f := range p.Files {
		w("    <Compile Include=\"%s\" />\n", xmlEscape(f))
	}
	if p.Asmdef != "" {
		w("    <None Include=\"%s\" />\n", xmlEscape(p.Asmdef))
	}
	w("  </ItemGroup>\n  <ItemGroup>\n")
	for _, dll := range append(unityReferences(u), p.DLLs...) {
		w("    <Reference Include=\"%s\">\n      <HintPath>%s</HintPath>\n    </Reference>\n",
			xmlEscape(strings.TrimSuffix(filepath.Base(dll), filepath.Ext(dll))), xmlEscape(dll))
	}
	w("  </ItemGroup>\n  <ItemGroup>\n")
	for _, n := range p.Projects {
		w("    <ProjectReference Include=\"%s.csproj\">\n      <Project>{%s}</Project>\n      <Name>%s</Name>\n    </ProjectReference>\n",
			xmlEscape(n), projectGUID(n), xmlEscape(n))
	}
	w("  </ItemGroup>\n  <Import Project=\"$(MSBuildToolsPath)\\Microsoft.CSharp.targets\" />\n</Project>\n")
	return b.String()
}

func solutionText(ps []Generated) string {
	var b bytes.Buffer
	b.WriteString("\r\nMicrosoft Visual Studio Solution File, Format Version 12.00\r\n# Visual Studio 16\r\n")
	for _, p := range ps {
		fmt.Fprintf(&b, "Project(\"{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}\") = \"%s\", \"%s\", \"{%s}\"\r\nEndProject\r\n", p.Name, filepath.Base(p.Project), projectGUID(p.Name))
	}
	// The packages' projects go in a Packages folder.
	packages := projectGUID("Packages folder")
	nested := slices.ContainsFunc(ps, func(p Generated) bool { return p.External })
	if nested {
		fmt.Fprintf(&b, "Project(\"{2150E333-8FDC-42A3-9474-1A3956D46DE8}\") = \"Packages\", \"Packages\", \"{%s}\"\r\nEndProject\r\n", packages)
	}
	b.WriteString("Global\r\n\tGlobalSection(SolutionConfigurationPlatforms) = preSolution\r\n\t\tDebug|Any CPU = Debug|Any CPU\r\n\tEndGlobalSection\r\n")
	b.WriteString("\tGlobalSection(ProjectConfigurationPlatforms) = postSolution\r\n")
	for _, p := range ps {
		g := projectGUID(p.Name)
		fmt.Fprintf(&b, "\t\t{%s}.Debug|Any CPU.ActiveCfg = Debug|Any CPU\r\n\t\t{%s}.Debug|Any CPU.Build.0 = Debug|Any CPU\r\n", g, g)
	}
	b.WriteString("\tEndGlobalSection\r\n")
	if nested {
		b.WriteString("\tGlobalSection(NestedProjects) = preSolution\r\n")
		for _, p := range ps {
			if p.External {
				fmt.Fprintf(&b, "\t\t{%s} = {%s}\r\n", projectGUID(p.Name), packages)
			}
		}
		b.WriteString("\tEndGlobalSection\r\n")
	}
	b.WriteString("EndGlobal\r\n")
	return b.String()
}

// ProjectsDir is where the projects of a folder are written: a folder of
// the user's cache named after it.
func ProjectsDir(root string) (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	h := md5.Sum([]byte(strings.ToLower(filepath.Clean(root))))
	return filepath.Join(cache, "bmeditor-csharp", "projects", filepath.Base(root)+"-"+hex.EncodeToString(h[:4])), nil
}
