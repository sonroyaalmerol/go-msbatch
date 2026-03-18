package processor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMapPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Z drive root",
			input:    `Z:\`,
			expected: "/",
		},
		{
			name:     "Z drive path",
			input:    `Z:\home\user\file.txt`,
			expected: "/home/user/file.txt",
		},
		{
			name:     "Z drive with forward slashes",
			input:    `Z:/home/user/file.txt`,
			expected: "/home/user/file.txt",
		},
		{
			name:     "C drive path",
			input:    `C:\Users\test\file.txt`,
			expected: "/mnt/c/Users/test/file.txt",
		},
		{
			name:     "D drive path",
			input:    `D:\data\file.txt`,
			expected: "/mnt/d/data/file.txt",
		},
		{
			name:     "UNC path",
			input:    `\\server\share\file.txt`,
			expected: "/server/share/file.txt", // filepath.Clean normalizes // to /
		},
		{
			name:     "Unix path unchanged",
			input:    "/home/user/file.txt",
			expected: "/home/user/file.txt",
		},
		{
			name:     "Relative path unchanged",
			input:    "relative/path/file.txt",
			expected: "relative/path/file.txt",
		},
		{
			name:     "Quoted path",
			input:    `"Z:\home\user\file.txt"`,
			expected: "/home/user/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapPath(tt.input)
			if got != tt.expected {
				t.Errorf("MapPath(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestDriveMount(t *testing.T) {
	tests := []struct {
		name     string
		letter   byte
		expected string
	}{
		{
			name:     "Z drive maps to root",
			letter:   'Z',
			expected: "",
		},
		{
			name:     "z drive (lowercase) maps to root",
			letter:   'z',
			expected: "",
		},
		{
			name:     "C drive maps to /mnt/c",
			letter:   'C',
			expected: "/mnt/c",
		},
		{
			name:     "D drive maps to /mnt/d",
			letter:   'D',
			expected: "/mnt/d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := driveMount(tt.letter)
			if got != tt.expected {
				t.Errorf("driveMount(%q) = %q, want %q", tt.letter, got, tt.expected)
			}
		})
	}
}

func TestStripQuotes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "double quoted",
			input:    `"hello world"`,
			expected: "hello world",
		},
		{
			name:     "single quoted",
			input:    `'hello world'`,
			expected: "hello world",
		},
		{
			name:     "no quotes",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "empty quotes",
			input:    `""`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripQuotes(tt.input)
			if got != tt.expected {
				t.Errorf("StripQuotes(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestResolveCaseInsensitive(t *testing.T) {
	tmpDir := t.TempDir()

	lowerDir := filepath.Join(tmpDir, "lowercase")
	upperDir := filepath.Join(tmpDir, "UPPERCASE")
	mixedDir := filepath.Join(tmpDir, "MiXeDCaSe")

	for _, dir := range []string{lowerDir, upperDir, mixedDir} {
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}
	}

	lowerFile := filepath.Join(lowerDir, "file.txt")
	upperFile := filepath.Join(upperDir, "FILE.TXT")
	mixedFile := filepath.Join(mixedDir, "MiXeD.txt")

	for _, file := range []string{lowerFile, upperFile, mixedFile} {
		if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "exact match lowercase dir",
			input:    lowerDir,
			expected: lowerDir,
		},
		{
			name:     "uppercase input for lowercase dir",
			input:    filepath.Join(tmpDir, "LOWERCASE"),
			expected: lowerDir,
		},
		{
			name:     "mixed case input for uppercase dir",
			input:    filepath.Join(tmpDir, "uppercase"),
			expected: upperDir,
		},
		{
			name:     "exact match mixed case dir",
			input:    mixedDir,
			expected: mixedDir,
		},
		{
			name:     "wrong case for mixed case dir",
			input:    filepath.Join(tmpDir, "mixedcase"),
			expected: mixedDir,
		},
		{
			name:     "file with wrong case in dir",
			input:    filepath.Join(tmpDir, "lowercase", "FILE.TXT"),
			expected: lowerFile,
		},
		{
			name:     "nested path with wrong case",
			input:    filepath.Join(tmpDir, "MIXEDCASE", "mIxEd.TxT"),
			expected: mixedFile,
		},
		{
			name:     "nonexistent path returns input",
			input:    filepath.Join(tmpDir, "nonexistent", "file.txt"),
			expected: filepath.Join(tmpDir, "nonexistent", "file.txt"),
		},
		{
			name:     "relative path with dot",
			input:    "./some/path",
			expected: "./some/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveCaseInsensitive(tt.input)
			if got != tt.expected {
				t.Errorf("ResolveCaseInsensitive(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
