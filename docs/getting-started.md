# Getting Started

Welcome to **PromptPacker** — the fastest way to feed your entire codebase to an LLM.

## What It Does

PromptPacker scans your project directory and outputs a single Markdown file containing:

1. A text-based directory structure tree
2. The full contents of every included file, in syntax-highlighted code blocks

You copy that file into ChatGPT, Claude, Gemini, or any LLM context and ask questions about your entire codebase at once.

## Installation

### Windows (recommended)

Run this in **PowerShell**:

```powershell
irm https://raw.githubusercontent.com/immazoni/promptpacker/main/scripts/install-windows.ps1 | iex
```

Then open a new terminal and verify:

```powershell
promptpacker --help
```

**Manual alternative:**

1. Download `promptpacker-windows-amd64.exe` from the [Releases page](https://github.com/immazoni/promptpacker/releases)
2. Rename it to `promptpacker.exe` and move it to a folder like `C:\Tools\`
3. Add that folder to your `PATH` (Start → "Edit system environment variables")
4. Open a new terminal and run `promptpacker --help`

---

### macOS / Linux (recommended)

```bash
curl -sSL https://raw.githubusercontent.com/immazoni/promptpacker/main/scripts/install.sh | bash
```

Then verify:

```bash
promptpacker --help
```

**Manual alternative:**

1. Download the binary for your platform from the [Releases page](https://github.com/immazoni/promptpacker/releases):
   - macOS Intel: `promptpacker-darwin-amd64`
   - macOS Apple Silicon: `promptpacker-darwin-arm64`
   - Linux x86_64: `promptpacker-linux-amd64`
   - Linux ARM: `promptpacker-linux-arm64`
2. Make it executable and move it to your PATH:
   ```bash
   chmod +x ~/Downloads/promptpacker-*
   sudo mv ~/Downloads/promptpacker-* /usr/local/bin/promptpacker
   ```

---

### Build from Source (requires Go)

```bash
git clone https://github.com/immazoni/promptpacker.git
cd promptpacker
go build -o promptpacker promptpacker.go
```

### Running Tests

```bash
go test -v .
```

---

## Your First Run

Navigate to your project directory and run:

```bash
cd /path/to/your/project
promptpacker
```

This opens the **interactive file selector** — a TUI where you navigate your project and pick what to include:

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🚀 PromptPacker v0.2
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📁 /my-project  [0 selected]

  /src
  /tests
  go.mod
  go.sum
  README.md

↑/↓ or j/k: navigate  Space: toggle  Enter: navigate dir / confirm  a: all  d: none  q: quit
```

- Use **↑/↓** or **j/k** to navigate
- Press **Space** to toggle selection
- Press **Enter** on a directory to navigate into it, or to confirm when files are selected
- Press **←** or **h** to go back to the parent directory
- Press **a** to select all, **d** to deselect all
- Press **q** or **Ctrl+C** to quit

After confirming, PromptPacker shows a preview and asks you to confirm before writing `output.md`.

---

## Common Patterns

```bash
# Pick files interactively (default)
promptpacker

# Include specific files/dirs directly
promptpacker src/ go.mod README.md

# Scan the whole project automatically
promptpacker --mode auto

# Just the directory tree, no file contents
promptpacker --mode auto --structure-only

# Skip the preview prompt and generate immediately
promptpacker src/ --force
```

---

## Next Steps

- [CLI Reference](cli-reference.md) — full list of flags
- [Config File Guide](config-file.md) — set project-level defaults
- [Examples & Recipes](examples.md) — common real-world patterns
