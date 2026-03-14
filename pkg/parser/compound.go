package parser

import (
	"strconv"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/lexer"
)

// parseBlock parses a parenthesised command block: ( cmd... ).
// The opening ( must be the current token.
func (p *Parser) parseBlock() *Block {
	p.consume() // consume "("
	block := &Block{}
	for p.pos < len(p.tokens) {
		p.skipWS()
		t := p.peek()
		if t.Type == lexer.TokenEOF {
			break
		}
		if t.Type == lexer.TokenPunctuation && val(t) == ")" {
			p.consume()
			break
		}
		if n := p.parseCommand(); n != nil {
			block.Body = append(block.Body, n)
		} else {
			p.pos++
		}
	}
	return block
}

// parseIf parses an IF statement starting after the "if" keyword.
// The "if" token has NOT yet been consumed when this is called.
func (p *Parser) parseIf(_ bool) *IfNode {
	p.consume() // consume "if"
	n := &IfNode{}

	// Optional /i flag
	if p.peek().Type == lexer.TokenKeyword && strings.ToLower(val(p.peek())) == "/i" {
		n.CaseInsensitive = true
		p.consume()
	}

	// Optional NOT
	if p.peek().Type == lexer.TokenKeyword && strings.ToLower(val(p.peek())) == "not" {
		n.Cond.Not = true
		p.consume()
	}

	// Condition type
	switch {
	case p.peekKeyword("exist"):
		p.consume()
		n.Cond.Kind = CondExist
		n.Cond.Arg = p.collectStoken()

	case p.peekKeyword("defined"):
		p.consume()
		n.Cond.Kind = CondDefined
		n.Cond.Arg = p.collectStoken()

	case p.peekKeyword("errorlevel"):
		p.consume()
		n.Cond.Kind = CondErrorLevel
		if p.peek().Type == lexer.TokenNumber {
			n.Cond.Level, _ = strconv.Atoi(val(p.consume()))
		}

	case p.peekKeyword("cmdextversion"):
		p.consume()
		n.Cond.Kind = CondCmdExtVersion
		if p.peek().Type == lexer.TokenNumber {
			n.Cond.Level, _ = strconv.Atoi(val(p.consume()))
		}

	default:
		// Compare: LHS op RHS
		n.Cond.Kind = CondCompare
		n.Cond.Left = p.collectUntilOperator()
		n.Cond.Op = p.collectCompareOp()
		n.Cond.Right = p.collectStoken()
	}

	// THEN body
	n.Then = p.parseThenBody()

	// Optional ELSE
	p.skipWS()
	if p.peekKeyword("else") {
		p.consume()
		n.Else = p.parseThenBody()
	}

	return n
}

// parseThenBody parses the THEN or ELSE body of an IF.
// It is either a compound block starting with ( or an inline command.
func (p *Parser) parseThenBody() Node {
	p.skipWS()
	t := p.peek()
	if t.Type == lexer.TokenPunctuation && val(t) == "(" {
		return p.parseBlock()
	}
	// Inline body: collect until newline or pipe/amp
	return p.parseInlineBody()
}

// parseInlineBody parses a single inline command from remaining tokens,
// treating leading-space TokenText as command + args.
func (p *Parser) parseInlineBody() Node {
	p.skipWS()
	t := p.peek()
	if t.Type == lexer.TokenEOF {
		return nil
	}
	// The DO/THEN body from stateFollow arrives as TokenText(" command args")
	// where the text may start with a space.  We create a SimpleCommand by
	// treating the trimmed leading word as the name.
	if t.Type == lexer.TokenText {
		s := strings.TrimLeft(val(t), " \t")
		if s == "" || isNewlineOnly(s) {
			p.consume()
			return p.parseInlineBody()
		}
		// Split the text: first word = command name, rest = first arg chunk.
		p.consume()
		parts := strings.SplitN(s, " ", 2)
		cmd := &SimpleCommand{Name: parts[0]}
		if len(parts) > 1 && parts[1] != "" {
			// Split remaining text directly into args.
			for a := range strings.FieldsSeq(parts[1]) {
				if a != "" {
					cmd.Args = append(cmd.Args, a)
				}
			}
		}
		// Continue collecting additional tokens as args/redirects
		p.collectArgs(cmd)
		return cmd
	}
	// Proper keyword or other token — use normal parser
	return p.parseBinary()
}

// parseFor parses a FOR statement.
// The "for" token has NOT yet been consumed when this is called.
func (p *Parser) parseFor(_ bool) *ForNode {
	p.consume() // consume "for"
	n := &ForNode{}

	// /f or /l flag
	if p.peek().Type == lexer.TokenKeyword {
		switch strings.ToLower(val(p.peek())) {
		case "/f":
			n.Variant = ForF
			p.consume()
			// Optional options string (TokenStringDouble, TokenStringSingle, TokenStringBT)
			switch p.peek().Type {
			case lexer.TokenStringDouble, lexer.TokenStringSingle, lexer.TokenStringBT:
				n.Options = unquote(val(p.consume()))
			}
		case "/l":
			n.Variant = ForRange
			p.consume()
		}
	}

	// Loop variable: TokenNameVariable (%%i or %i)
	if p.peek().Type == lexer.TokenNameVariable {
		n.Variable = stripForVarPercents(val(p.consume()))
	}

	// "in" keyword
	if p.peekKeyword("in") {
		p.consume()
	}

	// ( set )
	if p.peek().Type == lexer.TokenPunctuation && val(p.peek()) == "(" {
		p.consume() // consume "("
		n.Set = p.collectForSet()
	}

	// "do" keyword
	if p.peekKeyword("do") {
		p.consume()
	}

	// DO body
	n.Do = p.parseInlineBody()
	return n
}

