package analyzer

import (
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

type pendingLabelRef struct {
	target string
	loc    Location
	kind   ReferenceKind
}

type builder struct {
	result                  *Result
	lines                   []string
	uri                     string
	scopeStack              []*Scope
	delayedExpansionEnabled bool
	hasDynamicJumps         bool
	callTargets             []string
	setlocalDepth           int
	pendingLabelRefs        []pendingLabelRef
}

func newBuilder(result *Result, lines []string, uri string) *builder {
	b := &builder{
		result:           result,
		lines:            lines,
		uri:              uri,
		callTargets:      []string{},
		pendingLabelRefs: []pendingLabelRef{},
	}
	b.scopeStack = []*Scope{result.Symbols.Global}
	return b
}

func (b *builder) currentScope() *Scope {
	if len(b.scopeStack) == 0 {
		return b.result.Symbols.Global
	}
	return b.scopeStack[len(b.scopeStack)-1]
}

func (b *builder) pushScope(kind ScopeKind, startLine int) *Scope {
	scope := NewScope(kind, b.currentScope())
	scope.StartLine = startLine
	scope.URI = b.uri
	b.currentScope().AddChild(scope)
	b.scopeStack = append(b.scopeStack, scope)
	return scope
}

func (b *builder) popScope(endLine int) {
	if len(b.scopeStack) > 1 {
		scope := b.currentScope()
		scope.EndLine = endLine
		b.scopeStack = b.scopeStack[:len(b.scopeStack)-1]
	}
}

func (b *builder) Build(node parser.Node) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	case *parser.SimpleCommand:
		b.buildSimpleCommand(n)
	case *parser.LabelNode:
		b.buildLabel(n)
	case *parser.CommentNode:
	case *parser.IfNode:
		b.buildIf(n)
	case *parser.ForNode:
		b.buildFor(n)
	case *parser.Block:
		b.buildBlock(n)
	case *parser.PipeNode:
		b.Build(n.Left)
		b.Build(n.Right)
	case *parser.BinaryNode:
		b.Build(n.Left)
		b.Build(n.Right)
	}
}

func (b *builder) buildSimpleCommand(cmd *parser.SimpleCommand) {
	if cmd.Line >= len(b.lines) {
		return
	}

	nameLower := strings.ToLower(cmd.Name)
	line := cmd.Line

	switch nameLower {
	case "set":
		b.buildSet(cmd, line)
	case "setlocal":
		b.buildSetlocal(cmd, line)
	case "endlocal":
		b.buildEndlocal(line)
	case "goto":
		b.buildGoto(cmd, line)
	case "call":
		b.buildCall(cmd, line)
	case "shift":
	default:
	}
}

func (b *builder) buildSet(cmd *parser.SimpleCommand, line int) {
	if len(cmd.RawArgs) == 0 {
		return
	}

	fullArg := processor.ExtractRawArgString(cmd.RawArgs)
	fullArgLower := strings.ToLower(fullArg)
	lineText := ""
	if line < len(b.lines) {
		lineText = b.lines[line]
	}

	switch {
	case strings.HasPrefix(fullArgLower, "/a"):
		expr := strings.TrimSpace(fullArg[2:])
		for _, name := range extractSetAVars(expr) {
			nameCol := findVarColInLine(lineText, name, cmd.Col+len(cmd.Name)+1)
			loc := Location{URI: b.uri, Line: line, Col: nameCol}
			sym := b.result.Symbols.DefineVariable(name, loc)
			sym.AddRef(loc, RefDefinition)
		}
	case strings.HasPrefix(fullArgLower, "/p"):
		promptPart := strings.TrimSpace(fullArg[2:])
		if idx := strings.IndexByte(promptPart, '='); idx > 0 {
			name := promptPart[:idx]
			nameCol := findVarColInLine(lineText, name, cmd.Col+len(cmd.Name)+1)
			loc := Location{URI: b.uri, Line: line, Col: nameCol}
			sym := b.result.Symbols.DefineVariable(name, loc)
			sym.AddRef(loc, RefDefinition)
		}
	default:
		arg := fullArg
		if strings.HasPrefix(arg, "\"") && strings.HasSuffix(arg, "\"") {
			arg = arg[1 : len(arg)-1]
		}
		if idx := strings.IndexByte(arg, '='); idx > 0 {
			name := arg[:idx]
			value := arg[idx+1:]
			nameCol := findVarColInLine(lineText, name, cmd.Col+len(cmd.Name)+1)
			loc := Location{URI: b.uri, Line: line, Col: nameCol}
			sym := b.result.Symbols.DefineVariable(name, loc)
			sym.AddRef(loc, RefDefinition)
			sym.InferredValue = value
		}
	}
}

