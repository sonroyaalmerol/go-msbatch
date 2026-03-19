package pathutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

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
			name:     "backtick quoted",
			input:    "`hello world`",
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
			got := DriveMount(tt.letter)
			if got != tt.expected {
				t.Errorf("DriveMount(%q) = %q, want %q", tt.letter, got, tt.expected)
			}
		})
	}
}

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
			expected: "/server/share/file.txt",
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
			got := IsPathLike(tt.input)
			if got != tt.expected {
				t.Errorf("IsPathLike(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsWindowsPathLike(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "windows path with backslash",
			input:    `C:\Users\test`,
			expected: true,
		},
		{
			name:     "windows path with drive letter only",
			input:    `D:data`,
			expected: true,
		},
		{
			name:     "UNC path",
			input:    `\\server\share`,
			expected: true,
		},
		{
			name:     "unix path",
			input:    "/home/user/file",
			expected: false,
		},
		{
			name:     "relative path with forward slash",
			input:    "some/path/file",
			expected: false,
		},
		{
			name:     "simple filename",
			input:    "file.txt",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsWindowsPathLike(tt.input)
			if got != tt.expected {
				t.Errorf("IsWindowsPathLike(%q) = %v, want %v", tt.input, got, tt.expected)
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
			got := MapArg(tt.input)
			if got != tt.expected {
				t.Errorf("MapArg(%q) = %q, want %q", tt.input, got, tt.expected)
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
			got := MapArg(tt.input)
			if got != tt.shouldMatch {
				t.Errorf("MapArg(%q) = %q, want %q", tt.input, got, tt.shouldMatch)
			}
		})
	}
}

func TestUnixToWinePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "root path",
			input:    "/",
			expected: "Z:\\",
		},
		{
			name:     "absolute unix path under root",
			input:    "/data/Data/Flight/file.txt",
			expected: "Z:\\data\\Data\\Flight\\file.txt",
		},
		{
			name:     "path under mnt c",
			input:    "/mnt/c/Users/test",
			expected: "C:\\Users\\test",
		},
		{
			name:     "path under mnt d",
			input:    "/mnt/d/data/file.txt",
			expected: "D:\\data\\file.txt",
		},
		{
			name:     "relative path unchanged",
			input:    "relative/path",
			expected: "relative/path",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "Z:\\",
		},
		{
			name:     "mnt c root",
			input:    "/mnt/c",
			expected: "C:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UnixToWinePath(tt.input)
			if got != tt.expected {
				t.Errorf("UnixToWinePath(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestMapArgForWine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "non-path argument unchanged",
			input:    "--option",
			expected: "--option",
		},
		{
			name:     "simple filename unchanged",
			input:    "file.txt",
			expected: "file.txt",
		},
		{
			name:     "unix path unchanged (no case resolution without actual file)",
			input:    "/home/user/file.txt",
			expected: "/home/user/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapArgForWine(tt.input)
			if got != tt.expected {
				t.Errorf("MapArgForWine(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestMapArgForWineCaseResolution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Wine path case resolution test on Windows")
	}

	tmpDir := t.TempDir()

	dataDir := filepath.Join(tmpDir, "DataFolder")
	if err := os.Mkdir(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	dataFile := filepath.Join(dataDir, "InputFile.txt")
	if err := os.WriteFile(dataFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	relativePath := filepath.Join(tmpDir, "datafolder", "inputfile.txt")
	resolved := MapArgForWine(relativePath)
	expected := filepath.Join(tmpDir, "DataFolder", "InputFile.txt")
	if resolved != expected {
		t.Errorf("MapArgForWine(%q) = %q, want %q", relativePath, resolved, expected)
	}
}

func TestMapArgForWineWindowsPathCaseResolution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Wine path case resolution test on Windows")
	}

	tmpDir := t.TempDir()

	dataDir := filepath.Join(tmpDir, "DataFolder")
	if err := os.Mkdir(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	dataFile := filepath.Join(dataDir, "InputFile.txt")
	if err := os.WriteFile(dataFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	unixPath := filepath.Join(tmpDir, "datafolder", "inputfile.txt")
	windowsPath := UnixToWinePath(unixPath)
	resolved := MapArgForWine(windowsPath)
	expected := UnixToWinePath(filepath.Join(tmpDir, "DataFolder", "InputFile.txt"))
	if resolved != expected {
		t.Errorf("MapArgForWine(%q) = %q, want %q", windowsPath, resolved, expected)
	}
}

func TestHasWildcard(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"*.txt", true},
		{"file?.txt", true},
		{"file[abc].txt", true},
		{"file.txt", false},
		{"", false},
		{"path/to/file", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := HasWildcard(tt.input); got != tt.expected {
				t.Errorf("HasWildcard(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestMatchCaseInsensitive(t *testing.T) {
	tests := []struct {
		pattern  string
		name     string
		expected bool
	}{
		{"*.txt", "FILE.TXT", true},
		{"*.txt", "file.txt", true},
		{"*.TXT", "file.txt", true},
		{"file?.txt", "FILE1.TXT", true},
		{"file?.txt", "file1.txt", true},
		{"FILE?.TXT", "file1.txt", true},
		{"*.txt", "file.log", false},
		{"file?.txt", "file.txt", false},
		{"file*", "file.txt", true},
		{"FILE*", "file.txt", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.name, func(t *testing.T) {
			if got := MatchCaseInsensitive(tt.pattern, tt.name); got != tt.expected {
				t.Errorf("MatchCaseInsensitive(%q, %q) = %v, want %v", tt.pattern, tt.name, got, tt.expected)
			}
		})
	}
}

func TestGlobCaseInsensitive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping case-insensitive glob test on Windows")
	}

	tmpDir := t.TempDir()

	files := []string{
		filepath.Join(tmpDir, "File1.txt"),
		filepath.Join(tmpDir, "File2.TXT"),
		filepath.Join(tmpDir, "Other.log"),
		filepath.Join(tmpDir, "DataFolder", "InputFile.txt"),
	}

	dataDir := filepath.Join(tmpDir, "DataFolder")
	if err := os.Mkdir(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	for _, f := range files {
		if err := os.WriteFile(f, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", f, err)
		}
	}

	t.Run("lowercase pattern with uppercase files", func(t *testing.T) {
		matches, err := GlobCaseInsensitive(filepath.Join(tmpDir, "*.txt"))
		if err != nil {
			t.Fatalf("GlobCaseInsensitive failed: %v", err)
		}
		if len(matches) != 2 {
			t.Errorf("Expected 2 matches, got %d: %v", len(matches), matches)
		}
	})

	t.Run("uppercase pattern with mixed case files", func(t *testing.T) {
		matches, err := GlobCaseInsensitive(filepath.Join(tmpDir, "*.TXT"))
		if err != nil {
			t.Fatalf("GlobCaseInsensitive failed: %v", err)
		}
		if len(matches) != 2 {
			t.Errorf("Expected 2 matches, got %d: %v", len(matches), matches)
		}
	})

	t.Run("question mark wildcard case insensitive", func(t *testing.T) {
		matches, err := GlobCaseInsensitive(filepath.Join(tmpDir, "file?.txt"))
		if err != nil {
			t.Fatalf("GlobCaseInsensitive failed: %v", err)
		}
		if len(matches) != 2 {
			t.Errorf("Expected 2 matches, got %d: %v", len(matches), matches)
		}
	})

	t.Run("no matches returns empty slice", func(t *testing.T) {
		matches, err := GlobCaseInsensitive(filepath.Join(tmpDir, "*.nonexistent"))
		if err != nil {
			t.Fatalf("GlobCaseInsensitive failed: %v", err)
		}
		if len(matches) != 0 {
			t.Errorf("Expected 0 matches, got %d: %v", len(matches), matches)
		}
	})

	t.Run("nested path case insensitive", func(t *testing.T) {
		matches, err := GlobCaseInsensitive(filepath.Join(tmpDir, "datafolder", "*.txt"))
		if err != nil {
			t.Fatalf("GlobCaseInsensitive failed: %v", err)
		}
		if len(matches) != 1 {
			t.Errorf("Expected 1 match, got %d: %v", len(matches), matches)
		}
	})
}
