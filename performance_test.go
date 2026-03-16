package main

import (
	"bufio"
	"io"
	"os"
	"testing"
)

func BenchmarkWriteStructure(b *testing.B) {
	// Create some dummy entries
	numEntries := 1000
	entries := make([]walkEntry, numEntries)

	// Create a dummy file for os.Stat if testing the old behavior, or just use size property.
	f, _ := os.CreateTemp("", "bench_file")
	defer os.Remove(f.Name())
	f.Write([]byte("dummy content"))
	f.Close()
	info, _ := os.Stat(f.Name())

	for i := 0; i < numEntries; i++ {
		entries[i] = walkEntry{
			relPath:  "dummy_file.txt",
			fullPath: f.Name(),
			isDir:    false,
			depth:    1,
			size:     info.Size(),
		}
	}

	cfg := config{
		showSizes:      true,
		showExtensions: true,
	}

	// Null writer
	writer := bufio.NewWriter(io.Discard)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writeStructure(writer, entries, cfg)
	}
}
