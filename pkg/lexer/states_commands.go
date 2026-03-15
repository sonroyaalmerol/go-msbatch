package lexer

import (
	"strings"
	"unicode"
)

func (bl *BatchLexer) stateSet() stateFn {
	bl.skipWS()
	if bl.check(func(r rune) bool { return r == '/' }) {
		bl.next()
		flag := bl.next()
		switch unicode.ToLower(flag) {
		case 'a':
			bl.emit(TokenKeyword)
			return bl.stateArithmetic
		case 'p':
			bl.emit(TokenKeyword)
			bl.skipWS()
			return bl.stateSetVar
		default:
			if flag != 0 {
				bl.prev()
			}
		}
	}
	return bl.stateSetVar
}

func (bl *BatchLexer) stateSetVar() stateFn {
	bl.acceptRun(func(r rune) bool {
		return r != 0 && !isNL(r) && !isWS(r) && !isPunct(r) && r != '='
	})
	if bl.width() > 0 {
		bl.emit(TokenNameVariable)
	}
	bl.skipWS()
	if bl.check(func(r rune) bool { return r == '=' }) {
		bl.next()
		bl.emit(TokenPunctuation)
	}
	return bl.stateFollow
}

func (bl *BatchLexer) stateArithmetic() stateFn {
	r := bl.next()
	switch {
	case r == 0:
		return bl.stateRoot
	case isNL(r):
		bl.prev()
		return bl.stateRoot
	case r == '|' || r == '&':
		bl.backup()
		return bl.stateRoot
	case r == ')' && bl.compoundDepth > 0:
		bl.backup()
		return bl.stateRoot
	case isWS(r):
		bl.acceptRun(isWS)
		bl.ignore()
		return bl.stateArithmetic
	case r == '(':
		bl.compoundDepth++
		bl.emit(TokenPunctuation)
		return bl.stateArithmetic
	case r == ')':
		bl.emit(TokenPunctuation)
		return bl.stateArithmetic
	case r == ',':
		bl.emit(TokenPunctuation)
		return bl.stateArithmetic
	case r == '%':
		bl.prev()
		bl.lexPercent()
		return bl.stateArithmetic
	case r == '!':
		bl.prev()
		bl.lexDelayedVar()
		return bl.stateArithmetic
	case r == '0':
		r2 := bl.next()
		if r2 == 'x' || r2 == 'X' {
			bl.acceptRun(func(r rune) bool {
				return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
			})
		} else if r2 >= '0' && r2 <= '7' {
			bl.acceptRun(func(r rune) bool { return r >= '0' && r >= '7' })
		} else if r2 != 0 {
			bl.prev()
		}
		bl.emit(TokenNumber)
		return bl.stateArithmetic
	case r >= '1' && r <= '9':
		bl.acceptRun(func(r rune) bool { return r >= '0' && r <= '9' })
		bl.emit(TokenNumber)
		return bl.stateArithmetic
	case strings.ContainsRune("=+-*/!~^", r):
		bl.acceptRun(func(r rune) bool { return strings.ContainsRune("=+-*/!~^", r) })
		bl.emit(TokenOperator)
		return bl.stateArithmetic
	default:
		bl.acceptRun(func(r rune) bool {
			return r != 0 && !isNL(r) && !isWS(r) && !isPunct(r) &&
				!strings.ContainsRune("=+-*/!~^(),", r) && r != '%' && r != '!'
		})
		if bl.width() > 0 {
			bl.emit(TokenNameVariable)
		}
	}
	return bl.stateArithmetic
}

