package parser

import (
	"strconv"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/lexer"
)

// parseBlock parses a parenthesised command block: ( cmd... ).
func (p *Parser) parseBlock() *Block {
	open := p.peek()
	p.consume() // consume "("
	p.compoundDepth++
	block := &Block{Line: open.Line, Col: open.Col, EndLine: open.Line}
	for p.pos < len(p.tokens) {
		p.skipWS()
		t := p.peek()
		if t.Type == lexer.TokenEOF {
			break
		}
		if t.Type == lexer.TokenPunctuation && val(t) == ")" {
			block.EndLine = t.Line
			p.consume()
			break
		}
		if n := p.parseCommand(); n != nil {
			block.Body = append(block.Body, n)
		} else {
			p.pos++
		}
	}
	p.compoundDepth--
	return block
}

// parseIf parses an IF statement.
func (p *Parser) parseIf() *IfNode {
	ifTok := p.peek()
	p.consume() // consume "if"
	n := &IfNode{Line: ifTok.Line, Col: ifTok.Col}

	p.skipWS()
	if p.peekKeyword("/i") {
		n.CaseInsensitive = true
		p.consume()
		p.skipWS()
	}

	n.Cond = p.parseCondition()

	// THEN body
	n.Then = p.parseThenBody()

	// ELSE branch
	p.skipWS()
	if p.peekKeyword("else") {
		p.consume()
		n.Else = p.parseThenBody()
	}

	return n
}

func (p *Parser) parseThenBody() Node {
	p.skipWS()
	t := p.peek()
	if t.Type == lexer.TokenPunctuation && val(t) == "(" {
		return p.parseBlock()
	}
	return p.parseBinary()
}

// parseCondition parses the condition part of an IF statement.
func (p *Parser) parseCondition() Condition {
	p.skipWS()
	cond := Condition{}

	if p.peekKeyword("not") {
		cond.Not = true
		p.consume()
		p.skipWS()
	}

	t := p.peek()
	v := strings.ToLower(val(t))

	switch v {
	case "exist":
		p.consume()
		p.skipWS()
		cond.Kind = CondExist
		cond.Arg = p.collectStoken()
	case "defined":
		p.consume()
		p.skipWS()
		cond.Kind = CondDefined
		cond.Arg = p.collectStoken()
	case "errorlevel":
		p.consume()
		p.skipWS()
		cond.Kind = CondErrorLevel
		cond.Level, _ = strconv.Atoi(p.collectStoken())
	default:
		cond.Kind = CondCompare
		cond.Left = p.collectStoken()
		p.skipWS()

		opToken := p.peek()
		opVal := strings.ToLower(val(opToken))
		isOp := false
		switch opVal {
		case "==", "equ", "neq", "lss", "leq", "gtr", "geq":
			cond.Op = CompareOp(opVal)
			p.consume()
			isOp = true
		}
		if !isOp {
			cond.Op = OpEqual
		}

		p.skipWS()
		cond.Right = p.collectStoken()
	}

	return cond
}

// parseFor parses a FOR statement.
func (p *Parser) parseFor(_ bool) *ForNode {
	forTok := p.peek()
	p.consume() // consume "for"
	n := &ForNode{Line: forTok.Line, Col: forTok.Col}

	p.skipWS()
	t := p.peek()
	if t.Type == lexer.TokenKeyword || t.Type == lexer.TokenText || t.Type == lexer.TokenWord {
		v := strings.ToLower(val(t))
		switch v {
		case "/f":
			n.Variant = ForF
			p.consume()
			p.skipWS()
			t2 := p.peek()
			if t2.Type == lexer.TokenStringDouble || t2.Type == lexer.TokenStringSingle || t2.Type == lexer.TokenStringBT {
				n.Options = p.collectQuotedString()
				p.skipWS()
			}
		case "/l":
			n.Variant = ForRange
			p.consume()
			p.skipWS()
		case "/d":
			n.Variant = ForDir
			p.consume()
			p.skipWS()
		case "/r":
			n.Variant = ForRecursive
			p.consume()
			p.skipWS()
			// The lexer emits the optional root path as TokenText before the loop variable.
			if p.peek().Type == lexer.TokenText {
				n.Options = strings.TrimSpace(val(p.consume()))
				p.skipWS()
			}
		}
	}

	if p.peek().Type == lexer.TokenNameVariable || p.peek().Type == lexer.TokenWord || p.peek().Type == lexer.TokenText {
		varTok := p.peek()
		raw := val(p.consume())
		n.Variable = stripForVarPercents(raw)
		// VarCol points to the letter: skip any leading %%
		leadingPercents := len(raw) - len(strings.TrimLeft(raw, "%"))
		n.VarLine = varTok.Line
		n.VarCol = varTok.Col + leadingPercents
		p.skipWS()
	} else if p.peek().Type == lexer.TokenStringEscape {
		// Fallback: %%X emitted as TokenStringEscape("%%") + TokenWord("X")
		escTok := p.peek()
		esc := val(p.consume())
		if p.peek().Type == lexer.TokenWord || p.peek().Type == lexer.TokenText || p.peek().Type == lexer.TokenNameVariable {
			letterTok := p.peek()
			n.Variable = stripForVarPercents(esc + val(p.consume()))
			n.VarLine = letterTok.Line
			n.VarCol = letterTok.Col
		} else {
			n.Variable = stripForVarPercents(esc)
			n.VarLine = escTok.Line
			n.VarCol = escTok.Col + len([]rune(esc))
		}
		p.skipWS()
	}

	if p.peekKeyword("in") {
		p.consume()
		p.skipWS()
	}

	if p.peek().Type == lexer.TokenPunctuation && val(p.peek()) == "(" {
		p.consume()
		n.Set = p.collectForSet()
		p.skipWS()
	}

	if p.peekKeyword("do") {
		p.consume()
		p.skipWS()
	}

	n.Do = p.parseBinary()
	return n
}

