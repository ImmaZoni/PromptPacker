package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

const defaultOutputFile = "output.md"
const gitignoreFilename = ".gitignore"

var executablePath string

// Style definitions for TUI
var (
	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)
	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)
	doneStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true)
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
	warnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")).
			Bold(true)
)

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

func logInfo(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	fmt.Println(infoStyle.Render("ℹ") + " " + msg)
}

func logWarn(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	fmt.Fprintf(os.Stderr, "%s %s\n", warnStyle.Render("⚠"), msg)
}

func logError(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	fmt.Fprintf(os.Stderr, "%s %s\n", errorStyle.Render("✗"), msg)
}

func logFatal(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	fmt.Fprintf(os.Stderr, "%s %s\n", errorStyle.Render("✗"), msg)
	os.Exit(1)
}

func logDone(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	fmt.Println(doneStyle.Render("✓") + " " + msg)
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
	if existingRules, ok := gitignoreCache[absDir]; ok {
		return existingRules, true
	}
	if gitignoreLoadAttempt[absDir] {
		return nil, false
	}
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
type modeType string

const (
	modeFileSpec modeType = "file-spec"
	modeAuto     modeType = "auto"
)

type config struct {
	rootDir         string
	outputFile      string
	excludePatterns []string
	numWorkers      int
	mode            modeType
	structureOnly   bool
	maxDepth        int // 0 means no limit
	force           bool
	slowMode        bool
	includePaths    []string // For file-spec mode
	// Phase 2: Extension filtering
	includeExts []string // Only include files with these extensions (empty = all)
	excludeExts []string // Exclude files with these extensions
	// Phase 2: File size limit
	maxFileSizeBytes int64 // 0 = no limit
	// Phase 2: Git-aware
	gitChanged bool   // Only include git-changed files
	gitSince   string // Include files changed since this ref
	gitBranch  string // Compare against this branch
	// Phase 2: Config file
	profile string // Named profile from .promptpacker.yml
	// Phase 3: Structure output enhancements
	showSizes      bool // Show file sizes in structure output
	showExtensions bool // (reserved, extensions visible in filenames already)
}
type fileTask struct{ entry walkEntry }
type fileResult struct {
	relPath string
	content string
	err     error
}

// App states
type appState int

const (
	stateFileSelection appState = iota
	stateScanning
	statePreview
	stateProcessing
	stateWriting
	stateDone
	stateError
	stateCancelled
)

// Progress update messages
type scanProgressMsg struct {
	currentFile string
	count       int
}

type processProgressMsg struct {
	processed int
	total     int
}

type writeProgressMsg struct {
	written int
	total   int
}

type scanCompleteMsg struct {
	entries []walkEntry
	err     error
}

type processCompleteMsg struct {
	content map[string]fileResult
	err     error
}

type writeCompleteMsg struct {
	err error
}

// Main application model
type appModel struct {
	state            appState
	cfg              config
	entries          []walkEntry
	processedContent map[string]fileResult

	// UI components
	scanSpinner     spinner.Model
	scanProgress    progress.Model
	processProgress progress.Model
	writeProgress   progress.Model

	// Progress tracking
	scanCount    int
	processCount int
	processTotal int
	writeCount   int
	writeTotal   int
	currentFile  string

	// Messages
	statusMsg string
	err       error

	// Progress Updates
	progressChan chan tea.Msg

	// File selector (embedded)
	fileSelector *fileSelectorModel
}

func initialAppModel(cfg config) appModel {
	scanSpinner := spinner.New()
	scanSpinner.Spinner = spinner.Dot
	scanSpinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))

	scanProg := progress.New(progress.WithScaledGradient("#39FF14", "#39FF14"))
	scanProg.Width = 50

	processProg := progress.New(progress.WithScaledGradient("#39FF14", "#39FF14"))
	processProg.Width = 50

	writeProg := progress.New(progress.WithScaledGradient("#39FF14", "#39FF14"))
	writeProg.Width = 50

	state := stateFileSelection
	if cfg.mode == modeAuto || len(cfg.includePaths) > 0 {
		state = stateScanning
	}

	app := appModel{
		state:            state,
		cfg:              cfg,
		scanSpinner:      scanSpinner,
		scanProgress:     scanProg,
		processProgress:  processProg,
		writeProgress:    writeProg,
		processedContent: make(map[string]fileResult),
		progressChan:     make(chan tea.Msg, 100),
	}

	if state == stateFileSelection {
		model, err := initialFileSelectorModel(cfg.rootDir)
		if err != nil {
			app.err = err
		} else {
			app.fileSelector = model
		}
	}

	return app
}

func (m appModel) Init() tea.Cmd {
	// Initialize gitignore cache
	loadAndCacheGitignore(m.cfg.rootDir)

	if m.state == stateFileSelection {
		if m.err != nil {
			return tea.Batch(
				m.scanSpinner.Tick,
				func() tea.Msg { return m.err },
			)
		}
		// Start file selection
		return tea.Batch(
			m.scanSpinner.Tick,
			m.fileSelector.Init(),
		)
	} else if m.state == stateScanning {
		// Start scanning
		return tea.Batch(
			m.scanSpinner.Tick,
			startScanning(m.cfg, m.progressChan),
			waitForProgress(m.progressChan),
		)
	}
	return nil
}

func waitForProgress(c chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-c
	}
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateFileSelection:
		return m.updateFileSelection(msg)
	case stateScanning:
		return m.updateScanning(msg)
	case statePreview:
		return m.updatePreview(msg)
	case stateProcessing:
		return m.updateProcessing(msg)
	case stateWriting:
		return m.updateWriting(msg)
	case stateDone, stateError, stateCancelled:
		return m.updateFinal(msg)
	}
	return m, nil
}

