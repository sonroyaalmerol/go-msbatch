// Package parser builds an AST from the BatchLexer token stream.
package parser

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
	FD     int          // source file descriptor (default 1 for out, 0 for in)
	Kind   RedirectKind
	Target string // file path, or FD string for FD redirections
}

// SimpleCommand is a leaf command: name, arguments, and redirections.
type SimpleCommand struct {
	Suppressed bool // true when preceded by @
	Name       string
	Args       []string
	Redirects  []Redirect
}

func (c *SimpleCommand) Kind() NodeKind { return NodeSimpleCommand }

// Block is a parenthesised sequence of commands: ( cmd1 \n cmd2 ).
type Block struct {
	Body []Node
}

func (b *Block) Kind() NodeKind { return NodeBlock }

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
	CaseInsensitive bool
	Cond            Condition
	Then            Node
	Else            Node // nil if absent
}

func (n *IfNode) Kind() NodeKind { return NodeIf }

// ForKind classifies a FOR variant.
type ForKind int

const (
	ForFiles ForKind = iota // FOR %%V IN (set) DO
	ForRange                // FOR /L %%V IN (start,step,end) DO
	ForF                    // FOR /F ["opts"] %%V IN (...) DO
)

// ForNode represents a FOR loop.
type ForNode struct {
	Variant  ForKind
	Options  string   // FOR /F option string (content of quotes, single-quotes, or backticks)
	Variable string   // loop variable name, e.g. "i" for %%i
	Set      []string // items between IN( and )
	Do       Node
}

func (n *ForNode) Kind() NodeKind { return NodeFor }

// PipeNode represents cmd1 | cmd2.
type PipeNode struct {
	Left  Node
	Right Node
}

func (n *PipeNode) Kind() NodeKind { return NodePipe }

// BinaryNode handles &&, ||, &.
type BinaryNode struct {
	Op    NodeKind // NodeConcat, NodeOrElse, NodeAndThen
	Left  Node
	Right Node
}

func (n *BinaryNode) Kind() NodeKind { return n.Op }

// LabelNode is a label definition (:name).
type LabelNode struct {
	Name string
}

func (n *LabelNode) Kind() NodeKind { return NodeLabel }

// CommentNode holds a REM comment or :: comment.
type CommentNode struct {
	Text string
}

func (n *CommentNode) Kind() NodeKind { return NodeComment }

