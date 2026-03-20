package tests

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/executor"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

func TestRedirectStdout(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
echo hello world > out.txt
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	content, err := os.ReadFile("out.txt")
	if err != nil {
		t.Fatalf("out.txt was not created: %v", err)
	}

	got := strings.TrimSpace(string(content))
	want := "hello world"
	if got != want {
		t.Errorf("Expected %q, got: %q", want, got)
	}
}

func TestRedirectStdoutNoSpace(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
echo hello world>out.txt
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	content, err := os.ReadFile("out.txt")
	if err != nil {
		t.Fatalf("out.txt was not created: %v", err)
	}

	got := strings.TrimSpace(string(content))
	want := "hello world"
	if got != want {
		t.Errorf("Expected %q, got: %q", want, got)
	}
}

func TestRedirectAppend(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
echo line1> out.txt
echo line2>> out.txt
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	content, err := os.ReadFile("out.txt")
	if err != nil {
		t.Fatalf("out.txt was not created: %v", err)
	}

	got := strings.TrimRight(string(content), " \t\r\n")
	want := "line1\nline2"
	if got != want {
		t.Errorf("Expected %q, got: %q", want, got)
	}
}

func TestRedirectStdin(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	if err := os.WriteFile("input.txt", []byte("test input\n"), 0644); err != nil {
		t.Fatal(err)
	}

	batContent := `@echo off
type < input.txt
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	want := "test input"
	if got != want {
		t.Errorf("Expected %q, got: %q", want, got)
	}
}

func TestRedirectStderr(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
echo error message >&2
`

	env := processor.NewEnvironment(true)
	var stdout, stderr bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Stderr = &stderr
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	if stdout.String() != "" {
		t.Errorf("Expected empty stdout, got: %q", stdout.String())
	}
	got := strings.TrimSpace(stderr.String())
	want := "error message"
	if got != want {
		t.Errorf("Expected stderr %q, got: %q", want, got)
	}
}

func TestRedirectStderrToFile(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
echo error message> err.txt
echo normal output
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Stderr = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	content, err := os.ReadFile("err.txt")
	if err != nil {
		t.Fatalf("err.txt was not created: %v", err)
	}

	got := strings.TrimSpace(string(content))
	want := "error message"
	if got != want {
		t.Errorf("Expected err.txt %q, got: %q", want, got)
	}
}

func TestRedirectStderrToNul(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
echo error message 2>nul
echo normal output
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Stderr = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	got := strings.TrimRight(stdout.String(), " \t\r\n")
	want := "error message\nnormal output"
	if got != want {
		t.Errorf("Expected %q, got: %q", want, got)
	}
}

func TestRedirectStdoutToNul(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
echo normal output > nul
echo this shows
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Stderr = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	want := "this shows"
	if got != want {
		t.Errorf("Expected %q, got: %q", want, got)
	}
}

func TestRedirectBothToNul(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
echo output> nul 2>&1
echo error 2>nul >&2
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Stderr = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	if stdout.String() != "" {
		t.Errorf("Expected empty output, got: %q", stdout.String())
	}
}

func TestRedirectBothToFile(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
echo output > combined.txt 2>&1
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Stderr = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	content, err := os.ReadFile("combined.txt")
	if err != nil {
		t.Fatalf("combined.txt was not created: %v", err)
	}

	got := strings.TrimSpace(string(content))
	want := "output"
	if got != want {
		t.Errorf("Expected %q, got: %q", want, got)
	}
}

