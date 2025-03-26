package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
)

const defaultOutputFile = "output.md"
const gitignoreFilename = ".gitignore"

var executablePath string

var defaultIgnorePatterns = []string{
	"*.log", "*.tmp", "*.temp", "*.cache", "*.bak", "*.swp", "*.swo", "*~", "._*",
	"npm-debug.log*", "yarn-error.log*", "hs_err_pid*", ".idea/", ".vscode/",
	"*.sublime-project", "*.sublime-workspace", ".project", ".classpath", ".settings/",
	"*.komodoproject", ".komodocfg/", "node_modules/", "bower_components/", "vendor/",
	"dist/", "build/", "out/", "target/", "coverage/", ".gradle/", "[Bb]in/", "[Oo]bj/",
	"__pycache__/", "*.py[cod]", "*$py.class", ".pytest_cache/", "*.egg-info/", "*.egg",
	"*.class", "*.jar", "*.war", "*.ear", "*.gem", ".bundle/", "*.exe", "*.dll", "*.so",
	"*.dylib", "*_test", ".next/", ".nuxt/", "instance/", ".env", ".env.*",
	"!.env.example", "!.env.sample", ".envrc", ".DS_Store", "Thumbs.db", ".terraform/",
	"*.tfstate", "*.tfstate.backup", "venv/", ".venv/", "env/", "ENV/", ".env/",
	".direnv/", ".git/", ".svn/", ".hg/",
}

const (
	logPrefixInfo = "[INFO] "
	logPrefixWarn = "[WARN] "
	logPrefixErr  = "[ERR]  "
	logPrefixDone = "[DONE] "
)

func logInfo(format string, v ...interface{}) {
	fmt.Printf(logPrefixInfo+format+"\n", v...)
}

func logWarn(format string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, logPrefixWarn+format+"\n", v...)
}

func logError(format string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, logPrefixErr+format+"\n", v...)
}

func logFatal(format string, v ...interface{}) {
	log.Fatalf(logPrefixErr+format+"\n", v...)
}

func checkDefaultIgnores(relPath string, isDir bool) bool {
	relPath = filepath.ToSlash(relPath)
	baseName := ""
	if idx := strings.LastIndex(relPath, "/"); idx != -1 {
		baseName = relPath[idx+1:]
	} else {
		baseName = relPath
	}
	for _, pattern := range defaultIgnorePatterns {
		isDirPattern := strings.HasSuffix(pattern, "/")
		matchPattern := pattern
		if isDirPattern {
			matchPattern = strings.TrimSuffix(pattern, "/")
		}
		isNegated := strings.HasPrefix(matchPattern, "!")
		if isNegated {
			matchPattern = matchPattern[1:]
		}
		var matched bool
		if strings.Contains(matchPattern, "/") {
			matched, _ = filepath.Match(matchPattern, relPath)
		} else {
			matched, _ = filepath.Match(matchPattern, baseName)
		}
		if matched {
			if isDirPattern && !isDir {
				continue
			}
			if !isNegated {
				return true
			}
		}
	}
	return false
}

type gitignoreRule struct {
	pattern       string
	patternParts  []string
	isNegated     bool
	matchDirsOnly bool
	isRooted      bool
	baseDir       string
}

var gitignoreCache = make(map[string][]gitignoreRule)
var cacheMutex sync.RWMutex
var gitignoreLoadAttempt = make(map[string]bool)

