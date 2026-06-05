package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/changmu/gomodfmt/internal/modfile"
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
		return writeFileAtomic(path, formatted, info.Mode())
	}

	os.Stdout.Write(formatted)
	return nil
}

// writeFileAtomic writes data to path atomically via "create temp in same dir
// + fsync + rename". This guarantees that an interrupted run (SIGKILL, panic,
// power loss) never leaves the target file truncated or partially written:
// either the rename happened (new content) or it didn't (original content).
//
// The temp file lives in the same directory as path so the rename is a single
// filesystem operation. Cross-filesystem renames fall back to copy+delete,
// which is not atomic.
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".gomodfmt-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if anything below fails before rename.
	defer func() {
		if _, statErr := os.Stat(tmpName); statErr == nil {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
