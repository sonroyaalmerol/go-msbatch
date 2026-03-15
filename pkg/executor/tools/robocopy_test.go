package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// mustExist asserts that path exists.
func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected %q to exist: %v", path, err)
	}
}

// mustNotExist asserts that path does not exist.
func mustNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("expected %q to not exist", path)
	}
}

// ── basic copy ────────────────────────────────────────────────────────────────

func TestRobocopyBasicCopy(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeFile(t, src, "hello.txt", "hello")

	p, _, _ := newProc(nil)
	Robocopy(p, cmd("robocopy", src, dst))

	mustExist(t, filepath.Join(dst, "hello.txt"))
	el, _ := p.Env.Get("ERRORLEVEL")
	if el != "1" { // bit 0 = files copied
		t.Errorf("expected ERRORLEVEL 1 (files copied), got %s", el)
	}
}

func TestRobocopyNoFilesToCopy(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeFile(t, src, "file.txt", "x")
	// Copy once so destination is up-to-date.
	p1, _, _ := newProc(nil)
	Robocopy(p1, cmd("robocopy", src, dst))

	// Second run: nothing new to copy.
	p2, _, _ := newProc(nil)
	Robocopy(p2, cmd("robocopy", src, dst))
	el, _ := p2.Env.Get("ERRORLEVEL")
	if el != "0" { // no files copied, no extras → 0
		t.Errorf("expected ERRORLEVEL 0 (nothing to do), got %s", el)
	}
}

// ── recursive ─────────────────────────────────────────────────────────────────

func TestRobocopyRecursive(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeFile(t, src, "root.txt", "r")
	writeFile(t, src, filepath.Join("sub", "child.txt"), "c")

	p, _, _ := newProc(nil)
	Robocopy(p, cmd("robocopy", src, dst, "/s"))

	mustExist(t, filepath.Join(dst, "root.txt"))
	mustExist(t, filepath.Join(dst, "sub", "child.txt"))
}

func TestRobocopyRecursiveSkipsEmptyByDefault(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	os.Mkdir(filepath.Join(src, "emptydir"), 0755)

	p, _, _ := newProc(nil)
	Robocopy(p, cmd("robocopy", src, dst, "/s"))

	mustNotExist(t, filepath.Join(dst, "emptydir"))
}

func TestRobocopyIncludesEmptyWithE(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	os.Mkdir(filepath.Join(src, "emptydir"), 0755)

	p, _, _ := newProc(nil)
	Robocopy(p, cmd("robocopy", src, dst, "/e"))

	mustExist(t, filepath.Join(dst, "emptydir"))
}

// ── list-only ─────────────────────────────────────────────────────────────────

func TestRobocopyListOnly(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeFile(t, src, "file.txt", "data")

	p, out, _ := newProc(nil)
	Robocopy(p, cmd("robocopy", src, dst, "/l"))

	// Nothing should have been written to dst.
	entries, _ := os.ReadDir(dst)
	if len(entries) != 0 {
		t.Error("/L should not write any files to destination")
	}
	if !strings.Contains(out.String(), "file.txt") {
		t.Errorf("/L should list files in output, got:\n%s", out.String())
	}
}

// ── purge & mirror ────────────────────────────────────────────────────────────

func TestRobocopyPurge(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeFile(t, src, "keep.txt", "k")
	writeFile(t, dst, "extra.txt", "e") // only in destination

	p, _, _ := newProc(nil)
	Robocopy(p, cmd("robocopy", src, dst, "/purge"))

	mustExist(t, filepath.Join(dst, "keep.txt"))
	mustNotExist(t, filepath.Join(dst, "extra.txt"))
	el, _ := p.Env.Get("ERRORLEVEL")
	// bit 0 (copied) | bit 1 (extras removed)
	if el != "3" {
		t.Errorf("expected ERRORLEVEL 3 (copied+extras), got %s", el)
	}
}

func TestRobocopyMirror(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeFile(t, src, "new.txt", "n")
	writeFile(t, src, filepath.Join("sub", "deep.txt"), "d")
	writeFile(t, dst, "obsolete.txt", "o")

	p, _, _ := newProc(nil)
	Robocopy(p, cmd("robocopy", src, dst, "/mir"))

	mustExist(t, filepath.Join(dst, "new.txt"))
	mustExist(t, filepath.Join(dst, "sub", "deep.txt"))
	mustNotExist(t, filepath.Join(dst, "obsolete.txt"))
}

// ── file filtering ────────────────────────────────────────────────────────────

func TestRobocopyExcludeFiles(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeFile(t, src, "keep.txt", "k")
	writeFile(t, src, "skip.log", "s")

	p, _, _ := newProc(nil)
	Robocopy(p, cmd("robocopy", src, dst, "/xf", "*.log"))

	mustExist(t, filepath.Join(dst, "keep.txt"))
	mustNotExist(t, filepath.Join(dst, "skip.log"))
}

func TestRobocopyExcludeDirs(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeFile(t, src, filepath.Join("included", "a.txt"), "a")
	writeFile(t, src, filepath.Join("excluded", "b.txt"), "b")

	p, _, _ := newProc(nil)
	Robocopy(p, cmd("robocopy", src, dst, "/s", "/xd", "excluded"))

	mustExist(t, filepath.Join(dst, "included", "a.txt"))
	mustNotExist(t, filepath.Join(dst, "excluded"))
}

