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
