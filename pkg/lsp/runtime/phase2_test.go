package runtime

import (
	"testing"
)

func TestPhase2_IfBranching(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		varName    string
		wantValues []string
		wantLines  []int
	}{
		{
			name: "IF with both branches - merge values",
			src: `SET VAR=initial
IF "%CONDITION%"=="yes" (
    SET VAR=then_value
) ELSE (
    SET VAR=else_value
)`,
			varName:    "VAR",
			wantValues: []string{"then_value", "else_value"},
			wantLines:  []int{2, 4},
		},
		{
			name: "IF without ELSE - merge with before state",
			src: `SET VAR=initial
IF "%CONDITION%"=="yes" (
    SET VAR=conditional
)`,
			varName:    "VAR",
			wantValues: []string{"initial", "conditional"},
			wantLines:  []int{0, 2},
		},
		{
			name: "nested IF - multiple branches",
			src: `SET VAR=start
IF "%A%"=="1" (
    IF "%B%"=="2" (
        SET VAR=nested_1_2
    ) ELSE (
        SET VAR=nested_1_not2
    )
) ELSE (
    SET VAR=else_branch
)`,
			varName:    "VAR",
			wantValues: []string{"nested_1_2", "nested_1_not2", "else_branch"},
		},
		{
			name: "IF DEFINED - branch tracking",
			src: `SET VAR=before
IF DEFINED MYVAR (
    SET VAR=defined
)`,
			varName:    "VAR",
			wantValues: []string{"before", "defined"},
		},
		{
			name: "IF EXIST - branch tracking",
			src: `SET VAR=before
IF EXIST "somefile.txt" (
    SET VAR=exists
)`,
			varName:    "VAR",
			wantValues: []string{"before", "exists"},
		},
		{
			name: "IF ERRORLEVEL - branch tracking",
			src: `SET VAR=before
IF ERRORLEVEL 1 (
    SET VAR=error
)`,
			varName:    "VAR",
			wantValues: []string{"before", "error"},
		},
		{
			name: "IF NOT - inverted branch",
			src: `SET VAR=before
IF NOT EXIST "file.txt" (
    SET VAR=not_exist
)`,
			varName:    "VAR",
			wantValues: []string{"before", "not_exist"},
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

			if len(v.Values) < len(tt.wantValues) {
				t.Fatalf("expected at least %d values, got %d: %v", len(tt.wantValues), len(v.Values), v.Values)
			}

			gotValues := make(map[string]bool)
			for _, pv := range v.Values {
				gotValues[pv.Value] = true
			}

			for _, want := range tt.wantValues {
				if !gotValues[want] {
					t.Errorf("missing expected value %q, got values: %v", want, v.Values)
				}
			}

			if len(tt.wantLines) > 0 {
				for i, wantLine := range tt.wantLines {
					if i >= len(v.Values) {
						break
					}
					if v.Values[i].SourceLine != wantLine {
						t.Errorf("value[%d].SourceLine = %d, want %d", i, v.Values[i].SourceLine, wantLine)
					}
				}
			}
		})
	}
}

