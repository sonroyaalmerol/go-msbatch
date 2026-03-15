package parser

import (
	"github.com/sonroyaalmerol/go-msbatch/internal/lex"
	"github.com/sonroyaalmerol/go-msbatch/pkg/lexer"
)

// item is a convenience alias.
type item = lex.Item[lexer.TokenType, rune]

// Parser converts a BatchLexer token stream into an AST.
type Parser struct {
	tokens        []item
	pos           int
	compoundDepth int
}

// New drains all tokens from src into a Parser.
func New(src *lexer.BatchLexer) *Parser {
	var tokens []item
	for {
		t := src.NextItem()
		if t.Type == lexer.TokenEOF || (t.Type == 0 && len(t.Value) == 0) {
			break
		}
		tokens = append(tokens, t)
	}
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
			p.pos++ // skip unrecognised token
		}
	}
	return nodes
}

// parseCommand parses one command, which may be a binary expression.
func (p *Parser) parseCommand() Node {
	return p.parseBinary()
}

// ---- token stream helpers ------------------------------------------------

func (p *Parser) peek() item {
	if p.pos >= len(p.tokens) {
		return item{Type: lexer.TokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) consume() item {
	t := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return t
}

// val returns the rune slice of a token as a string.
func val(t item) string {
	return string(t.Value)
}

// skipWS advances past whitespace and newline tokens.
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

// isPipeOrAmpVal reports whether a TokenPunctuation value is a command separator.
func isPipeOrAmpVal(v string) bool {
	switch v {
	case "|", "||", "&", "&&":
		return true
	}
	return false
}