func findVarColInLine(lineText, varName string, startSearch int) int {
	if lineText == "" {
		return startSearch
	}
	upperLine := strings.ToUpper(lineText)
	target := strings.ToUpper(varName)
	for i := startSearch; i <= len(upperLine)-len(target); i++ {
		if upperLine[i:i+len(target)] == target {
			afterEnd := i + len(target)
			if afterEnd >= len(upperLine) || upperLine[afterEnd] == '=' || upperLine[afterEnd] == ' ' || upperLine[afterEnd] == '\t' || upperLine[afterEnd] == ',' {
				if i == startSearch || !isWordChar(rune(upperLine[i-1])) {
					if upperLine[i-1] != '/' {
						return i
					}
				}
			}
		}
	}
	return startSearch
}

func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_' || r == '-'
}

func extractSetAVars(expr string) []string {
	var names []string
	for part := range strings.SplitSeq(expr, ",") {
		part = strings.TrimSpace(part)
		eqIdx := strings.IndexByte(part, '=')
		if eqIdx <= 0 {
			continue
		}
		name := strings.TrimRight(part[:eqIdx], "+-*/%&|^~! \t")
		if name != "" {
			names = append(names, strings.ToUpper(name))
		}
	}
	return names
}

func (b *builder) buildSetlocal(cmd *parser.SimpleCommand, line int) {
	b.setlocalDepth++
	b.pushScope(ScopeSetlocal, line)

	for _, arg := range cmd.Args {
		if strings.EqualFold(arg, "enabledelayedexpansion") {
			b.delayedExpansionEnabled = true
		}
	}
}

func (b *builder) buildEndlocal(line int) {
	if b.setlocalDepth > 0 {
		b.setlocalDepth--
		b.popScope(line)
	}
}

func (b *builder) buildLabel(label *parser.LabelNode) {
	if label.Name == "" {
		return
	}
	loc := Location{URI: b.uri, Line: label.Line, Col: label.Col, EndCol: label.Col + len(label.Name)}
	sym := b.result.Symbols.DefineLabel(label.Name, loc)
	sym.AddRef(loc, RefDefinition)
}

func (b *builder) buildGoto(cmd *parser.SimpleCommand, line int) {
	if len(cmd.Args) == 0 {
		return
	}
	target := strings.TrimPrefix(cmd.Args[0], ":")
	loc := Location{URI: b.uri, Line: line, Col: cmd.Col + len(cmd.Name) + 1, EndCol: cmd.Col + len(cmd.Name) + 1 + len(target)}
	b.pendingLabelRefs = append(b.pendingLabelRefs, pendingLabelRef{
		target: target,
		loc:    loc,
		kind:   RefGoto,
	})
	if strings.Contains(target, "%") {
		b.hasDynamicJumps = true
	}
}

func (b *builder) buildCall(cmd *parser.SimpleCommand, line int) {
	if len(cmd.Args) == 0 {
		return
	}
	arg0 := cmd.Args[0]
	loc := Location{URI: b.uri, Line: line, Col: cmd.Col + len(cmd.Name) + 1, EndCol: cmd.Col + len(cmd.Name) + 1 + len(arg0)}

	if strings.HasPrefix(arg0, ":") {
		name := arg0[1:]
		b.pendingLabelRefs = append(b.pendingLabelRefs, pendingLabelRef{
			target: name,
			loc:    loc,
			kind:   RefCall,
		})
		if strings.Contains(name, "%") {
			b.hasDynamicJumps = true
		}
	} else {
		pathLower := strings.ToLower(arg0)
		if strings.HasSuffix(pathLower, ".bat") || strings.HasSuffix(pathLower, ".cmd") {
			b.callTargets = append(b.callTargets, arg0)
		}
	}
}

func (b *builder) buildIf(node *parser.IfNode) {
	b.Build(node.Then)
	if node.Else != nil {
		b.Build(node.Else)
	}
}

