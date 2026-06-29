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
// - All comments are preserved (module, inline, and block comments)
func Format(data []byte) ([]byte, error) {
	f, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return nil, fmt.Errorf("parsing go.mod: %w", err)
	}

	// Build comment maps from original parsed file before restructuring
	comments := extractComments(f)

	// Create a fresh go.mod with just module and go version
	newData := []byte(fmt.Sprintf("module %s\n\ngo %s\n", f.Module.Mod.Path, f.Go.Version))
	if f.Toolchain != nil {
		newData = []byte(fmt.Sprintf("module %s\n\ngo %s\n\ntoolchain %s\n", f.Module.Mod.Path, f.Go.Version, f.Toolchain.Name))
	}

	newFile, err := modfile.Parse("go.mod", newData, nil)
	if err != nil {
		return nil, fmt.Errorf("creating new go.mod: %w", err)
	}

	// Transfer module comment from original
	if f.Module != nil && f.Module.Syntax != nil {
		newFile.Module.Syntax.Before = f.Module.Syntax.Before
	}

	// Add godebug (sorted) with comments
	sort.Slice(f.Godebug, func(i, j int) bool {
		return f.Godebug[i].Key < f.Godebug[j].Key
	})
	for _, g := range f.Godebug {
		newFile.AddGodebug(g.Key, g.Value)
	}
	applyGodebugComments(newFile, comments)

	// Add requires using SetRequireSeparateIndirect for proper block separation
	// Create fresh Require objects to ensure consolidation, then transfer comments
	sort.Slice(f.Require, func(i, j int) bool {
		if f.Require[i].Indirect != f.Require[j].Indirect {
			return !f.Require[i].Indirect
		}
		return f.Require[i].Mod.Path < f.Require[j].Mod.Path
	})
	var mods []*modfile.Require
	for _, r := range f.Require {
		mods = append(mods, &modfile.Require{
			Mod:      module.Version{Path: r.Mod.Path, Version: r.Mod.Version},
			Indirect: r.Indirect,
		})
	}
	newFile.SetRequireSeparateIndirect(mods)
	applyRequireComments(newFile, comments)

	// Add replaces (sorted) consolidated into a single block with comments.
	// modfile.AddReplace would emit one Line per (path, version) pair; we want
	// all replaces under one `replace ( ... )` block. Cleanup() will fold the
	// block back into a single Line when it contains only one entry.
	sort.Slice(f.Replace, func(i, j int) bool {
		if f.Replace[i].Old.Path != f.Replace[j].Old.Path {
			return f.Replace[i].Old.Path < f.Replace[j].Old.Path
		}
		return f.Replace[i].Old.Version < f.Replace[j].Old.Version
	})
	addReplaceBlock(newFile, f.Replace, comments)

	// Add excludes (sorted) with comments
	sort.Slice(f.Exclude, func(i, j int) bool {
		if f.Exclude[i].Mod.Path != f.Exclude[j].Mod.Path {
			return f.Exclude[i].Mod.Path < f.Exclude[j].Mod.Path
		}
		return f.Exclude[i].Mod.Version < f.Exclude[j].Mod.Version
	})
	for _, e := range f.Exclude {
		newFile.AddExclude(e.Mod.Path, e.Mod.Version)
	}
	applyExcludeComments(newFile, comments)

	// Add retracts (sorted) with comments
	// Note: Don't pass Rationale to AddRetract as it converts all comments to Before comments.
	// We'll manually apply the comments after adding.
	sort.Slice(f.Retract, func(i, j int) bool {
		if f.Retract[i].Low != f.Retract[j].Low {
			return f.Retract[i].Low < f.Retract[j].Low
		}
		return f.Retract[i].High < f.Retract[j].High
	})
	for _, r := range f.Retract {
		// Pass empty rationale to avoid auto-generated comments
		newFile.AddRetract(modfile.VersionInterval{Low: r.Low, High: r.High}, "")
	}
	applyRetractComments(newFile, comments)

	// Add tools (sorted) with comments
	sort.Slice(f.Tool, func(i, j int) bool {
		return f.Tool[i].Path < f.Tool[j].Path
	})
	for _, t := range f.Tool {
		newFile.AddTool(t.Path)
	}
	applyToolComments(newFile, comments)

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

