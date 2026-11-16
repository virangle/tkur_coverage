// SPDX-License-Identifier: EPL-2.0
// tkur_coverage imports measured engine output; it never runs a fuzz target.
package main

import (
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const version = "0.2.0"

//go:embed assets/*
var resources embed.FS

type options struct {
	Engine, Input, Root, Output, Module, Provenance string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "tkur_coverage:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Println("tkur_coverage " + version + "\nImport measured source coverage (no Python required).\n  rust | llvm-import --llvm-json FILE --source-root DIR --output NEW_DIR\n  go | go-import --profile FILE --module-path MOD --source-root DIR --output NEW_DIR\nOptional: --provenance FILE (external replay/build manifest, copied unchanged)")
		return nil
	}
	if args[0] == "--version" {
		fmt.Println("tkur_coverage", version)
		return nil
	}
	o := options{}
	switch args[0] {
	case "rust":
		o.Engine = "Rust / LLVM"
	case "llvm-import", "native-import":
		o.Engine = "LLVM"
	case "go", "go-import":
		o.Engine = "Go / cover"
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	if strings.HasPrefix(o.Engine, "Go") {
		fs.StringVar(&o.Input, "profile", "", "native Go coverage profile")
		fs.StringVar(&o.Module, "module-path", "", "module import prefix used in profile filenames")
	} else {
		fs.StringVar(&o.Input, "llvm-json", "", "complete llvm-cov export JSON (not summary-only)")
	}
	fs.StringVar(&o.Root, "source-root", "", "explicit source directory; paths outside it are rejected")
	fs.StringVar(&o.Output, "output", "", "new or empty report directory, disjoint from sources")
	fs.StringVar(&o.Provenance, "provenance", "", "optional external replay/build JSON, retained unchanged")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 || o.Input == "" || o.Root == "" || o.Output == "" {
		return errors.New("provide input, --source-root and --output; positional arguments are not accepted")
	}
	return generate(o)
}

func generate(o options) error {
	root, err := filepath.Abs(o.Root)
	if err != nil {
		return err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("source-root must be a directory")
	}
	o.Root = root
	raw, err := readRegular(o.Input, 64<<20)
	if err != nil {
		return err
	}
	var files []sourceFile
	if strings.HasPrefix(o.Engine, "Go") {
		files, err = parseGo(raw, o)
	} else {
		files, err = parseLLVM(raw, o)
	}
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("profile contains no source files")
	}
	var provenance []byte
	if o.Provenance != "" {
		provenance, err = readRegular(o.Provenance, 16<<20)
		if err != nil {
			return err
		}
		if !json.Valid(provenance) {
			return errors.New("provenance must be valid JSON")
		}
	}
	return writeReport(o, files, raw, provenance)
}