func loadAndCacheGitignore(absDir string) ([]gitignoreRule, bool) {
	cacheMutex.RLock()
	rules, found := gitignoreCache[absDir]
	loadAttempted := gitignoreLoadAttempt[absDir]
	cacheMutex.RUnlock()
	if found || loadAttempted {
		return rules, found
	}
	absDir = filepath.Clean(absDir)
	gitignorePath := filepath.Join(absDir, gitignoreFilename)
	var loadedRules []gitignoreRule
	var loadError error
	found = false
	file, err := os.Open(gitignorePath)
	if err != nil {
		if !os.IsNotExist(err) {
			loadError = fmt.Errorf("error opening %s: %w", gitignorePath, err)
		}
	} else {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			rule := gitignoreRule{baseDir: absDir, pattern: line}
			if strings.HasPrefix(line, "!") {
				rule.isNegated = true
				line = line[1:]
				if strings.HasPrefix(line, `\`) {
					rule.isNegated = false
					line = line[1:]
				} else if line == "" {
					continue
				}
			}
			if strings.HasPrefix(line, `\#`) {
				line = line[1:]
			} else if strings.HasPrefix(line, "#") {
				continue
			}
			line = strings.TrimRight(line, " ")
			if line == "" {
				continue
			}
			if strings.HasSuffix(line, "/") {
				rule.matchDirsOnly = true
				line = line[:len(line)-1]
			}
			if strings.HasPrefix(line, "/") {
				rule.isRooted = true
				line = line[1:]
			}
			if line == "" {
				continue
			}
			rule.patternParts = strings.Split(line, "/")
			cleanedParts := []string{}
			for _, p := range rule.patternParts {
				if p != "" {
					cleanedParts = append(cleanedParts, p)
				}
			}
			if line == "**" && len(cleanedParts) == 0 {
				rule.patternParts = []string{"**"}
			} else {
				rule.patternParts = cleanedParts
			}
			if len(rule.patternParts) == 0 {
				continue
			}
			loadedRules = append(loadedRules, rule)
		}
		if err := scanner.Err(); err != nil {
			loadError = fmt.Errorf("error reading %s: %w", gitignorePath, err)
		}
		if loadError == nil {
			found = true
		}
	}
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	if loadError != nil {
		logWarn("%v", loadError)
	}
	if found {
		gitignoreCache[absDir] = loadedRules
	}
	gitignoreLoadAttempt[absDir] = true
	return loadedRules, found
}
func match(patternParts, pathParts []string) bool {
	patLen, pathLen := len(patternParts), len(pathParts)
	patIdx, pathIdx := 0, 0
	for patIdx < patLen || pathIdx < pathLen {
		if patIdx == patLen {
			return pathIdx == pathLen
		}
		if pathIdx == pathLen {
			return patIdx == patLen-1 && patternParts[patIdx] == "**"
		}
		p := patternParts[patIdx]
		segment := pathParts[pathIdx]
		if p == "**" {
			if patIdx == patLen-1 {
				return true
			}
			if match(patternParts[patIdx+1:], pathParts[pathIdx:]) {
				return true
			}
			pathIdx++
			continue
		}
		matched, _ := filepath.Match(p, segment)
		if !matched {
			return false
		}
		patIdx++
		pathIdx++
	}
	return patIdx == patLen && pathIdx == pathLen
}
func checkIgnoreRules(relativePath string, isDir bool, rules []gitignoreRule) (ignored bool, matched bool) {
	ignored, matched = false, false
	relativePath = filepath.ToSlash(relativePath)
	pathParts := strings.Split(relativePath, "/")
	cleanedPathParts := []string{}
	for _, p := range pathParts {
		if p != "" {
			cleanedPathParts = append(cleanedPathParts, p)
		}
	}
	pathParts = cleanedPathParts
	baseName := ""
	if len(pathParts) > 0 {
		baseName = pathParts[len(pathParts)-1]
	}
	for _, rule := range rules {
		ruleMatches := false
		if !rule.isRooted && !strings.Contains(rule.pattern, "/") && len(rule.patternParts) == 1 && baseName != "" {
			ruleMatches, _ = filepath.Match(rule.patternParts[0], baseName)
		}
		if !ruleMatches {
			ruleMatches = match(rule.patternParts, pathParts)
		}
		if ruleMatches {
			if rule.matchDirsOnly && !isDir {
				continue
			}
			ignored = !rule.isNegated
			matched = true
		}
	}
	return ignored, matched
}
func shouldIgnoreHierarchical(absPath string, isDir bool, rootDir string) (ignored bool, decided bool) {
	finalIgnored, matchedRuleLevel := false, -1
	currentDir := filepath.Clean(absPath)
	if !isDir {
		currentDir = filepath.Dir(currentDir)
	}
	level := 0
	for {
		if !strings.HasPrefix(currentDir, rootDir) && currentDir != rootDir {
			break
		}
		rules, found := loadAndCacheGitignore(currentDir)
		if found {
			pathRelativeToRuleDir, err := filepath.Rel(currentDir, absPath)
			if err == nil {
				levelIgnored, levelMatched := checkIgnoreRules(pathRelativeToRuleDir, isDir, rules)
				if levelMatched && matchedRuleLevel == -1 {
					finalIgnored = levelIgnored
					matchedRuleLevel = level
					break
				}
			} else {
				logWarn("Could not get relative path %s to %s: %v", absPath, currentDir, err)
			}
		}
		if currentDir == rootDir {
			break
		}
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break
		}
		currentDir = parentDir
		level++
	}
	return finalIgnored, matchedRuleLevel != -1
}

