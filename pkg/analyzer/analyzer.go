package analyzer

import (
	"fmt"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/lexer"
	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

type Result struct {
	URI                     string
	Symbols                 *SymbolTable
	Diagnostics             []Diagnostic
	DelayedExpansionEnabled bool
	HasDynamicJumps         bool
	CallTargets             []string
}

type Analyzer struct {
	builtinVars map[string]bool
	builtinCmds map[string]bool
}

func NewAnalyzer() *Analyzer {
	return &Analyzer{
		builtinVars: processor.BuiltinVarNames(),
		builtinCmds: makeBuiltinCommands(),
	}
}

func makeBuiltinCommands() map[string]bool {
	cmds := make(map[string]bool)
	for _, kw := range lexer.Keywords {
		cmds[strings.ToLower(kw)] = true
	}
	for _, name := range []string{
		"echo", "set", "cd", "chdir", "type", "cls", "title", "ver", "pause",
		"color", "pushd", "popd", "mkdir", "md", "rmdir", "rd", "del", "erase",
		"copy", "move", "dir", "break", "path", "prompt", "verify", "vol",
		"assoc", "ftype", "mklink", "ren", "rename", "more", "start",
		"hostname", "whoami", "timeout", "sort", "where", "tree", "find",
		"findstr", "xcopy", "robocopy", "time", "date", "pkzip", "pkunzip",
		"pkzipc",
	} {
		cmds[name] = true
	}
	return cmds
}

func (a *Analyzer) Analyze(uri, content string) *Result {
	result := &Result{
		URI:     uri,
		Symbols: NewSymbolTable(uri),
	}

	lines := strings.Split(content, "\n")
	tokens := collectTokens(lines)
	nodes := parser.NewFromTokens(tokens).Parse()

	builder := newBuilder(result, lines, uri)
	for _, node := range nodes {
		builder.Build(node)
	}

	scanVariableRefs(result, lines, uri)
	builder.ComputeDiagnostics()

	result.DelayedExpansionEnabled = builder.delayedExpansionEnabled
	result.HasDynamicJumps = builder.hasDynamicJumps
	result.CallTargets = builder.callTargets

	return result
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
		allTokens = append(allTokens, lexer.Item{Line: i, Type: lexer.TokenNewline, Value: []rune{'\n'}})
	}
	return allTokens
}

func (a *Analyzer) IsBuiltinVar(name string) bool {
	return a.builtinVars[strings.ToUpper(name)]
}

func (a *Analyzer) IsBuiltinCommand(name string) bool {
	return a.builtinCmds[strings.ToLower(name)]
}

func (r *Result) GetDiagnostics() []Diagnostic {
	return r.Diagnostics
}

func (r *Result) DefinitionAt(line, col int) *Location {
	if r.Symbols == nil {
		return nil
	}

	for _, sym := range r.Symbols.Labels {
		if sym.Definition.Line == line && col >= sym.Definition.Col && col <= sym.Definition.Col+len(sym.Name) {
			return &sym.Definition
		}
		for _, ref := range sym.References {
			if ref.Location.Line == line && col >= ref.Location.Col && col <= ref.Location.Col+len(sym.Name) {
				return &sym.Definition
			}
		}
	}

	for _, sym := range r.Symbols.Vars {
		if sym.Definition.Line == line && col >= sym.Definition.Col && col <= sym.Definition.Col+len(sym.Name) {
			return &sym.Definition
		}
		for _, ref := range sym.References {
			if ref.Location.Line == line && col >= ref.Location.Col && col <= ref.Location.Col+len(sym.Name) {
				return &sym.Definition
			}
		}
	}

	for _, sym := range r.Symbols.ForVars {
		if sym.Definition.Line == line && col >= sym.Definition.Col && col <= sym.Definition.Col+2 {
			return &sym.Definition
		}
		for _, ref := range sym.References {
			if ref.Location.Line == line && col >= ref.Location.Col && col <= ref.Location.Col+2 {
				return &sym.Definition
			}
		}
	}

	return nil
}

