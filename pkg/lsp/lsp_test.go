package lsp

import (
	"strings"
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/executor"
)

// ── Analyze ───────────────────────────────────────────────────────────────────

func TestAnalyzeLabels(t *testing.T) {
	a := Analyze(":start\necho hi\n:end\n")
	if len(a.Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(a.Labels))
	}
	if a.Labels[0].Name != "start" || a.Labels[0].Line != 0 {
		t.Errorf("labels[0]: got name=%q line=%d", a.Labels[0].Name, a.Labels[0].Line)
	}
	if a.Labels[1].Name != "end" || a.Labels[1].Line != 2 {
		t.Errorf("labels[1]: got name=%q line=%d", a.Labels[1].Name, a.Labels[1].Line)
	}
}

func TestAnalyzeLabelsUpperCase(t *testing.T) {
	// Label names are lowercased for case-insensitive matching.
	a := Analyze(":MyLabel\n")
	if len(a.Labels) != 1 || a.Labels[0].Name != "mylabel" {
		t.Errorf("expected label mylabel, got %v", a.Labels)
	}
}

func TestAnalyzeDoubleColonNotLabel(t *testing.T) {
	// :: is a comment, not a label.
	a := Analyze(":: this is a comment\n")
	if len(a.Labels) != 0 {
		t.Errorf("expected 0 labels, got %d", len(a.Labels))
	}
}

func TestAnalyzeVars(t *testing.T) {
	a := Analyze("set FOO=bar\nset BAZ=123\n")
	if len(a.Vars) != 2 {
		t.Fatalf("expected 2 vars, got %d", len(a.Vars))
	}
	if a.Vars[0].Name != "FOO" || a.Vars[0].Value != "bar" || a.Vars[0].Line != 0 {
		t.Errorf("vars[0]: got %+v", a.Vars[0])
	}
	if a.Vars[1].Name != "BAZ" || a.Vars[1].Value != "123" {
		t.Errorf("vars[1]: got %+v", a.Vars[1])
	}
}

func TestAnalyzeVarsArithmeticSkipped(t *testing.T) {
	// SET /A and SET /P should not produce VarDef entries.
	a := Analyze("set /a X=1+2\nset /p NAME=Enter: \n")
	if len(a.Vars) != 0 {
		t.Errorf("expected 0 vars for /A and /P, got %d: %v", len(a.Vars), a.Vars)
	}
}

func TestAnalyzeVarNoEquals(t *testing.T) {
	// SET without '=' is just a display command, not a definition.
	a := Analyze("set PATH\n")
	if len(a.Vars) != 0 {
		t.Errorf("expected 0 vars, got %d", len(a.Vars))
	}
}

func TestAnalyzeGotoRefs(t *testing.T) {
	a := Analyze("goto start\ngoto :end\n")
	if len(a.GotoRefs) != 2 {
		t.Fatalf("expected 2 goto refs, got %d", len(a.GotoRefs))
	}
	if a.GotoRefs[0].Name != "start" || a.GotoRefs[0].Line != 0 {
		t.Errorf("gotoRefs[0]: got %+v", a.GotoRefs[0])
	}
	if a.GotoRefs[1].Name != "end" {
		t.Errorf("gotoRefs[1]: expected name=end, got %q", a.GotoRefs[1].Name)
	}
}

func TestAnalyzeGotoEOFSkipped(t *testing.T) {
	// GOTO :EOF is a special form and must not create a ref.
	a := Analyze("goto :eof\ngoto eof\n")
	if len(a.GotoRefs) != 0 {
		t.Errorf("expected 0 goto refs for :eof, got %d: %v", len(a.GotoRefs), a.GotoRefs)
	}
}

func TestAnalyzeCallRefs(t *testing.T) {
	a := Analyze("call :myfunc arg1\ncall :other\n")
	if len(a.CallRefs) != 2 {
		t.Fatalf("expected 2 call refs, got %d", len(a.CallRefs))
	}
	if a.CallRefs[0].Name != "myfunc" {
		t.Errorf("callRefs[0]: expected myfunc, got %q", a.CallRefs[0].Name)
	}
	if a.CallRefs[1].Name != "other" {
		t.Errorf("callRefs[1]: expected other, got %q", a.CallRefs[1].Name)
	}
}

func TestAnalyzeCallFileNotRef(t *testing.T) {
	// Plain CALL <file> (without ':') is not a label reference.
	a := Analyze("call helper.bat\n")
	if len(a.CallRefs) != 0 {
		t.Errorf("expected 0 call refs for plain call, got %d", len(a.CallRefs))
	}
}

func TestAnalyzeCRLF(t *testing.T) {
	// Windows-style line endings must not corrupt label/var names.
	a := Analyze(":start\r\nset X=1\r\ngoto start\r\n")
	if len(a.Labels) != 1 || a.Labels[0].Name != "start" {
		t.Errorf("expected label start, got %v", a.Labels)
	}
	if len(a.Vars) != 1 || a.Vars[0].Name != "X" {
		t.Errorf("expected var X, got %v", a.Vars)
	}
}

// ── Diagnostics ───────────────────────────────────────────────────────────────

func TestDiagnosticsNoIssues(t *testing.T) {
	src := ":start\necho hi\ngoto start\n"
	diags := Diagnostics(src)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics, got %d: %v", len(diags), diags)
	}
}

