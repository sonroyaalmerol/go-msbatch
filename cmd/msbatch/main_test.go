package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsBatchFile(t *testing.T) {
	// Create a temp directory for test files
	tmpDir := t.TempDir()

	// Create test batch files
	batFile := filepath.Join(tmpDir, "test.bat")
	if err := os.WriteFile(batFile, []byte("@echo off\necho hello"), 0644); err != nil {
		t.Fatal(err)
	}

	cmdFile := filepath.Join(tmpDir, "test.cmd")
	if err := os.WriteFile(cmdFile, []byte("@echo off\necho world"), 0644); err != nil {
		t.Fatal(err)
	}

	nonBatFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(nonBatFile, []byte("not a batch file"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		line      string
		wantFile  string
		wantIsBat bool
	}{
		{
			name:      "simple .bat file",
			line:      batFile,
			wantFile:  batFile,
			wantIsBat: true,
		},
		{
			name:      "simple .cmd file",
			line:      cmdFile,
			wantFile:  cmdFile,
			wantIsBat: true,
		},
		{
			name:      "./ style path",
			line:      "./" + filepath.Base(batFile),
			wantFile:  "./" + filepath.Base(batFile),
			wantIsBat: true, // File exists at ./ path after chdir
		},
		{
			name:      "non-batch file",
			line:      nonBatFile,
			wantFile:  "",
			wantIsBat: false,
		},
		{
			name:      "non-existent file",
			line:      "/nonexistent/file.bat",
			wantFile:  "",
			wantIsBat: false,
		},
		{
			name:      "empty line",
			line:      "",
			wantFile:  "",
			wantIsBat: false,
		},
		{
			name:      "regular command",
			line:      "echo hello",
			wantFile:  "",
			wantIsBat: false,
		},
		{
			name:      "batch file with trailing spaces",
			line:      batFile + "   ",
			wantFile:  batFile,
			wantIsBat: true,
		},
	}

	// Change to temp dir for relative path tests
	oldDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFile, gotIsBat := isBatchFile(tt.line)
			if gotIsBat != tt.wantIsBat {
				t.Errorf("isBatchFile(%q) = %v, want %v", tt.line, gotIsBat, tt.wantIsBat)
			}
			if tt.wantIsBat && gotFile != tt.wantFile {
				t.Errorf("isBatchFile(%q) file = %q, want %q", tt.line, gotFile, tt.wantFile)
			}
		})
	}
}

func TestIsBatchFileRelativePath(t *testing.T) {
	tmpDir := t.TempDir()

	batFile := filepath.Join(tmpDir, "relative.bat")
	if err := os.WriteFile(batFile, []byte("@echo off\necho relative"), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to temp dir
	oldDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	// Test relative path ./relative.bat
	gotFile, gotIsBat := isBatchFile("./relative.bat")
	if !gotIsBat {
		t.Errorf("isBatchFile('./relative.bat') = %v, want true", gotIsBat)
	}
	if gotIsBat && gotFile != "./relative.bat" {
		t.Errorf("isBatchFile('./relative.bat') file = %q, want './relative.bat'", gotFile)
	}

	// Test just the filename (should work since we're in the same directory)
	gotFile2, gotIsBat2 := isBatchFile("relative.bat")
	if !gotIsBat2 {
		t.Errorf("isBatchFile('relative.bat') = %v, want true", gotIsBat2)
	}
	if gotIsBat2 && gotFile2 != "relative.bat" {
		t.Errorf("isBatchFile('relative.bat') file = %q, want 'relative.bat'", gotFile2)
	}
}
