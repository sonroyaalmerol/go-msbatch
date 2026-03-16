package lsp

import (
	"fmt"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/lexer"
	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
)

// LabelDef is a :label definition found in the document.
type LabelDef struct {
	Name string
	Line int // 0-based
	Col  int // 0-based column where the label name starts (after ':')
}

// VarDef is a SET variable definition found in the document.
type VarDef struct {
	Name  string
	Value string
	Line  int // 0-based
	Col   int // 0-based column where the variable name starts (after 'set ')
}

// LabelRef is a GOTO or CALL :label reference.
type LabelRef struct {
	Name string
	Line int // 0-based
	Col  int // 0-based start column of the label name in the line
}

// VarRef is a %VARIABLE% usage found in the document.
type VarRef struct {
	Name string
	Line int // 0-based
	Col  int // 0-based start column of the name (the char after the opening %)
}

// Loc is a compact source range returned by DefinitionAt / ReferencesAt.
type Loc struct {
	Line   int // 0-based
	Col    int // 0-based start column
	EndCol int // 0-based exclusive end column (same line)
}

// Analysis holds the full analysis result for one document.
type Analysis struct {
	Labels   []LabelDef
	Vars     []VarDef
	GotoRefs []LabelRef // GOTO label
	CallRefs []LabelRef // CALL :label
	VarRefs  []VarRef   // %VARIABLE% usages
}

// Analyze parses the document content and extracts structural information.
// Position data (line numbers) comes from a text scan so the parser does not
// need to track positions itself.
func Analyze(content string) Analysis {
	var a Analysis
	lines := strings.Split(content, "\n")

	// --- text-based pass: collect positions ---
	for i, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		// Label definition: line starts with ':'
		if strings.HasPrefix(trimmed, ":") && !strings.HasPrefix(trimmed, "::") {
			name := strings.Fields(trimmed[1:])[0]
			if name != "" {
				indent := len(line) - len(strings.TrimLeft(line, " \t"))
				labelCol := indent + 1 // position after the ':'
				a.Labels = append(a.Labels, LabelDef{Name: strings.ToLower(name), Line: i, Col: labelCol})
			}
			continue
		}

		// GOTO: "goto label" or "goto :label"
		if strings.HasPrefix(lower, "goto ") || lower == "goto" {
			target := strings.TrimSpace(trimmed[4:])
			target = strings.TrimPrefix(target, ":")
			if target != "" && target != "eof" && target != ":eof" {
				col := labelColAfterKeyword(line, 4) // "goto" = 4 chars
				a.GotoRefs = append(a.GotoRefs, LabelRef{Name: strings.ToLower(target), Line: i, Col: col})
			}
			continue
		}

		// CALL :label (subroutine call — plain CALL <file> is not a label ref)
		if strings.HasPrefix(lower, "call :") {
			rest := strings.TrimSpace(trimmed[5:]) // after "call "
			fields := strings.Fields(rest)
			if len(fields) > 0 {
				name := strings.TrimPrefix(fields[0], ":")
				if name != "" {
					col := labelColAfterKeyword(line, 4) // "call" = 4 chars
					a.CallRefs = append(a.CallRefs, LabelRef{Name: strings.ToLower(name), Line: i, Col: col})
				}
			}
			continue
		}

		// SET: "set varname=value" or "set /a ..." or "set /p ..."
		if strings.HasPrefix(lower, "set ") {
			rest := strings.TrimSpace(trimmed[3:])
			if !strings.HasPrefix(strings.ToLower(rest), "/a") &&
				!strings.HasPrefix(strings.ToLower(rest), "/p") {
				if idx := strings.IndexByte(rest, '='); idx > 0 {
					name := rest[:idx]
					value := rest[idx+1:]
					// compute col: find where the varname starts in the original line
					indent := len(line) - len(strings.TrimLeft(line, " \t"))
					afterSet := line[indent+3:] // after "set"
					trimmedAfterSet := strings.TrimLeft(afterSet, " \t")
					varCol := indent + 3 + (len(afterSet) - len(trimmedAfterSet))
					a.Vars = append(a.Vars, VarDef{
						Name:  strings.ToUpper(name),
						Value: value,
						Line:  i,
						Col:   varCol,
					})
				}
			}
		}

		// %VARIABLE% usages on this line.
		a.VarRefs = appendVarRefs(a.VarRefs, line, i)
	}

	return a
}

