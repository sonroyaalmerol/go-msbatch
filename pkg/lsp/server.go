package lsp

import (
	"strconv"

	"github.com/sonroyaalmerol/go-msbatch/pkg/executor/tools"
	"github.com/sonroyaalmerol/go-msbatch/pkg/lexer"
	"github.com/sonroyaalmerol/go-msbatch/pkg/lsp/store"
	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
)

const lsName = "msbatch-lsp"

type Server struct {
	handler *protocol.Handler
	server  *server.Server
	store   *store.Store
	context *glsp.Context
}

func NewServer(debug bool) *Server {
	s := &Server{
		store: store.New(),
	}

	h := &protocol.Handler{
		Initialize:  s.initialize,
		Initialized: s.initialized,
		Shutdown:    s.shutdown,
		Exit:        s.exit,
		TextDocumentDidOpen: func(context *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
			s.context = context
			return s.didOpen(context, params)
		},
		TextDocumentDidChange: func(context *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
			s.context = context
			return s.didChange(context, params)
		},
		TextDocumentDidClose: func(context *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
			s.context = context
			return s.didClose(params)
		},
		TextDocumentHover:              s.hover,
		TextDocumentDefinition:         s.definition,
		TextDocumentReferences:         s.references,
		TextDocumentCompletion:         s.completion,
		TextDocumentSemanticTokensFull: s.semanticTokensFull,
		TextDocumentDocumentSymbol:     s.documentSymbol,
		TextDocumentFoldingRange:       s.foldingRange,
		TextDocumentPrepareRename:      s.prepareRename,
		TextDocumentRename:             s.rename,
		TextDocumentCodeAction:         s.codeAction,
	}
	s.handler = h
	s.server = server.NewServer(h, lsName, debug)

	return s
}

func (s *Server) RunStdio() error {
	return s.server.RunStdio()
}

func (s *Server) initialize(context *glsp.Context, params *protocol.InitializeParams) (any, error) {
	s.context = context
	legend := getSemanticTokensLegend()
	capabilities := &protocol.ServerCapabilities{
		TextDocumentSync: protocol.TextDocumentSyncKindFull,
		HoverProvider:    true,
		DefinitionProvider: &protocol.DefinitionOptions{
			WorkDoneProgressOptions: protocol.WorkDoneProgressOptions{},
		},
		ReferencesProvider: &protocol.ReferenceOptions{
			WorkDoneProgressOptions: protocol.WorkDoneProgressOptions{},
		},
		CompletionProvider: &protocol.CompletionOptions{
			TriggerCharacters: []string{"%", "!", ":"},
		},
		SemanticTokensProvider: protocol.SemanticTokensOptions{
			Legend: *legend,
		},
		FoldingRangeProvider: true,
		RenameProvider:       true,
		CodeActionProvider: &protocol.CodeActionOptions{
			CodeActionKinds: []protocol.CodeActionKind{
				protocol.CodeActionKindQuickFix,
				protocol.CodeActionKindRefactor,
			},
		},
	}

	return protocol.InitializeResult{
		Capabilities: *capabilities,
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name:    lsName,
			Version: ptr("0.1.0"),
		},
	}, nil
}

func (s *Server) initialized(context *glsp.Context, params *protocol.InitializedParams) error {
	return nil
}

func (s *Server) shutdown(context *glsp.Context) error {
	return nil
}

func (s *Server) exit(context *glsp.Context) error {
	return nil
}

func ptr[T any](v T) *T {
	return &v
}

func (s *Server) didOpen(context *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
	uri := params.TextDocument.URI
	version := int(params.TextDocument.Version)
	text := params.TextDocument.Text
	doc := s.store.Put(uri, version, text)
	s.publishDiagnostics(context, uri, version, doc.Diags)
	return nil
}

func (s *Server) didChange(context *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	uri := params.TextDocument.URI
	version := int(params.TextDocument.Version)
	for _, change := range params.ContentChanges {
		if c, ok := change.(protocol.TextDocumentContentChangeEventWhole); ok {
			doc := s.store.Put(uri, version, c.Text)
			s.publishDiagnostics(context, uri, version, doc.Diags)
		}
	}
	return nil
}

func (s *Server) didClose(params *protocol.DidCloseTextDocumentParams) error {
	s.store.Delete(params.TextDocument.URI)
	return nil
}

