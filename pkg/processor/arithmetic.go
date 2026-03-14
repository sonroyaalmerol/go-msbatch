package processor

import (
	"strconv"
	"strings"
	"unicode"
)

// EvalArithmetic evaluates a CMD-style arithmetic expression.
func (p *Processor) EvalArithmetic(expr string) (int, error) {
	// Strip carets (escapes) from the expression
	expr = strings.ReplaceAll(expr, "^", "")

	tokens := tokenizeArithmetic(expr)
	parser := &arithParser{
		tokens: tokens,
		pos:    0,
		p:      p,
	}

	var lastVal int
	var err error

	if len(parser.tokens) == 0 {
		return 0, nil
	}

	// SET /A supports multiple comma-separated expressions
	for parser.pos < len(parser.tokens) {
		lastVal, err = parser.parseAssignment()
		if err != nil {
			return 0, err
		}
		if parser.pos < len(parser.tokens) && parser.tokens[parser.pos] == "," {
			parser.pos++
		}
	}

	return lastVal, nil
}

type arithParser struct {
	tokens []string
	pos    int
	p      *Processor
}

func tokenizeArithmetic(expr string) []string {
	var tokens []string
	runes := []rune(expr)
	n := len(runes)

	for i := 0; i < n; i++ {
		r := runes[i]
		if unicode.IsSpace(r) {
			continue
		}

		if unicode.IsDigit(r) {
			start := i
			for i+1 < n && unicode.IsDigit(runes[i+1]) {
				i++
			}
			tokens = append(tokens, string(runes[start:i+1]))
		} else if unicode.IsLetter(r) || r == '_' {
			start := i
			for i+1 < n && (unicode.IsLetter(runes[i+1]) || unicode.IsDigit(runes[i+1]) || runes[i+1] == '_') {
				i++
			}
			tokens = append(tokens, string(runes[start:i+1]))
		} else {
			// Operators
			op := string(r)
			if i+1 < n {
				next := runes[i+1]
				combined := op + string(next)
				switch combined {
				case "+=", "-=", "*=", "/=", "%=", "<<", ">>", "==", "!=", "&&", "||", "&=", "^=", "|=":
					op = combined
					i++
					if i+1 < n && (combined == "<<" || combined == ">>") && runes[i+1] == '=' {
						op += "="
						i++
					}
				}
			}
			tokens = append(tokens, op)
		}
	}
	return tokens
}

func (ap *arithParser) peek() string {
	if ap.pos >= len(ap.tokens) {
		return ""
	}
	return ap.tokens[ap.pos]
}

func (ap *arithParser) consume() string {
	t := ap.peek()
	if ap.pos < len(ap.tokens) {
		ap.pos++
	}
	return t
}

func (ap *arithParser) parseAssignment() (int, error) {
	if ap.pos+1 < len(ap.tokens) && isIdentifier(ap.tokens[ap.pos]) && isAssignmentOp(ap.tokens[ap.pos+1]) {
		varName := ap.consume()
		op := ap.consume()
		val, err := ap.parseAssignment()
		if err != nil {
			return 0, err
		}

		currStr, _ := ap.p.Env.Get(varName)
		curr, _ := strconv.Atoi(currStr)

		newVal := val
		switch op {
		case "=":
			newVal = val
		case "+=":
			newVal = curr + val
		case "-=":
			newVal = curr - val
		case "*=":
			newVal = curr * val
		case "/=":
			if val == 0 {
				newVal = 0
			} else {
				newVal = curr / val
			}
		case "%=":
			if val == 0 {
				newVal = 0
			} else {
				newVal = curr % val
			}
		case "&=":
			newVal = curr & val
		case "^=":
			newVal = curr ^ val
		case "|=":
			newVal = curr | val
		case "<<=":
			newVal = curr << uint(val)
		case ">>=":
			newVal = curr >> uint(val)
		}
		ap.p.Env.Set(varName, strconv.Itoa(newVal))
		return newVal, nil
	}
	return ap.parseExpression()
}

func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	r := rune(s[0])
	return unicode.IsLetter(r) || r == '_'
}

func isAssignmentOp(op string) bool {
	switch op {
	case "=", "+=", "-=", "*=", "/=", "%=", "&=", "^=", "|=", "<<=", ">>=":
		return true
	}
	return false
}

// parseExpression is the top of the binary operator precedence chain:
// logical-or → logical-and → bitwise-or → bitwise-xor → bitwise-and → shift → add/sub
func (ap *arithParser) parseExpression() (int, error) {
	return ap.parseLogicalOr()
}

func (ap *arithParser) parseLogicalOr() (int, error) {
	val, err := ap.parseLogicalAnd()
	if err != nil {
		return 0, err
	}
	for ap.peek() == "||" {
		ap.consume()
		right, err := ap.parseLogicalAnd()
		if err != nil {
			return 0, err
		}
		if val != 0 || right != 0 {
			val = 1
		} else {
			val = 0
		}
	}
	return val, nil
}

