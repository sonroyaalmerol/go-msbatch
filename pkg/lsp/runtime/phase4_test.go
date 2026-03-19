package runtime

import (
	"testing"
)

func TestPhase4_SetlocalEndlocal(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		varName    string
		wantValues []string
		wantDepth  int
	}{
		{
			name: "simple SETLOCAL/ENDLOCAL",
			src: `SET VAR=outer
SETLOCAL
SET VAR=inner
ENDLOCAL
ECHO %VAR%`,
			varName:    "VAR",
			wantValues: []string{"outer"},
		},
		{
			name: "nested SETLOCAL",
			src: `SET VAR=level0
SETLOCAL
SET VAR=level1
SETLOCAL
SET VAR=level2
ENDLOCAL
ECHO %VAR%
ENDLOCAL
ECHO %VAR%`,
			varName:    "VAR",
			wantValues: []string{"level0"},
		},
		{
			name: "variable defined inside SETLOCAL not visible after ENDLOCAL",
			src: `SETLOCAL
SET INNER=visible
ENDLOCAL
ECHO %INNER%`,
			varName:    "INNER",
			wantValues: []string{""},
		},
		{
			name: "multiple ENDLOCAL values",
			src: `SET A=start
SETLOCAL
SET A=middle
ENDLOCAL
SET A=end`,
			varName:    "A",
			wantValues: []string{"start", "end"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes := parseBatch(t, tt.src)
			runtime := NewMiniRuntime(nodes)
			result := runtime.Execute()

			finalState := result.GetFinalState()
			if finalState == nil {
				t.Fatal("GetFinalState() returned nil")
			}

			v := finalState.GetVariable(tt.varName)
			if tt.wantValues[0] == "" {
				if v != nil && len(v.Values) > 0 && v.Values[0].Value != "" {
					t.Errorf("expected variable %q to be empty/undefined after ENDLOCAL", tt.varName)
				}
				return
			}

			if v == nil {
				t.Fatalf("GetVariable(%q) returned nil", tt.varName)
			}

			if len(v.Values) == 0 {
				t.Fatalf("variable %q has no values", tt.varName)
			}

			gotValues := make(map[string]bool)
			for _, pv := range v.Values {
				gotValues[pv.Value] = true
			}

			for _, want := range tt.wantValues {
				if !gotValues[want] {
					t.Errorf("missing expected value %q, got %v", want, v.Values)
				}
			}
		})
	}
}

func TestPhase4_DelayedExpansion(t *testing.T) {
	tests := []struct {
		name         string
		src          string
		varName      string
		wantValues   []string
		delayedAt    int
		delayedState bool
	}{
		{
			name: "delayed expansion enabled",
			src: `SETLOCAL ENABLEDELAYEDEXPANSION
SET VAR=hello
SET VAR=!VAR! world
ECHO !VAR!`,
			varName:    "VAR",
			wantValues: []string{"hello world"},
		},
		{
			name: "delayed expansion disabled - !VAR! not expanded",
			src: `SET VAR=hello
SET RESULT=!VAR!`,
			varName:    "RESULT",
			wantValues: []string{"!VAR!"},
		},
		{
			name: "toggle delayed expansion",
			src: `SETLOCAL ENABLEDELAYEDEXPANSION
SET VAR=expanded
SETLOCAL DISABLEDELAYEDEXPANSION
SET VAR2=!NOTEXPANDED!
ENDLOCAL
ECHO !VAR!`,
			varName:    "VAR2",
			wantValues: []string{"!NOTEXPANDED!"},
		},
		{
			name: "%VAR% vs !VAR! in loop",
			src: `SETLOCAL ENABLEDELAYEDEXPANSION
SET COUNT=0
FOR %%i IN (1 2 3) DO (
    SET /A COUNT+=1
    ECHO !COUNT!
)`,
			varName:    "COUNT",
			wantValues: []string{"3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes := parseBatch(t, tt.src)
			runtime := NewMiniRuntime(nodes)
			result := runtime.Execute()

			v := result.GetVariable(tt.varName)
			if v == nil {
				t.Fatalf("GetVariable(%q) returned nil", tt.varName)
			}

			if len(v.Values) == 0 {
				t.Fatalf("variable %q has no values", tt.varName)
			}

			gotValues := make(map[string]bool)
			for _, pv := range v.Values {
				gotValues[pv.Value] = true
			}

			for _, want := range tt.wantValues {
				if !gotValues[want] {
					t.Errorf("missing expected value %q, got %v", want, v.Values)
				}
			}
		})
	}
}

func TestPhase4_DelayedExpansionTracking(t *testing.T) {
	src := `SETLOCAL ENABLEDELAYEDEXPANSION
SET VAR=hello
SETLOCAL DISABLEDELAYEDEXPANSION
SET VAR2=value
ENDLOCAL`

	nodes := parseBatch(t, src)
	runtime := NewMiniRuntime(nodes)
	result := runtime.Execute()

	stateAtLine1 := result.GetStateAtLine(1)
	if stateAtLine1 == nil {
		t.Fatal("GetStateAtLine(1) returned nil")
	}
	if !stateAtLine1.DelayedExpansion() {
		t.Error("expected delayed expansion to be enabled at line 1")
	}

	stateAtLine3 := result.GetStateAtLine(3)
	if stateAtLine3 == nil {
		t.Fatal("GetStateAtLine(3) returned nil")
	}
	if stateAtLine3.DelayedExpansion() {
		t.Error("expected delayed expansion to be disabled at line 3")
	}
}

func TestPhase4_ScopeDepth(t *testing.T) {
	src := `SET A=0
SETLOCAL
SET A=1
SETLOCAL
SET A=2
SETLOCAL
SET A=3`

	nodes := parseBatch(t, src)
	runtime := NewMiniRuntime(nodes)
	result := runtime.Execute()

	tests := []struct {
		line      int
		wantDepth int
	}{
		{line: 0, wantDepth: 0},
		{line: 1, wantDepth: 1},
		{line: 3, wantDepth: 2},
		{line: 5, wantDepth: 3},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			state := result.GetStateAtLine(tt.line)
			if state == nil {
				t.Fatalf("GetStateAtLine(%d) returned nil", tt.line)
			}
			if state.ScopeDepth() != tt.wantDepth {
				t.Errorf("scope depth at line %d = %d, want %d", tt.line, state.ScopeDepth(), tt.wantDepth)
			}
		})
	}
}

func TestPhase4_EndlocalValuePropagation(t *testing.T) {
	src := `SET A=outer
SETLOCAL
SET A=inner
SET B=only_inside
ENDLOCAL & SET C=%A%_%B%`

	nodes := parseBatch(t, src)
	runtime := NewMiniRuntime(nodes)
	result := runtime.Execute()

	v := result.GetVariable("C")
	if v == nil {
		t.Fatal("variable C not found")
	}

	if len(v.Values) == 0 {
		t.Fatal("variable C has no values")
	}

	expected := "outer_%B%"
	if v.Values[0].Value != expected {
		t.Errorf("C = %q, want %q", v.Values[0].Value, expected)
	}
}
