package lsp

import (
	"fmt"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/analyzer"
	"github.com/sonroyaalmerol/go-msbatch/pkg/executor"
	"github.com/sonroyaalmerol/go-msbatch/pkg/lexer"
)

type Document struct {
	URI     string
	Content string
	Result  *analyzer.Result
}

var SemTokenTypes = []string{"keyword", "variable", "function", "comment", "string", "operator"}
var SemTokenModifiers = []string{"declaration", "readonly"}

type CompletionContext int

const (
	CompleteCommand CompletionContext = iota
	CompleteLabel
	CompleteVariable
	CompleteForVariable
	CompleteDelayedVariable
	CompleteFile
)

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

func NewAnalyzer() *analyzer.Analyzer {
	return analyzer.NewAnalyzer()
}

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
		(r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' || r == '%'
}

func CompletionContextAt(lineBefore string) CompletionContext {
	trimmed := strings.TrimLeft(lineBefore, " \t@")
	lower := strings.ToLower(trimmed)

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

	if strings.Count(lineBefore, "%")%2 == 1 {
		return CompleteVariable
	}

	if strings.Count(lineBefore, "!")%2 == 1 {
		return CompleteDelayedVariable
	}

	if strings.HasPrefix(lower, "goto ") || lower == "goto" {
		return CompleteLabel
	}
	if strings.HasPrefix(lower, "call :") || lower == "call :" {
		return CompleteLabel
	}

	if strings.HasPrefix(lower, "set ") && !strings.Contains(lower[4:], "=") {
		return CompleteVariable
	}

	if strings.HasPrefix(lower, "for ") && strings.Contains(lower, "%%") && !strings.Contains(lower, " in ") {
		return CompleteForVariable
	}

	if !strings.Contains(strings.TrimLeft(lower, " \t@"), " ") {
		return CompleteCommand
	}

	return CompleteFile
}

func DefinitionAt(workspace map[string]*Document, uri string, line, col int) (analyzer.Location, bool) {
	doc, ok := workspace[uri]
	if !ok || doc.Result == nil {
		return analyzer.Location{}, false
	}

	if sym := findSymbolAtPosition(doc, line, col); sym != nil {
		return sym.Definition, true
	}

	if sym := findSymbolInCalledFiles(workspace, uri, doc, line, col); sym != nil {
		return sym.Definition, true
	}

	if targetLoc, ok := findCallTargetAtPosition(workspace, uri, doc, line, col); ok {
		return targetLoc, true
	}

	return analyzer.Location{}, false
}

func findSymbolInCalledFiles(workspace map[string]*Document, uri string, doc *Document, line, col int) *analyzer.Symbol {
	if doc.Result == nil {
		return nil
	}

	lineText := ""
	lines := strings.Split(doc.Content, "\n")
	if line >= 0 && line < len(lines) {
		lineText = lines[line]
	}

	varName := extractVarNameAtPosition(lineText, col)
	if varName == "" {
		return nil
	}

	for _, target := range doc.Result.CallTargets {
		resolvedURI := resolveCallTargetURI(uri, target, workspace)
		if resolvedURI == "" || resolvedURI == uri {
			continue
		}
		calledDoc, ok := workspace[resolvedURI]
		if !ok || calledDoc.Result == nil || calledDoc.Result.Symbols == nil {
			continue
		}
		if sym := calledDoc.Result.Symbols.Vars[varName]; sym != nil {
			return sym
		}
	}

	return nil
}

func extractVarNameAtPosition(lineText string, col int) string {
	if col < 0 || col >= len(lineText) {
		return ""
	}

	if col >= 1 && lineText[col-1] == '%' {
		end := col
		for end < len(lineText) {
			c := rune(lineText[end])
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
				break
			}
			end++
		}
		if end < len(lineText) && lineText[end] == '%' {
			return strings.ToUpper(lineText[col:end])
		}
	}

	if col >= 1 && lineText[col-1] == '!' {
		end := col
		for end < len(lineText) {
			c := rune(lineText[end])
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
				break
			}
			end++
		}
		if end < len(lineText) && lineText[end] == '!' {
			return strings.ToUpper(lineText[col:end])
		}
	}

	if col >= 2 && lineText[col-2] == '%' && lineText[col-1] == '%' {
		c := rune(lineText[col])
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			return strings.ToUpper(string(c))
		}
	}

	return ""
}

