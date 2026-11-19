package dotnet

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
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

const library = `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><TargetFramework>net10.0</TargetFramework></PropertyGroup></Project>`
const program = `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><OutputType>Exe</OutputType><TargetFramework>net10.0</TargetFramework></PropertyGroup></Project>`

func TestArgs_RunsTheSolutionsProgram(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	write(t, filepath.Join(dir, "Lib", "Lib.csproj"), library)
	write(t, filepath.Join(dir, "App", "App.csproj"), program)
	sln := filepath.Join(dir, "App.slnx")
	write(t, sln, `<Solution><Project Path="Lib/Lib.csproj" /><Project Path="App/App.csproj" /></Solution>`)

	// Act
	got, err := Args("run", sln)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"run", "--project", filepath.Join(dir, "App", "App.csproj")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Args = %v, want %v", got, want)
	}
}

func TestArgs_NoProgramInTheSolution(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	write(t, filepath.Join(dir, "Lib", "Lib.csproj"), library)
	sln := filepath.Join(dir, "Lib.slnx")
	write(t, sln, `<Solution><Project Path="Lib/Lib.csproj" /></Solution>`)

	// Act
	_, err := Args("run", sln)

	// Assert
	if !errors.Is(err, ErrNothingToRun) {
		t.Errorf("err = %v, want ErrNothingToRun", err)
	}
}

func TestArgs_BuildsAndTestsTheTargetAsGiven(t *testing.T) {
	// Arrange
	target := filepath.Join("x", "App.sln")

	// Act
	build, errBuild := Args("build", target)
	_, errOther := Args("publish", target)

	// Assert
	if errBuild != nil || !reflect.DeepEqual(build, []string{"build", target, "-nologo"}) {
		t.Errorf("build = %v, %v", build, errBuild)
	}
	if !errors.Is(errOther, ErrUnknownVerb) {
		t.Errorf("publish err = %v, want ErrUnknownVerb", errOther)
	}
}

func TestParseProblem_ReadsCompilerErrors(t *testing.T) {
	// Arrange
	dir := filepath.Join(string(filepath.Separator), "src", "App")
	abs := filepath.Join(dir, "Program.cs")
	lines := []string{
		abs + "(3,14): error CS1002: ; expected [" + filepath.Join(dir, "App.csproj") + "]",
		"Program.cs(7,1,7,9): warning CS0168: The variable 'x' is declared but never used",
		"Build succeeded.",
	}

	// Act
	var got []Problem
	for _, l := range lines {
		if p, ok := ParseProblem(l, dir); ok {
			got = append(got, p)
		}
	}

	// Assert
	want := []Problem{
		{File: abs, Line: 3, Col: 14, Severity: "error", Message: "CS1002: ; expected", Source: "dotnet"},
		{File: abs, Line: 7, Col: 1, Severity: "warning", Message: "CS0168: The variable 'x' is declared but never used", Source: "dotnet"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}