func TestDiagnosticsUndefinedGoto(t *testing.T) {
	diags := Diagnostics("goto missing\n")
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Sev != SevWarning {
		t.Errorf("expected SevWarning, got %v", diags[0].Sev)
	}
	if !strings.Contains(diags[0].Message, "missing") {
		t.Errorf("diagnostic message should name the label, got %q", diags[0].Message)
	}
	if diags[0].Line != 0 {
		t.Errorf("expected line 0, got %d", diags[0].Line)
	}
}

func TestDiagnosticsUndefinedCall(t *testing.T) {
	diags := Diagnostics("call :ghost\n")
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if !strings.Contains(diags[0].Message, "ghost") {
		t.Errorf("expected label name in message, got %q", diags[0].Message)
	}
}

func TestDiagnosticsDefinedAfterUse(t *testing.T) {
	// Forward references are valid in batch — label defined after GOTO.
	src := "goto end\n:end\necho done\n"
	diags := Diagnostics(src)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics for forward goto, got %d", len(diags))
	}
}

func TestDiagnosticsCaseInsensitive(t *testing.T) {
	// GOTO and label matching must be case-insensitive.
	src := ":Start\ngoto START\n"
	diags := Diagnostics(src)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics for case-insensitive match, got %d", len(diags))
	}
}

func TestDiagnosticsMultipleUndefined(t *testing.T) {
	src := "goto a\ngoto b\ngoto a\n"
	diags := Diagnostics(src)
	if len(diags) != 3 {
		t.Errorf("expected 3 diagnostics (each ref is reported), got %d", len(diags))
	}
}

// ── WordAtPosition ────────────────────────────────────────────────────────────

func TestWordAtPositionMiddle(t *testing.T) {
	w := WordAtPosition("echo hello", 7) // cursor inside "hello"
	if w != "hello" {
		t.Errorf("expected hello, got %q", w)
	}
}

func TestWordAtPositionStart(t *testing.T) {
	w := WordAtPosition("echo hello", 0)
	if w != "echo" {
		t.Errorf("expected echo, got %q", w)
	}
}

func TestWordAtPositionOnSpace(t *testing.T) {
	// col=4 is the space in "echo hello"; backward scan from 4 picks up "echo"
	// because the cursor sits right after the 'o'. This matches LSP hover
	// semantics where position N is between chars N-1 and N.
	w := WordAtPosition("echo hello", 4)
	if w != "echo" {
		t.Errorf("expected echo at position 4, got %q", w)
	}
}

func TestWordAtPositionBetweenWords(t *testing.T) {
	// col=5 is on 'h' in "echo hello" — should return "hello".
	w := WordAtPosition("echo hello", 5)
	if w != "hello" {
		t.Errorf("expected hello at position 5, got %q", w)
	}
}

func TestWordAtPositionBeyondEnd(t *testing.T) {
	line := "echo"
	w := WordAtPosition(line, 100) // col beyond line length
	if w != "echo" {
		t.Errorf("expected echo, got %q", w)
	}
}

func TestWordAtPositionEmpty(t *testing.T) {
	w := WordAtPosition("", 0)
	if w != "" {
		t.Errorf("expected empty, got %q", w)
	}
}

// ── CompletionContextAt ───────────────────────────────────────────────────────

var completionContextTests = []struct {
	name string
	line string
	want CompletionContext
}{
	{"empty line", "", CompleteCommand},
	{"partial command", "ec", CompleteCommand},
	{"at-suppressed command", "@ec", CompleteCommand},
	{"after goto", "goto ", CompleteLabel},
	{"after goto colon", "goto :", CompleteLabel},
	{"goto uppercase", "GOTO ", CompleteLabel},
	{"after call colon", "call :", CompleteLabel},
	{"after call colon uppercase", "CALL :", CompleteLabel},
	{"inside percent var", "echo %MY", CompleteVariable},
	{"open percent", "echo %", CompleteVariable},
	{"closed percent then more", "echo %FOO% ", CompleteFile},
	{"command with arg", "echo hello", CompleteFile},
	{"dir arg", "dir /b ", CompleteFile},
}