func (s *Server) publishDiagnostics(context *glsp.Context, uri string, version int, diags []parser.Diagnostic) {
	lspDiags := make([]protocol.Diagnostic, len(diags))
	for i, d := range diags {
		severity := protocol.DiagnosticSeverityError
		switch d.Severity {
		case "warning":
			severity = protocol.DiagnosticSeverityWarning
		case "information":
			severity = protocol.DiagnosticSeverityInformation
		case "hint":
			severity = protocol.DiagnosticSeverityHint
		}
		lspDiags[i] = protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{Line: protocol.UInteger(d.Line), Character: protocol.UInteger(d.Col)},
				End:   protocol.Position{Line: protocol.UInteger(d.EndLine), Character: protocol.UInteger(d.EndCol)},
			},
			Severity: &severity,
			Source:   ptr("msbatch"),
			Message:  d.Message,
		}
	}

	context.Notify(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
		URI:         uri,
		Version:     ptr(protocol.UInteger(version)),
		Diagnostics: lspDiags,
	})
}

func (s *Server) hover(context *glsp.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	doc := s.store.Get(params.TextDocument.URI)
	if doc == nil || doc.Analysis == nil {
		return nil, nil
	}

	line := int(params.Position.Line)
	col := int(params.Position.Character)

	if label := doc.Analysis.GetLabelAt(line, col); label != nil {
		msg := "**Label:** `" + label.Name + "`"
		if len(label.References) > 0 {
			msg += "\n\n" + pluralize(len(label.References), "reference")
		}
		return &protocol.Hover{
			Contents: protocol.MarkupContent{
				Kind:  protocol.MarkupKindMarkdown,
				Value: msg,
			},
		}, nil
	}

	if v := doc.Analysis.GetVariableAt(line, col); v != nil {
		msg := "**Variable:** `" + v.Name + "`"
		if v.Value != "" {
			msg += "\n\n**Value:** `" + v.Value + "`"
		}
		return &protocol.Hover{
			Contents: protocol.MarkupContent{
				Kind:  protocol.MarkupKindMarkdown,
				Value: msg,
			},
		}, nil
	}

	if cmdDoc := s.getCommandHover(doc, line, col); cmdDoc != "" {
		return &protocol.Hover{
			Contents: protocol.MarkupContent{
				Kind:  protocol.MarkupKindMarkdown,
				Value: cmdDoc,
			},
		}, nil
	}

	return nil, nil
}

func (s *Server) getCommandHover(doc *store.Document, line, col int) string {
	lines := splitLines(doc.Text)
	if line >= len(lines) {
		return ""
	}
	l := lines[line]

	start := col
	for start > 0 && (l[start-1] != ' ' && l[start-1] != '\t') {
		start--
	}

	for start < len(l) && (l[start] == ' ' || l[start] == '\t') {
		start++
	}

	end := start
	for end < len(l) && l[end] != ' ' && l[end] != '\t' && l[end] != '\r' && l[end] != '\n' {
		end++
	}

	if start >= end {
		return ""
	}

	cmdName := stringsToUpper(l[start:end])
	return tools.GetCommandDocumentation(cmdName)
}

func pluralize(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}

func (s *Server) definition(context *glsp.Context, params *protocol.DefinitionParams) (any, error) {
	doc := s.store.Get(params.TextDocument.URI)
	if doc == nil || doc.Analysis == nil {
		return nil, nil
	}

	line := int(params.Position.Line)
	col := int(params.Position.Character)

	if label := doc.Analysis.GetLabelAt(line, col); label != nil {
		return protocol.Location{
			URI: params.TextDocument.URI,
			Range: protocol.Range{
				Start: protocol.Position{Line: protocol.UInteger(label.Definition.Start.Line), Character: protocol.UInteger(label.Definition.Start.Col)},
				End:   protocol.Position{Line: protocol.UInteger(label.Definition.End.Line), Character: protocol.UInteger(label.Definition.End.Col)},
			},
		}, nil
	}

	if v := doc.Analysis.GetVariableAt(line, col); v != nil {
		return protocol.Location{
			URI: params.TextDocument.URI,
			Range: protocol.Range{
				Start: protocol.Position{Line: protocol.UInteger(v.Definition.Start.Line), Character: protocol.UInteger(v.Definition.Start.Col)},
				End:   protocol.Position{Line: protocol.UInteger(v.Definition.End.Line), Character: protocol.UInteger(v.Definition.End.Col)},
			},
		}, nil
	}

	return nil, nil
}

