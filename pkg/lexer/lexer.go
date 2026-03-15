package lexer

import (
	"strings"
	"unicode"
)

// Item is a single lexed token.
type Item struct {
	Pos   int
	Type  TokenType
	Value []rune
}

// stateFn is a state-machine transition. It operates on the receiver BatchLexer
// and returns the next state function (nil terminates the machine).
type stateFn func() stateFn

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

// BatchLexer tokenises a Windows batch script. It contains the lexer engine
// state directly — no separate cursor or generic wrapper.
type BatchLexer struct {
	// engine state
	input []rune
	start int
	pos   int
	state stateFn
	items chan Item
	// batch-specific state
	compoundDepth int
}

// New creates a BatchLexer ready to tokenise src.
func New(src string) *BatchLexer {
	bl := &BatchLexer{
		input: []rune(src),
		items: make(chan Item, 10),
	}
	bl.state = bl.stateRoot
	return bl
}

// NextItem returns the next Item from the token stream.
func (bl *BatchLexer) NextItem() Item {
	for {
		select {
		case next := <-bl.items:
			return next
		default:
			if bl.state != nil {
				bl.state = bl.state()
				continue
			}
			close(bl.items)
			return Item{}
		}
	}
}

// ---- lexer engine primitives ------------------------------------------------

// Next consumes and returns the next rune (0 at EOF).
func (bl *BatchLexer) Next() rune {
	if bl.pos >= len(bl.input) {
		return 0
	}
	bl.pos++
	return bl.input[bl.pos-1]
}

// Prev unconsumes the last rune (single-step undo).
func (bl *BatchLexer) Prev() rune {
	if bl.pos == 0 {
		return 0
	}
	bl.pos--
	if bl.pos < bl.start {
		bl.start = bl.pos
	}
	return bl.input[bl.pos]
}

// Backup resets pos to start, discarding the current buffered run.
func (bl *BatchLexer) Backup() {
	bl.pos = bl.start
}

// Width returns the number of runes buffered since the last Emit/Ignore.
func (bl *BatchLexer) Width() int {
	return bl.pos - bl.start
}

// Ignore discards buffered input without emitting a token.
func (bl *BatchLexer) Ignore() {
	bl.start = bl.pos
}

// Emit sends the current buffer as a token of type t and advances start.
func (bl *BatchLexer) Emit(t TokenType) {
	bl.items <- Item{
		Pos:   bl.start,
		Type:  t,
		Value: bl.input[bl.start:bl.pos],
	}
	bl.start = bl.pos
}

// Check reports whether the rune at the current position satisfies fn
// without consuming it.
func (bl *BatchLexer) Check(fn func(rune) bool) bool {
	if bl.pos >= len(bl.input) {
		return fn(0)
	}
	return fn(bl.input[bl.pos])
}

// Accept consumes the next rune if fn returns true, otherwise unconsumes.
func (bl *BatchLexer) Accept(fn func(rune) bool) bool {
	if fn(bl.Next()) {
		return true
	}
	bl.Prev()
	return false
}

// AcceptRun consumes runes as long as fn returns true.
func (bl *BatchLexer) AcceptRun(fn func(rune) bool) {
	for {
		if bl.pos >= len(bl.input) {
			return
		}
		if !fn(bl.Next()) {
			bl.Prev()
			return
		}
	}
}

// ---- helper methods ---------------------------------------------------------

