package runtime

import (
	"testing"
)

func TestPhase6_HoverVariableInfo(t *testing.T) {
	src := `SET GREETING=Hello
SET NAME=World
SET MESSAGE=%GREETING% %NAME%
ECHO %MESSAGE%`

	nodes := parseBatch(t, src)
	runtime := NewMiniRuntime(nodes)
	result := runtime.Execute()

	tests := []struct {
		name          string
		line          int
		col           int
		wantVarName   string
		wantValues    []string
		wantHasSource bool
	}{
		{
			name:          "hover on GREETING reference",
			line:          2,
			col:           15,
			wantVarName:   "GREETING",
			wantValues:    []string{"Hello"},
			wantHasSource: true,
		},
		{
			name:          "hover on MESSAGE reference",
			line:          3,
			col:           5,
			wantVarName:   "MESSAGE",
			wantValues:    []string{"Hello World"},
			wantHasSource: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := result.GetHoverInfo(tt.line, tt.col)
			if info == nil {
				t.Fatalf("GetHoverInfo(%d, %d) returned nil", tt.line, tt.col)
			}

			if info.VariableName != tt.wantVarName {
				t.Errorf("VariableName = %q, want %q", info.VariableName, tt.wantVarName)
			}

			if len(info.PossibleValues) == 0 {
				t.Fatalf("PossibleValues is empty")
			}

			gotValues := make(map[string]bool)
			for _, v := range info.PossibleValues {
				gotValues[v.Value] = true
			}

			for _, want := range tt.wantValues {
				if !gotValues[want] {
					t.Errorf("missing expected value %q in PossibleValues", want)
				}
			}

			if tt.wantHasSource {
				foundSource := false
				for _, v := range info.PossibleValues {
					if v.SourceLine >= 0 {
						foundSource = true
						break
					}
				}
				if !foundSource {
					t.Error("expected at least one value to have a source line")
				}
			}
		})
	}
}

func TestPhase6_HoverWithModifier(t *testing.T) {
	src := `SET PATH=/usr/local/bin
ECHO %PATH:~0,10%`

	nodes := parseBatch(t, src)
	runtime := NewMiniRuntime(nodes)
	result := runtime.Execute()

	info := result.GetHoverInfo(1, 5)
	if info == nil {
		t.Fatal("GetHoverInfo returned nil")
	}

	if info.BaseValue != "/usr/local/bin" {
		t.Errorf("BaseValue = %q, want '/usr/local/bin'", info.BaseValue)
	}

	if info.ModifierApplied != "~0,10" {
		t.Errorf("ModifierApplied = %q, want '~0,10'", info.ModifierApplied)
	}

	if info.ExpandedValue != "/usr/local" {
		t.Errorf("ExpandedValue = %q, want '/usr/local'", info.ExpandedValue)
	}
}

func TestPhase6_HoverDelayedExpansion(t *testing.T) {
	src := `SETLOCAL ENABLEDELAYEDEXPANSION
SET COUNT=0
FOR %%i IN (1 2 3) DO (
    SET /A COUNT+=1
    ECHO !COUNT!
)`

	nodes := parseBatch(t, src)
	runtime := NewMiniRuntime(nodes)
	result := runtime.Execute()

	info := result.GetHoverInfo(4, 10)
	if info == nil {
		t.Fatal("GetHoverInfo returned nil")
	}

	if info.VariableName != "COUNT" {
		t.Errorf("VariableName = %q, want 'COUNT'", info.VariableName)
	}

	if !info.IsDelayedExpansion {
		t.Error("expected IsDelayedExpansion to be true")
	}

	gotValues := make(map[string]bool)
	for _, v := range info.PossibleValues {
		gotValues[v.Value] = true
	}

	expectedValues := []string{"1", "2", "3"}
	for _, want := range expectedValues {
		if !gotValues[want] {
			t.Errorf("missing expected value %q in PossibleValues", want)
		}
	}
}

func TestPhase6_HoverForVar(t *testing.T) {
	src := `FOR %%f IN (*.txt *.doc) DO (
    ECHO %%f
)`

	nodes := parseBatch(t, src)
	runtime := NewMiniRuntime(nodes)
	result := runtime.Execute()

	info := result.GetHoverInfo(1, 10)
	if info == nil {
		t.Fatal("GetHoverInfo returned nil")
	}

	if !info.IsForVar {
		t.Error("expected IsForVar to be true")
	}

	if info.ForVarName != "f" {
		t.Errorf("ForVarName = %q, want 'f'", info.ForVarName)
	}

	if info.IterationSource != "*.txt *.doc" {
		t.Errorf("IterationSource = %q, want '*.txt *.doc'", info.IterationSource)
	}
}

func TestPhase6_HoverBranchValues(t *testing.T) {
	src := `SET MODE=unknown
IF "%1"=="dev" (
    SET MODE=development
) ELSE IF "%1"=="prod" (
    SET MODE=production
) ELSE (
    SET MODE=default
)
ECHO %MODE%`

	nodes := parseBatch(t, src)
	runtime := NewMiniRuntime(nodes)
	result := runtime.Execute()

	info := result.GetHoverInfo(8, 5)
	if info == nil {
		t.Fatal("GetHoverInfo returned nil")
	}

	gotValues := make(map[string]bool)
	for _, v := range info.PossibleValues {
		gotValues[v.Value] = true
	}

	expectedValues := []string{"development", "production", "default"}
	for _, want := range expectedValues {
		if !gotValues[want] {
			t.Errorf("missing expected value %q in PossibleValues: %v", want, info.PossibleValues)
		}
	}
}

func TestPhase6_HoverFormat(t *testing.T) {
	src := `SET MSG=Hello World
ECHO %MSG:~0,5%`

	nodes := parseBatch(t, src)
	runtime := NewMiniRuntime(nodes)
	result := runtime.Execute()

	info := result.GetHoverInfo(1, 5)
	if info == nil {
		t.Fatal("GetHoverInfo returned nil")
	}

	formatted := info.Format()
	if formatted == "" {
		t.Error("Format() returned empty string")
	}

	expectedContains := []string{"MSG", "Hello World", "Hello"}
	for _, want := range expectedContains {
		if !contains(formatted, want) {
			t.Errorf("formatted output missing expected string %q", want)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
