package lsp

import (
	"fmt"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/executor"
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
	URI    string
	Line   int // 0-based
	Col    int // 0-based start column
	EndCol int // 0-based exclusive end column (same line)
}

// Document represents a single loaded file and its cached analysis.
type Document struct {
	Content  string
	Analysis Analysis
}

// FileRef is a CALL <file> reference.
type FileRef struct {
	Path string // The literal string typed after "CALL "
	Line int    // 0-based
	Col  int    // 0-based start column of the file path in the line
}

// Analysis holds the full analysis result for one document.
type Analysis struct {
	Labels          []LabelDef
	Vars            []VarDef
	GotoRefs        []LabelRef // GOTO label
	CallRefs        []LabelRef // CALL :label
	FileRefs        []FileRef  // CALL file.bat
	VarRefs         []VarRef   // %VARIABLE% usages
	HasDynamicJumps bool
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
			targetLower := strings.ToLower(target)
			if target != "" && targetLower != "eof" {
				col := labelColAfterKeyword(line, 4) // "goto" = 4 chars
				a.GotoRefs = append(a.GotoRefs, LabelRef{Name: targetLower, Line: i, Col: col})
				if strings.Contains(target, "%") {
					a.HasDynamicJumps = true
				}
			}
			// Don't continue, scan for %VAR% in the same line
		} else if strings.HasPrefix(lower, "call ") {
			// CALL <something>
			rest := strings.TrimSpace(trimmed[5:]) // after "call "
			if strings.HasPrefix(rest, ":") {
				// Subroutine call
				fields := strings.Fields(rest)
				if len(fields) > 0 {
					name := strings.TrimPrefix(fields[0], ":")
					nameLower := strings.ToLower(name)
					if name != "" && nameLower != "eof" {
						col := labelColAfterKeyword(line, 4) // "call" = 4 chars
						a.CallRefs = append(a.CallRefs, LabelRef{Name: nameLower, Line: i, Col: col})
						if strings.Contains(name, "%") {
							a.HasDynamicJumps = true
						}
					}
				}
			} else {
				// File call or external command
				fields := strings.Fields(rest)
				if len(fields) > 0 {
					path := fields[0]
					if strings.HasSuffix(strings.ToLower(path), ".bat") || strings.HasSuffix(strings.ToLower(path), ".cmd") {
						col := labelColAfterKeyword(line, 4)
						a.FileRefs = append(a.FileRefs, FileRef{Path: path, Line: i, Col: col})
					}
				}
			}
			// Don't continue, scan for %VAR% in the same line
		} else if strings.HasPrefix(lower, "set ") {
			// SET: "set varname=value" or "set /a ..." or "set /p ..."
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

	// Pre-collect variable values for dynamic jump resolution
	varValues := make(map[string][]string)
	for _, v := range a.Vars {
		varValues[v.Name] = append(varValues[v.Name], v.Value)
	}

	var diags []Diag

	for _, ref := range a.GotoRefs {
		if strings.Contains(ref.Name, "%") {
			continue // Suppress undefined label warning for dynamic targets
		}
		if !defined[ref.Name] {
			diags = append(diags, Diag{
				Line:    ref.Line,
				Message: "Undefined label: " + ref.Name,
				Sev:     SevWarning,
			})
		}
	}
	for _, ref := range a.CallRefs {
		if strings.Contains(ref.Name, "%") {
			continue // Suppress undefined label warning for dynamic targets
		}
		if !defined[ref.Name] {
			diags = append(diags, Diag{
				Line:    ref.Line,
				Message: "Undefined label: " + ref.Name,
				Sev:     SevWarning,
			})
		}
	}

	// Unused labels: defined but never referenced by any GOTO or CALL.
	refCounts := make(map[string]int, len(a.Labels))
	hasUnresolvedJumps := false

	resolveDynamic := func(name string) {
		if !strings.Contains(name, "%") {
			refCounts[name]++
			return
		}

		// Very basic resolution: if name is exactly "%varname%"
		if strings.HasPrefix(name, "%") && strings.HasSuffix(name, "%") && strings.Count(name, "%") == 2 {
			varName := strings.ToUpper(name[1 : len(name)-1])
			if vals, ok := varValues[varName]; ok {
				for _, v := range vals {
					refCounts[strings.ToLower(strings.TrimSpace(v))]++
				}
			} else {
				hasUnresolvedJumps = true
			}
		} else {
			hasUnresolvedJumps = true
		}
	}

	for _, ref := range a.GotoRefs {
		resolveDynamic(ref.Name)
	}
	for _, ref := range a.CallRefs {
		resolveDynamic(ref.Name)
	}

	// If the script contains unresolvable dynamic jumps, we suppress unused label warnings
	// because any label might be a target.
	if !hasUnresolvedJumps {
		for _, lbl := range a.Labels {
			if refCounts[lbl.Name] == 0 {
				diags = append(diags, Diag{
					Line:    lbl.Line,
					Col:     lbl.Col,
					EndCol:  lbl.Col + len(lbl.Name),
					Message: "Unused label: " + lbl.Name,
					Sev:     SevHint,
				})
			}
		}
	}

	// Variables defined but never used: SET VAR=... but %VAR% never appears.
	varUsed := make(map[string]bool)
	for _, ref := range a.VarRefs {
		varUsed[ref.Name] = true
	}
	for _, v := range a.Vars {
		if !varUsed[v.Name] {
			diags = append(diags, Diag{
				Line:    v.Line,
				Col:     v.Col,
				EndCol:  v.Col + len(v.Name),
				Message: "Variable defined but never used: " + v.Name,
				Sev:     SevHint,
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
func DefinitionAt(workspace map[string]*Document, uri string, line, col int) (Loc, bool) {
	doc, ok := workspace[uri]
	if !ok {
		return Loc{}, false
	}
	content := doc.Content
	src := lineAt(content, line)
	a := doc.Analysis

	lineBefore := src
	if col <= len(src) {
		lineBefore = src[:col]
	}

	// Prefer variable context when cursor is inside %...%
	if CompletionContextAt(lineBefore) == CompleteVariable {
		word := strings.ToUpper(WordAtPosition(src, col))

		// Search current document first
		for _, v := range a.Vars {
			if v.Name == word {
				return Loc{URI: uri, Line: v.Line, Col: 0, EndCol: len(lineAt(content, v.Line))}, true
			}
		}

		// Fallback to searching other documents in workspace
		for otherUri, otherDoc := range workspace {
			if otherUri == uri {
				continue
			}
			for _, v := range otherDoc.Analysis.Vars {
				if v.Name == word {
					return Loc{URI: otherUri, Line: v.Line, Col: 0, EndCol: len(lineAt(otherDoc.Content, v.Line))}, true
				}
			}
		}

		return Loc{}, false
	}

	// Check if cursor is on a CALL <file>
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(lineBefore)), "call ") {
		word := WordAtPosition(src, col)
		for _, ref := range a.FileRefs {
			// Compare path component (rough approximation, could be exact match)
			if strings.Contains(ref.Path, word) {
				// We need to resolve ref.Path to a URI.
				// A simple heuristic is to check if it ends with .bat or .cmd and find it in the workspace.
				lowerPath := strings.ToLower(ref.Path)
				for wUri := range workspace {
					if strings.HasSuffix(strings.ToLower(wUri), lowerPath) {
						return Loc{URI: wUri, Line: 0, Col: 0, EndCol: 0}, true
					}
				}
			}
		}
	}

	// Label context (local to document)
	word := strings.ToLower(WordAtPosition(src, col))
	if word == "" {
		return Loc{}, false
	}
	for _, lbl := range a.Labels {
		if lbl.Name == word {
			return Loc{URI: uri, Line: lbl.Line, Col: 0, EndCol: len(lineAt(content, lbl.Line))}, true
		}
	}
	return Loc{}, false
}

// ReferencesAt returns all reference locations for the symbol under (line, col).
// When includeDecl is true the definition site is included in the results.
func ReferencesAt(workspace map[string]*Document, uri string, line, col int, includeDecl bool) []Loc {
	doc, ok := workspace[uri]
	if !ok {
		return nil
	}
	content := doc.Content
	src := lineAt(content, line)
	a := doc.Analysis

	lineBefore := src
	if col <= len(src) {
		lineBefore = src[:col]
	}

	// Variable references (Workspace-wide)
	if CompletionContextAt(lineBefore) == CompleteVariable {
		name := strings.ToUpper(WordAtPosition(src, col))
		if name == "" {
			return nil
		}
		var locs []Loc
		for wUri, wDoc := range workspace {
			for _, ref := range wDoc.Analysis.VarRefs {
				if ref.Name == name {
					locs = append(locs, Loc{URI: wUri, Line: ref.Line, Col: ref.Col, EndCol: ref.Col + len(ref.Name)})
				}
			}
			if includeDecl {
				for _, v := range wDoc.Analysis.Vars {
					if v.Name == name {
						locs = append(locs, Loc{URI: wUri, Line: v.Line, Col: 0, EndCol: len(lineAt(wDoc.Content, v.Line))})
					}
				}
			}
		}
		return locs
	}

	// Label references (Local to document)
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
			locs = append(locs, Loc{URI: uri, Line: ref.Line, Col: ref.Col, EndCol: ref.Col + len(ref.Name)})
		}
	}
	for _, ref := range a.CallRefs {
		if ref.Name == name {
			locs = append(locs, Loc{URI: uri, Line: ref.Line, Col: ref.Col, EndCol: ref.Col + len(ref.Name)})
		}
	}
	if includeDecl {
		for _, lbl := range a.Labels {
			if lbl.Name == name {
				locs = append(locs, Loc{URI: uri, Line: lbl.Line, Col: 0, EndCol: len(lineAt(content, lbl.Line))})
			}
		}
	}
	return locs
}

// ── Code Actions ─────────────────────────────────────────────────────────────

// CodeActionData describes a code action that can be applied to the document.
type CodeActionData struct {
	Title        string
	Kind         string // "quickfix"
	NewLabelName string // label name for "create missing label" actions
	InsertLine   int    // line at which to insert (end of file)
}

// CodeActionsAt returns code actions available at the given line.
// Currently produces "Create missing label" quick-fix for undefined GOTO/CALL targets.
func CodeActionsAt(content string, line int) []CodeActionData {
	a := Analyze(content)

	defined := make(map[string]bool, len(a.Labels))
	for _, l := range a.Labels {
		defined[l.Name] = true
	}

	lines := strings.Split(content, "\n")
	lastLine := len(lines) - 1
	// find last non-empty line for insert point
	insertLine := lastLine
	for insertLine > 0 && strings.TrimSpace(strings.TrimRight(lines[insertLine], "\r")) == "" {
		insertLine--
	}

	seen := make(map[string]bool)
	var actions []CodeActionData

	for _, ref := range a.GotoRefs {
		if ref.Line == line && !defined[ref.Name] && !seen[ref.Name] {
			seen[ref.Name] = true
			actions = append(actions, CodeActionData{
				Title:        "Create missing label :" + ref.Name,
				Kind:         "quickfix",
				NewLabelName: ref.Name,
				InsertLine:   insertLine,
			})
		}
	}
	for _, ref := range a.CallRefs {
		if ref.Line == line && !defined[ref.Name] && !seen[ref.Name] {
			seen[ref.Name] = true
			actions = append(actions, CodeActionData{
				Title:        "Create missing label :" + ref.Name,
				Kind:         "quickfix",
				NewLabelName: ref.Name,
				InsertLine:   insertLine,
			})
		}
	}

	return actions
}

// ── Folding Ranges ────────────────────────────────────────────────────────────

// FoldRange represents a collapsible region in the document.
type FoldRange struct {
	StartLine int
	EndLine   int
	Kind      string // "region" for label sections
}

// FoldingRanges returns folding ranges for the document.
// Each label section (from :label to just before the next :label or end of file)
// becomes a fold if it has more than 1 line.
func FoldingRanges(content string) []FoldRange {
	lines := strings.Split(content, "\n")
	total := len(lines)

	// Find label definition lines
	var labelLines []int
	for i, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, ":") && !strings.HasPrefix(trimmed, "::") {
			labelLines = append(labelLines, i)
		}
	}

	if len(labelLines) == 0 {
		return nil
	}

	var folds []FoldRange
	for i, start := range labelLines {
		var end int
		if i+1 < len(labelLines) {
			end = labelLines[i+1] - 1
		} else {
			end = total - 1
			// skip trailing empty lines
			for end > start && strings.TrimSpace(strings.TrimRight(lines[end], "\r")) == "" {
				end--
			}
		}
		// Only fold if there is at least 1 line of content after the label line
		if end > start {
			folds = append(folds, FoldRange{
				StartLine: start,
				EndLine:   end,
				Kind:      "region",
			})
		}
	}
	return folds
}