// Diagnostics returns a list of issues found in the document.
func Diagnostics(content string) []Diag {
	a := Analyze(content)

	// Build a set of defined label names.
	defined := make(map[string]bool, len(a.Labels))
	for _, l := range a.Labels {
		defined[l.Name] = true
	}

	var diags []Diag

	for _, ref := range a.GotoRefs {
		if !defined[ref.Name] {
			diags = append(diags, Diag{
				Line:    ref.Line,
				Message: "Undefined label: " + ref.Name,
				Sev:     SevWarning,
			})
		}
	}
	for _, ref := range a.CallRefs {
		if !defined[ref.Name] {
			diags = append(diags, Diag{
				Line:    ref.Line,
				Message: "Undefined label: " + ref.Name,
				Sev:     SevWarning,
			})
		}
	}

	return diags
}

// DiagSeverity mirrors LSP DiagnosticSeverity values.
type DiagSeverity int

const (
	SevError   DiagSeverity = 1
	SevWarning DiagSeverity = 2
	SevInfo    DiagSeverity = 3
	SevHint    DiagSeverity = 4
)

// Diag is a language-agnostic diagnostic (converted to LSP types in server.go).
type Diag struct {
	Line    int // 0-based
	Col     int // 0-based start column
	EndCol  int // 0-based end column (0 means end of line)
	Message string
	Sev     DiagSeverity
}

// WordAtPosition returns the word under the cursor on the given line.
func WordAtPosition(line string, col int) string {
	if col > len(line) {
		col = len(line)
	}
	start := col
	for start > 0 && isWordChar(rune(line[start-1])) {
		start--
	}
	end := col
	for end < len(line) && isWordChar(rune(line[end])) {
		end++
	}
	return line[start:end]
}

func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.'
}

// CompletionContext describes what kind of completion is appropriate at the
// cursor position.
type CompletionContext int

const (
	CompleteCommand  CompletionContext = iota // start of line / after pipe/& etc.
	CompleteLabel                             // after GOTO or CALL :
	CompleteVariable                          // inside %...
	CompleteFile                              // generic path argument
)

// CompletionContextAt determines the completion context from the text up to
// the cursor.
func CompletionContextAt(lineBefore string) CompletionContext {
	trimmed := strings.TrimLeft(lineBefore, " \t@")
	lower := strings.ToLower(trimmed)

	// Inside a %variable% reference: an odd number of '%' signs means the
	// last '%' is an opening delimiter that has not yet been closed.
	if strings.Count(lineBefore, "%")%2 == 1 {
		return CompleteVariable
	}

	// After GOTO or inside CALL :
	if strings.HasPrefix(lower, "goto ") || lower == "goto" {
		return CompleteLabel
	}
	if strings.HasPrefix(lower, "call :") || lower == "call :" {
		return CompleteLabel
	}

	// If we are on the first word (no space yet) → command completion.
	if !strings.Contains(strings.TrimLeft(lower, " \t@"), " ") {
		return CompleteCommand
	}

	return CompleteFile
}

// ── position helpers ──────────────────────────────────────────────────────────

// labelColAfterKeyword returns the 0-based column where the label name begins
// after a keyword of keyLen chars (e.g. "goto"=4, "call"=4).
// It accounts for leading whitespace/tabs in the line and any spaces or ':'
// between the keyword and the name.
func labelColAfterKeyword(line string, keyLen int) int {
	indent := len(line) - len(strings.TrimLeft(line, " \t"))
	after := line[indent+keyLen:] // text after the keyword in the original line
	name := strings.TrimLeft(after, " \t:")
	return indent + keyLen + (len(after) - len(name))
}

