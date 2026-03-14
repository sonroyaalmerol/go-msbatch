// Package lexer implements a lexer for Windows Batch (.bat/.cmd) files,
// faithfully mirroring the Pygments BatchLexer, using github.com/zalgonoise/lex.
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
	TokenOperatorWord
	TokenRedirect
)

func (t TokenType) String() string {
	names := map[TokenType]string{
		TokenEOF: "EOF", TokenError: "Error", TokenText: "Text",
		TokenPunctuation: "Punctuation", TokenKeyword: "Keyword",
		TokenComment: "Comment", TokenNameLabel: "Name.Label",
		TokenNameVariable: "Name.Variable", TokenStringDouble: "String.Double",
		TokenStringSingle: "String.Single", TokenStringBT: "String.Backtick",
		TokenStringEscape: "String.Escape", TokenNumber: "Number",
		TokenOperator: "Operator", TokenOperatorWord: "Operator.Word",
		TokenRedirect: "Redirect",
	}
	if s, ok := names[t]; ok {
		return s
	}
	return "Unknown"
}

const wsChars = "\t\v\f\r ,;=\xa0"

func isNL(r rune) bool    { return r == '\n' || r == '\x1a' }
func isWS(r rune) bool    { return strings.ContainsRune(wsChars, r) }
func isPunct(r rune) bool { return r == '&' || r == '<' || r == '>' || r == '|' }
func isKeywordEnd(r rune) bool {
	return r == 0 || isWS(r) || isNL(r) || isPunct(r) ||
		r == '(' || r == '+' || r == '.' || r == '/' ||
		r == ':' || r == '[' || r == '\\' || r == ']'
}

var builtinCommands = map[string]bool{
	"assoc": true, "break": true, "cd": true, "chdir": true,
	"cls": true, "color": true, "copy": true, "date": true,
	"del": true, "dir": true, "dpath": true, "echo": true,
	"else": true, "endlocal": true, "erase": true, "exit": true, "ftype": true,
	"keys": true, "md": true, "mkdir": true, "mklink": true,
	"move": true, "path": true, "pause": true, "popd": true,
	"prompt": true, "pushd": true, "rd": true, "ren": true,
	"rename": true, "rmdir": true, "setlocal": true, "shift": true,
	"start": true, "time": true, "title": true, "type": true,
	"ver": true, "verify": true, "vol": true,
}

var opWords = map[string]bool{
	"equ": true, "geq": true, "gtr": true,
	"leq": true, "lss": true, "neq": true,
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
		l.Ignore()
	}
}

func (bl *BatchLexer) isFollowPlain(r rune) bool {
	if r == 0 || isNL(r) {
		return false
	}
	if r == '|' || r == '&' || r == '>' || r == '<' {
		return false
	}
	if r == '"' || r == '%' || r == '!' || r == '^' {
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
		l.Emit(TokenText)
		return bl.stateRoot
	case isWS(r):
		l.AcceptRun(isWS)
		l.Emit(TokenText)
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
		l.Emit(TokenText)
		return bl.stateRoot
	}
	word := bl.drainBuf(l)
	lower := strings.ToLower(word)
	switch lower {
	case "rem":
		l.Emit(TokenKeyword)
		return bl.stateRem
	case "echo":
		l.Emit(TokenKeyword)
		return bl.stateFollow
	case "set":
		l.Emit(TokenKeyword)
		return bl.stateSet
	case "for":
		l.Emit(TokenKeyword)
		return bl.stateFor
	case "if":
		l.Emit(TokenKeyword)
		return bl.stateIf
	case "goto":
		l.Emit(TokenKeyword)
		return bl.stateGoto
	case "call":
		l.Emit(TokenKeyword)
		return bl.stateCall
	default:
		if builtinCommands[lower] {
			l.Emit(TokenKeyword)
		} else {
			l.Emit(TokenText)
		}
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
		l.Emit(TokenText)
		return bl.stateRoot
	case r == '|' || r == '&':
		l.AcceptRun(func(r rune) bool { return r == '|' || r == '&' })
		l.Emit(TokenPunctuation)
		return bl.stateRoot
	case r == ')' && bl.compoundDepth > 0:
		bl.compoundDepth--
		l.Emit(TokenPunctuation)
		return bl.stateRoot
	case r == '>' || r == '<':
		return bl.stateRedirectRune(l, r)
	case r == '"':
		return bl.lexStringDoubleBody(bl.stateFollow)(l)
	case r == '%':
		l.Backup()
		bl.lexPercent(l)
		return bl.stateFollow
	case r == '!':
		l.Backup()
		bl.lexDelayedVar(l)
		return bl.stateFollow
	case r == '^':
		r2 := l.Next()
		if r2 == 0 {
			return bl.stateRoot
		}
		if isNL(r2) {
			l.Ignore()
		} else {
			l.Emit(TokenStringEscape)
		}
		return bl.stateFollow
	default:
		l.Emit(TokenText)
		return bl.stateRoot
	}
}

