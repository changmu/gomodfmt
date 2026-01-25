package modfile

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

// Format formats a go.mod file by:
// - Sorting require blocks alphabetically
// - Grouping direct and indirect dependencies separately
// - Aligning version comments
func Format(data []byte) ([]byte, error) {
	f, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return nil, fmt.Errorf("parsing go.mod: %w", err)
	}

	// Collect all requires
	type req struct {
		path     string
		version  string
		indirect bool
	}
	var requires []req
	for _, r := range f.Require {
		requires = append(requires, req{
			path:     r.Mod.Path,
			version:  r.Mod.Version,
			indirect: r.Indirect,
		})
	}

	// Sort: direct first, then alphabetically
	sort.Slice(requires, func(i, j int) bool {
		if requires[i].indirect != requires[j].indirect {
			return !requires[i].indirect
		}
		return requires[i].path < requires[j].path
	})

	// Collect all replaces
	type repl struct {
		oldPath    string
		oldVersion string
		newPath    string
		newVersion string
	}
	var replaces []repl
	for _, r := range f.Replace {
		replaces = append(replaces, repl{
			oldPath:    r.Old.Path,
			oldVersion: r.Old.Version,
			newPath:    r.New.Path,
			newVersion: r.New.Version,
		})
	}

	// Sort replaces alphabetically by old path
	sort.Slice(replaces, func(i, j int) bool {
		return replaces[i].oldPath < replaces[j].oldPath
	})

	// Collect all excludes
	type excl struct {
		path    string
		version string
	}
	var excludes []excl
	for _, e := range f.Exclude {
		excludes = append(excludes, excl{
			path:    e.Mod.Path,
			version: e.Mod.Version,
		})
	}

	// Sort excludes
	sort.Slice(excludes, func(i, j int) bool {
		if excludes[i].path != excludes[j].path {
			return excludes[i].path < excludes[j].path
		}
		return excludes[i].version < excludes[j].version
	})

	// Use SetRequire to properly set all requires in sorted order
	var mods []*modfile.Require
	for _, r := range requires {
		mods = append(mods, &modfile.Require{
			Mod:      module.Version{Path: r.path, Version: r.version},
			Indirect: r.indirect,
		})
	}
	f.SetRequire(mods)

	// Drop all existing replaces and excludes
	for _, r := range f.Replace {
		f.DropReplace(r.Old.Path, r.Old.Version)
	}
	for _, e := range f.Exclude {
		f.DropExclude(e.Mod.Path, e.Mod.Version)
	}

	// Re-add replaces in sorted order
	for _, r := range replaces {
		if err := f.AddReplace(r.oldPath, r.oldVersion, r.newPath, r.newVersion); err != nil {
			return nil, fmt.Errorf("adding replace %s: %w", r.oldPath, err)
		}
	}

	// Re-add excludes in sorted order
	for _, e := range excludes {
		if err := f.AddExclude(e.path, e.version); err != nil {
			return nil, fmt.Errorf("adding exclude %s: %w", e.path, err)
		}
	}

	// Clean up the syntax tree
	f.Cleanup()

	// Format with proper alignment
	formatted, err := f.Format()
	if err != nil {
		return nil, fmt.Errorf("formatting go.mod: %w", err)
	}

	// Ensure trailing newline
	if len(formatted) > 0 && formatted[len(formatted)-1] != '\n' {
		formatted = append(formatted, '\n')
	}

	return formatted, nil
}

// Diff returns a unified diff between old and new content.
func Diff(old, new []byte, filename string) ([]byte, error) {
	oldLines := strings.Split(string(old), "\n")
	newLines := strings.Split(string(new), "\n")

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "--- %s\n", filename)
	fmt.Fprintf(&buf, "+++ %s\n", filename)

	// Simple diff implementation
	chunks := diffLines(oldLines, newLines)
	for _, chunk := range chunks {
		buf.WriteString(chunk)
	}

	return buf.Bytes(), nil
}

// diffLines creates a simple unified diff
func diffLines(a, b []string) []string {
	var result []string

	// Find differences using a simple LCS-based approach
	i, j := 0, 0
	lineA, lineB := 1, 1
	hunkStart := -1
	var hunk []string

	flushHunk := func() {
		if len(hunk) > 0 {
			result = append(result, fmt.Sprintf("@@ -%d +%d @@\n", hunkStart, hunkStart))
			result = append(result, hunk...)
			hunk = nil
		}
	}

	for i < len(a) || j < len(b) {
		if i < len(a) && j < len(b) && a[i] == b[j] {
			flushHunk()
			i++
			j++
			lineA++
			lineB++
		} else if j < len(b) && (i >= len(a) || !containsFrom(a, i, b[j])) {
			if hunkStart < 0 {
				hunkStart = lineA
			}
			hunk = append(hunk, "+"+b[j]+"\n")
			j++
			lineB++
		} else if i < len(a) {
			if hunkStart < 0 {
				hunkStart = lineA
			}
			hunk = append(hunk, "-"+a[i]+"\n")
			i++
			lineA++
		}
	}
	flushHunk()

	return result
}

func containsFrom(lines []string, start int, target string) bool {
	for i := start; i < len(lines) && i < start+10; i++ {
		if lines[i] == target {
			return true
		}
	}
	return false
}
