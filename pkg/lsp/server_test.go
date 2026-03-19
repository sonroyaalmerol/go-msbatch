package lsp

import (
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
