package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTreeBasic(t *testing.T) {
	dir := t.TempDir()
	// Layout:
	//   dir/
	//     file.txt
	//     sub/
	//       nested.txt
	writeFile(t, dir, "file.txt", "")
	writeFile(t, dir, filepath.Join("sub", "nested.txt"), "")

	p, out, _ := newProc(nil)
	Tree(p, cmd("tree", dir))

	if errorLevel(p) != "0" {
		t.Error("expected ERRORLEVEL 0")
	}
	got := out.String()
	if !strings.Contains(got, "file.txt") {
		t.Errorf("expected 'file.txt' in tree output, got:\n%s", got)
	}
	if !strings.Contains(got, "sub") {
		t.Errorf("expected 'sub' directory in tree output, got:\n%s", got)
	}
	if !strings.Contains(got, "nested.txt") {
		t.Errorf("expected 'nested.txt' in tree output, got:\n%s", got)
	}
}

func TestTreeConnectors(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "")
	writeFile(t, dir, "b.txt", "")

	p, out, _ := newProc(nil)
	Tree(p, cmd("tree", dir))

	got := out.String()
	// Last entry uses └───, others use ├───
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
	// Root path is printed; no child entries
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 {
		t.Errorf("empty dir should produce only the root line, got:\n%s", out.String())
	}
}

func TestTreeDefaultToCwd(t *testing.T) {
	// Without an argument, Tree uses "." (current directory).
	orig, _ := os.Getwd()
	dir := t.TempDir()
	os.Chdir(dir)
	defer os.Chdir(orig)

	writeFile(t, dir, "hello.txt", "")

	p, out, _ := newProc(nil)
	Tree(p, cmd("tree")) // no path argument

	if !strings.Contains(out.String(), "hello.txt") {
		t.Errorf("expected 'hello.txt' in default-cwd tree output, got:\n%s", out.String())
	}
}
