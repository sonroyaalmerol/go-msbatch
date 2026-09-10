package lexer

import "sort"

// keywordEntry describes a registered keyword.
//
//   - next:   state to transition to after emitting the keyword token from
//             stateWord.  nil means the keyword is never the first word on a
//             line (e.g. IF modifier keywords).
//   - ifNext: handler used by stateIf when tryKeyword matches this word.
//             Returning nil means "keyword consumed, keep trying more
//             modifiers" (used for "not").  Returning a stateFn means
//             "keyword consumed, transition to that state".
//             nil means this keyword is not an IF modifier.
type keywordEntry struct {
	next   func(*BatchLexer) stateFn
	ifNext func(*BatchLexer) stateFn
}

// keywordTable is the authoritative keyword registry for the lexer.
var keywordTable = map[string]keywordEntry{}

// ifModifiers is the ordered list of keywords tried in stateIf.
// Order matters: "not" must come before the terminal modifiers.
var ifModifiers []string

// registerKeyword adds a command-position keyword (first word of a line).
func registerKeyword(name string, next func(*BatchLexer) stateFn) {
	e := keywordTable[name]
	e.next = next
	keywordTable[name] = e
}

// registerIfModifier adds a keyword tried inline by stateIf via tryKeyword.
// ifNext follows the same convention as keywordEntry.ifNext.
func registerIfModifier(name string, ifNext func(*BatchLexer) stateFn) {
	e := keywordTable[name]
	e.ifNext = ifNext
	keywordTable[name] = e
	ifModifiers = append(ifModifiers, name)
}

// Keywords lists every batch language keyword recognised by the lexer.
// Auto-derived from keywordTable so it is always in sync.
var Keywords []string

func init() {
	// command-position keywords
	registerKeyword("rem", func(bl *BatchLexer) stateFn { return bl.fnRem })
	registerKeyword("set", func(bl *BatchLexer) stateFn { return bl.fnSet })
	registerKeyword("for", func(bl *BatchLexer) stateFn { return bl.fnFor })
	registerKeyword("if", func(bl *BatchLexer) stateFn { return bl.fnIf })
	registerKeyword("goto", func(bl *BatchLexer) stateFn { return bl.fnGoto })
	registerKeyword("call", func(bl *BatchLexer) stateFn { return bl.fnCall })

	// IF modifier keywords (order matters: "not" must precede terminal modifiers)
	registerIfModifier("not", func(_ *BatchLexer) stateFn { return nil }) // prefix – continue
	registerIfModifier("exist", func(bl *BatchLexer) stateFn { return bl.fnFollow })
	registerIfModifier("defined", func(bl *BatchLexer) stateFn { return bl.fnFollow })
	registerIfModifier("errorlevel", func(bl *BatchLexer) stateFn { return bl.fnFollow })

	kws := make([]string, 0, len(keywordTable))
	for k := range keywordTable {
		kws = append(kws, k)
	}
	sort.Strings(kws)
	Keywords = kws
}
