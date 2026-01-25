# gomodfmt

A formatter for `go.mod` files that sorts and organizes dependencies.

## Features

- Sorts `require` blocks alphabetically
- Groups direct and indirect dependencies separately
- Sorts `replace` and `exclude` blocks
- Aligns version comments
- Supports stdin/stdout or in-place file editing

## Installation

```bash
go install github.com/albertocavalcante/gomodfmt/cmd/gomodfmt@latest
```

## Usage

```bash
# Format go.mod and print to stdout
gomodfmt go.mod

# Format go.mod in place
gomodfmt -w go.mod

# Show diff of what would change
gomodfmt -d go.mod

# List files that need formatting
gomodfmt -l go.mod

# Read from stdin
cat go.mod | gomodfmt
```

## Flags

| Flag | Description |
|------|-------------|
| `-w` | Write result to (source) file instead of stdout |
| `-d` | Display diffs instead of rewriting files |
| `-l` | List files whose formatting differs from gomodfmt's |

## Example

Before:
```
module example.com/myapp

go 1.21

require (
	github.com/pkg/errors v0.9.1
	golang.org/x/sync v0.5.0 // indirect
	github.com/stretchr/testify v1.8.4
	golang.org/x/tools v0.16.0 // indirect
	github.com/google/uuid v1.5.0
)
```

After:
```
module example.com/myapp

go 1.21

require (
	github.com/google/uuid v1.5.0
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.8.4
)

require (
	golang.org/x/sync v0.5.0 // indirect
	golang.org/x/tools v0.16.0 // indirect
)
```

## License

MIT