func (m appModel) View() string {
	var s strings.Builder

	// Header
	headerBorder := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true)
	headerText := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true).
		Padding(0, 1)

	s.WriteString(headerBorder.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━") + "\n")
	s.WriteString(headerText.Render("🚀 PromptPacker v0.2") + "\n")
	s.WriteString(headerBorder.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━") + "\n\n")

	switch m.state {
	case stateFileSelection:
		if m.fileSelector != nil {
			s.WriteString(m.fileSelector.View())
		}
	case stateScanning:
		s.WriteString(m.scanSpinner.View() + " ")
		s.WriteString(infoStyle.Render("Scanning directory structure...") + "\n")
		if m.scanCount > 0 {
			s.WriteString(dimStyle.Render(fmt.Sprintf("Found %d items...", m.scanCount)) + "\n")
		}
	case statePreview:
		s.WriteString(infoStyle.Render("Preview: Files and directories to be included") + "\n\n")

		fileCount := 0
		dirCount := 0
		totalSize := int64(0)
		for _, entry := range m.entries {
			if entry.isDir {
				dirCount++
			} else {
				fileCount++
				if info, err := os.Stat(entry.fullPath); err == nil {
					totalSize += info.Size()
				}
			}
		}

		sampleCount := 10
		for i, entry := range m.entries {
			if i >= sampleCount {
				remaining := len(m.entries) - sampleCount
				s.WriteString(dimStyle.Render(fmt.Sprintf("  ... and %d more items\n", remaining)))
				break
			}
			if entry.isDir {
				s.WriteString(fmt.Sprintf("  [DIR]  %s\n", entry.relPath))
			} else {
				info, err := os.Stat(entry.fullPath)
				sizeStr := ""
				if err == nil {
					sizeStr = fmt.Sprintf(" (%s)", formatSize(info.Size()))
				}
				s.WriteString(fmt.Sprintf("  [FILE] %s%s\n", entry.relPath, sizeStr))
			}
		}

		s.WriteString(fmt.Sprintf("\n%s\n", strings.Repeat("─", 60)))
		s.WriteString("Summary:\n")
		s.WriteString(fmt.Sprintf("  Directories: %d\n", dirCount))
		s.WriteString(fmt.Sprintf("  Files: %d\n", fileCount))
		s.WriteString(fmt.Sprintf("  Total size: %s\n", formatSize(totalSize)))
		s.WriteString(fmt.Sprintf("%s\n\n", strings.Repeat("─", 60)))
		s.WriteString(warnStyle.Render("Continue with generation? (y/N)"))

	case stateProcessing:
		s.WriteString(m.scanSpinner.View() + " ")
		s.WriteString(infoStyle.Render("Processing file contents...") + "\n\n")
		if m.processTotal > 0 {
			percent := float64(m.processCount) / float64(m.processTotal)
			s.WriteString(m.processProgress.ViewAs(percent) + "\n")
			s.WriteString(dimStyle.Render(fmt.Sprintf("%d/%d files processed", m.processCount, m.processTotal)) + "\n")
		}
	case stateWriting:
		s.WriteString(m.scanSpinner.View() + " ")
		s.WriteString(infoStyle.Render("Writing to output file...") + "\n\n")
		if m.writeTotal > 0 {
			percent := float64(m.writeCount) / float64(m.writeTotal)
			s.WriteString(m.writeProgress.ViewAs(percent) + "\n")
			s.WriteString(dimStyle.Render(fmt.Sprintf("%d/%d files written", m.writeCount, m.writeTotal)) + "\n")
		}
	case stateDone:
		s.WriteString(doneStyle.Render("✓") + " ")
		if m.cfg.structureOnly {
			s.WriteString(doneStyle.Render(fmt.Sprintf("Successfully created structure-only output: %s", m.cfg.outputFile)) + "\n")
		} else {
			s.WriteString(doneStyle.Render(fmt.Sprintf("Successfully created %s", m.cfg.outputFile)) + "\n")
		}
		s.WriteString("\n" + headerBorder.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━") + "\n")
	case stateError:
		s.WriteString(errorStyle.Render("✗") + " ")
		s.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n")
	case stateCancelled:
		s.WriteString(warnStyle.Render("⚠") + " ")
		s.WriteString(warnStyle.Render("Operation cancelled by user.") + "\n")
	}

	return s.String()
}

// Update methods for each state
func (m appModel) updateFileSelection(msg tea.Msg) (appModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.state = stateCancelled
			return m, tea.Quit
		}
	case error:
		m.state = stateError
		m.err = msg
		return m, tea.Quit
	}

	if m.fileSelector != nil {
		updated, cmd := m.fileSelector.Update(msg)
		if fsModel, ok := updated.(fileSelectorModel); ok {
			m.fileSelector = &fsModel
		}

		if m.fileSelector.confirmed {
			// Get selected paths and move to scanning
			selectedPaths, err := getSelectedPaths(m.fileSelector, m.cfg.rootDir)
			if err != nil {
				m.state = stateError
				m.err = err
				return m, tea.Quit
			}
			m.cfg.includePaths = selectedPaths
			m.state = stateScanning
			return m, tea.Batch(
				m.scanSpinner.Tick,
				startScanning(m.cfg, m.progressChan),
				waitForProgress(m.progressChan),
			)
		}

		if m.fileSelector.quitting {
			m.state = stateCancelled
			return m, tea.Quit
		}

		return m, cmd
	}
	return m, nil
}

func (m appModel) updateScanning(msg tea.Msg) (appModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.state = stateCancelled
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.scanSpinner, cmd = m.scanSpinner.Update(msg)
		cmds = append(cmds, cmd)
	case scanProgressMsg:
		m.scanCount = msg.count
		m.currentFile = msg.currentFile
		cmds = append(cmds, waitForProgress(m.progressChan))
	case scanCompleteMsg:
		if msg.err != nil {
			m.state = stateError
			m.err = msg.err
			return m, tea.Quit
		}
		m.entries = msg.entries
		sortEntries(m.entries)

		if m.cfg.force {
			// Skip preview, go straight to processing
			m.state = stateProcessing
			return m, tea.Batch(startProcessing(m.cfg, m.entries, m.progressChan), waitForProgress(m.progressChan))
		} else {
			// Show preview natively
			m.state = statePreview
			return m, nil
		}
	}

	return m, tea.Batch(cmds...)
}

func (m appModel) updatePreview(msg tea.Msg) (appModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "n", "N":
			m.state = stateCancelled
			return m, tea.Quit
		case "y", "Y", "enter":
			m.state = stateProcessing
			return m, tea.Batch(
				startProcessing(m.cfg, m.entries, m.progressChan),
				waitForProgress(m.progressChan),
				m.scanSpinner.Tick,
			)
		}
	}
	return m, nil
}

func (m appModel) updateProcessing(msg tea.Msg) (appModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.state = stateCancelled
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.scanSpinner, cmd = m.scanSpinner.Update(msg)
		cmds = append(cmds, cmd)
	case processProgressMsg:
		m.processCount = msg.processed
		m.processTotal = msg.total
		cmds = append(cmds, waitForProgress(m.progressChan))
	case processCompleteMsg:
		if msg.err != nil {
			m.state = stateError
			m.err = msg.err
			return m, tea.Quit
		}
		m.processedContent = msg.content
		m.state = stateWriting
		return m, tea.Batch(
			startWriting(m.cfg, m.entries, m.processedContent, m.progressChan),
			waitForProgress(m.progressChan),
		)
	}

	return m, tea.Batch(cmds...)
}

func (m appModel) updateWriting(msg tea.Msg) (appModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.state = stateCancelled
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.scanSpinner, cmd = m.scanSpinner.Update(msg)
		cmds = append(cmds, cmd)
	case writeProgressMsg:
		m.writeCount = msg.written
		m.writeTotal = msg.total
		cmds = append(cmds, waitForProgress(m.progressChan))
	case writeCompleteMsg:
		if msg.err != nil {
			m.state = stateError
			m.err = msg.err
			return m, tea.Quit
		}
		m.state = stateDone
		return m, tea.Quit
	}

	return m, tea.Batch(cmds...)
}

func (m appModel) updateFinal(msg tea.Msg) (appModel, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyMsg:
		return m, tea.Quit
	}
	return m, nil
}

