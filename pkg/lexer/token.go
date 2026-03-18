package lexer

// Item is a single lexed token.
type Item struct {
	Line  int       // 0-based line number within the input (set by lineOffset)
	Col   int       // 0-based rune column within the input line
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
		TokenEOF: "EOF", TokenText: "Text",
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
