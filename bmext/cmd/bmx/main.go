// Command bmx helps write a bmeditor extension. It works in the extension's
// folder, the one with bmext.json:
//
//	bmx init [-backend] [-name n]   make a new extension here, or update its copy of the SDK
//	bmx dev [-dir d]                build it for this system only and install it in the editor
//	bmx check                       install it from a clean git clone, as the editor would, and check the result
//	bmx pack                        build it for every system and zip it: <name>-<version>.zip, to publish
//
// Install it from a clone of this repository, in bmext/,
//
//	go install ./cmd/bmx
//
// or with go install github.com/Cn8001/BMEditorExtensions/bmext/cmd/bmx@latest.
package main

import (
	"flag"
	"fmt"
	"os"
)

const usage = `usage:
  bmx init [-backend] [-name n]   make a new extension here, or update its copy of the SDK
  bmx dev [-dir d]                build it for this system only and install it in the editor
  bmx check                       install it from a clean git clone and check the result
  bmx pack                        build it for every system and zip it, to publish
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmds := map[string]func([]string) error{"init": cmdInit, "dev": cmdDev, "check": cmdCheck, "pack": cmdPack}
	run, ok := cmds[os.Args[1]]
	if !ok {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err := run(os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, "bmx:", err)
		os.Exit(1)
	}
}

// flags parses a command's flags; it has no positional arguments.
func flags(name string, args []string, define func(*flag.FlagSet)) error {
	fs := flag.NewFlagSet("bmx "+name, flag.ContinueOnError)
	define(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("%s: unexpected %q", name, fs.Arg(0))
	}
	return nil
}
