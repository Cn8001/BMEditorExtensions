package unity

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ScriptKinds are the scripts NewScript makes, in the order a menu shows
// them.
var ScriptKinds = []struct {
	ID, Label string
}{
	{"monobehaviour", "MonoBehaviour"},
	{"scriptableobject", "ScriptableObject"},
	{"editor", "Custom Editor"},
	{"editorwindow", "Editor Window"},
	{"class", "Class"},
	{"interface", "Interface"},
	{"struct", "Struct"},
	{"enum", "Enum"},
	{"test", "Test (NUnit)"},
}

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Namespace is the namespace of a new script in dir: the rootNamespace of
// the nearest assembly definition at or above it, else the project's own.
func Namespace(w Workspace, dir string) string {
	for d := dir; ; d = filepath.Dir(d) {
		defs, _ := filepath.Glob(filepath.Join(d, "*.asmdef"))
		if len(defs) > 0 {
			raw, err := os.ReadFile(defs[0])
			var f asmdefFile
			if err == nil && json.Unmarshal(trimBOM(raw), &f) == nil {
				return f.RootNamespace
			}
			return ""
		}
		if SamePath(d, w.Root) || filepath.Dir(d) == d {
			break
		}
	}
	return w.RootNamespace
}

// ScriptText is the source of a new script of a kind.
func ScriptText(kind, name, namespace string) (string, error) {
	var usings []string
	var body string
	switch kind {
	case "monobehaviour":
		usings = []string{"UnityEngine"}
		body = "public class " + name + " : MonoBehaviour\n{\n    void Start()\n    {\n    }\n\n    void Update()\n    {\n    }\n}\n"
	case "scriptableobject":
		usings = []string{"UnityEngine"}
		menu := namespace
		if menu == "" {
			menu = "Scriptable Objects"
		}
		body = fmt.Sprintf("[CreateAssetMenu(fileName = %q, menuName = %q)]\npublic class %s : ScriptableObject\n{\n}\n",
			name, strings.ReplaceAll(menu, ".", "/")+"/"+name, name)
	case "editor":
		target := strings.TrimSuffix(name, "Editor")
		if target == "" || target == name {
			target = "MonoBehaviour"
		}
		usings = []string{"UnityEditor", "UnityEngine"}
		body = "[CustomEditor(typeof(" + target + "))]\npublic class " + name + " : Editor\n{\n    public override void OnInspectorGUI()\n    {\n        base.OnInspectorGUI();\n    }\n}\n"
	case "editorwindow":
		usings = []string{"UnityEditor", "UnityEngine"}
		body = "public class " + name + " : EditorWindow\n{\n    [MenuItem(\"Window/" + name + "\")]\n    static void Open()\n    {\n        GetWindow<" + name + ">(\"" + name + "\");\n    }\n\n    void CreateGUI()\n    {\n    }\n}\n"
	case "class":
		body = "public class " + name + "\n{\n}\n"
	case "interface":
		body = "public interface " + name + "\n{\n}\n"
	case "struct":
		body = "public struct " + name + "\n{\n}\n"
	case "enum":
		body = "public enum " + name + "\n{\n}\n"
	case "test":
		usings = []string{"System.Collections", "NUnit.Framework", "UnityEngine.TestTools"}
		body = "public class " + name + "\n{\n    [Test]\n    public void Passes()\n    {\n        Assert.Pass();\n    }\n\n    [UnityTest]\n    public IEnumerator PassesAfterAFrame()\n    {\n        yield return null;\n        Assert.Pass();\n    }\n}\n"
	default:
		return "", fmt.Errorf("unknown script kind %q", kind)
	}
	var b strings.Builder
	for _, u := range usings {
		b.WriteString("using " + u + ";\n")
	}
	if len(usings) > 0 {
		b.WriteString("\n")
	}
	if namespace == "" {
		b.WriteString(body)
		return b.String(), nil
	}
	b.WriteString("namespace " + namespace + "\n{\n")
	for _, l := range strings.Split(strings.TrimSuffix(body, "\n"), "\n") {
		if l != "" {
			b.WriteString("    " + l)
		}
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	return b.String(), nil
}

// NewScript writes dir/name.cs of a kind, and its .meta when meta is set
// and dir is a folder Unity imports. It returns the script's path.
func NewScript(w Workspace, dir, name, kind string, meta bool) (string, error) {
	if !identifier.MatchString(name) {
		return "", fmt.Errorf("%q is not a C# name", name)
	}
	if rel, err := filepath.Rel(w.Root, dir); err != nil || strings.HasPrefix(rel, "..") {
		return "", errors.New("the folder is outside the open folder")
	}
	path := filepath.Join(dir, name+".cs")
	if _, err := os.Lstat(path); err == nil {
		return "", fmt.Errorf("%s is there already", filepath.Base(path))
	}
	text, err := ScriptText(kind, name, Namespace(w, dir))
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return "", err
	}
	if meta && importedFolder(w, dir) {
		if err := WriteMeta(path); err != nil {
			return path, err
		}
	}
	return path, nil
}

// importedFolder says whether Unity imports what is in dir: it has a
// .meta itself, or it is Assets/ or a package's root.
func importedFolder(w Workspace, dir string) bool {
	if _, err := os.Stat(dir + ".meta"); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		return true
	}
	return w.Kind == "unity" && SamePath(dir, filepath.Join(w.Root, "Assets"))
}
