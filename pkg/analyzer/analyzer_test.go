package analyzer

import (
	"testing"
)

func TestNewAnalyzer(t *testing.T) {
	a := NewAnalyzer()
	if a == nil {
		t.Fatal("expected analyzer to be created")
	}
}

func TestAnalyzeSimple(t *testing.T) {
	a := NewAnalyzer()
	content := `@echo off
set MYVAR=hello
echo %MYVAR%
`
	result := a.Analyze("file:///test.bat", content)

	if result == nil {
		t.Fatal("expected result to be non-nil")
	}
	if result.Symbols == nil {
		t.Fatal("expected symbols to be non-nil")
	}
}

func TestAnalyzeVariables(t *testing.T) {
	a := NewAnalyzer()
	content := `@echo off
set MYVAR=hello
set /a COUNT=1+1
set /p INPUT=Enter value:
`
	result := a.Analyze("file:///test.bat", content)

	if len(result.Symbols.Vars) == 0 {
		t.Error("expected variables to be defined")
	}

	foundMyvar := false
	foundCount := false
	foundInput := false

	for name := range result.Symbols.Vars {
		if name == "MYVAR" {
			foundMyvar = true
		}
		if name == "COUNT" {
			foundCount = true
		}
		if name == "INPUT" {
			foundInput = true
		}
	}

	if !foundMyvar {
		t.Error("expected MYVAR to be defined")
	}
	if !foundCount {
		t.Error("expected COUNT to be defined")
	}
	if !foundInput {
		t.Error("expected INPUT to be defined")
	}
}

func TestAnalyzeLabels(t *testing.T) {
	a := NewAnalyzer()
	content := `@echo off
:start
echo Hello
goto start
:end
`
	result := a.Analyze("file:///test.bat", content)

	if len(result.Symbols.Labels) == 0 {
		t.Error("expected labels to be defined")
	}

	if _, ok := result.Symbols.Labels["start"]; !ok {
		t.Error("expected 'start' label to be defined")
	}
	if _, ok := result.Symbols.Labels["end"]; !ok {
		t.Error("expected 'end' label to be defined")
	}
}

func TestAnalyzeForLoop(t *testing.T) {
	a := NewAnalyzer()
	content := `@echo off
for %%i in (*.txt) do echo %%i
`
	result := a.Analyze("file:///test.bat", content)

	if len(result.Symbols.ForVars) == 0 {
		t.Error("expected FOR variables to be defined")
	}

	if _, ok := result.Symbols.ForVars["I"]; !ok {
		t.Errorf("expected 'I' FOR variable to be defined, got keys: %v", getForVarKeys(result.Symbols.ForVars))
	}
}

func getForVarKeys(m map[string]*Symbol) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestAnalyzeLabelReferences(t *testing.T) {
	a := NewAnalyzer()
	content := `@echo off
:start
echo Hello
goto start
call :end
:end
`
	result := a.Analyze("file:///test.bat", content)

	startLabel := result.Symbols.Labels["start"]
	if startLabel == nil {
		t.Fatal("expected 'start' label")
	}

	hasGotoRef := false
	for _, ref := range startLabel.References {
		if ref.Kind == RefGoto {
			hasGotoRef = true
		}
	}
	if !hasGotoRef {
		t.Error("expected GOTO reference for 'start' label")
	}
}

func TestResultDefinitionAt(t *testing.T) {
	a := NewAnalyzer()
	content := `@echo off
:mylabel
echo test
goto mylabel
`
	result := a.Analyze("file:///test.bat", content)

	loc := result.DefinitionAt(3, 5)
	if loc == nil {
		t.Error("expected to find definition for goto reference")
	}
	if loc.Line != 1 {
		t.Errorf("expected definition line 1, got %d", loc.Line)
	}
}

func TestResultReferencesAt(t *testing.T) {
	a := NewAnalyzer()
	content := `@echo off
:mylabel
echo test
goto mylabel
call :mylabel
`
	result := a.Analyze("file:///test.bat", content)

	refs := result.ReferencesAt(1, 1, true)
	if len(refs) == 0 {
		t.Error("expected to find references")
	}
}

func TestResultHoverAt(t *testing.T) {
	a := NewAnalyzer()
	content := `@echo off
set MYVAR=hello
echo %MYVAR%
`
	result := a.Analyze("file:///test.bat", content)

	hover := result.HoverAt(1, 5)
	if hover == nil {
		t.Error("expected hover info for variable definition")
	}
}

func TestResultRenameAt(t *testing.T) {
	a := NewAnalyzer()
	content := `@echo off
:oldname
echo test
goto oldname
`
	result := a.Analyze("file:///test.bat", content)

	edits := result.RenameAt(1, 1, "newname")
	if len(edits) == 0 {
		t.Error("expected rename edits")
	}
}