func TestRedirectSeparateStdoutStderr(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
echo stdout message > stdout.txt 2> stderr.txt
echo stderr message >&2
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Stderr = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	stdoutContent, err := os.ReadFile("stdout.txt")
	if err != nil {
		t.Fatalf("stdout.txt was not created: %v", err)
	}
	stderrContent, err := os.ReadFile("stderr.txt")
	if err != nil {
		t.Fatalf("stderr.txt was not created: %v", err)
	}

	stdoutGot := strings.TrimSpace(string(stdoutContent))
	stdoutWant := "stdout message"
	if stdoutGot != stdoutWant {
		t.Errorf("stdout.txt: expected %q, got: %q", stdoutWant, stdoutGot)
	}

	stderrGot := strings.TrimSpace(string(stderrContent))
	if stderrGot != "" {
		t.Errorf("stderr.txt should be empty, got: %q", stderrGot)
	}
}

func TestRedirectAnywhereOnLine(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "redirect_at_end",
			content: "echo A B>C\n",
			want:    "A B",
		},
		{
			name:    "redirect_in_middle",
			content: "echo A>C B\n",
			want:    "A B",
		},
		{
			name:    "redirect_at_start",
			content: ">C echo A B\n",
			want:    "A B",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			os.Remove("C")
			batContent := "@echo off\n" + tc.content

			env := processor.NewEnvironment(true)
			var stdout bytes.Buffer
			proc := processor.New(env, []string{"test.bat"}, executor.New())
			proc.Stdout = &stdout
			proc.Echo = false

			src := processor.Phase0ReadLine(batContent)
			nodes := processor.ParseExpanded(src)

			err := proc.Execute(nodes)
			if err != nil {
				t.Errorf("Execute failed: %v", err)
			}

			content, err := os.ReadFile("C")
			if err != nil {
				t.Fatalf("C was not created: %v", err)
			}

			got := strings.TrimSpace(string(content))
			if got != tc.want {
				t.Errorf("Expected %q, got: %q", tc.want, got)
			}
		})
	}
}

func TestRedirectTrailingNumber(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
set _demo=abc 5
echo %_demo% >> demofile.txt
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	content, err := os.ReadFile("demofile.txt")
	if err != nil {
		t.Fatalf("demofile.txt was not created: %v", err)
	}

	got := strings.TrimSpace(string(content))
	want := "abc 5"
	if got != want {
		t.Errorf("Expected %q, got: %q", want, got)
	}
}

func TestRedirectWithQuotes(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
echo hello world > "my file.txt"
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	content, err := os.ReadFile("my file.txt")
	if err != nil {
		t.Fatalf("'my file.txt' was not created: %v", err)
	}

	got := strings.TrimSpace(string(content))
	want := "hello world"
	if got != want {
		t.Errorf("Expected %q, got: %q", want, got)
	}
}

