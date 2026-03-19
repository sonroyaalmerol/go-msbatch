package lexer

// Item is a single lexed token.
type Item struct {
	Line  int // 0-based line number within the input (set by lineOffset)
	Col   int // 0-based rune column within the input line
	Type  TokenType
	Value []rune
}

// stateFn is a state-machine transition. It operates on the receiver
// BatchLexer and returns the next state (nil terminates the machine).
type stateFn func() stateFn

// TokenType identifies the kind of a lexed item.
type TokenType int

const (
	TokenEOF TokenType = iota
	TokenText
	TokenPunctuation
	TokenKeyword
	TokenComment
	TokenLabel
	TokenVariable
	TokenDelayedExpansion // !var! delayed expansion variable
	TokenForVar           // %%i FOR loop variable
	TokenStringDouble
	TokenStringSingle
	TokenStringBacktick
	TokenEscape
	TokenNumber
	TokenOperator
	TokenRedirect
	TokenWhitespace
	TokenNewline
	TokenWord
)

func (t TokenType) String() string {
	names := map[TokenType]string{
		TokenEOF: "EOF", TokenText: "Text",
		TokenPunctuation: "Punctuation", TokenKeyword: "Keyword",
		TokenComment: "Comment", TokenLabel: "Label",
		TokenVariable: "Variable", TokenDelayedExpansion: "DelayedExpansion",
		TokenForVar: "ForVar", TokenStringDouble: "String.Double",
		TokenStringSingle: "String.Single", TokenStringBacktick: "String.Backtick",
		TokenEscape: "Escape", TokenNumber: "Number",
		TokenOperator: "Operator",
		TokenRedirect: "Redirect", TokenWhitespace: "Whitespace",
		TokenNewline: "Newline", TokenWord: "Word",
	}
	if s, ok := names[t]; ok {
		return s
	}
	return "Unknown"
}
