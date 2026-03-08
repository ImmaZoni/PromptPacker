package main

import (
	"testing"
)

func TestGetLanguageHint(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		// Happy paths
		{"test.go", "go"},
		{"script.js", "javascript"},
		{"app.ts", "typescript"},
		{"main.py", "python"},
		{"Main.java", "java"},
		{"Program.cs", "csharp"},
		{"index.php", "php"},
		{"script.rb", "ruby"},
		{"lib.rs", "rust"},
		{"AppDelegate.swift", "swift"},
		{"Main.kt", "kotlin"},
		{"Script.kts", "kotlin"},
		{"Main.scala", "scala"},
		{"index.html", "html"},
		{"index.htm", "html"},
		{"style.css", "css"},
		{"style.scss", "scss"},
		{"style.sass", "scss"},
		{"style.less", "less"},
		{"config.json", "json"},
		{"config.yaml", "yaml"},
		{"config.yml", "yaml"},
		{"data.xml", "xml"},
		{"query.sql", "sql"},
		{"script.sh", "bash"},
		{"script.bash", "bash"},
		{"script.zsh", "bash"},
		{"script.ps1", "powershell"},
		{"README.md", "markdown"},
		{"README.markdown", "markdown"},
		{".dockerfile", "dockerfile"},
		{".docker", "dockerfile"},
		{".env", "bash"},
		{".gitignore", "gitignore"},
		{"go.mod", "go.mod"},
		{"go.sum", "go.sum"},
		{"config.toml", "toml"},
		{"script.lua", "lua"},
		{"script.perl", "perl"},
		{"script.pl", "perl"},
		{"script.r", "r"},
		{"app.dart", "dart"},
		{"Component.jsx", "jsx"},
		{"Component.tsx", "tsx"},
		{"App.vue", "vue"},
		{"App.svelte", "svelte"},

		// No extension / empty
		{"test.txt", ""},
		{"Makefile", ""},
		{"", ""},

		// Case sensitivity
		{"TEST.GO", "go"},
		{"SCRIPT.JS", "javascript"},

		// Multiple dots
		{"archive.tar.gz", "gz"},

		// Default case: Unknown extension <= 20 chars
		{"file.xyz", "xyz"},
		{"file.abcdefghijklmnopqrst", "abcdefghijklmnopqrst"},

		// Default case: Unknown extension > 20 chars
		{"file.thisextensioniswaytoolongtobevalid", ""},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			actual := getLanguageHint(tt.filename)
			if actual != tt.expected {
				t.Errorf("getLanguageHint(%q) = %q; want %q", tt.filename, actual, tt.expected)
			}
		})
	}
}
