package lsp

import (
	"strings"
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/lsp/store"
)

func TestSemanticTokensUndefinedLabel(t *testing.T) {
	s := store.New()
	doc := s.Put("test.bat", 1, `@echo off
GOTO undefined_label
:defined_label
echo Hello
`)

	if doc == nil {
		t.Fatal("document is nil")
	}

	if doc.Analysis == nil {
		t.Fatal("analysis is nil")
	}

	if len(doc.Analysis.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(doc.Analysis.Labels))
	}

	if label, ok := doc.Analysis.Labels["UNDEFINED_LABEL"]; ok {
		if label.Definition.Start.Line >= 0 {
			t.Errorf("undefined label should have negative definition line, got %d", label.Definition.Start.Line)
		}
		if len(label.References) == 0 {
			t.Error("undefined label should have references")
		}
	} else {
		t.Error("UNDEFINED_LABEL not found in labels")
	}

	if label, ok := doc.Analysis.Labels["DEFINED_LABEL"]; ok {
		if label.Definition.Start.Line < 0 {
			t.Errorf("defined label should have non-negative definition line, got %d", label.Definition.Start.Line)
		}
	} else {
		t.Error("DEFINED_LABEL not found in labels")
	}

	diags := doc.Analysis.Diagnostics
	found := false
	for _, d := range diags {
		if d.Code == "undefined-label" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected undefined-label diagnostic")
	}
}

func TestMissingEndlocalDiagnostic(t *testing.T) {
	s := store.New()
	doc := s.Put("test.bat", 1, `@echo off
SETLOCAL
set MYVAR=value
`)

	if doc == nil {
		t.Fatal("document is nil")
	}

	if doc.Analysis == nil {
		t.Fatal("analysis is nil")
	}

	diags := doc.Analysis.Diagnostics
	found := false
	for _, d := range diags {
		if d.Code == "missing-endlocal" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected missing-endlocal diagnostic")
	}
}

func TestGetLabelAt(t *testing.T) {
	s := store.New()
	doc := s.Put("test.bat", 1, `@echo off
:mylabel
echo Hello
GOTO mylabel
`)

	if doc == nil {
		t.Fatal("document is nil")
	}

	if doc.Analysis == nil {
		t.Fatal("analysis is nil")
	}

	label := doc.Analysis.GetLabelAt(1, 1)
	if label == nil {
		t.Error("expected to find label at line 1")
	} else if label.Name != "MYLABEL" {
		t.Errorf("expected MYLABEL, got %s", label.Name)
	}

	label = doc.Analysis.GetLabelAt(3, 5)
	if label == nil {
		t.Error("expected to find label reference at line 3")
	} else if label.Name != "MYLABEL" {
		t.Errorf("expected MYLABEL, got %s", label.Name)
	}
}

func TestGetVariableAt(t *testing.T) {
	s := store.New()
	doc := s.Put("test.bat", 1, `@echo off
set MYVAR=hello
echo %MYVAR%
`)

	if doc == nil {
		t.Fatal("document is nil")
	}

	if doc.Analysis == nil {
		t.Fatal("analysis is nil")
	}

	v := doc.Analysis.GetVariableAt(2, 6)
	if v == nil {
		t.Error("expected to find variable at line 2")
	} else if v.Name != "MYVAR" {
		t.Errorf("expected MYVAR, got %s", v.Name)
	}
}

func TestVariableDefinitionNotAtEchoOff(t *testing.T) {
	s := store.New()
	doc := s.Put("test.bat", 1, `@echo off
set MYVAR=hello
echo %MYVAR%
`)

	if doc == nil {
		t.Fatal("document is nil")
	}

	v := doc.Analysis.GetVariableAt(2, 6)
	if v == nil {
		t.Fatal("expected to find variable at line 2")
	}

	if v.Definition.Start.Line < 0 {
		t.Errorf("expected variable definition to have valid line, got %d", v.Definition.Start.Line)
	}

	if v.Definition.Start.Line == 0 {
		t.Error("variable definition should not point to @echo off line")
	}

	if v.Value != "hello" {
		t.Errorf("expected variable value 'hello', got %q", v.Value)
	}
}

func TestUndefinedVariableHasNoDefinition(t *testing.T) {
	s := store.New()
	doc := s.Put("test.bat", 1, `@echo off
echo %UNDEFINED%
`)

	if doc == nil {
		t.Fatal("document is nil")
	}

	v := doc.Analysis.GetVariableAt(1, 6)
	if v == nil {
		t.Fatal("expected to find variable reference at line 1")
	}

	if v.Definition.Start.Line >= 0 {
		t.Errorf("undefined variable should have negative definition line, got %d", v.Definition.Start.Line)
	}
}

