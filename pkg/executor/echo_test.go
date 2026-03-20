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
	p, out, _ := newEchoTestProc(nil)
	p.Echo = true

	cmdEcho(p, testEchoCmd("echo", nil, nil))

	output := strings.TrimSpace(out.String())
	if output != "ECHO is on" {
		t.Errorf("expected 'ECHO is on', got %q", output)
	}
}

func TestEchoNoArgsWhenOff(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)
	p.Echo = false

	cmdEcho(p, testEchoCmd("echo", nil, nil))

	output := strings.TrimSpace(out.String())
	if output != "ECHO is off" {
		t.Errorf("expected 'ECHO is off', got %q", output)
	}
}

func TestEchoOn(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)
	p.Echo = false

	cmdEcho(p, testEchoCmd("echo", []string{" ", "ON"}, []string{"ON"}))

	if !p.Echo {
		t.Error("expected Echo to be true after 'echo on'")
	}
	if out.String() != "" {
		t.Errorf("expected no output for 'echo on', got %q", out.String())
	}
}

func TestEchoOff(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)
	p.Echo = true

	cmdEcho(p, testEchoCmd("echo", []string{" ", "OFF"}, []string{"OFF"}))

	if p.Echo {
		t.Error("expected Echo to be false after 'echo off'")
	}
	if out.String() != "" {
		t.Errorf("expected no output for 'echo off', got %q", out.String())
	}
}

func TestEchoOnCaseInsensitive(t *testing.T) {
	tests := []string{"on", "ON", "On", "oN"}
	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			p, _, _ := newEchoTestProc(nil)
			p.Echo = false

			cmdEcho(p, testEchoCmd("echo", []string{" ", tc}, []string{tc}))

			if !p.Echo {
				t.Errorf("expected Echo=true for 'echo %s'", tc)
			}
		})
	}
}

func TestEchoOffCaseInsensitive(t *testing.T) {
	tests := []string{"off", "OFF", "Off", "oFf"}
	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			p, _, _ := newEchoTestProc(nil)
			p.Echo = true

			cmdEcho(p, testEchoCmd("echo", []string{" ", tc}, []string{tc}))

			if p.Echo {
				t.Errorf("expected Echo=false for 'echo %s'", tc)
			}
		})
	}
}

func TestEchoMessage(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)

	cmdEcho(p, testEchoCmd("echo", []string{" ", "Hello", " ", "World"}, []string{"Hello", "World"}))

	output := strings.TrimSpace(out.String())
	if output != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", output)
	}
}

func TestEchoMessageWithSpecialChars(t *testing.T) {
	tests := []struct {
		name     string
		rawArgs  []string
		args     []string
		expected string
	}{
		{
			name:     "with equals",
			rawArgs:  []string{" ", "a=b"},
			args:     []string{"a=b"},
			expected: "a=b",
		},
		{
			name:     "with semicolon",
			rawArgs:  []string{" ", "a;b"},
			args:     []string{"a;b"},
			expected: "a;b",
		},
		{
			name:     "with comma",
			rawArgs:  []string{" ", "a,b"},
			args:     []string{"a,b"},
			expected: "a,b",
		},
		{
			name:     "with tab",
			rawArgs:  []string{"\t", "tabbed"},
			args:     []string{"tabbed"},
			expected: "tabbed",
		},
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

func TestEchoDotBlankLine(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)

	cmdEcho(p, testEchoCmd("echo.", nil, nil))

	output := out.String()
	if output != "\n" {
		t.Errorf("expected single newline, got %q", output)
	}
}

func TestEchoColonBlankLine(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)

	cmdEcho(p, testEchoCmd("echo:", nil, nil))

	output := out.String()
	if output != "\n" {
		t.Errorf("expected single newline, got %q", output)
	}
}

func TestEchoSemicolonBlankLine(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)

	cmdEcho(p, testEchoCmd("echo;", nil, nil))

	output := out.String()
	if output != "\n" {
		t.Errorf("expected single newline, got %q", output)
	}
}

func TestEchoEqualsBlankLine(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)

	cmdEcho(p, testEchoCmd("echo=", nil, nil))

	output := out.String()
	if output != "\n" {
		t.Errorf("expected single newline, got %q", output)
	}
}

func TestEchoOpenParenBlankLine(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)

	cmdEcho(p, testEchoCmd("echo(", nil, nil))

	output := out.String()
	if output != "\n" {
		t.Errorf("expected single newline, got %q", output)
	}
}

func TestEchoSlashBlankLine(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)

	cmdEcho(p, testEchoCmd("echo/", nil, nil))

	output := out.String()
	if output != "\n" {
		t.Errorf("expected single newline, got %q", output)
	}
}

func TestEchoPlusBlankLine(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)

	cmdEcho(p, testEchoCmd("echo+", nil, nil))

	output := out.String()
	if output != "\n" {
		t.Errorf("expected single newline, got %q", output)
	}
}

func TestEchoColonWithOn(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)
	p.Echo = false

	cmdEcho(p, testEchoCmd("echo:", []string{"ON"}, []string{"ON"}))

	if p.Echo {
		t.Error("echo:ON should NOT change Echo state to true")
	}
	output := strings.TrimSpace(out.String())
	if output != "ON" {
		t.Errorf("expected 'ON' to be displayed, got %q", output)
	}
}

func TestEchoColonWithOff(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)
	p.Echo = true

	cmdEcho(p, testEchoCmd("echo:", []string{"OFF"}, []string{"OFF"}))

	if !p.Echo {
		t.Error("echo:OFF should NOT change Echo state to false")
	}
	output := strings.TrimSpace(out.String())
	if output != "OFF" {
		t.Errorf("expected 'OFF' to be displayed, got %q", output)
	}
}

