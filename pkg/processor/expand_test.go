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
	got := processor.Phase1PercentExpand("100%%", env, nil, nil)
	if got != "100%" {
		t.Errorf("expected 100%%, got %q", got)
	}
}

// TestPhase1PositionalArg tests guideline: %1–%9 → positional arg.
func TestPhase1PositionalArg(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	args := []string{"script.bat", "hello", "world"}
	got := processor.Phase1PercentExpand("echo %1 %2", env, args, nil)
	if got != "echo hello world" {
		t.Errorf("expected 'echo hello world', got %q", got)
	}
}

// TestPhase1PositionalArgZero tests %0 → script name.
func TestPhase1PositionalArgZero(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	args := []string{"myscript.bat"}
	got := processor.Phase1PercentExpand("%0", env, args, nil)
	if got != "myscript.bat" {
		t.Errorf("expected myscript.bat, got %q", got)
	}
}

// TestPhase1PositionalArgStar tests %* → all args joined (excluding %0).
func TestPhase1PositionalArgStar(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	originalArgs := []string{"a", "b", "c"}
	got := processor.Phase1PercentExpand("%*", env, nil, originalArgs)
	if got != "a b c" {
		t.Errorf("expected 'a b c', got %q", got)
	}
}

// TestPhase1VarExpand tests %VAR% expansion.
func TestPhase1VarExpand(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	env.Set("GREETING", "hello")
	got := processor.Phase1PercentExpand("echo %GREETING% world", env, nil, nil)
	if got != "echo hello world" {
		t.Errorf("expected 'echo hello world', got %q", got)
	}
}

// TestPhase1MissingVarBatchEmpty tests guideline: missing %VAR% → "" in batch.
func TestPhase1MissingVarBatchEmpty(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	got := processor.Phase1PercentExpand("echo %MISSING%", env, nil, nil)
	if got != "echo " {
		t.Errorf("expected 'echo ', got %q", got)
	}
}

// TestPhase1MissingVarCmdLineUnchanged tests guideline:
// undefined %VAR% is left unchanged in command-line mode.
func TestPhase1MissingVarCmdLineUnchanged(t *testing.T) {
	env := processor.NewEmptyEnvironment(false) // command-line mode
	got := processor.Phase1PercentExpand("echo %MISSING%", env, nil, nil)
	if got != "echo %MISSING%" {
		t.Errorf("expected 'echo %%MISSING%%', got %q", got)
	}
}

// TestPhase1NoCmdLinePositional tests guideline:
// %1 is left unchanged in command-line mode.
func TestPhase1NoCmdLinePositional(t *testing.T) {
	env := processor.NewEmptyEnvironment(false)
	got := processor.Phase1PercentExpand("echo %1", env, []string{"script"}, nil)
	if got != "echo %1" {
		t.Errorf("expected 'echo %%1', got %q", got)
	}
}

// TestPhase1VarCaseFolded tests that variable names are case-insensitive.
func TestPhase1VarCaseFolded(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	env.Set("myvar", "value")
	got := processor.Phase1PercentExpand("%MYVAR%", env, nil, nil)
	if got != "value" {
		t.Errorf("expected value, got %q", got)
	}
}

// ---- Phase 1: %~ tilde modifiers on positional parameters ------------------

// TestPhase1TildeBasic tests %~0 strips surrounding quotes.
func TestPhase1TildeBasic(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	args := []string{`"C:\scripts\deploy.bat"`}
	got := processor.Phase1PercentExpand("%~0", env, args, nil)
	if got != `C:\scripts\deploy.bat` {
		t.Errorf("expected unquoted path, got %q", got)
	}
}

// TestPhase1TildeN tests %~n0 (filename without extension).
func TestPhase1TildeN(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	args := []string{"/tmp/tilde_test.bat"}
	got := processor.Phase1PercentExpand("%~n0", env, args, nil)
	if got != "tilde_test" {
		t.Errorf("expected 'tilde_test', got %q", got)
	}
}

