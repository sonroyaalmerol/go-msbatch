package processor

import (
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
