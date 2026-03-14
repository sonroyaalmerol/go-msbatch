package tests

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

func TestIntegration(t *testing.T) {
	files, err := filepath.Glob("*.bat")
	if err != nil {
		t.Fatal(err)
	}

	for _, batFile := range files {
		t.Run(batFile, func(t *testing.T) {
			content, err := os.ReadFile(batFile)
			if err != nil {
				t.Fatal(err)
			}

			expectedFile := strings.TrimSuffix(batFile, ".bat") + ".out"
			expected, err := os.ReadFile(expectedFile)
			if err != nil {
				t.Logf("Warning: No .out file for %s, skipping comparison", batFile)
				return
			}

			env := processor.NewEnvironment(true)
			var stdout bytes.Buffer
			proc := processor.New(env, []string{batFile, "A", "B", "C"})
			proc.Stdout = &stdout
			proc.Echo = false // Match @echo off behavior for comparison

			src := string(content)
			src = processor.Phase0ReadLine(src)
			nodes := processor.ParseExpanded(src)

			err = proc.Execute(nodes)
			if err != nil {
				t.Errorf("Execute failed: %v", err)
			}

			got := normalize(stdout.String())
			want := normalize(string(expected))

			if got != want {
				t.Errorf("Output mismatch for %s\nGOT:\n%s\nWANT:\n%s", batFile, got, want)
			}
		})
	}
}

func normalize(s string) string {
	lines := strings.Split(s, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \r\t")
		if trimmed != "" || len(result) > 0 { // Skip leading empty lines, but keep internal ones
			result = append(result, trimmed)
		}
	}
	// Trim trailing empty lines
	for len(result) > 0 && result[len(result)-1] == "" {
		result = result[:len(result)-1]
	}
	return strings.Join(result, "\n")
}