func (bl *BatchLexer) stateRem(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	l.AcceptRun(func(r rune) bool { return r != 0 && !isNL(r) })
	if l.Width() > 0 {
		l.Emit(TokenComment)
	}
	return bl.stateRoot
}

func (bl *BatchLexer) stateLabelName(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	l.AcceptRun(func(r rune) bool {
		return r != 0 && !isNL(r) && !isWS(r) && !isPunct(r) && r != ':' && r != '+'
	})
	if l.Width() > 0 {
		l.Emit(TokenNameLabel)
		return bl.stateLabelName
	}
	l.AcceptRun(func(r rune) bool { return r != 0 && !isNL(r) })
	if l.Width() > 0 {
		l.Emit(TokenComment)
	}
	return bl.stateRoot
}

func (bl *BatchLexer) stateGoto(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	skipWS(l)
	if l.Check(func(r rune) bool { return r == ':' }) {
		l.Next()
		l.Emit(TokenPunctuation)
		return bl.stateGotoLabel
	}
	return bl.stateGotoLabel
}

func (bl *BatchLexer) stateGotoLabel(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	l.AcceptRun(func(r rune) bool {
		return r != 0 && !isNL(r) && !isWS(r) && !isPunct(r)
	})
	if l.Width() > 0 {
		l.Emit(TokenNameLabel)
	}
	return bl.stateFollow
}

func (bl *BatchLexer) stateCall(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	skipWS(l)
	if l.Check(func(r rune) bool { return r == ':' }) {
		l.Next()
		l.Emit(TokenPunctuation)
		return bl.stateCallLabel
	}
	return bl.stateFollow
}

func (bl *BatchLexer) stateCallLabel(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	l.AcceptRun(func(r rune) bool {
		return r != 0 && !isNL(r) && !isWS(r) && !isPunct(r)
	})
	if l.Width() > 0 {
		l.Emit(TokenNameLabel)
	}
	return bl.stateFollow
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
				l.Backup()
			}
			l.Backup()
		}
	}
	return bl.stateSetVar
}

func (bl *BatchLexer) stateSetVar(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	l.AcceptRun(func(r rune) bool { return r != 0 && r != '=' && !isNL(r) })
	if l.Width() > 0 {
		l.Emit(TokenNameVariable)
		return bl.stateSetEq
	}
	return bl.stateFollow
}

func (bl *BatchLexer) stateSetEq(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	if l.Check(func(r rune) bool { return r == '=' }) {
		l.Next()
		l.Emit(TokenPunctuation)
	}
	return bl.stateFollow
}

func (bl *BatchLexer) stateArithmetic(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	r := l.Next()
	switch {
	case r == 0 || isNL(r):
		return bl.stateRoot
	case r == '|' || r == '&':
		l.Backup()
		return bl.stateRoot
	case r == ')' && bl.compoundDepth > 0:
		l.Backup()
		return bl.stateRoot
	case isWS(r) && r != '=':
		// Treat '=' as an operator, not whitespace, in arithmetic expressions.
		l.AcceptRun(func(r rune) bool { return r != '=' && isWS(r) })
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
			l.AcceptRun(func(r rune) bool { return r >= '0' && r <= '7' })
		} else if r2 != 0 {
			l.Backup()
		}
		l.Emit(TokenNumber)
		return bl.stateArithmetic
	case r >= '1' && r <= '9':
		l.AcceptRun(func(r rune) bool { return r >= '0' && r <= '9' })
		l.Emit(TokenNumber)
		return bl.stateArithmetic
	case r == '-':
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
		return bl.stateArithmetic
	}
}

func (bl *BatchLexer) stateFor(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	skipWS(l)
	if l.Check(func(r rune) bool { return r == '/' }) {
		l.Next()
		flag := l.Next()
		switch unicode.ToLower(flag) {
		case 'f':
			l.Emit(TokenKeyword)
			skipWS(l)
			return bl.stateForF
		case 'l':
			l.Emit(TokenKeyword)
			return bl.stateForVarIn
		default:
			if flag != 0 {
				l.Backup()
			}
			l.Backup()
		}
	}
	return bl.stateForVarIn
}

