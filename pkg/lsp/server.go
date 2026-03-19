package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/sonroyaalmerol/go-msbatch/pkg/analyzer"
	"github.com/sonroyaalmerol/go-msbatch/pkg/executor"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
)

const serverName = "msbatch-lsp"

var allCommands = func() []string {
	names := make([]string, 0, len(batchKeywords))
	for name := range batchKeywords {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}()

type Server struct {
	mu   sync.RWMutex
	docs map[string]*Document
}

func NewServer() *Server {
	return &Server{docs: make(map[string]*Document)}
}

func (s *Server) Run() error {
	handler := protocol.Handler{
		Initialize:                     s.initialize,
		Initialized:                    s.initialized,
		Shutdown:                       s.shutdown,
		TextDocumentDidOpen:            s.didOpen,
		TextDocumentDidChange:          s.didChange,
		TextDocumentDidClose:           s.didClose,
		TextDocumentHover:              s.hover,
		TextDocumentCompletion:         s.completion,
		TextDocumentDocumentSymbol:     s.documentSymbol,
		TextDocumentDefinition:         s.definition,
		TextDocumentReferences:         s.references,
		TextDocumentPrepareRename:      s.prepareRename,
		TextDocumentRename:             s.rename,
		TextDocumentCodeLens:           s.codeLens,
		TextDocumentSemanticTokensFull: s.semanticTokensFull,
		TextDocumentFoldingRange:       s.foldingRange,
		TextDocumentCodeAction:         s.codeAction,
		WorkspaceDidChangeWatchedFiles: s.didChangeWatchedFiles,
		WorkspaceSymbol:                s.workspaceSymbol,
	}
	srv := server.NewServer(&handler, serverName, false)
	return srv.RunStdio()
}

func (s *Server) store(uri, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := NewAnalyzer().Analyze(uri, content)
	s.docs[uri] = &Document{
		Content: content,
		Result:  result,
	}
}

func (s *Server) load(uri string) (*Document, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.docs[uri]
	return d, ok
}

func (s *Server) remove(uri string) {
	s.mu.Lock()
	delete(s.docs, uri)
	s.mu.Unlock()
}

func ptr[T any](v T) *T { return &v }

func (s *Server) publishDiagnostics(ctx *glsp.Context, uri string) {
	s.mu.RLock()
	doc, ok := s.docs[uri]
	s.mu.RUnlock()

	if !ok || doc.Result == nil {
		return
	}

	lspDiags := make([]protocol.Diagnostic, 0, len(doc.Result.Diagnostics))
	for _, d := range doc.Result.Diagnostics {
		endChar := uint32(10000)
		if d.Location.EndCol > 0 {
			endChar = uint32(d.Location.EndCol)
		}
		lspDiags = append(lspDiags, protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(d.Location.Line), Character: uint32(d.Location.Col)},
				End:   protocol.Position{Line: uint32(d.Location.Line), Character: endChar},
			},
			Severity: ptr(protocol.DiagnosticSeverity(d.Severity + 1)),
			Source:   ptr(serverName),
			Message:  d.Message,
		})
	}

	ctx.Notify("textDocument/publishDiagnostics", &protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: lspDiags,
	})
}

func lineAt(content string, lineIdx int) string {
	lines := strings.Split(content, "\n")
	if lineIdx < 0 || lineIdx >= len(lines) {
		return ""
	}
	return strings.TrimRight(lines[lineIdx], "\r")
}