// commentData stores comments extracted from a parsed go.mod file
type commentData struct {
	// requireComments maps "path" -> (before, suffix, after comments)
	requireComments map[string]lineComments
	// replaceComments maps "oldPath|oldVersion" -> comments
	replaceComments map[string]lineComments
	// excludeComments maps "path|version" -> comments
	excludeComments map[string]lineComments
	// retractComments maps "low|high" -> comments
	retractComments map[string]lineComments
	// toolComments maps "path" -> comments
	toolComments map[string]lineComments
	// godebugComments maps "key" -> comments
	godebugComments map[string]lineComments
	// firstDirectRequireBefore stores comments before the first direct require
	firstDirectRequireBefore []modfile.Comment
	// firstIndirectRequireBefore stores comments before the first indirect require
	firstIndirectRequireBefore []modfile.Comment
	// firstReplaceBlockBefore stores comments before the first replace
	firstReplaceBlockBefore []modfile.Comment
	// firstExcludeBlockBefore stores comments before the first exclude
	firstExcludeBlockBefore []modfile.Comment
	// firstRetractBlockBefore stores comments before the first retract
	firstRetractBlockBefore []modfile.Comment
	// firstToolBlockBefore stores comments before the first tool
	firstToolBlockBefore []modfile.Comment
	// firstGodebugBlockBefore stores comments before the first godebug
	firstGodebugBlockBefore []modfile.Comment
	// replaceRParenBefore stores comments before the closing ) of replace blocks
	replaceRParenBefore []modfile.Comment
}

type lineComments struct {
	before []modfile.Comment
	suffix []modfile.Comment
	after  []modfile.Comment
}