// appendVarRefs scans line for %NAME% patterns and appends a VarRef for each.
// %% (escaped percent) and positional args %0-%9 are skipped.
func appendVarRefs(refs []VarRef, line string, lineIdx int) []VarRef {
	rest := line
	offset := 0
	for {
		pct := strings.Index(rest, "%")
		if pct < 0 {
			break
		}
		after := rest[pct+1:]
		end := strings.Index(after, "%")
		if end < 0 {
			break
		}
		name := after[:end]
		// Skip empty (%%→escaped %), positional args (%0–%9), and FOR vars (%%I)
		if name != "" && (name[0] < '0' || name[0] > '9') {
			refs = append(refs, VarRef{
				Name: strings.ToUpper(name),
				Line: lineIdx,
				Col:  offset + pct + 1, // col of first char of name (after %)
			})
		}
		advance := pct + 1 + end + 1 // past closing %
		offset += advance
		rest = rest[advance:]
	}
	return refs
}

// ── DefinitionAt / ReferencesAt ───────────────────────────────────────────────

// DefinitionAt returns the definition location for the symbol under (line, col),
// or the zero Loc and false if nothing was found. Lines and cols are 0-based.
func DefinitionAt(content string, line, col int) (Loc, bool) {
	src := lineAt(content, line)
	a := Analyze(content)

	// Prefer variable context when cursor is inside %...%
	lineBefore := src
	if col <= len(src) {
		lineBefore = src[:col]
	}
	if CompletionContextAt(lineBefore) == CompleteVariable {
		word := strings.ToUpper(WordAtPosition(src, col))
		for _, v := range a.Vars {
			if v.Name == word {
				return Loc{Line: v.Line, Col: 0, EndCol: len(lineAt(content, v.Line))}, true
			}
		}
		return Loc{}, false
	}

	// Label context
	word := strings.ToLower(WordAtPosition(src, col))
	if word == "" {
		return Loc{}, false
	}
	for _, lbl := range a.Labels {
		if lbl.Name == word {
			return Loc{Line: lbl.Line, Col: 0, EndCol: len(lineAt(content, lbl.Line))}, true
		}
	}
	return Loc{}, false
}

// ReferencesAt returns all reference locations for the symbol under (line, col).
// When includeDecl is true the definition site is included in the results.
func ReferencesAt(content string, line, col int, includeDecl bool) []Loc {
	src := lineAt(content, line)
	a := Analyze(content)

	lineBefore := src
	if col <= len(src) {
		lineBefore = src[:col]
	}

	// Variable references
	if CompletionContextAt(lineBefore) == CompleteVariable {
		name := strings.ToUpper(WordAtPosition(src, col))
		if name == "" {
			return nil
		}
		var locs []Loc
		for _, ref := range a.VarRefs {
			if ref.Name == name {
				locs = append(locs, Loc{Line: ref.Line, Col: ref.Col, EndCol: ref.Col + len(ref.Name)})
			}
		}
		if includeDecl {
			for _, v := range a.Vars {
				if v.Name == name {
					locs = append(locs, Loc{Line: v.Line, Col: 0, EndCol: len(lineAt(content, v.Line))})
				}
			}
		}
		return locs
	}

	// Label references
	name := strings.ToLower(WordAtPosition(src, col))
	if name == "" {
		return nil
	}
	// Confirm it's a known label
	known := false
	for _, lbl := range a.Labels {
		if lbl.Name == name {
			known = true
			break
		}
	}
	if !known {
		return nil
	}
	var locs []Loc
	for _, ref := range a.GotoRefs {
		if ref.Name == name {
			locs = append(locs, Loc{Line: ref.Line, Col: ref.Col, EndCol: ref.Col + len(ref.Name)})
		}
	}
	for _, ref := range a.CallRefs {
		if ref.Name == name {
			locs = append(locs, Loc{Line: ref.Line, Col: ref.Col, EndCol: ref.Col + len(ref.Name)})
		}
	}
	if includeDecl {
		for _, lbl := range a.Labels {
			if lbl.Name == name {
				locs = append(locs, Loc{Line: lbl.Line, Col: 0, EndCol: len(lineAt(content, lbl.Line))})
			}
		}
	}
	return locs
}

