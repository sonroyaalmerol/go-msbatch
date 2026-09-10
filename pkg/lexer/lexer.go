// Package lexer tokenises Windows batch scripts into a stream of Items.
// The lexer is a state machine: each stateFn method advances the input and
// transitions to the next state by returning it.
package lexer

import "unicode/utf8"

// BatchLexer tokenises a Windows batch script.
type BatchLexer struct {
	input          string
	start          int
	pos            int
	state          stateFn
	items          chan Item
	lineOffset     int
	compoundDepth  int
	atCommandStart bool
	wlPos          int
	wlLine         int
	wlCol          int
	fnRoot         stateFn
	fnWord         stateFn
	fnFollow       stateFn
	fnSetVar       stateFn
	fnArithmetic   stateFn
	fnLabelName    stateFn
	fnRedirect     stateFn
	fnIf           stateFn
	fnFor          stateFn
	fnRem          stateFn
	fnCall         stateFn
	fnGoto         stateFn
	fnSet          stateFn
}

// New creates a BatchLexer ready to tokenise src.
func New(src string) *BatchLexer {
	bl := &BatchLexer{
		input:          src,
		items:          make(chan Item, 10),
		atCommandStart: true,
		wlLine:         0,
	}
	bl.bindStates()
	bl.state = bl.fnRoot
	return bl
}

// bindStates binds every state method once so transitions return fields
// instead of allocating method values.
func (bl *BatchLexer) bindStates() {
	bl.fnRoot = bl.stateRoot
	bl.fnWord = bl.stateWord
	bl.fnFollow = bl.stateFollow
	bl.fnSetVar = bl.stateSetVar
	bl.fnArithmetic = bl.stateArithmetic
	bl.fnLabelName = bl.stateLabelName
	bl.fnRedirect = bl.stateRedirect
	bl.fnIf = bl.stateIf
	bl.fnFor = bl.stateFor
	bl.fnRem = bl.stateRem
	bl.fnCall = bl.stateCall
	bl.fnGoto = bl.stateGoto
	bl.fnSet = bl.stateSet
}

// NewWithLine creates a BatchLexer that stamps line on every emitted Item.
// Use this when lexing a single line of a multi-line document so that
// Item.Line reflects the actual source line number.
func NewWithLine(src string, line int) *BatchLexer {
	bl := New(src)
	bl.lineOffset = line
	bl.wlLine = line
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

// next consumes and returns the next rune (0 at EOF).
func (bl *BatchLexer) next() rune {
	if bl.pos >= len(bl.input) {
		return 0
	}
	r, w := utf8.DecodeRuneInString(bl.input[bl.pos:])
	bl.pos += w
	return r
}

// prev unconsumes the last rune (single-step undo).
func (bl *BatchLexer) prev() rune {
	if bl.pos == 0 {
		return 0
	}
	i := bl.pos - 1
	for i > 0 && bl.input[i]&0xC0 == 0x80 {
		i--
	}
	r, _ := utf8.DecodeRuneInString(bl.input[i:bl.pos])
	bl.pos = i
	if bl.pos < bl.start {
		bl.start = bl.pos
	}
	return r
}

// backup resets pos to start, discarding the current buffered run.
func (bl *BatchLexer) backup() {
	bl.pos = bl.start
}

// width returns the number of runes buffered since the last emit/ignore.
func (bl *BatchLexer) width() int {
	return bl.pos - bl.start
}

// ignore discards buffered input without emitting a token.
func (bl *BatchLexer) ignore() {
	bl.start = bl.pos
}

// emit sends the current buffer as a token of type t and advances start.
func (bl *BatchLexer) emit(t TokenType) {
	startLine, startCol := bl.lineColAt(bl.start)
	endLine, endCol := bl.lineColAt(bl.pos)
	bl.items <- Item{
		Line:    startLine,
		Col:     startCol,
		EndLine: endLine,
		EndCol:  endCol,
		Type:    t,
		Value:   bl.input[bl.start:bl.pos],
	}
	bl.start = bl.pos
}

func (bl *BatchLexer) lineColAt(pos int) (line, col int) {
	if pos < bl.wlPos {
		line = bl.lineOffset
		col = 0
		i := 0
		for i < pos && i < len(bl.input) {
			r, w := utf8.DecodeRuneInString(bl.input[i:])
			if r == '\n' {
				line++
				col = 0
			} else if r != '\r' {
				col++
			}
			i += w
		}
	} else {
		line, col = bl.wlLine, bl.wlCol
		i := bl.wlPos
		for i < pos && i < len(bl.input) {
			r, w := utf8.DecodeRuneInString(bl.input[i:])
			if r == '\n' {
				line++
				col = 0
			} else if r != '\r' {
				col++
			}
			i += w
		}
	}
	bl.wlPos, bl.wlLine, bl.wlCol = pos, line, col
	return line, col
}

// check reports whether the rune at the current position satisfies fn
// without consuming it.
func (bl *BatchLexer) check(fn func(rune) bool) bool {
	if bl.pos >= len(bl.input) {
		return fn(0)
	}
	r, _ := utf8.DecodeRuneInString(bl.input[bl.pos:])
	return fn(r)
}

// accept consumes the next rune if fn returns true, otherwise unconsumes it.
func (bl *BatchLexer) accept(fn func(rune) bool) bool {
	if fn(bl.next()) {
		return true
	}
	bl.prev()
	return false
}

// acceptRun consumes runes as long as fn returns true.
func (bl *BatchLexer) acceptRun(fn func(rune) bool) {
	for {
		if bl.pos >= len(bl.input) {
			return
		}
		if !fn(bl.next()) {
			bl.prev()
			return
		}
	}
}
