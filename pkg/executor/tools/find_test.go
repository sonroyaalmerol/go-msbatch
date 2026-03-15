package tools

import (
	"strings"
	"testing"
)

func TestFindStdinMatch(t *testing.T) {
	p, out, _ := newProc(strings.NewReader("hello world\nfoo bar\n"))
	Find(p, cmd("find", "hello"))
	if errorLevel(p) != "0" {
		t.Error("expected ERRORLEVEL 0 on match")
	}
	if !strings.Contains(out.String(), "hello world") {
		t.Errorf("expected matching line in output, got: %q", out.String())
	}
}

func TestFindStdinNoMatch(t *testing.T) {
	p, out, _ := newProc(strings.NewReader("hello world\nfoo bar\n"))
	Find(p, cmd("find", "zzz"))
	if errorLevel(p) != "1" {
		t.Error("expected ERRORLEVEL 1 on no match")
	}
	if out.String() != "" {
		t.Errorf("expected no output on no match, got: %q", out.String())
	}
}

func TestFindNoArgs(t *testing.T) {
	p, _, errOut := newProc(nil)
	Find(p, cmd("find"))
	if errorLevel(p) != "2" {
		t.Error("expected ERRORLEVEL 2 with no args")
	}
	if !strings.Contains(errOut.String(), "parameter") {
		t.Errorf("expected error message, got: %q", errOut.String())
	}
}

func TestFindCaseInsensitive(t *testing.T) {
	p, out, _ := newProc(strings.NewReader("Hello World\nFOO BAR\n"))
	Find(p, cmd("find", "/i", "hello"))
	if errorLevel(p) != "0" {
		t.Error("expected ERRORLEVEL 0 on case-insensitive match")
	}
	if !strings.Contains(out.String(), "Hello World") {
		t.Errorf("expected matching line in output, got: %q", out.String())
	}
}

func TestFindCaseSensitiveMiss(t *testing.T) {
	p, _, _ := newProc(strings.NewReader("Hello World\n"))
	Find(p, cmd("find", "hello")) // lowercase, no /I
	if errorLevel(p) != "1" {
		t.Error("expected ERRORLEVEL 1: case-sensitive miss")
	}
}

func TestFindInvert(t *testing.T) {
	p, out, _ := newProc(strings.NewReader("hello\nworld\nfoo\n"))
	Find(p, cmd("find", "/v", "hello"))
	if errorLevel(p) != "0" {
		t.Error("expected ERRORLEVEL 0 on invert match")
	}
	got := out.String()
	if strings.Contains(got, "hello") {
		t.Errorf("/V should suppress matching lines; got: %q", got)
	}
	if !strings.Contains(got, "world") || !strings.Contains(got, "foo") {
		t.Errorf("expected non-matching lines in output, got: %q", got)
	}
}

func TestFindCount(t *testing.T) {
	p, out, _ := newProc(strings.NewReader("hello\nhello world\nbye\n"))
	Find(p, cmd("find", "/c", "hello"))
	if errorLevel(p) != "0" {
		t.Error("expected ERRORLEVEL 0")
	}
	// /C without a file label prints just the count
	if !strings.Contains(out.String(), "2") {
		t.Errorf("expected count 2 in output, got: %q", out.String())
	}
}

func TestFindLineNumbers(t *testing.T) {
	p, out, _ := newProc(strings.NewReader("no\nyes match\nno\n"))
	Find(p, cmd("find", "/n", "yes"))
	if errorLevel(p) != "0" {
		t.Error("expected ERRORLEVEL 0")
	}
	if !strings.Contains(out.String(), "[2]") {
		t.Errorf("expected line number [2] in output, got: %q", out.String())
	}
}

func TestFindInFile(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "data.txt", "alpha\nbeta\ngamma\n")
	p, out, _ := newProc(nil)
	Find(p, cmd("find", "beta", path))
	if errorLevel(p) != "0" {
		t.Error("expected ERRORLEVEL 0")
	}
	if !strings.Contains(out.String(), "beta") {
		t.Errorf("expected 'beta' in output, got: %q", out.String())
	}
	if strings.Contains(out.String(), "alpha") || strings.Contains(out.String(), "gamma") {
		t.Errorf("expected only matching line, got: %q", out.String())
	}
}

func TestFindCountInFile(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "data.txt", "hit\nmiss\nhit\nhit\n")
	p, out, _ := newProc(nil)
	Find(p, cmd("find", "/c", "hit", path))
	if errorLevel(p) != "0" {
		t.Error("expected ERRORLEVEL 0")
	}
	// /C with a file prints "<label>: <count>"
	if !strings.Contains(out.String(), "3") {
		t.Errorf("expected count 3, got: %q", out.String())
	}
}