func (s *Server) initialize(ctx *glsp.Context, params *protocol.InitializeParams) (any, error) {
	var roots []string
	if params.RootURI != nil {
		roots = append(roots, *params.RootURI)
	}
	if params.WorkspaceFolders != nil {
		for _, f := range params.WorkspaceFolders {
			roots = append(roots, f.URI)
		}
	}

	for _, root := range roots {
		if after, ok := strings.CutPrefix(root, "file://"); ok {
			path := after
			_ = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if !d.IsDir() {
					ext := strings.ToLower(filepath.Ext(p))
					if ext == ".bat" || ext == ".cmd" {
						content, err := os.ReadFile(p)
						if err == nil {
							uri := "file://" + p
							s.store(uri, string(content))
						}
					}
				}
				return nil
			})
		}
	}

	syncKind := protocol.TextDocumentSyncKindFull
	return protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: &protocol.TextDocumentSyncOptions{
				OpenClose: ptr(true),
				Change:    &syncKind,
			},
			HoverProvider:           true,
			CompletionProvider:      &protocol.CompletionOptions{TriggerCharacters: []string{"%", ":", "!"}},
			DocumentSymbolProvider:  true,
			WorkspaceSymbolProvider: true,
			DefinitionProvider:      true,
			ReferencesProvider:      true,
			RenameProvider:          &protocol.RenameOptions{PrepareProvider: ptr(true)},
			CodeLensProvider:        &protocol.CodeLensOptions{},
			SemanticTokensProvider: &protocol.SemanticTokensOptions{
				Legend: protocol.SemanticTokensLegend{
					TokenTypes:     SemTokenTypes,
					TokenModifiers: SemTokenModifiers,
				},
				Full: true,
			},
			FoldingRangeProvider: true,
			CodeActionProvider:   &protocol.CodeActionOptions{CodeActionKinds: []protocol.CodeActionKind{protocol.CodeActionKindQuickFix}},
		},
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name:    serverName,
			Version: ptr("0.1.0"),
		},
	}, nil
}

func (s *Server) initialized(ctx *glsp.Context, _ *protocol.InitializedParams) error {
	ctx.Notify("client/registerCapability", &protocol.RegistrationParams{
		Registrations: []protocol.Registration{
			{
				ID:     "workspace/didChangeWatchedFiles",
				Method: "workspace/didChangeWatchedFiles",
				RegisterOptions: protocol.DidChangeWatchedFilesRegistrationOptions{
					Watchers: []protocol.FileSystemWatcher{
						{GlobPattern: "**/*.bat"},
						{GlobPattern: "**/*.cmd"},
						{GlobPattern: "**/*.BAT"},
						{GlobPattern: "**/*.CMD"},
					},
				},
			},
		},
	})
	return nil
}

func (s *Server) didChangeWatchedFiles(_ *glsp.Context, params *protocol.DidChangeWatchedFilesParams) error {
	for _, change := range params.Changes {
		uri := change.URI
		if change.Type == protocol.FileChangeTypeDeleted {
			s.remove(uri)
		} else {
			if after, ok := strings.CutPrefix(uri, "file://"); ok {
				path := after
				content, err := os.ReadFile(path)
				if err == nil {
					s.store(uri, string(content))
				}
			}
		}
	}
	return nil
}

func (s *Server) workspaceSymbol(_ *glsp.Context, params *protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var syms []protocol.SymbolInformation
	query := strings.ToLower(params.Query)

	for uri, doc := range s.docs {
		if doc.Result == nil || doc.Result.Symbols == nil {
			continue
		}
		for _, lbl := range doc.Result.Symbols.Labels {
			if query == "" || strings.Contains(strings.ToLower(lbl.Name), query) {
				syms = append(syms, protocol.SymbolInformation{
					Name: ":" + lbl.Name,
					Kind: protocol.SymbolKindFunction,
					Location: protocol.Location{
						URI:   uri,
						Range: lineRange(uint32(lbl.Definition.Line)),
					},
				})
			}
		}

		for _, v := range doc.Result.Symbols.Vars {
			if query == "" || strings.Contains(strings.ToLower(v.Name), query) {
				syms = append(syms, protocol.SymbolInformation{
					Name: v.Name,
					Kind: protocol.SymbolKindVariable,
					Location: protocol.Location{
						URI:   uri,
						Range: lineRange(uint32(v.Definition.Line)),
					},
				})
			}
		}
	}
	return syms, nil
}

func (s *Server) shutdown(_ *glsp.Context) error {
	return nil
}

func (s *Server) didOpen(ctx *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
	content := params.TextDocument.Text
	uri := string(params.TextDocument.URI)
	s.store(uri, content)
	s.publishDiagnostics(ctx, uri)
	return nil
}

