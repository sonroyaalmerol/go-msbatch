package executor

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

func newTestProc(stdin io.Reader) (*processor.Processor, *bytes.Buffer, *bytes.Buffer) {
	env := processor.NewEnvironment(false)
	noop := processor.CommandExecutorFunc(func(*processor.Processor, *parser.SimpleCommand) error { return nil })
	proc := processor.New(env, nil, noop)
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	proc.Stdout = out
	proc.Stderr = errOut
	proc.Stdin = stdin
	return proc, out, errOut
}

func testCmd(name string, args ...string) *parser.SimpleCommand {
	return &parser.SimpleCommand{Name: name, Args: args}
}

func testErrorLevel(p *processor.Processor) string {
	v, _ := p.Env.Get("ERRORLEVEL")
	return v
}

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCmdCopyWildcardDst(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	writeTestFile(t, srcDir, "chk.txt", "content1")
	writeTestFile(t, srcDir, "chk.bat", "content2")

	p, _, _ := newTestProc(nil)
	cmdCopy(p, testCmd("copy", filepath.Join(srcDir, "chk.*"), filepath.Join(dstDir, "chk.*")))

	if testErrorLevel(p) != "0" {
		t.Errorf("expected ERRORLEVEL 0, got %s", testErrorLevel(p))
	}

	if _, err := os.Stat(filepath.Join(dstDir, "chk.txt")); err != nil {
		t.Error("expected chk.txt in destination")
	}
	if _, err := os.Stat(filepath.Join(dstDir, "chk.bat")); err != nil {
		t.Error("expected chk.bat in destination")
	}

	if _, err := os.Stat(filepath.Join(dstDir, "chk.*")); err == nil {
		t.Error("should not create file with literal '*' in name")
	}
}

func TestCmdCopyWildcardDstSingleFile(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	writeTestFile(t, srcDir, "file.txt", "hello")

	p, _, _ := newTestProc(nil)
	cmdCopy(p, testCmd("copy", filepath.Join(srcDir, "file.*"), filepath.Join(dstDir, "file.*")))

	if testErrorLevel(p) != "0" {
		t.Errorf("expected ERRORLEVEL 0, got %s", testErrorLevel(p))
	}

	if _, err := os.Stat(filepath.Join(dstDir, "file.txt")); err != nil {
		t.Error("expected file.txt in destination")
	}

	if _, err := os.Stat(filepath.Join(dstDir, "file.*")); err == nil {
		t.Error("should not create file with literal '*' in name")
	}
}

func TestCmdCopyWildcardDstDifferentPattern(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	writeTestFile(t, srcDir, "test.txt", "hello")

	p, _, _ := newTestProc(nil)
	cmdCopy(p, testCmd("copy", filepath.Join(srcDir, "test.*"), filepath.Join(dstDir, "test.bak")))

	if testErrorLevel(p) != "0" {
		t.Errorf("expected ERRORLEVEL 0, got %s", testErrorLevel(p))
	}

	if _, err := os.Stat(filepath.Join(dstDir, "test.bak")); err != nil {
		t.Error("expected test.bak in destination")
	}
}

func TestCmdCopyWildcardQuestionMark(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	writeTestFile(t, srcDir, "file1.txt", "content")

	p, _, _ := newTestProc(nil)
	cmdCopy(p, testCmd("copy", filepath.Join(srcDir, "file?.txt"), filepath.Join(dstDir, "file?.txt")))

	if testErrorLevel(p) != "0" {
		t.Errorf("expected ERRORLEVEL 0, got %s", testErrorLevel(p))
	}

	if _, err := os.Stat(filepath.Join(dstDir, "file1.txt")); err != nil {
		t.Error("expected file1.txt in destination")
	}

	if _, err := os.Stat(filepath.Join(dstDir, "file?.txt")); err == nil {
		t.Error("should not create file with literal '?' in name")
	}
}

func TestCmdCopyToDirNoWildcard(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	writeTestFile(t, srcDir, "file.txt", "content")

	p, _, _ := newTestProc(nil)
	cmdCopy(p, testCmd("copy", filepath.Join(srcDir, "file.txt"), dstDir))

	if testErrorLevel(p) != "0" {
		t.Errorf("expected ERRORLEVEL 0, got %s", testErrorLevel(p))
	}

	if _, err := os.Stat(filepath.Join(dstDir, "file.txt")); err != nil {
		t.Error("expected file.txt in destination directory")
	}
}

func TestCmdCopyMultipleFilesToDir(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	writeTestFile(t, srcDir, "a.txt", "a")
	writeTestFile(t, srcDir, "b.txt", "b")

	p, _, _ := newTestProc(nil)
	cmdCopy(p, testCmd("copy", filepath.Join(srcDir, "*.txt"), dstDir))

	if testErrorLevel(p) != "0" {
		t.Errorf("expected ERRORLEVEL 0, got %s", testErrorLevel(p))
	}

	if _, err := os.Stat(filepath.Join(dstDir, "a.txt")); err != nil {
		t.Error("expected a.txt in destination")
	}
	if _, err := os.Stat(filepath.Join(dstDir, "b.txt")); err != nil {
		t.Error("expected b.txt in destination")
	}
}

func TestSubstituteWildcard(t *testing.T) {
	tests := []struct {
		name       string
		srcName    string
		srcPattern string
		dstPattern string
		expected   string
	}{
		{
			name:       "extension wildcard",
			srcName:    "chk.txt",
			srcPattern: "chk.*",
			dstPattern: "chk.*",
			expected:   "chk.txt",
		},
		{
			name:       "basename wildcard",
			srcName:    "file.txt",
			srcPattern: "*.txt",
			dstPattern: "*.bak",
			expected:   "file.bak",
		},
		{
			name:       "question mark",
			srcName:    "file1.txt",
			srcPattern: "file?.txt",
			dstPattern: "file?.bak",
			expected:   "file1.bak",
		},
		{
			name:       "multiple wildcards",
			srcName:    "test.txt",
			srcPattern: "*.*",
			dstPattern: "copy_*.*",
			expected:   "copy_test.txt",
		},
		{
			name:       "no wildcards in source",
			srcName:    "file.txt",
			srcPattern: "file.txt",
			dstPattern: "file.*",
			expected:   "file.*",
		},
		{
			name:       "no wildcards in dest",
			srcName:    "file.txt",
			srcPattern: "file.*",
			dstPattern: "output.txt",
			expected:   "output.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := substituteWildcard(tt.srcName, tt.srcPattern, tt.dstPattern)
			if got != tt.expected {
				t.Errorf("substituteWildcard(%q, %q, %q) = %q, want %q",
					tt.srcName, tt.srcPattern, tt.dstPattern, got, tt.expected)
			}
		})
	}
}
