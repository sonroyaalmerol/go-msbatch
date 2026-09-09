package executor

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

func newEchoTestProc(stdin io.Reader) (*processor.Processor, *bytes.Buffer, *bytes.Buffer) {
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

func testEchoCmd(name string, rawArgs []string, args []string) *parser.SimpleCommand {
	return &parser.SimpleCommand{Name: name, RawArgs: rawArgs, Args: args}
}

func TestEchoNoArgs(t *testing.T) {
	tests := []struct {
		echoOn bool
		want   string
	}{
		{true, "ECHO is on"},
		{false, "ECHO is off"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			p, out, _ := newEchoTestProc(nil)
			p.Echo = tc.echoOn

			cmdEcho(p, testEchoCmd("echo", nil, nil))

			if got := strings.TrimSpace(out.String()); got != tc.want {
				t.Errorf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

// cmd only treats bare on/off as state changes; every casing toggles.
func TestEchoOnOffToggles(t *testing.T) {
	tests := []struct {
		arg      string
		start    bool
		wantEcho bool
	}{
		{"on", false, true},
		{"ON", false, true},
		{"On", false, true},
		{"oN", false, true},
		{"off", true, false},
		{"OFF", true, false},
		{"Off", true, false},
		{"oFf", true, false},
	}
	for _, tc := range tests {
		t.Run(tc.arg, func(t *testing.T) {
			p, out, _ := newEchoTestProc(nil)
			p.Echo = tc.start

			cmdEcho(p, testEchoCmd("echo", []string{" ", tc.arg}, []string{tc.arg}))

			if p.Echo != tc.wantEcho {
				t.Errorf("after 'echo %s': Echo = %v, want %v", tc.arg, p.Echo, tc.wantEcho)
			}
			if out.String() != "" {
				t.Errorf("expected no output for 'echo %s', got %q", tc.arg, out.String())
			}
		})
	}
}

// echo<delim> prints an empty line for every cmd delimiter.
func TestEchoDelimiterBlankLine(t *testing.T) {
	for _, delim := range []string{".", ":", ";", "=", "(", "/", "+", "["} {
		t.Run("echo"+delim, func(t *testing.T) {
			p, out, _ := newEchoTestProc(nil)

			cmdEcho(p, testEchoCmd("echo"+delim, nil, nil))

			if got := out.String(); got != "\n" {
				t.Errorf("expected single newline, got %q", got)
			}
		})
	}
}

// echo<delim>ARG prints ARG literally and never changes the echo state:
// only the space-separated bare on/off form is a state change.
func TestEchoDelimiterArgPrintedLiterally(t *testing.T) {
	tests := []struct {
		delim  string
		arg    string
		start  bool
		wantOn bool
	}{
		{".", "ON", false, false},
		{".", "OFF", true, true},
		{":", "ON", false, false},
		{":", "OFF", true, true},
		{"(", "ON", false, false},
		{"(", "OFF", true, true},
		{";", "ON", false, false},
		{"=", "OFF", true, true},
		{"/", "ON", false, false},
		{"+", "OFF", true, true},
	}
	for _, tc := range tests {
		t.Run("echo"+tc.delim+tc.arg, func(t *testing.T) {
			p, out, _ := newEchoTestProc(nil)
			p.Echo = tc.start

			cmdEcho(p, testEchoCmd("echo"+tc.delim, []string{tc.arg}, []string{tc.arg}))

			if p.Echo != tc.wantOn {
				t.Errorf("Echo = %v, want unchanged %v", p.Echo, tc.wantOn)
			}
			if got := strings.TrimSpace(out.String()); got != tc.arg {
				t.Errorf("expected %q to be displayed, got %q", tc.arg, got)
			}
		})
	}
}

func TestEchoMessageWithSpecialChars(t *testing.T) {
	tests := []struct {
		name     string
		rawArgs  []string
		args     []string
		expected string
	}{
		{name: "with equals", rawArgs: []string{" ", "a=b"}, args: []string{"a=b"}, expected: "a=b"},
		{name: "with semicolon", rawArgs: []string{" ", "a;b"}, args: []string{"a;b"}, expected: "a;b"},
		{name: "with comma", rawArgs: []string{" ", "a,b"}, args: []string{"a,b"}, expected: "a,b"},
		{name: "with tab", rawArgs: []string{"\t", "tabbed"}, args: []string{"tabbed"}, expected: "tabbed"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, out, _ := newEchoTestProc(nil)
			cmdEcho(p, testEchoCmd("echo", tc.rawArgs, tc.args))
			output := strings.TrimSpace(out.String())
			if output != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, output)
			}
		})
	}
}

func TestEchoLeadingDelimiterStripped(t *testing.T) {
	tests := []struct {
		name     string
		rawArgs  []string
		args     []string
		expected string
	}{
		{name: "leading space", rawArgs: []string{" ", "test"}, args: []string{"test"}, expected: "test"},
		{name: "leading tab", rawArgs: []string{"\t", "test"}, args: []string{"test"}, expected: "test"},
		{name: "leading comma", rawArgs: []string{",", "test"}, args: []string{"test"}, expected: "test"},
		{name: "leading semicolon", rawArgs: []string{";", "test"}, args: []string{"test"}, expected: "test"},
		{name: "leading equals", rawArgs: []string{"=", "test"}, args: []string{"test"}, expected: "test"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, out, _ := newEchoTestProc(nil)
			cmdEcho(p, testEchoCmd("echo", tc.rawArgs, tc.args))
			output := strings.TrimSpace(out.String())
			if output != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, output)
			}
		})
	}
}