// Commands for async operations
func startScanning(cfg config, progressChan chan<- tea.Msg) tea.Cmd {
	return func() tea.Msg {
		// Pre-compute git-changed file set if needed
		var gitChangedSet map[string]struct{}
		if cfg.gitChanged || cfg.gitSince != "" || cfg.gitBranch != "" {
			var ref string
			switch {
			case cfg.gitBranch != "":
				ref = cfg.gitBranch
			case cfg.gitSince != "":
				ref = cfg.gitSince
			default:
				ref = "main"
			}
			files, err := getGitChangedFiles(cfg.rootDir, ref)
			if err != nil {
				logWarn("Git changed files detection failed: %v. Including all files.", err)
			} else {
				gitChangedSet = make(map[string]struct{}, len(files))
				for _, f := range files {
					gitChangedSet[f] = struct{}{}
				}
			}
		}
		// Run in goroutine to avoid blocking
		done := make(chan scanCompleteMsg, 1)
		go func() {
			var entries []walkEntry
			count := 0
			walkErr := filepath.WalkDir(cfg.rootDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				absPath, err := filepath.Abs(path)
				if err != nil {
					return nil
				}
				relPath, err := filepath.Rel(cfg.rootDir, absPath)
				if err != nil {
					return nil
				}
				relPath = filepath.ToSlash(relPath)
				if relPath == "." {
					return nil
				}
				isDir := d.IsDir()
				baseName := filepath.Base(absPath)
				depth := strings.Count(relPath, "/")

				if cfg.maxDepth > 0 && depth >= cfg.maxDepth {
					if isDir {
						return filepath.SkipDir
					}
					return nil
				}

				if cfg.mode == modeFileSpec {
					if !isPathIncluded(relPath, absPath, cfg.rootDir, cfg.includePaths) {
						if isDir {
							return filepath.SkipDir
						}
						return nil
					}
				}

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

				// Extension filtering (files only)
				if !isDir {
					ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(baseName)), ".")
					if len(cfg.includeExts) > 0 {
						matched := false
						for _, e := range cfg.includeExts {
							if ext == strings.ToLower(strings.TrimPrefix(e, ".")) {
								matched = true
								break
							}
						}
						if !matched {
							return nil
						}
					}
					for _, e := range cfg.excludeExts {
						if ext == strings.ToLower(strings.TrimPrefix(e, ".")) {
							return nil
						}
					}
					// Max file size check
					if cfg.maxFileSizeBytes > 0 {
						info, statErr := d.Info()
						if statErr == nil && info.Size() > cfg.maxFileSizeBytes {
							return nil
						}
					}
				}

				// Git-changed filtering (files only)
				if !isDir && gitChangedSet != nil {
					if _, ok := gitChangedSet[relPath]; !ok {
						return nil
					}
				}

				count++
				if count%10 == 0 {
					progressChan <- scanProgressMsg{count: count, currentFile: relPath}
				}

				if cfg.slowMode {
					time.Sleep(1000 * time.Millisecond)
				}

				entries = append(entries, walkEntry{relPath: relPath, fullPath: absPath, isDir: isDir, depth: depth})
				return nil
			})

			if walkErr != nil {
				done <- scanCompleteMsg{err: walkErr}
				return
			}
			done <- scanCompleteMsg{entries: entries}
		}()
		return <-done
	}
}

func startProcessing(cfg config, entries []walkEntry, progressChan chan<- tea.Msg) tea.Cmd {
	return func() tea.Msg {
		if cfg.structureOnly {
			return processCompleteMsg{content: make(map[string]fileResult)}
		}

		tasks := make(chan fileTask, len(entries))
		results := make(chan fileResult, len(entries))
		processedContent := make(map[string]fileResult)
		var wg sync.WaitGroup

		numFileTasks := 0
		for _, entry := range entries {
			if !entry.isDir {
				numFileTasks++
			}
		}

		for i := 0; i < cfg.numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for task := range tasks {
					if cfg.slowMode {
						time.Sleep(1000 * time.Millisecond)
					}
					formattedContent, err := processFileContent(task.entry)
					results <- fileResult{relPath: task.entry.relPath, content: formattedContent, err: err}
				}
			}()
		}

		for _, entry := range entries {
			if !entry.isDir {
				tasks <- fileTask{entry: entry}
			}
		}
		close(tasks)

		processedCount := 0
		go func() {
			for result := range results {
				processedContent[result.relPath] = result
				processedCount++
				if processedCount%5 == 0 || processedCount == numFileTasks {
					progressChan <- processProgressMsg{processed: processedCount, total: numFileTasks}
				}
			}
		}()

		wg.Wait()
		close(results)

		return processCompleteMsg{content: processedContent}
	}
}

func startWriting(cfg config, entries []walkEntry, processedContent map[string]fileResult, progressChan chan<- tea.Msg) tea.Cmd {
	return func() tea.Msg {
		outFile, err := os.Create(cfg.outputFile)
		if err != nil {
			return writeCompleteMsg{err: err}
		}
		defer outFile.Close()
		writer := bufio.NewWriter(outFile)

		writeStructure(writer, entries, cfg)

		if !cfg.structureOnly {
			writer.WriteString("# File Contents\n\n")

			writeCount := 0
			writeTotal := 0
			for _, entry := range entries {
				if !entry.isDir {
					writeTotal++
				}
			}

			for _, entry := range entries {
				if !entry.isDir {
					result, found := processedContent[entry.relPath]
					if found {
						writer.WriteString(result.content)
					}
					writeCount++
					if writeCount%10 == 0 || writeCount == writeTotal {
						progressChan <- writeProgressMsg{written: writeCount, total: writeTotal}
					}
					if cfg.slowMode {
						time.Sleep(1000 * time.Millisecond)
					}
				}
			}
		}

		err = writer.Flush()
		if err != nil {
			return writeCompleteMsg{err: err}
		}

		return writeCompleteMsg{}
	}
}

func getSelectedPaths(selector *fileSelectorModel, rootDir string) ([]string, error) {
	var selectedPaths []string
	for path := range selector.selected {
		relPath, err := filepath.Rel(rootDir, path)
		if err == nil {
			selectedPaths = append(selectedPaths, relPath)
		}
	}
	return selectedPaths, nil
}