func TestSymbolTable(t *testing.T) {
	st := NewSymbolTable("file:///test.bat")

	if st.Global == nil {
		t.Error("expected global scope")
	}
	if st.Labels == nil {
		t.Error("expected labels map")
	}
	if st.Vars == nil {
		t.Error("expected vars map")
	}
}

func TestSymbolTableDefineVariable(t *testing.T) {
	st := NewSymbolTable("file:///test.bat")
	loc := Location{URI: "file:///test.bat", Line: 1, Col: 4}

	sym := st.DefineVariable("MYVAR", loc)

	if sym == nil {
		t.Fatal("expected symbol to be defined")
	}
	if sym.Name != "MYVAR" {
		t.Errorf("expected name MYVAR, got %s", sym.Name)
	}
	if sym.Kind != SymbolVariable {
		t.Errorf("expected kind SymbolVariable, got %v", sym.Kind)
	}
}

func TestSymbolTableDefineLabel(t *testing.T) {
	st := NewSymbolTable("file:///test.bat")
	loc := Location{URI: "file:///test.bat", Line: 1, Col: 1}

	sym := st.DefineLabel("mylabel", loc)

	if sym == nil {
		t.Fatal("expected symbol to be defined")
	}
	if sym.Name != "mylabel" {
		t.Errorf("expected name mylabel, got %s", sym.Name)
	}
	if sym.Kind != SymbolLabel {
		t.Errorf("expected kind SymbolLabel, got %v", sym.Kind)
	}
}

func TestSymbolTableResolveLabel(t *testing.T) {
	st := NewSymbolTable("file:///test.bat")
	loc := Location{URI: "file:///test.bat", Line: 1, Col: 1}
	st.DefineLabel("mylabel", loc)

	sym := st.ResolveLabel("mylabel")
	if sym == nil {
		t.Error("expected to resolve mylabel")
	}

	sym = st.ResolveLabel("MYLABEL")
	if sym == nil {
		t.Error("expected to resolve MYLABEL (case insensitive)")
	}
}

func TestSymbolRefCount(t *testing.T) {
	loc := Location{URI: "file:///test.bat", Line: 1, Col: 1}
	sym := &Symbol{
		Name:       "mylabel",
		Kind:       SymbolLabel,
		Definition: loc,
		References: []Reference{
			{Location: Location{Line: 2, Col: 5}, Kind: RefGoto},
			{Location: Location{Line: 3, Col: 5}, Kind: RefCall},
		},
	}

	count := sym.RefCount()
	if count != 2 {
		t.Errorf("expected ref count 2, got %d", count)
	}
}

func TestScopeContains(t *testing.T) {
	scope := &Scope{
		Kind:      ScopeGlobal,
		StartLine: 0,
		EndLine:   10,
	}

	if !scope.Contains(5) {
		t.Error("expected scope to contain line 5")
	}
	if scope.Contains(15) {
		t.Error("expected scope to not contain line 15")
	}
}

func TestScopeResolve(t *testing.T) {
	parent := NewScope(ScopeGlobal, nil)
	parent.Symbols["MYVAR"] = &Symbol{Name: "MYVAR", Kind: SymbolVariable}

	child := NewScope(ScopeFor, parent)

	sym := child.Resolve("MYVAR", SymbolVariable)
	if sym == nil {
		t.Error("expected to resolve MYVAR from parent scope")
	}
}

func TestSeverityString(t *testing.T) {
	tests := []struct {
		severity Severity
		expected string
	}{
		{SeverityError, "error"},
		{SeverityWarning, "warning"},
		{SeverityInfo, "info"},
		{SeverityHint, "hint"},
	}

	for _, tt := range tests {
		_ = tt
	}
}

func TestSymbolKindString(t *testing.T) {
	tests := []struct {
		kind     SymbolKind
		expected string
	}{
		{SymbolVariable, "variable"},
		{SymbolForVar, "for-variable"},
		{SymbolLabel, "label"},
		{SymbolPositionalArg, "positional"},
	}

	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.expected {
			t.Errorf("SymbolKind(%d).String() = %q, want %q", tt.kind, got, tt.expected)
		}
	}
}

func TestDiagnostics(t *testing.T) {
	a := NewAnalyzer()
	content := `@echo off
:unused_label
echo test
`
	result := a.Analyze("file:///test.bat", content)

	diags := result.GetDiagnostics()
	if len(diags) == 0 {
		t.Error("expected diagnostics for unused label")
	}
}