// ── Code Lens ─────────────────────────────────────────────────────────────────

// CodeLensData holds data for a single code lens annotation on a label definition.
type CodeLensData struct {
	Line      int    // line of the :label definition
	LabelName string
	RefCount  int // total GOTO + CALL refs
}

// CodeLenses returns one CodeLensData per label in the document.
func CodeLenses(content string) []CodeLensData {
	a := Analyze(content)
	// count refs per label name
	refCounts := make(map[string]int, len(a.Labels))
	for _, ref := range a.GotoRefs {
		refCounts[ref.Name]++
	}
	for _, ref := range a.CallRefs {
		refCounts[ref.Name]++
	}
	lenses := make([]CodeLensData, 0, len(a.Labels))
	for _, lbl := range a.Labels {
		lenses = append(lenses, CodeLensData{
			Line:      lbl.Line,
			LabelName: lbl.Name,
			RefCount:  refCounts[lbl.Name],
		})
	}
	return lenses
}

// ── Rename ────────────────────────────────────────────────────────────────────

// TextEdit represents a single text replacement in the document.
type TextEdit struct {
	Line    int
	Col     int
	EndCol  int
	NewText string
}

// wordRangeAt returns the start and end columns of the word at col in line.
func wordRangeAt(line string, col int) (start, end int) {
	if col > len(line) {
		col = len(line)
	}
	start = col
	for start > 0 && isWordChar(rune(line[start-1])) {
		start--
	}
	end = col
	for end < len(line) && isWordChar(rune(line[end])) {
		end++
	}
	return start, end
}

// RenameAt returns all text edits required to rename the symbol at (line, col)
// to newName. Returns an error if there is no renameable symbol at the cursor.
func RenameAt(content string, line, col int, newName string) ([]TextEdit, error) {
	src := lineAt(content, line)
	a := Analyze(content)

	lineBefore := src
	if col <= len(src) {
		lineBefore = src[:col]
	}

	// Variable context
	if CompletionContextAt(lineBefore) == CompleteVariable {
		word := strings.ToUpper(WordAtPosition(src, col))
		if word == "" {
			return nil, fmt.Errorf("no renameable symbol at cursor")
		}
		var edits []TextEdit
		// Rename definition site
		for _, v := range a.Vars {
			if v.Name == word {
				edits = append(edits, TextEdit{
					Line:    v.Line,
					Col:     v.Col,
					EndCol:  v.Col + len(v.Name),
					NewText: strings.ToUpper(newName),
				})
			}
		}
		// Rename all usage sites
		for _, ref := range a.VarRefs {
			if ref.Name == word {
				edits = append(edits, TextEdit{
					Line:    ref.Line,
					Col:     ref.Col,
					EndCol:  ref.Col + len(ref.Name),
					NewText: strings.ToUpper(newName),
				})
			}
		}
		if len(edits) == 0 {
			return nil, fmt.Errorf("no renameable symbol at cursor")
		}
		return edits, nil
	}

	// Label context
	word := strings.ToLower(WordAtPosition(src, col))
	if word == "" {
		return nil, fmt.Errorf("no renameable symbol at cursor")
	}
	var edits []TextEdit
	found := false
	for _, lbl := range a.Labels {
		if lbl.Name == word {
			found = true
			edits = append(edits, TextEdit{
				Line:    lbl.Line,
				Col:     lbl.Col,
				EndCol:  lbl.Col + len(lbl.Name),
				NewText: strings.ToLower(newName),
			})
		}
	}
	for _, ref := range a.GotoRefs {
		if ref.Name == word {
			found = true
			edits = append(edits, TextEdit{
				Line:    ref.Line,
				Col:     ref.Col,
				EndCol:  ref.Col + len(ref.Name),
				NewText: strings.ToLower(newName),
			})
		}
	}
	for _, ref := range a.CallRefs {
		if ref.Name == word {
			found = true
			edits = append(edits, TextEdit{
				Line:    ref.Line,
				Col:     ref.Col,
				EndCol:  ref.Col + len(ref.Name),
				NewText: strings.ToLower(newName),
			})
		}
	}
	if !found {
		return nil, fmt.Errorf("no renameable symbol at cursor")
	}
	return edits, nil
}