func main() {
	var execErr error
	executablePath, execErr = os.Executable()
	if execErr != nil {
		executablePath = ""
	} else {
		executablePath, execErr = filepath.Abs(executablePath)
		if execErr != nil {
			executablePath = ""
		}
	}

	setupUsage()
	cfg := parseFlags()

	// Initialize and run the TUI application
	model := initialAppModel(cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running application: %v\n", err)
		os.Exit(1)
	}
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
	var includeList string
	var modeStr string
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
	modePtr := flag.String("mode", string(modeFileSpec), "Selection mode: 'file-spec' (default, requires explicit file selection) or 'auto' (scan entire directory).")
	structureOnlyPtr := flag.Bool("structure-only", false, "Only output directory structure, skip file contents.")
	showSizesPtr := flag.Bool("show-sizes", false, "Show file sizes in structure output (e.g. main.go (4.2 KB)).")
	showExtensionsPtr := flag.Bool("show-extensions", false, "Explicitly label file extensions in structure output.")
	maxDepthPtr := flag.Int("max-depth", 0, "Maximum directory depth to traverse (0 = unlimited).")
	forcePtr := flag.Bool("force", false, "Skip preview prompt and generate immediately.")
	slowPtr := flag.Bool("slow", false, "Artificially slow down operations (for UI testing).")
	includeListPtr := flag.String("include", "", "Comma-separated list of files/directories to include (for file-spec mode).")
	includeExtPtr := flag.String("include-ext", "", "Comma-separated file extensions to include (e.g. go,ts,md). Empty = all.")
	excludeExtPtr := flag.String("exclude-ext", "", "Comma-separated file extensions to exclude (e.g. log,tmp,bak).")
	maxFileSizePtr := flag.String("max-file-size", "", "Skip files larger than this size (e.g. 1MB, 500KB, 100000).")
	gitChangedPtr := flag.Bool("changed", false, "Only include files changed in git (vs main branch).")
	gitSincePtr := flag.String("since", "", "Only include files changed since this git ref (e.g. v1.0.0, HEAD~5).")
	gitBranchPtr := flag.String("branch", "", "Only include files changed relative to this branch.")
	profilePtr := flag.String("profile", "", "Named profile from .promptpacker.yml config file.")

	flag.Parse()

	cfg.rootDir = *rootDirPtr
	cfg.outputFile = *outputFilePtr
	excludeList = *excludeListPtr
	cfg.numWorkers = *numWorkersPtr
	modeStr = *modePtr
	cfg.structureOnly = *structureOnlyPtr
	cfg.showSizes = *showSizesPtr
	cfg.showExtensions = *showExtensionsPtr
	cfg.maxDepth = *maxDepthPtr
	cfg.force = *forcePtr
	cfg.slowMode = *slowPtr
	includeList = *includeListPtr
	cfg.gitChanged = *gitChangedPtr
	cfg.gitSince = *gitSincePtr
	cfg.gitBranch = *gitBranchPtr
	cfg.profile = *profilePtr

	// Parse extension filters
	if *includeExtPtr != "" {
		for _, e := range strings.Split(*includeExtPtr, ",") {
			e = strings.TrimSpace(e)
			if e != "" {
				cfg.includeExts = append(cfg.includeExts, e)
			}
		}
	}
	if *excludeExtPtr != "" {
		for _, e := range strings.Split(*excludeExtPtr, ",") {
			e = strings.TrimSpace(e)
			if e != "" {
				cfg.excludeExts = append(cfg.excludeExts, e)
			}
		}
	}
	// Parse max file size
	if *maxFileSizePtr != "" {
		cfg.maxFileSizeBytes = parseSize(*maxFileSizePtr)
	}

	// Parse mode
	switch modeStr {
	case "file-spec", "filespec", "spec":
		cfg.mode = modeFileSpec
	case "auto", "all":
		cfg.mode = modeAuto
	default:
		logFatal("Invalid mode '%s'. Must be 'file-spec' or 'auto'", modeStr)
	}

	// Load config file first (flags override config file)
	if fileCfg, ok := loadProjectConfig(cfg.rootDir, cfg.profile); ok {
		cfg = mergeConfigFile(cfg, fileCfg)
	}

	// Re-apply parsed flags over config (flags always win)
	if *modePtr != string(modeFileSpec) { // user explicitly set mode
		switch *modePtr {
		case "auto", "all":
			cfg.mode = modeAuto
		}
	}
	if *structureOnlyPtr {
		cfg.structureOnly = true
	}
	if *maxDepthPtr != 0 {
		cfg.maxDepth = *maxDepthPtr
	}
	if *forcePtr {
		cfg.force = true
	}

	// Parse include paths
	if includeList != "" {
		rawPaths := strings.Split(includeList, ",")
		for _, p := range rawPaths {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				cfg.includePaths = append(cfg.includePaths, trimmed)
			}
		}
	}

	// Also check for positional arguments (for file-spec mode)
	if cfg.mode == modeFileSpec && len(flag.Args()) > 0 {
		cfg.includePaths = append(cfg.includePaths, flag.Args()...)
	}

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
	if cfg.maxDepth < 0 {
		cfg.maxDepth = 0
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

// parseSize converts human-readable size strings to bytes (e.g. "1MB", "500KB", "1024")
func parseSize(s string) int64 {
	s = strings.TrimSpace(strings.ToUpper(s))
	multipliers := map[string]int64{
		"KB": 1024, "MB": 1024 * 1024, "GB": 1024 * 1024 * 1024,
		"K": 1024, "M": 1024 * 1024, "G": 1024 * 1024 * 1024,
	}
	for suffix, mult := range multipliers {
		if strings.HasSuffix(s, suffix) {
			numStr := strings.TrimSuffix(s, suffix)
			var n int64
			fmt.Sscanf(strings.TrimSpace(numStr), "%d", &n)
			return n * mult
		}
	}
	var n int64
	fmt.Sscanf(s, "%d", &n)
	return n
}

// getGitChangedFiles returns relative paths of files changed vs a git ref
func getGitChangedFiles(rootDir, ref string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", ref)
	cmd.Dir = rootDir
	out, err := cmd.Output()
	if err != nil {
		// Also try --diff-filter to handle git error gracefully
		return nil, fmt.Errorf("git diff failed: %w", err)
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = filepath.ToSlash(strings.TrimSpace(line))
		if line != "" {
			files = append(files, line)
		}
	}
	// Also include untracked/modified files from working tree
	cmd2 := exec.Command("git", "status", "--porcelain")
	cmd2.Dir = rootDir
	out2, err2 := cmd2.Output()
	if err2 == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out2)), "\n") {
			if len(line) < 3 {
				continue
			}
			f := filepath.ToSlash(strings.TrimSpace(line[3:]))
			if f != "" {
				files = append(files, f)
			}
		}
	}
	// Deduplicate
	seen := make(map[string]struct{})
	result := files[:0]
	for _, f := range files {
		if _, ok := seen[f]; !ok {
			seen[f] = struct{}{}
			result = append(result, f)
		}
	}
	return result, nil
}

// projectConfigFile represents .promptpacker.yml structure
type projectConfigFile struct {
	Defaults projectConfigDefaults            `json:"defaults"`
	Profiles map[string]projectConfigDefaults `json:"profiles"`
}

type projectConfigDefaults struct {
	Mode          string   `json:"mode"`
	StructureOnly bool     `json:"structure-only"`
	Output        string   `json:"output"`
	Workers       int      `json:"workers"`
	ExcludeExt    string   `json:"exclude-ext"`
	IncludeExt    string   `json:"include-ext"`
	Exclude       string   `json:"exclude"`
	Include       string   `json:"include"`
	MaxDepth      int      `json:"max-depth"`
	MaxFileSize   string   `json:"max-file-size"`
	Force         bool     `json:"force"`
	_             struct{} // prevent unkeyed init
}

// loadProjectConfig loads .promptpacker.yml (or .yaml) from rootDir and optionally applies a named profile
func loadProjectConfig(rootDir, profile string) (projectConfigFile, bool) {
	candidates := []string{
		filepath.Join(rootDir, ".promptpacker.yml"),
		filepath.Join(rootDir, ".promptpacker.yaml"),
	}
	var data []byte
	var found bool
	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err == nil {
			data = b
			found = true
			break
		}
	}
	if !found {
		return projectConfigFile{}, false
	}
	// Simple YAML -> JSON-compatible parse using a minimal approach:
	// We support key: value, nested sections (defaults:, profiles:, profilename:)
	var cfg projectConfigFile
	cfg.Profiles = make(map[string]projectConfigDefaults)
	jsonData := yamlToJSON(string(data))
	if err := json.Unmarshal([]byte(jsonData), &cfg); err != nil {
		logWarn("Could not parse .promptpacker.yml: %v", err)
		return projectConfigFile{}, false
	}
	// If a profile is requested, overlay on top of defaults
	if profile != "" {
		if p, ok := cfg.Profiles[profile]; ok {
			cfg.Defaults = overlayProfile(cfg.Defaults, p)
		} else {
			logWarn("Profile '%s' not found in .promptpacker.yml", profile)
		}
	}
	return cfg, true
}

