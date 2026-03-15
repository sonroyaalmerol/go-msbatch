package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestXcopySingleFile(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeFile(t, src, "hello.txt", "hello")

	p, _, _ := newProc(nil)
	Xcopy(p, cmd("xcopy", filepath.Join(src, "hello.txt"), filepath.Join(dst, "hello.txt")))

	if errorLevel(p) != "0" {
		t.Errorf("expected ERRORLEVEL 0, got %s", errorLevel(p))
	}
	if _, err := os.Stat(filepath.Join(dst, "hello.txt")); err != nil {
		t.Error("expected destination file to exist")
	}
}

func TestXcopyRecursive(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeFile(t, src, "a.txt", "a")
	writeFile(t, src, filepath.Join("sub", "b.txt"), "b")

	p, _, _ := newProc(nil)
	Xcopy(p, cmd("xcopy", src, dst, "/s", "/i"))

	if errorLevel(p) != "0" {
		t.Errorf("expected ERRORLEVEL 0, got %s", errorLevel(p))
	}
	if _, err := os.Stat(filepath.Join(dst, "a.txt")); err != nil {
		t.Error("expected a.txt in destination")
	}
	if _, err := os.Stat(filepath.Join(dst, "sub", "b.txt")); err != nil {
		t.Error("expected sub/b.txt in destination")
	}
}

func TestXcopyListOnly(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeFile(t, src, "data.txt", "data")

	p, out, _ := newProc(nil)
	Xcopy(p, cmd("xcopy", src, dst, "/l", "/i"))

	// /L should report files but not actually copy them.
	if strings.Contains(out.String(), "0 File(s)") {
		t.Error("/L should list the file, not report 0 files")
	}
	entries, _ := os.ReadDir(dst)
	if len(entries) != 0 {
		t.Error("/L should not copy files to destination")
	}
}

func TestXcopyQuiet(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeFile(t, src, "quiet.txt", "q")

	p, out, _ := newProc(nil)
	Xcopy(p, cmd("xcopy", filepath.Join(src, "quiet.txt"), filepath.Join(dst, "quiet.txt"), "/q"))

	if errorLevel(p) != "0" {
		t.Errorf("expected ERRORLEVEL 0, got %s", errorLevel(p))
	}
	// /Q suppresses per-file output; only the final count line should appear.
	if strings.Contains(out.String(), "quiet.txt") {
		t.Errorf("/Q should suppress per-file output, got: %q", out.String())
	}
}

func TestXcopyUpdateOnly(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeFile(t, src, "new.txt", "new")
	// Only one file exists in source; destination is empty.
	// /U should copy nothing because the destination file doesn't exist yet.
	p, _, _ := newProc(nil)
	Xcopy(p, cmd("xcopy", src, dst, "/u", "/i"))

	entries, _ := os.ReadDir(dst)
	if len(entries) != 0 {
		t.Error("/U should skip files not already in destination")
	}
	if errorLevel(p) != "1" {
		t.Errorf("expected ERRORLEVEL 1 (nothing copied), got %s", errorLevel(p))
	}
}

func TestXcopyNewerOnly(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	srcFile := writeFile(t, src, "file.txt", "new content")
	dstFile := writeFile(t, dst, "file.txt", "old content")

	// Make destination file appear newer by setting its mtime into the future.
	future := time.Now().Add(time.Hour)
	os.Chtimes(dstFile, future, future)

	p, _, _ := newProc(nil)
	Xcopy(p, cmd("xcopy", srcFile, dstFile, "/d"))

	// Source is older than destination → /D should skip it.
	content, _ := os.ReadFile(dstFile)
	if string(content) != "old content" {
		t.Error("/D should not overwrite destination when source is not newer")
	}
}

func TestXcopyIncludeEmpty(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	// Create an empty subdirectory.
	os.Mkdir(filepath.Join(src, "emptydir"), 0755)

	p, _, _ := newProc(nil)
	Xcopy(p, cmd("xcopy", src, dst, "/e", "/i"))

	if _, err := os.Stat(filepath.Join(dst, "emptydir")); err != nil {
		t.Error("/E should create empty directories in destination")
	}
}

func TestXcopyMissingSource(t *testing.T) {
	dst := t.TempDir()
	p, _, _ := newProc(nil)
	Xcopy(p, cmd("xcopy", "/nonexistent/file.txt", filepath.Join(dst, "out.txt")))

	if errorLevel(p) == "0" {
		t.Error("expected non-zero ERRORLEVEL for missing source")
	}
}

func TestXcopyNoArgs(t *testing.T) {
	p, _, errOut := newProc(nil)
	Xcopy(p, cmd("xcopy"))

	if errorLevel(p) != "4" {
		t.Errorf("expected ERRORLEVEL 4 with no args, got %s", errorLevel(p))
	}
	if !strings.Contains(errOut.String(), "syntax") {
		t.Errorf("expected syntax error message, got: %q", errOut.String())
	}
}
