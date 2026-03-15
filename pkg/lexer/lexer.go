package lexer

import (
	"strings"
	"unicode"

	"github.com/sonroyaalmerol/go-msbatch/internal/lex"
)

type TokenType int

const (
	TokenEOF TokenType = iota
	TokenError
	TokenText
	TokenPunctuation
	TokenKeyword
	TokenComment
	TokenNameLabel
	TokenNameVariable
	TokenStringDouble
	TokenStringSingle
	TokenStringBT
	TokenStringEscape
	TokenNumber
	TokenOperator
	TokenRedirect
	TokenWhitespace
	TokenNewline
	TokenWord
)

func (t TokenType) String() string {
	names := map[TokenType]string{
		TokenEOF: "EOF", TokenError: "Error", TokenText: "Text",
		TokenPunctuation: "Punctuation", TokenKeyword: "Keyword",
		TokenComment: "Comment", TokenNameLabel: "Name.Label",
		TokenNameVariable: "Name.Variable", TokenStringDouble: "String.Double",
		TokenStringSingle: "String.Single", TokenStringBT: "String.Backtick",
		TokenStringEscape: "String.Escape", TokenNumber: "Number",
		TokenOperator: "Operator",
		TokenRedirect: "Redirect", TokenWhitespace: "Whitespace",
		TokenNewline: "Newline", TokenWord: "Word",
	}
	if s, ok := names[t]; ok {
		return s
	}
	return "Unknown"
}

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

type BatchLexer struct {
	inner         lex.Lexer[TokenType, rune]
	compoundDepth int
}

func New(src string) *BatchLexer {
	bl := &BatchLexer{}
	bl.inner = lex.New(bl.stateRoot, []rune(src))
	return bl
}

func (bl *BatchLexer) NextItem() lex.Item[TokenType, rune] {
	return bl.inner.NextItem()
}

