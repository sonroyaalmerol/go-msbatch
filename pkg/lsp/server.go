// Package lsp implements a Language Server Protocol server for CMD/batch scripts.
//
// Features:
//   - Diagnostics: undefined GOTO/CALL labels
//   - Hover: built-in command help
//   - Completion: commands, labels, environment variables
//   - Document symbols: labels and SET variable definitions
package lsp

import (
	"fmt"
	"strings"
	"sync"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
)

const serverName = "msbatch-lsp"

// allCommands is the list of recognised command names used for completion.
var allCommands = func() []string {
	var cmds []string
	for name := range commandHelp {
		cmds = append(cmds, name)
	}
	return cmds
}()

// Server wraps the glsp server and owns the document store.
type Server struct {
	mu   sync.RWMutex
	docs map[string]string // URI → content
}

// NewServer creates a ready-to-run LSP server.
func NewServer() *Server {
	return &Server{docs: make(map[string]string)}
}

// Run starts the server on stdin/stdout and blocks until the connection closes.
func (s *Server) Run() error {
	handler := protocol.Handler{
		Initialize:                 s.initialize,
		Initialized:                s.initialized,
		Shutdown:                   s.shutdown,
		TextDocumentDidOpen:        s.didOpen,
		TextDocumentDidChange:      s.didChange,
		TextDocumentDidClose:       s.didClose,
		TextDocumentHover:          s.hover,
		TextDocumentCompletion:     s.completion,
		TextDocumentDocumentSymbol: s.documentSymbol,
		TextDocumentDefinition:     s.definition,
		TextDocumentReferences:     s.references,
	}
	srv := server.NewServer(&handler, serverName, false)
	return srv.RunStdio()
}

// ── document store ────────────────────────────────────────────────────────────

func (s *Server) store(uri, content string) {
	s.mu.Lock()
	s.docs[uri] = content
	s.mu.Unlock()
}

func (s *Server) load(uri string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.docs[uri]
	return c, ok
}

func (s *Server) remove(uri string) {
	s.mu.Lock()
	delete(s.docs, uri)
	s.mu.Unlock()
}

// ── helpers ───────────────────────────────────────────────────────────────────

func ptr[T any](v T) *T { return &v }

// publishDiagnostics sends textDocument/publishDiagnostics for uri.
func (s *Server) publishDiagnostics(ctx *glsp.Context, uri, content string) {
	diags := Diagnostics(content)

	lspDiags := make([]protocol.Diagnostic, 0, len(diags))
	for _, d := range diags {
		endChar := uint32(10000) // extend to end of line
		if d.EndCol > 0 {
			endChar = uint32(d.EndCol)
		}
		lspDiags = append(lspDiags, protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(d.Line), Character: uint32(d.Col)},
				End:   protocol.Position{Line: uint32(d.Line), Character: endChar},
			},
			Severity: ptr(protocol.DiagnosticSeverity(d.Sev)),
			Source:   ptr(serverName),
			Message:  d.Message,
		})
	}

	ctx.Notify("textDocument/publishDiagnostics", &protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: lspDiags,
	})
}

// lineAt returns the text of line lineIdx (0-based) in content.
func lineAt(content string, lineIdx int) string {
	lines := strings.Split(content, "\n")
	if lineIdx < 0 || lineIdx >= len(lines) {
		return ""
	}
	return strings.TrimRight(lines[lineIdx], "\r")
}

// ── LSP handlers ──────────────────────────────────────────────────────────────

func (s *Server) initialize(_ *glsp.Context, _ *protocol.InitializeParams) (any, error) {
	syncKind := protocol.TextDocumentSyncKindFull
	return protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: &protocol.TextDocumentSyncOptions{
				OpenClose: ptr(true),
				Change:    &syncKind,
			},
			HoverProvider:          true,
			CompletionProvider:     &protocol.CompletionOptions{TriggerCharacters: []string{"%", ":"}},
			DocumentSymbolProvider: true,
			DefinitionProvider:     true,
			ReferencesProvider:     true,
		},
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name:    serverName,
			Version: ptr("0.1.0"),
		},
	}, nil
}

func (s *Server) initialized(_ *glsp.Context, _ *protocol.InitializedParams) error {
	return nil
}

func (s *Server) shutdown(_ *glsp.Context) error {
	return nil
}

func (s *Server) didOpen(ctx *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
	content := params.TextDocument.Text
	s.store(string(params.TextDocument.URI), content)
	s.publishDiagnostics(ctx, string(params.TextDocument.URI), content)
	return nil
}

func (s *Server) didChange(ctx *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	if len(params.ContentChanges) == 0 {
		return nil
	}
	// Full-sync: the last change contains the whole document.
	var content string
	switch v := params.ContentChanges[len(params.ContentChanges)-1].(type) {
	case protocol.TextDocumentContentChangeEvent:
		content = v.Text
	case protocol.TextDocumentContentChangeEventWhole:
		content = v.Text
	}
	uri := string(params.TextDocument.URI)
	s.store(uri, content)
	s.publishDiagnostics(ctx, uri, content)
	return nil
}

