package lsp

import (
	"fmt"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/executor"
	"github.com/sonroyaalmerol/go-msbatch/pkg/lexer"
	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
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
	ExprLen   int  // length of the full expression between sigils when a :modifier is present
	// (e.g. len("STR:~0,5") for %STR:~0,5%); 0 means same as len(Name)
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

func collectTokens(lines []string) []lexer.Item {
	var allTokens []lexer.Item
	for i, raw := range lines {
		lineText := strings.TrimRight(raw, "\r")
		bl := lexer.NewWithLine(lineText, i)
		for {
			t := bl.NextItem()
			if t.Type == lexer.TokenEOF || (t.Type == 0 && len(t.Value) == 0) {
				break
			}
			allTokens = append(allTokens, t)
		}
		// Inject explicit newline between lines so the parser can detect statement
		// boundaries (it relies on TokenNewline to delimit commands).
		allTokens = append(allTokens, lexer.Item{Line: i, Type: lexer.TokenNewline, Value: []rune{'\n'}})
	}
	return allTokens
}

// Analyze parses the document content and extracts structural information.
// The AST (built from per-line lexer invocations with correct line numbers) drives
// label/variable/reference discovery. Variable-usage scanning (%VAR%, !VAR!, %%X)
// remains text-based because the lexer's stateGoto emits %VAR% inside GOTO targets
// as TokenNameLabel rather than TokenNameVariable.
func Analyze(content string) Analysis {
	var a Analysis
	lines := strings.Split(content, "\n")

	// Lex each line with its 0-based line number so every emitted token
	// carries accurate Line/Col fields.
	allTokens := collectTokens(lines)

	// Parse the full token stream into an AST.
	nodes := parser.NewFromTokens(allTokens).Parse()

	// Walk the AST to collect labels, variable definitions, and references.
	for _, node := range nodes {
		analyzeNode(&a, node, lines)
	}

	// Text pass: collect %VAR% / !VAR! / %%X usages.
	// This must stay text-based: stateGoto swallows %VAR% as TokenNameLabel so
	// a purely token-driven pass would miss variable refs inside GOTO targets.
	for i, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)
		stripped := strings.TrimLeft(trimmed, "@")
		strippedLower := strings.ToLower(stripped)
		// Skip comment lines — no variable refs live inside comments.
		if strings.HasPrefix(trimmed, "::") || strings.HasPrefix(strippedLower, "rem ") || strippedLower == "rem" {
			continue
		}
		// Skip label-definition lines — var refs on ':label' lines are meaningless.
		if strings.HasPrefix(trimmed, ":") && !strings.HasPrefix(trimmed, "::") {
			continue
		}
		a.VarRefs = appendVarRefs(a.VarRefs, line, i)
	}

	return a
}

// analyzeNode recursively walks an AST node and populates a.
func analyzeNode(a *Analysis, node parser.Node, lines []string) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *parser.LabelNode:
		if n.Name != "" {
			a.Labels = append(a.Labels, LabelDef{
				Name: strings.ToLower(n.Name),
				Line: n.Line,
				Col:  n.Col,
			})
		}

	case *parser.CommentNode:
		// No structural information in comments.

	case *parser.SimpleCommand:
		analyzeSimpleCmd(a, n, lines)

	case *parser.IfNode:
		analyzeNode(a, n.Then, lines)
		analyzeNode(a, n.Else, lines)

	case *parser.ForNode:
		analyzeForNode(a, n, lines)

	case *parser.Block:
		for _, child := range n.Body {
			analyzeNode(a, child, lines)
		}

	case *parser.PipeNode:
		analyzeNode(a, n.Left, lines)
		analyzeNode(a, n.Right, lines)

	case *parser.BinaryNode:
		analyzeNode(a, n.Left, lines)
		analyzeNode(a, n.Right, lines)
	}
}