func skipWS(l lex.Lexer[TokenType, rune]) {
	l.AcceptRun(isWS)
	if l.Width() > 0 {
		l.Emit(TokenWhitespace)
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

func (bl *BatchLexer) stateRoot(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	r := l.Next()
	switch {
	case r == 0:
		return nil
	case isNL(r):
		l.AcceptRun(isNL)
		l.Emit(TokenNewline)
		return bl.stateRoot
	case isWS(r):
		l.AcceptRun(isWS)
		l.Emit(TokenWhitespace)
		return bl.stateRoot
	case r == '(':
		bl.compoundDepth++
		l.Emit(TokenPunctuation)
		return bl.stateRoot
	case r == ')':
		if bl.compoundDepth > 0 {
			bl.compoundDepth--
		}
		l.Emit(TokenPunctuation)
		return bl.stateRoot
	case r == '@':
		l.AcceptRun(func(r rune) bool { return r == '@' })
		l.Emit(TokenPunctuation)
		return bl.stateRoot
	case r == ':':
		if l.Check(func(r rune) bool { return r == ':' }) {
			l.Next()
			l.AcceptRun(func(r rune) bool { return !isNL(r) && r != 0 })
			l.Emit(TokenComment)
			return bl.stateRoot
		}
		l.Emit(TokenPunctuation)
		return bl.stateLabelName
	case r == '|' || r == '&':
		l.AcceptRun(func(r rune) bool { return r == '|' || r == '&' })
		l.Emit(TokenPunctuation)
		return bl.stateRoot
	case r == '>' || r == '<':
		return bl.stateRedirectRune(l, r)
	case r == '"':
		return bl.lexStringDoubleBody(bl.stateRoot)(l)
	case r == '`':
		return bl.lexStringBTBody(bl.stateRoot)(l)
	case r == '^':
		r2 := l.Next()
		if r2 == 0 {
			l.Emit(TokenStringEscape)
			return nil
		}
		if isNL(r2) {
			l.Ignore()
		} else {
			l.Emit(TokenStringEscape)
		}
		return bl.stateRoot
	case r == '%':
		l.Backup()
		bl.lexPercent(l)
		return bl.stateRoot
	case r == '!':
		l.Backup()
		bl.lexDelayedVar(l)
		return bl.stateRoot
	case r >= '0' && r <= '9':
		l.AcceptRun(func(r rune) bool { return r >= '0' && r <= '9' })
		nextRune := l.Next()
		if nextRune == '>' || nextRune == '<' {
			l.Backup()
			l.Ignore()
			return bl.stateRedirect(l)
		}
		for i := 0; i < l.Width(); i++ {
			l.Backup()
		}
		return bl.stateWord
	case r == 0:
		l.Emit(TokenEOF)
		return nil
	default:
		l.Backup()
		return bl.stateWord
	}
}

func (bl *BatchLexer) stateWord(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	l.AcceptRun(func(r rune) bool {
		return r != 0 && !isNL(r) && !isWS(r) && !isPunct(r) &&
			r != '(' && r != ')' && r != '"' && r != '%' &&
			r != '!' && r != '^' && r != '>' && r != '<' && r != ':'
	})
	if l.Width() == 0 {
		r := l.Next()
		if r == 0 {
			return nil
		}
		l.Prev()
		return bl.stateRoot
	}
	word := bl.drainBuf(l)
	lower := strings.ToLower(word)

	if word == "==" {
		l.Emit(TokenOperator)
		return bl.stateFollow
	}

	switch lower {
	case "rem":
		l.Emit(TokenKeyword)
		return bl.stateRem
	case "set":
		l.Emit(TokenKeyword)
		return bl.stateSet
	case "for":
		l.Emit(TokenKeyword)
		return bl.stateFor
	case "if":
		l.Emit(TokenKeyword)
		return bl.stateIf
	case "else":
		l.Emit(TokenKeyword)
		return bl.stateRoot
	case "goto":
		l.Emit(TokenKeyword)
		return bl.stateGoto
	case "call":
		l.Emit(TokenKeyword)
		return bl.stateCall
	case "do":
		l.Emit(TokenKeyword)
		return bl.stateRoot
	case "in":
		l.Emit(TokenKeyword)
		return bl.stateRoot
	default:
		l.Emit(TokenWord)
		return bl.stateFollow
	}
}

func (bl *BatchLexer) drainBuf(l lex.Lexer[TokenType, rune]) string {
	w := l.Width()
	for range w {
		l.Backup()
	}
	var sb strings.Builder
	for range w {
		sb.WriteRune(l.Next())
	}
	return sb.String()
}

func (bl *BatchLexer) stateFollow(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	l.AcceptRun(bl.isFollowPlain)
	if l.Width() > 0 {
		l.Emit(TokenText)
	}
	r := l.Next()
	switch {
	case r == 0:
		return bl.stateRoot
	case isNL(r):
		l.Prev()
		return bl.stateRoot
	case isWS(r):
		l.Prev()
		return bl.stateRoot
	case r == '|' || r == '&':
		l.Prev()
		return bl.stateRoot
	case r == ')' || r == '(':
		l.Prev()
		return bl.stateRoot
	case r == '>' || r == '<':
		l.Prev()
		return bl.stateRoot
	case r == '"':
		l.Prev()
		return bl.stateRoot
	case r == '%':
		l.Prev()
		return bl.stateRoot
	case r == '!':
		l.Prev()
		return bl.stateRoot
	case r == '^':
		l.Prev()
		return bl.stateRoot
	default:
		l.Prev()
		return bl.stateRoot
	}
}

func (bl *BatchLexer) stateRem(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	l.AcceptRun(func(r rune) bool { return !isNL(r) && r != 0 })
	l.Emit(TokenComment)
	return bl.stateRoot
}

func (bl *BatchLexer) stateSet(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	skipWS(l)
	if l.Check(func(r rune) bool { return r == '/' }) {
		l.Next()
		flag := l.Next()
		switch unicode.ToLower(flag) {
		case 'a':
			l.Emit(TokenKeyword)
			return bl.stateArithmetic
		case 'p':
			l.Emit(TokenKeyword)
			skipWS(l)
			return bl.stateSetVar
		default:
			if flag != 0 {
				l.Prev()
			}
		}
	}
	return bl.stateSetVar
}

func (bl *BatchLexer) stateSetVar(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	l.AcceptRun(func(r rune) bool {
		return r != 0 && !isNL(r) && !isWS(r) && !isPunct(r) && r != '='
	})
	if l.Width() > 0 {
		l.Emit(TokenNameVariable)
	}
	skipWS(l)
	if l.Check(func(r rune) bool { return r == '=' }) {
		l.Next()
		l.Emit(TokenPunctuation)
	}
	return bl.stateFollow
}

func (bl *BatchLexer) stateArithmetic(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	r := l.Next()
	switch {
	case r == 0:
		return bl.stateRoot
	case isNL(r):
		l.Prev()
		return bl.stateRoot
	case r == '|' || r == '&':
		l.Backup()
		return bl.stateRoot
	case r == ')' && bl.compoundDepth > 0:
		l.Backup()
		return bl.stateRoot
	case isWS(r):
		l.AcceptRun(isWS)
		l.Ignore()
		return bl.stateArithmetic
	case r == '(':
		bl.compoundDepth++
		l.Emit(TokenPunctuation)
		return bl.stateArithmetic
	case r == ')':
		l.Emit(TokenPunctuation)
		return bl.stateArithmetic
	case r == ',':
		l.Emit(TokenPunctuation)
		return bl.stateArithmetic
	case r == '%':
		l.Backup()
		bl.lexPercent(l)
		return bl.stateArithmetic
	case r == '!':
		l.Backup()
		bl.lexDelayedVar(l)
		return bl.stateArithmetic
	case r == '0':
		r2 := l.Next()
		if r2 == 'x' || r2 == 'X' {
			l.AcceptRun(func(r rune) bool {
				return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
			})
		} else if r2 >= '0' && r2 <= '7' {
			l.AcceptRun(func(r rune) bool { return r >= '0' && r >= '7' })
		} else if r2 != 0 {
			l.Prev()
		}
		l.Emit(TokenNumber)
		return bl.stateArithmetic
	case r >= '1' && r <= '9':
		l.AcceptRun(func(r rune) bool { return r >= '0' && r <= '9' })
		l.Emit(TokenNumber)
		return bl.stateArithmetic
	case strings.ContainsRune("=+-*/!~^", r):
		l.AcceptRun(func(r rune) bool { return strings.ContainsRune("=+-*/!~^", r) })
		l.Emit(TokenOperator)
		return bl.stateArithmetic
	default:
		l.AcceptRun(func(r rune) bool {
			return r != 0 && !isNL(r) && !isWS(r) && !isPunct(r) &&
				!strings.ContainsRune("=+-*/!~^(),", r) && r != '%' && r != '!'
		})
		if l.Width() > 0 {
			l.Emit(TokenNameVariable)
		}
	}
	return bl.stateArithmetic
}

func (bl *BatchLexer) stateFor(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	skipWS(l)

	isForF := false
	if l.Check(func(r rune) bool { return r == '/' }) {
		l.Next()
		flag := l.Next()
		flagLower := unicode.ToLower(flag)
		if flagLower == 'f' || flagLower == 'l' || flagLower == 'd' || flagLower == 'r' {
			l.Emit(TokenKeyword)
			skipWS(l)
			isForF = (flagLower == 'f')
			// For /R: optionally scan a root path before the loop variable.
			if flagLower == 'r' && !l.Check(func(r rune) bool { return r == '%' }) {
				l.AcceptRun(func(r rune) bool { return r != 0 && !isNL(r) && r != '%' })
				if l.Width() > 0 {
					l.Emit(TokenText)
				}
				skipWS(l)
			}
		} else {
			if flag != 0 {
				l.Prev()
			}
			l.Prev()
		}
	}

	// For /F: consume optional options string before the loop variable.
	if isForF && l.Check(func(r rune) bool { return r == '"' || r == '\'' || r == '`' }) {
		quoteChar := l.Next()
		for {
			r2 := l.Next()
			if r2 == quoteChar || r2 == 0 || isNL(r2) {
				if r2 != quoteChar && r2 != 0 {
					l.Prev()
				}
				break
			}
		}
		switch quoteChar {
		case '\'':
			l.Emit(TokenStringSingle)
		case '`':
			l.Emit(TokenStringBT)
		default:
			l.Emit(TokenStringDouble)
		}
		skipWS(l)
	}

	// Consume loop variable: %%X or %X → emit as TokenNameVariable.
	if l.Check(func(r rune) bool { return r == '%' }) {
		l.Next() // first %
		if l.Check(func(r rune) bool { return r == '%' }) {
			l.Next() // second %
		}
		l.AcceptRun(func(r rune) bool {
			return r != 0 && !isNL(r) && !isWS(r) && !isPunct(r) && r != '(' && r != ')'
		})
		l.Emit(TokenNameVariable)
		skipWS(l)
	}

	bl.lexKeyword(l, "in")
	skipWS(l)
	if l.Check(func(r rune) bool { return r == '(' }) {
		l.Next()
		bl.compoundDepth++
		l.Emit(TokenPunctuation)
		return bl.stateRoot
	}
	return bl.stateRoot
}

func (bl *BatchLexer) stateIf(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	skipWS(l)
	if l.Check(func(r rune) bool { return r == '/' }) {
		l.Next()
		r2 := l.Next()
		if unicode.ToLower(r2) == 'i' {
			r3 := l.Next()
			if isKeywordEnd(r3) {
				if r3 != 0 {
					l.Prev()
				}
				l.Emit(TokenKeyword)
				skipWS(l)
			} else {
				if r3 != 0 {
					l.Prev()
				}
				if r2 != 0 {
					l.Prev()
				}
				l.Prev()
			}
		} else {
			if r2 != 0 {
				l.Prev()
			}
			l.Prev()
		}
	}

	if bl.tryKeyword(l, "not") {
		l.Emit(TokenKeyword)
		skipWS(l)
	}

	if bl.tryKeyword(l, "exist") {
		l.Emit(TokenKeyword)
		skipWS(l)
		return bl.stateFollow
	}

	if bl.tryKeyword(l, "defined") {
		l.Emit(TokenKeyword)
		skipWS(l)
		return bl.stateFollow
	}

	if bl.tryKeyword(l, "errorlevel") {
		l.Emit(TokenKeyword)
		skipWS(l)
		return bl.stateFollow
	}

	return bl.stateFollow
}

func (bl *BatchLexer) stateGoto(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	skipWS(l)
	if l.Check(func(r rune) bool { return r == ':' }) {
		l.Next()
		l.Emit(TokenPunctuation)
	}
	l.AcceptRun(func(r rune) bool { return !isWS(r) && !isNL(r) && r != 0 })
	l.Emit(TokenNameLabel)
	return bl.stateRoot
}

func (bl *BatchLexer) stateCall(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	skipWS(l)
	if l.Check(func(r rune) bool { return r == ':' }) {
		l.Next()
		l.Emit(TokenPunctuation)
		l.AcceptRun(func(r rune) bool { return !isWS(r) && !isNL(r) && r != 0 })
		l.Emit(TokenNameLabel)
		return bl.stateFollow
	}
	return bl.stateFollow
}

func (bl *BatchLexer) stateLabelName(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	l.AcceptRun(func(r rune) bool { return !isWS(r) && !isNL(r) && r != 0 })
	l.Emit(TokenNameLabel)
	return bl.stateRoot
}

func (bl *BatchLexer) stateRedirectRune(l lex.Lexer[TokenType, rune], r rune) lex.StateFn[TokenType, rune] {
	if r == '>' && l.Check(func(r rune) bool { return r == '>' }) {
		l.Next()
	}
	if l.Check(func(r rune) bool { return r == '&' }) {
		l.Next()
	}
	l.Emit(TokenRedirect)
	return bl.stateFollow
}

func (bl *BatchLexer) stateRedirect(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	l.Accept(func(r rune) bool { return r >= '0' && r <= '9' })
	r := l.Next()
	if r != '>' && r != '<' {
		if r != 0 {
			l.Prev()
		}
		if l.Width() > 0 {
			l.Emit(TokenText)
		}
		return bl.stateRoot
	}
	return bl.stateRedirectRune(l, r)
}

func (bl *BatchLexer) lexKeyword(l lex.Lexer[TokenType, rune], kw string) {
	if bl.tryKeyword(l, kw) {
		l.Emit(TokenKeyword)
	}
}

func (bl *BatchLexer) tryKeyword(l lex.Lexer[TokenType, rune], kw string) bool {
	for _, char := range kw {
		r := l.Next()
		if unicode.ToLower(r) != unicode.ToLower(char) {
			for i := 0; i < l.Width(); i++ {
				l.Backup()
			}
			return false
		}
	}
	if !isKeywordEnd(l.Next()) {
		for i := 0; i < l.Width(); i++ {
			l.Backup()
		}
		return false
	}
	l.Prev()
	return true
}

func (bl *BatchLexer) lexPercent(l lex.Lexer[TokenType, rune]) {
	l.Next()
	r := l.Next()
	switch {
	case r == '%':
		l.Emit(TokenStringEscape)
	case r >= '0' && r <= '9' || r == '*':
		l.Emit(TokenNameVariable)
	case r == '~':
		l.AcceptRun(func(r rune) bool { return r != 0 && !isNL(r) && !isWS(r) && !isPunct(r) })
		l.Emit(TokenNameVariable)
	case r == 0 || isNL(r):
		if r != 0 {
			l.Prev()
		}
		l.Emit(TokenNameVariable)
	default:
		for {
			r2 := l.Next()
			if r2 == '%' || r2 == 0 || isNL(r2) {
				if r2 != '%' && r2 != 0 {
					l.Prev()
				}
				break
			}
		}
		l.Emit(TokenNameVariable)
	}
}

func (bl *BatchLexer) lexDelayedVar(l lex.Lexer[TokenType, rune]) {
	l.Next()
	for {
		r := l.Next()
		if r == '!' || r == 0 || isNL(r) {
			if r != '!' && r != 0 {
				l.Prev()
			}
			break
		}
	}
	l.Emit(TokenNameVariable)
}

func (bl *BatchLexer) lexStringDoubleBody(next lex.StateFn[TokenType, rune]) lex.StateFn[TokenType, rune] {
	return func(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
		for {
			r := l.Next()
			switch r {
			case 0:
				l.Emit(TokenStringDouble)
				return nil
			case '"':
				l.Emit(TokenStringDouble)
				return next
			case '%':
				l.Prev()
				if l.Width() > 0 {
					l.Emit(TokenStringDouble)
				}
				bl.lexPercent(l)
				return bl.lexStringDoubleBody(next)
			case '^':
				r2 := l.Next()
				if r2 == 0 {
					l.Emit(TokenStringDouble)
					return nil
				}
			}
		}
	}
}

func (bl *BatchLexer) lexStringBTBody(next lex.StateFn[TokenType, rune]) lex.StateFn[TokenType, rune] {
	return func(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
		for {
			r := l.Next()
			switch r {
			case 0:
				l.Emit(TokenStringBT)
				return nil
			case '`':
				l.Emit(TokenStringBT)
				return next
			}
		}
	}
}