func (ap *arithParser) parseLogicalAnd() (int, error) {
	val, err := ap.parseBitwiseOr()
	if err != nil {
		return 0, err
	}
	for ap.peek() == "&&" {
		ap.consume()
		right, err := ap.parseBitwiseOr()
		if err != nil {
			return 0, err
		}
		if val != 0 && right != 0 {
			val = 1
		} else {
			val = 0
		}
	}
	return val, nil
}

func (ap *arithParser) parseBitwiseOr() (int, error) {
	val, err := ap.parseBitwiseXor()
	if err != nil {
		return 0, err
	}
	for ap.peek() == "|" {
		ap.consume()
		right, err := ap.parseBitwiseXor()
		if err != nil {
			return 0, err
		}
		val |= right
	}
	return val, nil
}

func (ap *arithParser) parseBitwiseXor() (int, error) {
	val, err := ap.parseBitwiseAnd()
	if err != nil {
		return 0, err
	}
	for ap.peek() == "^" {
		ap.consume()
		right, err := ap.parseBitwiseAnd()
		if err != nil {
			return 0, err
		}
		val ^= right
	}
	return val, nil
}

func (ap *arithParser) parseBitwiseAnd() (int, error) {
	val, err := ap.parseShift()
	if err != nil {
		return 0, err
	}
	for ap.peek() == "&" {
		ap.consume()
		right, err := ap.parseShift()
		if err != nil {
			return 0, err
		}
		val &= right
	}
	return val, nil
}

func (ap *arithParser) parseShift() (int, error) {
	val, err := ap.parseAddSub()
	if err != nil {
		return 0, err
	}
	for {
		op := ap.peek()
		if op != "<<" && op != ">>" {
			break
		}
		ap.consume()
		right, err := ap.parseAddSub()
		if err != nil {
			return 0, err
		}
		if op == "<<" {
			val <<= uint(right)
		} else {
			val >>= uint(right)
		}
	}
	return val, nil
}

func (ap *arithParser) parseAddSub() (int, error) {
	val, err := ap.parseMulDiv()
	if err != nil {
		return 0, err
	}

	for {
		op := ap.peek()
		if op != "+" && op != "-" {
			break
		}
		ap.consume()
		right, err := ap.parseMulDiv()
		if err != nil {
			return 0, err
		}
		if op == "+" {
			val += right
		} else {
			val -= right
		}
	}
	return val, nil
}

func (ap *arithParser) parseMulDiv() (int, error) {
	val, err := ap.parseUnary()
	if err != nil {
		return 0, err
	}

	for {
		op := ap.peek()
		if op != "*" && op != "/" && op != "%" {
			break
		}
		ap.consume()
		right, err := ap.parseUnary()
		if err != nil {
			return 0, err
		}
		switch op {
		case "*":
			val *= right
		case "/":
			if right != 0 {
				val /= right
			} else {
				val = 0
			}
		case "%":
			if right != 0 {
				val %= right
			} else {
				val = 0
			}
		}
	}
	return val, nil
}

func (ap *arithParser) parseUnary() (int, error) {
	op := ap.peek()
	if op == "-" {
		ap.consume()
		val, err := ap.parseUnary()
		return -val, err
	}
	if op == "+" {
		ap.consume()
		return ap.parseUnary()
	}
	if op == "!" {
		ap.consume()
		val, err := ap.parseUnary()
		if val == 0 {
			return 1, err
		}
		return 0, err
	}
	if op == "~" {
		ap.consume()
		val, err := ap.parseUnary()
		return ^val, err
	}
	return ap.parsePrimary()
}

func (ap *arithParser) parsePrimary() (int, error) {
	t := ap.peek()
	if t == "(" {
		ap.consume() // consume "("
		val, err := ap.parseExpression()
		if err != nil {
			return 0, err
		}
		if ap.peek() == ")" {
			ap.consume() // consume ")"
		}
		return val, nil
	}

	t = ap.consume()
	if t == "" {
		return 0, nil
	}

	// Hex/Octal support (basic)
	if strings.HasPrefix(t, "0x") || strings.HasPrefix(t, "0X") {
		v, _ := strconv.ParseInt(t[2:], 16, 32)
		return int(v), nil
	}
	if len(t) > 1 && t[0] == '0' {
		v, _ := strconv.ParseInt(t[1:], 8, 32)
		return int(v), nil
	}

	if unicode.IsDigit(rune(t[0])) {
		val, _ := strconv.Atoi(t)
		return val, nil
	}

	// Variable name
	vStr, ok := ap.p.Env.Get(t)
	if !ok {
		// Literal that Get failed on?
		v, err := strconv.Atoi(t)
		if err == nil {
			return v, nil
		}
		return 0, nil
	}
	val, _ := strconv.Atoi(vStr)
	return val, nil
}