// collectForSet reads tokens from inside FOR IN( ... ) until ")".
// The lexer emits the set body as TokenText tokens.
func (p *Parser) collectForSet() []string {
	var items []string
	for p.pos < len(p.tokens) {
		t := p.peek()
		if t.Type == lexer.TokenEOF {
			break
		}
		if t.Type == lexer.TokenPunctuation && val(t) == ")" {
			p.consume()
			break
		}
		if t.Type == lexer.TokenText {
			// Split on whitespace
			for part := range strings.FieldsSeq(val(p.consume())) {
				if part != "" {
					items = append(items, part)
				}
			}
		} else {
			items = append(items, val(p.consume()))
		}
	}
	return items
}

// ---- IF condition helpers -------------------------------------------------

// peekKeyword reports whether the current token is a keyword matching kw.
func (p *Parser) peekKeyword(kw string) bool {
	t := p.peek()
	return t.Type == lexer.TokenKeyword && strings.ToLower(val(t)) == kw
}

// collectStoken collects exactly ONE "stoken" — a quoted string, a variable
// reference, or a plain word — and returns the concatenated token values.
//
// A quoted string spans one or more TokenStringDouble tokens interleaved with
// TokenNameVariable/TokenStringEscape tokens.  The closing token is detected
// by: (a) for the first token with len>1 that ends with '"' → complete; (b)
// for subsequent TokenStringDouble tokens that end with '"' → closing.
func (p *Parser) collectStoken() string {
	t := p.peek()
	if t.Type == lexer.TokenEOF {
		return ""
	}
	var sb strings.Builder
	switch t.Type {
	case lexer.TokenStringDouble:
		return p.collectQuotedString()
	case lexer.TokenStringSingle, lexer.TokenStringBT:
		sb.WriteString(val(p.consume()))
	case lexer.TokenNameVariable, lexer.TokenStringEscape:
		sb.WriteString(val(p.consume()))
	case lexer.TokenText:
		s := val(t)
		if isCommandStart(s) || isNewlineOnly(s) {
			return "" // nothing to collect
		}
		p.consume()
		sb.WriteString(s)
	}
	return sb.String()
}

// collectQuotedString reads a "..." quoted string that may span multiple tokens
// due to embedded %VAR% references.
//
// The lexer always emits the opening '"' as the first char of the first
// TokenStringDouble.  The closing '"' appears as the last char of a subsequent
// (or the same) TokenStringDouble:
//   - len>1, ends with '"': the very first token contains both open and close → done.
//   - value == '"' (first token): opening only → keep collecting until a
//     subsequent TokenStringDouble ends with '"'.
//   - otherwise (first token like '"text'): open+content, closing comes later.
func (p *Parser) collectQuotedString() string {
	var sb strings.Builder
	first := val(p.consume()) // first TokenStringDouble
	sb.WriteString(first)
	// Complete single token?
	if len(first) > 1 && strings.HasSuffix(first, `"`) {
		return sb.String()
	}
	// Opening-only or content-without-close: collect until closing '"'
	for p.pos < len(p.tokens) {
		t := p.peek()
		switch t.Type {
		case lexer.TokenStringDouble:
			v := val(p.consume())
			sb.WriteString(v)
			if strings.HasSuffix(v, `"`) {
				return sb.String() // closing found
			}
		case lexer.TokenNameVariable, lexer.TokenStringEscape:
			sb.WriteString(val(p.consume()))
		default:
			return sb.String()
		}
	}
	return sb.String()
}

// collectUntilOperator collects tokens that form the LHS of a compare condition.
// Stops when it finds a TokenOperator or TokenOperatorWord.
func (p *Parser) collectUntilOperator() string {
	var sb strings.Builder
	for p.pos < len(p.tokens) {
		t := p.peek()
		switch t.Type {
		case lexer.TokenOperator, lexer.TokenOperatorWord:
			return sb.String()
		case lexer.TokenStringDouble:
			sb.WriteString(p.collectQuotedString())
		case lexer.TokenStringSingle, lexer.TokenStringBT,
			lexer.TokenNameVariable, lexer.TokenStringEscape:
			sb.WriteString(val(p.consume()))
		case lexer.TokenText:
			s := val(t)
			if isCommandStart(s) || isNewlineOnly(s) {
				return sb.String()
			}
			p.consume()
			sb.WriteString(s)
		default:
			return sb.String()
		}
	}
	return sb.String()
}

// collectCompareOp consumes and returns the comparison operator token.
func (p *Parser) collectCompareOp() CompareOp {
	t := p.peek()
	switch t.Type {
	case lexer.TokenOperator:
		p.consume()
		return OpEqual
	case lexer.TokenOperatorWord:
		p.consume()
		return CompareOp(strings.ToLower(val(t)))
	}
	return OpEqual
}

// isCommandStart reports whether s looks like the beginning of a new command
// rather than a continuation of the current token.  A leading whitespace
// character is the key indicator from stateFollow output.
func isCommandStart(s string) bool {
	return len(s) > 0 && isWSRune(rune(s[0]))
}

// ---- utility helpers ------------------------------------------------------

// unquote strips the outermost quote characters from a string token value.
func unquote(s string) string {
	if len(s) >= 2 {
		first, last := s[0], s[len(s)-1]
		if (first == '"' && last == '"') ||
			(first == '\'' && last == '\'') ||
			(first == '`' && last == '`') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// stripForVarPercents removes surrounding % or %% from a FOR loop variable.
// e.g. "%%i" → "i", "%i" → "i"
func stripForVarPercents(s string) string {
	s = strings.TrimPrefix(s, "%%")
	s = strings.TrimPrefix(s, "%")
	s = strings.TrimSuffix(s, "%")
	return s
}