type walkEntry struct {
	relPath  string
	fullPath string
	isDir    bool
	depth    int
}
type config struct {
	rootDir         string
	outputFile      string
	excludePatterns []string
	numWorkers      int
}
type fileTask struct{ entry walkEntry }
type fileResult struct {
	relPath string
	content string
	err     error
}

func main() {
	var execErr error
	executablePath, execErr = os.Executable()
	if execErr != nil {
		logWarn("Could not determine executable path: %v. Self-exclusion might fail.", execErr)
		executablePath = ""
	} else {
		executablePath, execErr = filepath.Abs(executablePath)
		if execErr != nil {
			logWarn("Could not determine absolute executable path: %v. Self-exclusion might fail.", execErr)
			executablePath = ""
		}
	}

	setupUsage()
	cfg := parseFlags()

	fmt.Println("------------------------------------")
	fmt.Println("       🚀 PromptPacker v0.1 🚀      ")
	fmt.Println("------------------------------------")
	logInfo("Scanning directory: %s", cfg.rootDir)
	logInfo("Outputting to: %s", cfg.outputFile)
	logInfo("Using %d workers for content processing.", cfg.numWorkers)
	if len(cfg.excludePatterns) > 0 {
		logInfo("Excluding patterns (custom): %v", cfg.excludePatterns)
	}

	loadAndCacheGitignore(cfg.rootDir)

	logInfo("Phase 1: Walking directory structure...")
	var entries []walkEntry
	walkErr := filepath.WalkDir(cfg.rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			logWarn("Error accessing path %q: %v", path, err)
			return nil
		}
		absPath, err := filepath.Abs(path)
		if err != nil {
			logWarn("Could not get absolute path for %q: %v", path, err)
			return nil
		}
		relPath, err := filepath.Rel(cfg.rootDir, absPath)
		if err != nil {
			logWarn("Could not get relative path for %q: %v", absPath, err)
			return nil
		}
		relPath = filepath.ToSlash(relPath)
		if relPath == "." {
			return nil
		}
		isDir := d.IsDir()
		baseName := filepath.Base(absPath)

		if executablePath != "" && absPath == executablePath {
			return nil
		}
		if absPath == cfg.outputFile {
			return nil
		}
		gitignoreIgnored, gitignoreDecided := shouldIgnoreHierarchical(absPath, isDir, cfg.rootDir)
		if gitignoreDecided && gitignoreIgnored {
			if isDir {
				return filepath.SkipDir
			}
			return nil
		}
		if !gitignoreDecided {
			if checkDefaultIgnores(relPath, isDir) {
				if isDir {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if !gitignoreDecided {
			isRootItselfHidden := strings.HasPrefix(filepath.Base(cfg.rootDir), ".")
			if strings.HasPrefix(baseName, ".") && baseName != "." && baseName != ".." {
				if !(isRootItselfHidden && absPath == cfg.rootDir) {
					if isDir {
						return filepath.SkipDir
					}
					return nil
				}
			}
		}
		for _, pattern := range cfg.excludePatterns {
			matched, _ := filepath.Match(pattern, relPath)
			if matched {
				if isDir {
					return filepath.SkipDir
				}
				return nil
			}
		}

		depth := strings.Count(relPath, "/")
		entries = append(entries, walkEntry{relPath: relPath, fullPath: absPath, isDir: isDir, depth: depth})
		return nil
	})
	if walkErr != nil {
		logFatal("Error walking directory %q: %v", cfg.rootDir, walkErr)
	}
	logInfo("Phase 1: Found %d filesystem entries to process.", len(entries))

	sortEntries(entries)

	outFile, err := os.Create(cfg.outputFile)
	if err != nil {
		logFatal("Error creating output file %q: %v", cfg.outputFile, err)
	}
	defer outFile.Close()
	writer := bufio.NewWriter(outFile)

	logInfo("Phase 2: Writing project structure...")
	writeStructure(writer, entries)

	logInfo("Phase 3: Processing file contents...")
	_, err = writer.WriteString("# File Contents\n\n")
	if err != nil {
		logFatal("Error writing content header: %v", err)
	}

	tasks := make(chan fileTask, len(entries))
	results := make(chan fileResult, len(entries))
	processedContent := make(map[string]fileResult)
	var wg sync.WaitGroup

	logInfo("Starting %d workers...", cfg.numWorkers)
	for i := 0; i < cfg.numWorkers; i++ {
		wg.Add(1)
		go worker(&wg, tasks, results)
	}

	numFileTasks := 0
	for _, entry := range entries {
		if !entry.isDir {
			tasks <- fileTask{entry: entry}
			numFileTasks++
		}
	}
	close(tasks)
	logInfo("Distributed %d file processing tasks.", numFileTasks)

	var collectWg sync.WaitGroup
	collectWg.Add(1)
	go func() {
		defer collectWg.Done()
		for result := range results {
			processedContent[result.relPath] = result
		}
	}()

	logInfo("Waiting for workers to finish...")
	wg.Wait()
	close(results)
	logInfo("Collecting final results...")
	collectWg.Wait()
	logInfo("All processing complete.")

	logInfo("Phase 4: Writing file contents to output...")
	writeErrors := 0
	for _, entry := range entries {
		if !entry.isDir {
			result, found := processedContent[entry.relPath]
			if !found {
				logError("Result not found for file %s", entry.relPath)
				errMsg := fmt.Sprintf("## %s\n\n```\nError: Processed content not found.\n```\n\n", entry.relPath)
				_, writeErr := writer.WriteString(errMsg)
				if writeErr != nil {
					logError("Error writing missing content message for %s: %v", entry.relPath, writeErr)
					writeErrors++
				}
				continue
			}
			_, writeErr := writer.WriteString(result.content)
			if writeErr != nil {
				logError("Error writing content for %s: %v", entry.relPath, writeErr)
				writeErrors++
				fallbackErr := fmt.Sprintf("## %s\n\n```\nError: Failed to write processed content to output file.\n```\n\n", entry.relPath)
				_, _ = writer.WriteString(fallbackErr)
			}
		}
	}

	logInfo("Flushing output buffer...")
	err = writer.Flush()
	if err != nil {
		logFatal("Error flushing output buffer: %v", err)
	}

	fmt.Println("------------------------------------")
	if writeErrors > 0 {
		logWarn("Completed with %d content writing errors.", writeErrors)
		fmt.Printf(logPrefixDone+"Created %s (with errors noted above).\n", cfg.outputFile)
	} else {
		fmt.Printf(logPrefixDone+"Successfully created %s\n", cfg.outputFile)
	}
	fmt.Println("------------------------------------")
}

func worker(wg *sync.WaitGroup, tasks <-chan fileTask, results chan<- fileResult) {
	defer wg.Done()
	for task := range tasks {
		formattedContent, err := processFileContent(task.entry)
		results <- fileResult{relPath: task.entry.relPath, content: formattedContent, err: err}
	}
}

func processFileContent(entry walkEntry) (string, error) {
	var buf bytes.Buffer
	header := fmt.Sprintf("## %s\n\n", entry.relPath)
	buf.WriteString(header)
	langBaseName := entry.relPath
	if idx := strings.LastIndex(entry.relPath, "/"); idx != -1 {
		langBaseName = entry.relPath[idx+1:]
	}
	lang := getLanguageHint(langBaseName)
	fenceOpen := fmt.Sprintf("```%s\n", lang)
	buf.WriteString(fenceOpen)
	file, err := os.Open(entry.fullPath)
	if err != nil {
		errorMsg := fmt.Sprintf("Error reading file: %v\n", err)
		buf.WriteString(errorMsg)
	} else {
		defer file.Close()
		_, copyErr := io.Copy(&buf, file)
		if copyErr != nil {
			buf.WriteString(fmt.Sprintf("\n\nError copying file content: %v\n", copyErr))
			err = copyErr
		}
	}
	buf.WriteRune('\n')
	buf.WriteString("```\n\n")
	return buf.String(), err
}

func parseFlags() config {
	var cfg config
	var excludeList string
	defaultRoot, err := os.Getwd()
	if err != nil {
		logWarn("Could not get current directory: %v. Using '.'", err)
		defaultRoot = "."
	}
	defaultWorkers := runtime.NumCPU()
	if defaultWorkers < 1 {
		defaultWorkers = 1
	}

	rootDirPtr := flag.String("root", defaultRoot, "Root directory of the project to scan.")
	outputFilePtr := flag.String("output", defaultOutputFile, "Path for the output markdown file.")
	excludeListPtr := flag.String("exclude", "", "Comma-separated list of extra glob patterns to exclude (use '/' separators).")
	numWorkersPtr := flag.Int("workers", defaultWorkers, "Number of concurrent workers for processing file content.")

	flag.Parse()

	cfg.rootDir = *rootDirPtr
	cfg.outputFile = *outputFilePtr
	excludeList = *excludeListPtr
	cfg.numWorkers = *numWorkersPtr

	cfg.rootDir, err = filepath.Abs(cfg.rootDir)
	if err != nil {
		logFatal("Error resolving absolute path for root directory '%s': %v", cfg.rootDir, err)
	}
	cfg.outputFile, err = filepath.Abs(cfg.outputFile)
	if err != nil {
		logFatal("Error resolving absolute path for output file '%s': %v", cfg.outputFile, err)
	}
	if cfg.numWorkers < 1 {
		cfg.numWorkers = 1
	}
	if excludeList != "" {
		rawPatterns := strings.Split(excludeList, ",")
		for _, p := range rawPatterns {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				cfg.excludePatterns = append(cfg.excludePatterns, trimmed)
			}
		}
	}
	return cfg
}

