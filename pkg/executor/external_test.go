package executor

import (
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
			name:     "unix path unchanged",
			input:    "/home/user/file",
			expected: "/home/user/file",
		},
		{
			name:     "simple filename unchanged",
			input:    "file.txt",
			expected: "file.txt",
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