func (s *Server) didChange(ctx *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	if len(params.ContentChanges) == 0 {
		return nil
	}
	var content string
	switch v := params.ContentChanges[len(params.ContentChanges)-1].(type) {
	case protocol.TextDocumentContentChangeEvent:
		content = v.Text
	case protocol.TextDocumentContentChangeEventWhole:
		content = v.Text
	}
	uri := string(params.TextDocument.URI)
	s.store(uri, content)
	s.publishDiagnostics(ctx, uri)
	return nil
}

func (s *Server) didClose(_ *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
	s.remove(string(params.TextDocument.URI))
	return nil
}

func (s *Server) hover(_ *glsp.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	doc, ok := s.load(string(params.TextDocument.URI))
	if !ok {
		return nil, nil
	}

	line := lineAt(doc.Content, int(params.Position.Line))
	word := WordAtPosition(line, int(params.Position.Character))
	if word == "" {
		return nil, nil
	}

	help := executor.CommandHelp(word)
	if help == "" {
		return nil, nil
	}

	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.MarkupKindMarkdown,
			Value: fmt.Sprintf("```\n%s\n```", help),
		},
	}, nil
}

func (s *Server) completion(_ *glsp.Context, params *protocol.CompletionParams) (any, error) {
	doc, ok := s.load(string(params.TextDocument.URI))
	if !ok || doc.Result == nil || doc.Result.Symbols == nil {
		return nil, nil
	}

	line := lineAt(doc.Content, int(params.Position.Line))
	col := min(int(params.Position.Character), len(line))
	lineBefore := line[:col]

	ctx := CompletionContextAt(lineBefore)
	symbols := doc.Result.Symbols

	var items []protocol.CompletionItem
	switch ctx {
	case CompleteCommand:
		prefix := strings.ToLower(WordAtPosition(lineBefore, col))
		for _, name := range allCommands {
			if strings.HasPrefix(name, prefix) {
				kind := protocol.CompletionItemKindKeyword
				help := executor.CommandHelp(name)
				item := protocol.CompletionItem{
					Label: name,
					Kind:  &kind,
				}
				if help != "" {
					item.Documentation = &protocol.MarkupContent{
						Kind:  protocol.MarkupKindMarkdown,
						Value: "```\n" + help + "\n```",
					}
				}
				items = append(items, item)
			}
		}

	case CompleteLabel:
		prefix := labelPrefixFromLine(lineBefore)
		for _, lbl := range symbols.Labels {
			if strings.HasPrefix(lbl.Name, strings.ToLower(prefix)) {
				kind := protocol.CompletionItemKindReference
				items = append(items, protocol.CompletionItem{
					Label:  lbl.Name,
					Kind:   &kind,
					Detail: ptr(fmt.Sprintf("line %d", lbl.Definition.Line+1)),
				})
			}
		}

	case CompleteForVariable:
		cursorLine := int(params.Position.Line)
		prefix := varPrefixFromLine(lineBefore)
		replaceStart := uint32(col - 2 - len(prefix))
		replaceEnd := uint32(col)
		seenForVars := make(map[string]bool)
		for _, v := range symbols.ForVars {
			if v.Definition.Line > cursorLine {
				continue
			}
			scopeEnd := -1
			if v.Scope != nil {
				scopeEnd = v.Scope.EndLine
			}
			if scopeEnd >= 0 && cursorLine > scopeEnd {
				continue
			}
			label := "%%" + v.Name
			if !seenForVars[label] && strings.HasPrefix(strings.ToUpper(v.Name), strings.ToUpper(prefix)) {
				seenForVars[label] = true
				kind := protocol.CompletionItemKindVariable
				items = append(items, protocol.CompletionItem{
					Label: label,
					Kind:  &kind,
					TextEdit: protocol.TextEdit{
						Range: protocol.Range{
							Start: protocol.Position{Line: params.Position.Line, Character: replaceStart},
							End:   protocol.Position{Line: params.Position.Line, Character: replaceEnd},
						},
						NewText: label,
					},
				})
			}
		}

	case CompleteVariable:
		cursorLine := int(params.Position.Line)
		prefix := varPrefixFromLine(lineBefore)
		replaceStart := uint32(col - 1 - len(prefix))
		replaceEnd := uint32(col)
		seenVars := make(map[string]bool)

		s.mu.RLock()
		currentURI := string(params.TextDocument.URI)
		for wUri, wDoc := range s.docs {
			if wDoc.Result == nil || wDoc.Result.Symbols == nil {
				continue
			}
			if wUri != currentURI && !s.isCalled(doc.Result, wUri) {
				continue
			}
			for _, v := range wDoc.Result.Symbols.Vars {
				if !seenVars[v.Name] && strings.HasPrefix(strings.ToUpper(v.Name), strings.ToUpper(prefix)) {
					seenVars[v.Name] = true
					kind := protocol.CompletionItemKindVariable
					items = append(items, protocol.CompletionItem{
						Label:  v.Name,
						Kind:   &kind,
						Detail: ptr(v.InferredValue),
						TextEdit: protocol.TextEdit{
							Range: protocol.Range{
								Start: protocol.Position{Line: params.Position.Line, Character: replaceStart},
								End:   protocol.Position{Line: params.Position.Line, Character: replaceEnd},
							},
							NewText: "%" + v.Name + "%",
						},
					})
				}
			}
		}
		s.mu.RUnlock()

		for _, v := range symbols.ForVars {
			if v.Definition.Line > cursorLine {
				continue
			}
			scopeEnd := -1
			if v.Scope != nil {
				scopeEnd = v.Scope.EndLine
			}
			if scopeEnd >= 0 && cursorLine > scopeEnd {
				continue
			}
			label := "%%" + v.Name
			if !seenVars[label] && strings.HasPrefix(strings.ToUpper(v.Name), strings.ToUpper(prefix)) {
				seenVars[label] = true
				kind := protocol.CompletionItemKindVariable
				items = append(items, protocol.CompletionItem{
					Label: label,
					Kind:  &kind,
					TextEdit: protocol.TextEdit{
						Range: protocol.Range{
							Start: protocol.Position{Line: params.Position.Line, Character: replaceStart},
							End:   protocol.Position{Line: params.Position.Line, Character: replaceEnd},
						},
						NewText: label,
					},
				})
			}
		}

	case CompleteDelayedVariable:
		if !doc.Result.DelayedExpansionEnabled {
			break
		}
		prefix := delayedVarPrefixFromLine(lineBefore)
		replaceStart := uint32(col - 1 - len(prefix))
		replaceEnd := uint32(col)
		seenVars := make(map[string]bool)

		s.mu.RLock()
		currentURI := string(params.TextDocument.URI)
		for wUri, wDoc := range s.docs {
			if wDoc.Result == nil || wDoc.Result.Symbols == nil {
				continue
			}
			if wUri != currentURI && !s.isCalled(doc.Result, wUri) {
				continue
			}
			for _, v := range wDoc.Result.Symbols.Vars {
				if !seenVars[v.Name] && strings.HasPrefix(strings.ToUpper(v.Name), strings.ToUpper(prefix)) {
					seenVars[v.Name] = true
					kind := protocol.CompletionItemKindVariable
					items = append(items, protocol.CompletionItem{
						Label:  v.Name,
						Kind:   &kind,
						Detail: ptr(v.InferredValue),
						TextEdit: protocol.TextEdit{
							Range: protocol.Range{
								Start: protocol.Position{Line: params.Position.Line, Character: replaceStart},
								End:   protocol.Position{Line: params.Position.Line, Character: replaceEnd},
							},
							NewText: "!" + v.Name + "!",
						},
					})
				}
			}
		}
		s.mu.RUnlock()
	}

	return items, nil
}

