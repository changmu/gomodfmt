# gomodfmt

An opinionated formatter for `go.mod` files that enforces consistent styling and organization.

## Philosophy

`gomodfmt` is opinionated. It enforces a specific style:

- **Exactly two require blocks**: direct dependencies first, indirect dependencies second
- **Alphabetical sorting**: all directives are sorted alphabetically
- **Consolidated blocks**: scattered single-line directives are grouped into blocks
- **No endless require blocks**: messy go.mod files with multiple scattered require blocks are cleaned up

## Supported Directives

- `require` - sorted alphabetically, separated into direct/indirect blocks
- `replace` - sorted alphabetically by old path
- `exclude` - sorted alphabetically by path, then version
- `retract` - sorted by version
- `tool` - sorted alphabetically (Go 1.24+)
- `godebug` - sorted alphabetically by key (Go 1.21+)

## Installation

```bash
go install github.com/albertocavalcante/gomodfmt/cmd/gomodfmt@latest
```

Requires Go 1.24 or later.

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

### Before

```
module example.com/myapp

go 1.24

require github.com/zzz/pkg v1.0.0

require (
	github.com/aaa/pkg v1.0.0
	golang.org/x/text v0.14.0 // indirect
)

tool github.com/zzz/tool
tool github.com/aaa/tool

godebug zipinsecurepath=0
godebug asynctimerchan=0

require golang.org/x/sync v0.5.0 // indirect
```

### After

```
module example.com/myapp

go 1.24

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

tool (
	github.com/aaa/tool
	github.com/zzz/tool
)
```

## Limitations

- Comments at the top of the file are not preserved (trade-off for block consolidation)
- Inline comments on directives are preserved

## License

MIT
