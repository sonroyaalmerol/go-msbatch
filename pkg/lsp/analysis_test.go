package lsp

import (
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/analyzer"
)

func TestDefinitionAt(t *testing.T) {
	content := `@echo off
set MYVAR=hello
:mylabel
echo %MYVAR%
goto mylabel
`
	workspace := map[string]*Document{
		"file:///test.bat": {
			Content: content,
			Result:  NewAnalyzer().Analyze("file:///test.bat", content),
		},
	}

	loc, found := DefinitionAt(workspace, "file:///test.bat", 4, 5)
	if !found {
		t.Error("expected to find definition for goto mylabel")
	}
	if loc.Line != 2 {
		t.Errorf("expected definition line 2, got %d", loc.Line)
	}
}

func TestReferencesAt(t *testing.T) {
	content := `@echo off
:mylabel
echo test
goto mylabel
`
	workspace := map[string]*Document{
		"file:///test.bat": {
			Content: content,
			Result:  NewAnalyzer().Analyze("file:///test.bat", content),
		},
	}

	refs := ReferencesAt(workspace, "file:///test.bat", 1, 1, true)
	if len(refs) == 0 {
		t.Error("expected to find references for mylabel")
	}
}

func TestPrepareRenameAt(t *testing.T) {
	content := `@echo off
:mylabel
goto mylabel
`
	loc, found := PrepareRenameAt(content, 1, 1)
	if !found {
		t.Error("expected to find symbol for rename at label definition")
	}
	if loc.Line != 1 {
		t.Errorf("expected line 1, got %d", loc.Line)
	}

	loc, found = PrepareRenameAt(content, 2, 5)
	if !found {
		t.Error("expected to find symbol for rename at goto reference")
	}
}

func TestSemanticTokens(t *testing.T) {
	content := `@echo off
set MYVAR=hello
:mylabel
echo %MYVAR%
`
	tokens := SemanticTokens(content)
	if len(tokens) == 0 {
		t.Error("expected semantic tokens to be generated")
	}

	hasVar := false
	hasLabel := false
	for _, tok := range tokens {
		if tok.TokenType == tokenTypeVariable {
			hasVar = true
		}
		if tok.TokenType == tokenTypeFunction {
			hasLabel = true
		}
	}
	if !hasVar {
		t.Error("expected variable tokens")
	}
	if !hasLabel {
		t.Error("expected label/function tokens")
	}
}

func TestFoldingRanges(t *testing.T) {
	content := `@echo off
setlocal
set MYVAR=hello
echo %MYVAR%
endlocal
`
	folds := FoldingRanges(content)
	if len(folds) == 0 {
		t.Error("expected folding ranges to be found")
	}
}

func TestCodeLenses(t *testing.T) {
	content := `@echo off
:mylabel
echo test
goto mylabel
`
	lenses := CodeLenses(content)
	if len(lenses) == 0 {
		t.Error("expected code lenses to be generated")
	}

	found := false
	for _, lens := range lenses {
		if lens.LabelName == "mylabel" {
			found = true
			if lens.RefCount != 1 {
				t.Errorf("expected 1 reference, got %d", lens.RefCount)
			}
		}
	}
	if !found {
		t.Error("expected to find code lens for 'mylabel'")
	}
}

func TestWordAtPosition(t *testing.T) {
	tests := []struct {
		line     string
		col      int
		expected string
	}{
		{"echo hello", 2, "echo"},
		{"echo hello", 6, "hello"},
		{"set MYVAR=value", 5, "MYVAR"},
		{"goto mylabel", 6, "mylabel"},
	}

	for _, tt := range tests {
		result := WordAtPosition(tt.line, tt.col)
		if result != tt.expected {
			t.Errorf("WordAtPosition(%q, %d) = %q, want %q", tt.line, tt.col, result, tt.expected)
		}
	}
}

func TestCompletionContextAt(t *testing.T) {
	tests := []struct {
		lineBefore string
		expected   CompletionContext
	}{
		{"ech", CompleteCommand},
		{"goto ", CompleteLabel},
		{"call :", CompleteLabel},
		{"echo %MY", CompleteVariable},
		{"for %%i in (*) do echo %%", CompleteForVariable},
	}

	for _, tt := range tests {
		result := CompletionContextAt(tt.lineBefore)
		if result != tt.expected {
			t.Errorf("CompletionContextAt(%q) = %v, want %v", tt.lineBefore, result, tt.expected)
		}
	}
}

func TestEncodeSemanticTokens(t *testing.T) {
	tokens := []SemToken{
		{Line: 0, Col: 0, Len: 5, TokenType: 0, Modifiers: 0},
		{Line: 0, Col: 10, Len: 3, TokenType: 1, Modifiers: 0},
		{Line: 1, Col: 0, Len: 4, TokenType: 2, Modifiers: 1},
	}

	data := EncodeSemanticTokens(tokens)

	expectedLen := len(tokens) * 5
	if len(data) != expectedLen {
		t.Errorf("expected %d data elements, got %d", expectedLen, len(data))
	}
}