func findCallTargetAtPosition(workspace map[string]*Document, uri string, doc *Document, line, col int) (analyzer.Location, bool) {
	if doc.Result == nil {
		return analyzer.Location{}, false
	}

	for _, target := range doc.Result.CallTargets {
		targetLower := strings.ToLower(target)
		if !strings.HasSuffix(targetLower, ".bat") && !strings.HasSuffix(targetLower, ".cmd") {
			continue
		}

		for _, cmdLine := range strings.Split(doc.Content, "\n") {
			if strings.Contains(strings.ToLower(cmdLine), targetLower) {
				lineIdx := strings.Index(doc.Content, cmdLine)
				lineNum := strings.Count(doc.Content[:lineIdx], "\n")

				if lineNum == line {
					colIdx := strings.Index(strings.ToLower(cmdLine), targetLower)
					if col >= colIdx && col < colIdx+len(target) {
						resolvedURI := resolveCallTargetURI(uri, target, workspace)
						if resolvedURI != "" {
							return analyzer.Location{URI: resolvedURI, Line: 0, Col: 0}, true
						}
					}
				}
			}
		}
	}

	return analyzer.Location{}, false
}

func resolveCallTargetURI(fromURI, target string, workspace map[string]*Document) string {
	targetLower := strings.ToLower(target)

	for uri := range workspace {
		if strings.HasSuffix(strings.ToLower(uri), targetLower) {
			return uri
		}
	}

	fromDir := ""
	if idx := strings.LastIndex(fromURI, "/"); idx >= 0 {
		fromDir = fromURI[:idx]
	}
	candidate := fromDir + "/" + target
	candidateLower := strings.ToLower(candidate)

	for uri := range workspace {
		if strings.ToLower(uri) == candidateLower {
			return uri
		}
	}

	for uri := range workspace {
		uriBase := ""
		if idx := strings.LastIndex(uri, "/"); idx >= 0 {
			uriBase = uri[idx+1:]
		} else {
			uriBase = uri
		}
		if strings.ToLower(uriBase) == targetLower {
			return uri
		}
	}

	return ""
}

func findSymbolAtPosition(doc *Document, line, col int) *analyzer.Symbol {
	result := doc.Result
	if result == nil || result.Symbols == nil {
		return nil
	}

	for _, sym := range result.Symbols.Labels {
		if sym.Definition.Line == line && col >= sym.Definition.Col && col <= sym.Definition.Col+len(sym.Name) {
			return sym
		}
		for _, ref := range sym.References {
			if ref.Location.EndCol > 0 {
				if ref.Location.Line == line && col >= ref.Location.Col && col < ref.Location.EndCol {
					return sym
				}
			} else {
				if ref.Location.Line == line && col >= ref.Location.Col && col <= ref.Location.Col+len(sym.Name) {
					return sym
				}
			}
		}
	}

	for _, sym := range result.Symbols.Vars {
		if sym.Definition.Line == line && col >= sym.Definition.Col && col <= sym.Definition.Col+len(sym.Name) {
			return sym
		}
		for _, ref := range sym.References {
			if ref.Location.EndCol > 0 {
				if ref.Location.Line == line && col >= ref.Location.Col && col < ref.Location.EndCol {
					return sym
				}
			} else {
				if ref.Location.Line == line && col >= ref.Location.Col && col <= ref.Location.Col+len(sym.Name) {
					return sym
				}
			}
		}
	}

	for _, sym := range result.Symbols.ForVars {
		if sym.Definition.Line == line && col >= sym.Definition.Col && col <= sym.Definition.Col+2 {
			return sym
		}
		for _, ref := range sym.References {
			if ref.Location.EndCol > 0 {
				if ref.Location.Line == line && col >= ref.Location.Col && col < ref.Location.EndCol {
					return sym
				}
			} else {
				if ref.Location.Line == line && col >= ref.Location.Col && col <= ref.Location.Col+2 {
					return sym
				}
			}
		}
	}

	return nil
}

func ReferencesAt(workspace map[string]*Document, uri string, line, col int, includeDecl bool) []analyzer.Location {
	doc, ok := workspace[uri]
	if !ok || doc.Result == nil {
		return nil
	}

	sym := findSymbolAtPosition(doc, line, col)
	if sym == nil {
		return nil
	}

	var locs []analyzer.Location
	seen := make(map[string]bool)

	addLoc := func(loc analyzer.Location) {
		key := fmt.Sprintf("%s:%d:%d", loc.URI, loc.Line, loc.Col)
		if !seen[key] {
			seen[key] = true
			locs = append(locs, loc)
		}
	}

	if includeDecl {
		addLoc(sym.Definition)
	}

	for _, ref := range sym.References {
		addLoc(ref.Location)
	}

	return locs
}