// yamlToJSON converts a simple flat/nested YAML to JSON (handles the subset used in .promptpacker.yml)
func yamlToJSON(yaml string) string {
	// This is a minimal YAML parser for our specific config format.
	// It handles: top-level keys, nested blocks (2-space indent), string/bool/int values.
	lines := strings.Split(yaml, "\n")
	var buf strings.Builder
	buf.WriteString("{")
	type stackEntry struct{ key string }
	indent := 0
	first := [3]bool{true, true, true}
	_ = indent
	_ = stackEntry{}
	// Simple line-by-line parsing
	topSection := ""
	subSection := ""
	firstTop := true
	firstSub := true
	firstItem := true
	for _, raw := range lines {
		line := strings.TrimRight(raw, " \r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		spaces := len(line) - len(strings.TrimLeft(line, " "))
		trimmed := strings.TrimSpace(line)
		if spaces == 0 {
			// Top-level key
			if strings.HasSuffix(trimmed, ":") {
				// Close previous sub
				if subSection != "" {
					buf.WriteString("}")
					subSection = ""
					firstItem = true
				}
				// Close previous top
				if topSection != "" {
					buf.WriteString("}")
				}
				if !firstTop {
					buf.WriteString(",")
				}
				firstTop = false
				firstSub = true
				topSection = strings.TrimSuffix(trimmed, ":")
				buf.WriteString(fmt.Sprintf("%q:{", topSection))
			} else {
				kv := strings.SplitN(trimmed, ":", 2)
				if len(kv) == 2 {
					if !firstTop {
						buf.WriteString(",")
					}
					firstTop = false
					buf.WriteString(fmt.Sprintf("%q:%s", strings.TrimSpace(kv[0]), jsonVal(strings.TrimSpace(kv[1]))))
				}
			}
		} else if spaces == 2 {
			// Sub-section or key under top-level section
			if topSection == "profiles" && strings.HasSuffix(trimmed, ":") {
				// Close previous sub
				if subSection != "" {
					buf.WriteString("}")
					firstItem = true
				}
				if !firstSub {
					buf.WriteString(",")
				}
				firstSub = false
				subSection = strings.TrimSuffix(trimmed, ":")
				buf.WriteString(fmt.Sprintf("%q:{", subSection))
			} else {
				kv := strings.SplitN(trimmed, ":", 2)
				if len(kv) == 2 {
					if subSection == "" {
						if !first[0] {
							buf.WriteString(",")
						}
						first[0] = false
					} else {
						if !firstItem {
							buf.WriteString(",")
						}
						firstItem = false
					}
					buf.WriteString(fmt.Sprintf("%q:%s", strings.TrimSpace(kv[0]), jsonVal(strings.TrimSpace(kv[1]))))
				}
			}
		} else if spaces == 4 {
			// Key under profile sub-section
			kv := strings.SplitN(trimmed, ":", 2)
			if len(kv) == 2 {
				if !firstItem {
					buf.WriteString(",")
				}
				firstItem = false
				buf.WriteString(fmt.Sprintf("%q:%s", strings.TrimSpace(kv[0]), jsonVal(strings.TrimSpace(kv[1]))))
			}
		}
	}
	if subSection != "" {
		buf.WriteString("}")
	}
	if topSection != "" {
		buf.WriteString("}")
	}
	buf.WriteString("}")
	return buf.String()
}

func jsonVal(s string) string {
	if s == "true" || s == "false" || s == "null" {
		return s
	}
	// Check if it's a number
	var n int64
	if _, err := fmt.Sscanf(s, "%d", &n); err == nil && fmt.Sprintf("%d", n) == s {
		return s
	}
	// String - strip surrounding quotes if present
	s = strings.Trim(s, "\"'")
	return fmt.Sprintf("%q", s)
}

// overlayProfile merges a profile over the defaults (non-zero values from profile win)
func overlayProfile(base, overlay projectConfigDefaults) projectConfigDefaults {
	if overlay.Mode != "" {
		base.Mode = overlay.Mode
	}
	if overlay.StructureOnly {
		base.StructureOnly = true
	}
	if overlay.Output != "" {
		base.Output = overlay.Output
	}
	if overlay.Workers > 0 {
		base.Workers = overlay.Workers
	}
	if overlay.ExcludeExt != "" {
		base.ExcludeExt = overlay.ExcludeExt
	}
	if overlay.IncludeExt != "" {
		base.IncludeExt = overlay.IncludeExt
	}
	if overlay.Exclude != "" {
		base.Exclude = overlay.Exclude
	}
	if overlay.Include != "" {
		base.Include = overlay.Include
	}
	if overlay.MaxDepth > 0 {
		base.MaxDepth = overlay.MaxDepth
	}
	if overlay.MaxFileSize != "" {
		base.MaxFileSize = overlay.MaxFileSize
	}
	if overlay.Force {
		base.Force = true
	}
	return base
}

// mergeConfigFile applies config file defaults to cfg, but only for fields not already explicitly set by CLI flags
func mergeConfigFile(cfg config, fileCfg projectConfigFile) config {
	d := fileCfg.Defaults
	if d.Mode != "" && cfg.mode == modeFileSpec {
		switch d.Mode {
		case "auto", "all":
			cfg.mode = modeAuto
		}
	}
	if d.StructureOnly && !cfg.structureOnly {
		cfg.structureOnly = true
	}
	if d.Output != "" && cfg.outputFile == "" {
		cfg.outputFile = d.Output
	}
	if d.Workers > 0 && cfg.numWorkers == 0 {
		cfg.numWorkers = d.Workers
	}
	if d.MaxDepth > 0 && cfg.maxDepth == 0 {
		cfg.maxDepth = d.MaxDepth
	}
	if d.MaxFileSize != "" && cfg.maxFileSizeBytes == 0 {
		cfg.maxFileSizeBytes = parseSize(d.MaxFileSize)
	}
	if d.Force && !cfg.force {
		cfg.force = true
	}
	if d.ExcludeExt != "" && len(cfg.excludeExts) == 0 {
		for _, e := range strings.Split(d.ExcludeExt, ",") {
			e = strings.TrimSpace(e)
			if e != "" {
				cfg.excludeExts = append(cfg.excludeExts, e)
			}
		}
	}
	if d.IncludeExt != "" && len(cfg.includeExts) == 0 {
		for _, e := range strings.Split(d.IncludeExt, ",") {
			e = strings.TrimSpace(e)
			if e != "" {
				cfg.includeExts = append(cfg.includeExts, e)
			}
		}
	}
	if d.Exclude != "" && len(cfg.excludePatterns) == 0 {
		for _, p := range strings.Split(d.Exclude, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				cfg.excludePatterns = append(cfg.excludePatterns, p)
			}
		}
	}
	if d.Include != "" && len(cfg.includePaths) == 0 {
		for _, p := range strings.Split(d.Include, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				cfg.includePaths = append(cfg.includePaths, p)
			}
		}
	}
	return cfg
}

