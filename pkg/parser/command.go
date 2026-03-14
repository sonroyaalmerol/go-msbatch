package parser

import (
	"strconv"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/lexer"
)

// parseBinary handles |, &, ||, && at the top level.
func (p *Parser) parseBinary() Node {
	left := p.parsePrimary()
	if left == nil {
		return nil
	}
	for {
		p.skipWS()
		t := p.peek()
		if t.Type != lexer.TokenPunctuation {
			break
		}
		v := val(t)
		var op NodeKind
		switch v {
		case "|":
			op = NodePipe
		case "||":
			op = NodeOrElse
		case "&":
			op = NodeConcat
		case "&&":
			op = NodeAndThen
		default:
			break
		}
		if op == 0 {
			break
		}
		p.consume() // consume operator
		right := p.parsePrimary()
		if right == nil {
			break
		}
		if op == NodePipe {
			left = &PipeNode{Left: left, Right: right}
		} else {
			left = &BinaryNode{Op: op, Left: left, Right: right}
		}
	}
	return left
}

// parsePrimary parses a single command: block, label, comment, or simple command.
func (p *Parser) parsePrimary() Node {
	p.skipWS()
	t := p.peek()
	if t.Type == lexer.TokenEOF {
		return nil
	}

	// @ suppression prefix
	suppressed := false
	if t.Type == lexer.TokenPunctuation && val(t) == "@" {
		suppressed = true
		p.consume()
		p.skipWS()
		t = p.peek()
	}

	// Compound block
	if t.Type == lexer.TokenPunctuation && val(t) == "(" {
		return p.parseBlock()
	}

	// :: comment
	if t.Type == lexer.TokenComment {
		p.consume()
		return &CommentNode{Text: val(t)}
	}

	// Label definition :name
	if t.Type == lexer.TokenPunctuation && val(t) == ":" {
		p.consume()
		lt := p.peek()
		if lt.Type == lexer.TokenNameLabel {
			p.consume()
			return &LabelNode{Name: val(lt)}
		}
		return &LabelNode{}
	}

	// Keyword-dispatch
	if t.Type == lexer.TokenKeyword {
		switch strings.ToLower(val(t)) {
		case "if":
			return p.parseIf(suppressed)
		case "for":
			return p.parseFor(suppressed)
		case "rem":
			return p.parseRem()
		}
	}

	// Bare label name token (seen after : in some lexer paths)
	if t.Type == lexer.TokenNameLabel {
		p.consume()
		return &LabelNode{Name: val(t)}
	}

	// Everything else is a simple command
	return p.parseSimpleCommand(suppressed)
}

// parseSimpleCommand reads one keyword or text token as the command name,
// then collects argument and redirection tokens.
func (p *Parser) parseSimpleCommand(suppressed bool) *SimpleCommand {
	t := p.peek()
	if t.Type == lexer.TokenEOF || t.Type == lexer.TokenPunctuation {
		return nil
	}
	cmd := &SimpleCommand{Suppressed: suppressed}
	cmd.Name = strings.TrimSpace(val(p.consume()))
	p.collectArgs(cmd)
	return cmd
}

// collectArgs reads tokens after the command name and fills cmd.Args and cmd.Redirects.
func (p *Parser) collectArgs(cmd *SimpleCommand) {
	var cur strings.Builder

	flushArg := func() {
		if s := cur.String(); s != "" {
			cmd.Args = append(cmd.Args, s)
			cur.Reset()
		}
	}

	for p.pos < len(p.tokens) {
		t := p.peek()
		switch t.Type {
		case lexer.TokenEOF:
			flushArg()
			return

		case lexer.TokenPunctuation:
			v := val(t)
			if isPipeOrAmpVal(v) || v == ")" {
				flushArg()
				return
			}
			// Other punctuation (like leftover "@") — skip
			p.consume()

		case lexer.TokenRedirect:
			flushArg()
			p.collectRedirect(cmd)

		case lexer.TokenText:
			s := val(p.consume())
			// A token that is only newline characters ends the command.
			if isNewlineOnly(s) {
				flushArg()
				return
			}
			// Split on whitespace; whitespace segments flush the current arg.
			splitTextArgs(s, &cur, cmd)

		default:
			// TokenNameVariable, TokenStringDouble, TokenStringEscape,
			// TokenNumber, TokenOperator, TokenOperatorWord, TokenKeyword,
			// TokenComment, TokenNameLabel — concatenate to current arg.
			p.consume()
			cur.WriteString(val(t))
		}
	}
	flushArg()
}

