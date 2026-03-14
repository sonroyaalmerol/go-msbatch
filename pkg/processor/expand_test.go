package processor_test

import (
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

// ---- Phase 0 ---------------------------------------------------------------

// TestPhase0CtrlZReplacedWithNewline tests guideline phase 0:
// 0x1A (Ctrl-Z) is treated as <LF>.
func TestPhase0CtrlZReplacedWithNewline(t *testing.T) {
	input := "echo hello\x1aecho world"
	got := processor.Phase0ReadLine(input)
	expected := "echo hello\necho world"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestPhase0NoCtrlZ(t *testing.T) {
	input := "echo hello"
	got := processor.Phase0ReadLine(input)
	if got != input {
		t.Errorf("expected unchanged, got %q", got)
	}
}

// ---- Phase 1 (percent expansion, batch mode) --------------------------------

// TestPhase1DoublePercent tests guideline: %% → % in batch mode.
func TestPhase1DoublePercent(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	got := processor.Phase1PercentExpand("100%%", env, nil)
	if got != "100%" {
		t.Errorf("expected 100%%, got %q", got)
	}
}

// TestPhase1PositionalArg tests guideline: %1–%9 → positional arg.
func TestPhase1PositionalArg(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	args := []string{"script.bat", "hello", "world"}
	got := processor.Phase1PercentExpand("echo %1 %2", env, args)
	if got != "echo hello world" {
		t.Errorf("expected 'echo hello world', got %q", got)
	}
}

// TestPhase1PositionalArgZero tests %0 → script name.
func TestPhase1PositionalArgZero(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	args := []string{"myscript.bat"}
	got := processor.Phase1PercentExpand("%0", env, args)
	if got != "myscript.bat" {
		t.Errorf("expected myscript.bat, got %q", got)
	}
}

// TestPhase1PositionalArgStar tests %* → all args joined.
func TestPhase1PositionalArgStar(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	args := []string{"s.bat", "a", "b", "c"}
	got := processor.Phase1PercentExpand("%*", env, args)
	if got != "s.bat a b c" {
		t.Errorf("expected 's.bat a b c', got %q", got)
	}
}

// TestPhase1VarExpand tests %VAR% expansion.
func TestPhase1VarExpand(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	env.Set("GREETING", "hello")
	got := processor.Phase1PercentExpand("echo %GREETING% world", env, nil)
	if got != "echo hello world" {
		t.Errorf("expected 'echo hello world', got %q", got)
	}
}

// TestPhase1MissingVarBatchEmpty tests guideline: missing %VAR% → "" in batch.
func TestPhase1MissingVarBatchEmpty(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	got := processor.Phase1PercentExpand("echo %MISSING%", env, nil)
	if got != "echo " {
		t.Errorf("expected 'echo ', got %q", got)
	}
}

// TestPhase1MissingVarCmdLineUnchanged tests guideline:
// undefined %VAR% is left unchanged in command-line mode.
func TestPhase1MissingVarCmdLineUnchanged(t *testing.T) {
	env := processor.NewEmptyEnvironment(false) // command-line mode
	got := processor.Phase1PercentExpand("echo %MISSING%", env, nil)
	if got != "echo %MISSING%" {
		t.Errorf("expected 'echo %%MISSING%%', got %q", got)
	}
}

// TestPhase1NoCmdLinePositional tests guideline:
// %1 is left unchanged in command-line mode.
func TestPhase1NoCmdLinePositional(t *testing.T) {
	env := processor.NewEmptyEnvironment(false)
	got := processor.Phase1PercentExpand("echo %1", env, []string{"script"})
	if got != "echo %1" {
		t.Errorf("expected 'echo %%1', got %q", got)
	}
}

// TestPhase1VarCaseFolded tests that variable names are case-insensitive.
func TestPhase1VarCaseFolded(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	env.Set("myvar", "value")
	got := processor.Phase1PercentExpand("%MYVAR%", env, nil)
	if got != "value" {
		t.Errorf("expected value, got %q", got)
	}
}

// ---- Phase 4 (FOR variable expansion) --------------------------------------

// TestPhase4ForVarBasic tests guideline phase 4: %%X in batch → %X after
// phase 1, which phase 4 then resolves against the loop-variable map.
func TestPhase4ForVarBasic(t *testing.T) {
	// After phase 1, %%i became %i.  Phase 4 resolves %i.
	got := processor.Phase4ForVarExpand("echo %i", map[string]string{"i": "hello"})
	if got != "echo hello" {
		t.Errorf("expected 'echo hello', got %q", got)
	}
}

// TestPhase4ForVarCaseSensitive tests guideline: FOR variable names are case-sensitive.
func TestPhase4ForVarCaseSensitive(t *testing.T) {
	got := processor.Phase4ForVarExpand("echo %I", map[string]string{"i": "hello"})
	// "I" (upper) should NOT expand because "i" (lower) is the variable.
	if got != "echo %I" {
		t.Errorf("expected 'echo %%I' (no expansion), got %q", got)
	}
}

// TestPhase4ForVarUnknownLeft tests that unknown vars are left unchanged.
func TestPhase4ForVarUnknownLeft(t *testing.T) {
	got := processor.Phase4ForVarExpand("echo %x", map[string]string{"i": "hello"})
	if got != "echo %x" {
		t.Errorf("expected 'echo %%x', got %q", got)
	}
}

// TestPhase4ForVarEmptyMap tests that empty map leaves src unchanged.
func TestPhase4ForVarEmptyMap(t *testing.T) {
	src := "echo %i"
	got := processor.Phase4ForVarExpand(src, nil)
	if got != src {
		t.Errorf("expected unchanged, got %q", got)
	}
}

// TestPhase4ForVarModifierN tests ~n (name without extension) modifier.
func TestPhase4ForVarModifierN(t *testing.T) {
	got := processor.Phase4ForVarExpand("echo %~ni", map[string]string{"i": "file.txt"})
	if got != "echo file" {
		t.Errorf("expected 'echo file', got %q", got)
	}
}

// TestPhase4ForVarModifierX tests ~x (extension only) modifier.
func TestPhase4ForVarModifierX(t *testing.T) {
	got := processor.Phase4ForVarExpand("echo %~xi", map[string]string{"i": "file.txt"})
	if got != "echo .txt" {
		t.Errorf("expected 'echo .txt', got %q", got)
	}
}

// ---- Phase 5 (delayed expansion) -------------------------------------------

// TestPhase5DelayedBasic tests guideline phase 5: !VAR! expansion.
func TestPhase5DelayedBasic(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	env.Set("VAR", "world")
	env.SetDelayedExpansion(true)
	got := processor.Phase5DelayedExpand("echo !VAR!", env)
	if got != "echo world" {
		t.Errorf("expected 'echo world', got %q", got)
	}
}

// TestPhase5DelayedOff tests that !VAR! is left unchanged when disabled.
func TestPhase5DelayedOff(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	env.Set("VAR", "world")
	// delayed expansion OFF by default
	got := processor.Phase5DelayedExpand("echo !VAR!", env)
	if got != "echo !VAR!" {
		t.Errorf("expected unchanged, got %q", got)
	}
}

// TestPhase5DelayedMissingBatchEmpty tests guideline:
// undefined !VAR! → "" in batch mode.
func TestPhase5DelayedMissingBatchEmpty(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	env.SetDelayedExpansion(true)
	got := processor.Phase5DelayedExpand("echo !NOVAR!", env)
	if got != "echo " {
		t.Errorf("expected 'echo ', got %q", got)
	}
}

// TestPhase5DelayedMissingCmdLineUnchanged tests guideline:
// undefined !VAR! is left unchanged in command-line mode.
func TestPhase5DelayedMissingCmdLineUnchanged(t *testing.T) {
	env := processor.NewEmptyEnvironment(false)
	env.SetDelayedExpansion(true)
	got := processor.Phase5DelayedExpand("echo !NOVAR!", env)
	if got != "echo !NOVAR!" {
		t.Errorf("expected unchanged, got %q", got)
	}
}

// TestPhase5DelayedCaretEscapedBang tests guideline:
// ^! inside a !-containing token → literal !.
func TestPhase5DelayedCaretEscapedBang(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	env.SetDelayedExpansion(true)
	got := processor.Phase5DelayedExpand("echo ^!VAR^!", env)
	if got != "echo !VAR!" {
		t.Errorf("expected 'echo !VAR!', got %q", got)
	}
}