func (s *Server) isCalled(result *analyzer.Result, targetURI string) bool {
	targetLower := strings.ToLower(targetURI)
	for _, target := range result.CallTargets {
		if strings.HasSuffix(targetLower, strings.ToLower(target)) {
			return true
		}
	}
	return false
}

func (s *Server) documentSymbol(_ *glsp.Context, params *protocol.DocumentSymbolParams) (any, error) {
	doc, ok := s.load(string(params.TextDocument.URI))
	if !ok || doc.Result == nil || doc.Result.Symbols == nil {
		return nil, nil
	}

	symbols := doc.Result.Symbols
	var syms []protocol.DocumentSymbol

	for _, lbl := range symbols.Labels {
		r := lineRange(uint32(lbl.Definition.Line))
		syms = append(syms, protocol.DocumentSymbol{
			Name:           ":" + lbl.Name,
			Kind:           protocol.SymbolKindFunction,
			Range:          r,
			SelectionRange: r,
		})
	}

	for _, v := range symbols.Vars {
		r := lineRange(uint32(v.Definition.Line))
		syms = append(syms, protocol.DocumentSymbol{
			Name:           v.Name,
			Kind:           protocol.SymbolKindVariable,
			Detail:         ptr(v.InferredValue),
			Range:          r,
			SelectionRange: r,
		})
	}

	return syms, nil
}

