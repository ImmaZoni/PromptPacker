# Config File Guide

PromptPacker supports a per-project config file (`.promptpacker.yml` or `.promptpacker.yaml`) placed in your project root. It lets you set defaults so you don't need to type flags every time.

## Quick Example

Create `.promptpacker.yml` in your project root:

```yaml
defaults:
  mode: auto
  output: llm-snapshot.md
  exclude-ext: log,tmp,bak,swp

profiles:
  llm:
    mode: auto
    exclude-ext: log,tmp,bak,swp,min.js,min.css
    exclude: node_modules/,dist/,build/

  structure:
    mode: auto
    structure-only: true
    max-depth: 3

  backend:
    mode: file-spec
    include: src/,cmd/,go.mod,go.sum
    exclude-ext: ts,tsx,jsx
```

Then run:

```bash
# Use defaults
promptpacker

# Use a named profile
promptpacker --profile llm
promptpacker --profile structure
promptpacker --profile backend
```

---

## `defaults:` Section

The `defaults:` block sets the fallback values used when no CLI flags override them.

| Key | Type | Description |
|---|---|---|
| `mode` | string | `file-spec` or `auto` |
| `output` | string | Output file path (e.g. `snapshot.md`) |
| `structure-only` | bool | `true` to output structure only |
| `workers` | int | Number of concurrent workers |
| `max-depth` | int | Maximum directory depth (0 = unlimited) |
| `max-file-size` | string | Skip files larger than this (e.g. `1MB`) |
| `force` | bool | `true` to skip the preview prompt |
| `include` | string | Comma-separated paths to include |
| `exclude` | string | Comma-separated glob patterns to exclude |
| `include-ext` | string | Comma-separated extensions to include |
| `exclude-ext` | string | Comma-separated extensions to exclude |

---

## `profiles:` Section

Profiles let you define multiple named configurations. Each profile has the same keys as `defaults:`. When you activate a profile with `--profile <name>`, its values are merged on top of the defaults (profile wins over defaults, but CLI flags always win over everything).

```yaml
profiles:
  myprofile:
    mode: auto
    output: myprofile-output.md
    exclude-ext: log,tmp
```

```bash
promptpacker --profile myprofile
```

---

## Priority Order

When the same setting is specified in multiple places, this is the priority (highest wins):

```
CLI flags  >  Profile  >  Defaults section  >  Built-in defaults
```

---

## Full Example

```yaml
# .promptpacker.yml

defaults:
  mode: file-spec
  output: output.md
  workers: 4
  exclude-ext: log,tmp,bak,swp
  max-depth: 0
  force: false

profiles:

  # Great for feeding an entire project to an LLM
  llm:
    mode: auto
    output: llm-context.md
    exclude-ext: log,tmp,bak,swp,min.js,min.css,map
    exclude: node_modules/,dist/,build/,.cache/
    max-file-size: 1MB

  # Quick architecture overview
  structure:
    mode: auto
    structure-only: true
    max-depth: 3
    output: structure.md

  # Backend code only
  backend:
    mode: file-spec
    include: src/,cmd/,internal/,go.mod,go.sum
    exclude-ext: ts,tsx,jsx,css,html

  # Documentation only
  docs:
    mode: file-spec
    include-ext: md
    output: docs-snapshot.md
```

---

## Notes

- If `.promptpacker.yml` is not found, PromptPacker uses built-in defaults with no error
- The config file is silently ignored if it can't be parsed (a warning is printed)
- Both `.promptpacker.yml` and `.promptpacker.yaml` are supported (`.yml` takes priority)
- You can commit `.promptpacker.yml` to share consistent settings with your team