// ── Semantic Tokens ───────────────────────────────────────────────────────────

// SemTokenTypes is the ordered legend sent in initialize.
var SemTokenTypes = []string{"keyword", "variable", "function", "comment", "string", "operator"}

// SemTokenModifiers is the ordered modifier legend.
var SemTokenModifiers = []string{"declaration", "readonly"}

// Token type indices
const (
	semKeyword  = uint32(0)
	semVariable = uint32(1)
	semFunction = uint32(2) // used for labels
	semComment  = uint32(3)
	semString   = uint32(4)
	semOperator = uint32(5)
)

const semDeclaration = uint32(1 << 0) // bitmask

// SemToken represents one semantic token.
type SemToken struct {
	Line      int
	Col       int
	Len       int
	TokenType uint32
	Modifiers uint32
}

// batchKeywords is the set of known batch keyword names for semantic highlighting.
// It is built from the executor registry (all registered command names) plus all
// lexer keywords (the single authoritative list in pkg/lexer/keywords.go).
var batchKeywords = func() map[string]bool {
	m := make(map[string]bool)
	for _, name := range executor.New().Names() {
		m[name] = true
	}
	for _, kw := range lexer.Keywords {
		m[kw] = true
	}
	return m
}()

// SemanticTokens returns semantic tokens for the document, sorted by position.
func SemanticTokens(content string) []SemToken {
	a := Analyze(content)
	lines := strings.Split(content, "\n")

	// Build lookup sets for goto/call refs by (line,col)
	type lineCol struct{ line, col int }
	gotoRefSet := make(map[lineCol]int)
	for _, ref := range a.GotoRefs {
		gotoRefSet[lineCol{ref.Line, ref.Col}] = len(ref.Name)
	}
	callRefSet := make(map[lineCol]int)
	for _, ref := range a.CallRefs {
		callRefSet[lineCol{ref.Line, ref.Col}] = len(ref.Name)
	}

	var tokens []SemToken

	for i, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		indent := len(line) - len(strings.TrimLeft(line, " \t"))

		// Comment line: :: or rem
		if strings.HasPrefix(trimmed, "::") || strings.HasPrefix(lower, "rem ") || lower == "rem" {
			tokens = append(tokens, SemToken{
				Line:      i,
				Col:       indent,
				Len:       len(trimmed),
				TokenType: semComment,
			})
			continue
		}

		// Label definition line: starts with ':'
		if strings.HasPrefix(trimmed, ":") {
			name := trimmed[1:]
			if fields := strings.Fields(name); len(fields) > 0 {
				name = fields[0]
			}
			if name != "" {
				tokens = append(tokens, SemToken{
					Line:      i,
					Col:       indent + 1,
					Len:       len(name),
					TokenType: semFunction,
					Modifiers: semDeclaration,
				})
			}
			continue
		}

		// First word keyword detection
		if trimmed != "" {
			// strip leading '@' from trimmed
			stripped := strings.TrimLeft(trimmed, "@")
			firstWord := strings.ToLower(strings.Fields(stripped)[0])
			if batchKeywords[firstWord] {
				// find actual col: skip indent and '@' chars
				kwCol := indent + (len(trimmed) - len(stripped))
				tokens = append(tokens, SemToken{
					Line:      i,
					Col:       kwCol,
					Len:       len(firstWord),
					TokenType: semKeyword,
				})
			}
		}

		// Scan for goto/call label refs on this line
		for lc, nameLen := range gotoRefSet {
			if lc.line == i {
				tokens = append(tokens, SemToken{
					Line:      i,
					Col:       lc.col,
					Len:       nameLen,
					TokenType: semFunction,
				})
			}
		}
		for lc, nameLen := range callRefSet {
			if lc.line == i {
				tokens = append(tokens, SemToken{
					Line:      i,
					Col:       lc.col,
					Len:       nameLen,
					TokenType: semFunction,
				})
			}
		}

		// Scan for %VAR% occurrences
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
			if name != "" && (name[0] < '0' || name[0] > '9') {
				tokens = append(tokens, SemToken{
					Line:      i,
					Col:       offset + pct + 1,
					Len:       len(name),
					TokenType: semVariable,
				})
			}
			advance := pct + 1 + end + 1
			offset += advance
			rest = rest[advance:]
		}

		// Scan for quoted strings "..."
		inStr := false
		strStart := 0
		for ci, ch := range line {
			if ch == '"' {
				if !inStr {
					inStr = true
					strStart = ci
				} else {
					// end of string
					tokens = append(tokens, SemToken{
						Line:      i,
						Col:       strStart,
						Len:       ci - strStart + 1,
						TokenType: semString,
					})
					inStr = false
				}
			}
		}
	}

	// Sort tokens by (Line, Col)
	for i := 1; i < len(tokens); i++ {
		for j := i; j > 0; j-- {
			a, b := tokens[j-1], tokens[j]
			if a.Line > b.Line || (a.Line == b.Line && a.Col > b.Col) {
				tokens[j-1], tokens[j] = tokens[j], tokens[j-1]
			} else {
				break
			}
		}
	}

	return tokens
}

