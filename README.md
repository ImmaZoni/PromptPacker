# 🚀 PromptPacker 🚀

**Consolidate your code project into a single, navigable Markdown file – perfect for Large Language Models (LLMs).**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/immazoni/promptpacker)](https://golang.org/dl/)

`PromptPacker` is a simple, dependency-free Go utility that scans a project directory and generates a single Markdown file containing:

1.  A text-based representation of the project's file structure.
2.  The complete content of each included file, wrapped in appropriate Markdown code blocks with language hints.

This is particularly useful for:

*   Getting a holistic view of a codebase.
*   Sharing projects easily in a single text format.
*   Archiving project snapshots.
*   **Feeding entire local codebases into Large Language Models (LLMs)** for analysis, summarization, or review, without needing to upload individual files.

## Features

*   **Single File Output:** Creates one `.md` file summarizing the project.
*   **Project Structure Tree:** Generates an easy-to-read file tree at the beginning of the document.
*   **Code Concatenation:** Includes the full content of detected files.
*   **Syntax Highlighting Hints:** Adds language identifiers (e.g., `go`, `python`, `javascript`) to Markdown code blocks based on file extensions.
*   **`.gitignore` Support:** Intelligently parses `.gitignore` files (including nested ones) to exclude ignored files and directories, respecting standard rules like `*`, `?`, `**`, `!`, and directory markers (`/`).
*   **Built-in Default Ignores:** Automatically excludes common temporary files, build artifacts, dependency directories (like `node_modules`, `vendor`), IDE configuration (`.idea`, `.vscode`), environment files (`.env`), OS-specific files (`.DS_Store`), and more across various languages and frameworks.
*   **Custom Exclusions:** Allows specifying additional exclusion patterns via command-line flags.
*   **Configurable:** Set the root directory to scan, the output file path, and the number of processing workers.
*   **Dependency-Free:** Written in pure Go, requiring only the Go compiler/runtime.
*   **Concurrent Processing:** Reads and formats file contents concurrently for improved performance on multi-core systems.
*   **Memory Efficient:** Streams file content directly to the output file and processes files concurrently to handle large codebases without excessive memory usage.

## Installation

Recommended for most users: use the one-line installer for your OS, which downloads the latest release and puts `promptpacker` on your `PATH`. Manual binary install steps are provided as an alternative.

### 1. Windows (recommended)

**Quick install (preferred):**

From PowerShell (run as Administrator if you want to install for all users):

```powershell
irm https://raw.githubusercontent.com/immazoni/promptpacker/main/scripts/install-windows.ps1 | iex
```

After the script completes, open a new PowerShell or Command Prompt and verify:

```powershell
promptpacker --help
```

**Manual install (alternative):**

1. Go to the **[Releases page](https://github.com/immazoni/promptpacker/releases)**.
2. Download `promptpacker-windows-amd64.exe` from the latest release.
3. (Optional) Rename it to `promptpacker.exe`.
4. Move the file to a directory you want to use for CLI tools, e.g. `C:\Tools\promptpacker\promptpacker.exe`.
5. Add that directory to your `PATH`:
   - Open **Start → “Edit the system environment variables” → “Environment Variables…”**.
   - Edit the `Path` entry for your user and add `C:\Tools\promptpacker`.
6. Open a new PowerShell or Command Prompt and verify:
```powershell
promptpacker --help
```

### 2. macOS

**Quick install (preferred):**

```bash
curl -sSL https://raw.githubusercontent.com/immazoni/promptpacker/main/scripts/install.sh | bash
```

Then verify:

```bash
promptpacker --help
```

**Manual install (alternative):**

1. Go to the **[Releases page](https://github.com/immazoni/promptpacker/releases)**.
2. Download the binary for your architecture:
   - `promptpacker-darwin-amd64` (Intel)
   - `promptpacker-darwin-arm64` (Apple Silicon: M1/M2/M3)
3. Make it executable and move it into a directory on your `PATH`:
```bash
chmod +x ~/Downloads/promptpacker-darwin-*
sudo mv ~/Downloads/promptpacker-darwin-* /usr/local/bin/promptpacker
```
   - If you use Homebrew with `/opt/homebrew/bin`, you can move it there instead.
4. Verify:
```bash
promptpacker --help
```

### 3. Linux

**Quick install (preferred):**

```bash
curl -sSL https://raw.githubusercontent.com/immazoni/promptpacker/main/scripts/install.sh | bash
```

Then verify:

```bash
promptpacker --help
```

**Manual install (alternative):**

1. Go to the **[Releases page](https://github.com/immazoni/promptpacker/releases)**.
2. Download the binary matching your architecture:
   - `promptpacker-linux-amd64` (most x86_64 distros)
   - `promptpacker-linux-arm64` (ARM devices like Raspberry Pi 4+)
3. Make it executable and move it into a directory on your `PATH` (e.g. `/usr/local/bin`):
```bash
chmod +x ~/Downloads/promptpacker-linux-*
sudo mv ~/Downloads/promptpacker-linux-* /usr/local/bin/promptpacker
```
4. Verify:
```bash
promptpacker --help
```

### 4. Build from source (cross-platform, requires Go)

If you prefer to build the binary yourself:

```bash
git clone https://github.com/immazoni/promptpacker.git
cd promptpacker
go build -o promptpacker promptpacker.go
```

You can either:

- Run it from the build directory:

```bash
./promptpacker [options]
```

- Or move it into a directory on your `PATH` as shown in the OS-specific sections above so you can just run `promptpacker`.

### 5. Run directly with `go run` (no installation)

For quick, one-off use (requires Go):

```bash
git clone https://github.com/immazoni/promptpacker.git
cd promptpacker
go run promptpacker.go [options]
```

## Usage

```
go run promptpacker.go [options]
```

or, if compiled:

```
./promptpacker [options]
```

**(Run with `-h` or `--help` to see the formatted options list)**

**Options:**

*   `-root <path>`: Root directory of the project to scan. (Default: current directory)
*   `-output <path>`: Path for the output markdown file. (Default: `output.md`)
*   `-exclude <patterns>`: Comma-separated list of extra glob patterns to exclude (use '/' separators).
*   `-workers <int>`: Number of concurrent workers for processing file content. (Default: number of CPU cores)

**Examples:**

_(Use `go run promptpacker.go` or your compiled binary name like `./promptpacker` instead of `promptpacker` below)_

```bash
# Scan current directory, output to output.md using default workers
promptpacker

# Scan a specific project and save to a specific file
promptpacker --root /path/to/project --output /docs/project_summary.md

# Scan current directory, exclude *.log and build/ directory
promptpacker --exclude "*.log,build/*"

# Use only 4 workers for processing
promptpacker --workers 4

# Combine options
promptpacker --root ../my-app --output my-app.md --exclude "coverage/*,*.bak" --workers 8
```

## Exclusion Logic

Files and directories are excluded based on the following order of precedence (the first rule that matches and dictates exclusion/inclusion wins):

1.  **Executable/Output Skip:** The running `PromptPacker` executable itself and the specified `--output` file are always excluded.
2.  **`.gitignore` Hierarchy:** Rules from `.gitignore` files are checked, starting from the directory containing the item and moving up towards the `--root`.
    *   The rule from the *most specific* (deepest) `.gitignore` file that matches the item takes precedence.
    *   Supports standard patterns (`*`, `?`, `**`), directory markers (`/`), root anchors (`/`), and negation (`!`).
    *   If a `.gitignore` rule (positive or negative) matches, that decision is final regarding `.gitignore` rules, and processing moves to the next item (if excluded) or continues to default ignores (if included by `!`).
3.  **Default Ignore Patterns:** If no `.gitignore` rule explicitly included or excluded the item, a built-in list of common patterns (e.g., `node_modules/`, `*.log`, `.env`, `.idea/`) is checked. If a positive default pattern matches, the item is excluded. (See code for the full list).
4.  **Hidden Files/Directories:** If the item was not excluded by the above rules, and its name starts with a dot (`.`), it is excluded (e.g., `.git/`, `.DS_Store`). This check does not apply if the `--root` directory itself starts with a dot, or if a `.gitignore` rule explicitly included (`!`) the hidden item.
5.  **Custom `--exclude` Patterns:** Finally, if the item has not been excluded yet, the patterns provided via the `--exclude` flag are checked against the item's relative path.

**In short:** Your `.gitignore` files have the highest priority. The default rules catch common clutter. Hidden files are generally ignored. `--exclude` provides final custom overrides.

## Example Output (`output.md`)
    ```markdown
    # Project Structure

    ```
    /src
    - main.go
    - /utils
    -- helpers.go
    go.mod
    README.md
    ```

    # File Contents

    ## src/main.go

    ```go
    package main

    import (
        "fmt"
        "project/src/utils"
    )

    func main() {
        message := utils.GetGreeting()
        fmt.Println(message)
    }

    ```

    ## src/utils/helpers.go

    ```go
    package utils

    // GetGreeting returns a simple greeting string.
    func GetGreeting() string {
        return "Hello from PromptPacker!"
    }

    ```

    ## go.mod

    ```go.mod
    module project

    go 1.20

    ```

    ## README.md

    ```markdown
    # My Project

    This is a sample project.

    ```


## Limitations

*   **`.gitignore` Parsing:** The built-in parser aims for compatibility but might differ from native `git` behavior in some complex edge cases (e.g., intricate combinations of `**`, negations, and escaped characters).
*   **Language Detection:** Relies solely on file extensions. It won't detect languages for files without extensions or use heuristics/shebangs. `.gitattributes` are not used.
*   **Performance:** While concurrent and memory-efficient for file *content*, the initial directory walk and metadata collection phase still requires memory proportional to the *number* of files in the project. Extremely large repositories (millions of files) might still consume significant memory during this initial scan.

## Contributing

Contributions are welcome! Please feel free to submit pull requests or open issues for bugs, feature requests, or improvements.

## License

This project is licensed under the [MIT License](LICENSE.md).