// analyzeSimpleCmd handles SET, GOTO, CALL, SETLOCAL simple commands.
func analyzeSimpleCmd(a *Analysis, cmd *parser.SimpleCommand, lines []string) {
	if cmd.Line >= len(lines) {
		return
	}
	line := lines[cmd.Line]
	cmdLower := strings.ToLower(cmd.Name)

	switch cmdLower {
	case "setlocal":
		for _, arg := range cmd.Args {
			if strings.EqualFold(arg, "enabledelayedexpansion") {
				a.DelayedExpansionEnabled = true
			}
		}

	case "goto":
		if len(cmd.Args) == 0 {
			return
		}
		target := strings.TrimPrefix(cmd.Args[0], ":")
		targetLower := strings.ToLower(target)
		if target != "" && targetLower != "eof" {
			col := labelColAfterKeyword(line, len(cmd.Name))
			a.GotoRefs = append(a.GotoRefs, LabelRef{Name: targetLower, Line: cmd.Line, Col: col})
			if strings.Contains(target, "%") {
				a.HasDynamicJumps = true
			}
		}

	case "call":
		if len(cmd.Args) == 0 {
			return
		}
		arg0 := cmd.Args[0]
		if strings.HasPrefix(arg0, ":") {
			name := arg0[1:]
			nameLower := strings.ToLower(name)
			if name != "" && nameLower != "eof" {
				col := labelColAfterKeyword(line, len(cmd.Name))
				a.CallRefs = append(a.CallRefs, LabelRef{Name: nameLower, Line: cmd.Line, Col: col})
				if strings.Contains(name, "%") {
					a.HasDynamicJumps = true
				}
			}
		} else {
			pathLower := strings.ToLower(arg0)
			if strings.HasSuffix(pathLower, ".bat") || strings.HasSuffix(pathLower, ".cmd") {
				col := labelColAfterKeyword(line, len(cmd.Name))
				a.FileRefs = append(a.FileRefs, FileRef{Path: arg0, Line: cmd.Line, Col: col})
			}
		}

	case "set":
		analyzeSetCmd(a, cmd, line)
	}
}

// analyzeSetCmd extracts variable definitions from a SET command node.
func analyzeSetCmd(a *Analysis, cmd *parser.SimpleCommand, line string) {
	if len(cmd.RawArgs) == 0 {
		return
	}
	// Compute the column where the argument begins (after "set " and optional spaces).
	afterSet := line[cmd.Col+len(cmd.Name):]
	trimmedAfterSet := strings.TrimLeft(afterSet, " \t\v\f\xa0,;=")
	baseCol := cmd.Col + len(cmd.Name) + (len(afterSet) - len(trimmedAfterSet))

	// Reconstruct the raw argument string after "set "
	fullArg := processor.ExtractRawArgString(cmd.RawArgs)

	fullArgLower := strings.ToLower(fullArg)

	switch {
	case strings.HasPrefix(fullArgLower, "/a"):
		// SET /A: arithmetic — extract all assigned variable names.
		expr := strings.TrimSpace(fullArg[2:])
		for _, name := range extractSetAVars(expr) {
			a.Vars = append(a.Vars, VarDef{Name: name, Line: cmd.Line, Col: baseCol, ScopeEnd: -1})
		}

	case strings.HasPrefix(fullArgLower, "/p"):
		// SET /P: prompt — variable name before '='.
		promptPart := strings.TrimSpace(fullArg[2:])
		if idx := strings.IndexByte(promptPart, '='); idx > 0 {
			a.Vars = append(a.Vars, VarDef{
				Name: strings.ToUpper(promptPart[:idx]), Line: cmd.Line, Col: baseCol, ScopeEnd: -1,
			})
		}

	default:
		// Plain SET: "VAR=value"
		// Strip quotes if they enclose the whole expression
		arg := fullArg
		if strings.HasPrefix(arg, "\"") && strings.HasSuffix(arg, "\"") {
			arg = arg[1 : len(arg)-1]
		}
		if idx := strings.IndexByte(arg, '='); idx > 0 {
			name := arg[:idx]
			value := arg[idx+1:]
			a.Vars = append(a.Vars, VarDef{
				Name: strings.ToUpper(name), Value: value, Line: cmd.Line, Col: baseCol, ScopeEnd: -1,
			})
		}
	}
}