func (bl *BatchLexer) skipWS() {
	bl.AcceptRun(isWS)
	if bl.Width() > 0 {
		bl.Emit(TokenWhitespace)
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

// ---- state functions --------------------------------------------------------

func (bl *BatchLexer) stateRoot() stateFn {
	r := bl.Next()
	switch {
	case r == 0:
		return nil
	case isNL(r):
		bl.AcceptRun(isNL)
		bl.Emit(TokenNewline)
		return bl.stateRoot
	case isWS(r):
		bl.AcceptRun(isWS)
		bl.Emit(TokenWhitespace)
		return bl.stateRoot
	case r == '(':
		bl.compoundDepth++
		bl.Emit(TokenPunctuation)
		return bl.stateRoot
	case r == ')':
		if bl.compoundDepth > 0 {
			bl.compoundDepth--
		}
		bl.Emit(TokenPunctuation)
		return bl.stateRoot
	case r == '@':
		bl.AcceptRun(func(r rune) bool { return r == '@' })
		bl.Emit(TokenPunctuation)
		return bl.stateRoot
	case r == ':':
		if bl.Check(func(r rune) bool { return r == ':' }) {
			bl.Next()
			bl.AcceptRun(func(r rune) bool { return !isNL(r) && r != 0 })
			bl.Emit(TokenComment)
			return bl.stateRoot
		}
		bl.Emit(TokenPunctuation)
		return bl.stateLabelName
	case r == '|' || r == '&':
		bl.AcceptRun(func(r rune) bool { return r == '|' || r == '&' })
		bl.Emit(TokenPunctuation)
		return bl.stateRoot
	case r == '>' || r == '<':
		return bl.stateRedirectRune(r)
	case r == '"':
		return bl.lexStringDoubleBody(bl.stateRoot)()
	case r == '`':
		return bl.lexStringBTBody(bl.stateRoot)()
	case r == '^':
		r2 := bl.Next()
		if r2 == 0 {
			bl.Emit(TokenStringEscape)
			return nil
		}
		if isNL(r2) {
			bl.Ignore()
		} else {
			bl.Emit(TokenStringEscape)
		}
		return bl.stateRoot
	case r == '%':
		bl.Prev()
		bl.lexPercent()
		return bl.stateRoot
	case r == '!':
		bl.Prev()
		bl.lexDelayedVar()
		return bl.stateRoot
	case r >= '0' && r <= '9':
		bl.AcceptRun(func(r rune) bool { return r >= '0' && r <= '9' })
		nextRune := bl.Next()
		if nextRune == '>' || nextRune == '<' {
			bl.Prev()
			bl.Ignore()
			return bl.stateRedirect()
		}
		for i := 0; i < bl.Width(); i++ {
			bl.Backup()
		}
		return bl.stateWord
	default:
		bl.Prev()
		return bl.stateWord
	}
}

func (bl *BatchLexer) stateWord() stateFn {
	bl.AcceptRun(func(r rune) bool {
		return r != 0 && !isNL(r) && !isWS(r) && !isPunct(r) &&
			r != '(' && r != ')' && r != '"' && r != '%' &&
			r != '!' && r != '^' && r != '>' && r != '<' && r != ':'
	})
	if bl.Width() == 0 {
		r := bl.Next()
		if r == 0 {
			return nil
		}
		bl.Prev()
		return bl.stateRoot
	}
	word := bl.drainBuf()
	lower := strings.ToLower(word)

	if word == "==" {
		bl.Emit(TokenOperator)
		return bl.stateFollow
	}

	switch lower {
	case "rem":
		bl.Emit(TokenKeyword)
		return bl.stateRem
	case "set":
		bl.Emit(TokenKeyword)
		return bl.stateSet
	case "for":
		bl.Emit(TokenKeyword)
		return bl.stateFor
	case "if":
		bl.Emit(TokenKeyword)
		return bl.stateIf
	case "else":
		bl.Emit(TokenKeyword)
		return bl.stateRoot
	case "goto":
		bl.Emit(TokenKeyword)
		return bl.stateGoto
	case "call":
		bl.Emit(TokenKeyword)
		return bl.stateCall
	case "do":
		bl.Emit(TokenKeyword)
		return bl.stateRoot
	case "in":
		bl.Emit(TokenKeyword)
		return bl.stateRoot
	default:
		bl.Emit(TokenWord)
		return bl.stateFollow
	}
}

func (bl *BatchLexer) stateFollow() stateFn {
	bl.AcceptRun(bl.isFollowPlain)
	if bl.Width() > 0 {
		bl.Emit(TokenText)
	}
	r := bl.Next()
	switch {
	case r == 0:
		return bl.stateRoot
	case isNL(r), isWS(r), r == '|', r == '&', r == ')', r == '(',
		r == '>', r == '<', r == '"', r == '%', r == '!', r == '^':
		bl.Prev()
		return bl.stateRoot
	default:
		bl.Prev()
		return bl.stateRoot
	}
}

func (bl *BatchLexer) stateRem() stateFn {
	bl.AcceptRun(func(r rune) bool { return !isNL(r) && r != 0 })
	bl.Emit(TokenComment)
	return bl.stateRoot
}

func (bl *BatchLexer) stateSet() stateFn {
	bl.skipWS()
	if bl.Check(func(r rune) bool { return r == '/' }) {
		bl.Next()
		flag := bl.Next()
		switch unicode.ToLower(flag) {
		case 'a':
			bl.Emit(TokenKeyword)
			return bl.stateArithmetic
		case 'p':
			bl.Emit(TokenKeyword)
			bl.skipWS()
			return bl.stateSetVar
		default:
			if flag != 0 {
				bl.Prev()
			}
		}
	}
	return bl.stateSetVar
}

func (bl *BatchLexer) stateSetVar() stateFn {
	bl.AcceptRun(func(r rune) bool {
		return r != 0 && !isNL(r) && !isWS(r) && !isPunct(r) && r != '='
	})
	if bl.Width() > 0 {
		bl.Emit(TokenNameVariable)
	}
	bl.skipWS()
	if bl.Check(func(r rune) bool { return r == '=' }) {
		bl.Next()
		bl.Emit(TokenPunctuation)
	}
	return bl.stateFollow
}

func (bl *BatchLexer) stateArithmetic() stateFn {
	r := bl.Next()
	switch {
	case r == 0:
		return bl.stateRoot
	case isNL(r):
		bl.Prev()
		return bl.stateRoot
	case r == '|' || r == '&':
		bl.Backup()
		return bl.stateRoot
	case r == ')' && bl.compoundDepth > 0:
		bl.Backup()
		return bl.stateRoot
	case isWS(r):
		bl.AcceptRun(isWS)
		bl.Ignore()
		return bl.stateArithmetic
	case r == '(':
		bl.compoundDepth++
		bl.Emit(TokenPunctuation)
		return bl.stateArithmetic
	case r == ')':
		bl.Emit(TokenPunctuation)
		return bl.stateArithmetic
	case r == ',':
		bl.Emit(TokenPunctuation)
		return bl.stateArithmetic
	case r == '%':
		bl.Prev()
		bl.lexPercent()
		return bl.stateArithmetic
	case r == '!':
		bl.Prev()
		bl.lexDelayedVar()
		return bl.stateArithmetic
	case r == '0':
		r2 := bl.Next()
		if r2 == 'x' || r2 == 'X' {
			bl.AcceptRun(func(r rune) bool {
				return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
			})
		} else if r2 >= '0' && r2 <= '7' {
			bl.AcceptRun(func(r rune) bool { return r >= '0' && r >= '7' })
		} else if r2 != 0 {
			bl.Prev()
		}
		bl.Emit(TokenNumber)
		return bl.stateArithmetic
	case r >= '1' && r <= '9':
		bl.AcceptRun(func(r rune) bool { return r >= '0' && r <= '9' })
		bl.Emit(TokenNumber)
		return bl.stateArithmetic
	case strings.ContainsRune("=+-*/!~^", r):
		bl.AcceptRun(func(r rune) bool { return strings.ContainsRune("=+-*/!~^", r) })
		bl.Emit(TokenOperator)
		return bl.stateArithmetic
	default:
		bl.AcceptRun(func(r rune) bool {
			return r != 0 && !isNL(r) && !isWS(r) && !isPunct(r) &&
				!strings.ContainsRune("=+-*/!~^(),", r) && r != '%' && r != '!'
		})
		if bl.Width() > 0 {
			bl.Emit(TokenNameVariable)
		}
	}
	return bl.stateArithmetic
}

func (bl *BatchLexer) stateFor() stateFn {
	bl.skipWS()

	isForF := false
	if bl.Check(func(r rune) bool { return r == '/' }) {
		bl.Next()
		flag := bl.Next()
		flagLower := unicode.ToLower(flag)
		if flagLower == 'f' || flagLower == 'l' || flagLower == 'd' || flagLower == 'r' {
			bl.Emit(TokenKeyword)
			bl.skipWS()
			isForF = (flagLower == 'f')
			// For /R: optionally scan a root path before the loop variable.
			if flagLower == 'r' && !bl.Check(func(r rune) bool { return r == '%' }) {
				bl.AcceptRun(func(r rune) bool { return r != 0 && !isNL(r) && r != '%' })
				if bl.Width() > 0 {
					bl.Emit(TokenText)
				}
				bl.skipWS()
			}
		} else {
			if flag != 0 {
				bl.Prev()
			}
			bl.Prev()
		}
	}

	// For /F: consume optional options string before the loop variable.
	if isForF && bl.Check(func(r rune) bool { return r == '"' || r == '\'' || r == '`' }) {
		quoteChar := bl.Next()
		for {
			r2 := bl.Next()
			if r2 == quoteChar || r2 == 0 || isNL(r2) {
				if r2 != quoteChar && r2 != 0 {
					bl.Prev()
				}
				break
			}
		}
		switch quoteChar {
		case '\'':
			bl.Emit(TokenStringSingle)
		case '`':
			bl.Emit(TokenStringBT)
		default:
			bl.Emit(TokenStringDouble)
		}
		bl.skipWS()
	}

	// Consume loop variable: %%X or %X → emit as TokenNameVariable.
	if bl.Check(func(r rune) bool { return r == '%' }) {
		bl.Next() // first %
		if bl.Check(func(r rune) bool { return r == '%' }) {
			bl.Next() // second %
		}
		bl.AcceptRun(func(r rune) bool {
			return r != 0 && !isNL(r) && !isWS(r) && !isPunct(r) && r != '(' && r != ')'
		})
		bl.Emit(TokenNameVariable)
		bl.skipWS()
	}

	bl.lexKeyword("in")
	bl.skipWS()
	if bl.Check(func(r rune) bool { return r == '(' }) {
		bl.Next()
		bl.compoundDepth++
		bl.Emit(TokenPunctuation)
		return bl.stateRoot
	}
	return bl.stateRoot
}

func (bl *BatchLexer) stateIf() stateFn {
	bl.skipWS()
	if bl.Check(func(r rune) bool { return r == '/' }) {
		bl.Next()
		r2 := bl.Next()
		if unicode.ToLower(r2) == 'i' {
			r3 := bl.Next()
			if isKeywordEnd(r3) {
				if r3 != 0 {
					bl.Prev()
				}
				bl.Emit(TokenKeyword)
				bl.skipWS()
			} else {
				if r3 != 0 {
					bl.Prev()
				}
				if r2 != 0 {
					bl.Prev()
				}
				bl.Prev()
			}
		} else {
			if r2 != 0 {
				bl.Prev()
			}
			bl.Prev()
		}
	}

	if bl.tryKeyword("not") {
		bl.Emit(TokenKeyword)
		bl.skipWS()
	}

	if bl.tryKeyword("exist") {
		bl.Emit(TokenKeyword)
		bl.skipWS()
		return bl.stateFollow
	}

	if bl.tryKeyword("defined") {
		bl.Emit(TokenKeyword)
		bl.skipWS()
		return bl.stateFollow
	}

	if bl.tryKeyword("errorlevel") {
		bl.Emit(TokenKeyword)
		bl.skipWS()
		return bl.stateFollow
	}

	return bl.stateFollow
}

func (bl *BatchLexer) stateGoto() stateFn {
	bl.skipWS()
	if bl.Check(func(r rune) bool { return r == ':' }) {
		bl.Next()
		bl.Emit(TokenPunctuation)
	}
	bl.AcceptRun(func(r rune) bool { return !isWS(r) && !isNL(r) && r != 0 })
	bl.Emit(TokenNameLabel)
	return bl.stateRoot
}

func (bl *BatchLexer) stateCall() stateFn {
	bl.skipWS()
	if bl.Check(func(r rune) bool { return r == ':' }) {
		bl.Next()
		bl.Emit(TokenPunctuation)
		bl.AcceptRun(func(r rune) bool { return !isWS(r) && !isNL(r) && r != 0 })
		bl.Emit(TokenNameLabel)
		return bl.stateFollow
	}
	return bl.stateFollow
}

func (bl *BatchLexer) stateLabelName() stateFn {
	bl.AcceptRun(func(r rune) bool { return !isWS(r) && !isNL(r) && r != 0 })
	bl.Emit(TokenNameLabel)
	return bl.stateRoot
}

func (bl *BatchLexer) stateRedirectRune(r rune) stateFn {
	if r == '>' && bl.Check(func(r rune) bool { return r == '>' }) {
		bl.Next()
	}
	if bl.Check(func(r rune) bool { return r == '&' }) {
		bl.Next()
	}
	bl.Emit(TokenRedirect)
	return bl.stateFollow
}

func (bl *BatchLexer) stateRedirect() stateFn {
	bl.Accept(func(r rune) bool { return r >= '0' && r <= '9' })
	r := bl.Next()
	if r != '>' && r != '<' {
		if r != 0 {
			bl.Prev()
		}
		if bl.Width() > 0 {
			bl.Emit(TokenText)
		}
		return bl.stateRoot
	}
	return bl.stateRedirectRune(r)
}

func (bl *BatchLexer) lexKeyword(kw string) {
	if bl.tryKeyword(kw) {
		bl.Emit(TokenKeyword)
	}
}

func (bl *BatchLexer) tryKeyword(kw string) bool {
	for _, char := range kw {
		r := bl.Next()
		if unicode.ToLower(r) != unicode.ToLower(char) {
			bl.Backup()
			return false
		}
	}
	if !isKeywordEnd(bl.Next()) {
		bl.Backup()
		return false
	}
	bl.Prev()
	return true
}

func (bl *BatchLexer) lexPercent() {
	bl.Next()
	r := bl.Next()
	switch {
	case r == '%':
		bl.Emit(TokenStringEscape)
	case r >= '0' && r <= '9' || r == '*':
		bl.Emit(TokenNameVariable)
	case r == '~':
		bl.AcceptRun(func(r rune) bool { return r != 0 && !isNL(r) && !isWS(r) && !isPunct(r) })
		bl.Emit(TokenNameVariable)
	case r == 0 || isNL(r):
		if r != 0 {
			bl.Prev()
		}
		bl.Emit(TokenNameVariable)
	default:
		for {
			r2 := bl.Next()
			if r2 == '%' || r2 == 0 || isNL(r2) {
				if r2 != '%' && r2 != 0 {
					bl.Prev()
				}
				break
			}
		}
		bl.Emit(TokenNameVariable)
	}
}

func (bl *BatchLexer) lexDelayedVar() {
	bl.Next()
	for {
		r := bl.Next()
		if r == '!' || r == 0 || isNL(r) {
			if r != '!' && r != 0 {
				bl.Prev()
			}
			break
		}
	}
	bl.Emit(TokenNameVariable)
}

func (bl *BatchLexer) lexStringDoubleBody(next stateFn) stateFn {
	return func() stateFn {
		for {
			r := bl.Next()
			switch r {
			case 0:
				bl.Emit(TokenStringDouble)
				return nil
			case '"':
				bl.Emit(TokenStringDouble)
				return next
			case '%':
				bl.Prev()
				if bl.Width() > 0 {
					bl.Emit(TokenStringDouble)
				}
				bl.lexPercent()
				return bl.lexStringDoubleBody(next)
			case '^':
				r2 := bl.Next()
				if r2 == 0 {
					bl.Emit(TokenStringDouble)
					return nil
				}
			}
		}
	}
}

func (bl *BatchLexer) lexStringBTBody(next stateFn) stateFn {
	return func() stateFn {
		for {
			r := bl.Next()
			switch r {
			case 0:
				bl.Emit(TokenStringBT)
				return nil
			case '`':
				bl.Emit(TokenStringBT)
				return next
			}
		}
	}
}