func TestFindSymbolAtPosition(t *testing.T) {
	content := `@echo off
:mylabel
echo test
goto mylabel
`
	doc := &Document{
		Content: content,
		Result:  NewAnalyzer().Analyze("file:///test.bat", content),
	}

	sym := findSymbolAtPosition(doc, 1, 1)
	if sym == nil {
		t.Error("expected to find label symbol")
	}
	if sym.Kind != analyzer.SymbolLabel {
		t.Errorf("expected SymbolLabel, got %v", sym.Kind)
	}
}

func TestVariableReferenceEndCol(t *testing.T) {
	content := `@echo off
set MYVAR=hello
echo %MYVAR%
`
	doc := &Document{
		Content: content,
		Result:  NewAnalyzer().Analyze("file:///test.bat", content),
	}

	sym := findSymbolAtPosition(doc, 2, 6)
	if sym == nil {
		t.Fatal("expected to find variable symbol")
	}
	if sym.Name != "MYVAR" {
		t.Errorf("expected MYVAR, got %s", sym.Name)
	}

	var readRef *analyzer.Reference
	for i := range sym.References {
		if sym.References[i].Kind == analyzer.RefRead {
			readRef = &sym.References[i]
			break
		}
	}
	if readRef == nil {
		t.Fatal("expected to find read reference")
	}

	if readRef.Location.Col != 6 {
		t.Errorf("expected Col 6 (position of M in MYVAR), got %d", readRef.Location.Col)
	}
	if readRef.Location.EndCol != 11 {
		t.Errorf("expected EndCol 11 (position after R in MYVAR), got %d", readRef.Location.EndCol)
	}
}

func TestForVarDefinitionAndReference(t *testing.T) {
	content := `@echo off
for %%i in (*.txt) do echo %%i
`
	doc := &Document{
		Content: content,
		Result:  NewAnalyzer().Analyze("file:///test.bat", content),
	}

	sym := findSymbolAtPosition(doc, 1, 4)
	if sym == nil {
		t.Fatal("expected to find FOR variable symbol at definition position")
	}
	if sym.Name != "I" {
		t.Errorf("expected FOR variable I, got %s", sym.Name)
	}
	if sym.Kind != analyzer.SymbolForVar {
		t.Errorf("expected SymbolForVar, got %v", sym.Kind)
	}

	sym2 := findSymbolAtPosition(doc, 1, 27)
	if sym2 == nil {
		t.Fatal("expected to find FOR variable symbol at reference position")
	}
	if sym2.Name != "I" {
		t.Errorf("expected FOR variable I at reference, got %s", sym2.Name)
	}
}

func TestForVarReferenceEndCol(t *testing.T) {
	content := `@echo off
for %%a in (*.txt) do echo %%a
`
	doc := &Document{
		Content: content,
		Result:  NewAnalyzer().Analyze("file:///test.bat", content),
	}

	sym := findSymbolAtPosition(doc, 1, 27)
	if sym == nil {
		t.Fatal("expected to find FOR variable symbol")
	}

	var readRef *analyzer.Reference
	for i := range sym.References {
		if sym.References[i].Kind == analyzer.RefRead && sym.References[i].Location.Line == 1 {
			if sym.References[i].Location.Col >= 25 {
				readRef = &sym.References[i]
			}
		}
	}
	if readRef == nil {
		t.Fatal("expected to find read reference at echo %%a")
	}

	if readRef.Location.Col != 27 {
		t.Errorf("expected Col 27 (position of first %%), got %d", readRef.Location.Col)
	}
	if readRef.Location.EndCol != 30 {
		t.Errorf("expected EndCol 30 (position after 'a'), got %d", readRef.Location.EndCol)
	}
}

func TestDefinitionAtCallTarget(t *testing.T) {
	mainContent := `@echo off
call helper.bat
echo done
`
	helperContent := `@echo off
echo Hello from helper
`

	workspace := map[string]*Document{
		"file:///test/main.bat": {
			Content: mainContent,
			Result:  NewAnalyzer().Analyze("file:///test/main.bat", mainContent),
		},
		"file:///test/helper.bat": {
			Content: helperContent,
			Result:  NewAnalyzer().Analyze("file:///test/helper.bat", helperContent),
		},
	}

	loc, found := DefinitionAt(workspace, "file:///test/main.bat", 1, 6)
	if !found {
		t.Error("expected to find definition for call helper.bat")
	}
	if loc.URI != "file:///test/helper.bat" {
		t.Errorf("expected URI to be helper.bat, got %s", loc.URI)
	}
}