func PrepareRenameAt(content string, line, col int) (analyzer.Location, bool) {
	result := NewAnalyzer().Analyze("", content)
	if result == nil || result.Symbols == nil {
		return analyzer.Location{}, false
	}

	for _, sym := range result.Symbols.Labels {
		if sym.Definition.Line == line && col >= sym.Definition.Col && col <= sym.Definition.Col+len(sym.Name) {
			return analyzer.Location{
				Line:   sym.Definition.Line,
				Col:    sym.Definition.Col,
				EndCol: sym.Definition.Col + len(sym.Name),
			}, true
		}
		for _, ref := range sym.References {
			if ref.Location.Line == line && col >= ref.Location.Col && col <= ref.Location.Col+len(sym.Name) {
				return analyzer.Location{
					Line:   ref.Location.Line,
					Col:    ref.Location.Col,
					EndCol: ref.Location.Col + len(sym.Name),
				}, true
			}
		}
	}

	for _, sym := range result.Symbols.Vars {
		if sym.Definition.Line == line && col >= sym.Definition.Col && col <= sym.Definition.Col+len(sym.Name) {
			return analyzer.Location{
				Line:   sym.Definition.Line,
				Col:    sym.Definition.Col,
				EndCol: sym.Definition.Col + len(sym.Name),
			}, true
		}
		for _, ref := range sym.References {
			if ref.Location.Line == line && col >= ref.Location.Col && col <= ref.Location.Col+len(sym.Name) {
				return analyzer.Location{
					Line:   ref.Location.Line,
					Col:    ref.Location.Col,
					EndCol: ref.Location.Col + len(sym.Name),
				}, true
			}
		}
	}

	for _, sym := range result.Symbols.ForVars {
		if sym.Definition.Line == line && col >= sym.Definition.Col && col <= sym.Definition.Col+2 {
			return analyzer.Location{
				Line:   sym.Definition.Line,
				Col:    sym.Definition.Col,
				EndCol: sym.Definition.Col + 2,
			}, true
		}
		for _, ref := range sym.References {
			if ref.Location.Line == line && col >= ref.Location.Col && col <= ref.Location.Col+2 {
				return analyzer.Location{
					Line:   ref.Location.Line,
					Col:    ref.Location.Col,
					EndCol: ref.Location.Col + 2,
				}, true
			}
		}
	}

	return analyzer.Location{}, false
}

func RenameAt(workspace map[string]*Document, uri string, line, col int, newName string) (map[string][]TextEdit, error) {
	doc, ok := workspace[uri]
	if !ok || doc.Result == nil {
		return nil, nil
	}

	sym := findSymbolAtPosition(doc, line, col)
	if sym == nil {
		return nil, nil
	}

	editsByURI := make(map[string][]TextEdit)
	seen := make(map[string]bool)

	addEdit := func(loc analyzer.Location, text string) {
		key := fmt.Sprintf("%s:%d:%d", loc.URI, loc.Line, loc.Col)
		if seen[key] {
			return
		}
		seen[key] = true
		editsByURI[loc.URI] = append(editsByURI[loc.URI], TextEdit{
			Line:    loc.Line,
			Col:     loc.Col,
			EndCol:  loc.Col + len(sym.Name),
			NewText: text,
		})
	}

	switch sym.Kind {
	case analyzer.SymbolLabel:
		addEdit(sym.Definition, newName)
		for _, ref := range sym.References {
			addEdit(ref.Location, newName)
		}
	case analyzer.SymbolVariable:
		addEdit(sym.Definition, newName)
		for _, ref := range sym.References {
			addEdit(ref.Location, newName)
		}
	case analyzer.SymbolForVar:
		upperName := strings.ToUpper(newName)
		if len(upperName) != 1 || upperName[0] < 'A' || upperName[0] > 'Z' {
			return nil, fmt.Errorf("FOR variable must be a single letter A-Z")
		}
		addEdit(sym.Definition, "%%"+upperName)
		for _, ref := range sym.References {
			addEdit(ref.Location, "%%"+upperName)
		}
	}

	if len(editsByURI) == 0 {
		return nil, nil
	}

	return editsByURI, nil
}

func CodeActionsAt(content string, line int) []CodeActionData {
	result := NewAnalyzer().Analyze("", content)
	if result == nil {
		return nil
	}

	var actions []CodeActionData

	for _, d := range result.Diagnostics {
		if d.Severity == analyzer.SeverityError && strings.Contains(d.Message, "Undefined") {
			if strings.Contains(d.Message, "label") {
				parts := strings.Split(d.Message, ": ")
				if len(parts) > 1 {
					labelName := parts[len(parts)-1]
					actions = append(actions, CodeActionData{
						Title:        "Create label :" + labelName,
						Kind:         "quickfix",
						NewLabelName: labelName,
						InsertLine:   line,
					})
				}
			}
		}
	}

	return actions
}