func setupUsage() {
	flag.Usage = func() {
		invocationName := filepath.Base(os.Args[0])

		fmt.Println("------------------------------------")
		fmt.Println("       🚀 PromptPacker v0.2 🚀      ")
		fmt.Println("------------------------------------")
		fmt.Fprintf(os.Stderr, "Consolidates a code project into a single Markdown file, suitable for LLMs.\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s [options] [files...]\n", invocationName)
		fmt.Fprintf(os.Stderr, "  go run promptpacker.go [options] [files...]  (if running source directly)\n\n")

		fmt.Fprintf(os.Stderr, "Modes:\n")
		fmt.Fprintf(os.Stderr, "  file-spec (default): Explicitly specify files/directories to include\n")
		fmt.Fprintf(os.Stderr, "  auto:                Automatically scan entire directory tree\n\n")

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
				} else if f.Name == "mode" {
					flagLine += " (Default: file-spec)"
				} else if f.Name == "max-depth" {
					flagLine += " (Default: 0 = unlimited)"
				} else {
					flagLine += fmt.Sprintf(" (Default: %s)", f.DefValue)
				}
			}
			fmt.Fprintln(w, flagLine)
		})
		w.Flush()

		fmt.Fprintf(os.Stderr, "\nFile Selection:\n")
		fmt.Fprintf(os.Stderr, "  In file-spec mode (default), you must specify files to include:\n")
		fmt.Fprintf(os.Stderr, "    - Use positional arguments: %s src/ cmd/ go.mod\n", invocationName)
		fmt.Fprintf(os.Stderr, "    - Use --include flag: %s --include src/,cmd/,go.mod\n", invocationName)
		fmt.Fprintf(os.Stderr, "    - If no files specified, interactive menu will appear\n\n")

		fmt.Fprintf(os.Stderr, "Preview Mode:\n")
		fmt.Fprintf(os.Stderr, "  By default, a preview is shown before generating output.\n")
		fmt.Fprintf(os.Stderr, "  Use --force to skip preview and generate immediately.\n\n")

		fmt.Fprintf(os.Stderr, "Exclusion Logic:\n")
		fmt.Fprintf(os.Stderr, "  Files are excluded based on: .gitignore rules > Default ignores > Hidden files > --exclude patterns.\n")
		fmt.Fprintf(os.Stderr, "  See README for full details on default ignores.\n")

		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # File-spec mode: Include specific files/directories\n")
		fmt.Fprintf(os.Stderr, "  %s src/ cmd/ go.mod README.md\n\n", invocationName)

		fmt.Fprintf(os.Stderr, "  # Auto mode: Scan entire directory (legacy behavior)\n")
		fmt.Fprintf(os.Stderr, "  %s --mode auto\n\n", invocationName)

		fmt.Fprintf(os.Stderr, "  # Structure-only: Only output directory tree, no file contents\n")
		fmt.Fprintf(os.Stderr, "  %s --structure-only\n\n", invocationName)

		fmt.Fprintf(os.Stderr, "  # Limit depth: Only show top 2 levels\n")
		fmt.Fprintf(os.Stderr, "  %s --max-depth 2\n\n", invocationName)

		fmt.Fprintf(os.Stderr, "  # Skip preview: Generate immediately\n")
		fmt.Fprintf(os.Stderr, "  %s --force src/ cmd/\n\n", invocationName)

		fmt.Fprintf(os.Stderr, "  # Only include Go and Markdown files\n")
		fmt.Fprintf(os.Stderr, "  %s --mode auto --include-ext go,md\n\n", invocationName)

		fmt.Fprintf(os.Stderr, "  # Exclude log and temp files, skip files over 1MB\n")
		fmt.Fprintf(os.Stderr, "  %s --mode auto --exclude-ext log,tmp --max-file-size 1MB\n\n", invocationName)

		fmt.Fprintf(os.Stderr, "  # Only include git-changed files (vs main branch)\n")
		fmt.Fprintf(os.Stderr, "  %s --changed\n\n", invocationName)

		fmt.Fprintf(os.Stderr, "  # Only include files changed since a git tag\n")
		fmt.Fprintf(os.Stderr, "  %s --since v1.0.0\n\n", invocationName)

		fmt.Fprintf(os.Stderr, "  # Use a named profile from .promptpacker.yml\n")
		fmt.Fprintf(os.Stderr, "  %s --profile llm\n\n", invocationName)

		fmt.Fprintf(os.Stderr, "  # Combine options\n")
		fmt.Fprintf(os.Stderr, "  %s --mode auto --structure-only --max-depth 3 --output structure.md\n\n", invocationName)

		fmt.Fprintf(os.Stderr, "Config File (.promptpacker.yml):\n")
		fmt.Fprintf(os.Stderr, "  Place a .promptpacker.yml in your project root to set defaults.\n")
		fmt.Fprintf(os.Stderr, "  Example:\n")
		fmt.Fprintf(os.Stderr, "    defaults:\n")
		fmt.Fprintf(os.Stderr, "      mode: auto\n")
		fmt.Fprintf(os.Stderr, "      exclude-ext: log,tmp,bak\n")
		fmt.Fprintf(os.Stderr, "    profiles:\n")
		fmt.Fprintf(os.Stderr, "      llm:\n")
		fmt.Fprintf(os.Stderr, "        mode: auto\n")
		fmt.Fprintf(os.Stderr, "        exclude-ext: log,tmp,min.js,min.css\n")
		fmt.Fprintf(os.Stderr, "      structure:\n")
		fmt.Fprintf(os.Stderr, "        structure-only: true\n")
		fmt.Fprintf(os.Stderr, "        max-depth: 3\n\n")
	}
}

// showPreview displays a preview of what will be included and asks for confirmation using Huh
func showPreview(entries []walkEntry, cfg config) bool {
	if cfg.force {
		return true
	}

	// Calculate summary
	fileCount := 0
	dirCount := 0
	totalSize := int64(0)

	for _, entry := range entries {
		if entry.isDir {
			dirCount++
		} else {
			fileCount++
			info, err := os.Stat(entry.fullPath)
			if err == nil {
				totalSize += info.Size()
			}
		}
	}

	// Build preview text
	var previewText strings.Builder
	previewStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true)
	previewText.WriteString(previewStyle.Render("Preview: Files and directories to be included\n\n"))

	// Show first 10 items as sample
	sampleCount := 10
	for i, entry := range entries {
		if i >= sampleCount {
			remaining := len(entries) - sampleCount
			previewText.WriteString(dimStyle.Render(fmt.Sprintf("  ... and %d more items\n", remaining)))
			break
		}
		if entry.isDir {
			previewText.WriteString(fmt.Sprintf("  [DIR]  %s\n", entry.relPath))
		} else {
			info, err := os.Stat(entry.fullPath)
			sizeStr := ""
			if err == nil {
				sizeStr = fmt.Sprintf(" (%s)", formatSize(info.Size()))
			}
			previewText.WriteString(fmt.Sprintf("  [FILE] %s%s\n", entry.relPath, sizeStr))
		}
	}

	previewText.WriteString(fmt.Sprintf("\n%s\n", strings.Repeat("─", 60)))
	previewText.WriteString("Summary:\n")
	previewText.WriteString(fmt.Sprintf("  Directories: %d\n", dirCount))
	previewText.WriteString(fmt.Sprintf("  Files: %d\n", fileCount))
	previewText.WriteString(fmt.Sprintf("  Total size: %s\n", formatSize(totalSize)))
	previewText.WriteString(fmt.Sprintf("%s\n", strings.Repeat("─", 60)))

	// Use Huh for confirmation
	var confirmed bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Continue with generation?").
				Description(previewText.String()).
				Value(&confirmed),
		),
	).WithTheme(huh.ThemeBase16())

	err := form.Run()
	if err != nil {
		logWarn("Error in preview confirmation: %v. Assuming 'no'", err)
		return false
	}

	return confirmed
}

