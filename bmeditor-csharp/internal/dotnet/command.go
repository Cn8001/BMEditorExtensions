package dotnet

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	ErrNoDotnet     = errors.New("dotnet is not on the PATH")
	ErrNothingToRun = errors.New("no project to run")
	ErrUnknownVerb  = errors.New("unknown dotnet command")
)

// Bin is the dotnet program on the PATH.
func Bin() (string, error) {
	p, err := exec.LookPath("dotnet")
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrNoDotnet, err)
	}
	return p, nil
}

// Args are dotnet's arguments for verb (run, build, test) on target, a
// solution or a project. A solution cannot be run, so run takes its first
// program, as Visual Studio's startup project.
func Args(verb, target string) ([]string, error) {
	switch verb {
	case "build", "test":
		return []string{verb, target, "-nologo"}, nil
	case "run":
		project := target
		if !strings.EqualFold(filepath.Ext(target), ".csproj") {
			p, err := Startup(target)
			if err != nil {
				return nil, err
			}
			project = p
		}
		return []string{"run", "--project", project}, nil
	}
	return nil, fmt.Errorf("%q: %w", verb, ErrUnknownVerb)
}

// Startup is the solution's first project that builds a program.
func Startup(solution string) (string, error) {
	s, err := Load(solution)
	if err != nil {
		return "", err
	}
	found := ""
	walkItems(s.Items, func(it *Item) {
		if found == "" && it.Project != nil && isProgram(it.Project.OutputType) {
			found = it.Project.Path
		}
	})
	if found == "" {
		return "", fmt.Errorf("%s: %w", filepath.Base(solution), ErrNothingToRun)
	}
	return found, nil
}

func isProgram(outputType string) bool {
	return strings.EqualFold(outputType, "Exe") || strings.EqualFold(outputType, "WinExe")
}