func TestRobocopyExcludeOlder(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	srcFile := writeFile(t, src, "file.txt", "new")
	dstFile := writeFile(t, dst, "file.txt", "old")

	// Make the source file appear older than the destination.
	past := time.Now().Add(-time.Hour)
	os.Chtimes(srcFile, past, past)

	p, _, _ := newProc(nil)
	Robocopy(p, cmd("robocopy", src, dst, "/xo"))

	// /XO skips source files that are older than destination.
	content, _ := os.ReadFile(dstFile)
	if string(content) != "old" {
		t.Error("/XO should not overwrite destination with older source file")
	}
}

func TestRobocopyFilePatternFilter(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeFile(t, src, "a.txt", "a")
	writeFile(t, src, "b.csv", "b")

	p, _, _ := newProc(nil)
	// Only copy *.txt files.
	Robocopy(p, cmd("robocopy", src, dst, "*.txt"))

	mustExist(t, filepath.Join(dst, "a.txt"))
	mustNotExist(t, filepath.Join(dst, "b.csv"))
}

// ── size filters ──────────────────────────────────────────────────────────────

func TestRobocopyMaxSize(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeFile(t, src, "small.txt", "hi")         // 2 bytes
	writeFile(t, src, "big.txt", "hello world!")  // 12 bytes

	p, _, _ := newProc(nil)
	Robocopy(p, cmd("robocopy", src, dst, "/max:5"))

	mustExist(t, filepath.Join(dst, "small.txt"))
	mustNotExist(t, filepath.Join(dst, "big.txt"))
}

func TestRobocopyMinSize(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeFile(t, src, "small.txt", "hi")          // 2 bytes
	writeFile(t, src, "big.txt", "hello world!")   // 12 bytes

	p, _, _ := newProc(nil)
	Robocopy(p, cmd("robocopy", src, dst, "/min:5"))

	mustNotExist(t, filepath.Join(dst, "small.txt"))
	mustExist(t, filepath.Join(dst, "big.txt"))
}

// ── multi-threading ───────────────────────────────────────────────────────────

func TestRobocopyMTProducesSameResult(t *testing.T) {
	// Verify that /MT:4 copies the same set of files as single-threaded.
	src := t.TempDir()
	dstSingle := t.TempDir()
	dstMT := t.TempDir()

	for i := 0; i < 20; i++ {
		name := filepath.Join("dir", strings.Repeat("x", i)+"file.txt")
		writeFile(t, src, name, "content")
	}

	p1, _, _ := newProc(nil)
	Robocopy(p1, cmd("robocopy", src, dstSingle, "/s"))

	p2, _, _ := newProc(nil)
	Robocopy(p2, cmd("robocopy", src, dstMT, "/s", "/mt:4"))

	// Both destinations should have identical directory trees.
	checkDirsEqual(t, dstSingle, dstMT)
}

// checkDirsEqual asserts that every file present in a also exists in b with
// the same content, and that b contains no extra files.
func checkDirsEqual(t *testing.T, a, b string) {
	t.Helper()
	err := filepath.Walk(a, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(a, path)
		bPath := filepath.Join(b, rel)
		aContent, _ := os.ReadFile(path)
		bContent, err := os.ReadFile(bPath)
		if err != nil {
			t.Errorf("file %q missing in b: %v", rel, err)
			return nil
		}
		if string(aContent) != string(bContent) {
			t.Errorf("file %q content differs", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// ── exit code bits ────────────────────────────────────────────────────────────

func TestRobocopyExitCodeBits(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(src, dst string)
		args    []string
		wantEL  string
	}{
		{
			name:   "nothing to do",
			setup:  func(src, dst string) {},
			args:   nil,
			wantEL: "0",
		},
		{
			name: "files copied only",
			setup: func(src, dst string) {
				writeFile(t, src, "f.txt", "x")
			},
			args:   nil,
			wantEL: "1",
		},
		{
			name: "extras in dst only",
			setup: func(src, dst string) {
				writeFile(t, dst, "extra.txt", "e")
			},
			args:   []string{"/purge"},
			wantEL: "2",
		},
		{
			name: "copied and extras",
			setup: func(src, dst string) {
				writeFile(t, src, "new.txt", "n")
				writeFile(t, dst, "extra.txt", "e")
			},
			args:   []string{"/purge"},
			wantEL: "3",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := t.TempDir()
			dst := t.TempDir()
			tc.setup(src, dst)
			p, _, _ := newProc(nil)
			args := append([]string{src, dst}, tc.args...)
			Robocopy(p, cmd("robocopy", args...))
			el, _ := p.Env.Get("ERRORLEVEL")
			if el != tc.wantEL {
				t.Errorf("ERRORLEVEL: got %s, want %s", el, tc.wantEL)
			}
		})
	}
}

// ── error cases ───────────────────────────────────────────────────────────────

func TestRobocopyMissingSource(t *testing.T) {
	dst := t.TempDir()
	p, _, _ := newProc(nil)
	Robocopy(p, cmd("robocopy", "/nonexistent/src", dst))
	el, _ := p.Env.Get("ERRORLEVEL")
	if el != "16" {
		t.Errorf("expected ERRORLEVEL 16 for missing source, got %s", el)
	}
}

func TestRobocopySrcEqualsDst(t *testing.T) {
	dir := t.TempDir()
	p, _, _ := newProc(nil)
	Robocopy(p, cmd("robocopy", dir, dir))
	el, _ := p.Env.Get("ERRORLEVEL")
	if el != "16" {
		t.Errorf("expected ERRORLEVEL 16 when src==dst, got %s", el)
	}
}

func TestRobocopyTooFewArgs(t *testing.T) {
	p, _, _ := newProc(nil)
	Robocopy(p, cmd("robocopy", "/only/one/arg"))
	el, _ := p.Env.Get("ERRORLEVEL")
	if el != "16" {
		t.Errorf("expected ERRORLEVEL 16 with too few args, got %s", el)
	}
}
