package executor

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

func TestRunBatchFileEnablesBatchMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "args.bat")
	if err := os.WriteFile(path, []byte("@echo off\r\necho A=%1 B=%2 ALL=%*\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	env := processor.NewEmptyEnvironment(false)
	proc := processor.New(env, nil, New())
	var stdout bytes.Buffer
	proc.Stdout = &stdout

	if err := runBatchFile(proc, path, []string{"one", "two"}, true); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "A=one B=two ALL=one two\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if env.BatchMode() {
		t.Fatal("batch mode was not restored")
	}
}

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
			name:     "entirely quoted string - strip outer quotes",
			input:    `"hello world"`,
			expected: "hello world",
		},
		{
			name:     "entirely quoted with escaped quotes inside",
			input:    `"say \"hello\""`,
			expected: `say "hello"`,
		},
		{
			name:     "embedded quotes preserved",
			input:    `unquoted "quoted part" more`,
			expected: `unquoted "quoted part" more`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "just quotes - empty after strip",
			input:    `""`,
			expected: "",
		},
		{
			name:     "assignment with quoted value - quotes preserved",
			input:    `Instrument="King Radar"`,
			expected: `Instrument="King Radar"`,
		},
		{
			name:     "gawk program - outer quotes stripped, inner quotes unescaped",
			input:    `"{print \"hello\"}"`,
			expected: `{print "hello"}`,
		},
		{
			name:     "path with space - outer quotes stripped",
			input:    `"C:\Program Files\app.exe"`,
			expected: `C:\Program Files\app.exe`,
		},
		{
			name:     "multiple escaped quotes in outer-quoted string",
			input:    `"echo \"hello\" and \"world\""`,
			expected: `echo "hello" and "world"`,
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