func (s *Server) references(context *glsp.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	doc := s.store.Get(params.TextDocument.URI)
	if doc == nil || doc.Analysis == nil {
		return nil, nil
	}

	line := int(params.Position.Line)
	col := int(params.Position.Character)
	includeDecl := params.Context.IncludeDeclaration

	var locations []protocol.Location

	if label := doc.Analysis.GetLabelAt(line, col); label != nil {
		if includeDecl && label.Definition.Start.Line >= 0 {
			locations = append(locations, protocol.Location{
				URI: params.TextDocument.URI,
				Range: protocol.Range{
					Start: protocol.Position{Line: protocol.UInteger(label.Definition.Start.Line), Character: protocol.UInteger(label.Definition.Start.Col)},
					End:   protocol.Position{Line: protocol.UInteger(label.Definition.End.Line), Character: protocol.UInteger(label.Definition.End.Col)},
				},
			})
		}
		for _, ref := range label.References {
			locations = append(locations, protocol.Location{
				URI: params.TextDocument.URI,
				Range: protocol.Range{
					Start: protocol.Position{Line: protocol.UInteger(ref.Start.Line), Character: protocol.UInteger(ref.Start.Col)},
					End:   protocol.Position{Line: protocol.UInteger(ref.End.Line), Character: protocol.UInteger(ref.End.Col)},
				},
			})
		}
		return locations, nil
	}

	if v := doc.Analysis.GetVariableAt(line, col); v != nil {
		if includeDecl && v.Definition.Start.Line >= 0 {
			locations = append(locations, protocol.Location{
				URI: params.TextDocument.URI,
				Range: protocol.Range{
					Start: protocol.Position{Line: protocol.UInteger(v.Definition.Start.Line), Character: protocol.UInteger(v.Definition.Start.Col)},
					End:   protocol.Position{Line: protocol.UInteger(v.Definition.End.Line), Character: protocol.UInteger(v.Definition.End.Col)},
				},
			})
		}
		for _, ref := range v.References {
			locations = append(locations, protocol.Location{
				URI: params.TextDocument.URI,
				Range: protocol.Range{
					Start: protocol.Position{Line: protocol.UInteger(ref.Range.Start.Line), Character: protocol.UInteger(ref.Range.Start.Col)},
					End:   protocol.Position{Line: protocol.UInteger(ref.Range.End.Line), Character: protocol.UInteger(ref.Range.End.Col)},
				},
			})
		}
		return locations, nil
	}

	return nil, nil
}

func (s *Server) completion(context *glsp.Context, params *protocol.CompletionParams) (any, error) {
	doc := s.store.Get(params.TextDocument.URI)
	if doc == nil {
		return nil, nil
	}

	line := int(params.Position.Line)
	col := int(params.Position.Character)

	var items []protocol.CompletionItem

	if col > 0 {
		prefix := getLinePrefix(doc.Text, line, col)
		items = s.getCompletionsForContext(prefix, doc)
	} else {
		items = s.getCommandCompletions(doc)
	}

	return items, nil
}

func getLinePrefix(text string, line, col int) string {
	lines := splitLines(text)
	if line >= len(lines) {
		return ""
	}
	l := lines[line]
	if col > len(l) {
		col = len(l)
	}
	return l[:col]
}

func splitLines(text string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			lines = append(lines, text[start:i])
			start = i + 1
		} else if text[i] == '\r' {
			if i+1 < len(text) && text[i+1] == '\n' {
				continue
			}
			lines = append(lines, text[start:i])
			start = i + 1
		}
	}
	if start < len(text) {
		lines = append(lines, text[start:])
	}
	if len(text) > 0 && (text[len(text)-1] == '\n' || text[len(text)-1] == '\r') {
		lines = append(lines, "")
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}

func (s *Server) getCompletionsForContext(prefix string, doc *store.Document) []protocol.CompletionItem {
	trimmed := stringsTrimRight(prefix, " \t")

	if stringsHasSuffix(trimmed, "GOTO ") || stringsHasSuffix(trimmed, "goto ") {
		return s.getLabelCompletions(doc, "")
	}

	if stringsHasSuffix(trimmed, "CALL :") || stringsHasSuffix(trimmed, "call :") {
		return s.getLabelCompletions(doc, ":")
	}

	if lastPercent := findLastPercent(trimmed); lastPercent >= 0 {
		afterPercent := trimmed[lastPercent:]
		if len(afterPercent) >= 1 && (afterPercent[0] == '%' || afterPercent[0] == '!') {
			return s.getVariableCompletions(doc, afterPercent[1:])
		}
	}

	return s.getCommandCompletions(doc)
}

