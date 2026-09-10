package tests

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sonroyaalmerol/go-msbatch/pkg/executor"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

func runCaller(t *testing.T, script string) (string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	env := processor.NewEnvironment(true)
	proc := processor.New(env, []string{"main.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Stderr = &stderr
	proc.Echo = false
	nodes := processor.ParseExpanded(processor.Phase0ReadLine(script))
	if err := proc.Execute(nodes); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	return stdout.String(), stderr.String()
}

func TestCallParseCacheInvalidation(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	write := func(content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "b.bat"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("@echo off\r\necho one\r\n")
	out, _ := runCaller(t, "@echo off\r\ncall b.bat\r\n")
	if !strings.Contains(out, "one") {
		t.Fatalf("first call output = %q", out)
	}

	out, _ = runCaller(t, "@echo off\r\ncall b.bat\r\n")
	if !strings.Contains(out, "one") {
		t.Fatalf("cached call output = %q", out)
	}

	write("@echo off\r\necho twenty-two\r\n")
	bumpFileMtime(t, filepath.Join(dir, "b.bat"))
	out, _ = runCaller(t, "@echo off\r\ncall b.bat\r\n")
	if !strings.Contains(out, "twenty-two") {
		t.Fatalf("post-rewrite call output = %q", out)
	}
}

func bumpFileMtime(t *testing.T, path string) {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	future := fi.ModTime().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
}
