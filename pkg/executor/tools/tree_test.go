package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTreeBasic(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "")
	writeFile(t, dir, filepath.Join("sub", "nested.txt"), "")

	p, out, _ := newProc(nil)
	Tree(p, cmd("tree", dir))

	if errorLevel(p) != "0" {
		t.Error("expected ERRORLEVEL 0")
	}
	got := out.String()
	if strings.Contains(got, "file.txt") {
		t.Errorf("without /F, files should not be shown, got:\n%s", got)
	}
	if !strings.Contains(got, "sub") {
		t.Errorf("expected 'sub' directory in tree output, got:\n%s", got)
	}
	if strings.Contains(got, "nested.txt") {
		t.Errorf("without /F, files should not be shown, got:\n%s", got)
	}
}

func TestTreeWithFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "")
	writeFile(t, dir, filepath.Join("sub", "nested.txt"), "")

	p, out, _ := newProc(nil)
	Tree(p, cmd("tree", dir, "/f"))

	if errorLevel(p) != "0" {
		t.Error("expected ERRORLEVEL 0")
	}
	got := out.String()
	if !strings.Contains(got, "file.txt") {
		t.Errorf("with /F, expected 'file.txt' in tree output, got:\n%s", got)
	}
	if !strings.Contains(got, "sub") {
		t.Errorf("expected 'sub' directory in tree output, got:\n%s", got)
	}
	if !strings.Contains(got, "nested.txt") {
		t.Errorf("with /F, expected 'nested.txt' in tree output, got:\n%s", got)
	}
}

func TestTreeConnectors(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "a"), 0755)
	os.Mkdir(filepath.Join(dir, "b"), 0755)

	p, out, _ := newProc(nil)
	Tree(p, cmd("tree", dir))

	got := out.String()
	if !strings.Contains(got, "└───") {
		t.Errorf("expected last-entry connector '└───', got:\n%s", got)
	}
	if !strings.Contains(got, "├───") {
		t.Errorf("expected middle-entry connector '├───', got:\n%s", got)
	}
}

func TestTreeEmpty(t *testing.T) {
	dir := t.TempDir()
	p, out, _ := newProc(nil)
	Tree(p, cmd("tree", dir))

	if errorLevel(p) != "0" {
		t.Error("expected ERRORLEVEL 0")
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 {
		t.Errorf("empty dir should produce only the root line, got:\n%s", out.String())
	}
}

func TestTreeDefaultToCwd(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	os.Chdir(dir)
	defer os.Chdir(orig)

	os.Mkdir(filepath.Join(dir, "subdir"), 0755)

	p, out, _ := newProc(nil)
	Tree(p, cmd("tree"))

	if !strings.Contains(out.String(), "subdir") {
		t.Errorf("expected 'subdir' in default-cwd tree output, got:\n%s", out.String())
	}
}
