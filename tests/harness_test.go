package tests

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/executor"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

type result struct {
	stdout     string
	stderr     string
	errorlevel string
}

// isolate gives the test its own working directory. It uses t.Chdir, so these
// tests cannot be parallel: the working directory is process-wide state.
func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	return dir
}

func runScript(t *testing.T, script string, args ...string) result {
	t.Helper()

	var stdout, stderr bytes.Buffer
	env := processor.NewEnvironment(true)
	proc := processor.New(env, append([]string{"test.bat"}, args...), executor.New())
	proc.Stdout = &stdout
	proc.Stderr = &stderr
	proc.Echo = false

	nodes := processor.ParseExpanded(processor.Phase0ReadLine(script))
	if err := proc.Execute(nodes); err != nil {
		t.Fatalf("Execute(%q) returned error: %v", script, err)
	}

	level, _ := env.Get("ERRORLEVEL")
	return result{stdout: stdout.String(), stderr: stderr.String(), errorlevel: level}
}

func writeFile(t *testing.T, name, content string) {
	t.Helper()
	if dir := filepath.Dir(name); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(name, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func mkdir(t *testing.T, name string) {
	t.Helper()
	if err := os.MkdirAll(name, 0755); err != nil {
		t.Fatal(err)
	}
}

// assertFile compares a file's exact bytes. Redirected output is CRLF in cmd,
// so trimming here would hide the line-ending fidelity these tests exist for.
func assertFile(t *testing.T, name, want string) {
	t.Helper()
	got, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if string(got) != want {
		t.Errorf("%s content = %q, want %q", name, got, want)
	}
}

func assertNoFile(t *testing.T, name string) {
	t.Helper()
	if _, err := os.Stat(name); err == nil {
		t.Errorf("%s exists, want it absent", name)
	}
}
