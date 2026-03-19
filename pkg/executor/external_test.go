package executor

import (
	"strings"
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

func TestProcessArgForNative(t *testing.T) {
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
			name:     "quoted string with space - quotes preserved",
			input:    `"hello world"`,
			expected: `"hello world"`,
		},
		{
			name:     "quoted string without space - quotes preserved",
			input:    `"hello"`,
			expected: `"hello"`,
		},
		{
			name:     "escaped quote inside quotes becomes literal quote",
			input:    `"say \"hello\""`,
			expected: `"say "hello""`,
		},
		{
			name:     "assignment with quoted value containing space",
			input:    `Instrument="King Radar"`,
			expected: `Instrument="King Radar"`,
		},
		{
			name:     "assignment with simple value",
			input:    "LstFile=N",
			expected: "LstFile=N",
		},
		{
			name:     "mixed quoted and unquoted - quotes preserved",
			input:    `unquoted "quoted part" more`,
			expected: `unquoted "quoted part" more`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "just quotes",
			input:    `""`,
			expected: `""`,
		},
		{
			name:     "gawk program with escaped quotes",
			input:    `"{print \"hello\"}"`,
			expected: `"{print "hello"}"`,
		},
		{
			name:     "path with space in quotes",
			input:    `"C:\Program Files\app.exe"`,
			expected: `"C:\Program Files\app.exe"`,
		},
		{
			name:     "multiple escaped quotes",
			input:    `"echo \"hello\" and \"world\""`,
			expected: `"echo "hello" and "world""`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processArgForNative(tt.input)
			if got != tt.expected {
				t.Errorf("processArgForNative(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestProcessArgForNativePreservesWordBoundaries(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedCount int
		description   string
	}{
		{
			name:          "arg with space inside quotes remains single arg",
			input:         `Instrument="King Radar"`,
			expectedCount: 1,
			description:   "space inside quotes should not create separate args when passed to exec.Command",
		},
		{
			name:          "simple arg without space",
			input:         "LstFile=N",
			expectedCount: 1,
			description:   "simple arg should be single arg",
		},
		{
			name:          "gawk print statement with space",
			input:         `"{print $0}"`,
			expectedCount: 1,
			description:   "space inside quoted gawk program should not create separate args",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processArgForNative(tt.input)
			if got != tt.input {
				t.Errorf("processArgForNative(%q) = %q, want %q", tt.input, got, tt.input)
			}
			if strings.Count(got, `"`)%2 != 0 {
				t.Errorf("processArgForNative(%q) has unbalanced quotes", tt.input)
			}
		})
	}
}

func TestStripExeArgVsProcessArgForNative(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		stripResult   string
		processResult string
	}{
		{
			name:          "arg with space - stripExeArg breaks it, processArgForNative preserves",
			input:         `Instrument="King Radar"`,
			stripResult:   `Instrument=King Radar`,
			processResult: `Instrument="King Radar"`,
		},
		{
			name:          "simple quoted string with space",
			input:         `"hello world"`,
			stripResult:   `hello world`,
			processResult: `"hello world"`,
		},
		{
			name:          "escaped quotes are handled differently",
			input:         `"say \"hello\""`,
			stripResult:   `say "hello"`,
			processResult: `"say "hello""`,
		},
		{
			name:          "no quotes - same result",
			input:         `hello`,
			stripResult:   `hello`,
			processResult: `hello`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stripGot := stripExeArg(tt.input)
			if stripGot != tt.stripResult {
				t.Errorf("stripExeArg(%q) = %q, want %q", tt.input, stripGot, tt.stripResult)
			}
			processGot := processArgForNative(tt.input)
			if processGot != tt.processResult {
				t.Errorf("processArgForNative(%q) = %q, want %q", tt.input, processGot, tt.processResult)
			}
		})
	}
}
