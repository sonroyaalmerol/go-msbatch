package lexer

// Keyword string constants – the single definition of every batch keyword
// string in the lexer.  Use these in states.go and states_commands.go so
// that all three files stay in sync automatically.
const (
	KwRem        = "rem"
	KwSet        = "set"
	KwFor        = "for"
	KwIf         = "if"
	KwElse       = "else"
	KwGoto       = "goto"
	KwCall       = "call"
	KwDo         = "do"
	KwIn         = "in"
	KwNot        = "not"
	KwExist      = "exist"
	KwDefined    = "defined"
	KwErrorlevel = "errorlevel"
)

// Keywords lists every batch language keyword recognised by the lexer.
// It is the authoritative source used by other packages (e.g. pkg/lsp) to
// know the full keyword set without duplicating the strings.
var Keywords = []string{
	// control-flow commands
	KwRem, KwSet, KwFor, KwIf, KwElse, KwGoto, KwCall, KwDo, KwIn,
	// IF condition modifiers
	KwNot, KwExist, KwDefined, KwErrorlevel,
}
