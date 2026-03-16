package lsp

import (
	"fmt"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/executor"
	"github.com/sonroyaalmerol/go-msbatch/pkg/lexer"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

// LabelDef is a :label definition found in the document.
type LabelDef struct {
	Name string
	Line int // 0-based
	Col  int // 0-based column where the label name starts (after ':')
}

// VarDef is a SET variable definition found in the document.
type VarDef struct {
	Name     string
	Value    string
	Line     int // 0-based definition line
	Col      int // 0-based column of the identifier (after 'set ' or after '%%')
	ScopeEnd int // 0-based last line of scope; -1 means file-wide (SET vars)
	// FOR loop vars (Name starts with "%") have ScopeEnd set to the last line
	// of the DO body so completions are only offered inside the loop.
}

// LabelRef is a GOTO or CALL :label reference.
type LabelRef struct {
	Name string
	Line int // 0-based
	Col  int // 0-based start column of the label name in the line
}

// VarRef is a %VARIABLE% or !VARIABLE! usage found in the document.
type VarRef struct {
	Name      string
	Line      int  // 0-based
	Col       int  // 0-based start column of the name (the char after the opening sigil)
	IsDelayed bool // true when this is a !VAR! delayed-expansion reference
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
	Labels                  []LabelDef
	Vars                    []VarDef
	GotoRefs                []LabelRef // GOTO label
	CallRefs                []LabelRef // CALL :label
	FileRefs                []FileRef  // CALL file.bat
	VarRefs                 []VarRef   // %VARIABLE% and !VARIABLE! usages
	HasDynamicJumps         bool
	DelayedExpansionEnabled bool // true when SETLOCAL ENABLEDELAYEDEXPANSION is present
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

		// Skip comment lines: no labels, vars, or var-refs live inside comments.
		stripped := strings.TrimLeft(trimmed, "@")
		strippedLower := strings.ToLower(stripped)
		if strings.HasPrefix(trimmed, "::") || strings.HasPrefix(strippedLower, "rem ") || strippedLower == "rem" {
			continue
		}

		// SETLOCAL ENABLEDELAYEDEXPANSION detection
		if strings.HasPrefix(strippedLower, "setlocal") {
			rest := strings.TrimSpace(strippedLower[8:])
			if strings.Contains(rest, "enabledelayedexpansion") {
				a.DelayedExpansionEnabled = true
			}
		}

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
			restLower := strings.ToLower(rest)
			indent := len(line) - len(strings.TrimLeft(line, " \t"))
			afterSet := line[indent+3:] // after "set"
			trimmedAfterSet := strings.TrimLeft(afterSet, " \t")
			baseCol := indent + 3 + (len(afterSet) - len(trimmedAfterSet))
			if strings.HasPrefix(restLower, "/a") {
				// SET /A: arithmetic — extract all assigned variable names.
				expr := strings.TrimSpace(rest[2:])
				for _, name := range extractSetAVars(expr) {
					a.Vars = append(a.Vars, VarDef{
						Name:     name,
						Value:    "",
						Line:     i,
						Col:      baseCol,
						ScopeEnd: -1,
					})
				}
			} else if strings.HasPrefix(restLower, "/p") {
				// SET /P: prompt — variable name before '='.
				promptPart := strings.TrimSpace(rest[2:])
				if idx := strings.IndexByte(promptPart, '='); idx > 0 {
					a.Vars = append(a.Vars, VarDef{
						Name:     strings.ToUpper(promptPart[:idx]),
						Value:    "",
						Line:     i,
						Col:      baseCol,
						ScopeEnd: -1,
					})
				}
			} else {
				if idx := strings.IndexByte(rest, '='); idx > 0 {
					name := rest[:idx]
					value := rest[idx+1:]
					a.Vars = append(a.Vars, VarDef{
						Name:     strings.ToUpper(name),
						Value:    value,
						Line:     i,
						Col:      baseCol,
						ScopeEnd: -1,
					})
				}
			}
		} else if strings.HasPrefix(lower, "for ") {
			// Find the variable like %%I
			rest := trimmed[4:]
			idx := strings.Index(rest, "%%")
			if idx >= 0 && idx+2 < len(rest) {
				char := rest[idx+2]
				// FOR loop variables must be letters; %%0-%%9 are escaped positional args.
				if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
					indent := len(line) - len(strings.TrimLeft(line, " \t"))
					// +2 skips the %% sigil so Col points to the letter, consistent
					// with how %FOO% vars store Col at the first letter after %.
					varCol := indent + 4 + idx + 2
					scopeEnd := forScopeEnd(lines, i)
					a.Vars = append(a.Vars, VarDef{
						Name:     "%" + strings.ToUpper(string(char)),
						Value:    "",
						Line:     i,
						Col:      varCol,
						ScopeEnd: scopeEnd,
					})
					// FOR /F with tokens= can capture multiple values into
					// successive letters. Add implicit VarDefs for each.
					if tokenSpec := extractForFTokensSpec(trimmed); tokenSpec != "" {
						nTokens := countForFTokens(tokenSpec)
						for k := 1; k < nTokens; k++ {
							nextChar := char + byte(k)
							if !((nextChar >= 'a' && nextChar <= 'z') || (nextChar >= 'A' && nextChar <= 'Z')) {
								break
							}
							a.Vars = append(a.Vars, VarDef{
								Name:     "%" + strings.ToUpper(string(nextChar)),
								Value:    "",
								Line:     i,
								Col:      varCol,
								ScopeEnd: scopeEnd,
							})
						}
					}
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

	// Variables defined but never used: SET VAR=... but %VAR% / !VAR! never appears.
	varUsed := make(map[string]bool)
	for _, ref := range a.VarRefs {
		varUsed[ref.Name] = true
	}
	// Delayed refs reference the same variable as their SET counterpart.
	// varUsed already contains the name (no sigil) for delayed refs since
	// VarRef.Name is always just the identifier (e.g. "MYVAR").
	for _, v := range a.Vars {
		if !varUsed[v.Name] {
			endCol := v.Col + len(v.Name)
			if strings.HasPrefix(v.Name, "%") {
				endCol = v.Col + 1 // FOR var: Col points to letter, identifier is 1 char
			}
			diags = append(diags, Diag{
				Line:    v.Line,
				Col:     v.Col,
				EndCol:  endCol,
				Message: "Variable defined but never used: " + v.Name,
				Sev:     SevHint,
			})
		}
	}

	// If the script makes any external CALL file.bat, those scripts can set
	// arbitrary variables. Suppress "not defined in this file" hints in that case
	// since we cannot statically know what the called script exports.
	hasFileCalls := len(a.FileRefs) > 0

	// Build lookup: defined SET vars (file-wide) and FOR vars with their scopes.
	type forScope struct{ start, end int }
	forScopes := make(map[string][]forScope) // Name → list of scopes
	setDefined := make(map[string]bool)
	for _, v := range a.Vars {
		if strings.HasPrefix(v.Name, "%") {
			forScopes[v.Name] = append(forScopes[v.Name], forScope{v.Line, v.ScopeEnd})
		} else {
			setDefined[v.Name] = true
		}
	}

	// Variables used but never defined.
	for _, ref := range a.VarRefs {
		if ref.IsDelayed {
			// !VAR! delayed-expansion reference.
			if !a.DelayedExpansionEnabled {
				diags = append(diags, Diag{
					Line:    ref.Line,
					Col:     ref.Col - 1, // include the leading '!'
					EndCol:  ref.Col + len(ref.Name) + 1,
					Message: "Delayed expansion used but SETLOCAL ENABLEDELAYEDEXPANSION not found",
					Sev:     SevWarning,
				})
			} else if !setDefined[ref.Name] && !cmdBuiltinVars[ref.Name] && !hasFileCalls {
				diags = append(diags, Diag{
					Line:    ref.Line,
					Col:     ref.Col - 1,
					EndCol:  ref.Col + len(ref.Name) + 1,
					Message: "Variable not defined in this file: " + ref.Name,
					Sev:     SevHint,
				})
			}
		} else if strings.HasPrefix(ref.Name, "%") {
			// FOR-style %%X: must have a VarDef whose scope contains ref.Line.
			inScope := false
			for _, s := range forScopes[ref.Name] {
				if ref.Line >= s.start && (s.end < 0 || ref.Line <= s.end) {
					inScope = true
					break
				}
			}
			if !inScope {
				diags = append(diags, Diag{
					Line:    ref.Line,
					Col:     ref.Col - 2, // ref.Col points to letter; token starts at %%
					EndCol:  ref.Col + 1,
					Message: "Undefined FOR loop variable: " + ref.Name,
					Sev:     SevWarning,
				})
			}
		} else {
			// SET-style %VAR%: hint if not defined anywhere in this file.
			// Suppressed for built-in CMD vars and when the script makes external
			// CALL file.bat references (those scripts can set any variable).
			if !setDefined[ref.Name] && !cmdBuiltinVars[ref.Name] && !hasFileCalls {
				diags = append(diags, Diag{
					Line:    ref.Line,
					Col:     ref.Col,
					EndCol:  ref.Col + len(ref.Name),
					Message: "Variable not defined in this file: " + ref.Name,
					Sev:     SevHint,
				})
			}
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
	// For loop variables like %%I we want the Word to be just `%I` or we need to extract it properly.
	// But `WordAtPosition` currently doesn't include `%`. Let's update `isWordChar` to include `%`.
	return line[start:end]
}

func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.'
}

// cmdBuiltinVars is the set of variable names that are always present in a CMD
// environment at runtime (OS environment + ERRORLEVEL). Populated once from
// processor.BuiltinVarNames() so the LSP and the processor stay in sync.
var cmdBuiltinVars = processor.BuiltinVarNames()

// extractForFTokensSpec returns the tokens= value from a FOR /F options string,
// or "" if there is none. It finds the first double-quoted string in forLine and
// searches it for "tokens=<spec>".
func extractForFTokensSpec(forLine string) string {
	start := strings.IndexByte(forLine, '"')
	if start < 0 {
		return ""
	}
	end := strings.IndexByte(forLine[start+1:], '"')
	if end < 0 {
		return ""
	}
	opts := forLine[start+1 : start+1+end]
	optsLower := strings.ToLower(opts)
	tokIdx := strings.Index(optsLower, "tokens=")
	if tokIdx < 0 {
		return ""
	}
	spec := opts[tokIdx+7:]
	if spaceIdx := strings.IndexAny(spec, " \t"); spaceIdx >= 0 {
		spec = spec[:spaceIdx]
	}
	return spec
}

// countForFTokens returns the number of tokens captured by a FOR /F tokens= spec.
// Examples: "2,3" → 2, "1-3" → 3, "1,2,3" → 3, "*" → 1.
func countForFTokens(spec string) int {
	count := 0
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "*" {
			count++
		} else if dashIdx := strings.Index(part, "-"); dashIdx > 0 {
			start, end := 0, 0
			for _, c := range part[:dashIdx] {
				if c >= '0' && c <= '9' {
					start = start*10 + int(c-'0')
				}
			}
			for _, c := range part[dashIdx+1:] {
				if c >= '0' && c <= '9' {
					end = end*10 + int(c-'0')
				}
			}
			if end >= start && start > 0 {
				count += end - start + 1
			} else {
				count++
			}
		} else {
			count++
		}
	}
	if count == 0 {
		return 1
	}
	return count
}

// extractSetAVars parses a SET /A expression and returns all variable names
// that are assigned (including augmented assignments like +=, -= etc.).
// Handles comma-separated multiple expressions: "A=1, B=2, C=A+B".
func extractSetAVars(expr string) []string {
	var names []string
	for _, part := range strings.Split(expr, ",") {
		part = strings.TrimSpace(part)
		eqIdx := strings.IndexByte(part, '=')
		if eqIdx <= 0 {
			continue
		}
		// Strip any trailing assignment operator (+=, -=, *=, /=, %=, &=, |=, ^=)
		name := strings.TrimRight(part[:eqIdx], "+-*/%&|^~! \t")
		if name != "" {
			names = append(names, strings.ToUpper(name))
		}
	}
	return names
}

// forScopeEnd returns the 0-based last line of the body of a FOR loop whose
// definition starts on forLine. For a single-line DO body it returns forLine;
// for a block body ("do (…)") it scans forward for the matching ")".
func forScopeEnd(lines []string, forLine int) int {
	if forLine >= len(lines) {
		return forLine
	}
	raw := strings.TrimRight(lines[forLine], "\r")
	lower := strings.ToLower(raw)

	// Find the last standalone "do" keyword in the line.
	// Standalone means preceded by space/tab (or start-of-string) and
	// followed by space/tab/'(' (or end-of-string).
	doPos := -1
	for i := 0; i <= len(lower)-2; i++ {
		if lower[i] != 'd' || lower[i+1] != 'o' {
			continue
		}
		before := i == 0 || lower[i-1] == ' ' || lower[i-1] == '\t'
		var afterCh byte
		if i+2 < len(lower) {
			afterCh = lower[i+2]
		}
		after := afterCh == 0 || afterCh == ' ' || afterCh == '\t' || afterCh == '('
		if before && after {
			doPos = i // keep scanning — take the last occurrence
		}
	}
	if doPos < 0 {
		return forLine
	}

	// Check whether a '(' immediately follows "do" (optionally separated by spaces).
	afterDo := strings.TrimLeft(raw[doPos+2:], " \t")
	if !strings.HasPrefix(afterDo, "(") {
		return forLine // single-line DO body
	}

	// Multi-line block: find the matching ')' by tracking parenthesis depth.
	openOff := doPos + 2 + (len(raw[doPos+2:]) - len(afterDo)) // byte index of '('
	depth := 0
	for j := forLine; j < len(lines); j++ {
		l := strings.TrimRight(lines[j], "\r")
		start := 0
		if j == forLine {
			start = openOff
		}
		for k := start; k < len(l); k++ {
			switch l[k] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					return j
				}
			}
		}
	}
	return len(lines) - 1 // unclosed block — assume it reaches end of file
}

