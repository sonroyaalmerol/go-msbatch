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
			left = &PipeNode{
				Line:    left.Pos().Line,
				Col:     left.Pos().Col,
				EndLine: right.EndPos().Line,
				EndCol:  right.EndPos().Col,
				Left:    left,
				Right:   right,
			}
		} else {
			left = &BinaryNode{
				Line:    left.Pos().Line,
				Col:     left.Pos().Col,
				EndLine: right.EndPos().Line,
				EndCol:  right.EndPos().Col,
				Op:      op,
				Left:    left,
				Right:   right,
			}
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
		return &CommentNode{
			Line:    t.Line,
			Col:     t.Col,
			EndLine: t.Line,
			EndCol:  t.Col + len(t.Value),
			Text:    val(t),
		}
	}

	// Label definition :name
	if t.Type == lexer.TokenPunctuation && val(t) == ":" {
		colonLine, colonCol := t.Line, t.Col
		p.consume()
		lt := p.peek()
		if lt.Type == lexer.TokenNameLabel {
			p.consume()
			labelVal := val(lt)
			return &LabelNode{
				Line:    colonLine,
				Col:     lt.Col,
				EndLine: lt.Line,
				EndCol:  lt.Col + len(lt.Value),
				Name:    labelVal,
			}
		}
		return &LabelNode{Line: colonLine, Col: colonCol, EndLine: colonLine, EndCol: colonCol + 1}
	}

	// Keyword-dispatch
	if t.Type == lexer.TokenKeyword {
		switch strings.ToLower(val(t)) {
		case "if":
			return p.parseIf()
		case "for":
			return p.parseFor(suppressed)
		case "rem":
			return p.parseRem()
		}
	}

	// Bare label name token (seen after : in some lexer paths)
	if t.Type == lexer.TokenNameLabel {
		p.consume()
		return &LabelNode{
			Line:    t.Line,
			Col:     t.Col,
			EndLine: t.Line,
			EndCol:  t.Col + len(t.Value),
			Name:    val(t),
		}
	}

	// Skip standalone ; or , tokens (not valid command names in CMD)
	// In CMD, ; appearing outside of FOR loops is essentially ignored,
	// along with any trailing content on the same line.
	if t.Type == lexer.TokenWord && (val(t) == ";" || val(t) == ",") {
		p.consume()
		// Consume remaining tokens until newline/EOF
		for p.pos < len(p.tokens) {
			t := p.peek()
			if t.Type == lexer.TokenNewline || t.Type == lexer.TokenEOF {
				break
			}
			p.consume()
		}
		return nil
	}

	// Everything else is a simple command
	cmd := p.parseSimpleCommand(suppressed)
	if cmd == nil {
		return nil
	}
	return cmd
}

// parseSimpleCommand reads one keyword or text/word token as the command name,
// then collects argument and redirection tokens.
func (p *Parser) parseSimpleCommand(suppressed bool) *SimpleCommand {
	t := p.peek()
	if t.Type == lexer.TokenEOF || t.Type == lexer.TokenPunctuation || t.Type == lexer.TokenNewline || t.Type == lexer.TokenRedirect {
		return nil
	}
	cmd := &SimpleCommand{Suppressed: suppressed, Line: t.Line, Col: t.Col}
	firstTok := p.consume()
	cmd.Name = val(firstTok)
	endLine := firstTok.Line
	endCol := firstTok.Col + len(firstTok.Value)

	// Support commands that the lexer might have split (e.g. C:\bin\ls.exe
	// being lexed as "C" [Word] and ":\bin\ls.exe" [Text]).  Join any
	// immediately following tokens that are not separators.
	//
	// Keywords (like GOTO, CALL) are never joined; they must stand alone
	// so that following punctuation (like :) is treated as part of the
	// argument list (GOTO:EOF -> Name="GOTO", Args=[":EOF"]).
	if firstTok.Type != lexer.TokenKeyword {
		for p.pos < len(p.tokens) {
			t := p.peek()
			if t.Type == lexer.TokenWhitespace || t.Type == lexer.TokenNewline || t.Type == lexer.TokenEOF {
				break
			}
			// If it's punctuation, only join if it's not a shell operator.
			if t.Type == lexer.TokenPunctuation {
				v := val(t)
				if v == "|" || v == "&" || v == ">" || v == "<" || v == "(" || v == ")" {
					break
				}
			}
			// CMD treats '/' as a delimiter between command and switch.
			if strings.HasPrefix(val(t), "/") {
				break
			}
			consumed := p.consume()
			cmd.Name += val(consumed)
			endLine = consumed.Line
			endCol = consumed.Col + len(consumed.Value)
		}
	}

	endLine, endCol = p.collectArgs(cmd, endLine, endCol)
	cmd.EndLine = endLine
	cmd.EndCol = endCol
	return cmd
}