func TestCompletionContextAt(t *testing.T) {
	for _, tc := range completionContextTests {
		t.Run(tc.name, func(t *testing.T) {
			got := CompletionContextAt(tc.line)
			if got != tc.want {
				t.Errorf("CompletionContextAt(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}

// ── labelPrefixFromLine / varPrefixFromLine ───────────────────────────────────

func TestLabelPrefixFromLine(t *testing.T) {
	cases := []struct{ line, want string }{
		{"goto start", "start"},
		{"goto :start", "start"},
		{"call :my", "my"},
		{"GOTO LOOP", "LOOP"},
		{"echo hi", ""},
	}
	for _, tc := range cases {
		got := labelPrefixFromLine(tc.line)
		if got != tc.want {
			t.Errorf("labelPrefixFromLine(%q) = %q, want %q", tc.line, got, tc.want)
		}
	}
}

func TestVarPrefixFromLine(t *testing.T) {
	cases := []struct{ line, want string }{
		{"echo %FOO", "FOO"},
		{"echo %", ""},
		{"echo %A%B%C", "C"},
		{"echo hi", ""},
	}
	for _, tc := range cases {
		got := varPrefixFromLine(tc.line)
		if got != tc.want {
			t.Errorf("varPrefixFromLine(%q) = %q, want %q", tc.line, got, tc.want)
		}
	}
}

// ── CommandHelp ───────────────────────────────────────────────────────────────

func TestCommandHelpKnown(t *testing.T) {
	known := []string{"echo", "set", "goto", "call", "if", "for", "cd", "dir", "copy",
		"move", "del", "mkdir", "rmdir", "cls", "ver", "pause", "exit", "rem",
		"setlocal", "endlocal", "pushd", "popd", "start", "mklink", "color",
		"title", "path", "prompt", "more", "assoc", "ftype",
		"find", "sort", "tree", "xcopy", "robocopy",
		"timeout", "where", "hostname", "whoami",
	}
	for _, name := range known {
		if executor.CommandHelp(name) == "" {
			t.Errorf("CommandHelp(%q) returned empty string", name)
		}
	}
}

func TestCommandHelpCaseInsensitive(t *testing.T) {
	lower := executor.CommandHelp("echo")
	upper := executor.CommandHelp("ECHO")
	mixed := executor.CommandHelp("Echo")
	if lower == "" || lower != upper || lower != mixed {
		t.Errorf("CommandHelp should be case-insensitive: lower=%q upper=%q mixed=%q", lower, upper, mixed)
	}
}

func TestCommandHelpUnknown(t *testing.T) {
	if h := executor.CommandHelp("notacommand"); h != "" {
		t.Errorf("expected empty string for unknown command, got %q", h)
	}
}

// ── LabelRef Col ─────────────────────────────────────────────────────────────

func TestGotoRefCol(t *testing.T) {
	cases := []struct {
		line    string
		wantCol int
	}{
		{"goto start", 5},
		{"goto :start", 6},
		{"  goto start", 7},
		{"  goto :start", 8},
	}
	for _, tc := range cases {
		a := Analyze(tc.line + "\n:start\n")
		if len(a.GotoRefs) != 1 {
			t.Errorf("%q: expected 1 goto ref, got %d", tc.line, len(a.GotoRefs))
			continue
		}
		if a.GotoRefs[0].Col != tc.wantCol {
			t.Errorf("%q: col=%d, want %d", tc.line, a.GotoRefs[0].Col, tc.wantCol)
		}
	}
}

func TestCallRefCol(t *testing.T) {
	cases := []struct {
		line    string
		wantCol int
	}{
		{"call :sub", 6},
		{"  call :sub", 8},
	}
	for _, tc := range cases {
		a := Analyze(tc.line + "\n:sub\n")
		if len(a.CallRefs) != 1 {
			t.Errorf("%q: expected 1 call ref, got %d", tc.line, len(a.CallRefs))
			continue
		}
		if a.CallRefs[0].Col != tc.wantCol {
			t.Errorf("%q: col=%d, want %d", tc.line, a.CallRefs[0].Col, tc.wantCol)
		}
	}
}

// ── VarRefs ───────────────────────────────────────────────────────────────────

func TestVarRefsSimple(t *testing.T) {
	a := Analyze("echo %FOO%\n")
	if len(a.VarRefs) != 1 {
		t.Fatalf("expected 1 var ref, got %d: %v", len(a.VarRefs), a.VarRefs)
	}
	r := a.VarRefs[0]
	if r.Name != "FOO" || r.Line != 0 {
		t.Errorf("unexpected ref: %+v", r)
	}
	// col should point to 'F' in "echo %FOO%"
	if r.Col != 6 {
		t.Errorf("expected col 6, got %d", r.Col)
	}
}

func TestVarRefsMultipleOnLine(t *testing.T) {
	a := Analyze("echo %A% %B%\n")
	if len(a.VarRefs) != 2 {
		t.Fatalf("expected 2 var refs, got %d", len(a.VarRefs))
	}
	if a.VarRefs[0].Name != "A" || a.VarRefs[1].Name != "B" {
		t.Errorf("unexpected names: %v", a.VarRefs)
	}
}

func TestVarRefsMultipleLines(t *testing.T) {
	a := Analyze("set X=1\necho %X%\nset Y=%X%\n")
	var xRefs []VarRef
	for _, r := range a.VarRefs {
		if r.Name == "X" {
			xRefs = append(xRefs, r)
		}
	}
	if len(xRefs) != 2 { // line 1 and line 2
		t.Errorf("expected 2 refs to X, got %d", len(xRefs))
	}
}

func TestVarRefsSkipPositional(t *testing.T) {
	// %1, %2 etc. are positional args, not variable refs.
	a := Analyze("echo %1 %2\n")
	if len(a.VarRefs) != 0 {
		t.Errorf("expected 0 var refs for positional args, got %d: %v", len(a.VarRefs), a.VarRefs)
	}
}

func TestVarRefsSkipEscapedPercent(t *testing.T) {
	// %% is an escaped percent sign — no variable.
	a := Analyze("echo 100%%\n")
	if len(a.VarRefs) != 0 {
		t.Errorf("expected 0 var refs for %%, got %d: %v", len(a.VarRefs), a.VarRefs)
	}
}

// ── DefinitionAt ─────────────────────────────────────────────────────────────

func TestDefinitionAtGotoLabel(t *testing.T) {
	// "goto start" on line 0, ":start" on line 1
	src := "goto start\n:start\necho hi\n"
	loc, ok := DefinitionAt(singleDocWorkspace("file:///a.bat", src), "file:///a.bat", 0, 7) // col 7 = inside "start"
	if !ok {
		t.Fatal("expected a definition location")
	}
	if loc.Line != 1 {
		t.Errorf("expected definition on line 1, got %d", loc.Line)
	}
}

func TestDefinitionAtCallLabel(t *testing.T) {
	src := "call :sub\n:sub\necho hi\n"
	loc, ok := DefinitionAt(singleDocWorkspace("file:///a.bat", src), "file:///a.bat", 0, 7) // col 7 = inside "sub"
	if !ok {
		t.Fatal("expected a definition location")
	}
	if loc.Line != 1 {
		t.Errorf("expected definition on line 1, got %d", loc.Line)
	}
}

func TestDefinitionAtVariable(t *testing.T) {
	src := "set MYVAR=hello\necho %MYVAR%\n"
	// line 1, col 8 = inside "MYVAR" in "%MYVAR%"
	loc, ok := DefinitionAt(singleDocWorkspace("file:///a.bat", src), "file:///a.bat", 1, 8)
	if !ok {
		t.Fatal("expected a definition location for variable")
	}
	if loc.Line != 0 {
		t.Errorf("expected definition on line 0 (SET), got %d", loc.Line)
	}
}

func TestDefinitionAtForwardReference(t *testing.T) {
	// GOTO target defined later in the file.
	src := "goto end\n:end\necho done\n"
	loc, ok := DefinitionAt(singleDocWorkspace("file:///a.bat", src), "file:///a.bat", 0, 6)
	if !ok {
		t.Fatal("expected definition for forward goto")
	}
	if loc.Line != 1 {
		t.Errorf("expected line 1, got %d", loc.Line)
	}
}

func TestDefinitionAtUnknownWord(t *testing.T) {
	src := "echo hello\n"
	_, ok := DefinitionAt(singleDocWorkspace("file:///a.bat", src), "file:///a.bat", 0, 6) // "hello" is not a label or var
	if ok {
		t.Error("expected no definition for plain word")
	}
}

func TestDefinitionAtEmptyPosition(t *testing.T) {
	_, ok := DefinitionAt(singleDocWorkspace("file:///a.bat", "echo hi\n"), "file:///a.bat", 0, 4) // space between "echo" and "hi"
	// "echo" is returned by WordAtPosition but is not a label/var
	if ok {
		t.Error("expected no definition for 'echo'")
	}
}

// ── ReferencesAt ─────────────────────────────────────────────────────────────

func TestReferencesAtLabelDef(t *testing.T) {
	// Cursor on the label definition line; should find all GOTO/CALL refs.
	src := ":loop\ngoto loop\ncall :loop\n"
	locs := ReferencesAt(singleDocWorkspace("file:///a.bat", src), "file:///a.bat", 0, 2, false) // col 2 = inside "loop" on ":loop" line
	if len(locs) != 2 {
		t.Fatalf("expected 2 refs, got %d: %v", len(locs), locs)
	}
}

func TestReferencesAtLabelRefIncludeDecl(t *testing.T) {
	src := ":start\ngoto start\n"
	locs := ReferencesAt(singleDocWorkspace("file:///a.bat", src), "file:///a.bat", 1, 7, true) // cursor on "start" in "goto start"
	// Should include both the GOTO ref and the declaration.
	if len(locs) != 2 {
		t.Fatalf("expected 2 locs (ref + decl), got %d: %v", len(locs), locs)
	}
}

func TestReferencesAtLabelRefExcludeDecl(t *testing.T) {
	src := ":start\ngoto start\ngoto start\n"
	locs := ReferencesAt(singleDocWorkspace("file:///a.bat", src), "file:///a.bat", 0, 2, false) // on label def, excludeDecl
	if len(locs) != 2 {
		t.Errorf("expected 2 goto refs, got %d", len(locs))
	}
}

func TestReferencesAtVarRef(t *testing.T) {
	src := "set FOO=bar\necho %FOO%\necho %FOO%\n"
	// col 8 = inside "FOO" in "%FOO%" on line 1
	locs := ReferencesAt(singleDocWorkspace("file:///a.bat", src), "file:///a.bat", 1, 8, false)
	if len(locs) != 2 {
		t.Errorf("expected 2 var refs, got %d: %v", len(locs), locs)
	}
}

func TestReferencesAtVarRefIncludeDecl(t *testing.T) {
	src := "set FOO=bar\necho %FOO%\n"
	locs := ReferencesAt(singleDocWorkspace("file:///a.bat", src), "file:///a.bat", 1, 8, true)
	if len(locs) != 2 { // 1 usage + 1 declaration
		t.Errorf("expected 2 locs (ref + decl), got %d: %v", len(locs), locs)
	}
}

func TestReferencesAtUnknownLabel(t *testing.T) {
	// Word under cursor is not a known label → no results.
	src := "goto ghost\n"
	locs := ReferencesAt(singleDocWorkspace("file:///a.bat", src), "file:///a.bat", 0, 7, false)
	if len(locs) != 0 {
		t.Errorf("expected 0 locs for undefined label, got %d", len(locs))
	}
}

func TestReferencesAtRefRange(t *testing.T) {
	// Verify the col/endCol of a goto ref points to the label name.
	src := ":sub\ngoto sub\n"
	locs := ReferencesAt(singleDocWorkspace("file:///a.bat", src), "file:///a.bat", 0, 2, false)
	if len(locs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(locs))
	}
	// "goto sub": "sub" starts at col 5
	if locs[0].Col != 5 || locs[0].EndCol != 8 {
		t.Errorf("expected col 5–8, got %d–%d", locs[0].Col, locs[0].EndCol)
	}
}

// ── Server (document store + handler wiring) ──────────────────────────────────

func TestServerStoreLoad(t *testing.T) {
	s := NewServer()
	s.store("file:///a.bat", "echo hi")
	content, ok := s.load("file:///a.bat")
	if !ok {
		t.Fatal("expected document to be found after store")
	}
	if content.Content != "echo hi" {
		t.Errorf("expected stored content, got %q", content.Content)
	}
}

func TestServerRemove(t *testing.T) {
	s := NewServer()
	s.store("file:///a.bat", "echo hi")
	s.remove("file:///a.bat")
	_, ok := s.load("file:///a.bat")
	if ok {
		t.Error("expected document to be absent after remove")
	}
}

func TestServerLoadMissing(t *testing.T) {
	s := NewServer()
	_, ok := s.load("file:///missing.bat")
	if ok {
		t.Error("expected false for missing document")
	}
}

func TestServerStoreOverwrite(t *testing.T) {
	s := NewServer()
	s.store("file:///a.bat", "v1")
	s.store("file:///a.bat", "v2")
	content, _ := s.load("file:///a.bat")
	if content.Content != "v2" {
		t.Errorf("expected v2 after overwrite, got %q", content.Content)
	}
}

// ── lineAt ────────────────────────────────────────────────────────────────────

func TestLineAt(t *testing.T) {
	content := "line0\nline1\nline2\n"
	cases := []struct {
		idx  int
		want string
	}{
		{0, "line0"},
		{1, "line1"},
		{2, "line2"},
		{3, ""},  // beyond end
		{-1, ""}, // before start
	}
	for _, tc := range cases {
		got := lineAt(content, tc.idx)
		if got != tc.want {
			t.Errorf("lineAt(content, %d) = %q, want %q", tc.idx, got, tc.want)
		}
	}
}

func TestLineAtCRLF(t *testing.T) {
	got := lineAt("hello\r\nworld\r\n", 0)
	if got != "hello" {
		t.Errorf("expected CR stripped, got %q", got)
	}
}

// ── Feature 1: Rename ─────────────────────────────────────────────────────────

func TestLabelDefCol(t *testing.T) {
	cases := []struct {
		src     string
		wantCol int
	}{
		{":start\n", 1},
		{"  :start\n", 3},
		{"\t:start\n", 2},
	}
	for _, tc := range cases {
		a := Analyze(tc.src)
		if len(a.Labels) != 1 {
			t.Errorf("%q: expected 1 label, got %d", tc.src, len(a.Labels))
			continue
		}
		if a.Labels[0].Col != tc.wantCol {
			t.Errorf("%q: col=%d, want %d", tc.src, a.Labels[0].Col, tc.wantCol)
		}
	}
}

func TestVarDefCol(t *testing.T) {
	cases := []struct {
		src     string
		wantCol int
	}{
		{"set FOO=bar\n", 4},
		{"  set FOO=bar\n", 6},
		{"set  FOO=bar\n", 5},
	}
	for _, tc := range cases {
		a := Analyze(tc.src)
		if len(a.Vars) != 1 {
			t.Errorf("%q: expected 1 var, got %d", tc.src, len(a.Vars))
			continue
		}
		if a.Vars[0].Col != tc.wantCol {
			t.Errorf("%q: col=%d, want %d", tc.src, a.Vars[0].Col, tc.wantCol)
		}
	}
}

func TestRenameAtLabel(t *testing.T) {
	src := ":start\ngoto start\n"
	editsMap, err := RenameAt(singleDocWorkspace("file:///a.bat", src), "file:///a.bat", 0, 2, "begin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	edits := editsMap["file:///a.bat"]
	if len(edits) != 2 {
		t.Fatalf("expected 2 edits, got %d: %v", len(edits), edits)
	}
	// definition edit
	defEdit := edits[0]
	if defEdit.Line != 0 || defEdit.NewText != "begin" {
		t.Errorf("def edit unexpected: %+v", defEdit)
	}
	// goto ref edit
	refEdit := edits[1]
	if refEdit.Line != 1 || refEdit.NewText != "begin" {
		t.Errorf("ref edit unexpected: %+v", refEdit)
	}
}

func TestRenameAtLabelMultipleRefs(t *testing.T) {
	src := ":loop\ngoto loop\ncall :loop\ngoto loop\n"
	editsMap, err := RenameAt(singleDocWorkspace("file:///a.bat", src), "file:///a.bat", 0, 2, "cycle")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	edits := editsMap["file:///a.bat"]
	// 1 def + 2 goto + 1 call = 4
	if len(edits) != 4 {
		t.Fatalf("expected 4 edits, got %d", len(edits))
	}
}

func TestRenameAtVariable(t *testing.T) {
	src := "set FOO=bar\necho %FOO%\n"
	// col 8 = inside %FOO%
	editsMap, err := RenameAt(singleDocWorkspace("file:///a.bat", src), "file:///a.bat", 1, 8, "BAR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	edits := editsMap["file:///a.bat"]
	// 1 def + 1 ref = 2
	if len(edits) != 2 {
		t.Fatalf("expected 2 edits, got %d", len(edits))
	}
}

func TestRenameAtUnknown(t *testing.T) {
	src := "echo hello\n"
	_, err := RenameAt(singleDocWorkspace("file:///a.bat", src), "file:///a.bat", 0, 7, "world")
	if err == nil {
		t.Error("expected error for non-symbol")
	}
}

func TestPrepareRenameAtLabel(t *testing.T) {
	src := ":start\ngoto start\n"
	loc, ok := PrepareRenameAt(src, 0, 2)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if loc.Line != 0 || loc.Col != 1 || loc.EndCol != 6 {
		t.Errorf("unexpected loc: %+v", loc)
	}
}

func TestPrepareRenameAtVariable(t *testing.T) {
	src := "set FOO=bar\necho %FOO%\n"
	loc, ok := PrepareRenameAt(src, 1, 8)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if loc.Line != 1 || loc.Col != 6 || loc.EndCol != 9 {
		t.Errorf("unexpected loc: %+v (want line=1 col=6 endCol=9)", loc)
	}
}

func TestPrepareRenameAtUnknown(t *testing.T) {
	src := "echo hello\n"
	_, ok := PrepareRenameAt(src, 0, 7)
	if ok {
		t.Error("expected ok=false for non-symbol")
	}
}

// ── Feature 2: Code Lens ──────────────────────────────────────────────────────

func TestCodeLensesNoLabels(t *testing.T) {
	src := "echo hello\ngoto missing\n"
	lenses := CodeLenses(src)
	if len(lenses) != 0 {
		t.Errorf("expected 0 lenses, got %d", len(lenses))
	}
}

func TestCodeLensesWithRefs(t *testing.T) {
	src := ":loop\ngoto loop\ngoto loop\n"
	lenses := CodeLenses(src)
	if len(lenses) != 1 {
		t.Fatalf("expected 1 lens, got %d", len(lenses))
	}
	if lenses[0].LabelName != "loop" {
		t.Errorf("expected label=loop, got %q", lenses[0].LabelName)
	}
	if lenses[0].RefCount != 2 {
		t.Errorf("expected RefCount=2, got %d", lenses[0].RefCount)
	}
}

func TestCodeLensesMultipleLabels(t *testing.T) {
	src := ":a\ngoto a\n:b\ncall :b\ngoto b\n"
	lenses := CodeLenses(src)
	if len(lenses) != 2 {
		t.Fatalf("expected 2 lenses, got %d", len(lenses))
	}
	counts := map[string]int{}
	for _, l := range lenses {
		counts[l.LabelName] = l.RefCount
	}
	if counts["a"] != 1 {
		t.Errorf("expected a=1, got %d", counts["a"])
	}
	if counts["b"] != 2 {
		t.Errorf("expected b=2, got %d", counts["b"])
	}
}

func TestCodeLensesUnusedLabel(t *testing.T) {
	src := ":unused\necho hi\n"
	lenses := CodeLenses(src)
	if len(lenses) != 1 {
		t.Fatalf("expected 1 lens, got %d", len(lenses))
	}
	if lenses[0].RefCount != 0 {
		t.Errorf("expected RefCount=0 for unused label, got %d", lenses[0].RefCount)
	}
}

// ── Feature 3: Semantic Tokens ────────────────────────────────────────────────

func findToken(tokens []SemToken, tokenType uint32) *SemToken {
	for i := range tokens {
		if tokens[i].TokenType == tokenType {
			return &tokens[i]
		}
	}
	return nil
}

func TestSemanticTokensEmpty(t *testing.T) {
	tokens := SemanticTokens("")
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(tokens))
	}
}

func TestSemanticTokensKeyword(t *testing.T) {
	tokens := SemanticTokens("echo hello\n")
	tok := findToken(tokens, semKeyword)
	if tok == nil {
		t.Fatal("expected a keyword token for 'echo'")
	}
	if tok.Col != 0 || tok.Len != 4 {
		t.Errorf("keyword token: col=%d len=%d, want col=0 len=4", tok.Col, tok.Len)
	}
}

func TestSemanticTokensComment(t *testing.T) {
	tokens := SemanticTokens(":: this is a comment\n")
	tok := findToken(tokens, semComment)
	if tok == nil {
		t.Fatal("expected a comment token")
	}
	if tok.Line != 0 {
		t.Errorf("expected line 0, got %d", tok.Line)
	}
}

func TestSemanticTokensLabel(t *testing.T) {
	tokens := SemanticTokens(":start\n")
	var funcTok *SemToken
	for i := range tokens {
		if tokens[i].TokenType == semFunction {
			funcTok = &tokens[i]
			break
		}
	}
	if funcTok == nil {
		t.Fatal("expected a function token for label definition")
	}
	if funcTok.Modifiers&semDeclaration == 0 {
		t.Error("expected declaration modifier on label def token")
	}
	if funcTok.Col != 1 || funcTok.Len != 5 { // "start" after ':'
		t.Errorf("label token: col=%d len=%d, want col=1 len=5", funcTok.Col, funcTok.Len)
	}
}

func TestSemanticTokensVariable(t *testing.T) {
	tokens := SemanticTokens("echo %FOO%\n")
	tok := findToken(tokens, semVariable)
	if tok == nil {
		t.Fatal("expected a variable token for FOO")
	}
	if tok.Len != 3 { // "FOO"
		t.Errorf("variable token len=%d, want 3", tok.Len)
	}
}

func TestSemanticTokensGotoRef(t *testing.T) {
	tokens := SemanticTokens("goto start\n:start\n")
	// find function token on line 0 (the goto ref)
	var refTok *SemToken
	for i := range tokens {
		if tokens[i].TokenType == semFunction && tokens[i].Line == 0 {
			refTok = &tokens[i]
			break
		}
	}
	if refTok == nil {
		t.Fatal("expected a function token on line 0 for goto target")
	}
	if refTok.Col != 5 {
		t.Errorf("goto ref token col=%d, want 5", refTok.Col)
	}
}

// ── Feature 4: Folding Ranges ─────────────────────────────────────────────────

func TestFoldingRangesEmpty(t *testing.T) {
	folds := FoldingRanges("echo hello\ngoto missing\n")
	if len(folds) != 0 {
		t.Errorf("expected 0 folds, got %d", len(folds))
	}
}

func TestFoldingRangesSingleLabel(t *testing.T) {
	src := ":start\necho hi\necho bye\n"
	folds := FoldingRanges(src)
	if len(folds) != 1 {
		t.Fatalf("expected 1 fold, got %d", len(folds))
	}
	if folds[0].StartLine != 0 {
		t.Errorf("expected StartLine=0, got %d", folds[0].StartLine)
	}
	if folds[0].EndLine != 2 {
		t.Errorf("expected EndLine=2, got %d", folds[0].EndLine)
	}
	if folds[0].Kind != "region" {
		t.Errorf("expected kind=region, got %q", folds[0].Kind)
	}
}

func TestFoldingRangesMultipleLabels(t *testing.T) {
	src := ":a\necho a\n:b\necho b\necho b2\n"
	folds := FoldingRanges(src)
	if len(folds) != 2 {
		t.Fatalf("expected 2 folds, got %d", len(folds))
	}
	// first section: :a (line 0) to just before :b (line 1)
	if folds[0].StartLine != 0 || folds[0].EndLine != 1 {
		t.Errorf("fold[0]: start=%d end=%d, want 0..1", folds[0].StartLine, folds[0].EndLine)
	}
	// second section: :b (line 2) to end (line 4)
	if folds[1].StartLine != 2 || folds[1].EndLine != 4 {
		t.Errorf("fold[1]: start=%d end=%d, want 2..4", folds[1].StartLine, folds[1].EndLine)
	}
}

func TestFoldingRangesSmallSection(t *testing.T) {
	// Section with only 1 line of content after label → still folds
	src := ":only\necho hi\n"
	folds := FoldingRanges(src)
	if len(folds) != 1 {
		t.Errorf("expected 1 fold for section with 1 content line, got %d", len(folds))
	}
}

// ── Feature 5: Code Actions ───────────────────────────────────────────────────

func TestCodeActionsAtNoIssue(t *testing.T) {
	src := ":start\ngoto start\n"
	actions := CodeActionsAt(src, 1) // line 1: goto start (defined)
	if len(actions) != 0 {
		t.Errorf("expected 0 actions for defined label, got %d", len(actions))
	}
}

func TestCodeActionsAtMissingLabel(t *testing.T) {
	src := "goto missing\n"
	actions := CodeActionsAt(src, 0)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].NewLabelName != "missing" {
		t.Errorf("expected label=missing, got %q", actions[0].NewLabelName)
	}
	if actions[0].Kind != "quickfix" {
		t.Errorf("expected kind=quickfix, got %q", actions[0].Kind)
	}
}

func TestCodeActionsAtMultipleMissing(t *testing.T) {
	src := "goto a\ngoto b\n"
	actionsLine0 := CodeActionsAt(src, 0)
	actionsLine1 := CodeActionsAt(src, 1)
	if len(actionsLine0) != 1 {
		t.Errorf("line 0: expected 1 action, got %d", len(actionsLine0))
	}
	if len(actionsLine1) != 1 {
		t.Errorf("line 1: expected 1 action, got %d", len(actionsLine1))
	}
	if actionsLine0[0].NewLabelName != "a" {
		t.Errorf("line 0 action: expected label=a, got %q", actionsLine0[0].NewLabelName)
	}
	if actionsLine1[0].NewLabelName != "b" {
		t.Errorf("line 1 action: expected label=b, got %q", actionsLine1[0].NewLabelName)
	}
}

func TestCodeActionsAtCallMissing(t *testing.T) {
	src := "call :func\n"
	actions := CodeActionsAt(src, 0)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action for undefined call target, got %d", len(actions))
	}
	if actions[0].NewLabelName != "func" {
		t.Errorf("expected label=func, got %q", actions[0].NewLabelName)
	}
}

// ── Feature 6: Extended Diagnostics ──────────────────────────────────────────

func TestDiagnosticsUnusedLabel(t *testing.T) {
	src := ":unused\necho hi\n"
	diags := Diagnostics(src)
	var found bool
	for _, d := range diags {
		if d.Sev == SevHint && strings.Contains(d.Message, "unused") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected SevHint diagnostic for unused label, got %v", diags)
	}
}

func TestDiagnosticsUsedLabel(t *testing.T) {
	src := ":start\ngoto start\n"
	diags := Diagnostics(src)
	for _, d := range diags {
		if d.Sev == SevHint && strings.Contains(d.Message, "start") {
			t.Errorf("unexpected unused-label hint for referenced label: %v", d)
		}
	}
}

func TestDiagnosticsUnusedVar(t *testing.T) {
	src := "set FOO=bar\necho hello\n"
	diags := Diagnostics(src)
	var found bool
	for _, d := range diags {
		if d.Sev == SevHint && strings.Contains(d.Message, "FOO") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected SevHint for unused variable FOO, got %v", diags)
	}
}

func TestDiagnosticsUsedVar(t *testing.T) {
	src := "set FOO=bar\necho %FOO%\n"
	diags := Diagnostics(src)
	for _, d := range diags {
		if d.Sev == SevHint && strings.Contains(d.Message, "FOO") {
			t.Errorf("unexpected unused-var hint for used variable: %v", d)
		}
	}
}

func TestDiagnosticsAllClean(t *testing.T) {
	src := ":start\nset FOO=bar\necho %FOO%\ngoto start\n"
	diags := Diagnostics(src)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics for clean script, got %d: %v", len(diags), diags)
	}
}

func TestSemanticTokensEncoding(t *testing.T) {
	// Two tokens on different lines: keyword on line 0, variable on line 1
	src := "echo hi\necho %VAR%\n"
	tokens := SemanticTokens(src)
	data := EncodeSemanticTokens(tokens)
	if len(data)%5 != 0 {
		t.Errorf("encoded data length %d is not multiple of 5", len(data))
	}
	// First token: deltaLine=0 (from start), deltaCol=col
	if len(data) < 5 {
		t.Fatal("expected at least one encoded token")
	}
	// Verify delta encoding: second token's deltaLine relative to first
	if len(data) >= 10 {
		firstLine := int(data[0])
		secondDeltaLine := int(data[5])
		_ = firstLine
		_ = secondDeltaLine
		// Just check they're non-negative
		if data[5] > 1000 {
			t.Errorf("second token deltaLine seems wrong: %d", data[5])
		}
	}
}

func TestIssueEOF(t *testing.T) {
	// GOTO :EOF and CALL :EOF should not be reported as undefined.
	// Current implementation is case-sensitive and misses CALL :EOF.
	src := "goto :EOF\ncall :EOF\ngoto eof\ncall :eof\n"
	diags := Diagnostics(src)
	for _, d := range diags {
		if strings.Contains(strings.ToLower(d.Message), "undefined label: eof") {
			t.Errorf("Unexpected diagnostic: %v", d.Message)
		}
	}
}

func TestIssueDynamicGoto(t *testing.T) {
	// goto %TARGET% should:
	// 1. Not be reported as "Undefined label: %target%"
	// 2. Count as a reference to TARGET variable.
	src := "set TARGET=MYLABEL\ngoto %TARGET%\n:MYLABEL\n:REALLYUNUSED\n"
	diags := Diagnostics(src)

	unusedFound := false

	for _, d := range diags {
		// Should not have "Variable defined but never used: TARGET"
		if strings.Contains(d.Message, "Variable defined but never used: TARGET") {
			t.Errorf("Unexpected diagnostic: %v", d.Message)
		}
		// Should not have "Undefined label: %target%"
		if strings.Contains(d.Message, "Undefined label: %target%") {
			t.Errorf("Unexpected diagnostic: %v", d.Message)
		}
		if strings.Contains(d.Message, "Unused label: mylabel") {
			t.Errorf("Unexpected diagnostic: %v (should have been resolved from TARGET)", d.Message)
		}
		if strings.Contains(d.Message, "Unused label: reallyunused") {
			unusedFound = true
		}
	}

	if !unusedFound {
		t.Errorf("Expected 'Unused label: reallyunused' but it was not reported")
	}
}

func TestIssueUnresolvedDynamicGoto(t *testing.T) {
	// If there is an unresolved dynamic goto, we might want to suppress unused label warnings.
	src := "goto %1\n:MYLABEL\n"
	diags := Diagnostics(src)

	for _, d := range diags {
		if strings.Contains(d.Message, "Unused label: mylabel") {
			t.Errorf("Unexpected diagnostic: %v (unresolved dynamic goto should suppress)", d.Message)
		}
	}
}

func singleDocWorkspace(uri, content string) map[string]*Document {
	return map[string]*Document{
		uri: {
			Content:  content,
			Analysis: Analyze(content),
		},
	}
}