// CalledDocURIs returns the set of workspace document URIs that the given
// analysis explicitly calls via "CALL <file>", reusing the FileRefs already
// collected by Analyze.
func CalledDocURIs(a Analysis, workspace map[string]*Document) map[string]bool {
	called := make(map[string]bool)
	for _, ref := range a.FileRefs {
		lowerPath := strings.ToLower(ref.Path)
		for uri := range workspace {
			if strings.HasSuffix(strings.ToLower(uri), lowerPath) {
				called[uri] = true
			}
		}
	}
	return called
}

// CompletionContext describes what kind of completion is appropriate at the
// cursor position.
type CompletionContext int

const (
	CompleteCommand         CompletionContext = iota // start of line / after pipe/& etc.
	CompleteLabel                                    // after GOTO or CALL :
	CompleteVariable                                 // inside %VAR% (odd number of %)
	CompleteForVariable                              // inside %%VAR (lineBefore ends with %%)
	CompleteDelayedVariable                          // inside !VAR! (odd number of !)
	CompleteFile                                     // generic path argument
)

// CompletionContextAt determines the completion context from the text up to
// the cursor.
func CompletionContextAt(lineBefore string) CompletionContext {
	trimmed := strings.TrimLeft(lineBefore, " \t@")
	lower := strings.ToLower(trimmed)

	// %%VAR style: lineBefore has "%%" followed only by word chars to end of input.
	if idx := strings.LastIndex(lineBefore, "%%"); idx >= 0 {
		rest := lineBefore[idx+2:]
		allWord := true
		for _, c := range rest {
			if !isWordChar(c) {
				allWord = false
				break
			}
		}
		if allWord {
			return CompleteForVariable
		}
	}
	// %VAR% style: odd number of '%' means the last '%' is an unclosed opener.
	if strings.Count(lineBefore, "%")%2 == 1 {
		return CompleteVariable
	}

	// !VAR! style: odd number of '!' means the last '!' is an unclosed opener.
	if strings.Count(lineBefore, "!")%2 == 1 {
		return CompleteDelayedVariable
	}

	// After GOTO or inside CALL :
	if strings.HasPrefix(lower, "goto ") || lower == "goto" {
		return CompleteLabel
	}
	if strings.HasPrefix(lower, "call :") || lower == "call :" {
		return CompleteLabel
	}

	// If we are on a SET command definition: "set VAR"
	if strings.HasPrefix(lower, "set ") && !strings.Contains(lower[4:], "=") {
		return CompleteVariable
	}

	// If we are on a FOR loop variable definition: "for %%I"
	if strings.HasPrefix(lower, "for ") && strings.Contains(lower, "%%") && !strings.Contains(lower, " in ") {
		return CompleteForVariable
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

// appendVarRefs scans line for %NAME% / %%X / !NAME! patterns and appends a
// VarRef for each. %% (escaped percent) and positional args %0-%9 are skipped.
// !! (escaped exclamation) is also skipped.
func appendVarRefs(refs []VarRef, line string, lineIdx int) []VarRef {
	// --- %NAME% and %%X pass ---
	rest := line
	offset := 0
	for {
		pct := strings.Index(rest, "%")
		if pct < 0 {
			break
		}

		// Check for %%I style FOR loop variables (letters only; %%0-%%9 are escaped positional args)
		if pct+2 < len(rest) && rest[pct+1] == '%' {
			char := rest[pct+2]
			if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
				refs = append(refs, VarRef{
					Name: "%" + strings.ToUpper(string(char)),
					Line: lineIdx,
					Col:  offset + pct + 2, // skip %% sigil; Col points to the letter
				})
				offset += pct + 3
				rest = rest[pct+3:]
				continue
			}
		}

		after := rest[pct+1:]
		end := strings.Index(after, "%")
		if end < 0 {
			break
		}
		name := after[:end]
		// Skip empty (%%→escaped %) and positional args (%0–%9)
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

	// --- !NAME! delayed-expansion pass ---
	rest = line
	offset = 0
	for {
		bang := strings.Index(rest, "!")
		if bang < 0 {
			break
		}
		after := rest[bang+1:]
		end := strings.Index(after, "!")
		if end < 0 {
			break
		}
		name := after[:end]
		// Skip empty !!→escaped ! ; name must be a valid identifier
		if name != "" {
			refs = append(refs, VarRef{
				Name:      strings.ToUpper(name),
				Line:      lineIdx,
				Col:       offset + bang + 1, // col of first char of name (after !)
				IsDelayed: true,
			})
		}
		advance := bang + 1 + end + 1 // past closing !
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

	// Helper to search workspace for a variable definition.
	// Scope-aware: FOR loop vars (ScopeEnd >= 0) only match when line is within
	// [v.Line, v.ScopeEnd]; they are never returned from other documents.
	findVarDef := func(word string) (Loc, bool) {
		// word is just the text, e.g. "MYVAR" or "I"
		wordWithPercent := "%" + word

		// varLoc converts a VarDef to a Loc whose Col/EndCol span the identifier
		// letter(s) only — no sigil prefix:
		//   %%I  → Col points to 'I', EndCol = Col+1  (Name="%I", but Col already at letter)
		//   %FOO% → Col points to 'F', EndCol = Col+len("FOO")
		varLoc := func(v VarDef, docURI string) Loc {
			if strings.HasPrefix(v.Name, "%") {
				// FOR loop var: Col already points to the letter; identifier is 1 char.
				return Loc{URI: docURI, Line: v.Line, Col: v.Col, EndCol: v.Col + 1}
			}
			return Loc{URI: docURI, Line: v.Line, Col: v.Col, EndCol: v.Col + len(v.Name)}
		}

		// Search current document first
		for _, v := range a.Vars {
			if v.Name != word && v.Name != wordWithPercent {
				continue
			}
			// FOR loop vars: only match if cursor line is within the loop's scope
			if v.ScopeEnd >= 0 && (line < v.Line || line > v.ScopeEnd) {
				continue
			}
			return varLoc(v, uri), true
		}
		// Fallback to other documents: only for file-wide vars (FOR vars are never shared)
		for otherUri, otherDoc := range workspace {
			if otherUri == uri {
				continue
			}
			for _, v := range otherDoc.Analysis.Vars {
				if (v.Name == word || v.Name == wordWithPercent) && v.ScopeEnd < 0 {
					return varLoc(v, otherUri), true
				}
			}
		}
		return Loc{}, false
	}

	// Prefer variable context when cursor is inside %...%, %%X, or !...!
	ctx := CompletionContextAt(lineBefore)
	if ctx == CompleteVariable || ctx == CompleteForVariable || ctx == CompleteDelayedVariable {
		word := strings.ToUpper(WordAtPosition(src, col))
		return findVarDef(word)
	}

	// Check if cursor is on a variable definition (SET or FOR)
	lowerSrc := strings.ToLower(strings.TrimSpace(src))
	if strings.HasPrefix(lowerSrc, "set ") || strings.HasPrefix(lowerSrc, "for ") {
		word := strings.ToUpper(WordAtPosition(src, col))
		for _, v := range a.Vars {
			if (v.Name == word || v.Name == "%"+word) && v.Line == line {
				return findVarDef(word)
			}
		}
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
			// lbl.Col already points to the first letter of the name (after ':').
			return Loc{URI: uri, Line: lbl.Line, Col: lbl.Col, EndCol: lbl.Col + len(lbl.Name)}, true
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

	// Check if we are on a variable usage or definition
	word := strings.ToUpper(WordAtPosition(src, col))
	wordWithPercent := "%" + word
	isVar := false

	refCtx := CompletionContextAt(lineBefore)
	if refCtx == CompleteVariable || refCtx == CompleteForVariable || refCtx == CompleteDelayedVariable {
		isVar = true
	} else {
		lowerSrc := strings.ToLower(strings.TrimSpace(src))
		if strings.HasPrefix(lowerSrc, "set ") || strings.HasPrefix(lowerSrc, "for ") {
			for _, v := range a.Vars {
				if (v.Name == word || v.Name == wordWithPercent) && v.Line == line {
					isVar = true
					break
				}
			}
		}
	}

	// Variable references
	if isVar {
		if word == "" {
			return nil
		}

		// Find the VarDef that covers the cursor position (scope-aware).
		// FOR loop vars may have multiple definitions (nested loops); pick the
		// innermost one whose scope contains the cursor line.
		var scopedVar *VarDef
		for i := range a.Vars {
			v := &a.Vars[i]
			if v.Name != word && v.Name != wordWithPercent {
				continue
			}
			if v.ScopeEnd >= 0 && (line < v.Line || line > v.ScopeEnd) {
				continue // cursor outside this loop's scope
			}
			scopedVar = v
			break
		}

		var locs []Loc
		seenLocs := make(map[string]bool)
		addLoc := func(l Loc) {
			key := fmt.Sprintf("%s:%d:%d", l.URI, l.Line, l.Col)
			if !seenLocs[key] {
				seenLocs[key] = true
				locs = append(locs, l)
			}
		}

		for wUri, wDoc := range workspace {
			for _, ref := range wDoc.Analysis.VarRefs {
				if ref.Name != word && ref.Name != wordWithPercent {
					continue
				}
				// FOR loop vars: restrict to this loop's scope (current doc only)
				if scopedVar != nil && scopedVar.ScopeEnd >= 0 {
					if wUri != uri || ref.Line < scopedVar.Line || ref.Line > scopedVar.ScopeEnd {
						continue
					}
				}
				addLoc(Loc{URI: wUri, Line: ref.Line, Col: ref.Col, EndCol: ref.Col + len(ref.Name)})
			}
			if includeDecl {
				for _, v := range wDoc.Analysis.Vars {
					if v.Name != word && v.Name != wordWithPercent {
						continue
					}
					// FOR loop vars: only include the specific definition we matched
					if v.ScopeEnd >= 0 {
						if scopedVar == nil || wUri != uri || v.Line != scopedVar.Line {
							continue
						}
					}
					var endCol int
					if strings.HasPrefix(v.Name, "%") {
						endCol = v.Col + 1
					} else {
						endCol = v.Col + len(v.Name)
					}
					addLoc(Loc{URI: wUri, Line: v.Line, Col: v.Col, EndCol: endCol})
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
				// lbl.Col points to the first letter (after ':'), matching where
				// references require the cursor to be placed.
				locs = append(locs, Loc{URI: uri, Line: lbl.Line, Col: lbl.Col, EndCol: lbl.Col + len(lbl.Name)})
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
	a := Analyze(content)
	if len(a.Labels) == 0 {
		return nil
	}

	lines := strings.Split(content, "\n")
	total := len(lines)

	labelLines := make([]int, len(a.Labels))
	for i, lbl := range a.Labels {
		labelLines[i] = lbl.Line
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

	// Index VarRefs by line for O(1) per-line lookup.
	varRefsByLine := make(map[int][]VarRef, len(a.VarRefs))
	for _, ref := range a.VarRefs {
		varRefsByLine[ref.Line] = append(varRefsByLine[ref.Line], ref)
	}

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

		// Variable refs (collected by Analyze, indexed by line above).
		for _, ref := range varRefsByLine[i] {
			col, tokenLen := ref.Col, len(ref.Name)
			if ref.IsDelayed {
				// !NAME! — ref.Col points to first char of name; token is !NAME! (name+2 chars).
				col -= 1
				tokenLen = len(ref.Name) + 2
			} else if strings.HasPrefix(ref.Name, "%") {
				// %%X style FOR-loop variable: ref.Col points to the letter (X),
				// but the full source token is %%X (3 chars starting 2 before).
				col -= 2
				tokenLen = 3
			}
			tokens = append(tokens, SemToken{
				Line:      i,
				Col:       col,
				Len:       tokenLen,
				TokenType: semVariable,
			})
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
	renameCtx := CompletionContextAt(lineBefore)
	if renameCtx == CompleteVariable || renameCtx == CompleteDelayedVariable {
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

	prepCtx := CompletionContextAt(lineBefore)
	if prepCtx == CompleteVariable || prepCtx == CompleteDelayedVariable {
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