func TestEchoHelp(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)

	cmdEcho(p, testEchoCmd("echo", []string{" ", "/?"}, []string{"/?"}))

	output := out.String()
	if !strings.Contains(output, "ECHO") {
		t.Errorf("expected help output to contain 'ECHO', got %q", output)
	}
	if !strings.Contains(output, "ON") || !strings.Contains(output, "OFF") {
		t.Errorf("expected help output to contain ON/OFF, got %q", output)
	}
}

func TestEchoColonWithHelp(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)

	cmdEcho(p, testEchoCmd("echo:", []string{"/?"}, []string{"/?"}))

	if got := strings.TrimSpace(out.String()); got != "/?" {
		t.Errorf("echo:/? should display '/?' literally, got %q", got)
	}
}

func TestEchoDotWithMessage(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)

	cmdEcho(p, testEchoCmd("echo.", []string{" ", "hello"}, []string{"hello"}))

	if got := strings.TrimSpace(out.String()); got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

func TestEchoDoesNotChangeErrorlevel(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)
	p.SetErrorLevel(5)

	cmdEcho(p, testEchoCmd("echo", []string{" ", "test"}, []string{"test"}))

	errLevel, _ := p.Env.Get("ERRORLEVEL")
	if errLevel != "5" {
		t.Errorf("ECHO should not change ERRORLEVEL, expected 5, got %s", errLevel)
	}

	if out.String() != "test\n" {
		t.Errorf("expected 'test\\n', got %q", out.String())
	}
}

func TestEchoEmptyString(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)

	cmdEcho(p, testEchoCmd("echo", []string{}, []string{}))

	output := strings.TrimSpace(out.String())
	if p.Echo && output != "ECHO is on" {
		t.Errorf("expected 'ECHO is on' for empty args, got %q", output)
	}
}

func TestEchoMultipleDelimiters(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)

	cmdEcho(p, testEchoCmd("echo", []string{"  ", "test"}, []string{"test"}))

	if got := strings.TrimSpace(out.String()); got != "test" {
		t.Errorf("expected 'test', got %q", got)
	}
}

func TestEchoPreservesSpacing(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)

	cmdEcho(p, testEchoCmd("echo", []string{" ", "a", "  ", "b"}, []string{"a", "b"}))

	if got := strings.TrimSpace(out.String()); got != "a  b" {
		t.Errorf("expected 'a  b' (preserving spacing), got %q", got)
	}
}