func TestCallEofNotFlagged(t *testing.T) {
	s := store.New()
	doc := s.Put("test.bat", 1, `@echo off
call :sub
echo done
goto :eof
:sub
echo in sub
call :eof
echo should not print
`)

	if doc == nil {
		t.Fatal("document is nil")
	}

	if doc.Analysis == nil {
		t.Fatal("analysis is nil")
	}

	for _, d := range doc.Analysis.Diagnostics {
		if d.Code == "undefined-label" {
			t.Errorf("unexpected undefined-label diagnostic: %s", d.Message)
		}
	}

	for _, d := range doc.Diags {
		if strings.Contains(d.Message, "undefined label") {
			t.Errorf("unexpected undefined label diagnostic: %s", d.Message)
		}
	}
}

func TestDynamicGotoNotFlagged(t *testing.T) {
	s := store.New()
	doc := s.Put("test.bat", 1, `@echo off
set TARGET=MYLABEL
goto %TARGET%
:MYLABEL
echo reached
`)

	if doc == nil {
		t.Fatal("document is nil")
	}

	if doc.Analysis == nil {
		t.Fatal("analysis is nil")
	}

	for _, d := range doc.Analysis.Diagnostics {
		if d.Code == "undefined-label" {
			t.Errorf("unexpected undefined-label diagnostic for dynamic label: %s", d.Message)
		}
	}
}

func TestParenthesesInBlockNotFlagged(t *testing.T) {
	s := store.New()
	doc := s.Put("test.bat", 1, `@echo off
if 1==1 (
    echo test (paren)
)
`)

	if doc == nil {
		t.Fatal("document is nil")
	}

	for _, d := range doc.Diags {
		if strings.Contains(d.Message, "unexpected token") {
			t.Errorf("unexpected parser diagnostic: %s", d.Message)
		}
	}
}

func TestForwardGotoLabelReference(t *testing.T) {
	s := store.New()
	doc := s.Put("test.bat", 1, `@echo off
goto :END
:MIDDLE
echo middle
:END
echo end
`)

	if doc == nil {
		t.Fatal("document is nil")
	}

	if doc.Analysis == nil {
		t.Fatal("analysis is nil")
	}

	label := doc.Analysis.Labels["END"]
	if label == nil {
		t.Fatal("expected END label to exist")
	}

	if label.Definition.Start.Line < 0 {
		t.Errorf("expected END label to have valid definition, got line %d", label.Definition.Start.Line)
	}

	if len(label.References) != 1 {
		t.Errorf("expected END label to have 1 reference, got %d", len(label.References))
	}

	labelAtRef := doc.Analysis.GetLabelAt(1, 1)
	if labelAtRef == nil {
		t.Error("expected to find label at goto :END line")
	} else if labelAtRef.Name != "END" {
		t.Errorf("expected END, got %s", labelAtRef.Name)
	}

	labelAtDef := doc.Analysis.GetLabelAt(4, 1)
	if labelAtDef == nil {
		t.Error("expected to find label at :END definition")
	} else if labelAtDef.Name != "END" {
		t.Errorf("expected END, got %s", labelAtDef.Name)
	}
}

func TestCallLabelReference(t *testing.T) {
	s := store.New()
	doc := s.Put("test.bat", 1, `@echo off
call :SUB
echo done
:SUB
echo in sub
`)

	if doc == nil {
		t.Fatal("document is nil")
	}

	if doc.Analysis == nil {
		t.Fatal("analysis is nil")
	}

	label := doc.Analysis.Labels["SUB"]
	if label == nil {
		t.Fatal("expected SUB label to exist")
	}

	if label.Definition.Start.Line < 0 {
		t.Errorf("expected SUB label to have valid definition, got line %d", label.Definition.Start.Line)
	}

	if len(label.References) != 1 {
		t.Errorf("expected SUB label to have 1 reference, got %d", len(label.References))
	}

	labelAtRef := doc.Analysis.GetLabelAt(1, 6)
	if labelAtRef == nil {
		t.Error("expected to find label at call :SUB line")
	} else if labelAtRef.Name != "SUB" {
		t.Errorf("expected SUB, got %s", labelAtRef.Name)
	}
}