// analyzeForNode extracts a FOR loop variable definition and recurses into Do.
func analyzeForNode(a *Analysis, n *parser.ForNode, lines []string) {
	if n.Variable == "" {
		analyzeNode(a, n.Do, lines)
		return
	}

	char := n.Variable[0]
	// FOR loop variables must be letters.
	if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')) {
		analyzeNode(a, n.Do, lines)
		return
	}

	// Scope ends at the last line covered by the Do body.
	scopeEnd := max(nodeLastLine(n.Do), n.Line)

	a.Vars = append(a.Vars, VarDef{
		Name:     "%" + strings.ToUpper(string(char)),
		Line:     n.VarLine,
		Col:      n.VarCol,
		ScopeEnd: scopeEnd,
	})

	// FOR /F with tokens= can capture multiple values into successive letters.
	if n.Variant == parser.ForF && n.VarLine < len(lines) {
		rawLine := strings.TrimRight(lines[n.Line], "\r")
		if tokenSpec := extractForFTokensSpec(rawLine); tokenSpec != "" {
			nTokens := countForFTokens(tokenSpec)
			for k := 1; k < nTokens; k++ {
				nextChar := char + byte(k)
				if !((nextChar >= 'a' && nextChar <= 'z') || (nextChar >= 'A' && nextChar <= 'Z')) {
					break
				}
				a.Vars = append(a.Vars, VarDef{
					Name:     "%" + strings.ToUpper(string(nextChar)),
					Line:     n.VarLine,
					Col:      n.VarCol,
					ScopeEnd: scopeEnd,
				})
			}
		}
	}

	analyzeNode(a, n.Do, lines)
}

// nodeLastLine returns the last source line covered by a parser node.
// Used to compute the scope end of FOR loop bodies.
func nodeLastLine(node parser.Node) int {
	if node == nil {
		return 0
	}
	switch n := node.(type) {
	case *parser.SimpleCommand:
		return n.Line
	case *parser.LabelNode:
		return n.Line
	case *parser.CommentNode:
		return n.Line
	case *parser.IfNode:
		if n.Else != nil {
			return nodeLastLine(n.Else)
		}
		return nodeLastLine(n.Then)
	case *parser.ForNode:
		return nodeLastLine(n.Do)
	case *parser.Block:
		// EndLine tracks the closing ')' line, which is the true scope boundary.
		return n.EndLine
	case *parser.PipeNode:
		return nodeLastLine(n.Right)
	case *parser.BinaryNode:
		return nodeLastLine(n.Right)
	}
	return 0
}

// Diagnostics returns a list of issues found in the document.
// Diagnostics returns diagnostics for a batch script given only its content.
// Use DiagnosticsWithContext when the workspace is available for accurate
// cross-file scoping.
func Diagnostics(content string) []Diag {
	a := Analyze(content)
	return diagnosticsFromAnalysis("", a, nil)
}

// DiagnosticsWithContext returns diagnostics for the document at uri using the
// full workspace to resolve cross-file variable definitions and usages.
func DiagnosticsWithContext(uri string, workspace map[string]*Document) []Diag {
	doc, ok := workspace[uri]
	if !ok {
		return nil
	}
	return diagnosticsFromAnalysis(uri, doc.Analysis, workspace)
}

