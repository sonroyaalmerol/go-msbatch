package analyzer

import (
	"testing"
)

func TestNewWorkspaceAnalyzer(t *testing.T) {
	wa := NewWorkspaceAnalyzer()
	if wa == nil {
		t.Fatal("expected workspace analyzer to be created")
	}
}

func TestWorkspaceAnalyzerAnalyzeDocument(t *testing.T) {
	wa := NewWorkspaceAnalyzer()
	content := `@echo off
set MYVAR=hello
echo %MYVAR%
`

	result := wa.AnalyzeDocument("file:///test.bat", content)

	if result == nil {
		t.Error("expected result to be non-nil")
	}
	if result.Symbols == nil {
		t.Error("expected symbols to be non-nil")
	}
}

func TestWorkspaceAnalyzerRemoveDocument(t *testing.T) {
	wa := NewWorkspaceAnalyzer()
	content := `@echo off
echo test
`

	wa.AnalyzeDocument("file:///test.bat", content)

	if wa.GetDocument("file:///test.bat") == nil {
		t.Error("expected document to exist")
	}

	wa.RemoveDocument("file:///test.bat")

	if wa.GetDocument("file:///test.bat") != nil {
		t.Error("expected document to be removed")
	}
}

func TestWorkspaceAnalyzerCallGraph(t *testing.T) {
	wa := NewWorkspaceAnalyzer()

	files := map[string]string{
		"file:///main.bat": `@echo off
call helper.bat
`,
		"file:///helper.bat": `@echo off
echo Hello
`,
	}

	result := wa.AnalyzeWorkspace(files)

	if result == nil {
		t.Fatal("expected result to be non-nil")
	}

	if len(result.Documents) != 2 {
		t.Errorf("expected 2 documents, got %d", len(result.Documents))
	}
}

func TestWorkspaceAnalyzerGetCalledURIs(t *testing.T) {
	wa := NewWorkspaceAnalyzer()

	wa.AnalyzeDocument("file:///test/main.bat", `@echo off
call helper.bat
`)
	wa.AnalyzeDocument("file:///test/helper.bat", `@echo off
echo test
`)

	called := wa.GetCalledURIs("file:///test/main.bat")

	if len(called) == 0 {
		t.Errorf("expected called URIs for main.bat, got %v", called)
	}
}

func TestWorkspaceAnalyzerGetCallerURIs(t *testing.T) {
	wa := NewWorkspaceAnalyzer()

	wa.AnalyzeDocument("file:///test/main.bat", `@echo off
call helper.bat
`)
	wa.AnalyzeDocument("file:///test/helper.bat", `@echo off
echo test
`)

	callers := wa.GetCallerURIs("file:///test/helper.bat")

	if len(callers) == 0 {
		t.Errorf("expected caller URIs for helper.bat, got %v", callers)
	}
}

func TestWorkspaceAnalyzerGetInheritedVariables(t *testing.T) {
	wa := NewWorkspaceAnalyzer()

	wa.AnalyzeDocument("file:///test/main.bat", `@echo off
set MYVAR=hello
call helper.bat
`)
	wa.AnalyzeDocument("file:///test/helper.bat", `@echo off
echo test
`)

	vars := wa.GetInheritedVariables("file:///test/helper.bat")

	if len(vars) == 0 {
		t.Errorf("expected inherited variables from main.bat, got %v", vars)
	}
}
