package main

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkWriteStructure(b *testing.B) {
	var entries []walkEntry
	filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		absPath, _ := filepath.Abs(path)

		entries = append(entries, walkEntry{
			relPath:  path,
			fullPath: absPath,
			isDir:    false,
			depth:    1,
		})
		return nil
	})

	cfg := config{
		showSizes: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		writer := bufio.NewWriter(&buf)
		writeStructure(writer, entries, cfg)
		writer.Flush()
	}
}
