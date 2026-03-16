// Package lexer tokenises Windows batch scripts into a stream of Items.
// The lexer is a state machine: each stateFn method advances the input and
// transitions to the next state by returning it.
package lexer

// BatchLexer tokenises a Windows batch script.
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

// next consumes and returns the next rune (0 at EOF).
func (bl *BatchLexer) next() rune {
	if bl.pos >= len(bl.input) {
		return 0
	}
	bl.pos++
	return bl.input[bl.pos-1]
}

// prev unconsumes the last rune (single-step undo).
func (bl *BatchLexer) prev() rune {
	if bl.pos == 0 {
		return 0
	}
	bl.pos--
	if bl.pos < bl.start {
		bl.start = bl.pos
	}
	return bl.input[bl.pos]
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
	bl.items <- Item{
		Col:   bl.start,
		Type:  t,
		Value: bl.input[bl.start:bl.pos],
	}
	bl.start = bl.pos
}

// check reports whether the rune at the current position satisfies fn
// without consuming it.
func (bl *BatchLexer) check(fn func(rune) bool) bool {
	if bl.pos >= len(bl.input) {
		return fn(0)
	}
	return fn(bl.input[bl.pos])
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