func stringsTrimRight(s, cutset string) string {
	for len(s) > 0 {
		found := false
		for i := 0; i < len(cutset); i++ {
			if s[len(s)-1] == cutset[i] {
				s = s[:len(s)-1]
				found = true
				break
			}
		}
		if !found {
			break
		}
	}
	return s
}

func stringsHasSuffix(s, suffix string) bool {
	if len(suffix) > len(s) {
		return false
	}
	return s[len(s)-len(suffix):] == suffix
}

func findLastPercent(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '%' || s[i] == '!' {
			return i
		}
	}
	return -1
}

func (s *Server) getLabelCompletions(doc *store.Document, prefix string) []protocol.CompletionItem {
	var items []protocol.CompletionItem
	if doc.Analysis == nil {
		return items
	}
	for name, label := range doc.Analysis.Labels {
		if label.Definition.Start.Line >= 0 {
			items = append(items, protocol.CompletionItem{
				Label:  prefix + name,
				Kind:   ptr(protocol.CompletionItemKindFunction),
				Detail: ptr("Label"),
			})
		}
	}
	return items
}

func (s *Server) getVariableCompletions(doc *store.Document, prefix string) []protocol.CompletionItem {
	var items []protocol.CompletionItem
	if doc.Analysis == nil {
		return items
	}
	upperPrefix := stringsToUpper(prefix)
	for name := range doc.Analysis.Variables {
		if len(prefix) == 0 || stringsHasPrefix(stringsToUpper(name), upperPrefix) {
			items = append(items, protocol.CompletionItem{
				Label:  name,
				Kind:   ptr(protocol.CompletionItemKindVariable),
				Detail: ptr("Variable"),
			})
		}
	}
	return items
}

func stringsHasPrefix(s, prefix string) bool {
	if len(prefix) > len(s) {
		return false
	}
	return s[:len(prefix)] == prefix
}

func stringsToUpper(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'a' && b[i] <= 'z' {
			b[i] -= 'a' - 'A'
		}
	}
	return string(b)
}

func (s *Server) getCommandCompletions(doc *store.Document) []protocol.CompletionItem {
	commands := tools.GetPrimaryCommandNames()
	items := make([]protocol.CompletionItem, len(commands))
	for i, cmd := range commands {
		items[i] = protocol.CompletionItem{
			Label:  cmd,
			Kind:   ptr(protocol.CompletionItemKindKeyword),
			Detail: ptr("Command"),
		}
	}
	return items
}

func (s *Server) semanticTokensFull(context *glsp.Context, params *protocol.SemanticTokensParams) (*protocol.SemanticTokens, error) {
	doc := s.store.Get(params.TextDocument.URI)
	if doc == nil {
		return nil, nil
	}

	legend := getSemanticTokensLegend()
	data := encodeSemanticTokens(doc.Tokens, legend)

	return &protocol.SemanticTokens{
		Data: data,
	}, nil
}

func getSemanticTokensLegend() *protocol.SemanticTokensLegend {
	return &protocol.SemanticTokensLegend{
		TokenTypes: []string{
			"keyword",
			"variable",
			"parameter",
			"label",
			"string",
			"comment",
			"operator",
			"number",
			"macro",
		},
		TokenModifiers: []string{},
	}
}

func encodeSemanticTokens(tokens []lexer.Item, legend *protocol.SemanticTokensLegend) []protocol.UInteger {
	var data []protocol.UInteger
	prevLine := 0
	prevChar := 0

	for _, tok := range tokens {
		tokenType := mapTokenType(tok.Type, legend)
		if tokenType < 0 {
			continue
		}

		line := protocol.UInteger(tok.Line)
		char := protocol.UInteger(tok.Col)
		length := protocol.UInteger(len(tok.Value))

		deltaLine := line - protocol.UInteger(prevLine)
		var deltaChar protocol.UInteger
		if deltaLine == 0 {
			deltaChar = char - protocol.UInteger(prevChar)
		} else {
			deltaChar = char
		}

		data = append(data, deltaLine, deltaChar, length, protocol.UInteger(tokenType), 0)

		prevLine = int(line)
		prevChar = int(char)
	}

	return data
}

