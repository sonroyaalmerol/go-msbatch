// Package parser builds an AST from the BatchLexer token stream.
package parser

import (
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/lexer"
)

// NodeKind identifies the kind of an AST node.
type NodeKind int

const (
	NodeSimpleCommand NodeKind = iota
	NodeBlock
	NodeIf
	NodeFor
	NodePipe
	NodeConcat  // &
	NodeOrElse  // ||
	NodeAndThen // &&
	NodeLabel
	NodeComment
)

// Node is the interface implemented by all AST nodes.
type Node interface {
	Kind() NodeKind
	Pos() Position
	EndPos() Position
}

// Position represents a source location.
type Position struct {
	Line int // 0-based line number
	Col  int // 0-based column number
}

// RedirectKind classifies a redirection operator.
type RedirectKind int

const (
	RedirectOut    RedirectKind = iota // >
	RedirectAppend                     // >>
	RedirectIn                         // <
	RedirectOutFD                      // >&N
	RedirectInFD                       // <&N
)

// Redirect represents a single I/O redirection.
type Redirect struct {
	FD     int // source file descriptor (default 1 for out, 0 for in)
	Kind   RedirectKind
	Target string // file path, or FD string for FD redirections
}

// SimpleCommand is a leaf command: name, arguments, and redirections.
type SimpleCommand struct {
	Line             int  // 0-based source line of the command name token
	Col              int  // 0-based source column of the command name token
	EndLine          int  // 0-based source line of the end of the command
	EndCol           int  // 0-based source column of the end of the command
	Suppressed       bool // true when preceded by @
	RedirectsApplied bool // internal flag to avoid re-applying redirections in recursive dispatches
	Name             string
	Args             []string
	RawArgs          []string
	Redirects        []Redirect
}

func (c *SimpleCommand) Kind() NodeKind   { return NodeSimpleCommand }
func (c *SimpleCommand) Pos() Position    { return Position{Line: c.Line, Col: c.Col} }
func (c *SimpleCommand) EndPos() Position { return Position{Line: c.EndLine, Col: c.EndCol} }