func (s *Server) definition(_ *glsp.Context, params *protocol.DefinitionParams) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	uri := string(params.TextDocument.URI)
	loc, found := DefinitionAt(s.docs, uri, int(params.Position.Line), int(params.Position.Character))
	if !found {
		return nil, nil
	}
	return protocol.Location{
		URI: protocol.DocumentUri(loc.URI),
		Range: protocol.Range{
			Start: protocol.Position{Line: uint32(loc.Line), Character: uint32(loc.Col)},
			End:   protocol.Position{Line: uint32(loc.Line), Character: uint32(loc.EndCol)},
		},
	}, nil
}

func (s *Server) references(_ *glsp.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	uri := string(params.TextDocument.URI)
	raw := ReferencesAt(s.docs, uri, int(params.Position.Line), int(params.Position.Character), params.Context.IncludeDeclaration)
	if len(raw) == 0 {
		return nil, nil
	}
	locs := make([]protocol.Location, len(raw))
	for i, r := range raw {
		locs[i] = protocol.Location{
			URI: protocol.DocumentUri(r.URI),
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(r.Line), Character: uint32(r.Col)},
				End:   protocol.Position{Line: uint32(r.Line), Character: uint32(r.EndCol)},
			},
		}
	}
	return locs, nil
}

func (s *Server) codeAction(_ *glsp.Context, params *protocol.CodeActionParams) (any, error) {
	doc, ok := s.load(string(params.TextDocument.URI))
	if !ok {
		return nil, nil
	}
	content := doc.Content
	startLine := int(params.Range.Start.Line)
	endLine := int(params.Range.End.Line)

	seen := make(map[string]bool)
	var actions []protocol.CodeAction
	for line := startLine; line <= endLine; line++ {
		for _, ca := range CodeActionsAt(content, line) {
			if seen[ca.NewLabelName] {
				continue
			}
			seen[ca.NewLabelName] = true
			newText := "\n:" + ca.NewLabelName + "\n"
			insertLine := uint32(ca.InsertLine)
			lineText := lineAt(content, ca.InsertLine)
			insertChar := uint32(len(lineText))
			kind := protocol.CodeActionKind(ca.Kind)
			uri := params.TextDocument.URI
			changes := map[protocol.DocumentUri][]protocol.TextEdit{
				uri: {
					{
						Range: protocol.Range{
							Start: protocol.Position{Line: insertLine, Character: insertChar},
							End:   protocol.Position{Line: insertLine, Character: insertChar},
						},
						NewText: newText,
					},
				},
			}
			actions = append(actions, protocol.CodeAction{
				Title: ca.Title,
				Kind:  &kind,
				Edit: &protocol.WorkspaceEdit{
					Changes: changes,
				},
			})
		}
	}
	if len(actions) == 0 {
		return nil, nil
	}
	return actions, nil
}

func (s *Server) foldingRange(_ *glsp.Context, params *protocol.FoldingRangeParams) ([]protocol.FoldingRange, error) {
	doc, ok := s.load(string(params.TextDocument.URI))
	if !ok {
		return nil, nil
	}
	folds := FoldingRanges(doc.Content)
	if len(folds) == 0 {
		return nil, nil
	}
	result := make([]protocol.FoldingRange, len(folds))
	for i, f := range folds {
		result[i] = protocol.FoldingRange{
			StartLine: uint32(f.StartLine),
			EndLine:   uint32(f.EndLine),
			Kind:      ptr(f.Kind),
		}
	}
	return result, nil
}