func extractComments(f *modfile.File) *commentData {
	c := &commentData{
		requireComments: make(map[string]lineComments),
		replaceComments: make(map[string]lineComments),
		excludeComments: make(map[string]lineComments),
		retractComments: make(map[string]lineComments),
		toolComments:    make(map[string]lineComments),
		godebugComments: make(map[string]lineComments),
	}

	// Build a set of indirect require paths for quick lookup
	indirectPaths := make(map[string]bool)
	for _, r := range f.Require {
		if r.Indirect {
			indirectPaths[r.Mod.Path] = true
		}
	}

	// Extract block-level comments from syntax tree
	// Comments before "require (" blocks are stored on the LineBlock, not on individual lines
	firstDirectBlockSeen := false
	firstIndirectBlockSeen := false
	for _, stmt := range f.Syntax.Stmt {
		switch s := stmt.(type) {
		case *modfile.LineBlock:
			if len(s.Token) > 0 && s.Token[0] == "require" && len(s.Before) > 0 {
				// Determine if this block is mostly direct or indirect
				hasIndirect := false
				hasDirect := false
				for _, line := range s.Line {
					if len(line.Token) > 0 {
						path := line.Token[0]
						if indirectPaths[path] {
							hasIndirect = true
						} else {
							hasDirect = true
						}
					}
				}
				// If block has any direct requires, treat it as a direct block
				if hasDirect && !firstDirectBlockSeen {
					c.firstDirectRequireBefore = s.Before
					firstDirectBlockSeen = true
				} else if hasIndirect && !hasDirect && !firstIndirectBlockSeen {
					c.firstIndirectRequireBefore = s.Before
					firstIndirectBlockSeen = true
				}
			}
		case *modfile.Line:
			// Single-line require statements
			if len(s.Token) > 0 && s.Token[0] == "require" && len(s.Before) > 0 {
				// Check if it's direct or indirect by looking at the path
				if len(s.Token) >= 2 {
					path := s.Token[1]
					if indirectPaths[path] && !firstIndirectBlockSeen {
						c.firstIndirectRequireBefore = s.Before
						firstIndirectBlockSeen = true
					} else if !indirectPaths[path] && !firstDirectBlockSeen {
						c.firstDirectRequireBefore = s.Before
						firstDirectBlockSeen = true
					}
				}
			}
		}
	}

	// Build set of syntax lines that live inside a LineBlock.
	// For block-internal lines, before/after represent per-line comments and
	// must be preserved. For top-level single-line directives, before/after
	// represent block-level comments and are already captured via
	// firstXxxBefore — re-storing them here would duplicate the comment in
	// the rendered output.
	inBlock := make(map[*modfile.Line]bool)
	for _, stmt := range f.Syntax.Stmt {
		if block, ok := stmt.(*modfile.LineBlock); ok {
			for _, line := range block.Line {
				inBlock[line] = true
			}
		}
	}

	// Extract individual line comments from require entries
	for _, r := range f.Require {
		if r.Syntax != nil {
			lc := lineComments{suffix: r.Syntax.Suffix}
			if inBlock[r.Syntax] {
				lc.before = r.Syntax.Before
				lc.after = r.Syntax.After
			}
			c.requireComments[r.Mod.Path] = lc
		}
	}

	// Extract block-level comments for other directives from syntax tree
	for _, stmt := range f.Syntax.Stmt {
		switch s := stmt.(type) {
		case *modfile.LineBlock:
			if len(s.Token) > 0 && len(s.Before) > 0 {
				switch s.Token[0] {
				case "replace":
					if len(c.firstReplaceBlockBefore) == 0 {
						c.firstReplaceBlockBefore = s.Before
					}
				case "exclude":
					if len(c.firstExcludeBlockBefore) == 0 {
						c.firstExcludeBlockBefore = s.Before
					}
				case "retract":
					if len(c.firstRetractBlockBefore) == 0 {
						c.firstRetractBlockBefore = s.Before
					}
				case "tool":
					if len(c.firstToolBlockBefore) == 0 {
						c.firstToolBlockBefore = s.Before
					}
				case "godebug":
					if len(c.firstGodebugBlockBefore) == 0 {
						c.firstGodebugBlockBefore = s.Before
					}
				}
			}
			if len(s.Token) > 0 && s.Token[0] == "replace" && len(s.RParen.Before) > 0 {
				c.replaceRParenBefore = append(c.replaceRParenBefore, s.RParen.Before...)
			}
		case *modfile.Line:
			if len(s.Token) > 0 && len(s.Before) > 0 {
				switch s.Token[0] {
				case "replace":
					if len(c.firstReplaceBlockBefore) == 0 {
						c.firstReplaceBlockBefore = s.Before
					}
				case "exclude":
					if len(c.firstExcludeBlockBefore) == 0 {
						c.firstExcludeBlockBefore = s.Before
					}
				case "retract":
					if len(c.firstRetractBlockBefore) == 0 {
						c.firstRetractBlockBefore = s.Before
					}
				case "tool":
					if len(c.firstToolBlockBefore) == 0 {
						c.firstToolBlockBefore = s.Before
					}
				case "godebug":
					if len(c.firstGodebugBlockBefore) == 0 {
						c.firstGodebugBlockBefore = s.Before
					}
				}
			}
		}
	}

	// Extract individual line comments. Apply inBlock gate same as require:
	// only block-internal lines have meaningful before/after comments.
	for _, r := range f.Replace {
		if r.Syntax != nil {
			key := r.Old.Path + "|" + r.Old.Version
			lc := lineComments{suffix: r.Syntax.Suffix}
			if inBlock[r.Syntax] {
				lc.before = r.Syntax.Before
				lc.after = r.Syntax.After
			}
			c.replaceComments[key] = lc
		}
	}

	for _, e := range f.Exclude {
		if e.Syntax != nil {
			key := e.Mod.Path + "|" + e.Mod.Version
			lc := lineComments{suffix: e.Syntax.Suffix}
			if inBlock[e.Syntax] {
				lc.before = e.Syntax.Before
				lc.after = e.Syntax.After
			}
			c.excludeComments[key] = lc
		}
	}

	for _, r := range f.Retract {
		if r.Syntax != nil {
			key := r.Low + "|" + r.High
			lc := lineComments{suffix: r.Syntax.Suffix}
			if inBlock[r.Syntax] {
				lc.before = r.Syntax.Before
				lc.after = r.Syntax.After
			}
			c.retractComments[key] = lc
		}
	}

	for _, t := range f.Tool {
		if t.Syntax != nil {
			lc := lineComments{suffix: t.Syntax.Suffix}
			if inBlock[t.Syntax] {
				lc.before = t.Syntax.Before
				lc.after = t.Syntax.After
			}
			c.toolComments[t.Path] = lc
		}
	}

	for _, g := range f.Godebug {
		if g.Syntax != nil {
			lc := lineComments{suffix: g.Syntax.Suffix}
			if inBlock[g.Syntax] {
				lc.before = g.Syntax.Before
				lc.after = g.Syntax.After
			}
			c.godebugComments[g.Key] = lc
		}
	}

	return c
}

