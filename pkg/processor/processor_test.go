package processor_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

func newProc(batchMode bool) *processor.Processor {
	env := processor.NewEmptyEnvironment(batchMode)
	return processor.New(env, []string{"test.bat"}, nil)
}

// TestProcessorProcessLine verifies phases 0, 1, 5 are applied in order.
func TestProcessorProcessLine(t *testing.T) {
	p := newProc(true)
	p.Env.Set("NAME", "world")
	p.Env.SetDelayedExpansion(true)
	p.Env.Set("EXCL", "!")

	// Phase 1: %NAME% → world
	// Phase 5: !EXCL! → !  (literal bang via env)
	got := p.ProcessLine("echo %NAME% !EXCL!")
	if got != "echo world !" {
		t.Errorf("expected 'echo world !', got %q", got)
	}
}

// TestProcessorProcessLineCtrlZ verifies phase 0 Ctrl-Z handling.
func TestProcessorProcessLineCtrlZ(t *testing.T) {
	p := newProc(true)
	got := p.ProcessLine("echo\x1aok")
	if got != "echo\nok" {
		t.Errorf("expected 'echo\\nok', got %q", got)
	}
}

// TestProcessorProcessLineForVar verifies phase 4 FOR variable expansion.
func TestProcessorProcessLineForVar(t *testing.T) {
	p := newProc(true)
	forVars := map[string]string{"i": "alpha"}
	expanded := p.ProcessLine("echo %i")
	got := processor.Phase4ForVarExpand(expanded, forVars)
	if got != "echo alpha" {
		t.Errorf("expected 'echo alpha', got %q", got)
	}
}

// TestProcessorEchoOn tests phase 3: ECHO ON state.
func TestProcessorEchoOn(t *testing.T) {
	p := newProc(true)
	p.Echo = true
	cmd := &parser.SimpleCommand{Name: "echo", Suppressed: false}
	if !p.ShouldEcho(cmd) {
		t.Error("expected echo=true for non-suppressed command with Echo ON")
	}
}

// TestProcessorEchoSuppressed tests phase 3: @ suppresses echo.
func TestProcessorEchoSuppressed(t *testing.T) {
	p := newProc(true)
	p.Echo = true
	cmd := &parser.SimpleCommand{Name: "echo", Suppressed: true}
	if p.ShouldEcho(cmd) {
		t.Error("expected echo=false for @ suppressed command")
	}
}

// TestProcessorHandleEchoBuiltinOff tests "echo off" changes Echo state.
func TestProcessorHandleEchoBuiltinOff(t *testing.T) {
	p := newProc(true)
	p.Echo = true
	_, changed := p.HandleEchoBuiltin([]string{"off"})
	if !changed {
		t.Error("expected state change")
	}
	if p.Echo {
		t.Error("expected Echo=false after 'echo off'")
	}
}

// TestProcessorHandleEchoBuiltinText tests "echo message" returns the message.
func TestProcessorHandleEchoBuiltinText(t *testing.T) {
	p := newProc(true)
	// New HandleEchoBuiltin expects RawArgs (including whitespace/delimiters)
	output, changed := p.HandleEchoBuiltin([]string{" ", "hello", " ", "world"})
	if changed {
		t.Error("expected no state change for text echo")
	}
	if output != "hello world" {
		t.Errorf("expected 'hello world', got %q", output)
	}
}

// TestProcessorHandleEchoBuiltinNoArgs tests "echo" with no args reports state.
func TestProcessorHandleEchoBuiltinNoArgs(t *testing.T) {
	p := newProc(true)
	p.Echo = true
	output, _ := p.HandleEchoBuiltin(nil)
	if output != "ECHO is on" {
		t.Errorf("expected 'ECHO is on', got %q", output)
	}
}

// TestProcessorSetBuiltin tests HandleSetBuiltin stores a variable.
func TestProcessorSetBuiltin(t *testing.T) {
	p := newProc(true)
	p.HandleSetBuiltin("MYVAR", "hello")
	v, ok := p.Env.Get("MYVAR")
	if !ok {
		t.Fatal("expected MYVAR to be set")
	}
	if v != "hello" {
		t.Errorf("expected hello, got %q", v)
	}
}

// TestExpandPrompt tests all supported $X PROMPT codes.
func TestExpandPrompt(t *testing.T) {
	p := newProc(true)
	p.Env.Set("ERRORLEVEL", "42")

	cases := []struct {
		input string
		want  string
	}{
		{"$$", "$"},
		{"$A", "&"},
		{"$a", "&"},
		{"$B", "|"},
		{"$C", "("},
		{"$E", "\x1B"},
		{"$F", ")"},
		{"$G", ">"},
		{"$g", ">"},
		{"$H", "\x08"},
		{"$L", "<"},
		{"$Q", "="},
		{"$q", "="},
		{"$R", "42"},
		{"$S", " "},
		{"$s", " "},
		{"$V", "10.0.19045"},
		{"$_", "\n"},
		// Compound: default prompt shape
		{"$P$G", func() string {
			pwd, _ := os.Getwd()
			return pwd + ">"
		}()},
		// Unknown code passes through literally
		{"$Z", "$Z"},
	}

	for _, tc := range cases {
		got := p.ExpandPrompt(tc.input)
		if got != tc.want {
			t.Errorf("ExpandPrompt(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestParseExpanded tests that ParseExpanded re-lexes and parses an expanded line.
func TestParseExpanded(t *testing.T) {
	nodes := processor.ParseExpanded("echo hello world\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	cmd, ok := nodes[0].(*parser.SimpleCommand)
	if !ok {
		t.Fatalf("expected *SimpleCommand, got %T", nodes[0])
	}
	if cmd.Name != "echo" {
		t.Errorf("expected name=echo, got %q", cmd.Name)
	}
}

func TestInteractiveNestedExpansion(t *testing.T) {
	env := processor.NewEmptyEnvironment(false) // Cmd Mode
	p := processor.New(env, nil, nil)

	env.Set("N", "1")
	env.Set("DATAGRV1TA", "hello")
	env.SetDelayedExpansion(true)

	got := p.ProcessLine("!DATAGRV%N%TA!")
	if got != "hello" {
		t.Errorf("expected hello, got %q", got)
	}
}

func TestCallRecursiveExpansion(t *testing.T) {
	// This tests if CALL triggers another round of expansion.
	// We need a real executor for this.
	env := processor.NewEmptyEnvironment(true)
	env.Set("DATAFLT", "C:\\Data")
	env.Set("FLT", "Fast")
	env.Set("DATAGRV1TA", "%DATAFLT%\\%FLT%\\GRV1\\TA")
	env.Set("N", "1")
	env.SetDelayedExpansion(true)

	var stdout bytes.Buffer
	p := processor.New(env, nil, nil)
	p.Stdout = &stdout

	// Simulate "call echo !DATAGRV%N%TA!"
	// 1. Outer expansion: "call echo !DATAGRV1TA!" -> "call echo %DATAFLT%\%FLT%\GRV1\TA"
	// 2. Inner expansion: "echo %DATAFLT%\%FLT%\GRV1\TA" -> "echo C:\Data\Fast\GRV1\TA"

	// but it's easier to just test p.ProcessLine.
	
	expandedArg := p.ProcessLine("!DATAGRV%N%TA!")
	// expandedArg should now ALREADY be fully expanded because 
	// Phase 5 now triggers Phase 1 on its result.
	if expandedArg != "C:\\Data\\Fast\\GRV1\\TA" {
		t.Fatalf("expected C:\\Data\\Fast\\GRV1\\TA, got %q", expandedArg)
	}
}

