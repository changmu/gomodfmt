package modfile

import (
	"strings"
	"testing"
)

func TestFormat(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		// ==================== REQUIRE BLOCK TESTS ====================
		{
			name: "sorts require block alphabetically",
			input: `module example.com/test

go 1.24

require (
	github.com/zzz/pkg v1.0.0
	github.com/aaa/pkg v1.0.0
	github.com/mmm/pkg v1.0.0
)
`,
			want: `module example.com/test

go 1.24

require (
	github.com/aaa/pkg v1.0.0
	github.com/mmm/pkg v1.0.0
	github.com/zzz/pkg v1.0.0
)
`,
		},
		{
			name: "separates direct and indirect into exactly two blocks",
			input: `module example.com/test

go 1.24

require (
	github.com/pkg/errors v0.9.1
	golang.org/x/sync v0.5.0 // indirect
	github.com/google/uuid v1.5.0
	golang.org/x/tools v0.16.0 // indirect
)
`,
			want: `module example.com/test

go 1.24

require (
	github.com/google/uuid v1.5.0
	github.com/pkg/errors v0.9.1
)

require (
	golang.org/x/sync v0.5.0 // indirect
	golang.org/x/tools v0.16.0 // indirect
)
`,
		},
		{
			name: "consolidates multiple scattered require blocks into two",
			input: `module example.com/test

go 1.24

require github.com/zzz/pkg v1.0.0

require (
	github.com/aaa/pkg v1.0.0
	golang.org/x/text v0.14.0 // indirect
)

require github.com/mmm/pkg v1.0.0

require golang.org/x/sync v0.5.0 // indirect

require (
	github.com/bbb/pkg v1.0.0
	golang.org/x/tools v0.16.0 // indirect
)
`,
			want: `module example.com/test

go 1.24

require (
	github.com/aaa/pkg v1.0.0
	github.com/bbb/pkg v1.0.0
	github.com/mmm/pkg v1.0.0
	github.com/zzz/pkg v1.0.0
)

require (
	golang.org/x/sync v0.5.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/tools v0.16.0 // indirect
)
`,
		},
		{
			name: "handles only direct deps - single block",
			input: `module example.com/test

go 1.24

require (
	github.com/zzz/pkg v1.0.0
	github.com/aaa/pkg v1.0.0
)
`,
			want: `module example.com/test

go 1.24

require (
	github.com/aaa/pkg v1.0.0
	github.com/zzz/pkg v1.0.0
)
`,
		},
		{
			name: "handles only indirect deps - single block",
			input: `module example.com/test

go 1.24

require (
	golang.org/x/sync v0.5.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)
`,
			want: `module example.com/test

go 1.24

require (
	golang.org/x/sync v0.5.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)
`,
		},
		{
			name: "handles single require statement",
			input: `module example.com/test

go 1.24

require github.com/google/uuid v1.5.0
`,
			want: `module example.com/test

go 1.24

require github.com/google/uuid v1.5.0
`,
		},

		// ==================== TOOL DIRECTIVE TESTS ====================
		{
			name: "sorts tool directives alphabetically",
			input: `module example.com/test

go 1.24

tool (
	golang.org/x/tools/cmd/stringer
	github.com/zzz/tool
	github.com/aaa/tool
)
`,
			want: `module example.com/test

go 1.24

tool (
	github.com/aaa/tool
	github.com/zzz/tool
	golang.org/x/tools/cmd/stringer
)
`,
		},
		{
			name: "consolidates scattered tool directives into one block",
			input: `module example.com/test

go 1.24

tool github.com/zzz/tool

tool (
	github.com/mmm/tool
	github.com/aaa/tool
)

tool github.com/bbb/tool
`,
			want: `module example.com/test

go 1.24

tool (
	github.com/aaa/tool
	github.com/bbb/tool
	github.com/mmm/tool
	github.com/zzz/tool
)
`,
		},

		// ==================== GODEBUG DIRECTIVE TESTS ====================
		{
			name: "sorts godebug directives alphabetically by key",
			input: `module example.com/test

go 1.24

godebug zipinsecurepath=0
godebug asynctimerchan=0
godebug httplaxcontentlength=1
`,
			// Library groups into a block - that's fine
			want: `module example.com/test

go 1.24

godebug (
	asynctimerchan=0
	httplaxcontentlength=1
	zipinsecurepath=0
)
`,
		},

		// ==================== REPLACE DIRECTIVE TESTS ====================
		{
			name: "sorts replace directives alphabetically",
			input: `module example.com/test

go 1.24

replace (
	github.com/zzz/pkg => ../zzz
	github.com/aaa/pkg => ../aaa
	github.com/mmm/pkg => ../mmm
)
`,
			want: `module example.com/test

go 1.24

replace github.com/aaa/pkg => ../aaa

replace github.com/mmm/pkg => ../mmm

replace github.com/zzz/pkg => ../zzz
`,
		},
		{
			name: "sorts replace with versions",
			input: `module example.com/test

go 1.24

replace (
	github.com/zzz/pkg v1.0.0 => github.com/fork/zzz v1.0.1
	github.com/aaa/pkg v0.9.0 => github.com/fork/aaa v0.9.1
)
`,
			want: `module example.com/test

go 1.24

replace github.com/aaa/pkg v0.9.0 => github.com/fork/aaa v0.9.1

replace github.com/zzz/pkg v1.0.0 => github.com/fork/zzz v1.0.1
`,
		},

		// ==================== EXCLUDE DIRECTIVE TESTS ====================
		{
			name: "sorts exclude directives",
			input: `module example.com/test

go 1.24

exclude (
	github.com/zzz/pkg v1.0.0
	github.com/aaa/pkg v1.0.0
	github.com/aaa/pkg v0.9.0
)
`,
			// Library groups excludes with same path together
			want: `module example.com/test

go 1.24

exclude (
	github.com/aaa/pkg v0.9.0
	github.com/aaa/pkg v1.0.0
)

exclude github.com/zzz/pkg v1.0.0
`,
		},

		// ==================== RETRACT DIRECTIVE TESTS ====================
		{
			name: "sorts retract directives",
			input: `module example.com/test

go 1.24

retract v1.0.0
retract v0.5.0
retract [v0.1.0, v0.2.0]
`,
			// Library groups retracts
			want: `module example.com/test

go 1.24

retract (
	[v0.1.0, v0.2.0]
	v0.5.0
	v1.0.0
)
`,
		},

		// ==================== COMPLEX/COMBINED TESTS ====================
		{
			name: "formats complex go.mod with all directives",
			input: `module example.com/myapp

go 1.24

toolchain go1.25.6

godebug zipinsecurepath=0
godebug asynctimerchan=0

require github.com/zzz/pkg v1.0.0

require (
	github.com/aaa/pkg v1.0.0
	golang.org/x/text v0.14.0 // indirect
)

tool github.com/zzz/tool
tool github.com/aaa/tool

replace github.com/zzz/pkg => ../zzz
replace github.com/aaa/pkg => ../aaa

exclude github.com/bad/pkg v0.0.1

require golang.org/x/sync v0.5.0 // indirect
`,
			want: `module example.com/myapp

go 1.24

toolchain go1.25.6

godebug (
	asynctimerchan=0
	zipinsecurepath=0
)

require (
	github.com/aaa/pkg v1.0.0
	github.com/zzz/pkg v1.0.0
)

require (
	golang.org/x/sync v0.5.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)

replace github.com/aaa/pkg => ../aaa

replace github.com/zzz/pkg => ../zzz

exclude github.com/bad/pkg v0.0.1

tool (
	github.com/aaa/tool
	github.com/zzz/tool
)
`,
		},

		// ==================== EDGE CASES ====================
		{
			name: "preserves module and go version",
			input: `module github.com/example/myproject

go 1.24

toolchain go1.25.6
`,
			want: `module github.com/example/myproject

go 1.24

toolchain go1.25.6
`,
		},
		{
			name: "handles empty require block",
			input: `module example.com/test

go 1.24
`,
			want: `module example.com/test

go 1.24
`,
		},
		{
			name:    "rejects invalid go.mod",
			input:   "this is not a valid go.mod file",
			wantErr: true,
		},

		// ==================== COMMENT PRESERVATION TESTS ====================
		{
			name: "preserves module comment",
			input: `// Package myapp provides utilities.
module example.com/myapp

go 1.24

require github.com/google/uuid v1.5.0
`,
			want: `// Package myapp provides utilities.
module example.com/myapp

go 1.24

require github.com/google/uuid v1.5.0
`,
		},
		{
			name: "preserves inline comments on requires",
			input: `module example.com/test

go 1.24

require (
	github.com/zzz/pkg v1.0.0 // needed for feature X
	github.com/aaa/pkg v1.0.0 // core dependency
)
`,
			want: `module example.com/test

go 1.24

require (
	github.com/aaa/pkg v1.0.0 // core dependency
	github.com/zzz/pkg v1.0.0 // needed for feature X
)
`,
		},
		{
			name: "preserves block comments before require sections",
			input: `module example.com/test

go 1.24

// Direct dependencies
require (
	github.com/zzz/pkg v1.0.0
	github.com/aaa/pkg v1.0.0
)
`,
			want: `module example.com/test

go 1.24

// Direct dependencies
require (
	github.com/aaa/pkg v1.0.0
	github.com/zzz/pkg v1.0.0
)
`,
		},
		{
			name: "preserves comments on replace directives",
			input: `module example.com/test

go 1.24

// Use local development version
replace github.com/some/pkg => ../local-pkg
`,
			want: `module example.com/test

go 1.24

// Use local development version
replace github.com/some/pkg => ../local-pkg
`,
		},
		{
			name: "preserves inline comments on replace directives",
			input: `module example.com/test

go 1.24

replace (
	github.com/zzz/pkg => ../zzz // local dev
	github.com/aaa/pkg => ../aaa // fork
)
`,
			want: `module example.com/test

go 1.24

replace github.com/aaa/pkg => ../aaa // fork

replace github.com/zzz/pkg => ../zzz // local dev
`,
		},
		{
			name: "preserves comments on tool directives",
			input: `module example.com/test

go 1.24

// Development tools
tool (
	github.com/zzz/tool // linter
	github.com/aaa/tool // formatter
)
`,
			want: `module example.com/test

go 1.24

// Development tools
tool (
	github.com/aaa/tool // formatter
	github.com/zzz/tool // linter
)
`,
		},
		{
			name: "preserves comments on exclude directives",
			input: `module example.com/test

go 1.24

// Broken versions
exclude github.com/bad/pkg v0.0.1 // security issue
`,
			want: `module example.com/test

go 1.24

// Broken versions
exclude github.com/bad/pkg v0.0.1 // security issue
`,
		},
		{
			name: "preserves comments on retract directives",
			input: `module example.com/test

go 1.24

// Released by mistake
retract v1.0.0 // accidental release
`,
			want: `module example.com/test

go 1.24

// Released by mistake
retract v1.0.0 // accidental release
`,
		},
		{
			name: "preserves comments on godebug directives",
			input: `module example.com/test

go 1.24

// Compatibility settings
godebug asynctimerchan=0 // for legacy code
`,
			want: `module example.com/test

go 1.24

// Compatibility settings
godebug asynctimerchan=0 // for legacy code
`,
		},
		{
			name: "preserves mixed comments throughout file",
			input: `// Main application module
module example.com/test

go 1.24

// Core dependencies
require (
	github.com/zzz/pkg v1.0.0 // utility lib
	github.com/aaa/pkg v1.0.0 // core lib
)

// Indirect deps from tooling
require golang.org/x/sync v0.5.0 // indirect

// Local overrides for development
replace github.com/some/pkg => ../local // dev override
`,
			want: `// Main application module
module example.com/test

go 1.24

// Core dependencies
require (
	github.com/aaa/pkg v1.0.0 // core lib
	github.com/zzz/pkg v1.0.0 // utility lib
)

// Indirect deps from tooling
require golang.org/x/sync v0.5.0 // indirect

// Local overrides for development
replace github.com/some/pkg => ../local // dev override
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Format([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			gotStr := strings.TrimSpace(string(got))
			wantStr := strings.TrimSpace(tt.want)
			if gotStr != wantStr {
				t.Errorf("Format() mismatch:\n=== GOT ===\n%s\n=== WANT ===\n%s\n", gotStr, wantStr)
			}
		})
	}
}

func TestFormat_Idempotent(t *testing.T) {
	input := `module example.com/test

go 1.24

toolchain go1.25.6

godebug asynctimerchan=0

require (
	github.com/aaa/pkg v1.0.0
	github.com/zzz/pkg v1.0.0
)

require golang.org/x/sync v0.5.0 // indirect

replace github.com/old/pkg => github.com/new/pkg v1.0.0

tool github.com/some/tool
`
	// Format once
	first, err := Format([]byte(input))
	if err != nil {
		t.Fatalf("First Format() error = %v", err)
	}

	// Format again - should be identical
	second, err := Format(first)
	if err != nil {
		t.Fatalf("Second Format() error = %v", err)
	}

	if string(first) != string(second) {
		t.Errorf("Format() not idempotent:\n=== FIRST ===\n%s\n=== SECOND ===\n%s\n", first, second)
	}
}

func TestFormat_IdempotentWithComments(t *testing.T) {
	input := `// Main application module
module example.com/test

go 1.24

toolchain go1.25.6

// Debug settings
godebug asynctimerchan=0 // legacy compat

// Core dependencies
require (
	github.com/aaa/pkg v1.0.0 // important
	github.com/zzz/pkg v1.0.0 // also important
)

// Indirect deps
require golang.org/x/sync v0.5.0 // indirect

// Local development overrides
replace github.com/old/pkg => github.com/new/pkg v1.0.0 // use fork

// Dev tools
tool github.com/some/tool // formatter
`
	// Format once
	first, err := Format([]byte(input))
	if err != nil {
		t.Fatalf("First Format() error = %v", err)
	}

	// Format again - should be identical
	second, err := Format(first)
	if err != nil {
		t.Fatalf("Second Format() error = %v", err)
	}

	if string(first) != string(second) {
		t.Errorf("Format() with comments not idempotent:\n=== FIRST ===\n%s\n=== SECOND ===\n%s\n", first, second)
	}
}
