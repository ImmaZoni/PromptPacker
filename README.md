# 🚀 PromptPacker

**Consolidate your code project into a single, LLM-ready Markdown file.**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/immazoni/promptpacker)](https://golang.org/dl/)

PromptPacker scans your project and generates a single Markdown file containing your directory structure and file contents — ready to paste into ChatGPT, Claude, Gemini, or any other LLM.

## Features

- **Interactive TUI** — Beautiful terminal UI for selecting files when no args are given
- **File-Spec Mode** — Explicitly choose which files/directories to include
- **Auto Mode** — Scan the entire directory tree automatically (`--mode auto`)
- **Structure-Only** — Output just the directory tree, no file contents (`--structure-only`)
- **Preview Before Output** — See what will be included before writing the file
- **Extension Filtering** — Include or exclude by file extension (`--include-ext`, `--exclude-ext`)
- **Git-Aware** — Only include changed files (`--changed`, `--since`, `--branch`)
- **Config File** — Save defaults per-project in `.promptpacker.yml`, with named profiles
- **Max File Size** — Automatically skip files above a size threshold (`--max-file-size`)
- **`.gitignore` Support** — Respects your gitignore rules out of the box
- **Fast** — Concurrent processing with worker pool, streams to disk

## Documentation

| Guide | Description |
|---|---|
| [Getting Started](docs/getting-started.md) | Installation + first run |
| [CLI Reference](docs/cli-reference.md) | All flags and options |
| [Config File Guide](docs/config-file.md) | `.promptpacker.yml` format and profiles |
| [Examples & Recipes](docs/examples.md) | Common real-world use cases |
| [Exclusion Logic](docs/exclusion-logic.md) | How file filtering works |

## Quick Start

```bash
# Interactive file selector (default — pick what to include)
promptpacker

# Include specific files/dirs
promptpacker src/ cmd/ go.mod README.md

# Scan entire directory automatically
promptpacker --mode auto

# Only output the directory tree
promptpacker --mode auto --structure-only

# Skip preview and generate immediately
promptpacker src/ --force
```

## Installation

### Windows

```powershell
irm https://raw.githubusercontent.com/immazoni/promptpacker/main/scripts/install-windows.ps1 | iex
```

### macOS / Linux

```bash
curl -sSL https://raw.githubusercontent.com/immazoni/promptpacker/main/scripts/install.sh | bash
```

### Build from source

```bash
git clone https://github.com/immazoni/promptpacker.git
cd promptpacker
go build -o promptpacker promptpacker.go
```

### Running Tests

```bash
go test -v .
```

See [Getting Started](docs/getting-started.md) for full installation instructions.

## License

[MIT License](LICENSE)