func (p *Parser) collectForSet() []string {
	var items []string
	for p.pos < len(p.tokens) {
		p.skipSetWS()
		t := p.peek()
		if t.Type == lexer.TokenEOF {
			break
		}
		if t.Type == lexer.TokenPunctuation && val(t) == ")" {
			p.consume()
			break
		}

		stoken := p.collectStoken()
		if stoken != "" {
			// Quoted strings are single items even if they contain commas.
			if len(stoken) >= 2 && (stoken[0] == '"' || stoken[0] == '\'') && stoken[len(stoken)-1] == stoken[0] {
				items = append(items, stoken)
			} else {
				// Commas are separators in FOR sets (equivalent to spaces).
				for part := range strings.SplitSeq(stoken, ",") {
					part = strings.TrimSpace(part)
					if part != "" {
						items = append(items, part)
					}
				}
			}
		} else {
			p.pos++
		}
	}
	return items
}

func (p *Parser) skipSetWS() {
	for p.pos < len(p.tokens) {
		t := p.tokens[p.pos]
		if t.Type == lexer.TokenWhitespace || t.Type == lexer.TokenNewline {
			p.pos++
			continue
		}
		if t.Type == lexer.TokenPunctuation {
			v := val(t)
			if v == "," || v == ";" || v == "=" {
				p.pos++
				continue
			}
		}
		break
	}
}

// ---- IF condition helpers -------------------------------------------------

func (p *Parser) peekKeyword(kw string) bool {
	t := p.peek()
	v := strings.ToLower(val(t))
	return (t.Type == lexer.TokenKeyword || t.Type == lexer.TokenWord || t.Type == lexer.TokenText) && v == kw
}

func (p *Parser) collectStoken() string {
	var sb strings.Builder
	for p.pos < len(p.tokens) {
		t := p.peek()
		switch t.Type {
		case lexer.TokenEOF, lexer.TokenNewline, lexer.TokenWhitespace:
			return sb.String()

		case lexer.TokenStringDouble, lexer.TokenStringSingle, lexer.TokenStringBT:
			if sb.Len() > 0 {
				return sb.String()
			}
			return p.collectQuotedString()

		case lexer.TokenKeyword:
			sb.WriteString(val(p.consume()))
			return sb.String()

		case lexer.TokenText, lexer.TokenWord, lexer.TokenNameVariable, lexer.TokenStringEscape, lexer.TokenNumber:
			sb.WriteString(val(p.consume()))
			return sb.String()

		case lexer.TokenPunctuation:
			v := val(t)
			if v == "(" || v == ")" || isPipeOrAmpVal(v) || v == "==" || v == "," || v == ";" || v == "=" {
				return sb.String()
			}
			sb.WriteString(val(p.consume()))

		case lexer.TokenOperator:
			v := val(t)
			if v == "==" || v == "&&" || v == "||" {
				return sb.String()
			}
			sb.WriteString(val(p.consume()))

		default:
			sb.WriteString(val(p.consume()))
		}
	}
	return sb.String()
}

func (p *Parser) collectQuotedString() string {
	var sb strings.Builder
	t := p.consume()
	sb.WriteString(val(t))
	if len(val(t)) == 0 {
		return sb.String()
	}
	quoteChar := val(t)[0]

	if len(val(t)) > 1 && strings.HasSuffix(val(t), string(quoteChar)) {
		return sb.String()
	}

	for p.pos < len(p.tokens) {
		t2 := p.consume()
		sb.WriteString(val(t2))
		if strings.HasSuffix(val(t2), string(quoteChar)) {
			break
		}
	}
	return sb.String()
}

func stripForVarPercents(s string) string {
	s = strings.TrimPrefix(s, "%%")
	s = strings.TrimPrefix(s, "%")
	s = strings.TrimSuffix(s, "%")
	return s
}