func (b *builder) buildFor(node *parser.ForNode) {
	scope := b.pushScope(ScopeFor, node.Line)
	scope.EndLine = nodeLastLine(node.Do)

	if node.Variable != "" {
		char := node.Variable[0]
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
			defCol := node.VarCol - 2
			if defCol < 0 {
				defCol = node.VarCol
			}
			loc := Location{URI: b.uri, Line: node.VarLine, Col: defCol, EndCol: defCol + 3}
			sym := b.result.Symbols.DefineForVar(node.Variable, loc, scope)
			sym.AddRef(loc, RefDefinition)

			additionalVars := extractAdditionalForVars(node.Options, char)
			for _, addVar := range additionalVars {
				addLoc := Location{URI: b.uri, Line: node.VarLine, Col: defCol, EndCol: defCol + 3}
				addSym := b.result.Symbols.DefineForVar(string(addVar), addLoc, scope)
				addSym.AddRef(addLoc, RefDefinition)
			}
		}
	}

	b.Build(node.Do)
	b.popScope(scope.EndLine)
}

func extractAdditionalForVars(options string, baseVar byte) []byte {
	var additionalVars []byte

	tokensMatch := strings.Split(options, "tokens=")
	if len(tokensMatch) < 2 {
		return nil
	}

	afterTokens := tokensMatch[1]
	end := strings.IndexAny(afterTokens, " \t\"'")
	if end > 0 {
		afterTokens = afterTokens[:end]
	}

	tokenCount := 1
	for _, part := range strings.Split(afterTokens, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) == 2 {
				start := 0
				end := 0
				for _, c := range rangeParts[0] {
					if c >= '0' && c <= '9' {
						start = start*10 + int(c-'0')
					}
				}
				for _, c := range rangeParts[1] {
					if c >= '0' && c <= '9' {
						end = end*10 + int(c-'0')
					}
				}
				if end > tokenCount {
					tokenCount = end
				}
			}
		} else {
			num := 0
			for _, c := range part {
				if c >= '0' && c <= '9' {
					num = num*10 + int(c-'0')
				}
			}
			if num > tokenCount {
				tokenCount = num
			}
		}
	}

	for i := 2; i <= tokenCount; i++ {
		nextVar := baseVar + byte(i-1)
		if nextVar > 'z' && baseVar >= 'a' && baseVar <= 'z' {
			nextVar = 'a' + byte(i-1) - ('z' - baseVar + 1)
		} else if nextVar > 'Z' && baseVar >= 'A' && baseVar <= 'Z' {
			nextVar = 'A' + byte(i-1) - ('Z' - baseVar + 1)
		}
		if (nextVar >= 'a' && nextVar <= 'z') || (nextVar >= 'A' && nextVar <= 'Z') {
			additionalVars = append(additionalVars, nextVar)
		}
	}

	return additionalVars
}

func (b *builder) buildBlock(block *parser.Block) {
	scope := b.pushScope(ScopeBlock, block.Line)
	scope.EndLine = block.EndLine

	for _, child := range block.Body {
		b.Build(child)
	}

	b.popScope(scope.EndLine)
}

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
		return n.EndLine
	case *parser.PipeNode:
		return nodeLastLine(n.Right)
	case *parser.BinaryNode:
		return nodeLastLine(n.Right)
	}
	return 0
}

func (b *builder) ComputeDiagnostics() {
	b.resolveLabelRefs()

	definedLabels := make(map[string]bool)
	for _, sym := range b.result.Symbols.Labels {
		definedLabels[sym.Name] = true
	}

	var unrefs []string
	for _, sym := range b.result.Symbols.Labels {
		if sym.RefCount() == 0 {
			unrefs = append(unrefs, sym.Name)
		}
	}

	if !b.hasDynamicJumps {
		for _, name := range unrefs {
			sym := b.result.Symbols.Labels[name]
			b.result.Diagnostics = append(b.result.Diagnostics, Diagnostic{
				Location: Location{
					URI:    b.uri,
					Line:   sym.Definition.Line,
					Col:    sym.Definition.Col,
					EndCol: sym.Definition.Col + len(sym.Name),
				},
				Message:  "Unused label: " + name,
				Severity: SeverityHint,
			})
		}
	}
}

func (b *builder) resolveLabelRefs() {
	for _, ref := range b.pendingLabelRefs {
		if strings.ToLower(ref.target) == "eof" && b.result.Symbols.ResolveLabel(ref.target) == nil {
			continue
		}
		if sym := b.result.Symbols.ResolveLabel(ref.target); sym != nil {
			sym.AddRef(ref.loc, ref.kind)
		}
	}
}