func (bl *BatchLexer) stateFor() stateFn {
	bl.skipWS()

	isForF := false
	if bl.check(func(r rune) bool { return r == '/' }) {
		bl.next()
		flag := bl.next()
		flagLower := unicode.ToLower(flag)
		if flagLower == 'f' || flagLower == 'l' || flagLower == 'd' || flagLower == 'r' {
			bl.emit(TokenKeyword)
			bl.skipWS()
			isForF = (flagLower == 'f')
			// For /R: optionally scan a root path before the loop variable.
			if flagLower == 'r' && !bl.check(func(r rune) bool { return r == '%' }) {
				bl.acceptRun(func(r rune) bool { return r != 0 && !isNL(r) && r != '%' })
				if bl.width() > 0 {
					bl.emit(TokenText)
				}
				bl.skipWS()
			}
		} else {
			if flag != 0 {
				bl.prev()
			}
			bl.prev()
		}
	}

	// For /F: consume optional options string before the loop variable.
	if isForF && bl.check(func(r rune) bool { return r == '"' || r == '\'' || r == '`' }) {
		quoteChar := bl.next()
		for {
			r2 := bl.next()
			if r2 == quoteChar || r2 == 0 || isNL(r2) {
				if r2 != quoteChar && r2 != 0 {
					bl.prev()
				}
				break
			}
		}
		switch quoteChar {
		case '\'':
			bl.emit(TokenStringSingle)
		case '`':
			bl.emit(TokenStringBT)
		default:
			bl.emit(TokenStringDouble)
		}
		bl.skipWS()
	}

	// Consume loop variable: %%X or %X → emit as TokenNameVariable.
	if bl.check(func(r rune) bool { return r == '%' }) {
		bl.next() // first %
		if bl.check(func(r rune) bool { return r == '%' }) {
			bl.next() // second %
		}
		bl.acceptRun(func(r rune) bool {
			return r != 0 && !isNL(r) && !isWS(r) && !isPunct(r) && r != '(' && r != ')'
		})
		bl.emit(TokenNameVariable)
		bl.skipWS()
	}

	bl.lexKeyword("in")
	bl.skipWS()
	if bl.check(func(r rune) bool { return r == '(' }) {
		bl.next()
		bl.compoundDepth++
		bl.emit(TokenPunctuation)
		return bl.stateRoot
	}
	return bl.stateRoot
}

func (bl *BatchLexer) stateIf() stateFn {
	bl.skipWS()
	if bl.check(func(r rune) bool { return r == '/' }) {
		bl.next()
		r2 := bl.next()
		if unicode.ToLower(r2) == 'i' {
			r3 := bl.next()
			if isKeywordEnd(r3) {
				if r3 != 0 {
					bl.prev()
				}
				bl.emit(TokenKeyword)
				bl.skipWS()
			} else {
				if r3 != 0 {
					bl.prev()
				}
				if r2 != 0 {
					bl.prev()
				}
				bl.prev()
			}
		} else {
			if r2 != 0 {
				bl.prev()
			}
			bl.prev()
		}
	}

	if bl.tryKeyword("not") {
		bl.emit(TokenKeyword)
		bl.skipWS()
	}

	if bl.tryKeyword("exist") {
		bl.emit(TokenKeyword)
		bl.skipWS()
		return bl.stateFollow
	}

	if bl.tryKeyword("defined") {
		bl.emit(TokenKeyword)
		bl.skipWS()
		return bl.stateFollow
	}

	if bl.tryKeyword("errorlevel") {
		bl.emit(TokenKeyword)
		bl.skipWS()
		return bl.stateFollow
	}

	return bl.stateFollow
}

func (bl *BatchLexer) stateGoto() stateFn {
	bl.skipWS()
	if bl.check(func(r rune) bool { return r == ':' }) {
		bl.next()
		bl.emit(TokenPunctuation)
	}
	bl.acceptRun(func(r rune) bool { return !isWS(r) && !isNL(r) && r != 0 })
	bl.emit(TokenNameLabel)
	return bl.stateRoot
}

func (bl *BatchLexer) stateCall() stateFn {
	bl.skipWS()
	if bl.check(func(r rune) bool { return r == ':' }) {
		bl.next()
		bl.emit(TokenPunctuation)
		bl.acceptRun(func(r rune) bool { return !isWS(r) && !isNL(r) && r != 0 })
		bl.emit(TokenNameLabel)
		return bl.stateFollow
	}
	return bl.stateFollow
}