func mapTokenType(tt lexer.TokenType, legend *protocol.SemanticTokensLegend) int {
	var name string
	switch tt {
	case lexer.TokenKeyword:
		name = "keyword"
	case lexer.TokenVariable, lexer.TokenDelayedExpansion:
		name = "variable"
	case lexer.TokenForVar:
		name = "parameter"
	case lexer.TokenLabel:
		name = "label"
	case lexer.TokenStringDouble, lexer.TokenStringSingle, lexer.TokenStringBacktick:
		name = "string"
	case lexer.TokenComment:
		name = "comment"
	case lexer.TokenOperator:
		name = "operator"
	case lexer.TokenNumber:
		name = "number"
	default:
		return -1
	}

	for i, t := range legend.TokenTypes {
		if t == name {
			return i
		}
	}
	return -1
}

func (s *Server) documentSymbol(context *glsp.Context, params *protocol.DocumentSymbolParams) (any, error) {
	doc := s.store.Get(params.TextDocument.URI)
	if doc == nil || doc.Analysis == nil {
		return nil, nil
	}

	var symbols []protocol.DocumentSymbol

	for _, label := range doc.Analysis.Labels {
		symbols = append(symbols, protocol.DocumentSymbol{
			Name: label.Name,
			Kind: protocol.SymbolKindFunction,
			Range: protocol.Range{
				Start: protocol.Position{Line: protocol.UInteger(label.Definition.Start.Line), Character: protocol.UInteger(label.Definition.Start.Col)},
				End:   protocol.Position{Line: protocol.UInteger(label.Definition.End.Line), Character: protocol.UInteger(label.Definition.End.Col)},
			},
			SelectionRange: protocol.Range{
				Start: protocol.Position{Line: protocol.UInteger(label.Definition.Start.Line), Character: protocol.UInteger(label.Definition.Start.Col)},
				End:   protocol.Position{Line: protocol.UInteger(label.Definition.End.Line), Character: protocol.UInteger(label.Definition.End.Col)},
			},
		})
	}

	return symbols, nil
}

func (s *Server) foldingRange(context *glsp.Context, params *protocol.FoldingRangeParams) ([]protocol.FoldingRange, error) {
	doc := s.store.Get(params.TextDocument.URI)
	if doc == nil {
		return nil, nil
	}

	var ranges []protocol.FoldingRange

	for _, node := range doc.Nodes {
		collectFoldingRanges(node, &ranges)
	}

	return ranges, nil
}

func collectFoldingRanges(node parser.Node, ranges *[]protocol.FoldingRange) {
	switch n := node.(type) {
	case *parser.IfNode:
		*ranges = append(*ranges, protocol.FoldingRange{
			StartLine: protocol.UInteger(n.Line),
			EndLine:   protocol.UInteger(n.EndLine),
			Kind:      ptr(string(protocol.FoldingRangeKindRegion)),
		})
		if n.Then != nil {
			collectFoldingRanges(n.Then, ranges)
		}
		if n.Else != nil {
			collectFoldingRanges(n.Else, ranges)
		}
	case *parser.ForNode:
		*ranges = append(*ranges, protocol.FoldingRange{
			StartLine: protocol.UInteger(n.Line),
			EndLine:   protocol.UInteger(n.EndLine),
			Kind:      ptr(string(protocol.FoldingRangeKindRegion)),
		})
		if n.Do != nil {
			collectFoldingRanges(n.Do, ranges)
		}
	case *parser.Block:
		*ranges = append(*ranges, protocol.FoldingRange{
			StartLine: protocol.UInteger(n.Line),
			EndLine:   protocol.UInteger(n.EndLine),
			Kind:      ptr(string(protocol.FoldingRangeKindRegion)),
		})
		for _, child := range n.Body {
			collectFoldingRanges(child, ranges)
		}
	case *parser.BinaryNode:
		collectFoldingRanges(n.Left, ranges)
		collectFoldingRanges(n.Right, ranges)
	case *parser.PipeNode:
		collectFoldingRanges(n.Left, ranges)
		collectFoldingRanges(n.Right, ranges)
	}
}

func (s *Server) prepareRename(context *glsp.Context, params *protocol.PrepareRenameParams) (any, error) {
	doc := s.store.Get(params.TextDocument.URI)
	if doc == nil || doc.Analysis == nil {
		return nil, nil
	}

	line := int(params.Position.Line)
	col := int(params.Position.Character)

	if label := doc.Analysis.GetLabelAt(line, col); label != nil {
		if label.Definition.Start.Line < 0 {
			return nil, nil
		}
		return protocol.Range{
			Start: protocol.Position{Line: protocol.UInteger(label.Definition.Start.Line), Character: protocol.UInteger(label.Definition.Start.Col)},
			End:   protocol.Position{Line: protocol.UInteger(label.Definition.End.Line), Character: protocol.UInteger(label.Definition.End.Col)},
		}, nil
	}

	if v := doc.Analysis.GetVariableAt(line, col); v != nil {
		return nil, nil
	}

	return nil, nil
}