func diagnosticsFromAnalysis(uri string, a Analysis, workspace map[string]*Document) []Diag {
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

	// Resolve cross-file context when workspace is available.
	var calledURIs, callerURIs map[string]bool
	if workspace != nil && uri != "" {
		calledURIs = CalledDocURIs(a, workspace)
		callerURIs = CallerDocURIs(uri, workspace)
	}

	// Variables defined but never used: SET VAR=... but %VAR% / !VAR! never appears.
	// A variable is considered "used" if it is referenced in this file OR in any
	// file that this file calls (since batch inherits the environment on CALL).
	varUsed := make(map[string]bool)
	for _, ref := range a.VarRefs {
		varUsed[ref.Name] = true
	}
	for calledURI := range calledURIs {
		if calledDoc, ok := workspace[calledURI]; ok {
			for _, ref := range calledDoc.Analysis.VarRefs {
				varUsed[ref.Name] = true
			}
		}
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

	// Build lookup: defined SET vars (file-wide) and FOR vars with their scopes.
	// Include SET vars from caller files (batch inherits env when called) and
	// from called files (variables exported back by the called script).
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
	for callerURI := range callerURIs {
		if callerDoc, ok := workspace[callerURI]; ok {
			for _, v := range callerDoc.Analysis.Vars {
				if !strings.HasPrefix(v.Name, "%") && v.ScopeEnd < 0 {
					setDefined[v.Name] = true
				}
			}
		}
	}
	for calledURI := range calledURIs {
		if calledDoc, ok := workspace[calledURI]; ok {
			for _, v := range calledDoc.Analysis.Vars {
				if !strings.HasPrefix(v.Name, "%") && v.ScopeEnd < 0 {
					setDefined[v.Name] = true
				}
			}
		}
	}

	// hasUnresolvableFileCalls: true when this file calls external scripts that are
	// not present in the workspace — those may set arbitrary variables so we
	// cannot safely warn about undefined variables in that case.
	hasUnresolvableFileCalls := false
	if workspace == nil {
		hasUnresolvableFileCalls = len(a.FileRefs) > 0
	} else {
		for _, ref := range a.FileRefs {
			lowerPath := strings.ToLower(ref.Path)
			found := false
			for wURI := range workspace {
				if strings.HasSuffix(strings.ToLower(wURI), lowerPath) {
					found = true
					break
				}
			}
			if !found {
				hasUnresolvableFileCalls = true
				break
			}
		}
	}

	// Variables used but never defined.
	for _, ref := range a.VarRefs {
		// exprEndCol returns the end column covering the full expression (including any :modifier).
		exprEndCol := func(startCol int) int {
			if ref.ExprLen > 0 {
				return startCol + ref.ExprLen
			}
			return startCol + len(ref.Name)
		}
		if ref.IsDelayed {
			// !VAR! delayed-expansion reference.
			if !a.DelayedExpansionEnabled {
				diags = append(diags, Diag{
					Line:    ref.Line,
					Col:     ref.Col - 1, // include the leading '!'
					EndCol:  exprEndCol(ref.Col) + 1,
					Message: "Delayed expansion used but SETLOCAL ENABLEDELAYEDEXPANSION not found",
					Sev:     SevWarning,
				})
			} else if !setDefined[ref.Name] && !cmdBuiltinVars[ref.Name] && !hasUnresolvableFileCalls {
				diags = append(diags, Diag{
					Line:    ref.Line,
					Col:     ref.Col - 1,
					EndCol:  exprEndCol(ref.Col) + 1,
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
			// SET-style %VAR% (possibly with :modifier): hint if not defined in this
			// file, any caller file, or any called file — and no unresolvable file
			// calls exist that might define it.
			if !setDefined[ref.Name] && !cmdBuiltinVars[ref.Name] && !hasUnresolvableFileCalls {
				diags = append(diags, Diag{
					Line:    ref.Line,
					Col:     ref.Col,
					EndCol:  exprEndCol(ref.Col),
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
	for part := range strings.SplitSeq(spec, ",") {
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
	for part := range strings.SplitSeq(expr, ",") {
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

// forScopeEnd returns the 0-based last line of the body of a FOR loop starting
// on forLine. It uses the parser so that scope boundaries come from the AST
// rather than manual parenthesis counting. Kept as an exported-to-test helper.
func forScopeEnd(lines []string, forLine int) int {
	if forLine >= len(lines) {
		return forLine
	}
	allTokens := collectTokens(lines)
	nodes := parser.NewFromTokens(allTokens).Parse()
	// Find the ForNode on forLine and return its scope end.
	var findFor func([]parser.Node) int
	findFor = func(ns []parser.Node) int {
		for _, node := range ns {
			switch n := node.(type) {
			case *parser.ForNode:
				if n.Line == forLine {
					return nodeLastLine(n.Do)
				}
				// recurse into Do
				if result := findFor([]parser.Node{n.Do}); result >= 0 {
					return result
				}
			case *parser.Block:
				if result := findFor(n.Body); result >= 0 {
					return result
				}
			case *parser.IfNode:
				if result := findFor([]parser.Node{n.Then, n.Else}); result >= 0 {
					return result
				}
			case *parser.BinaryNode:
				if result := findFor([]parser.Node{n.Left, n.Right}); result >= 0 {
					return result
				}
			case *parser.PipeNode:
				if result := findFor([]parser.Node{n.Left, n.Right}); result >= 0 {
					return result
				}
			}
		}
		return -1
	}
	if result := findFor(nodes); result >= 0 {
		return result
	}
	return forLine
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

// CallerDocURIs returns the set of workspace document URIs that explicitly
// call the given URI via "CALL <file>".
func CallerDocURIs(uri string, workspace map[string]*Document) map[string]bool {
	callers := make(map[string]bool)
	lowerURI := strings.ToLower(uri)
	for wURI, doc := range workspace {
		if wURI == uri {
			continue
		}
		for _, ref := range doc.Analysis.FileRefs {
			if strings.HasSuffix(lowerURI, strings.ToLower(ref.Path)) {
				callers[wURI] = true
				break
			}
		}
	}
	return callers
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
//
// %%X (FOR-loop variable usage) requires a text scan: outside stateFor the
// batch lexer emits %% as TokenStringEscape and cannot produce a single
// variable token for %%X in echo/if/other contexts.
//
// %NAME% and !NAME! also use text scanning for full coverage: the lexer's
// stateGoto consumes "goto %VAR%" as TokenNameLabel rather than
// TokenNameVariable, so a purely lexer-driven pass would miss variable
// references inside GOTO targets.
//
// processor.SplitVarModifier is used to strip any :modifier suffix so the
// stored Name is always the bare variable name.
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
		advance := pct + 1 + end + 1 // past closing %
		// Skip empty (%%→escaped %) and positional args (%0–%9)
		if name != "" && (name[0] < '0' || name[0] > '9') {
			// Skip tilde-modifier positional-param patterns like %~n0, %~$PATH:1
			// that got captured because a later % on the line closed the token.
			if name[0] != '~' {
				// Strip :modifier suffix using the shared processor helper.
				baseName, _ := processor.SplitVarModifier(name)
				var exprLen int
				if baseName != name {
					exprLen = len(name)
				}
				refs = append(refs, VarRef{
					Name:    strings.ToUpper(baseName),
					Line:    lineIdx,
					Col:     offset + pct + 1, // col of first char of name (after %)
					ExprLen: exprLen,
				})
			}
		}
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
		advance := bang + 1 + end + 1 // past closing !
		// Skip empty !!→escaped !
		if name != "" {
			baseName, _ := processor.SplitVarModifier(name)
			var exprLen int
			if baseName != name {
				exprLen = len(name)
			}
			refs = append(refs, VarRef{
				Name:      strings.ToUpper(baseName),
				Line:      lineIdx,
				Col:       offset + bang + 1, // col of first char of name (after !)
				IsDelayed: true,
				ExprLen:   exprLen,
			})
		}
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

	// varLoc converts a VarDef to a Loc spanning just the identifier (no sigils).
	varLoc := func(v VarDef, docURI string) Loc {
		if strings.HasPrefix(v.Name, "%") {
			return Loc{URI: docURI, Line: v.Line, Col: v.Col, EndCol: v.Col + 1}
		}
		return Loc{URI: docURI, Line: v.Line, Col: v.Col, EndCol: v.Col + len(v.Name)}
	}

	// findVarDef looks up a variable by its canonical name.
	//   FOR vars  → name starts with "%", e.g. "%A"  — local file + scope only
	//   SET vars  → bare name, e.g. "MYVAR"          — current file first, then called files
	findVarDef := func(name string) (Loc, bool) {
		isForVar := strings.HasPrefix(name, "%")
		for _, v := range a.Vars {
			if v.Name != name {
				continue
			}
			if v.ScopeEnd >= 0 && (line < v.Line || line > v.ScopeEnd) {
				continue
			}
			return varLoc(v, uri), true
		}
		if isForVar {
			return Loc{}, false // FOR vars never cross file boundaries
		}
		calledURIs := CalledDocURIs(a, workspace)
		for otherUri, otherDoc := range workspace {
			if otherUri == uri || !calledURIs[otherUri] {
				continue
			}
			for _, v := range otherDoc.Analysis.Vars {
				if v.Name == name && v.ScopeEnd < 0 {
					return varLoc(v, otherUri), true
				}
			}
		}
		return Loc{}, false
	}

	// Dispatch based on cursor context to get the canonical variable name.
	ctx := CompletionContextAt(lineBefore)
	word := strings.ToUpper(WordAtPosition(src, col))
	switch ctx {
	case CompleteForVariable:
		return findVarDef("%" + word)
	case CompleteVariable, CompleteDelayedVariable:
		return findVarDef(word)
	}

	// Check if cursor is on a variable definition line (SET or FOR).
	// Use the VarDef's own Name as the canonical form so we never conflate
	// a SET var "A" with a FOR var "%A".
	lowerSrc := strings.ToLower(strings.TrimSpace(src))
	if strings.HasPrefix(lowerSrc, "set ") || strings.HasPrefix(lowerSrc, "for ") {
		for _, v := range a.Vars {
			if v.Line != line {
				continue
			}
			if v.Name == word || v.Name == "%"+word {
				return findVarDef(v.Name)
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
	word = strings.ToLower(WordAtPosition(src, col))
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

	// Resolve the canonical variable name from cursor context.
	//   CompleteForVariable     → "%A"  (FOR var, local file + scope only)
	//   CompleteVariable /
	//   CompleteDelayedVariable → "FOO" (SET var, current + called files)
	//   VarDef line             → use v.Name directly (exact canonical form)
	word := strings.ToUpper(WordAtPosition(src, col))
	var targetName string
	refCtx := CompletionContextAt(lineBefore)
	switch refCtx {
	case CompleteForVariable:
		targetName = "%" + word
	case CompleteVariable, CompleteDelayedVariable:
		targetName = word
	default:
		lowerSrc := strings.ToLower(strings.TrimSpace(src))
		if strings.HasPrefix(lowerSrc, "set ") || strings.HasPrefix(lowerSrc, "for ") {
			for _, v := range a.Vars {
				if v.Line != line {
					continue
				}
				if v.Name == word || v.Name == "%"+word {
					targetName = v.Name
					break
				}
			}
		}
	}
	isVar := targetName != ""
	isForVar := strings.HasPrefix(targetName, "%")

	// Variable references
	if isVar {
		// Find the enclosing VarDef (scope-aware for FOR vars).
		var scopedVar *VarDef
		for i := range a.Vars {
			v := &a.Vars[i]
			if v.Name != targetName {
				continue
			}
			if v.ScopeEnd >= 0 && (line < v.Line || line > v.ScopeEnd) {
				continue
			}
			scopedVar = v
			break
		}

		// For FOR vars we need a scopedVar to know the loop boundaries.
		// If not found (e.g. cursor on def line itself, scope check excluded it),
		// relax to any definition in this file with matching name.
		if isForVar && scopedVar == nil {
			for i := range a.Vars {
				v := &a.Vars[i]
				if v.Name == targetName {
					scopedVar = v
					break
				}
			}
		}

		calledURIs := CalledDocURIs(a, workspace)

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
			// FOR vars are strictly local — never cross file boundaries.
			if isForVar && wUri != uri {
				continue
			}
			// SET vars: only current file and explicitly called files.
			if !isForVar && wUri != uri && !calledURIs[wUri] {
				continue
			}

			for _, ref := range wDoc.Analysis.VarRefs {
				if ref.Name != targetName {
					continue
				}
				// FOR vars: restrict to this loop's scope.
				if isForVar && scopedVar != nil {
					if ref.Line < scopedVar.Line || ref.Line > scopedVar.ScopeEnd {
						continue
					}
				}
				endCol := ref.Col + len(ref.Name)
				if isForVar {
					endCol = ref.Col + 1 // only the letter, not the "%" prefix in Name
				}
				addLoc(Loc{URI: wUri, Line: ref.Line, Col: ref.Col, EndCol: endCol})
			}
			if includeDecl {
				for _, v := range wDoc.Analysis.Vars {
					if v.Name != targetName {
						continue
					}
					if isForVar {
						if scopedVar == nil || v.Line != scopedVar.Line {
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
			// exprLen: length of the full expression between sigils (e.g. "STR:~0,5" = 8)
			exprLen := len(ref.Name)
			if ref.ExprLen > 0 {
				exprLen = ref.ExprLen
			}
			col, tokenLen := ref.Col, exprLen
			if ref.IsDelayed {
				// !NAME[modifier]! — ref.Col points to first char of name; token is !expr! (exprLen+2 chars).
				col -= 1
				tokenLen = exprLen + 2
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

	// FOR loop variable rename: local file + scope only.
	if renameCtx == CompleteForVariable {
		word := strings.ToUpper(WordAtPosition(src, col))
		if word == "" {
			return nil, fmt.Errorf("no renameable symbol at cursor")
		}
		targetName := "%" + word
		// Find the enclosing scope
		var scopedVar *VarDef
		for i := range a.Vars {
			v := &a.Vars[i]
			if v.Name != targetName {
				continue
			}
			if v.ScopeEnd >= 0 && (line < v.Line || line > v.ScopeEnd) {
				continue
			}
			scopedVar = v
			break
		}
		if scopedVar == nil {
			return nil, fmt.Errorf("no renameable symbol at cursor")
		}
		newLetter := strings.ToLower(strings.TrimLeft(newName, "%"))
		if newLetter == "" {
			return nil, fmt.Errorf("new name must be a single letter")
		}
		newLetter = string([]rune(newLetter)[0])
		var edits []TextEdit
		// Rename the definition letter
		edits = append(edits, TextEdit{
			Line: scopedVar.Line, Col: scopedVar.Col, EndCol: scopedVar.Col + 1,
			NewText: strings.ToUpper(newLetter),
		})
		// Rename all in-scope refs
		for _, ref := range a.VarRefs {
			if ref.Name != targetName {
				continue
			}
			if ref.Line < scopedVar.Line || ref.Line > scopedVar.ScopeEnd {
				continue
			}
			edits = append(edits, TextEdit{
				Line: ref.Line, Col: ref.Col, EndCol: ref.Col + 1,
				NewText: strings.ToLower(newLetter),
			})
		}
		editsByURI := map[string][]TextEdit{uri: edits}
		return editsByURI, nil
	}

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

	if prepCtx == CompleteForVariable {
		word := strings.ToUpper(WordAtPosition(src, col))
		if word == "" {
			return Loc{}, false
		}
		targetName := "%" + word
		for _, v := range a.Vars {
			if v.Name == targetName && !(v.ScopeEnd >= 0 && (line < v.Line || line > v.ScopeEnd)) {
				start, end := wordRangeAt(src, col)
				return Loc{Line: line, Col: start, EndCol: end}, true
			}
		}
		for _, ref := range a.VarRefs {
			if ref.Name == targetName && ref.Line == line {
				start, end := wordRangeAt(src, col)
				return Loc{Line: line, Col: start, EndCol: end}, true
			}
		}
		return Loc{}, false
	}

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
