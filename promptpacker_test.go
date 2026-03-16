package main

import (

	"testing"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1536 * 1024, "1.5 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{1024 * 1024 * 1024 * 1024, "1.0 TB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatSize(tt.input)
			if result != tt.expected {
				t.Errorf("formatSize(%d) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"1000", 1000},
		{"1KB", 1024},
		{"1MB", 1024 * 1024},
		{"1GB", 1024 * 1024 * 1024},
		{"500 K", 500 * 1024},
		{" 2 M ", 2 * 1024 * 1024},
		{"10G", 10 * 1024 * 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseSize(tt.input)
			if result != tt.expected {
				t.Errorf("parseSize(%q) = %d; want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsPathIncluded(t *testing.T) {
	tests := []struct {
		name         string
		relPath      string
		absPath      string
		rootDir      string
		includePaths []string
		expected     bool
	}{
		{
			name:         "Exact match",
			relPath:      "src/main.go",
			absPath:      "/project/src/main.go",
			rootDir:      "/project",
			includePaths: []string{"src/main.go"},
			expected:     true,
		},
		{
			name:         "Directory inclusion",
			relPath:      "src/utils/math.go",
			absPath:      "/project/src/utils/math.go",
			rootDir:      "/project",
			includePaths: []string{"src/"},
			expected:     true,
		},
		{
			name:         "Not included",
			relPath:      "tests/math_test.go",
			absPath:      "/project/tests/math_test.go",
			rootDir:      "/project",
			includePaths: []string{"src/"},
			expected:     false,
		},
		{
			name:         "Parent directory of include path (should return true to allow traversal)",
			relPath:      "src",
			absPath:      "/project/src",
			rootDir:      "/project",
			includePaths: []string{"src/main.go"},
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPathIncluded(tt.relPath, tt.absPath, tt.rootDir, tt.includePaths)
			if result != tt.expected {
				t.Errorf("isPathIncluded(%q) = %v; want %v", tt.relPath, result, tt.expected)
			}
		})
	}
}

func TestMergeConfigFile(t *testing.T) {
	// Base configuration from flags
	baseCfg := config{
		mode:             modeFileSpec,
		structureOnly:    false,
		numWorkers:       0, // usually uninitialized or default
		maxFileSizeBytes: 0,
		force:            false,
		excludeExts:      []string{},
		includeExts:      []string{},
	}

	// Configuration from .promptpacker.yml
	fileCfg := projectConfigFile{
		Defaults: projectConfigDefaults{
			Mode:          "auto",
			StructureOnly: true,
			Workers:       4,
			MaxFileSize:   "5MB",
			Force:         true,
			ExcludeExt:    "log,tmp",
			IncludeExt:    "go,md",
		},
	}

	merged := mergeConfigFile(baseCfg, fileCfg)

	if merged.mode != modeAuto {
		t.Errorf("Expected modeAuto, got %v", merged.mode)
	}
	if !merged.structureOnly {
		t.Errorf("Expected structureOnly=true")
	}
	if merged.numWorkers != 4 {
		t.Errorf("Expected numWorkers=4, got %d", merged.numWorkers)
	}
	if merged.maxFileSizeBytes != 5*1024*1024 {
		t.Errorf("Expected maxFileSizeBytes=5242880, got %d", merged.maxFileSizeBytes)
	}
	if !merged.force {
		t.Errorf("Expected force=true")
	}
	if len(merged.excludeExts) != 2 || merged.excludeExts[0] != "log" || merged.excludeExts[1] != "tmp" {
		t.Errorf("Expected excludeExts=[log tmp], got %v", merged.excludeExts)
	}
	if len(merged.includeExts) != 2 || merged.includeExts[0] != "go" || merged.includeExts[1] != "md" {
		t.Errorf("Expected includeExts=[go md], got %v", merged.includeExts)
	}

	// Test that explicit CLI flags (represented by values already set in baseCfg) override file defaults
	cliCfg := config{
		mode:             modeFileSpec,
		structureOnly:    true,
		numWorkers:       8,
		maxFileSizeBytes: 10 * 1024 * 1024,
		force:            false,
		excludeExts:      []string{"bak"},
	}

	// We'll simulate `mergeConfigFile` on `cliCfg` but note that the CLI flags explicitly set aren't easily detectable if they match zeroes.
	// But `mergeConfigFile` handles 0-values in cfg and populates them if file has them.
	// We'll just test that existing non-zero values in cfg aren't overwritten.
	merged2 := mergeConfigFile(cliCfg, fileCfg)

	if merged2.numWorkers != 8 {
		t.Errorf("Expected numWorkers=8 (from CLI), got %d", merged2.numWorkers)
	}
	if merged2.maxFileSizeBytes != 10*1024*1024 {
		t.Errorf("Expected maxFileSizeBytes=10MB (from CLI), got %d", merged2.maxFileSizeBytes)
	}
	if len(merged2.excludeExts) != 1 || merged2.excludeExts[0] != "bak" {
		t.Errorf("Expected excludeExts=[bak] (from CLI), got %v", merged2.excludeExts)
	}
}

func TestCheckIgnoreRules(t *testing.T) {
	tests := []struct {
		name     string
		relPath  string
		isDir    bool
		rules    []gitignoreRule
		ignored  bool
		matched  bool
	}{
		{
			name:    "Match file extension",
			relPath: "test.log",
			isDir:   false,
			rules: []gitignoreRule{
				{pattern: "*.log", patternParts: []string{"*.log"}, isNegated: false, matchDirsOnly: false, isRooted: false},
			},
			ignored: true,
			matched: true,
		},
		{
			name:    "Match directory",
			relPath: "node_modules",
			isDir:   true,
			rules: []gitignoreRule{
				{pattern: "node_modules", patternParts: []string{"node_modules"}, isNegated: false, matchDirsOnly: true, isRooted: false},
			},
			ignored: true,
			matched: true,
		},
		{
			name:    "Match negated file",
			relPath: "important.log",
			isDir:   false,
			rules: []gitignoreRule{
				{pattern: "*.log", patternParts: []string{"*.log"}, isNegated: false, matchDirsOnly: false, isRooted: false},
				{pattern: "!important.log", patternParts: []string{"important.log"}, isNegated: true, matchDirsOnly: false, isRooted: false},
			},
			ignored: false,
			matched: true,
		},
		{
			name:    "Double asterisk wildcard",
			relPath: "src/utils/test.log",
			isDir:   false,
			rules: []gitignoreRule{
				{pattern: "src/**/*.log", patternParts: []string{"src", "**", "*.log"}, isNegated: false, matchDirsOnly: false, isRooted: false},
			},
			ignored: true,
			matched: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ignored, matched := checkIgnoreRules(tt.relPath, tt.isDir, tt.rules)
			if ignored != tt.ignored {
				t.Errorf("checkIgnoreRules(%q) ignored = %v; want %v", tt.relPath, ignored, tt.ignored)
			}
			if matched != tt.matched {
				t.Errorf("checkIgnoreRules(%q) matched = %v; want %v", tt.relPath, matched, tt.matched)
			}
		})
	}
}

func TestShouldIgnoreHierarchical(t *testing.T) {
	// Not practically testing file I/O here because it depends on gitignoreCache.
	// But we can test it handles cache and directory traversal by caching directly.
	cacheMutex.Lock()
	gitignoreCache["/fake/project"] = []gitignoreRule{
		{pattern: "*.log", patternParts: []string{"*.log"}, isNegated: false, matchDirsOnly: false, isRooted: false},
	}
	gitignoreCache["/fake/project/src"] = []gitignoreRule{
		{pattern: "!important.log", patternParts: []string{"important.log"}, isNegated: true, matchDirsOnly: false, isRooted: false},
	}
	cacheMutex.Unlock()

	ignored, decided := shouldIgnoreHierarchical("/fake/project/test.log", false, "/fake/project")
	if !ignored || !decided {
		t.Errorf("Expected test.log to be ignored (ignored=%v, decided=%v)", ignored, decided)
	}

	ignored, decided = shouldIgnoreHierarchical("/fake/project/src/important.log", false, "/fake/project")
	if ignored || !decided {
		t.Errorf("Expected important.log to NOT be ignored (ignored=%v, decided=%v)", ignored, decided)
	}
}