func (s *Server) semanticTokensFull(_ *glsp.Context, params *protocol.SemanticTokensParams) (*protocol.SemanticTokens, error) {
	doc, ok := s.load(string(params.TextDocument.URI))
	if !ok {
		return nil, nil
	}
	tokens := SemanticTokens(doc.Content)
	data := EncodeSemanticTokens(tokens)
	return &protocol.SemanticTokens{Data: data}, nil
}

func (s *Server) codeLens(_ *glsp.Context, params *protocol.CodeLensParams) ([]protocol.CodeLens, error) {
	doc, ok := s.load(string(params.TextDocument.URI))
	if !ok {
		return nil, nil
	}
	lenses := CodeLenses(doc.Content)
	if len(lenses) == 0 {
		return nil, nil
	}
	result := make([]protocol.CodeLens, len(lenses))
	for i, l := range lenses {
		var title string
		switch l.RefCount {
		case 0:
			title = "0 references"
		case 1:
			title = "1 reference"
		default:
			title = fmt.Sprintf("%d references", l.RefCount)
		}
		result[i] = protocol.CodeLens{
			Range: lineRange(uint32(l.Line)),
			Command: &protocol.Command{
				Title: title,
			},
		}
	}
	return result, nil
}

func (s *Server) prepareRename(_ *glsp.Context, params *protocol.PrepareRenameParams) (any, error) {
	doc, ok := s.load(string(params.TextDocument.URI))
	if !ok {
		return nil, nil
	}
	loc, found := PrepareRenameAt(doc.Content, int(params.Position.Line), int(params.Position.Character))
	if !found {
		return nil, nil
	}
	return &protocol.Range{
		Start: protocol.Position{Line: uint32(loc.Line), Character: uint32(loc.Col)},
		End:   protocol.Position{Line: uint32(loc.Line), Character: uint32(loc.EndCol)},
	}, nil
}

func (s *Server) rename(_ *glsp.Context, params *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	uri := string(params.TextDocument.URI)
	editsByURI, err := RenameAt(s.docs, uri, int(params.Position.Line), int(params.Position.Character), params.NewName)
	if err != nil || len(editsByURI) == 0 {
		return nil, nil
	}

	workspaceChanges := make(map[protocol.DocumentUri][]protocol.TextEdit)
	for wUri, edits := range editsByURI {
		lspEdits := make([]protocol.TextEdit, len(edits))
		for i, e := range edits {
			lspEdits[i] = protocol.TextEdit{
				Range: protocol.Range{
					Start: protocol.Position{Line: uint32(e.Line), Character: uint32(e.Col)},
					End:   protocol.Position{Line: uint32(e.Line), Character: uint32(e.EndCol)},
				},
				NewText: e.NewText,
			}
		}
		workspaceChanges[protocol.DocumentUri(wUri)] = lspEdits
	}

	return &protocol.WorkspaceEdit{
		Changes: workspaceChanges,
	}, nil
}

func lineRange(line uint32) protocol.Range {
	return protocol.Range{
		Start: protocol.Position{Line: line, Character: 0},
		End:   protocol.Position{Line: line, Character: 10000},
	}
}

func labelPrefixFromLine(lineBefore string) string {
	lower := strings.ToLower(lineBefore)
	for _, kw := range []string{"goto :", "goto ", "call :"} {
		if idx := strings.LastIndex(lower, kw); idx >= 0 {
			rest := lineBefore[idx+len(kw):]
			return strings.TrimPrefix(rest, ":")
		}
	}
	return ""
}

func varPrefixFromLine(lineBefore string) string {
	idx := strings.LastIndex(lineBefore, "%")
	if idx < 0 {
		return ""
	}
	return lineBefore[idx+1:]
}

func delayedVarPrefixFromLine(lineBefore string) string {
	idx := strings.LastIndex(lineBefore, "!")
	if idx < 0 {
		return ""
	}
	return lineBefore[idx+1:]
}