func (s *Server) didClose(_ *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
	s.remove(string(params.TextDocument.URI))
	return nil
}

func (s *Server) hover(_ *glsp.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	content, ok := s.load(string(params.TextDocument.URI))
	if !ok {
		return nil, nil
	}

	line := lineAt(content, int(params.Position.Line))
	word := WordAtPosition(line, int(params.Position.Character))
	if word == "" {
		return nil, nil
	}

	help := CommandHelp(word)
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
	content, ok := s.load(string(params.TextDocument.URI))
	if !ok {
		return nil, nil
	}

	line := lineAt(content, int(params.Position.Line))
	col := min(int(params.Position.Character), len(line))
	lineBefore := line[:col]

	ctx := CompletionContextAt(lineBefore)
	a := Analyze(content)

	var items []protocol.CompletionItem
	switch ctx {
	case CompleteCommand:
		prefix := strings.ToLower(WordAtPosition(lineBefore, col))
		for _, name := range allCommands {
			if strings.HasPrefix(name, prefix) {
				kind := protocol.CompletionItemKindKeyword
				help := CommandHelp(name)
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
		for _, lbl := range a.Labels {
			if strings.HasPrefix(lbl.Name, strings.ToLower(prefix)) {
				kind := protocol.CompletionItemKindReference
				items = append(items, protocol.CompletionItem{
					Label:  lbl.Name,
					Kind:   &kind,
					Detail: ptr(fmt.Sprintf("line %d", lbl.Line+1)),
				})
			}
		}

	case CompleteVariable:
		prefix := varPrefixFromLine(lineBefore)
		for _, v := range a.Vars {
			if strings.HasPrefix(strings.ToUpper(v.Name), strings.ToUpper(prefix)) {
				kind := protocol.CompletionItemKindVariable
				items = append(items, protocol.CompletionItem{
					Label:  v.Name,
					Kind:   &kind,
					Detail: ptr(v.Value),
				})
			}
		}
	}

	return items, nil
}

func (s *Server) documentSymbol(_ *glsp.Context, params *protocol.DocumentSymbolParams) (any, error) {
	content, ok := s.load(string(params.TextDocument.URI))
	if !ok {
		return nil, nil
	}

	a := Analyze(content)
	var syms []protocol.DocumentSymbol

	for _, lbl := range a.Labels {
		r := lineRange(uint32(lbl.Line))
		syms = append(syms, protocol.DocumentSymbol{
			Name:           ":" + lbl.Name,
			Kind:           protocol.SymbolKindFunction,
			Range:          r,
			SelectionRange: r,
		})
	}

	for _, v := range a.Vars {
		r := lineRange(uint32(v.Line))
		syms = append(syms, protocol.DocumentSymbol{
			Name:           v.Name,
			Kind:           protocol.SymbolKindVariable,
			Detail:         ptr(v.Value),
			Range:          r,
			SelectionRange: r,
		})
	}

	return syms, nil
}

func (s *Server) definition(_ *glsp.Context, params *protocol.DefinitionParams) (any, error) {
	content, ok := s.load(string(params.TextDocument.URI))
	if !ok {
		return nil, nil
	}
	loc, found := DefinitionAt(content, int(params.Position.Line), int(params.Position.Character))
	if !found {
		return nil, nil
	}
	return protocol.Location{
		URI: params.TextDocument.URI,
		Range: protocol.Range{
			Start: protocol.Position{Line: uint32(loc.Line), Character: uint32(loc.Col)},
			End:   protocol.Position{Line: uint32(loc.Line), Character: uint32(loc.EndCol)},
		},
	}, nil
}

func (s *Server) references(_ *glsp.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	content, ok := s.load(string(params.TextDocument.URI))
	if !ok {
		return nil, nil
	}
	raw := ReferencesAt(content,
		int(params.Position.Line),
		int(params.Position.Character),
		params.Context.IncludeDeclaration,
	)
	if len(raw) == 0 {
		return nil, nil
	}
	locs := make([]protocol.Location, len(raw))
	for i, r := range raw {
		locs[i] = protocol.Location{
			URI: params.TextDocument.URI,
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(r.Line), Character: uint32(r.Col)},
				End:   protocol.Position{Line: uint32(r.Line), Character: uint32(r.EndCol)},
			},
		}
	}
	return locs, nil
}

// ── small utilities ───────────────────────────────────────────────────────────

func lineRange(line uint32) protocol.Range {
	return protocol.Range{
		Start: protocol.Position{Line: line, Character: 0},
		End:   protocol.Position{Line: line, Character: 10000},
	}
}

// labelPrefixFromLine extracts the partial label name after "goto " or "call :".
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

// varPrefixFromLine extracts the partial variable name after the last '%'.
func varPrefixFromLine(lineBefore string) string {
	idx := strings.LastIndex(lineBefore, "%")
	if idx < 0 {
		return ""
	}
	return lineBefore[idx+1:]
}