func FoldingRanges(content string) []FoldRange {
	result := NewAnalyzer().Analyze("", content)
	if result == nil || result.Symbols == nil {
		return nil
	}

	var folds []FoldRange
	collectFolds(result.Symbols.Global, &folds)
	return folds
}

func collectFolds(scope *analyzer.Scope, folds *[]FoldRange) {
	if scope == nil {
		return
	}
	if scope.StartLine >= 0 && scope.EndLine > scope.StartLine {
		*folds = append(*folds, FoldRange{
			StartLine: scope.StartLine,
			EndLine:   scope.EndLine,
			Kind:      "region",
		})
	}
	for _, child := range scope.Children {
		collectFolds(child, folds)
	}
}

func SemanticTokens(content string) []SemToken {
	result := NewAnalyzer().Analyze("", content)
	if result == nil || result.Symbols == nil {
		return nil
	}

	var tokens []SemToken

	for _, sym := range result.Symbols.Labels {
		tokens = append(tokens, SemToken{
			Line:      sym.Definition.Line,
			Col:       sym.Definition.Col,
			Len:       len(sym.Name),
			TokenType: tokenTypeFunction,
			Modifiers: modDeclaration,
		})
		for _, ref := range sym.References {
			tokens = append(tokens, SemToken{
				Line:      ref.Location.Line,
				Col:       ref.Location.Col,
				Len:       len(sym.Name),
				TokenType: tokenTypeFunction,
				Modifiers: 0,
			})
		}
	}

	for _, sym := range result.Symbols.Vars {
		tokens = append(tokens, SemToken{
			Line:      sym.Definition.Line,
			Col:       sym.Definition.Col,
			Len:       len(sym.Name),
			TokenType: tokenTypeVariable,
			Modifiers: modDeclaration,
		})
		for _, ref := range sym.References {
			tokens = append(tokens, SemToken{
				Line:      ref.Location.Line,
				Col:       ref.Location.Col,
				Len:       len(sym.Name),
				TokenType: tokenTypeVariable,
				Modifiers: 0,
			})
		}
	}

	for _, sym := range result.Symbols.ForVars {
		tokens = append(tokens, SemToken{
			Line:      sym.Definition.Line,
			Col:       sym.Definition.Col,
			Len:       2,
			TokenType: tokenTypeVariable,
			Modifiers: modDeclaration,
		})
		for _, ref := range sym.References {
			tokens = append(tokens, SemToken{
				Line:      ref.Location.Line,
				Col:       ref.Location.Col,
				Len:       2,
				TokenType: tokenTypeVariable,
				Modifiers: 0,
			})
		}
	}

	sortTokens(tokens)
	return tokens
}

const (
	tokenTypeKeyword  uint32 = 0
	tokenTypeVariable uint32 = 1
	tokenTypeFunction uint32 = 2
	tokenTypeComment  uint32 = 3
	tokenTypeString   uint32 = 4
	tokenTypeOperator uint32 = 5
	modDeclaration    uint32 = 1
	modReadonly       uint32 = 2
)

func sortTokens(tokens []SemToken) {
	for i := 0; i < len(tokens)-1; i++ {
		for j := i + 1; j < len(tokens); j++ {
			if tokens[j].Line < tokens[i].Line ||
				(tokens[j].Line == tokens[i].Line && tokens[j].Col < tokens[i].Col) {
				tokens[i], tokens[j] = tokens[j], tokens[i]
			}
		}
	}
}

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

func CodeLenses(content string) []CodeLensData {
	result := NewAnalyzer().Analyze("", content)
	if result == nil || result.Symbols == nil {
		return nil
	}

	var lenses []CodeLensData

	for _, sym := range result.Symbols.Labels {
		if sym.Definition.Line >= 0 {
			lenses = append(lenses, CodeLensData{
				Line:      sym.Definition.Line,
				LabelName: sym.Name,
				RefCount:  sym.RefCount(),
			})
		}
	}

	return lenses
}

type TextEdit struct {
	Line    int
	Col     int
	EndCol  int
	NewText string
}

type CodeActionData struct {
	Title        string
	Kind         string
	NewLabelName string
	InsertLine   int
}

type FoldRange struct {
	StartLine int
	EndLine   int
	Kind      string
}

type SemToken struct {
	Line      int
	Col       int
	Len       int
	TokenType uint32
	Modifiers uint32
}

type CodeLensData struct {
	Line      int
	LabelName string
	RefCount  int
}