func TestRedirectOverwrites(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	if err := os.WriteFile("out.txt", []byte("old content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	batContent := `@echo off
echo new content > out.txt
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	content, err := os.ReadFile("out.txt")
	if err != nil {
		t.Fatalf("out.txt was not found: %v", err)
	}

	got := strings.TrimSpace(string(content))
	want := "new content"
	if got != want {
		t.Errorf("Expected %q, got: %q", want, got)
	}
}

func TestRedirectBlock(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
(
  echo sample text1
  echo sample text2
) > logfile.txt
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	content, err := os.ReadFile("logfile.txt")
	if err != nil {
		t.Fatalf("logfile.txt was not created: %v", err)
	}

	got := strings.TrimSpace(string(content))
	want := "sample text1\nsample text2"
	if got != want {
		t.Errorf("Expected %q, got: %q", want, got)
	}
}

func TestRedirectPipe(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
echo hello | cat
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	want := "hello"
	if got != want {
		t.Errorf("Expected %q, got: %q", want, got)
	}
}

func TestRedirectConcatOperator(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
echo first > a.txt & echo second > b.txt
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	aContent, err := os.ReadFile("a.txt")
	if err != nil {
		t.Fatalf("a.txt was not created: %v", err)
	}
	bContent, err := os.ReadFile("b.txt")
	if err != nil {
		t.Fatalf("b.txt was not created: %v", err)
	}

	aGot := strings.TrimSpace(string(aContent))
	bGot := strings.TrimSpace(string(bContent))
	if aGot != "first" || bGot != "second" {
		t.Errorf("Expected 'first' and 'second', got: %q and %q", aGot, bGot)
	}
}

func TestRedirectAndThenOperator(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
echo first > a.txt && echo second > b.txt
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	aContent, err := os.ReadFile("a.txt")
	if err != nil {
		t.Fatalf("a.txt was not created: %v", err)
	}
	bContent, err := os.ReadFile("b.txt")
	if err != nil {
		t.Fatalf("b.txt was not created: %v", err)
	}

	aGot := strings.TrimSpace(string(aContent))
	bGot := strings.TrimSpace(string(bContent))
	if aGot != "first" || bGot != "second" {
		t.Errorf("Expected 'first' and 'second', got: %q and %q", aGot, bGot)
	}
}

func TestRedirectOrElseOperator(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
echo first > a.txt || echo second > b.txt
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	aContent, err := os.ReadFile("a.txt")
	if err != nil {
		t.Fatalf("a.txt was not created: %v", err)
	}

	aGot := strings.TrimSpace(string(aContent))
	if aGot != "first" {
		t.Errorf("Expected 'first' in a.txt, got: %q", aGot)
	}

	if _, err := os.Stat("b.txt"); err == nil {
		bContent, _ := os.ReadFile("b.txt")
		t.Errorf("b.txt should not be created, but contains: %q", string(bContent))
	}
}

func TestRedirectStderrAppend(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
echo error1>> err.txt 2>&1
echo error2>> err.txt 2>&1
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Stderr = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	content, err := os.ReadFile("err.txt")
	if err != nil {
		t.Fatalf("err.txt was not created: %v", err)
	}

	got := strings.TrimRight(string(content), " \t\r\n")
	want := "error1\nerror2"
	if got != want {
		t.Errorf("Expected %q, got: %q", want, got)
	}
}

func TestRedirectInputAndOutput(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	if err := os.WriteFile("names.txt", []byte("Jones\nSmith\nWilson\n"), 0644); err != nil {
		t.Fatal(err)
	}

	batContent := `@echo off
type < names.txt > output.txt
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	content, err := os.ReadFile("output.txt")
	if err != nil {
		t.Fatalf("output.txt was not created: %v", err)
	}

	got := strings.TrimSpace(string(content))
	want := "Jones\nSmith\nWilson"
	if got != want {
		t.Errorf("Expected %q, got: %q", want, got)
	}
}

func TestRedirectCaseInsensitiveNul(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	tests := []string{"nul", "NUL", "Nul", "NuL"}

	for i, nulVariant := range tests {
		t.Run(nulVariant, func(t *testing.T) {
			batContent := "@echo off\necho test" + strconv.Itoa(i) + " > " + nulVariant + "\n"

			env := processor.NewEnvironment(true)
			var stdout bytes.Buffer
			proc := processor.New(env, []string{"test.bat"}, executor.New())
			proc.Stdout = &stdout
			proc.Echo = false

			src := processor.Phase0ReadLine(batContent)
			nodes := processor.ParseExpanded(src)

			err := proc.Execute(nodes)
			if err != nil {
				t.Errorf("Execute failed: %v", err)
			}

			if _, err := os.Stat(nulVariant); err == nil {
				t.Errorf("File %q should not have been created (should be nul device)", nulVariant)
			}
		})
	}
}

func TestRedirectWithVariables(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
set FILE=output.txt
echo hello > %FILE%
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	content, err := os.ReadFile("output.txt")
	if err != nil {
		t.Fatalf("output.txt was not created: %v", err)
	}

	got := strings.TrimSpace(string(content))
	want := "hello"
	if got != want {
		t.Errorf("Expected %q, got: %q", want, got)
	}
}

func TestRedirectFileDescriptorIn(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
echo test <&0
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	want := "test"
	if got != want {
		t.Errorf("Expected %q, got: %q", want, got)
	}
}
