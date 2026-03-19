package lexer

import "unicode"

// lexKeyword emits a TokenKeyword for kw if it matches at the current position.
func (bl *BatchLexer) lexKeyword(kw string) {
	if bl.tryKeyword(kw) {
		bl.emit(TokenKeyword)
	}
}

// tryKeyword consumes kw (case-insensitive) and returns true if it is followed
// by a keyword-end rune. On failure the position is reset to where it was.
func (bl *BatchLexer) tryKeyword(kw string) bool {
	for _, char := range kw {
		r := bl.next()
		if unicode.ToLower(r) != unicode.ToLower(char) {
			bl.backup()
			return false
		}
	}
	if !isKeywordEnd(bl.next()) {
		bl.backup()
		return false
	}
	bl.prev()
	return true
}

// lexPercent scans a %…% variable reference or escape starting at the
// current position (the leading % has not yet been consumed).
func (bl *BatchLexer) lexPercent() {
	bl.next() // opening %
	r := bl.next()
	switch {
	case r == '%':
		// %% could be an escaped literal % or a FOR variable (%%X or %%~modsX)
		r2 := bl.next()
		if r2 == '~' {
			// %%~modsX — FOR variable with modifiers
			bl.acceptRun(func(r rune) bool {
				return r != 0 && !isNL(r) && !IsWS(r) && !isPunct(r) && r != '"' && r != '\'' && r != '`'
			})
			bl.emit(TokenForVar)
		} else if (r2 >= 'a' && r2 <= 'z') || (r2 >= 'A' && r2 <= 'Z') {
			// %%X — FOR variable
			bl.emit(TokenForVar)
		} else {
			// %% alone or followed by non-letter — literal %
			if r2 != 0 {
				bl.prev()
			}
			bl.emit(TokenEscape)
		}
	case r >= '0' && r <= '9' || r == '*':
		bl.emit(TokenVariable)
	case r == '~':
		bl.acceptRun(func(r rune) bool {
			return r != 0 && !isNL(r) && !IsWS(r) && !isPunct(r) && r != '"' && r != '\'' && r != '`'
		})
		bl.emit(TokenVariable)
	case r == 0 || isNL(r):
		if r != 0 {
			bl.prev()
		}
		bl.emit(TokenVariable)
	default:
		for {
			r2 := bl.next()
			if r2 == '%' || r2 == 0 || isNL(r2) {
				if r2 != '%' && r2 != 0 {
					bl.prev()
				}
				break
			}
		}
		bl.emit(TokenVariable)
	}
}

// lexDelayedVar scans a !VAR! delayed-expansion variable starting at the
// current position (the leading ! has not yet been consumed).
func (bl *BatchLexer) lexDelayedVar() {
	bl.next() // opening !
	for {
		r := bl.next()
		if r == '!' || r == 0 || isNL(r) {
			if r != '!' && r != 0 {
				bl.prev()
			}
			break
		}
	}
	bl.emit(TokenDelayedExpansion)
}

// lexStringDoubleBody returns a stateFn that scans the body of a "…" string,
// then transitions to next. The opening " has already been consumed.
func (bl *BatchLexer) lexStringDoubleBody(next stateFn) stateFn {
	return func() stateFn {
		for {
			r := bl.next()
			switch r {
			case 0:
				bl.emit(TokenStringDouble)
				return nil
			case '"':
				bl.emit(TokenStringDouble)
				return next
			case '%':
				bl.prev()
				if bl.width() > 0 {
					bl.emit(TokenStringDouble)
				}
				bl.lexPercent()
				return bl.lexStringDoubleBody(next)
			case '^':
				r2 := bl.next()
				if r2 == 0 {
					bl.emit(TokenStringDouble)
					return nil
				}
			case '\\':
				// Windows CRT (not CMD itself) treats \" as a literal " inside a
				// double-quoted argument, which is how programs like gawk receive
				// arguments on Windows.  Implement the same rule here so that
				// batch files using \" to embed quotes in awk programs work on
				// Linux as well.
				r2 := bl.next()
				if r2 == 0 {
					bl.emit(TokenStringDouble)
					return nil
				}
				// r2 is already consumed into the token buffer; whether it was '"'
				// or something else, just continue scanning.
			}
		}
	}
}