// formatSize formats bytes into human-readable size
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// isPathIncluded checks if a path matches any of the include patterns in file-spec mode
func isPathIncluded(relPath string, absPath string, rootDir string, includePaths []string) bool {
	relPathSlash := filepath.ToSlash(relPath)
	for _, includePath := range includePaths {
		includePath = strings.TrimSpace(includePath)
		if includePath == "" {
			continue
		}
		includePathSlash := filepath.ToSlash(includePath)

		// Check if the path starts with the include path (directory match)
		if strings.HasPrefix(relPathSlash, includePathSlash) {
			// Check if it's an exact match or a subdirectory/file
			if relPathSlash == includePathSlash {
				return true
			}
			// Check if it's a subdirectory/file (next char should be / or end of string)
			remaining := relPathSlash[len(includePathSlash):]
			if len(remaining) > 0 && (remaining[0] == '/' || includePathSlash == ".") {
				return true
			}
		}

		// Check if the current path is a parent directory of the include path
		if strings.HasPrefix(includePathSlash, relPathSlash+"/") {
			return true
		}

		// Check for exact match
		if relPathSlash == includePathSlash {
			return true
		}

		// Check if includePath is a directory and relPath is inside it
		absIncludePath := includePath
		if !filepath.IsAbs(includePath) {
			absIncludePath = filepath.Join(rootDir, includePath)
		}
		absIncludePath, err := filepath.Abs(absIncludePath)
		if err == nil {
			rel, err := filepath.Rel(absIncludePath, absPath)
			if err == nil && !strings.HasPrefix(rel, "..") {
				return true
			}
		}
	}
	return false
}

// selectableItem represents a file or directory that can be selected
type selectableItem struct {
	name          string
	path          string
	isDir         bool
	selected      bool
	ignored       bool   // true if this is an auto-ignored file
	ignoredReason string // reason for ignoring
}

// fileSelectorModel is the Bubble Tea model for file selection with directory navigation
type fileSelectorModel struct {
	items       []selectableItem
	cursor      int
	currentDir  string
	rootDir     string
	selected    map[string]struct{} // Use full path as key
	quitting    bool
	confirmed   bool
	pathHistory []string // For navigation back
	showIgnored bool     // Whether to show ignored files
}

// initialFileSelectorModel creates the initial model for file selection
func initialFileSelectorModel(rootDir string) (*fileSelectorModel, error) {
	model := &fileSelectorModel{
		currentDir:  rootDir,
		rootDir:     rootDir,
		selected:    make(map[string]struct{}),
		pathHistory: []string{rootDir},
		showIgnored: true, // Show ignored files by default
	}

	err := model.loadDirectory(rootDir)
	if err != nil {
		return nil, err
	}

	return model, nil
}

// loadDirectory loads items from a directory
func (m *fileSelectorModel) loadDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("error reading directory: %w", err)
	}

	var items []selectableItem
	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(dir, name)
		relPath, _ := filepath.Rel(m.rootDir, fullPath)
		relPath = filepath.ToSlash(relPath)

		// Check if this item is ignored
		ignored := false
		ignoredReason := ""

		// Check default ignores
		if checkDefaultIgnores(relPath, entry.IsDir()) {
			ignored = true
			ignoredReason = "default ignore pattern"
		}

		// Check gitignore
		absPath, _ := filepath.Abs(fullPath)
		gitignoreIgnored, _ := shouldIgnoreHierarchical(absPath, entry.IsDir(), m.rootDir)
		if gitignoreIgnored {
			ignored = true
			ignoredReason = ".gitignore"
		}

		// Check if hidden (but still show if showIgnored is true)
		isHidden := strings.HasPrefix(name, ".")

		// Always include directories for navigation, but mark ignored ones
		// For files, include if not hidden or if showIgnored is true
		if entry.IsDir() || !isHidden || m.showIgnored {
			_, selected := m.selected[fullPath]
			items = append(items, selectableItem{
				name:          name,
				path:          fullPath,
				isDir:         entry.IsDir(),
				selected:      selected,
				ignored:       ignored,
				ignoredReason: ignoredReason,
			})
		}
	}

	if len(items) == 0 {
		return fmt.Errorf("no items found in directory")
	}

	m.items = items
	m.cursor = 0
	return nil
}

// Init is called when the model is created
func (m fileSelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (m fileSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}

		case "right", "l", "enter":
			// Enter directory or confirm selection
			if m.cursor < len(m.items) {
				item := m.items[m.cursor]
				if item.isDir {
					// Navigate into directory
					m.pathHistory = append(m.pathHistory, item.path)
					m.currentDir = item.path
					if err := m.loadDirectory(item.path); err != nil {
						logWarn("Error loading directory: %v", err)
					}
				} else {
					// Confirm selection if items are selected
					if len(m.selected) > 0 {
						m.confirmed = true
						return m, tea.Quit
					}
				}
			}

		case "left", "h":
			// Go back to parent directory
			if len(m.pathHistory) > 1 {
				m.pathHistory = m.pathHistory[:len(m.pathHistory)-1]
				m.currentDir = m.pathHistory[len(m.pathHistory)-1]
				if err := m.loadDirectory(m.currentDir); err != nil {
					logWarn("Error loading directory: %v", err)
				}
			}

		case " ":
			// Toggle selection
			if m.cursor < len(m.items) {
				item := &m.items[m.cursor]
				if _, ok := m.selected[item.path]; ok {
					// Deselect: remove itself and all descendants
					delete(m.selected, item.path)
					item.selected = false
					if item.isDir {
						m.deselectDirRecursive(item.path)
					}
				} else if m.isSelectedByParent(item.path) {
					// Item is selected via parent — toggling it deselects the parent
					// and re-selects all siblings individually (except this one)
					// Find the parent that's selected
					for selPath := range m.selected {
						if strings.HasPrefix(filepath.ToSlash(item.path), filepath.ToSlash(selPath)+"/") {
							// Remove the parent selection
							delete(m.selected, selPath)
							// Re-select all direct children of that parent except this item
							// (recursively handled by adding them to selected)
							m.selectDirRecursive(selPath)
							// Now remove this specific item and its descendants
							delete(m.selected, item.path)
							if item.isDir {
								m.deselectDirRecursive(item.path)
							}
							item.selected = false
							break
						}
					}
				} else {
					// Select: add itself; if directory, recursively add all children
					m.selected[item.path] = struct{}{}
					item.selected = true
					if item.isDir {
						m.selectDirRecursive(item.path)
					}
				}
			}

		case "a":
			// Select all (non-ignored)
			for i := range m.items {
				if !m.items[i].ignored {
					m.selected[m.items[i].path] = struct{}{}
					m.items[i].selected = true
				}
			}

		case "d":
			// Deselect all
			m.selected = make(map[string]struct{})
			for i := range m.items {
				m.items[i].selected = false
			}

		case "i":
			// Toggle showing ignored files
			m.showIgnored = !m.showIgnored
			if err := m.loadDirectory(m.currentDir); err != nil {
				logWarn("Error reloading directory: %v", err)
			}
		}
	}

	return m, nil
}