func setupUsage() {
	flag.Usage = func() {
		invocationName := filepath.Base(os.Args[0])

		fmt.Println("------------------------------------")
		fmt.Println("       🚀 PromptPacker v0.1 🚀      ")
		fmt.Println("------------------------------------")
		fmt.Fprintf(os.Stderr, "Consolidates a code project into a single Markdown file, suitable for LLMs.\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s [options]\n", invocationName)
		fmt.Fprintf(os.Stderr, "  go run PromptPacker.go [options]  (if running source directly)\n\n")

		fmt.Fprintf(os.Stderr, "Options:\n")

		w := tabwriter.NewWriter(os.Stderr, 0, 4, 2, ' ', 0)
		flag.VisitAll(func(f *flag.Flag) {
			var flagLine string
			flagName, usage := flag.UnquoteUsage(f)
			flagLine = fmt.Sprintf("  -%s %s\t%s", f.Name, flagName, usage)
			if f.DefValue != "" {
				if f.Name == "root" && f.DefValue == "." {
					flagLine += " (Default: current directory)"
				} else if f.Name == "workers" {
					defaultWorkers := runtime.NumCPU()
					if defaultWorkers < 1 {
						defaultWorkers = 1
					}
					if f.DefValue == fmt.Sprintf("%d", defaultWorkers) {
						flagLine += fmt.Sprintf(" (Default: %d - num CPU cores)", defaultWorkers)
					} else {
						flagLine += fmt.Sprintf(" (Default: %s)", f.DefValue)
					}
				} else {
					flagLine += fmt.Sprintf(" (Default: %s)", f.DefValue)
				}
			}
			fmt.Fprintln(w, flagLine)
		})
		w.Flush()

		fmt.Fprintf(os.Stderr, "\nExclusion Logic:\n")
		fmt.Fprintf(os.Stderr, "  Files are excluded based on: .gitignore rules > Default ignores > Hidden files > --exclude patterns.\n")
		fmt.Fprintf(os.Stderr, "  See README for full details on default ignores.\n")

		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  (Use 'go run PromptPacker.go' or your compiled binary name like './promptpacker' instead of 'promptpacker')\n\n")

		fmt.Fprintf(os.Stderr, "  # Scan current directory, output to output.md\n")
		fmt.Fprintf(os.Stderr, "  promptpacker\n\n")

		fmt.Fprintf(os.Stderr, "  # Scan a specific project and save to a specific file\n")
		fmt.Fprintf(os.Stderr, "  promptpacker --root /path/to/project --output /path/to/project_summary.md\n\n")

		fmt.Fprintf(os.Stderr, "  # Scan current directory, exclude *.log and build/ directory\n")
		fmt.Fprintf(os.Stderr, "  promptpacker --exclude \"*.log,build/*\"\n\n")

		fmt.Fprintf(os.Stderr, "  # Use only 4 workers\n")
		fmt.Fprintf(os.Stderr, "  promptpacker --workers 4\n")
	}
}