func (bl *BatchLexer) stateForVarIn(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	skipWS(l)
	if l.Check(func(r rune) bool { return r == '%' }) {
		l.Next()
		if !l.Check(func(r rune) bool { return r == '%' }) {
			l.AcceptRun(func(r rune) bool { return r != 0 && !isWS(r) && !isNL(r) })
		} else {
			l.Next()
			l.AcceptRun(func(r rune) bool { return r != 0 && !isWS(r) && !isNL(r) })
		}
		l.Emit(TokenNameVariable)
	}
	skipWS(l)
	bl.lexKeyword(l, "in")
	skipWS(l)
	if l.Check(func(r rune) bool { return r == '(' }) {
		l.Next()
		l.Emit(TokenPunctuation)
	}
	return bl.stateForVarInBody
}

func (bl *BatchLexer) stateForVarInBody(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	l.AcceptRun(func(r rune) bool { return r != ')' && r != 0 && !isNL(r) })
	if l.Width() > 0 {
		l.Emit(TokenText)
		return bl.stateForVarInBody
	}
	if l.Check(func(r rune) bool { return r == ')' }) {
		l.Next()
		l.Emit(TokenPunctuation)
	}
	skipWS(l)
	bl.lexKeyword(l, "do")
	return bl.stateFollow
}

func (bl *BatchLexer) stateForF(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	r := l.Next()
	switch r {
	case '"':
		l.AcceptRun(func(r rune) bool { return r != '"' && r != 0 && !isNL(r) })
		l.Accept(func(r rune) bool { return r == '"' })
		l.Emit(TokenStringDouble)
	case '\'':
		l.AcceptRun(func(r rune) bool { return r != '\'' && r != 0 && !isNL(r) })
		l.Accept(func(r rune) bool { return r == '\'' })
		l.Emit(TokenStringSingle)
	case '`':
		l.AcceptRun(func(r rune) bool { return r != '`' && r != 0 && !isNL(r) })
		l.Accept(func(r rune) bool { return r == '`' })
		l.Emit(TokenStringBT)
	default:
		if r != 0 {
			l.Backup()
		}
	}
	return bl.stateForVarIn
}

func (bl *BatchLexer) lexKeyword(l lex.Lexer[TokenType, rune], kw string) {
	runes := []rune(kw)
	for i, expected := range runes {
		r := l.Next()
		if r == 0 || unicode.ToLower(r) != expected {
			if r != 0 {
				l.Backup()
			}
			for range i {
				l.Backup()
			}
			if l.Width() > 0 {
				l.Emit(TokenText)
			}
			return
		}
	}
	if !l.Check(isKeywordEnd) {
		l.Backup()
		if l.Width() > 0 {
			l.Emit(TokenText)
		}
		return
	}
	l.Emit(TokenKeyword)
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
					l.Backup()
				}
				l.Emit(TokenKeyword)
				skipWS(l)
			} else {
				if r3 != 0 {
					l.Backup()
				}
				if r2 != 0 {
					l.Backup()
				}
				l.Backup()
			}
		} else {
			if r2 != 0 {
				l.Backup()
			}
			l.Backup()
		}
	}

	if bl.tryKeyword(l, "not") {
		l.Emit(TokenKeyword)
		skipWS(l)
	}

	switch {
	case bl.tryKeyword(l, "exist"):
		l.Emit(TokenKeyword)
		skipWS(l)
		bl.lexStoken(l)
	case bl.tryKeyword(l, "defined"):
		l.Emit(TokenKeyword)
		skipWS(l)
		bl.lexStoken(l)
	case bl.tryKeyword(l, "errorlevel"):
		l.Emit(TokenKeyword)
		skipWS(l)
		l.AcceptRun(func(r rune) bool { return r >= '0' && r <= '9' })
		if l.Width() > 0 {
			l.Emit(TokenNumber)
		}
	case bl.tryKeyword(l, "cmdextversion"):
		l.Emit(TokenKeyword)
		skipWS(l)
		l.AcceptRun(func(r rune) bool { return r >= '0' && r <= '9' })
		if l.Width() > 0 {
			l.Emit(TokenNumber)
		}
	default:
		bl.lexStoken(l)
		if l.Check(func(r rune) bool { return r == '=' }) {
			l.Next()
			l.Accept(func(r rune) bool { return r == '=' })
			l.Emit(TokenOperator)
		} else {
			skipWS(l)
			var buf strings.Builder
			l.AcceptRun(func(r rune) bool {
				if r != 0 && !isWS(r) && !isNL(r) && !isPunct(r) {
					buf.WriteRune(unicode.ToLower(r))
					return true
				}
				return false
			})
			if l.Width() > 0 {
				if opWords[buf.String()] {
					l.Emit(TokenOperatorWord)
				} else {
					l.Emit(TokenText)
				}
			}
		}
		skipWS(l)
		bl.lexStoken(l)
	}
	return bl.stateIfThen
}

