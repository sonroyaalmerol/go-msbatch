package tests

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/executor"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

// runEcho runs a script with echo on, console captured, PROMPT pinned to "$G" so wants are cwd-independent.
func runEcho(t *testing.T, script string) string {
	t.Helper()

	callerDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(callerDir) }()
	isolate(t)

	var buf bytes.Buffer
	env := processor.NewEnvironment(true)
	env.Set("PROMPT", "$G")
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &buf
	proc.Stderr = &buf
	proc.Console = &buf
	proc.Echo = true

	nodes := processor.ParseExpanded(processor.Phase0ReadLine(script))
	if err := proc.Execute(nodes); err != nil {
		t.Fatalf("Execute(%q) returned error: %v", script, err)
	}
	return buf.String()
}

// Byte-exact wants verified against real cmd.exe on Windows (probes p2-p7).
// Every echoed command line is preceded by a blank line; block bodies use
// cmd's decoration: first line flush, later lines one leading space, two
// trailing spaces except one on the last line.
func TestEchoTrace(t *testing.T) {
	cases := []struct {
		name   string
		script string
		want   string
	}{
		{
			name:   "simple command",
			script: "@echo on\r\nset V=hello\r\necho %V%\r\n",
			want: "\n>set V=hello \n" +
				"\n>echo hello \n" +
				"hello\n",
		},
		{
			name:   "rem expands vars",
			script: "@echo on\r\nset Q=N\r\nrem var=%Q% tail\r\n",
			want: "\n>set Q=N \n" +
				"\n>rem var=N tail \n",
		},
		{
			name:   "label comment not traced",
			script: "@echo on\r\n:: hidden\r\necho end\r\n",
			want: "\n>echo end \n" +
				"end\n",
		},
		{
			name:   "if inline",
			script: "@echo on\r\nif 1 == 1 (echo x)\r\n",
			want: "\n>if 1 == 1 (echo x ) \n" +
				"x\n",
		},
		{
			name:   "if bare command",
			script: "@echo on\r\nif 1 == 1 echo noparen\r\n",
			want: "\n>if 1 == 1 echo noparen \n" +
				"noparen\n",
		},
		{
			name:   "false if still traces",
			script: "@echo on\r\nif 1 == 2 (echo no)\r\necho done\r\n",
			want: "\n>if 1 == 2 (echo no ) \n" +
				"\n>echo done \n" +
				"done\n",
		},
		{
			name:   "if else inline",
			script: "@echo on\r\nif 1 == 2 (echo no) else (echo yes)\r\n",
			want: "\n>if 1 == 2 (echo no )  else (echo yes ) \n" +
				"yes\n",
		},
		{
			name:   "if multiline body",
			script: "@echo on\r\nset V=hello\r\nif 1 == 1 (\r\necho V=%V%\r\n set Y=2\r\n)\r\n",
			want: "\n>set V=hello \n" +
				"\n>if 1 == 1 (\n" +
				"echo V=hello  \n" +
				" set Y=2 \n" +
				") \n" +
				"V=hello\n",
		},
		{
			name:   "for multiline header literal and iteration expanded",
			script: "@echo on\r\nset V=hello\r\nfor %%a in (1) do (\r\necho A=%%a V=%V%\r\n   echo deep\r\n)\r\n",
			want: "\n>set V=hello \n" +
				"\n>for %a in (1) do (\n" +
				"echo A=%a V=hello  \n" +
				" echo deep \n" +
				") \n" +
				"\n>(\n" +
				"echo A=1 V=hello  \n" +
				" echo deep \n" +
				") \n" +
				"A=1 V=hello\n" +
				"deep\n",
		},
		{
			name:   "for bare do",
			script: "@echo on\r\nfor %%a in (1) do echo simple %%a\r\n",
			want: "\n>for %a in (1) do echo simple %a \n" +
				"\n>echo simple 1 \n" +
				"simple 1\n",
		},
		{
			name:   "bare block single command collapses",
			script: "@echo on\r\n(\r\necho m1\r\n)\r\n",
			want: "\n>(echo m1 ) \n" +
				"m1\n",
		},
		{
			name:   "bare block multiline",
			script: "@echo on\r\n(\r\necho m1\r\necho m2\r\n)\r\n",
			want: "\n>(\n" +
				"echo m1  \n" +
				" echo m2 \n" +
				") \n" +
				"m1\nm2\n",
		},
		{
			name:   "for body with label lines continues",
			script: "@echo on\r\nfor %%a in (1) do (\r\n echo one\r\n :S2\r\n echo two\r\n)\r\n",
			want: "\n>for %a in (1) do (\n" +
				"echo one  \n" +
				" echo two \n" +
				") \n" +
				"\n>(\n" +
				"echo one  \n" +
				" echo two \n" +
				") \n" +
				"one\ntwo\n",
		},
		{
			name:   "binary compound one line",
			script: "@echo on\r\necho a & echo b\r\n",
			want: "\n>echo a   & echo b \n" +
				"a \nb\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runEcho(t, tc.script)
			if got != tc.want {
				t.Errorf("trace = %q, want %q", got, tc.want)
			}
		})
	}
}

// runEchoFiles is runEcho with files written into the script directory first.
func runEchoFiles(t *testing.T, files map[string]string, script string) string {
	t.Helper()

	callerDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(callerDir) }()
	isolate(t)
	for name, content := range files {
		writeFile(t, name, content)
	}

	var buf bytes.Buffer
	env := processor.NewEnvironment(true)
	env.Set("PROMPT", "$G")
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &buf
	proc.Stderr = &buf
	proc.Console = &buf
	proc.Echo = true

	nodes := processor.ParseExpanded(processor.Phase0ReadLine(script))
	if err := proc.Execute(nodes); err != nil {
		t.Fatalf("Execute(%q) returned error: %v", script, err)
	}
	return buf.String()
}

// TestEchoTraceCallResetsDepth: a called batch inside a block echoes its own top-level lines with the prompt again.
func TestEchoTraceCallResetsDepth(t *testing.T) {
	got := runEchoFiles(t, map[string]string{"inner.bat": "echo from-inner\r\n"}, "if 1 == 1 (\r\ncall inner.bat\r\n)\r\n")
	want := "\n>if 1 == 1 (call inner.bat ) \n" +
		"\n>echo from-inner \n" +
		"from-inner\n"
	if !strings.Contains(got, want) {
		t.Errorf("trace = %q, want it to contain %q", got, want)
	}
}