// Words returns RawArgs grouped by true whitespace. This is useful for
// external commands where delimiters like '=' or ',' should only split
// arguments if they are surrounded by actual whitespace.
func (c *SimpleCommand) Words() []string {
	var words []string
	var current strings.Builder

	for _, arg := range c.RawArgs {
		// Check if it's true whitespace
		isTrueWS := false
		if len(arg) > 0 {
			r := rune(arg[0])
			if lexer.IsWS(r) {
				isTrueWS = true
			}
		}

		if isTrueWS {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		} else {
			current.WriteString(arg)
		}
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}

// Block is a parenthesised sequence of commands: ( cmd1 \n cmd2 ).
type Block struct {
	Line    int // 0-based source line of the opening '('
	Col     int // 0-based source column of the opening '('
	EndLine int // 0-based source line of the closing ')'; same as Line if unclosed
	EndCol  int // 0-based source column after the closing ')'
	Body    []Node
}

func (b *Block) Kind() NodeKind   { return NodeBlock }
func (b *Block) Pos() Position    { return Position{Line: b.Line, Col: b.Col} }
func (b *Block) EndPos() Position { return Position{Line: b.EndLine, Col: b.EndCol} }

// CondKind classifies an IF condition.
type CondKind int

const (
	CondCompare       CondKind = iota // LHS op RHS
	CondExist                         // EXIST path
	CondErrorLevel                    // ERRORLEVEL n
	CondDefined                       // DEFINED varname
	CondCmdExtVersion                 // CMDEXTVERSION n
)

// CompareOp is the comparison operator used in CondCompare.
type CompareOp string

const (
	OpEqual CompareOp = "=="
	OpEqu   CompareOp = "equ"
	OpNeq   CompareOp = "neq"
	OpLss   CompareOp = "lss"
	OpLeq   CompareOp = "leq"
	OpGtr   CompareOp = "gtr"
	OpGeq   CompareOp = "geq"
)

// Condition holds all fields for an IF condition.
type Condition struct {
	Not   bool
	Kind  CondKind
	Left  string    // CondCompare: left operand (reconstructed from token values)
	Op    CompareOp // CondCompare: operator
	Right string    // CondCompare: right operand
	Arg   string    // CondExist: path; CondDefined: varname
	Level int       // CondErrorLevel, CondCmdExtVersion: numeric value
}

// IfNode represents an IF statement.
type IfNode struct {
	Line            int // 0-based source line of the "if" keyword
	Col             int // 0-based source column of the "if" keyword
	EndLine         int // 0-based source line of the end of the IF statement
	EndCol          int // 0-based source column of the end of the IF statement
	CaseInsensitive bool
	Cond            Condition
	Then            Node
	Else            Node // nil if absent
}

func (n *IfNode) Kind() NodeKind   { return NodeIf }
func (n *IfNode) Pos() Position    { return Position{Line: n.Line, Col: n.Col} }
func (n *IfNode) EndPos() Position { return Position{Line: n.EndLine, Col: n.EndCol} }

// ForKind classifies a FOR variant.
type ForKind int

const (
	ForFiles     ForKind = iota // FOR %%V IN (set) DO
	ForRange                    // FOR /L %%V IN (start,step,end) DO
	ForF                        // FOR /F ["opts"] %%V IN (...) DO
	ForDir                      // FOR /D %%V IN (set) DO  — directories only
	ForRecursive                // FOR /R [root] %%V IN (set) DO  — recursive walk
)

// ForNode represents a FOR loop.
type ForNode struct {
	Line     int // 0-based source line of the "for" keyword
	Col      int // 0-based source column of the "for" keyword
	EndLine  int // 0-based source line of the end of the FOR statement
	EndCol   int // 0-based source column of the end of the FOR statement
	VarLine  int // 0-based source line of the loop variable token
	VarCol   int // 0-based source column of the loop variable letter
	Variant  ForKind
	Options  string   // FOR /F option string (content of quotes, single-quotes, or backticks)
	Variable string   // loop variable name, e.g. "i" for %%i
	Set      []string // items between IN( and )
	Do       Node
}

func (n *ForNode) Kind() NodeKind   { return NodeFor }
func (n *ForNode) Pos() Position    { return Position{Line: n.Line, Col: n.Col} }
func (n *ForNode) EndPos() Position { return Position{Line: n.EndLine, Col: n.EndCol} }

// PipeNode represents cmd1 | cmd2.
type PipeNode struct {
	Line    int // 0-based source line of the left operand
	Col     int // 0-based source column of the left operand
	EndLine int // 0-based source line of the end of the pipe
	EndCol  int // 0-based source column of the end of the pipe
	Left    Node
	Right   Node
}

func (n *PipeNode) Kind() NodeKind   { return NodePipe }
func (n *PipeNode) Pos() Position    { return Position{Line: n.Line, Col: n.Col} }
func (n *PipeNode) EndPos() Position { return Position{Line: n.EndLine, Col: n.EndCol} }

// BinaryNode handles &&, ||, &.
type BinaryNode struct {
	Line    int      // 0-based source line of the left operand
	Col     int      // 0-based source column of the left operand
	EndLine int      // 0-based source line of the end of the binary expression
	EndCol  int      // 0-based source column of the end of the binary expression
	Op      NodeKind // NodeConcat, NodeOrElse, NodeAndThen
	Left    Node
	Right   Node
}

func (n *BinaryNode) Kind() NodeKind   { return n.Op }
func (n *BinaryNode) Pos() Position    { return Position{Line: n.Line, Col: n.Col} }
func (n *BinaryNode) EndPos() Position { return Position{Line: n.EndLine, Col: n.EndCol} }

// LabelNode is a label definition (:name).
type LabelNode struct {
	Line    int // 0-based source line of the ':' token
	Col     int // 0-based source column of the label name (after ':')
	EndLine int // 0-based source line of the end of the label
	EndCol  int // 0-based source column after the label name
	Name    string
}

func (n *LabelNode) Kind() NodeKind   { return NodeLabel }
func (n *LabelNode) Pos() Position    { return Position{Line: n.Line, Col: n.Col} }
func (n *LabelNode) EndPos() Position { return Position{Line: n.EndLine, Col: n.EndCol} }

// CommentNode holds a REM comment or :: comment.
type CommentNode struct {
	Line    int // 0-based source line
	Col     int // 0-based source column of the comment token
	EndLine int // 0-based source line of the end of the comment
	EndCol  int // 0-based source column after the comment
	Text    string
}

func (n *CommentNode) Kind() NodeKind   { return NodeComment }
func (n *CommentNode) Pos() Position    { return Position{Line: n.Line, Col: n.Col} }
func (n *CommentNode) EndPos() Position { return Position{Line: n.EndLine, Col: n.EndCol} }
