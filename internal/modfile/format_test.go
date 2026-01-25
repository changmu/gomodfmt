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
		{
			name: "sorts require alphabetically",
			input: `module example.com/test

go 1.21

require (
	github.com/pkg/errors v0.9.1
	github.com/google/uuid v1.5.0
	github.com/aws/aws-sdk-go v1.50.0
)
`,
			want: `module example.com/test

go 1.21

require (
	github.com/aws/aws-sdk-go v1.50.0
	github.com/google/uuid v1.5.0
	github.com/pkg/errors v0.9.1
)
`,
		},
		{
			name: "sorts direct and indirect deps together alphabetically",
			input: `module example.com/test

go 1.21

require (
	github.com/pkg/errors v0.9.1
	golang.org/x/sync v0.5.0 // indirect
	github.com/google/uuid v1.5.0
	golang.org/x/tools v0.16.0 // indirect
)
`,
			want: `module example.com/test

go 1.21

require (
	github.com/google/uuid v1.5.0
	github.com/pkg/errors v0.9.1
	golang.org/x/sync v0.5.0 // indirect
	golang.org/x/tools v0.16.0 // indirect
)
`,
		},
		{
			name: "sorts replace directives",
			input: `module example.com/test

go 1.21

replace (
	github.com/zzz/pkg => ../zzz
	github.com/aaa/pkg => ../aaa
	github.com/mmm/pkg => ../mmm
)
`,
			want: `module example.com/test

go 1.21

replace github.com/aaa/pkg => ../aaa

replace github.com/mmm/pkg => ../mmm

replace github.com/zzz/pkg => ../zzz
`,
		},
		{
			name:    "handles invalid go.mod",
			input:   "this is not a valid go.mod",
			wantErr: true,
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
			if strings.TrimSpace(string(got)) != strings.TrimSpace(tt.want) {
				t.Errorf("Format() =\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestFormat_PreservesModule(t *testing.T) {
	input := `module github.com/example/myproject

go 1.22

require github.com/google/uuid v1.5.0
`
	got, err := Format([]byte(input))
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	if !strings.Contains(string(got), "module github.com/example/myproject") {
		t.Error("Format() lost module declaration")
	}
	if !strings.Contains(string(got), "go 1.22") {
		t.Error("Format() lost go version")
	}
}
