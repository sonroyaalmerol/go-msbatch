package store

import (
	"sync"

	"github.com/sonroyaalmerol/go-msbatch/pkg/lexer"
	"github.com/sonroyaalmerol/go-msbatch/pkg/lsp/analysis"
	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
)

type Document struct {
	URI        string
	Version    int
	Text       string
	Tokens     []lexer.Item
	Nodes      []parser.Node
	Diags      []parser.Diagnostic
	ParseError error
	Analysis   *analysis.Result
}

type Store struct {
	mu   sync.RWMutex
	docs map[string]*Document
}

func New() *Store {
	return &Store{docs: make(map[string]*Document)}
}

func (s *Store) Get(uri string) *Document {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.docs[uri]
}

func (s *Store) Put(uri string, version int, text string) *Document {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc := &Document{
		URI:     uri,
		Version: version,
		Text:    text,
	}
	s.parseLocked(doc)
	s.docs[uri] = doc
	return doc
}

func (s *Store) Delete(uri string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.docs, uri)
}

func (s *Store) parseLocked(doc *Document) {
	l := lexer.New(doc.Text)
	var tokens []lexer.Item
	for {
		t := l.NextItem()
		if t.Type == lexer.TokenEOF || (t.Type == 0 && len(t.Value) == 0) {
			break
		}
		tokens = append(tokens, t)
	}
	doc.Tokens = tokens

	p := parser.NewFromTokens(tokens)
	doc.Nodes = p.Parse()
	doc.Diags = p.Diagnostics

	doc.Analysis = analysis.Analyze(doc.Nodes, doc.Tokens)

	for _, d := range doc.Analysis.Diagnostics {
		doc.Diags = append(doc.Diags, parser.Diagnostic{
			Line:     d.Line,
			Col:      d.Col,
			EndLine:  d.EndLine,
			EndCol:   d.EndCol,
			Severity: d.Severity,
			Message:  d.Message,
		})
	}
}
