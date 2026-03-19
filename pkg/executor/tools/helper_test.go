package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/pathutil"
)

func TestResolveWildcardDst_WindowsPaths(t *testing.T) {
	tests := []struct {
		name       string
		srcPath    string
		srcPattern string
		dstPattern string
		dst        string
		wantBase   string
	}{
		{
			name:       "windows_path_wildcard_star",
			srcPath:    "/data/test.txt",
			srcPattern: `Z:\data\*.txt`,
			dstPattern: `Z:\data\*.bak`,
			dst:        "/data",
			wantBase:   "test.bak",
		},
		{
			name:       "windows_path_with_subdirs",
			srcPath:    "/data/Data/Flight/test.clb",
			srcPattern: `Z:\data\Data\Flight\*.clb`,
			dstPattern: `Z:\data\Data\Flight\*.bak`,
			dst:        "/data/Data/Flight",
			wantBase:   "test.bak",
		},
		{
			name:       "windows_path_question_mark",
			srcPath:    "/data/file1.txt",
			srcPattern: `Z:\data\file?.txt`,
			dstPattern: `Z:\data\new?.txt`,
			dst:        "/data",
			wantBase:   "new1.txt",
		},
		{
			name:       "unix_path_wildcard",
			srcPath:    "/data/test.txt",
			srcPattern: "/data/*.txt",
			dstPattern: "/data/*.bak",
			dst:        "/data",
			wantBase:   "test.bak",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveWildcardDst(tt.srcPath, tt.srcPattern, tt.dstPattern, tt.dst)
			gotBase := filepath.Base(got)
			if gotBase != tt.wantBase {
				t.Errorf("ResolveWildcardDst() base = %q, want %q (full path: %s)", gotBase, tt.wantBase, got)
			}
		})
	}
}

func TestResolveWildcardDst_FileCreation(t *testing.T) {
	tmpDir := t.TempDir()

	writeFile(t, tmpDir, "test.clb", "content")

	srcPath := filepath.Join(tmpDir, "test.clb")
	srcPattern := `Z:\data\Data\Flight\*.clb`
	dstPattern := `Z:\data\Data\Flight\*.bak`
	dst := tmpDir

	result := ResolveWildcardDst(pathutil.MapPath(srcPath), srcPattern, dstPattern, dst)

	expectedFile := filepath.Join(tmpDir, "test.bak")
	if result != expectedFile {
		t.Errorf("ResolveWildcardDst() = %q, want %q", result, expectedFile)
	}

	f, err := os.Create(result)
	if err != nil {
		t.Fatalf("failed to create file at result path %q: %v", result, err)
	}
	f.Close()

	if _, err := os.Stat(expectedFile); err != nil {
		t.Errorf("expected file %q to exist: %v", expectedFile, err)
	}

	badFile := filepath.Join(tmpDir, "Z:\\data\\Data\\Flight\\test.bak")
	if _, err := os.Stat(badFile); err == nil {
		t.Errorf("file with Windows path as filename should NOT exist: %q", badFile)
	}
}

func TestSubstituteWildcard_WindowsPatterns(t *testing.T) {
	tests := []struct {
		name       string
		srcName    string
		srcPattern string
		dstPattern string
		want       string
	}{
		{
			name:       "windows_pattern_star",
			srcName:    "test.clb",
			srcPattern: `*.clb`,
			dstPattern: `*.bak`,
			want:       "test.bak",
		},
		{
			name:       "unix_pattern_star",
			srcName:    "test.clb",
			srcPattern: "*.clb",
			dstPattern: "*.bak",
			want:       "test.bak",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SubstituteWildcard(tt.srcName, tt.srcPattern, tt.dstPattern)
			if got != tt.want {
				t.Errorf("SubstituteWildcard() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGlobOrLiteral_WindowsPath(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, tmpDir, "test.clb", "content")

	winPath := `Z:\` + filepath.ToSlash(tmpDir)[1:] + `\*.clb`
	mappedPath := pathutil.MapPath(winPath)

	results := GlobOrLiteral(mappedPath)

	if len(results) == 0 {
		t.Error("GlobOrLiteral should find at least one match")
		return
	}

	found := false
	for _, r := range results {
		if filepath.Base(r) == "test.clb" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("GlobOrLiteral should find test.clb, got: %v", results)
	}
}
