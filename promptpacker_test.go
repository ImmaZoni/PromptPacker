package main

import (
	"testing"
)

func TestYamlToJSON(t *testing.T) {
	testYAML := `
defaults:
  mode: auto
  structure-only: true
profiles:
  llm:
    mode: auto
    exclude-ext: log,tmp
  structure:
    structure-only: true
    max-depth: 3
`
	expected := `{"defaults":{"mode":"auto","structure-only":true},"profiles":{"llm":{"mode":"auto","exclude-ext":"log,tmp"},"structure":{"structure-only":true,"max-depth":3}}}`
	result := yamlToJSON(testYAML)
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}