func TestForVarReferences(t *testing.T) {
	content := `@echo off
for %%x in (a b c) do echo %%x
`
	doc := &Document{
		Content: content,
		Result:  NewAnalyzer().Analyze("file:///test.bat", content),
	}

	sym := findSymbolAtPosition(doc, 1, 5)
	if sym == nil {
		t.Fatal("expected to find FOR variable symbol")
	}

	refs := ReferencesAt(map[string]*Document{"file:///test.bat": doc}, "file:///test.bat", 1, 5, true)
	if len(refs) == 0 {
		t.Error("expected to find references for FOR variable")
	}

	hasRef := false
	for _, ref := range refs {
		if ref.Line == 1 && ref.Col >= 25 && ref.Col <= 27 {
			hasRef = true
		}
	}
	if !hasRef {
		t.Error("expected reference at line 1, col ~25-27 (echo percentX)")
	}
}

func TestLabelReferenceEndCol(t *testing.T) {
	content := `@echo off
:start
echo test
goto start
`
	doc := &Document{
		Content: content,
		Result:  NewAnalyzer().Analyze("file:///test.bat", content),
	}

	sym := findSymbolAtPosition(doc, 1, 1)
	if sym == nil {
		t.Fatal("expected to find label symbol")
	}

	var gotoRef *analyzer.Reference
	for i := range sym.References {
		if sym.References[i].Kind == analyzer.RefGoto {
			gotoRef = &sym.References[i]
			break
		}
	}
	if gotoRef == nil {
		t.Fatal("expected to find goto reference")
	}

	if gotoRef.Location.EndCol <= gotoRef.Location.Col {
		t.Errorf("expected EndCol > Col, got Col=%d, EndCol=%d", gotoRef.Location.Col, gotoRef.Location.EndCol)
	}
}

func TestDefinitionAtCallTargetWithSubdir(t *testing.T) {
	mainContent := `@echo off
call scripts/helper.bat
`
	helperContent := `@echo off
echo Hello
`

	workspace := map[string]*Document{
		"file:///test/main.bat": {
			Content: mainContent,
			Result:  NewAnalyzer().Analyze("file:///test/main.bat", mainContent),
		},
		"file:///test/scripts/helper.bat": {
			Content: helperContent,
			Result:  NewAnalyzer().Analyze("file:///test/scripts/helper.bat", helperContent),
		},
	}

	loc, found := DefinitionAt(workspace, "file:///test/main.bat", 1, 6)
	if !found {
		t.Error("expected to find definition for call scripts/helper.bat")
	}
	if loc.URI != "file:///test/scripts/helper.bat" {
		t.Errorf("expected URI to be scripts/helper.bat, got %s", loc.URI)
	}
}

func TestDefinitionAtCallTargetNotExists(t *testing.T) {
	mainContent := `@echo off
call nonexistent.bat
`

	workspace := map[string]*Document{
		"file:///test/main.bat": {
			Content: mainContent,
			Result:  NewAnalyzer().Analyze("file:///test/main.bat", mainContent),
		},
	}

	_, found := DefinitionAt(workspace, "file:///test/main.bat", 1, 6)
	if found {
		t.Error("expected NOT to find definition for nonexistent.bat")
	}
}

func TestForVarNestedScopes(t *testing.T) {
	content := `@echo off
for %%a in (1 2 3) do (
    for %%b in (x y z) do echo %%a %%b
)
`
	doc := &Document{
		Content: content,
		Result:  NewAnalyzer().Analyze("file:///test.bat", content),
	}

	symA := findSymbolAtPosition(doc, 1, 4)
	if symA == nil {
		t.Fatal("expected to find FOR variable a")
	}

	symB := findSymbolAtPosition(doc, 2, 8)
	if symB == nil {
		t.Fatal("expected to find FOR variable b")
	}

	if symA.Name == symB.Name {
		t.Error("expected different FOR variables")
	}
}

func TestVariableDefinitionEndCol(t *testing.T) {
	content := `@echo off
set MYVAR=hello
echo %MYVAR%
`
	doc := &Document{
		Content: content,
		Result:  NewAnalyzer().Analyze("file:///test.bat", content),
	}

	sym := findSymbolAtPosition(doc, 1, 4)
	if sym == nil {
		t.Fatal("expected to find variable symbol")
	}

	def := sym.Definition
	if def.Col != 4 {
		t.Errorf("expected Col 4, got %d", def.Col)
	}

	var defRef *analyzer.Reference
	for i := range sym.References {
		if sym.References[i].Kind == analyzer.RefDefinition {
			defRef = &sym.References[i]
			break
		}
	}
	if defRef == nil {
		t.Fatal("expected to find definition reference")
	}

	if defRef.Location.Col != 4 {
		t.Errorf("expected definition Col 4, got %d", defRef.Location.Col)
	}
}