// TestPhase1TildeX tests %~x0 (extension only).
func TestPhase1TildeX(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	args := []string{"/tmp/tilde_test.bat"}
	got := processor.Phase1PercentExpand("%~x0", env, args, nil)
	if got != ".bat" {
		t.Errorf("expected '.bat', got %q", got)
	}
}

// TestPhase1TildeNX tests %~nx0 (name + extension = full basename).
func TestPhase1TildeNX(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	args := []string{"/tmp/tilde_test.bat"}
	got := processor.Phase1PercentExpand("%~nx0", env, args, nil)
	if got != "tilde_test.bat" {
		t.Errorf("expected 'tilde_test.bat', got %q", got)
	}
}

// TestPhase1TildeDP tests %~dp0 (directory with trailing separator) — the most
// common real-world pattern for "directory of this script".
func TestPhase1TildeDP(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	args := []string{"/tmp/scripts/deploy.bat"}
	got := processor.Phase1PercentExpand("%~dp0", env, args, nil)
	if got != "/tmp/scripts/" {
		t.Errorf("expected '/tmp/scripts/', got %q", got)
	}
}

// TestPhase1TildeF tests %~f0 (absolute path).
func TestPhase1TildeF(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	args := []string{"/tmp/tilde_test.bat"}
	got := processor.Phase1PercentExpand("%~f0", env, args, nil)
	if got != "/tmp/tilde_test.bat" {
		t.Errorf("expected '/tmp/tilde_test.bat', got %q", got)
	}
}

// TestPhase1TildeNotBatchMode tests %~0 is left unchanged outside batch mode.
func TestPhase1TildeNotBatchMode(t *testing.T) {
	env := processor.NewEmptyEnvironment(false)
	args := []string{"script.bat"}
	got := processor.Phase1PercentExpand("%~n0", env, args, nil)
	if got != "%~n0" {
		t.Errorf("expected literal '%%~n0', got %q", got)
	}
}

// TestPhase1TildeOutOfRange tests %~1 when args[1] is absent → empty string.
func TestPhase1TildeOutOfRange(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	args := []string{"script.bat"} // only %0, no %1
	got := processor.Phase1PercentExpand("%~n1", env, args, nil)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
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

func TestReproNestedExpansion(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	env.Set("N", "1")
	env.Set("DATAGRV1TA", "hello")
	env.SetDelayedExpansion(true)

	p := processor.New(env, nil, nil)
	got := p.ProcessLine("!DATAGRV%N%TA!")
	expected := "hello"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

// TestDynamicVarTime verifies that %TIME% expands to a string matching the
// Windows CMD format: " H:MM:SS.CC" (space-padded 24-h hour, no leading zero).
func TestDynamicVarTime(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	p := processor.New(env, nil, nil)
	got := p.ExpandPhase1("%TIME%")
	// Expected pattern: optional leading space + digits : MM : SS . CC
	// e.g. " 9:05:03.07" or "14:30:00.00"
	if len(got) != 11 {
		t.Fatalf("expected length 11, got %d: %q", len(got), got)
	}
	if got[2] != ':' || got[5] != ':' || got[8] != '.' {
		t.Errorf("unexpected TIME format: %q", got)
	}
}

// TestDynamicVarTimeOverridesEnv verifies that dynamic %TIME% is not shadowed
// by a SET TIME=... assignment, matching CMD behaviour.
func TestDynamicVarTimeOverridesEnv(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	env.Set("TIME", "fixed")
	p := processor.New(env, nil, nil)
	got := p.ExpandPhase1("%TIME%")
	if got == "fixed" {
		t.Errorf("dynamic %%TIME%% should not be overridden by SET, got %q", got)
	}
}

func TestUserRealEnv(t *testing.T) {
	env := processor.NewEnvironment(true)
	env.Set("N", "1")
	env.Set("DATAGRV1TA", "hello")
	env.SetDelayedExpansion(true)

	p := processor.New(env, nil, nil)
	got := p.ProcessLine("!DATAGRV%N%TA!")
	if got != "hello" {
		t.Errorf("expected hello, got %q", got)
	}
}
