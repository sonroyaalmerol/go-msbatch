package processor_test

import (
	"os"
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

func newProc(batchMode bool) *processor.Processor {
	env := processor.NewEmptyEnvironment(batchMode)
	return processor.New(env, []string{"test.bat"})
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
	got := p.ProcessLineForVar("echo %i", forVars)
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
	output, changed := p.HandleEchoBuiltin([]string{"hello", "world"})
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

// TestProcessorExpandNode tests ExpandNode applies phase 1 to name and args.
func TestProcessorExpandNode(t *testing.T) {
	p := newProc(true)
	p.Env.Set("CMD", "echo")
	p.Env.Set("ARG", "world")
	cmd := &parser.SimpleCommand{
		Name: "%CMD%",
		Args: []string{"%ARG%"},
	}
	expanded := p.ExpandNode(cmd)
	if expanded.Name != "echo" {
		t.Errorf("expected expanded name=echo, got %q", expanded.Name)
	}
	if len(expanded.Args) == 0 || expanded.Args[0] != "world" {
		t.Errorf("expected expanded arg=world, got %v", expanded.Args)
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