func TestPhase2_ForLoop(t *testing.T) {
	tests := []struct {
		name           string
		src            string
		forVar         string
		wantIterType   string
		wantIterSource string
		wantBodyVar    string
		wantBodyValues []string
	}{
		{
			name:           "FOR files - basic",
			src:            "FOR %%f IN (*.txt) DO ECHO %%f",
			forVar:         "f",
			wantIterType:   "files",
			wantIterSource: "*.txt",
		},
		{
			name:           "FOR range",
			src:            "FOR /L %%i IN (1,1,5) DO ECHO %%i",
			forVar:         "i",
			wantIterType:   "range",
			wantIterSource: "(1,1,5)",
		},
		{
			name:           "FOR /F parse",
			src:            `FOR /F "tokens=1,2" %%a IN (file.txt) DO ECHO %%a`,
			forVar:         "a",
			wantIterType:   "f_parse",
			wantIterSource: `file.txt`,
		},
		{
			name:           "FOR /D directories",
			src:            "FOR /D %%d IN (*) DO ECHO %%d",
			forVar:         "d",
			wantIterType:   "dir",
			wantIterSource: "*",
		},
		{
			name:           "FOR /R recursive",
			src:            "FOR /R C:\\ %%f IN (*.txt) DO ECHO %%f",
			forVar:         "f",
			wantIterType:   "recursive",
			wantIterSource: "*.txt",
		},
		{
			name:           "FOR with SET in body",
			src:            "FOR %%i IN (a b c) DO SET RESULT=%i",
			forVar:         "i",
			wantIterType:   "files",
			wantIterSource: "a b c",
			wantBodyVar:    "RESULT",
			wantBodyValues: []string{"a", "b", "c"},
		},
		{
			name:           "FOR multiple tokens",
			src:            `FOR /F "tokens=1,2 delims=," %%a IN ("one,two") DO SET FIRST=%%a`,
			forVar:         "a",
			wantIterType:   "f_parse",
			wantIterSource: `"one,two"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes := parseBatch(t, tt.src)
			runtime := NewMiniRuntime(nodes)
			result := runtime.Execute()

			forInfo := result.GetForVar(tt.forVar)
			if forInfo == nil {
				t.Fatalf("GetForVar(%q) returned nil", tt.forVar)
			}

			if forInfo.IterType != tt.wantIterType {
				t.Errorf("IterType = %q, want %q", forInfo.IterType, tt.wantIterType)
			}

			if forInfo.IterSource != tt.wantIterSource {
				t.Errorf("IterSource = %q, want %q", forInfo.IterSource, tt.wantIterSource)
			}

			if tt.wantBodyVar != "" {
				v := result.GetVariable(tt.wantBodyVar)
				if v == nil {
					t.Fatalf("GetVariable(%q) returned nil", tt.wantBodyVar)
				}

				if len(v.Values) < len(tt.wantBodyValues) {
					t.Errorf("expected at least %d values for body variable, got %d", len(tt.wantBodyValues), len(v.Values))
				}

				gotValues := make(map[string]bool)
				for _, pv := range v.Values {
					gotValues[pv.Value] = true
				}

				for _, want := range tt.wantBodyValues {
					if !gotValues[want] {
						t.Errorf("missing expected body value %q", want)
					}
				}
			}
		})
	}
}

func TestPhase2_NestedForLoops(t *testing.T) {
	src := `FOR %%i IN (1 2) DO (
    FOR %%j IN (a b) DO (
        SET RESULT=%%i_%%j
    )
)`

	nodes := parseBatch(t, src)
	runtime := NewMiniRuntime(nodes)
	result := runtime.Execute()

	forInfoI := result.GetForVar("i")
	if forInfoI == nil {
		t.Fatal("FOR variable 'i' not found")
	}
	if forInfoI.IterType != "files" {
		t.Errorf("i.IterType = %q, want 'files'", forInfoI.IterType)
	}

	forInfoJ := result.GetForVar("j")
	if forInfoJ == nil {
		t.Fatal("FOR variable 'j' not found")
	}
	if forInfoJ.IterType != "files" {
		t.Errorf("j.IterType = %q, want 'files'", forInfoJ.IterType)
	}

	v := result.GetVariable("RESULT")
	if v == nil {
		t.Fatal("RESULT variable not found")
	}

	wantValues := []string{"1_a", "1_b", "2_a", "2_b"}
	gotValues := make(map[string]bool)
	for _, pv := range v.Values {
		gotValues[pv.Value] = true
	}

	for _, want := range wantValues {
		if !gotValues[want] {
			t.Errorf("missing expected value %q", want)
		}
	}
}

func TestPhase2_ForVarReference(t *testing.T) {
	src := `FOR %%f IN (file1.txt file2.txt) DO (
    SET CURRENT=%%f
    ECHO Processing %%f
)`

	nodes := parseBatch(t, src)
	runtime := NewMiniRuntime(nodes)
	result := runtime.Execute()

	stateAtLine1 := result.GetStateAtLine(1)
	if stateAtLine1 == nil {
		t.Fatal("GetStateAtLine(1) returned nil")
	}

	forInfo := result.GetForVar("f")
	if forInfo == nil {
		t.Fatal("FOR variable 'f' not found")
	}
	if forInfo.IterSource != "file1.txt file2.txt" {
		t.Errorf("IterSource = %q, want 'file1.txt file2.txt'", forInfo.IterSource)
	}
}

func TestPhase2_BinaryOperators(t *testing.T) {
	src := `SET A=first
SET B=second
SET C=third
IF "%A%"=="first" SET D=conditional & SET E=always`

	nodes := parseBatch(t, src)
	runtime := NewMiniRuntime(nodes)
	result := runtime.Execute()

	v := result.GetVariable("E")
	if v == nil {
		t.Fatal("variable E not found")
	}
	if len(v.Values) == 0 || v.Values[0].Value != "always" {
		t.Errorf("E = %v, want 'always'", v.Values)
	}
}
