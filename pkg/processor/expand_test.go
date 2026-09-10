package processor_test

import (
	"os"
	"path/filepath"
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

// TestPhase1TildeInvalidModifierLeftLiteral: %~ with non-cmd modifier letters stays literal.
func TestPhase1TildeInvalidModifierLeftLiteral(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	args := []string{"C:\\Windows\\notepad.exe"}
	for _, expr := range []string{"%~e1", "%~q1"} {
		if got := processor.Phase1PercentExpand(expr, env, args, nil); got != expr {
			t.Errorf("%s: expected literal %q, got %q", expr, expr, got)
		}
	}
}

// TestPhase4InvalidModifierLeftLiteral: FOR %~ with non-cmd modifier letters stays literal.
func TestPhase4InvalidModifierLeftLiteral(t *testing.T) {
	got := processor.Phase4ForVarExpand("echo %~qf", map[string]string{"f": "x"})
	if want := "echo %~qf"; got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

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

// TestPhase1TildePathModifiers covers the %~ location modifiers on both
// Windows- and Unix-style values. cmd.exe splits on '\' as well as '/', which
// path/filepath does not do on Unix.
func TestPhase1TildePathModifiers(t *testing.T) {
	tests := []struct {
		name string
		expr string
		arg  string
		want string
	}{
		{"name_windows", "%~n1", `C:\dir\f.txt`, "f"},
		{"ext_windows", "%~x1", `C:\dir\f.txt`, ".txt"},
		{"name_ext_windows", "%~nx1", `C:\dir\f.txt`, "f.txt"},
		{"drive_windows", "%~d1", `C:\dir\f.txt`, "C:"},
		{"dir_windows", "%~p1", `C:\dir\f.txt`, `\dir\`},
		{"drive_and_dir_windows", "%~dp1", `C:\dir\f.txt`, `C:\dir\`},
		{"name_unix", "%~n1", "/tmp/dir/f.txt", "f"},
		{"dir_unix", "%~p1", "/tmp/dir/f.txt", "/tmp/dir/"},
		{"quotes_stripped", "%~1", `"q w"`, "q w"},
		{"quoted_windows_path", "%~nx1", `"C:\dir\f.txt"`, "f.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := processor.NewEmptyEnvironment(true)
			got := processor.Phase1PercentExpand(tt.expr, env, []string{"script.bat", tt.arg}, nil)
			if got != tt.want {
				t.Errorf("Phase1PercentExpand(%q) with %%1=%q = %q, want %q", tt.expr, tt.arg, got, tt.want)
			}
		})
	}
}

// TestPhase1TildeResolvesRelativeValue pins the %~dp0 contract: cmd.exe
// resolves location modifiers against the current directory, so a script
// invoked by a relative path still reports its real directory.
func TestPhase1TildeResolvesRelativeValue(t *testing.T) {
	t.Chdir(t.TempDir())
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	env := processor.NewEmptyEnvironment(true)
	got := processor.Phase1PercentExpand("%~dp0", env, []string{"sub/s.bat"}, nil)

	want := filepath.Join(cwd, "sub") + "/"
	if got != want {
		t.Errorf("%%~dp0 = %q, want %q", got, want)
	}
}

// TestPhase4ForVarTildeStripsQuotes pins that a bare %~ on a FOR variable
// removes surrounding quotes, matching positional %~1.
func TestPhase4ForVarTildeStripsQuotes(t *testing.T) {
	tests := []struct {
		name string
		src  string
		val  string
		want string
	}{
		{"bare_tilde", "%~a", `"q w"`, "q w"},
		{"name_ext_quoted_windows", "%~nxa", `"C:\dir\f.txt"`, "f.txt"},
		{"unquoted_value_untouched", "%~a", "plain", "plain"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processor.Phase4ForVarExpand(tt.src, map[string]string{"a": tt.val})
			if got != tt.want {
				t.Errorf("Phase4ForVarExpand(%q) with %%a=%q = %q, want %q", tt.src, tt.val, got, tt.want)
			}
		})
	}
}

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
func TestPhase5DelayedSubstrAndSubst(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	env.Set("STR", "hello world")
	env.SetDelayedExpansion(true)

	tests := map[string]string{
		"!STR:~0,5!":     "hello",
		"!STR:hello=hi!": "hi world",
	}
	for input, want := range tests {
		if got := processor.Phase5DelayedExpand(input, env); got != want {
			t.Errorf("Phase5DelayedExpand(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestPhase5DelayedRecursesPercentVars(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	env.Set("DATAFLT", "C:\\Data")
	env.Set("DATAGRV1TA", "%DATAFLT%\\TA")
	env.SetDelayedExpansion(true)

	got := processor.Phase5DelayedExpand("!DATAGRV1TA!", env)
	if got != "C:\\Data\\TA" {
		t.Errorf("expected C:\\Data\\TA, got %q", got)
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
