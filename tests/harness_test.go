package tests

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/executor"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

type result struct {
	stdout     string
	stderr     string
	errorlevel string
	exitCode   int
}

func TestProcessExitCode(t *testing.T) {
	cases := []struct {
		name           string
		script         string
		wantErrorLevel string
		wantExitCode   int
	}{
		{name: "successful_command_after_sticky_error", script: "@echo off\nset /a n=1/0\necho after\n", wantErrorLevel: "1073750993", wantExitCode: 0},
		{name: "final_command_failure", script: "@echo off\ncopy missing.txt out.txt\n", wantErrorLevel: "1", wantExitCode: 1},
		{name: "explicit_exit", script: "@echo off\nexit /b 7\n", wantErrorLevel: "7", wantExitCode: 7},
		{name: "parser_abort", script: "@echo off\n& echo unreachable\n", wantErrorLevel: "255", wantExitCode: 255},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runScript(t, tc.script)
			if got.errorlevel != tc.wantErrorLevel {
				t.Errorf("ERRORLEVEL = %s, want %s", got.errorlevel, tc.wantErrorLevel)
			}
			if got.exitCode != tc.wantExitCode {
				t.Errorf("exit code = %d, want %d", got.exitCode, tc.wantExitCode)
			}
		})
	}
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

	callerDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(callerDir) }()

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
	return result{stdout: stdout.String(), stderr: stderr.String(), errorlevel: level, exitCode: proc.ExitCode}
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

func assertFileExists(t *testing.T, name string) {
	t.Helper()
	if _, err := os.Stat(name); err != nil {
		t.Errorf("%s: %v", name, err)
	}
}

func assertFileContains(t *testing.T, name, want string) {
	t.Helper()
	got, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if !strings.Contains(string(got), want) {
		t.Errorf("%s content %q does not contain %q", name, got, want)
	}
}

func assertFileMinLen(t *testing.T, name string, minLen int) {
	t.Helper()
	got, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if len(strings.TrimSpace(string(got))) < minLen {
		t.Errorf("%s content %q shorter than %d bytes", name, got, minLen)
	}
}

func assertDirEntries(t *testing.T, dir string, want []string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("%s entries = %v, want %v", dir, got, want)
	}
}
