package lexer

import "sort"

// Keyword string constants – the single definition of every batch keyword
// string in the lexer.  Use these in states_commands.go (tryKeyword calls)
// so that all files stay in sync automatically.
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

// keywordEntry describes a registered keyword.
// next is the state function to transition to after emitting the keyword
// token from stateWord.  It is nil for modifier keywords (not, exist,
// defined, errorlevel) that are only matched inside constructs like IF and
// are never the first word of a line.
type keywordEntry struct {
	next func(*BatchLexer) stateFn
}

// keywordTable is the authoritative keyword registry for the lexer.
// Every entry here is reflected in the Keywords slice automatically.
var keywordTable = map[string]keywordEntry{}

// registerKeyword adds a keyword to the table.
// next may be nil for modifier-only keywords (they are still included in
// Keywords so callers know the full set).
func registerKeyword(name string, next func(*BatchLexer) stateFn) {
	keywordTable[name] = keywordEntry{next: next}
}

// Keywords lists every batch language keyword recognised by the lexer.
// It is auto-derived from keywordTable so it is always in sync.
// Other packages (e.g. pkg/lsp) should use this slice as the authoritative
// source instead of maintaining their own copies.
var Keywords []string

func init() {
	// command-position keywords: appear as the first word of a statement
	// and dispatch to a dedicated state function.
	registerKeyword(KwRem, func(bl *BatchLexer) stateFn { return bl.stateRem })
	registerKeyword(KwSet, func(bl *BatchLexer) stateFn { return bl.stateSet })
	registerKeyword(KwFor, func(bl *BatchLexer) stateFn { return bl.stateFor })
	registerKeyword(KwIf, func(bl *BatchLexer) stateFn { return bl.stateIf })
	registerKeyword(KwElse, func(bl *BatchLexer) stateFn { return bl.stateRoot })
	registerKeyword(KwGoto, func(bl *BatchLexer) stateFn { return bl.stateGoto })
	registerKeyword(KwCall, func(bl *BatchLexer) stateFn { return bl.stateCall })
	registerKeyword(KwDo, func(bl *BatchLexer) stateFn { return bl.stateRoot })
	registerKeyword(KwIn, func(bl *BatchLexer) stateFn { return bl.stateRoot })

	// modifier keywords: only matched mid-line inside IF/FOR constructs via
	// tryKeyword; not dispatched from stateWord.  Registered with nil next so
	// they appear in Keywords without affecting stateWord dispatch.
	registerKeyword(KwNot, nil)
	registerKeyword(KwExist, nil)
	registerKeyword(KwDefined, nil)
	registerKeyword(KwErrorlevel, nil)

	// derive Keywords from the table (sorted for stability)
	kws := make([]string, 0, len(keywordTable))
	for k := range keywordTable {
		kws = append(kws, k)
	}
	sort.Strings(kws)
	Keywords = kws
}