func (r *Result) ReferencesAt(line, col int, includeDecl bool) []Location {
	if r.Symbols == nil {
		return nil
	}

	sym := r.findSymbolAt(line, col)
	if sym == nil {
		return nil
	}

	var locs []Location
	seen := make(map[string]bool)

	addLoc := func(loc Location) {
		key := fmt.Sprintf("%d:%d", loc.Line, loc.Col)
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

func (r *Result) findSymbolAt(line, col int) *Symbol {
	for _, sym := range r.Symbols.Labels {
		if sym.Definition.Line == line && col >= sym.Definition.Col && col <= sym.Definition.Col+len(sym.Name) {
			return sym
		}
		for _, ref := range sym.References {
			if ref.Location.Line == line && col >= ref.Location.Col && col <= ref.Location.Col+len(sym.Name) {
				return sym
			}
		}
	}

	for _, sym := range r.Symbols.Vars {
		if sym.Definition.Line == line && col >= sym.Definition.Col && col <= sym.Definition.Col+len(sym.Name) {
			return sym
		}
		for _, ref := range sym.References {
			if ref.Location.Line == line && col >= ref.Location.Col && col <= ref.Location.Col+len(sym.Name) {
				return sym
			}
		}
	}

	for _, sym := range r.Symbols.ForVars {
		if sym.Definition.Line == line && col >= sym.Definition.Col && col <= sym.Definition.Col+2 {
			return sym
		}
		for _, ref := range sym.References {
			if ref.Location.Line == line && col >= ref.Location.Col && col <= ref.Location.Col+2 {
				return sym
			}
		}
	}

	return nil
}

func (r *Result) CompletionsAt(line, col int, lineBefore string) []Completion {
	return nil
}

func (r *Result) HoverAt(line, col int) *Hover {
	if r.Symbols == nil {
		return nil
	}

	sym := r.findSymbolAt(line, col)
	if sym == nil {
		return nil
	}

	var contents string
	switch sym.Kind {
	case SymbolLabel:
		contents = "label :" + sym.Name
		if sym.InferredValue != "" {
			contents += "\nValue: " + sym.InferredValue
		}
	case SymbolVariable:
		contents = "variable " + sym.Name
		if sym.InferredValue != "" {
			contents += "\nValue: " + sym.InferredValue
		}
	case SymbolForVar:
		contents = "FOR loop variable %%" + sym.Name
	}

	if contents == "" {
		return nil
	}

	return &Hover{
		Contents: contents,
		Range:    sym.Definition,
	}
}

func (r *Result) RenameAt(line, col int, newName string) []TextEdit {
	if r.Symbols == nil {
		return nil
	}

	sym := r.findSymbolAt(line, col)
	if sym == nil {
		return nil
	}

	var edits []TextEdit
	seen := make(map[string]bool)

	addEdit := func(loc Location, text string) {
		key := fmt.Sprintf("%d:%d", loc.Line, loc.Col)
		if !seen[key] {
			seen[key] = true
			edits = append(edits, TextEdit{
				Location: loc,
				NewText:  text,
			})
		}
	}

	switch sym.Kind {
	case SymbolLabel:
		addEdit(sym.Definition, newName)
		for _, ref := range sym.References {
			addEdit(ref.Location, newName)
		}
	case SymbolVariable:
		addEdit(sym.Definition, newName)
		for _, ref := range sym.References {
			addEdit(ref.Location, newName)
		}
	case SymbolForVar:
		upperName := strings.ToUpper(newName)
		if len(upperName) == 1 && upperName[0] >= 'A' && upperName[0] <= 'Z' {
			addEdit(sym.Definition, "%%"+upperName)
			for _, ref := range sym.References {
				addEdit(ref.Location, "%%"+upperName)
			}
		}
	}

	return edits
}
