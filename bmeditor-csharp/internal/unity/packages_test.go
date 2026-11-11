package unity

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// writeTgz writes a package .tgz as Unity ships them: its files under package/.
func writeTgz(t *testing.T, path string, files map[string]string) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, text := range files {
		if err := tw.WriteHeader(&tar.Header{Name: "package/" + name, Mode: 0o644, Size: int64(len(text)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		_, _ = tw.Write([]byte(text))
	}
	_ = tw.Close()
	_ = gz.Close()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, text := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProjects_TakesTheTestFrameworkFromTheEditor(t *testing.T) {
	// Arrange: a package with tests, and an editor that ships the Test
	// Framework and NUnit as .tgz files, as Unity 2022 does.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())
	u := fakeUnity(t)
	pm := filepath.Join(u.Data, "Resources", "PackageManager", "Editor")
	writeTgz(t, filepath.Join(pm, "com.unity.test-framework-1.1.33.tgz"), map[string]string{
		"package.json": `{"name":"com.unity.test-framework","version":"1.1.33","dependencies":{"com.unity.ext.nunit":"1.0.6","com.unity.modules.imgui":"1.0.0"}}`,
		"UnityEngine.TestRunner/UnityEngine.TestRunner.asmdef": `{"name":"UnityEngine.TestRunner","overrideReferences":true,"precompiledReferences":["nunit.framework.dll"]}`,
		"UnityEngine.TestRunner/Assertions/LogAssert.cs":       "class LogAssert {}",
		"UnityEditor.TestRunner/UnityEditor.TestRunner.asmdef": `{"name":"UnityEditor.TestRunner","references":["UnityEngine.TestRunner"],"includePlatforms":["Editor"],"overrideReferences":true,"precompiledReferences":["nunit.framework.dll"]}`,
		"UnityEditor.TestRunner/TestRunnerWindow.cs":           "class TestRunnerWindow {}",
		"Documentation~/index.md":                              "docs",
	})
	writeTgz(t, filepath.Join(pm, "com.unity.test-framework.performance-3.5.0.tgz"), map[string]string{"package.json": `{"name":"com.unity.test-framework.performance"}`})
	writeTgz(t, filepath.Join(pm, "com.unity.ext.nunit-1.0.6.tgz"), map[string]string{
		"package.json":                           `{"name":"com.unity.ext.nunit","version":"1.0.6"}`,
		"net35/unity-custom/nunit.framework.dll": "",
	})
	root := filepath.Join(t.TempDir(), "Pkg")
	writeFiles(t, root, map[string]string{
		"package.json":                `{"name":"com.test.pkg","version":"1.0.0","unity":"2022.3"}`,
		"Runtime/Test.Pkg.asmdef":     `{"name":"Test.Pkg"}`,
		"Runtime/Thing.cs":            "class Thing {}",
		"Tests/Test.Pkg.Tests.asmdef": `{"name":"Test.Pkg.Tests","references":["UnityEngine.TestRunner","UnityEditor.TestRunner","Test.Pkg"],"overrideReferences":true,"precompiledReferences":["nunit.framework.dll"],"defineConstraints":["UNITY_INCLUDE_TESTS"]}`,
		"Tests/ThingTests.cs":         "class ThingTests {}",
	})
	w := Detect(root)

	// Act
	g, err := Projects(w, filepath.Join(t.TempDir(), "projects"), u)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Generated{}
	for _, p := range g.Projects {
		got[p.Name] = p
	}
	tests := got["Test.Pkg.Tests"]
	if want := []string{"UnityEngine.TestRunner", "UnityEditor.TestRunner", "Test.Pkg"}; !slices.Equal(tests.Projects, want) {
		t.Errorf("the tests reference %v, want %v", tests.Projects, want)
	}
	if len(tests.DLLs) != 1 || filepath.Base(tests.DLLs[0]) != "nunit.framework.dll" {
		t.Errorf("the tests' assemblies = %v, want NUnit's", tests.DLLs)
	}
	runner, ok := got["UnityEngine.TestRunner"]
	if !ok || !runner.External || len(runner.Files) != 1 || filepath.Base(runner.Files[0]) != "LogAssert.cs" || len(runner.DLLs) != 1 {
		t.Errorf("UnityEngine.TestRunner = %+v", runner)
	}
	if e := got["UnityEditor.TestRunner"]; !slices.Equal(e.Projects, []string{"UnityEngine.TestRunner"}) {
		t.Errorf("UnityEditor.TestRunner references %v", e.Projects)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(runner.Dir), "Documentation~")); !os.IsNotExist(err) {
		t.Error("a folder Unity leaves out should not be unpacked")
	}
	sln, _ := os.ReadFile(g.Solution)
	if !strings.Contains(string(sln), `"Packages", "Packages"`) || !strings.Contains(string(sln), "NestedProjects") {
		t.Errorf("the packages' projects should be in a Packages folder:\n%s", sln)
	}
}

func TestAutoReferenced_OnlyDotNetAssemblies(t *testing.T) {
	// A file that is no PE image at all, a native plugin say, is not.
	p := filepath.Join(t.TempDir(), "native.dll")
	_ = os.WriteFile(p, []byte("MZ not really"), 0o644)
	if autoReferenced(p) {
		t.Error("a file that is not a .NET assembly should not be referenced")
	}
}

func TestWanted(t *testing.T) {
	for rel, want := range map[string]bool{
		"package.json":             true,
		"Runtime/A.cs":             true,
		"Runtime/A.asmdef":         true,
		"Runtime/A.asmdef.meta":    true,
		"Runtime/A.cs.meta":        false,
		"Samples~/S/B.cs":          false,
		".Runtime/x.dll":           false,
		"Documentation~/index.md":  false,
		"../evil.cs":               false,
		"net35/unity-custom/n.dll": true,
	} {
		if got := wanted(rel); got != want {
			t.Errorf("wanted(%q) = %v, want %v", rel, got, want)
		}
	}
}
