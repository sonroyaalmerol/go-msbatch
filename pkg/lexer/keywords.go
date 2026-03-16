package lexer

// Keywords lists every batch language keyword recognised by the lexer.
// It covers both command-position keywords (for, if, goto …) and modifier
// keywords used inside control structures (not, defined, exist …).
// This is the single place to update when the lexer learns a new keyword.
var Keywords = []string{
	// control-flow commands
	"rem", "set", "for", "if", "else", "goto", "call", "do", "in",
	// IF condition modifiers
	"not", "exist", "defined", "errorlevel",
}
