# CLI Reference

Complete reference for all PromptPacker flags and options.

## Usage

```
promptpacker [options] [files...]
```

When no `files` are given in file-spec mode (the default), the interactive TUI launches.

---

## Modes

| Mode | Flag | Description |
|---|---|---|
| File-Spec | `--mode file-spec` | **(Default)** Only include explicitly specified files/dirs |
| Auto | `--mode auto` | Scan the entire directory tree automatically |

### File-Spec Mode

The default. You must tell PromptPacker what to include, either via:

- **Positional args**: `promptpacker src/ cmd/ go.mod README.md`
- **`--include` flag**: `promptpacker --include src/,cmd/,go.mod`
- **Interactive TUI**: Run `promptpacker` with no args

### Auto Mode

```bash
promptpacker --mode auto
```

Scans everything (subject to `.gitignore` and default ignore rules). Equivalent to the v0.1 behavior.

---

## All Flags

### Core

| Flag | Type | Default | Description |
|---|---|---|---|
| `--root <path>` | string | current dir | Root directory to scan |
| `--output <file>` | string | `output.md` | Output Markdown file path |
| `--mode <mode>` | string | `file-spec` | Selection mode: `file-spec` or `auto` |
| `--include <list>` | string | — | Comma-separated files/dirs to include (file-spec mode) |
| `--workers <n>` | int | # CPU cores | Concurrent workers for file processing |
| `--force` | bool | false | Skip the preview prompt, generate immediately |

### Structure & Depth

| Flag | Type | Default | Description |
|---|---|---|---|
| `--structure-only` | bool | false | Output directory tree only, no file contents |
| `--max-depth <n>` | int | 0 (unlimited) | Maximum directory depth to traverse |
| `--show-sizes` | bool | false | Show file sizes in structure output (e.g. `main.go (4.2 KB)`) |
| `--show-extensions` | bool | false | Label file extensions explicitly in structure output |

### Filtering

| Flag | Type | Default | Description |
|---|---|---|---|
| `--exclude <patterns>` | string | — | Comma-separated glob patterns to exclude |
| `--include-ext <exts>` | string | — | Only include files with these extensions (e.g. `go,ts,md`) |
| `--exclude-ext <exts>` | string | — | Exclude files with these extensions (e.g. `log,tmp,bak`) |
| `--max-file-size <size>` | string | — | Skip files larger than this (e.g. `1MB`, `500KB`) |

### Git-Aware

| Flag | Type | Default | Description |
|---|---|---|---|
| `--changed` | bool | false | Only include files changed vs `main` branch |
| `--since <ref>` | string | — | Only include files changed since a git ref (tag, commit, branch) |
| `--branch <name>` | string | — | Compare against a specific branch instead of `main` |

> **Note:** Git flags require `git` to be installed and the project to be a git repository. If git is unavailable, a warning is printed and all files are included.

### Config

| Flag | Type | Default | Description |
|---|---|---|---|
| `--profile <name>` | string | — | Activate a named profile from `.promptpacker.yml` |

---

## Examples

```bash
# Interactive file selector (default)
promptpacker

# Include specific files
promptpacker src/ cmd/ go.mod README.md

# Scan everything automatically
promptpacker --mode auto

# Structure tree only, 3 levels deep, with file sizes
promptpacker --mode auto --structure-only --max-depth 3 --show-sizes

# Only Go and Markdown files
promptpacker --mode auto --include-ext go,md

# Exclude test files and binaries
promptpacker --mode auto --exclude-ext test,exe,dll

# Skip files over 500KB
promptpacker --mode auto --max-file-size 500KB

# Only changed files vs main
promptpacker --changed

# Only files changed since a tag
promptpacker --since v1.0.0

# Only files different from a branch
promptpacker --branch develop

# Use a config profile
promptpacker --profile llm

# Override output file and skip preview
promptpacker --mode auto --output snapshot.md --force
```

---

## Exit Codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Error (printed to stderr) |
| Ctrl+C | Cancelled by user |
