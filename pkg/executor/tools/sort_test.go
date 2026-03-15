package tools

import (
	"strings"
	"testing"
)

func TestSortAscending(t *testing.T) {
	p, out, _ := newProc(strings.NewReader("banana\napple\ncherry\n"))
	Sort(p, cmd("sort"))
	if errorLevel(p) != "0" {
		t.Error("expected ERRORLEVEL 0")
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	want := []string{"apple", "banana", "cherry"}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line %d: got %q, want %q", i, lines[i], w)
		}
	}
}

func TestSortDescending(t *testing.T) {
	p, out, _ := newProc(strings.NewReader("banana\napple\ncherry\n"))
	Sort(p, cmd("sort", "/r"))
	if errorLevel(p) != "0" {
		t.Error("expected ERRORLEVEL 0")
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	want := []string{"cherry", "banana", "apple"}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line %d: got %q, want %q", i, lines[i], w)
		}
	}
}

func TestSortFile(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "items.txt", "zebra\nant\nmouse\n")
	p, out, _ := newProc(nil)
	Sort(p, cmd("sort", path))
	if errorLevel(p) != "0" {
		t.Error("expected ERRORLEVEL 0")
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	want := []string{"ant", "mouse", "zebra"}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line %d: got %q, want %q", i, lines[i], w)
		}
	}
}

func TestSortMissingFile(t *testing.T) {
	p, _, errOut := newProc(nil)
	Sort(p, cmd("sort", "/nonexistent/file.txt"))
	if errorLevel(p) != "1" {
		t.Error("expected ERRORLEVEL 1 for missing file")
	}
	if !strings.Contains(errOut.String(), "cannot find") {
		t.Errorf("expected error message, got: %q", errOut.String())
	}
}

func TestSortEmpty(t *testing.T) {
	p, out, _ := newProc(strings.NewReader(""))
	Sort(p, cmd("sort"))
	if errorLevel(p) != "0" {
		t.Error("expected ERRORLEVEL 0 on empty input")
	}
	if out.String() != "" {
		t.Errorf("expected empty output, got: %q", out.String())
	}
}