func sortEntries(entries []walkEntry) {
	sort.Slice(entries, func(i, j int) bool {
		pathI := entries[i].relPath
		pathJ := entries[j].relPath
		partsI := strings.Split(pathI, "/")
		partsJ := strings.Split(pathJ, "/")
		lenI, lenJ := len(partsI), len(partsJ)
		minLen := lenI
		if lenJ < minLen {
			minLen = lenJ
		}
		for k := 0; k < minLen; k++ {
			if partsI[k] != partsJ[k] {
				return partsI[k] < partsJ[k]
			}
		}
		if lenI != lenJ {
			return lenI < lenJ
		}
		if entries[i].isDir != entries[j].isDir {
			return entries[i].isDir
		}
		return pathI < pathJ
	})
}

func writeStructure(writer *bufio.Writer, entries []walkEntry) {
	_, err := writer.WriteString("# Project Structure\n\n```\n")
	if err != nil {
		logWarn("Error writing structure header: %v", err)
		return
	}

	for _, entry := range entries {
		var lineBuilder strings.Builder

		if entry.depth > 0 {
			lineBuilder.WriteString(strings.Repeat("-", entry.depth))
			lineBuilder.WriteString(" ")
		}

		baseName := entry.relPath
		if idx := strings.LastIndex(entry.relPath, "/"); idx != -1 {
			baseName = entry.relPath[idx+1:]
		}

		if entry.isDir {
			lineBuilder.WriteString("/")
		}
		lineBuilder.WriteString(baseName)
		lineBuilder.WriteRune('\n')

		_, err = writer.WriteString(lineBuilder.String())
		if err != nil {
			logWarn("Error writing structure line for %s: %v", entry.relPath, err)

		}
	}

	_, err = writer.WriteString("```\n\n")
	if err != nil {
		logWarn("Error writing structure footer: %v", err)
	}
}

