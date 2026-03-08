# Examples & Recipes

A collection of common real-world uses of PromptPacker.

---

## Prepare a Full Project for an LLM

```bash
# Scan everything, output to llm-context.md
promptpacker --mode auto --output llm-context.md --force
```

Then open `llm-context.md` and paste it into your LLM of choice.

**With a config profile:**

```yaml
# .promptpacker.yml
profiles:
  llm:
    mode: auto
    output: llm-context.md
    exclude-ext: log,tmp,bak,min.js,min.css,map
    max-file-size: 500KB
    force: true
```

```bash
promptpacker --profile llm
```

---

## Include Only Specific Files

```bash
# Positional args
promptpacker src/ cmd/ go.mod README.md

# Or with --include flag
promptpacker --include src/,cmd/,go.mod,README.md
```

---

## Interactive File Picker

Just run `promptpacker` with no arguments in your project directory. The TUI will appear:

- **↑/↓** or **j/k** — navigate
- **Space** — toggle selection
- **Enter** — navigate into directory / confirm selection
- **←/h** — go up to parent directory
- **a** — select all, **d** — deselect all
- **q** / **Ctrl+C** — quit without generating

---

## Only Include Backend or Infrastructure Files

```bash
promptpacker src/backend/ infrastructure/ docker-compose.yml Makefile

# Or in auto mode with extension filtering
promptpacker --mode auto --include-ext go,py,sh,yml,yaml,tf --exclude-ext ts,tsx,jsx,css
```

---

## Generate a Structure-Only Overview

Useful for huge repos — shows the tree without any file contents.

```bash
# Full tree
promptpacker --mode auto --structure-only

# Limit to top 3 levels, include file sizes
promptpacker --mode auto --structure-only --max-depth 3 --show-sizes

# Save to a specific file
promptpacker --mode auto --structure-only --output structure.md
```

---

## Create a PR Summary (Git-Changed Files)

Only include files that differ from main — perfect for reviewing a feature branch.

```bash
# Changed vs main (default)
promptpacker --changed

# Changed vs a specific branch
promptpacker --branch develop

# Changed since a tag
promptpacker --since v1.2.0

# Changed since a commit hash
promptpacker --since abc1234
```

---

## Skip Large Files

Useful for projects with generated files, binaries, or large datasets.

```bash
# Skip anything over 1MB
promptpacker --mode auto --max-file-size 1MB

# Skip anything over 500KB, also exclude logs
promptpacker --mode auto --max-file-size 500KB --exclude-ext log,csv
```

---

## Filter by File Extension

```bash
# Only Go and Markdown
promptpacker --mode auto --include-ext go,md

# Everything except logs and temp files
promptpacker --mode auto --exclude-ext log,tmp,bak,swp

# Only TypeScript/React files
promptpacker src/ --include-ext ts,tsx
```

---

## Documentation Snapshot

```bash
# Only Markdown files
promptpacker --mode auto --include-ext md --output docs-snapshot.md

# Or with an interactive pick of just the docs/ folder
promptpacker docs/
```

---

## Monorepo — Per-Service Snapshots

```bash
# Just the API service
promptpacker services/api/ shared/

# Just the frontend
promptpacker apps/frontend/src/ apps/frontend/package.json

# Everything but keep it small
promptpacker --mode auto --max-depth 4 --max-file-size 200KB --exclude-ext map,min.js,min.css
```

---

## Save Defaults in Config

Put a `.promptpacker.yml` in your repo root so your team always gets consistent behavior:

```yaml
defaults:
  mode: auto
  exclude-ext: log,tmp,bak,swp

profiles:
  llm:
    output: llm-context.md
    max-file-size: 1MB
  
  structure:
    structure-only: true
    max-depth: 3
    output: structure.md
```

```bash
# Team member runs
promptpacker --profile llm    # → llm-context.md with size limits
promptpacker --profile structure  # → structure.md tree overview
```

---

## Tips

- Always review the **preview** before generating — it lists exactly what will be included
- Use `--force` in scripts to skip the confirmation prompt
- Combine `--changed` with `--structure-only` for a quick "what changed" tree
- Use `--output` to give your snapshot a meaningful name, e.g. `feature-auth.md`