// splitTextArgs splits a raw text token by whitespace, flushing the current
// arg builder on each whitespace boundary.
func splitTextArgs(s string, cur *strings.Builder, cmd *SimpleCommand) {
	runes := []rune(s)
	i := 0
	for i < len(runes) {
		// consume whitespace run → flush arg
		j := i
		for j < len(runes) && isWSRune(runes[j]) {
			j++
		}
		if j > i {
			if cur.Len() > 0 {
				cmd.Args = append(cmd.Args, cur.String())
				cur.Reset()
			}
			i = j
			continue
		}
		// consume non-whitespace run → build arg
		j = i
		for j < len(runes) && !isWSRune(runes[j]) {
			j++
		}
		cur.WriteString(string(runes[i:j]))
		i = j
	}
}

// collectRedirect reads a TokenRedirect and its target from the stream.
func (p *Parser) collectRedirect(cmd *SimpleCommand) {
	rt := p.consume() // TokenRedirect
	v := val(rt)
	r := Redirect{}

	switch {
	case strings.Contains(v, ">>"):
		r.Kind = RedirectAppend
		r.FD = extractFD(v, ">>", 1)
	case strings.Contains(v, ">&"):
		r.Kind = RedirectOutFD
		r.FD = extractFD(v, ">&", 1)
		if p.pos < len(p.tokens) && p.tokens[p.pos].Type == lexer.TokenNumber {
			r.Target = val(p.consume())
		}
		cmd.Redirects = append(cmd.Redirects, r)
		return
	case strings.Contains(v, "<&"):
		r.Kind = RedirectInFD
		r.FD = extractFD(v, "<&", 0)
		if p.pos < len(p.tokens) && p.tokens[p.pos].Type == lexer.TokenNumber {
			r.Target = val(p.consume())
		}
		cmd.Redirects = append(cmd.Redirects, r)
		return
	case strings.Contains(v, ">"):
		r.Kind = RedirectOut
		r.FD = extractFD(v, ">", 1)
	case strings.Contains(v, "<"):
		r.Kind = RedirectIn
		r.FD = extractFD(v, "<", 0)
	}

	// Target file is the next TokenText
	if p.pos < len(p.tokens) && p.tokens[p.pos].Type == lexer.TokenText {
		r.Target = strings.TrimSpace(val(p.consume()))
	}
	cmd.Redirects = append(cmd.Redirects, r)
}

// extractFD parses an optional leading digit from a redirect token like "2>".
func extractFD(token, op string, defaultFD int) int {
	prefix := strings.TrimSuffix(token, op)
	if prefix == "" {
		return defaultFD
	}
	n, err := strconv.Atoi(prefix)
	if err != nil {
		return defaultFD
	}
	return n
}

// parseRem consumes "rem" and its comment body.
func (p *Parser) parseRem() Node {
	p.consume() // "rem" keyword
	var sb strings.Builder
	for p.pos < len(p.tokens) {
		t := p.peek()
		if t.Type == lexer.TokenComment {
			sb.WriteString(val(p.consume()))
			break
		}
		if isNewlineToken(t) {
			break
		}
		// rare: consume stray tokens
		p.consume()
	}
	return &CommentNode{Text: sb.String()}
}

func isNewlineToken(t item) bool {
	if t.Type != lexer.TokenText {
		return false
	}
	return isNewlineOnly(val(t))
}

// isNewlineOnly reports whether s contains only CR/LF characters.
func isNewlineOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r != '\n' && r != '\r' {
			return false
		}
	}
	return true
}
