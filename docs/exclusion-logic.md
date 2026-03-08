# Exclusion Logic

PromptPacker applies multiple layers of exclusion rules to decide which files to include. Understanding this helps you get exactly the output you want.

## Priority Order

Rules are applied in this order — the **first match wins**:

```
1. Always-excluded (self + output file)
2. .gitignore rules (most specific directory first)
3. Default ignore patterns (built-in)
4. Hidden files/directories (dot-prefixed names)
5. --exclude glob patterns (your custom overrides)
6. Extension filters (--include-ext / --exclude-ext)
7. File size limit (--max-file-size)
8. Git-changed filter (--changed / --since / --branch)
```

---

## 1. Always-Excluded

The PromptPacker executable itself and the output file (e.g. `output.md`) are always excluded, regardless of any other rules.

---

## 2. `.gitignore` Rules

PromptPacker reads `.gitignore` files hierarchically:

- It looks for `.gitignore` in the file's directory, then parent directories, up to the `--root`
- The **most specific** (deepest) matching rule wins
- Supports standard gitignore syntax: `*`, `?`, `**`, negation (`!`), directory markers (`/`), root anchors

**Example:** If `src/.gitignore` has `*.log` and the root `.gitignore` has `!debug.log`, PromptPacker will exclude `src/debug.log` (because the nested rule is more specific).

---

## 3. Default Ignore Patterns

If no `.gitignore` rule matched, PromptPacker checks a built-in list of common patterns:

**Build & dependency directories:**
`node_modules/`, `vendor/`, `bower_components/`, `dist/`, `build/`, `out/`, `target/`, `coverage/`, `.gradle/`, `[Bb]in/`, `[Oo]bj/`

**Python:**
`__pycache__/`, `*.py[cod]`, `.pytest_cache/`, `*.egg-info/`

**Java/JVM:**
`*.class`, `*.jar`, `*.war`, `*.ear`

**Ruby:**
`*.gem`, `.bundle/`

**Compiled binaries:**
`*.exe`, `*.dll`, `*.so`, `*.dylib`

**Temp & log files:**
`*.log`, `*.tmp`, `*.temp`, `*.cache`, `*.bak`, `*.swp`, `*~`

**IDE configs:**
`.idea/`, `.vscode/`, `*.sublime-project`, `*.sublime-workspace`

**Environment files:**
`.env`, `.env.*`, `.envrc` (but NOT `.env.example` or `.env.sample`)

**OS files:**
`.DS_Store`, `Thumbs.db`, `._*`

**Source control:**
`.git/`, `.svn/`, `.hg/`

**JavaScript:**
`npm-debug.log*`, `yarn-error.log*`, `.next/`, `.nuxt/`

**Infrastructure:**
`.terraform/`, `*.tfstate`, `*.tfstate.backup`

---

## 4. Hidden Files/Directories

Any file or directory whose **name starts with `.`** is excluded by default (e.g. `.DS_Store`, `.env`, `.github/`).

**Exceptions:**
- Does not apply if the `--root` directory itself starts with a dot
- If a `.gitignore` rule explicitly includes (`!`) the hidden file, it takes precedence

---

## 5. `--exclude` Patterns

After all the above, you can add custom glob patterns:

```bash
promptpacker --mode auto --exclude "*.generated.go,coverage/*,**/*.test.ts"
```

Patterns are matched against the relative path from `--root`.

---

## 6. Extension Filters

Applied after path-based exclusion:

- **`--include-ext go,ts,md`** — only include files with these extensions (directories are always traversed)
- **`--exclude-ext log,tmp,bak`** — exclude files with these extensions

```bash
# Only Go and Markdown
promptpacker --mode auto --include-ext go,md

# Exclude test artifacts
promptpacker --mode auto --exclude-ext test,snap,log
```

---

## 7. Max File Size

Files exceeding the threshold are silently skipped:

```bash
promptpacker --mode auto --max-file-size 1MB
promptpacker --mode auto --max-file-size 500KB
promptpacker --mode auto --max-file-size 100000  # bytes
```

Accepted suffixes: `KB`, `MB`, `GB`, `K`, `M`, `G`, or plain bytes.

---

## 8. Git-Changed Filter

When using `--changed`, `--since`, or `--branch`, only files that appear in the git diff are included:

```bash
# vs main branch
promptpacker --changed

# vs a tag
promptpacker --since v1.0.0

# vs another branch
promptpacker --branch feature/payments
```

If `git` is not installed or the directory is not a git repo, a warning is printed and all files are included normally.

---

## Combining Rules

All rules compose together. For example:

```bash
# Changed files vs main, Go only, skip files over 500KB
promptpacker --changed --include-ext go --max-file-size 500KB
```

This first applies the git filter, then the extension filter, then the size filter — giving you only changed Go files under 500KB.

---

## File-Spec Mode

In file-spec mode (`--mode file-spec`, the default), the **include paths** act as an *additional* first filter — only files under the specified paths are even considered. All the other exclusion rules still apply on top.

```bash
# Only consider files inside src/ and docs/, then apply all other rules
promptpacker src/ docs/
```