func TestEchoOpenParenWithOn(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)
	p.Echo = false

	cmdEcho(p, testEchoCmd("echo(", []string{"ON"}, []string{"ON"}))

	if p.Echo {
		t.Error("echo(ON should NOT change Echo state to true")
	}
	output := strings.TrimSpace(out.String())
	if output != "ON" {
		t.Errorf("expected 'ON' to be displayed, got %q", output)
	}
}

func TestEchoOpenParenWithOff(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)
	p.Echo = true

	cmdEcho(p, testEchoCmd("echo(", []string{"OFF"}, []string{"OFF"}))

	if !p.Echo {
		t.Error("echo(OFF should NOT change Echo state to false")
	}
	output := strings.TrimSpace(out.String())
	if output != "OFF" {
		t.Errorf("expected 'OFF' to be displayed, got %q", output)
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

	output := strings.TrimSpace(out.String())
	if output != "/?" {
		t.Errorf("echo:/? should display '/?' literally, got %q", output)
	}
}

func TestEchoDotWithMessage(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)

	cmdEcho(p, testEchoCmd("echo.", []string{" ", "hello"}, []string{"hello"}))

	output := strings.TrimSpace(out.String())
	if output != "hello" {
		t.Errorf("expected 'hello', got %q", output)
	}
}

func TestEchoLeadingDelimiterStripped(t *testing.T) {
	tests := []struct {
		name     string
		rawArgs  []string
		args     []string
		expected string
	}{
		{
			name:     "leading space",
			rawArgs:  []string{" ", "test"},
			args:     []string{"test"},
			expected: "test",
		},
		{
			name:     "leading tab",
			rawArgs:  []string{"\t", "test"},
			args:     []string{"test"},
			expected: "test",
		},
		{
			name:     "leading comma",
			rawArgs:  []string{",", "test"},
			args:     []string{"test"},
			expected: "test",
		},
		{
			name:     "leading semicolon",
			rawArgs:  []string{";", "test"},
			args:     []string{"test"},
			expected: "test",
		},
		{
			name:     "leading equals",
			rawArgs:  []string{"=", "test"},
			args:     []string{"test"},
			expected: "test",
		},
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

	output := strings.TrimSpace(out.String())
	if output != "test" {
		t.Errorf("expected 'test', got %q", output)
	}
}

func TestEchoPreservesSpacing(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)

	cmdEcho(p, testEchoCmd("echo", []string{" ", "a", "  ", "b"}, []string{"a", "b"}))

	output := strings.TrimSpace(out.String())
	if output != "a  b" {
		t.Errorf("expected 'a  b' (preserving spacing), got %q", output)
	}
}

func TestEchoSemicolonWithOn(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)
	p.Echo = false

	cmdEcho(p, testEchoCmd("echo;", []string{"ON"}, []string{"ON"}))

	if p.Echo {
		t.Error("echo;ON should NOT change Echo state to true")
	}
	output := strings.TrimSpace(out.String())
	if output != "ON" {
		t.Errorf("expected 'ON' to be displayed, got %q", output)
	}
}

func TestEchoEqualsWithOff(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)
	p.Echo = true

	cmdEcho(p, testEchoCmd("echo=", []string{"OFF"}, []string{"OFF"}))

	if !p.Echo {
		t.Error("echo=OFF should NOT change Echo state to false")
	}
	output := strings.TrimSpace(out.String())
	if output != "OFF" {
		t.Errorf("expected 'OFF' to be displayed, got %q", output)
	}
}

func TestEchoSlashWithOn(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)
	p.Echo = false

	cmdEcho(p, testEchoCmd("echo/", []string{"ON"}, []string{"ON"}))

	if p.Echo {
		t.Error("echo/ON should NOT change Echo state to true")
	}
	output := strings.TrimSpace(out.String())
	if output != "ON" {
		t.Errorf("expected 'ON' to be displayed, got %q", output)
	}
}

func TestEchoPlusWithOff(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)
	p.Echo = true

	cmdEcho(p, testEchoCmd("echo+", []string{"OFF"}, []string{"OFF"}))

	if !p.Echo {
		t.Error("echo+OFF should NOT change Echo state to false")
	}
	output := strings.TrimSpace(out.String())
	if output != "OFF" {
		t.Errorf("expected 'OFF' to be displayed, got %q", output)
	}
}

func TestEchoDotWithOn(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)
	p.Echo = false

	cmdEcho(p, testEchoCmd("echo.", []string{"ON"}, []string{"ON"}))

	if p.Echo {
		t.Error("echo.ON should NOT change Echo state to true")
	}
	output := strings.TrimSpace(out.String())
	if output != "ON" {
		t.Errorf("expected 'ON' to be displayed, got %q", output)
	}
}

func TestEchoDotWithOff(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)
	p.Echo = true

	cmdEcho(p, testEchoCmd("echo.", []string{"OFF"}, []string{"OFF"}))

	if !p.Echo {
		t.Error("echo.OFF should NOT change Echo state to false")
	}
	output := strings.TrimSpace(out.String())
	if output != "OFF" {
		t.Errorf("expected 'OFF' to be displayed, got %q", output)
	}
}

func TestEchoOpenBracketBlankLine(t *testing.T) {
	p, out, _ := newEchoTestProc(nil)

	cmdEcho(p, testEchoCmd("echo[", nil, nil))

	output := out.String()
	if output != "\n" {
		t.Errorf("expected single newline, got %q", output)
	}
}
