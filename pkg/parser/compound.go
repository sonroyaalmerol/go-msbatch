package parser

import (
	"strconv"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/lexer"
)

// parseBlock parses a parenthesised command block: ( cmd... ).
//
// Windows CMD semantics: a label definition (:name) inside a parenthesised
// block terminates the block at that line.  The ':' token is left unconsumed
// so the outer parser picks it up as a top-level (or enclosing-block-level)
// label node.  Because each enclosing parseBlock call performs the same check,
// the label propagates all the way to the top-level node list, which is the
// correct CMD behaviour.
func (p *Parser) parseBlock() *Block {
	open := p.peek()
	p.consume() // consume "("
	p.compoundDepth++
	block := &Block{Line: open.Line, Col: open.Col, EndLine: open.Line, EndCol: open.Col + 1}
	for p.pos < len(p.tokens) {
		p.skipWS()
		t := p.peek()
		if t.Type == lexer.TokenEOF {
			break
		}
		if t.Type == lexer.TokenPunctuation && val(t) == ")" {
			block.EndLine = t.Line
			block.EndCol = t.Col + 1
			p.consume()
			break
		}
		// A label definition inside a parenthesised block terminates the block
		// in Windows CMD.  Leave the ':' token for the outer parser.
		if t.Type == lexer.TokenPunctuation && val(t) == ":" {
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
		cond.Arg, _, _ = p.collectStokenWithPos(0, 0)
	case "defined":
		p.consume()
		p.skipWS()
		cond.Kind = CondDefined
		cond.Arg, _, _ = p.collectStokenWithPos(0, 0)
	case "errorlevel":
		p.consume()
		p.skipWS()
		cond.Kind = CondErrorLevel
		stoken, _, _ := p.collectStokenWithPos(0, 0)
		cond.Level, _ = strconv.Atoi(stoken)
	default:
		cond.Kind = CondCompare
		cond.Left, _, _ = p.collectStokenWithPos(0, 0)
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
		cond.Right, _, _ = p.collectStokenWithPos(0, 0)
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

	if p.peek().Type == lexer.TokenNameForVar || p.peek().Type == lexer.TokenNameVariable || p.peek().Type == lexer.TokenWord || p.peek().Type == lexer.TokenText {
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

		var stoken string
		if t.Type == lexer.TokenStringDouble || t.Type == lexer.TokenStringSingle || t.Type == lexer.TokenStringBT {
			stoken = p.collectQuotedString()
		} else if t.Type == lexer.TokenText || t.Type == lexer.TokenWord {
			word := val(t)
			if strings.HasPrefix(word, "`") || strings.HasPrefix(word, "'") {
				stoken = p.collectQuotedCommandString(word[0])
			} else {
				stoken, _, _ = p.collectStokenWithPos(0, 0)
			}
		} else {
			stoken, _, _ = p.collectStokenWithPos(0, 0)
		}

		if stoken != "" {
			if len(stoken) >= 2 && (stoken[0] == '"' || stoken[0] == '\'' || stoken[0] == '`') && stoken[len(stoken)-1] == stoken[0] {
				items = append(items, stoken)
			} else {
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

func (p *Parser) collectQuotedCommandString(quoteChar byte) string {
	var sb strings.Builder
	quoteRune := string(rune(quoteChar))
	for p.pos < len(p.tokens) {
		t := p.peek()
		switch t.Type {
		case lexer.TokenEOF, lexer.TokenNewline:
			return sb.String()
		case lexer.TokenWhitespace:
			sb.WriteString(val(p.consume()))
		case lexer.TokenText, lexer.TokenWord, lexer.TokenNameVariable, lexer.TokenNameForVar, lexer.TokenNameDelayedVar, lexer.TokenStringEscape, lexer.TokenNumber:
			word := val(t)
			sb.WriteString(val(p.consume()))
			if strings.HasSuffix(word, quoteRune) && len(word) > 1 {
				return sb.String()
			}
		case lexer.TokenPunctuation:
			v := val(t)
			if v == ")" {
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

// collectQuotedStringWithToken collects a quoted string and returns it along with the last token consumed.
func (p *Parser) collectQuotedStringWithToken() (string, lexer.Item) {
	var sb strings.Builder
	t := p.consume()
	sb.WriteString(val(t))
	lastTok := t
	if len(val(t)) == 0 {
		return sb.String(), lastTok
	}
	quoteChar := val(t)[0]

	if len(val(t)) > 1 && strings.HasSuffix(val(t), string(quoteChar)) {
		return sb.String(), lastTok
	}

	for p.pos < len(p.tokens) {
		t2 := p.consume()
		sb.WriteString(val(t2))
		lastTok = t2
		if strings.HasSuffix(val(t2), string(quoteChar)) {
			break
		}
	}
	return sb.String(), lastTok
}

// collectStokenWithPos collects a simple token and returns it along with updated end position.
func (p *Parser) collectStokenWithPos(endLine, endCol int) (string, int, int) {
	var sb strings.Builder
	for p.pos < len(p.tokens) {
		t := p.peek()
		switch t.Type {
		case lexer.TokenEOF, lexer.TokenNewline, lexer.TokenWhitespace:
			return sb.String(), endLine, endCol

		case lexer.TokenStringDouble, lexer.TokenStringSingle, lexer.TokenStringBT:
			if sb.Len() > 0 {
				return sb.String(), endLine, endCol
			}
			quoted, lastTok := p.collectQuotedStringWithToken()
			sb.WriteString(quoted)
			if lastTok.Line > endLine || (lastTok.Line == endLine && lastTok.Col+len(lastTok.Value) > endCol) {
				endLine = lastTok.Line
				endCol = lastTok.Col + len(lastTok.Value)
			}
			return sb.String(), endLine, endCol

		case lexer.TokenKeyword:
			consumed := p.consume()
			sb.WriteString(val(consumed))
			if consumed.Line > endLine || (consumed.Line == endLine && consumed.Col+len(consumed.Value) > endCol) {
				endLine = consumed.Line
				endCol = consumed.Col + len(consumed.Value)
			}

		case lexer.TokenText, lexer.TokenWord, lexer.TokenNameVariable, lexer.TokenNameForVar, lexer.TokenNameDelayedVar, lexer.TokenStringEscape, lexer.TokenNumber:
			consumed := p.consume()
			sb.WriteString(val(consumed))
			if consumed.Line > endLine || (consumed.Line == endLine && consumed.Col+len(consumed.Value) > endCol) {
				endLine = consumed.Line
				endCol = consumed.Col + len(consumed.Value)
			}

		case lexer.TokenPunctuation:
			v := val(t)
			if v == "(" || v == ")" || isPipeOrAmpVal(v) || v == "==" || v == "," || v == ";" || v == "=" {
				return sb.String(), endLine, endCol
			}
			consumed := p.consume()
			sb.WriteString(val(consumed))
			if consumed.Line > endLine || (consumed.Line == endLine && consumed.Col+len(consumed.Value) > endCol) {
				endLine = consumed.Line
				endCol = consumed.Col + len(consumed.Value)
			}

		case lexer.TokenOperator:
			v := val(t)
			if v == "==" || v == "&&" || v == "||" {
				return sb.String(), endLine, endCol
			}
			consumed := p.consume()
			sb.WriteString(val(consumed))
			if consumed.Line > endLine || (consumed.Line == endLine && consumed.Col+len(consumed.Value) > endCol) {
				endLine = consumed.Line
				endCol = consumed.Col + len(consumed.Value)
			}

		default:
			consumed := p.consume()
			sb.WriteString(val(consumed))
			if consumed.Line > endLine || (consumed.Line == endLine && consumed.Col+len(consumed.Value) > endCol) {
				endLine = consumed.Line
				endCol = consumed.Col + len(consumed.Value)
			}
		}
	}
	return sb.String(), endLine, endCol
}