// applyLineComments restores before/suffix/after comments onto a Line.
// before/after are only meaningful for block-internal lines; extractComments
// gates them via the inBlock set.
func applyLineComments(line *modfile.Line, lc lineComments) {
	if len(lc.before) > 0 {
		line.Before = lc.before
	}
	if len(lc.suffix) > 0 {
		line.Suffix = lc.suffix
	}
	if len(lc.after) > 0 {
		line.After = lc.after
	}
}

func applyRequireComments(f *modfile.File, c *commentData) {
	if len(f.Require) == 0 {
		return
	}

	// Apply individual line comments (before, suffix, after)
	for _, r := range f.Require {
		if lc, ok := c.requireComments[r.Mod.Path]; ok {
			applyLineComments(r.Syntax, lc)
		}
	}

	// Apply block-level comments to LineBlocks in syntax tree
	// SetRequireSeparateIndirect creates exactly two blocks: first for direct, second for indirect
	directBlockDone := false
	indirectBlockDone := false
	for _, stmt := range f.Syntax.Stmt {
		if block, ok := stmt.(*modfile.LineBlock); ok {
			if len(block.Token) > 0 && block.Token[0] == "require" {
				// Check if this block has indirect requires (by checking suffix comments)
				hasIndirect := false
				for _, line := range block.Line {
					for _, suffix := range line.Suffix {
						if strings.Contains(suffix.Token, "indirect") {
							hasIndirect = true
							break
						}
					}
					if hasIndirect {
						break
					}
				}

				if !hasIndirect && !directBlockDone && len(c.firstDirectRequireBefore) > 0 {
					block.Before = c.firstDirectRequireBefore
					directBlockDone = true
				} else if hasIndirect && !indirectBlockDone && len(c.firstIndirectRequireBefore) > 0 {
					block.Before = c.firstIndirectRequireBefore
					indirectBlockDone = true
				}
			}
		}
	}
}

// addReplaceBlock appends all replaces as a single `replace ( ... )` block to
// newFile, restoring per-line and block-level comments. Cleanup() will fold a
// one-entry block back into a single Line, so this also handles the singleton
// case correctly.
func addReplaceBlock(newFile *modfile.File, replaces []*modfile.Replace, c *commentData) {
	if len(replaces) == 0 {
		return
	}

	block := &modfile.LineBlock{Token: []string{"replace"}}
	if len(c.firstReplaceBlockBefore) > 0 {
		block.Before = c.firstReplaceBlockBefore
	}

	for _, r := range replaces {
		tokens := []string{modfile.AutoQuote(r.Old.Path)}
		if r.Old.Version != "" {
			tokens = append(tokens, r.Old.Version)
		}
		tokens = append(tokens, "=>", modfile.AutoQuote(r.New.Path))
		if r.New.Version != "" {
			tokens = append(tokens, r.New.Version)
		}

		line := &modfile.Line{Token: tokens, InBlock: true}
		key := r.Old.Path + "|" + r.Old.Version
		if lc, ok := c.replaceComments[key]; ok {
			applyLineComments(line, lc)
		}
		block.Line = append(block.Line, line)

		newFile.Replace = append(newFile.Replace, &modfile.Replace{
			Old:    module.Version{Path: r.Old.Path, Version: r.Old.Version},
			New:    module.Version{Path: r.New.Path, Version: r.New.Version},
			Syntax: line,
		})
	}

	for _, com := range c.replaceRParenBefore {
		block.Line = append(block.Line, &modfile.Line{
			Token:   []string{com.Token},
			InBlock: true,
		})
	}

	newFile.Syntax.Stmt = append(newFile.Syntax.Stmt, block)
}

func applyExcludeComments(f *modfile.File, c *commentData) {
	if len(f.Exclude) == 0 {
		return
	}
	// Apply individual line comments (before, suffix, after)
	for _, e := range f.Exclude {
		key := e.Mod.Path + "|" + e.Mod.Version
		if lc, ok := c.excludeComments[key]; ok {
			applyLineComments(e.Syntax, lc)
		}
	}
	// Apply block-level comment
	if len(c.firstExcludeBlockBefore) > 0 {
		applyBlockCommentToDirective(f.Syntax, "exclude", c.firstExcludeBlockBefore)
	}
}