func TestForVariableResolution(t *testing.T) {
	a := NewAnalyzer()
	content := `@echo off
for /L %%n in (1, 1, 3) do echo Step %%n
`
	result := a.Analyze("file:///test.bat", content)

	forVarDiags := 0
	for _, d := range result.Diagnostics {
		if d.Message == "Undefined FOR loop variable: N" {
			forVarDiags++
		}
	}
	if forVarDiags > 0 {
		t.Errorf("expected no undefined FOR variable diagnostics, got %d", forVarDiags)
	}

	if sym := result.Symbols.ForVars["N"]; sym == nil {
		t.Error("expected FOR variable N to be defined")
	} else if sym.RefCount() == 0 {
		t.Error("expected FOR variable N to have references")
	}
}

func TestForwardLabelReference(t *testing.T) {
	a := NewAnalyzer()
	content := `@echo off
goto :end
echo middle
:end
echo end
`
	result := a.Analyze("file:///test.bat", content)

	endLabel := result.Symbols.Labels["end"]
	if endLabel == nil {
		t.Fatal("expected 'end' label to be defined")
	}

	if endLabel.RefCount() == 0 {
		t.Error("expected 'end' label to have references (forward goto)")
	}
}

func TestUserDefinedEOFLabel(t *testing.T) {
	a := NewAnalyzer()
	content := `@echo off
goto eof
echo middle
:EOF
echo custom eof
`
	result := a.Analyze("file:///test.bat", content)

	eofLabel := result.Symbols.Labels["eof"]
	if eofLabel == nil {
		t.Fatal("expected 'eof' label to be defined")
	}

	if eofLabel.RefCount() == 0 {
		t.Error("expected user-defined 'eof' label to have references")
	}
}

func TestCallWithoutSpace(t *testing.T) {
	a := NewAnalyzer()
	content := `@echo off
call:subroutine
echo done
:subroutine
echo in subroutine
`
	result := a.Analyze("file:///test.bat", content)

	subLabel := result.Symbols.Labels["subroutine"]
	if subLabel == nil {
		t.Fatal("expected 'subroutine' label to be defined")
	}

	if subLabel.RefCount() == 0 {
		t.Error("expected 'subroutine' label to have references (call without space)")
	}
}

func TestNestedForVariableResolution(t *testing.T) {
	a := NewAnalyzer()
	content := `@echo off
if "x"=="x" (
    for /L %%i in (1, 1, 2) do (
        echo Loop %%i inside nested IF
    )
)
`
	result := a.Analyze("file:///test.bat", content)

	forVarDiags := 0
	for _, d := range result.Diagnostics {
		if d.Message == "Undefined FOR loop variable: I" {
			forVarDiags++
		}
	}
	if forVarDiags > 0 {
		t.Errorf("expected no undefined FOR variable diagnostics in nested context, got %d", forVarDiags)
	}
}

func TestForFTokenVariables(t *testing.T) {
	a := NewAnalyzer()
	content := `@echo off
for /F "tokens=1,2,3 delims=," %%a in ("one,two,three") do (
    echo First: %%a
    echo Second: %%b
    echo Third: %%c
)
`
	result := a.Analyze("file:///test.bat", content)

	for _, d := range result.Diagnostics {
		if d.Message == "Undefined FOR loop variable: B" || d.Message == "Undefined FOR loop variable: C" {
			t.Errorf("unexpected diagnostic: %s", d.Message)
		}
	}

	if sym := result.Symbols.ForVars["A"]; sym == nil {
		t.Error("expected FOR variable A to be defined")
	}
	if sym := result.Symbols.ForVars["B"]; sym == nil {
		t.Error("expected FOR variable B to be defined (from tokens=1,2,3)")
	}
	if sym := result.Symbols.ForVars["C"]; sym == nil {
		t.Error("expected FOR variable C to be defined (from tokens=1,2,3)")
	}
}

func TestForFTokenRangeVariables(t *testing.T) {
	a := NewAnalyzer()
	content := `@echo off
for /F "tokens=1-3" %%x in ("one two three") do (
    echo %%x %%y %%z
)
`
	result := a.Analyze("file:///test.bat", content)

	for _, d := range result.Diagnostics {
		if d.Message == "Undefined FOR loop variable: Y" || d.Message == "Undefined FOR loop variable: Z" {
			t.Errorf("unexpected diagnostic: %s", d.Message)
		}
	}

	if sym := result.Symbols.ForVars["X"]; sym == nil {
		t.Error("expected FOR variable X to be defined")
	}
	if sym := result.Symbols.ForVars["Y"]; sym == nil {
		t.Error("expected FOR variable Y to be defined (from tokens=1-3)")
	}
	if sym := result.Symbols.ForVars["Z"]; sym == nil {
		t.Error("expected FOR variable Z to be defined (from tokens=1-3)")
	}
}
