package parser

import (
	"unicode/utf8"

	"github.com/sonroyaalmerol/go-msbatch/pkg/lexer"
)

// Parser converts a BatchLexer token stream into an AST.
type Parser struct {
	tokens        []lexer.Item
	pos           int
	compoundDepth int
	Diagnostics   []Diagnostic
}

// New drains all tokens from src into a Parser.
func New(src *lexer.BatchLexer) *Parser {
	tokens := make([]lexer.Item, 0, src.InputLen()/3+8)
	for {
		t := src.NextItem()
		if t.Type == lexer.TokenEOF || (t.Type == 0 && runeLen(t.Value) == 0) {
			break
		}
		tokens = append(tokens, t)
	}
	return &Parser{tokens: tokens}
}

// NewFromTokens creates a Parser from a pre-collected token slice.
// Use this when tokens have been assembled from multiple per-line lexer
// invocations (e.g. with NewWithLine) so that Item.Line values are correct.
func NewFromTokens(tokens []lexer.Item) *Parser {
	return &Parser{tokens: tokens}
}

// Parse returns all top-level nodes, processing the entire token stream.
func (p *Parser) Parse() []Node {
	var nodes []Node
	for p.pos < len(p.tokens) {
		p.skipWS()
		if p.pos >= len(p.tokens) {
			break
		}

		if n := p.parseCommand(); n != nil {
			nodes = append(nodes, n)
		} else {
			t := p.peek()
			if t.Type != lexer.TokenEOF {
				p.addErrorAtToken(t, "unexpected token: "+t.Value)
			}
			p.pos++
		}
	}
	return nodes
}

// parseCommand parses one command, which may be a binary expression.
func (p *Parser) parseCommand() Node {
	return p.parseBinary()
}

// ---- token stream helpers ------------------------------------------------

func (p *Parser) peek() lexer.Item {
	if p.pos >= len(p.tokens) {
		return lexer.Item{Type: lexer.TokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) consume() lexer.Item {
	t := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return t
}

// val returns a token's value; tokens hold zero-copy source substrings.
func val(t lexer.Item) string {
	return t.Value
}

func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

// skipSpaces advances past whitespace tokens but stops at a line break.
func (p *Parser) skipSpaces() {
	for p.pos < len(p.tokens) && p.tokens[p.pos].Type == lexer.TokenWhitespace {
		p.pos++
	}
}

func isOperatorRun(v string) bool {
	if v == "" {
		return false
	}
	for _, r := range v {
		if r != '&' && r != '|' {
			return false
		}
	}
	return true
}

func (p *Parser) peekOperatorRun() string {
	t := p.peek()
	if t.Type == lexer.TokenPunctuation && isOperatorRun(val(t)) {
		return val(t)
	}
	return ""
}

func operatorAbort(op string, line, col int) *AbortNode {
	return &AbortNode{Line: line, Col: col, Message: op + " was unexpected at this time."}
}

func (p *Parser) skipWS() {
	for p.pos < len(p.tokens) {
		t := p.tokens[p.pos]
		if t.Type == lexer.TokenWhitespace || t.Type == lexer.TokenNewline {
			p.pos++
			continue
		}
		break
	}
}

// skipWSOnly advances past whitespace tokens only (not newlines).
func (p *Parser) skipWSOnly() {
	for p.pos < len(p.tokens) {
		t := p.tokens[p.pos]
		if t.Type == lexer.TokenWhitespace {
			p.pos++
			continue
		}
		break
	}
}

// isPipeOrAmpVal reports whether a TokenPunctuation value is a command separator.
func isPipeOrAmpVal(v string) bool {
	switch v {
	case "|", "||", "&", "&&":
		return true
	}
	return false
}

// updatePos returns updated endLine and endCol if the token extends beyond the current position.
func (p *Parser) updatePos(endLine, endCol int, t lexer.Item) (int, int) {
	if t.Line > endLine || (t.Line == endLine && t.Col+runeLen(t.Value) > endCol) {
		return t.Line, t.Col + runeLen(t.Value)
	}
	return endLine, endCol
}

// addError records a parse error at the given location.
func (p *Parser) addError(line, col, endLine, endCol int, message string) {
	p.Diagnostics = append(p.Diagnostics, Diagnostic{
		Line:     line,
		Col:      col,
		EndLine:  endLine,
		EndCol:   endCol,
		Severity: "error",
		Message:  message,
	})
}

// addErrorAtToken records a parse error spanning the given token.
func (p *Parser) addErrorAtToken(t lexer.Item, message string) {
	p.addError(t.Line, t.Col, t.EndLine, t.EndCol, message)
}