func applyRetractComments(f *modfile.File, c *commentData) {
	// AddRetract adds to syntax tree but doesn't populate f.Retract slice.
	// We need to find retract lines directly in the syntax tree and apply comments.
	// For top-level single-line retracts, suppress per-line before/after since
	// those are block-level (already applied via firstRetractBlockBefore).
	firstRetractDone := false
	for _, stmt := range f.Syntax.Stmt {
		switch s := stmt.(type) {
		case *modfile.Line:
			if len(s.Token) >= 2 && s.Token[0] == "retract" {
				low, high := parseRetractVersion(s.Token[1:])
				key := low + "|" + high
				if lc, ok := c.retractComments[key]; ok {
					// Top-level Line: only restore suffix
					s.Suffix = lc.suffix
				}
				if !firstRetractDone && len(c.firstRetractBlockBefore) > 0 {
					s.Before = c.firstRetractBlockBefore
					firstRetractDone = true
				}
			}
		case *modfile.LineBlock:
			if len(s.Token) > 0 && s.Token[0] == "retract" {
				if !firstRetractDone && len(c.firstRetractBlockBefore) > 0 {
					s.Before = c.firstRetractBlockBefore
					firstRetractDone = true
				}
				// Block-internal lines: restore full (before, suffix, after)
				for _, line := range s.Line {
					low, high := parseRetractVersion(line.Token)
					key := low + "|" + high
					if lc, ok := c.retractComments[key]; ok {
						applyLineComments(line, lc)
					}
				}
			}
		}
	}
}

// parseRetractVersion extracts low and high versions from retract tokens
func parseRetractVersion(tokens []string) (low, high string) {
	if len(tokens) == 0 {
		return "", ""
	}
	// Single version: [v1.0.0]
	if len(tokens) == 1 {
		v := strings.Trim(tokens[0], "[]")
		return v, v
	}
	// Range: [[v1.0.0, v2.0.0] or [v1.0.0 v2.0.0]
	if tokens[0] == "[" {
		// Format: [ v1.0.0 , v2.0.0 ]
		low = ""
		high = ""
		for _, t := range tokens {
			if t == "[" || t == "," || t == "]" {
				continue
			}
			if low == "" {
				low = t
			} else if high == "" {
				high = t
				break
			}
		}
		return low, high
	}
	// Format: [v1.0.0, v2.0.0] as a single token (unlikely but handle it)
	if strings.HasPrefix(tokens[0], "[") {
		inner := strings.Trim(tokens[0], "[]")
		parts := strings.Split(inner, ",")
		if len(parts) == 2 {
			return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		}
	}
	return tokens[0], tokens[0]
}

func applyToolComments(f *modfile.File, c *commentData) {
	if len(f.Tool) == 0 {
		return
	}
	// Apply individual line comments (before, suffix, after)
	for _, t := range f.Tool {
		if lc, ok := c.toolComments[t.Path]; ok {
			applyLineComments(t.Syntax, lc)
		}
	}
	// Apply block-level comment
	if len(c.firstToolBlockBefore) > 0 {
		applyBlockCommentToDirective(f.Syntax, "tool", c.firstToolBlockBefore)
	}
}

func applyGodebugComments(f *modfile.File, c *commentData) {
	if len(f.Godebug) == 0 {
		return
	}
	// Apply individual line comments (before, suffix, after)
	for _, g := range f.Godebug {
		if lc, ok := c.godebugComments[g.Key]; ok {
			applyLineComments(g.Syntax, lc)
		}
	}
	// Apply block-level comment
	if len(c.firstGodebugBlockBefore) > 0 {
		applyBlockCommentToDirective(f.Syntax, "godebug", c.firstGodebugBlockBefore)
	}
}

// applyBlockCommentToDirective finds the first occurrence of a directive type
// in the syntax tree and applies the block-level comment to it.
func applyBlockCommentToDirective(syntax *modfile.FileSyntax, directive string, comments []modfile.Comment) {
	for _, stmt := range syntax.Stmt {
		switch s := stmt.(type) {
		case *modfile.LineBlock:
			if len(s.Token) > 0 && s.Token[0] == directive {
				s.Before = comments
				return
			}
		case *modfile.Line:
			if len(s.Token) > 0 && s.Token[0] == directive {
				s.Before = comments
				return
			}
		}
	}
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