// collectArgs reads tokens after the command name and fills cmd.Args and cmd.Redirects.
// Returns the end line and column after processing.
func (p *Parser) collectArgs(cmd *SimpleCommand, endLine, endCol int) (int, int) {
	var cur strings.Builder

	updateEnd := func(t lexer.Item) {
		if t.Line > endLine || (t.Line == endLine && t.Col+len(t.Value) > endCol) {
			endLine = t.Line
			endCol = t.Col + len(t.Value)
		}
	}

	flushArg := func() {
		if cur.Len() > 0 {
			v := cur.String()
			cmd.Args = append(cmd.Args, v)
			cmd.RawArgs = append(cmd.RawArgs, v)
			cur.Reset()
		}
	}

	for p.pos < len(p.tokens) {
		t := p.peek()
		switch t.Type {
		case lexer.TokenEOF, lexer.TokenNewline:
			flushArg()
			return endLine, endCol

		case lexer.TokenWhitespace:
			flushArg()
			consumed := p.consume()
			updateEnd(consumed)
			cmd.RawArgs = append(cmd.RawArgs, val(consumed))

		case lexer.TokenKeyword:
			// Structural keywords emitted by specialised lexer states (e.g. "in"
			// from stateFor) are never arg terminators; include them verbatim.
			consumed := p.consume()
			cur.WriteString(val(consumed))
			updateEnd(consumed)

		case lexer.TokenWord:
			v := val(t)
			if v == "," || v == ";" {
				flushArg()
				consumed := p.consume()
				updateEnd(consumed)
				cmd.RawArgs = append(cmd.RawArgs, val(consumed))
				continue
			}
			// "else" at the top level (compoundDepth == 0) terminates the
			// then-branch of an IF statement written without enclosing parens.
			// Inside a compound block the word is a plain argument.
			if strings.ToLower(v) == "else" && p.compoundDepth == 0 {
				flushArg()
				return endLine, endCol
			}
			consumed := p.consume()
			cur.WriteString(val(consumed))
			updateEnd(consumed)

		case lexer.TokenPunctuation:
			v := val(t)
			if v == "=" {
				flushArg()
				consumed := p.consume()
				updateEnd(consumed)
				cmd.RawArgs = append(cmd.RawArgs, val(consumed))
				continue
			}
			if isPipeOrAmpVal(v) || (v == ")" && p.compoundDepth > 0) {
				flushArg()
				return endLine, endCol
			}
			consumed := p.consume()
			cur.WriteString(val(consumed))
			updateEnd(consumed)

		case lexer.TokenRedirect:
			flushArg()
			el, ec := p.collectRedirect(cmd, endLine, endCol)
			endLine, endCol = el, ec

		case lexer.TokenStringDouble, lexer.TokenStringSingle, lexer.TokenStringBT:
			quoted, lastTok := p.collectQuotedStringWithToken()
			cur.WriteString(quoted)
			updateEnd(lastTok)

		default:
			consumed := p.consume()
			cur.WriteString(val(consumed))
			updateEnd(consumed)
		}
	}
	flushArg()
	return endLine, endCol
}

// collectRedirect reads a TokenRedirect and its target from the stream.
// Returns the end line and column after processing.
func (p *Parser) collectRedirect(cmd *SimpleCommand, endLine, endCol int) (int, int) {
	rt := p.consume() // TokenRedirect
	if rt.Line > endLine || (rt.Line == endLine && rt.Col+len(rt.Value) > endCol) {
		endLine = rt.Line
		endCol = rt.Col + len(rt.Value)
	}
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
			numTok := p.consume()
			r.Target = val(numTok)
			if numTok.Line > endLine || (numTok.Line == endLine && numTok.Col+len(numTok.Value) > endCol) {
				endLine = numTok.Line
				endCol = numTok.Col + len(numTok.Value)
			}
		}
		cmd.Redirects = append(cmd.Redirects, r)
		return endLine, endCol
	case strings.Contains(v, "<&"):
		r.Kind = RedirectInFD
		r.FD = extractFD(v, "<&", 0)
		if p.pos < len(p.tokens) && p.tokens[p.pos].Type == lexer.TokenNumber {
			numTok := p.consume()
			r.Target = val(numTok)
			if numTok.Line > endLine || (numTok.Line == endLine && numTok.Col+len(numTok.Value) > endCol) {
				endLine = numTok.Line
				endCol = numTok.Col + len(numTok.Value)
			}
		}
		cmd.Redirects = append(cmd.Redirects, r)
		return endLine, endCol
	case strings.Contains(v, ">"):
		r.Kind = RedirectOut
		r.FD = extractFD(v, ">", 1)
	case strings.Contains(v, "<"):
		r.Kind = RedirectIn
		r.FD = extractFD(v, "<", 0)
	}

	p.skipWS() // skip to target
	t := p.peek()
	if t.Type != lexer.TokenEOF && t.Type != lexer.TokenNewline && t.Type != lexer.TokenPunctuation {
		r.Target, endLine, endCol = p.collectStokenWithPos(endLine, endCol)
	}
	cmd.Redirects = append(cmd.Redirects, r)
	return endLine, endCol
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
	remTok := p.consume() // "rem" keyword
	endLine := remTok.Line
	endCol := remTok.Col + len(remTok.Value)
	var sb strings.Builder
	for p.pos < len(p.tokens) {
		t := p.peek()
		if t.Type == lexer.TokenComment {
			consumed := p.consume()
			sb.WriteString(val(consumed))
			endLine = consumed.Line
			endCol = consumed.Col + len(consumed.Value)
			break
		}
		if t.Type == lexer.TokenNewline || t.Type == lexer.TokenEOF {
			break
		}
		consumed := p.consume()
		endLine = consumed.Line
		endCol = consumed.Col + len(consumed.Value)
	}
	return &CommentNode{
		Line:    remTok.Line,
		Col:     remTok.Col,
		EndLine: endLine,
		EndCol:  endCol,
		Text:    sb.String(),
	}
}
