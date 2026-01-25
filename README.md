# gomodfmt

A formatter for `go.mod` files that sorts and organizes directives.

## What it does

- Separates require directives into two blocks: direct dependencies first, indirect second
- Sorts all directives alphabetically
- Consolidates scattered directives into blocks
- Preserves all comments (module, block, and inline)

## Installation

**Homebrew:**

```bash
brew install albertocavalcante/tap/gomodfmt

# or tap first, then install by name
brew tap albertocavalcante/tap
brew install gomodfmt
```

**Go:**

```bash
go install github.com/albertocavalcante/gomodfmt/cmd/gomodfmt@latest
```

## Usage

```bash
gomodfmt go.mod          # print to stdout
gomodfmt -w go.mod       # write to file
gomodfmt -d go.mod       # show diff
gomodfmt -l go.mod       # list files that need formatting
cat go.mod | gomodfmt    # read from stdin
```

| Flag | Description                            |
| ---- | -------------------------------------- |
| `-w` | Write result to file instead of stdout |
| `-d` | Display diff                           |
| `-l` | List files whose formatting differs    |

## Example

**Before:**

```
module example.com/myapp

go 1.24

// Core library
require github.com/zzz/pkg v1.0.0 // utils

require (
    github.com/aaa/pkg v1.0.0
    golang.org/x/text v0.14.0 // indirect
)

tool github.com/zzz/tool
tool github.com/aaa/tool

require golang.org/x/sync v0.5.0 // indirect
```

**After:**

```
module example.com/myapp

go 1.24

// Core library
require (
    github.com/aaa/pkg v1.0.0
    github.com/zzz/pkg v1.0.0 // utils
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

## Directive ordering

Output order is fixed:

```
module
go / toolchain
godebug
require (direct)
require (indirect)
replace
exclude
retract
tool
```

The `tool` directive is placed last intentionally. When reading a go.mod, dependencies are the primary concern—tools are secondary, "for development you'll also need these" information. Keeping them at the end maintains focus on what the module actually needs to run.

> **Note:** Ordering is not configurable. For a single-file formatter, configuration adds complexity without much benefit. This might change if gomodfmt grows to support recursive formatting or `go.work` files, where project-wide consistency would make configuration more valuable.

## Supported directives

| Directive | Sorting                                |
| --------- | -------------------------------------- |
| `require` | alphabetically, direct before indirect |
| `replace` | by module path, then version           |
| `exclude` | by module path, then version           |
| `retract` | by version                             |
| `tool`    | alphabetically                         |
| `godebug` | by key                                 |

## Development

```bash
go install github.com/evilmartians/lefthook@latest
lefthook install
```

## License

MIT