func getLanguageHint(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	ext = filepath.ToSlash(ext)
	switch ext {
	case ".go":
		return "go"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".py":
		return "python"
	case ".java":
		return "java"
	case ".cs":
		return "csharp"
	case ".php":
		return "php"
	case ".rb":
		return "ruby"
	case ".rs":
		return "rust"
	case ".swift":
		return "swift"
	case ".kt", ".kts":
		return "kotlin"
	case ".scala":
		return "scala"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".scss", ".sass":
		return "scss"
	case ".less":
		return "less"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".xml":
		return "xml"
	case ".sql":
		return "sql"
	case ".sh", ".bash", ".zsh":
		return "bash"
	case ".ps1":
		return "powershell"
	case ".md", ".markdown":
		return "markdown"
	case ".txt", "":
		return ""
	case ".dockerfile", ".docker":
		return "dockerfile"
	case ".env":
		return "bash"
	case ".gitignore":
		return "gitignore"
	case ".mod":
		return "go.mod"
	case ".sum":
		return "go.sum"
	case ".toml":
		return "toml"
	case ".lua":
		return "lua"
	case ".perl", ".pl":
		return "perl"
	case ".r":
		return "r"
	case ".dart":
		return "dart"
	case ".jsx":
		return "jsx"
	case ".tsx":
		return "tsx"
	case ".vue":
		return "vue"
	case ".svelte":
		return "svelte"
	default:
		trimmedExt := strings.TrimPrefix(ext, ".")
		if len(trimmedExt) > 20 {
			return ""
		}
		return trimmedExt
	}
}