// View renders the UI
// dirHasSelectedDescendants returns true if any selected path is inside the given directory
func (m fileSelectorModel) dirHasSelectedDescendants(dirPath string) bool {
	// Normalize to forward slashes for consistent prefix matching
	dirSlash := filepath.ToSlash(dirPath) + "/"
	for selPath := range m.selected {
		if strings.HasPrefix(filepath.ToSlash(selPath), dirSlash) {
			return true
		}
	}
	return false
}

// isSelectedByParent returns true if any ancestor directory of the given path is directly selected
func (m fileSelectorModel) isSelectedByParent(itemPath string) bool {
	itemSlash := filepath.ToSlash(itemPath)
	for selPath := range m.selected {
		selSlash := filepath.ToSlash(selPath) + "/"
		if strings.HasPrefix(itemSlash, selSlash) {
			return true
		}
	}
	return false
}

// selectDirRecursive adds all files and subdirectories within dirPath to m.selected
func (m *fileSelectorModel) selectDirRecursive(dirPath string) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return
	}
	for _, entry := range entries {
		childPath := filepath.Join(dirPath, entry.Name())
		m.selected[childPath] = struct{}{}
		if entry.IsDir() {
			m.selectDirRecursive(childPath)
		}
	}
}

// deselectDirRecursive removes all files and subdirectories within dirPath from m.selected
func (m *fileSelectorModel) deselectDirRecursive(dirPath string) {
	dirSlash := filepath.ToSlash(dirPath) + "/"
	for selPath := range m.selected {
		if strings.HasPrefix(filepath.ToSlash(selPath), dirSlash) {
			delete(m.selected, selPath)
		}
	}
}

func (m fileSelectorModel) View() string {
	if m.quitting {
		return "\n  Selection cancelled.\n\n"
	}

	if m.confirmed {
		return "\n  Confirming selection...\n\n"
	}

	var b strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		PaddingBottom(1)
	b.WriteString(headerStyle.Render("Select files and directories to include"))
	b.WriteString("\n\n")

	// Current directory path
	pathStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Italic(true)
	relPath, _ := filepath.Rel(m.rootDir, m.currentDir)
	if relPath == "." {
		relPath = m.rootDir
	}
	b.WriteString(pathStyle.Render("📁 " + relPath))
	b.WriteString("\n\n")

	// Instructions
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Italic(true)
	b.WriteString(helpStyle.Render("↑/↓: navigate  →/Enter: enter dir  ←: go back  Space: toggle  a: select all  d: deselect  i: toggle ignored  q: quit"))
	b.WriteString("\n\n")

	// Items list
	for i, item := range m.items {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
			cursorStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("39")).
				Bold(true)
			cursor = cursorStyle.Render(cursor)
		}

		checkbox := "[ ]"
		_, directlySelected := m.selected[item.path]
		parentSelected := !directlySelected && m.isSelectedByParent(item.path)
		if directlySelected || parentSelected {
			checkbox = "[x]"
			if item.ignored && !parentSelected {
				// Yellow warning for selected ignored files
				checkboxStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color("220")).
					Bold(true)
				checkbox = checkboxStyle.Render(checkbox)
			} else {
				checkboxStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color("42")).
					Bold(true)
				checkbox = checkboxStyle.Render(checkbox)
			}
		} else if item.isDir && m.dirHasSelectedDescendants(item.path) {
			// Partial selection — some children are selected
			checkboxStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("39")).
				Bold(true)
			checkbox = checkboxStyle.Render("[~]")
		} else {
			checkboxStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))
			checkbox = checkboxStyle.Render(checkbox)
		}

		name := item.name
		if item.isDir {
			dirStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("75")).
				Bold(true)
			name = dirStyle.Render(name + "/")
		} else {
			if item.ignored {
				// Yellow warning style for ignored files
				fileStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color("220")).
					Italic(true)
				name = fileStyle.Render(name + " ⚠")
			} else {
				fileStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color("252"))
				name = fileStyle.Render(name)
			}
		}

		// Add directory indicator for navigation
		if item.isDir {
			navHint := lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Italic(true)
			name += " " + navHint.Render("(→ to enter)")
		}

		// Highlight selected row
		if m.cursor == i {
			rowStyle := lipgloss.NewStyle().
				Background(lipgloss.Color("236")).
				PaddingLeft(1).
				PaddingRight(1)
			b.WriteString(rowStyle.Render(fmt.Sprintf("%s %s %s", cursor, checkbox, name)))
		} else {
			b.WriteString(fmt.Sprintf("%s %s %s", cursor, checkbox, name))
		}
		b.WriteString("\n")
	}

	// Footer with selection count
	b.WriteString("\n")
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))
	selectedCount := len(m.selected)
	if selectedCount == 0 {
		b.WriteString(footerStyle.Render("No items selected. Select at least one item to continue."))
	} else {
		countStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)
		b.WriteString(footerStyle.Render(fmt.Sprintf("Selected: %s item(s). Press Enter to confirm.", countStyle.Render(fmt.Sprintf("%d", selectedCount)))))
	}

	return b.String()
}

// interactiveFileSelector provides a Bubble Tea TUI for selecting files/directories
func interactiveFileSelector(rootDir string) ([]string, error) {
	model, err := initialFileSelectorModel(rootDir)
	if err != nil {
		return nil, err
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running TUI: %w", err)
	}

	m := finalModel.(fileSelectorModel)

	if m.quitting {
		return nil, fmt.Errorf("selection cancelled")
	}

	if len(m.selected) == 0 {
		return nil, fmt.Errorf("no items selected")
	}

	// Collect selected paths
	var selectedPaths []string
	for path := range m.selected {
		relPath, err := filepath.Rel(rootDir, path)
		if err == nil {
			selectedPaths = append(selectedPaths, relPath)
		}
	}

	return selectedPaths, nil
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

func writeStructure(writer *bufio.Writer, entries []walkEntry, cfg config) {
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

		// Append optional annotations
		var annotations []string
		if cfg.showSizes && !entry.isDir {
			info, statErr := os.Stat(entry.fullPath)
			if statErr == nil {
				annotations = append(annotations, formatSize(info.Size()))
			}
		}
		if cfg.showExtensions && !entry.isDir {
			ext := filepath.Ext(baseName)
			if ext != "" {
				annotations = append(annotations, strings.TrimPrefix(ext, "."))
			}
		}
		if len(annotations) > 0 {
			lineBuilder.WriteString(" (" + strings.Join(annotations, ", ") + ")")
		}

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
