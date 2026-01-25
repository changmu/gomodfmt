package modfile

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

// Format formats a go.mod file with opinionated styling:
// - Exactly two require blocks: direct deps first, indirect deps second
// - All directive types sorted alphabetically
// - Consolidated blocks (no scattered single-line directives)
func Format(data []byte) ([]byte, error) {
	f, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return nil, fmt.Errorf("parsing go.mod: %w", err)
	}

	// Extract all data first
	extracted := extractData(f)

	// Create a fresh go.mod with just module and go version
	newData := []byte(fmt.Sprintf("module %s\n\ngo %s\n", f.Module.Mod.Path, f.Go.Version))
	if f.Toolchain != nil {
		newData = []byte(fmt.Sprintf("module %s\n\ngo %s\n\ntoolchain %s\n", f.Module.Mod.Path, f.Go.Version, f.Toolchain.Name))
	}

	newFile, err := modfile.Parse("go.mod", newData, nil)
	if err != nil {
		return nil, fmt.Errorf("creating new go.mod: %w", err)
	}

	// Add godebug (sorted)
	sort.Slice(extracted.godebugs, func(i, j int) bool {
		return extracted.godebugs[i].key < extracted.godebugs[j].key
	})
	for _, g := range extracted.godebugs {
		newFile.AddGodebug(g.key, g.value)
	}

	// Add requires using SetRequireSeparateIndirect for proper block separation
	sort.Slice(extracted.requires, func(i, j int) bool {
		if extracted.requires[i].indirect != extracted.requires[j].indirect {
			return !extracted.requires[i].indirect
		}
		return extracted.requires[i].path < extracted.requires[j].path
	})
	var mods []*modfile.Require
	for _, r := range extracted.requires {
		mods = append(mods, &modfile.Require{
			Mod:      module.Version{Path: r.path, Version: r.version},
			Indirect: r.indirect,
		})
	}
	newFile.SetRequireSeparateIndirect(mods)

	// Add replaces (sorted)
	sort.Slice(extracted.replaces, func(i, j int) bool {
		if extracted.replaces[i].oldPath != extracted.replaces[j].oldPath {
			return extracted.replaces[i].oldPath < extracted.replaces[j].oldPath
		}
		return extracted.replaces[i].oldVersion < extracted.replaces[j].oldVersion
	})
	for _, r := range extracted.replaces {
		newFile.AddReplace(r.oldPath, r.oldVersion, r.newPath, r.newVersion)
	}

	// Add excludes (sorted)
	sort.Slice(extracted.excludes, func(i, j int) bool {
		if extracted.excludes[i].path != extracted.excludes[j].path {
			return extracted.excludes[i].path < extracted.excludes[j].path
		}
		return extracted.excludes[i].version < extracted.excludes[j].version
	})
	for _, e := range extracted.excludes {
		newFile.AddExclude(e.path, e.version)
	}

	// Add retracts (sorted)
	sort.Slice(extracted.retracts, func(i, j int) bool {
		if extracted.retracts[i].low != extracted.retracts[j].low {
			return extracted.retracts[i].low < extracted.retracts[j].low
		}
		return extracted.retracts[i].high < extracted.retracts[j].high
	})
	for _, r := range extracted.retracts {
		newFile.AddRetract(modfile.VersionInterval{Low: r.low, High: r.high}, r.rationale)
	}

	// Add tools (sorted)
	sort.Strings(extracted.tools)
	for _, t := range extracted.tools {
		newFile.AddTool(t)
	}

	newFile.Cleanup()

	formatted, err := newFile.Format()
	if err != nil {
		return nil, fmt.Errorf("formatting go.mod: %w", err)
	}

	// Ensure trailing newline
	if len(formatted) > 0 && formatted[len(formatted)-1] != '\n' {
		formatted = append(formatted, '\n')
	}

	return formatted, nil
}

type extractedData struct {
	requires []struct {
		path, version string
		indirect      bool
	}
	replaces []struct {
		oldPath, oldVersion, newPath, newVersion string
	}
	excludes []struct {
		path, version string
	}
	tools    []string
	godebugs []struct {
		key, value string
	}
	retracts []struct {
		low, high, rationale string
	}
}

func extractData(f *modfile.File) extractedData {
	var data extractedData

	for _, r := range f.Require {
		data.requires = append(data.requires, struct {
			path, version string
			indirect      bool
		}{r.Mod.Path, r.Mod.Version, r.Indirect})
	}

	for _, r := range f.Replace {
		data.replaces = append(data.replaces, struct {
			oldPath, oldVersion, newPath, newVersion string
		}{r.Old.Path, r.Old.Version, r.New.Path, r.New.Version})
	}

	for _, e := range f.Exclude {
		data.excludes = append(data.excludes, struct {
			path, version string
		}{e.Mod.Path, e.Mod.Version})
	}

	for _, t := range f.Tool {
		data.tools = append(data.tools, t.Path)
	}

	for _, g := range f.Godebug {
		data.godebugs = append(data.godebugs, struct {
			key, value string
		}{g.Key, g.Value})
	}

	for _, r := range f.Retract {
		data.retracts = append(data.retracts, struct {
			low, high, rationale string
		}{r.Low, r.High, r.Rationale})
	}

	return data
}

// Diff returns a unified diff between old and new content.
func Diff(old, new []byte, filename string) ([]byte, error) {
	oldLines := strings.Split(string(old), "\n")
	newLines := strings.Split(string(new), "\n")

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "--- %s\n", filename)
	fmt.Fprintf(&buf, "+++ %s\n", filename)

	chunks := diffLines(oldLines, newLines)
	for _, chunk := range chunks {
		buf.WriteString(chunk)
	}

	return buf.Bytes(), nil
}

func diffLines(a, b []string) []string {
	var result []string

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
