package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/albertocavalcante/gomodfmt/internal/modfile"
)

var (
	write = flag.Bool("w", false, "write result to (source) file instead of stdout")
	diff  = flag.Bool("d", false, "display diffs instead of rewriting files")
	list  = flag.Bool("l", false, "list files whose formatting differs from gomodfmt's")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: gomodfmt [flags] [path ...]\n")
		fmt.Fprintf(os.Stderr, "\nFormats go.mod files by sorting and aligning dependencies.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		// Read from stdin
		if err := processReader(os.Stdin, "<stdin>"); err != nil {
			fmt.Fprintf(os.Stderr, "gomodfmt: %v\n", err)
			os.Exit(1)
		}
		return
	}

	exitCode := 0
	for _, path := range args {
		if err := processFile(path); err != nil {
			fmt.Fprintf(os.Stderr, "gomodfmt: %v\n", err)
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

func processReader(r io.Reader, name string) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading %s: %w", name, err)
	}

	formatted, err := modfile.Format(data)
	if err != nil {
		return fmt.Errorf("formatting %s: %w", name, err)
	}

	os.Stdout.Write(formatted)
	return nil
}

func processFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	formatted, err := modfile.Format(data)
	if err != nil {
		return fmt.Errorf("formatting %s: %w", path, err)
	}

	if string(data) == string(formatted) {
		return nil // no changes needed
	}

	if *list {
		fmt.Println(path)
		return nil
	}

	if *diff {
		d, err := modfile.Diff(data, formatted, path)
		if err != nil {
			return err
		}
		os.Stdout.Write(d)
		return nil
	}

	if *write {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		return os.WriteFile(path, formatted, info.Mode())
	}

	os.Stdout.Write(formatted)
	return nil
}