// EncodeSemanticTokens converts SemToken slice to the LSP flat uint32 format.
func EncodeSemanticTokens(tokens []SemToken) []uint32 {
	data := make([]uint32, 0, len(tokens)*5)
	prevLine, prevCol := 0, 0
	for _, t := range tokens {
		deltaLine := t.Line - prevLine
		deltaCol := t.Col
		if deltaLine == 0 {
			deltaCol = t.Col - prevCol
		}
		data = append(data, uint32(deltaLine), uint32(deltaCol), uint32(t.Len), t.TokenType, t.Modifiers)
		prevLine = t.Line
		prevCol = t.Col
	}
	return data
}

// ── Code Lens ─────────────────────────────────────────────────────────────────

// CodeLensData holds data for a single code lens annotation on a label definition.
type CodeLensData struct {
	Line      int // line of the :label definition
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
func RenameAt(workspace map[string]*Document, uri string, line, col int, newName string) (map[string][]TextEdit, error) {
	doc, ok := workspace[uri]
	if !ok {
		return nil, fmt.Errorf("document not found")
	}
	content := doc.Content
	src := lineAt(content, line)
	a := doc.Analysis

	lineBefore := src
	if col <= len(src) {
		lineBefore = src[:col]
	}

	editsByURI := make(map[string][]TextEdit)

	// Variable context
	if CompletionContextAt(lineBefore) == CompleteVariable {
		word := strings.ToUpper(WordAtPosition(src, col))
		if word == "" {
			return nil, fmt.Errorf("no renameable symbol at cursor")
		}

		for wUri, wDoc := range workspace {
			var edits []TextEdit
			// Rename definition site
			for _, v := range wDoc.Analysis.Vars {
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
			for _, ref := range wDoc.Analysis.VarRefs {
				if ref.Name == word {
					edits = append(edits, TextEdit{
						Line:    ref.Line,
						Col:     ref.Col,
						EndCol:  ref.Col + len(ref.Name),
						NewText: strings.ToUpper(newName),
					})
				}
			}
			if len(edits) > 0 {
				editsByURI[wUri] = edits
			}
		}

		if len(editsByURI) == 0 {
			return nil, fmt.Errorf("no renameable symbol at cursor")
		}
		return editsByURI, nil
	}

	// Label context (Local only)
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

	editsByURI[uri] = edits
	return editsByURI, nil
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