// PrepareRenameAt returns the range of the symbol under cursor if renameable.
// Returns Loc{} and false if there is nothing renameable at the cursor.
func PrepareRenameAt(content string, line, col int) (Loc, bool) {
	src := lineAt(content, line)
	a := Analyze(content)

	lineBefore := src
	if col <= len(src) {
		lineBefore = src[:col]
	}

	if CompletionContextAt(lineBefore) == CompleteVariable {
		word := strings.ToUpper(WordAtPosition(src, col))
		if word == "" {
			return Loc{}, false
		}
		for _, v := range a.Vars {
			if v.Name == word {
				start, end := wordRangeAt(src, col)
				return Loc{Line: line, Col: start, EndCol: end}, true
			}
		}
		// also check if it's used (even if not defined)
		for _, ref := range a.VarRefs {
			if ref.Name == word && ref.Line == line {
				start, end := wordRangeAt(src, col)
				return Loc{Line: line, Col: start, EndCol: end}, true
			}
		}
		return Loc{}, false
	}

	word := strings.ToLower(WordAtPosition(src, col))
	if word == "" {
		return Loc{}, false
	}
	for _, lbl := range a.Labels {
		if lbl.Name == word {
			start, end := wordRangeAt(src, col)
			return Loc{Line: line, Col: start, EndCol: end}, true
		}
	}
	// also allow renaming from a goto/call ref site if label is known
	for _, ref := range a.GotoRefs {
		if ref.Name == word && ref.Line == line {
			for _, lbl := range a.Labels {
				if lbl.Name == word {
					start, end := wordRangeAt(src, col)
					return Loc{Line: line, Col: start, EndCol: end}, true
				}
			}
		}
	}
	return Loc{}, false
}

// parseNodes is a thin wrapper to lex+parse a content string.
func parseNodes(content string) []parser.Node {
	bl := lexer.New(content)
	pr := parser.New(bl)
	return pr.Parse()
}

// collectLabels walks the AST and collects all LabelNode names (lower-cased).
// Used as a cross-check alongside the text-based scan.
func collectLabelsFromAST(nodes []parser.Node) []string {
	var out []string
	var walk func([]parser.Node)
	walk = func(ns []parser.Node) {
		for _, n := range ns {
			switch v := n.(type) {
			case *parser.LabelNode:
				out = append(out, strings.ToLower(v.Name))
			case *parser.Block:
				walk(v.Body)
			case *parser.IfNode:
				if v.Then != nil {
					walk([]parser.Node{v.Then})
				}
				if v.Else != nil {
					walk([]parser.Node{v.Else})
				}
			case *parser.ForNode:
				if v.Do != nil {
					walk([]parser.Node{v.Do})
				}
			case *parser.BinaryNode:
				walk([]parser.Node{v.Left, v.Right})
			case *parser.PipeNode:
				walk([]parser.Node{v.Left, v.Right})
			}
		}
	}
	walk(nodes)
	return out
}
