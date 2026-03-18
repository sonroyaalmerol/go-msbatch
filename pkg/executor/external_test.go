package executor

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestErrCommandNotFound(t *testing.T) {
	if ErrCommandNotFound == nil {
		t.Error("ErrCommandNotFound should not be nil")
	}
	if ErrCommandNotFound.Error() != "command not found" {
		t.Errorf("ErrCommandNotFound.Error() = %q, want %q", ErrCommandNotFound.Error(), "command not found")
	}
}

func TestStripExeArg(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no quotes",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "quoted string",
			input:    `"hello world"`,
			expected: "hello world",
		},
		{
			name:     "escaped quote inside quotes",
			input:    `"say \"hello\""`,
			expected: `say "hello"`,
		},
		{
			name:     "mixed quoted and unquoted",
			input:    `unquoted "quoted part" more`,
			expected: `unquoted quoted part more`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "just quotes",
			input:    `""`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripExeArg(tt.input)
			if got != tt.expected {
				t.Errorf("stripExeArg(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsPathLike(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "absolute path",
			input:    "/home/user/file",
			expected: true,
		},
		{
			name:     "relative path with dot slash",
			input:    "./file.txt",
			expected: true,
		},
		{
			name:     "parent directory relative path",
			input:    "../file.txt",
			expected: true,
		},
		{
			name:     "path with slash in middle",
			input:    "some/path/file",
			expected: true,
		},
		{
			name:     "simple filename",
			input:    "file.txt",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "just a dot",
			input:    ".",
			expected: false,
		},
		{
			name:     "just two dots",
			input:    "..",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPathLike(tt.input)
			if got != tt.expected {
				t.Errorf("isPathLike(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestMapArg(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "windows path with backslash",
			input:    `C:\Users\test`,
			expected: `/mnt/c/Users/test`,
		},
		{
			name:     "windows path with drive letter",
			input:    `D:\data`,
			expected: `/mnt/d/data`,
		},
		{
			name:     "simple filename unchanged",
			input:    "file.txt",
			expected: "file.txt",
		},
		{
			name:     "non-path argument unchanged",
			input:    "--option",
			expected: "--option",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapArg(tt.input)
			if got != tt.expected {
				t.Errorf("mapArg(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestMapArgUnixPathCaseResolution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix path case resolution test on Windows")
	}

	tmpDir := t.TempDir()

	actualDir := filepath.Join(tmpDir, "ActualCase")
	if err := os.Mkdir(actualDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	actualFile := filepath.Join(actualDir, "File.txt")
	if err := os.WriteFile(actualFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name        string
		input       string
		shouldMatch string
	}{
		{
			name:        "lowercase dir with uppercase actual",
			input:       filepath.Join(tmpDir, "actualcase", "File.txt"),
			shouldMatch: actualFile,
		},
		{
			name:        "uppercase dir with lowercase file",
			input:       filepath.Join(tmpDir, "ACTUALCASE", "file.txt"),
			shouldMatch: actualFile,
		},
		{
			name:        "mixed case path",
			input:       filepath.Join(tmpDir, "AcTuAlCaSe", "FiLe.TxT"),
			shouldMatch: actualFile,
		},
		{
			name:        "exact match",
			input:       actualFile,
			shouldMatch: actualFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapArg(tt.input)
			if got != tt.shouldMatch {
				t.Errorf("mapArg(%q) = %q, want %q", tt.input, got, tt.shouldMatch)
			}
		})
	}
}
