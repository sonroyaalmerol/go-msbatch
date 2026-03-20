package lexer

import "strings"

func (bl *BatchLexer) stateRoot() stateFn {
	r := bl.next()
	switch {
	case r == 0:
		return nil
	case isNL(r):
		bl.acceptRun(isNL)
		bl.emit(TokenNewline)
		bl.atCommandStart = true
		return bl.stateRoot
	case IsWS(r):
		bl.acceptRun(IsWS)
		bl.emit(TokenWhitespace)
		return bl.stateRoot
	case r == '(':
		bl.atCommandStart = false
		bl.compoundDepth++
		bl.emit(TokenPunctuation)
		return bl.stateRoot
	case r == ')':
		bl.atCommandStart = false
		if bl.compoundDepth > 0 {
			bl.compoundDepth--
		}
		bl.emit(TokenPunctuation)
		return bl.stateRoot
	case r == '@':
		// @ is an echo-suppression prefix; treat it as transparent to label
		// detection so that @:label still defines a label.
		bl.acceptRun(func(r rune) bool { return r == '@' })
		bl.emit(TokenPunctuation)
		return bl.stateRoot
	case r == ':':
		if bl.check(func(r rune) bool { return r == ':' }) {
			bl.next()
			bl.acceptRun(func(r rune) bool { return !isNL(r) && r != 0 })
			bl.emit(TokenComment)
			bl.atCommandStart = false
			return bl.stateRoot
		}
		// A colon defines a label only when it is the first non-whitespace
		// token on the line.  In all other positions it is plain punctuation.
		isAtStart := bl.atCommandStart
		bl.atCommandStart = false
		bl.emit(TokenPunctuation)
		if isAtStart {
			return bl.stateLabelName
		}
		return bl.stateRoot
	case r == '|' || r == '&':
		bl.atCommandStart = false
		bl.acceptRun(func(r rune) bool { return r == '|' || r == '&' })
		bl.emit(TokenPunctuation)
		return bl.stateRoot
	case r == '>' || r == '<':
		bl.atCommandStart = false
		return bl.stateRedirectRune(r)
	case r == '"':
		bl.atCommandStart = false
		return bl.lexStringDoubleBody(bl.stateRoot)()
	case r == '^':
		bl.atCommandStart = false
		r2 := bl.next()
		if r2 == 0 {
			bl.emit(TokenEscape)
			return nil
		}
		if isNL(r2) {
			bl.acceptRun(isNL)
			bl.ignore()
		} else {
			bl.start++
			bl.emit(TokenEscape)
		}
		return bl.stateRoot
	case r == '%':
		bl.atCommandStart = false
		bl.prev()
		bl.lexPercent()
		return bl.stateRoot
	case r == '!':
		bl.atCommandStart = false
		bl.prev()
		bl.lexDelayedVar()
		return bl.stateRoot
	case r == '=':
		bl.atCommandStart = false
		r2 := bl.next()
		if r2 == '=' {
			bl.emit(TokenOperator) // emit "=="
		} else {
			if r2 != 0 && !isNL(r2) {
				bl.prev()
			}
			bl.emit(TokenPunctuation) // emit "="
		}
		return bl.stateRoot
	case r == '/':
		bl.atCommandStart = false
		bl.emit(TokenPunctuation)
		return bl.stateRoot
	case r >= '0' && r <= '9':
		bl.atCommandStart = false
		bl.acceptRun(func(r rune) bool { return r >= '0' && r <= '9' })
		nextRune := bl.next()
		if nextRune == '>' || nextRune == '<' {
			bl.prev()
			return bl.stateRedirect()
		}
		for i := 0; i < bl.width(); i++ {
			bl.backup()
		}
		return bl.stateWord
	default:
		bl.atCommandStart = false
		bl.prev()
		return bl.stateWord
	}
}

func (bl *BatchLexer) stateWord() stateFn {
	bl.acceptRun(func(r rune) bool {
		return r != 0 && !isNL(r) && !IsWS(r) && !isPunct(r) &&
			r != '(' && r != ')' && r != '"' && r != '%' &&
			r != '!' && r != '^' && r != '>' && r != '<' && r != ':' && r != '=' && r != '/'
	})
	if bl.width() == 0 {
		r := bl.next()
		if r == 0 {
			return nil
		}
		bl.prev()
		return bl.stateRoot
	}
	word := bl.drainBuf()
	lower := strings.ToLower(word)

	if word == "==" {
		bl.emit(TokenOperator)
		return bl.stateFollow
	}

	if entry, ok := keywordTable[lower]; ok {
		bl.emit(TokenKeyword)
		if entry.next != nil {
			return entry.next(bl)
		}
		return bl.stateFollow
	}
	bl.emit(TokenWord)
	return bl.stateFollow
}

func (bl *BatchLexer) stateFollow() stateFn {
	bl.acceptRun(bl.isFollowPlain)
	if bl.width() > 0 {
		bl.emit(TokenText)
	}
	r := bl.next()
	if r == 0 {
		return bl.stateRoot
	}
	bl.prev()
	return bl.stateRoot
}

func (bl *BatchLexer) stateRem() stateFn {
	bl.acceptRun(func(r rune) bool { return !isNL(r) && r != 0 })
	bl.emit(TokenComment)
	return bl.stateRoot
}

func (bl *BatchLexer) stateLabelName() stateFn {
	bl.acceptRun(func(r rune) bool { return !IsWS(r) && !isNL(r) && r != 0 })
	bl.emit(TokenLabel)
	// In CMD, everything after the label name on a label line is a comment.
	// Discard it so ":021 TIDAL" defines label "021" with "TIDAL" ignored.
	bl.acceptRun(func(r rune) bool { return !isNL(r) && r != 0 })
	bl.ignore()
	return bl.stateRoot
}

func (bl *BatchLexer) stateRedirectRune(r rune) stateFn {
	if r == '>' && bl.check(func(r rune) bool { return r == '>' }) {
		bl.next()
	}
	if bl.check(func(r rune) bool { return r == '&' }) {
		bl.next()
	}
	bl.emit(TokenRedirect)
	return bl.stateFollow
}

func (bl *BatchLexer) stateRedirect() stateFn {
	bl.accept(func(r rune) bool { return r >= '0' && r <= '9' })
	r := bl.next()
	if r != '>' && r != '<' {
		if r != 0 {
			bl.prev()
		}
		if bl.width() > 0 {
			bl.emit(TokenText)
		}
		return bl.stateRoot
	}
	return bl.stateRedirectRune(r)
}
