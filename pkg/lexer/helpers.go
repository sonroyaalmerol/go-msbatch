package lexer

import "strings"

func isWS(r rune) bool {
	return r == ' ' || r == '\t' || r == '\v' || r == '\f' || r == '\xa0'
}

func isNL(r rune) bool {
	return r == '\r' || r == '\n'
}

func isPunct(r rune) bool {
	return strings.ContainsRune("()@:|[&><", r)
}

func isKeywordEnd(r rune) bool {
	return r == 0 || isNL(r) || isWS(r) || isPunct(r) || r == '/'
}

func (bl *BatchLexer) skipWS() {
	bl.acceptRun(isWS)
	if bl.width() > 0 {
		bl.emit(TokenWhitespace)
	}
}

func (bl *BatchLexer) isFollowPlain(r rune) bool {
	if r == 0 || isNL(r) || isWS(r) {
		return false
	}
	if r == '|' || r == '&' || r == '>' || r == '<' {
		return false
	}
	if r == '"' || r == '%' || r == '!' || r == '^' || r == '`' {
		return false
	}
	if r == ')' && bl.compoundDepth > 0 {
		return false
	}
	return true
}

// drainBuf returns the runes buffered since the last Emit/Ignore as a string.
// It does not change pos.
func (bl *BatchLexer) drainBuf() string {
	return string(bl.input[bl.start:bl.pos])
}