func (s *Server) rename(context *glsp.Context, params *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
	doc := s.store.Get(params.TextDocument.URI)
	if doc == nil || doc.Analysis == nil {
		return nil, nil
	}

	line := int(params.Position.Line)
	col := int(params.Position.Character)
	newName := params.NewName

	edits := make(map[protocol.DocumentUri][]protocol.TextEdit)

	if label := doc.Analysis.GetLabelAt(line, col); label != nil {
		if label.Definition.Start.Line >= 0 {
			edits[params.TextDocument.URI] = append(edits[params.TextDocument.URI], protocol.TextEdit{
				Range: protocol.Range{
					Start: protocol.Position{Line: protocol.UInteger(label.Definition.Start.Line), Character: protocol.UInteger(label.Definition.Start.Col)},
					End:   protocol.Position{Line: protocol.UInteger(label.Definition.End.Line), Character: protocol.UInteger(label.Definition.End.Col)},
				},
				NewText: newName,
			})
		}
		for _, ref := range label.References {
			edits[params.TextDocument.URI] = append(edits[params.TextDocument.URI], protocol.TextEdit{
				Range: protocol.Range{
					Start: protocol.Position{Line: protocol.UInteger(ref.Start.Line), Character: protocol.UInteger(ref.Start.Col)},
					End:   protocol.Position{Line: protocol.UInteger(ref.End.Line), Character: protocol.UInteger(ref.End.Col)},
				},
				NewText: newName,
			})
		}
	}

	return &protocol.WorkspaceEdit{
		Changes: edits,
	}, nil
}

func (s *Server) codeAction(context *glsp.Context, params *protocol.CodeActionParams) (any, error) {
	doc := s.store.Get(params.TextDocument.URI)
	if doc == nil || doc.Analysis == nil {
		return nil, nil
	}

	var actions []protocol.CodeAction

	for _, d := range params.Context.Diagnostics {
		if d.Code == nil || d.Code.Value == nil {
			continue
		}

		code := ""
		switch v := d.Code.Value.(type) {
		case string:
			code = v
		}

		switch code {
		case "undefined-label":
			action := s.createUndefinedLabelCodeAction(params.TextDocument.URI, d)
			if action != nil {
				actions = append(actions, *action)
			}
		case "missing-endlocal":
			action := s.createMissingEndlocalCodeAction(params.TextDocument.URI, d)
			if action != nil {
				actions = append(actions, *action)
			}
		}
	}

	return actions, nil
}

func (s *Server) createUndefinedLabelCodeAction(uri string, d protocol.Diagnostic) *protocol.CodeAction {
	labelName := d.Message
	if len(labelName) > 17 && labelName[:17] == "undefined label: " {
		labelName = labelName[17:]
	} else {
		return nil
	}

	kind := protocol.CodeActionKindQuickFix
	return &protocol.CodeAction{
		Title: "Create label :" + labelName,
		Kind:  &kind,
		Diagnostics: []protocol.Diagnostic{
			d,
		},
		Edit: &protocol.WorkspaceEdit{
			Changes: map[protocol.DocumentUri][]protocol.TextEdit{
				uri: {
					{
						Range: protocol.Range{
							Start: protocol.Position{Line: d.Range.End.Line + 1, Character: 0},
							End:   protocol.Position{Line: d.Range.End.Line + 1, Character: 0},
						},
						NewText: "\n:" + labelName + "\n",
					},
				},
			},
		},
	}
}

func (s *Server) createMissingEndlocalCodeAction(uri string, d protocol.Diagnostic) *protocol.CodeAction {
	kind := protocol.CodeActionKindQuickFix
	return &protocol.CodeAction{
		Title: "Add ENDLOCAL",
		Kind:  &kind,
		Diagnostics: []protocol.Diagnostic{
			d,
		},
		Edit: &protocol.WorkspaceEdit{
			Changes: map[protocol.DocumentUri][]protocol.TextEdit{
				uri: {
					{
						Range: protocol.Range{
							Start: protocol.Position{Line: d.Range.End.Line + 1, Character: 0},
							End:   protocol.Position{Line: d.Range.End.Line + 1, Character: 0},
						},
						NewText: "ENDLOCAL\n",
					},
				},
			},
		},
	}
}