func (bl *BatchLexer) lexStoken(l lex.Lexer[TokenType, rune]) {
	r := l.Next()
	switch r {
	case '"':
		fn := bl.lexStringDoubleBody(nil)
		for fn != nil {
			fn = fn(l)
		}
		return
	case '%':
		l.Backup()
		bl.lexPercent(l)
	case '!':
		l.Backup()
		bl.lexDelayedVar(l)
	case 0:
	default:
		l.Backup()
		l.AcceptRun(func(r rune) bool {
			return r != 0 && !isNL(r) && !isWS(r) && !isPunct(r) &&
				r != '"' && r != '%' && r != '!'
		})
		if l.Width() > 0 {
			l.Emit(TokenText)
		}
	}
}

func (bl *BatchLexer) tryKeyword(l lex.Lexer[TokenType, rune], kw string) bool {
	runes := []rune(kw)
	consumed := 0
	for _, expected := range runes {
		r := l.Next()
		consumed++
		if r == 0 || unicode.ToLower(r) != expected {
			for i := 0; i < consumed; i++ {
				l.Backup()
			}
			return false
		}
	}
	if !l.Check(isKeywordEnd) {
		l.Backup()
		return false
	}
	return true
}

func (bl *BatchLexer) stateIfThen(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	skipWS(l)
	if l.Check(func(r rune) bool { return r == '(' }) {
		l.Next()
		bl.compoundDepth++
		l.Emit(TokenPunctuation)
		return bl.stateRoot
	}
	return bl.stateFollow
}

// lexStringDoubleBody handles the interior of a double-quoted string, assuming
// the opening '"' has already been consumed.
func (bl *BatchLexer) lexStringDoubleBody(
	next lex.StateFn[TokenType, rune],
) lex.StateFn[TokenType, rune] {
	return func(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
		for {
			r := l.Next()
			switch {
			case r == '"' || isNL(r) || r == 0:
				l.Emit(TokenStringDouble)
				return next
			case r == '%':
				l.Prev()
				l.Emit(TokenStringDouble)
				bl.lexPercent(l)
				return bl.lexStringDoubleBody(next)
			case r == '^':
				r2 := l.Next()
				if r2 == '!' {
					l.Emit(TokenStringEscape)
					return bl.lexStringDoubleBody(next)
				}
				if r2 == 0 {
					l.Emit(TokenStringDouble)
					return next
				}
			}
		}
	}
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
			l.Backup()
		}
		l.Emit(TokenNameVariable)
	default:
		for {
			r2 := l.Next()
			if r2 == '%' || r2 == 0 || isNL(r2) {
				if r2 != '%' && r2 != 0 {
					l.Backup()
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
				l.Backup()
			}
			break
		}
	}
	l.Emit(TokenNameVariable)
}

// stateRedirect handles the old call pattern from stateRoot where the rune
// has NOT yet been consumed. It peeks for a leading digit first.
func (bl *BatchLexer) stateRedirect(l lex.Lexer[TokenType, rune]) lex.StateFn[TokenType, rune] {
	l.Accept(func(r rune) bool { return r >= '0' && r <= '9' })
	r := l.Next()
	if r != '>' && r != '<' {
		if r != 0 {
			l.Backup()
		}
		if l.Width() > 0 {
			l.Emit(TokenText)
		}
		return bl.stateFollow
	}
	return bl.stateRedirectRune(l, r)
}

// stateRedirectRune completes redirect handling after the '<' or '>' rune r
// has already been consumed.
func (bl *BatchLexer) stateRedirectRune(
	l lex.Lexer[TokenType, rune],
	r rune,
) lex.StateFn[TokenType, rune] {
	switch r {
	case '>':
		l.Accept(func(r rune) bool { return r == '>' })
		if l.Accept(func(r rune) bool { return r == '&' }) {
			l.Emit(TokenRedirect)
			skipWS(l)
			l.Accept(func(r rune) bool { return r >= '0' && r <= '9' })
			if l.Width() > 0 {
				l.Emit(TokenNumber)
			}
		} else {
			l.Emit(TokenRedirect)
			skipWS(l)
			l.AcceptRun(func(r rune) bool {
				return r != 0 && !isNL(r) && !isWS(r) && !isPunct(r)
			})
			if l.Width() > 0 {
				l.Emit(TokenText)
			}
		}
	case '<':
		if l.Accept(func(r rune) bool { return r == '&' }) {
			l.Emit(TokenRedirect)
			skipWS(l)
			l.Accept(func(r rune) bool { return r >= '0' && r <= '9' })
			if l.Width() > 0 {
				l.Emit(TokenNumber)
			}
		} else {
			l.Emit(TokenRedirect)
			skipWS(l)
			l.AcceptRun(func(r rune) bool {
				return r != 0 && !isNL(r) && !isWS(r) && !isPunct(r)
			})
			if l.Width() > 0 {
				l.Emit(TokenText)
			}
		}
	}
	return bl.stateFollow
}
