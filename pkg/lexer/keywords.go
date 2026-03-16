package lexer

import "sort"

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
	registerKeyword("rem", func(bl *BatchLexer) stateFn { return bl.stateRem })
	registerKeyword("set", func(bl *BatchLexer) stateFn { return bl.stateSet })
	registerKeyword("for", func(bl *BatchLexer) stateFn { return bl.stateFor })
	registerKeyword("if", func(bl *BatchLexer) stateFn { return bl.stateIf })
	registerKeyword("else", func(bl *BatchLexer) stateFn { return bl.stateRoot })
	registerKeyword("goto", func(bl *BatchLexer) stateFn { return bl.stateGoto })
	registerKeyword("call", func(bl *BatchLexer) stateFn { return bl.stateCall })
	registerKeyword("do", func(bl *BatchLexer) stateFn { return bl.stateRoot })
	registerKeyword("in", func(bl *BatchLexer) stateFn { return bl.stateRoot })

	// modifier keywords: only matched mid-line inside IF/FOR constructs via
	// tryKeyword; not dispatched from stateWord.  Registered with nil next so
	// they appear in Keywords without affecting stateWord dispatch.
	registerKeyword("not", nil)
	registerKeyword("exist", nil)
	registerKeyword("defined", nil)
	registerKeyword("errorlevel", nil)

	// derive Keywords from the table (sorted for stability)
	kws := make([]string, 0, len(keywordTable))
	for k := range keywordTable {
		kws = append(kws, k)
	}
	sort.Strings(kws)
	Keywords = kws
}
