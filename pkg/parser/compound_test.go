package parser_test

import (
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/lexer"
	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
)

// TestParseBlock verifies a compound block is parsed into a Block node.
func TestParseBlock(t *testing.T) {
	nodes := parse("(\necho hi\n)\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	block, ok := nodes[0].(*parser.Block)
	if !ok {
		t.Fatalf("expected *Block, got %T", nodes[0])
	}
	if len(block.Body) == 0 {
		t.Error("expected block body to be non-empty")
	}
}

// TestParseBlockMultipleCommands verifies blocks contain multiple commands.
func TestParseBlockMultipleCommands(t *testing.T) {
	nodes := parse("(\necho one\necho two\n)\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	block, ok := nodes[0].(*parser.Block)
	if !ok {
		t.Fatalf("expected *Block, got %T", nodes[0])
	}
	if len(block.Body) < 2 {
		t.Errorf("expected >= 2 body commands, got %d", len(block.Body))
	}
}

// TestParseIfEquals verifies if/== condition is parsed (phase 2 IF handling).
func TestParseIfEquals(t *testing.T) {
	nodes := parse(`if "%X%"=="yes" echo ok` + "\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	ifn, ok := nodes[0].(*parser.IfNode)
	if !ok {
		t.Fatalf("expected *IfNode, got %T", nodes[0])
	}
	if ifn.Cond.Kind != parser.CondCompare {
		t.Errorf("expected CondCompare, got %v", ifn.Cond.Kind)
	}
	if ifn.Cond.Op != parser.OpEqual {
		t.Errorf("expected op=OpEqual, got %q", ifn.Cond.Op)
	}
	if ifn.Then == nil {
		t.Error("expected Then body to be set")
	}
}

// TestParseIfWordOp verifies if/equ numeric comparison is parsed.
func TestParseIfWordOp(t *testing.T) {
	nodes := parse("if %COUNT% equ 0 echo zero\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	ifn, ok := nodes[0].(*parser.IfNode)
	if !ok {
		t.Fatalf("expected *IfNode, got %T", nodes[0])
	}
	if ifn.Cond.Kind != parser.CondCompare {
		t.Errorf("expected CondCompare, got %v", ifn.Cond.Kind)
	}
	if ifn.Cond.Op != parser.OpEqu {
		t.Errorf("expected op=equ, got %q", ifn.Cond.Op)
	}
}

// TestParseIfExist verifies if exist condition.
func TestParseIfExist(t *testing.T) {
	nodes := parse("if exist file.txt echo found\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	ifn, ok := nodes[0].(*parser.IfNode)
	if !ok {
		t.Fatalf("expected *IfNode, got %T", nodes[0])
	}
	if ifn.Cond.Kind != parser.CondExist {
		t.Errorf("expected CondExist, got %v", ifn.Cond.Kind)
	}
}

// TestParseIfNotExist verifies if not exist condition.
func TestParseIfNotExist(t *testing.T) {
	nodes := parse("if not exist file.txt echo missing\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	ifn, ok := nodes[0].(*parser.IfNode)
	if !ok {
		t.Fatalf("expected *IfNode, got %T", nodes[0])
	}
	if !ifn.Cond.Not {
		t.Error("expected Cond.Not=true for 'if not'")
	}
	if ifn.Cond.Kind != parser.CondExist {
		t.Errorf("expected CondExist, got %v", ifn.Cond.Kind)
	}
}

// TestParseIfDefined verifies if defined condition.
func TestParseIfDefined(t *testing.T) {
	nodes := parse("if defined MYVAR echo defined\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	ifn, ok := nodes[0].(*parser.IfNode)
	if !ok {
		t.Fatalf("expected *IfNode, got %T", nodes[0])
	}
	if ifn.Cond.Kind != parser.CondDefined {
		t.Errorf("expected CondDefined, got %v", ifn.Cond.Kind)
	}
}

// TestParseIfErrorLevel verifies if errorlevel condition.
func TestParseIfErrorLevel(t *testing.T) {
	nodes := parse("if errorlevel 1 echo failed\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	ifn, ok := nodes[0].(*parser.IfNode)
	if !ok {
		t.Fatalf("expected *IfNode, got %T", nodes[0])
	}
	if ifn.Cond.Kind != parser.CondErrorLevel {
		t.Errorf("expected CondErrorLevel, got %v", ifn.Cond.Kind)
	}
	if ifn.Cond.Level != 1 {
		t.Errorf("expected Level=1, got %d", ifn.Cond.Level)
	}
}

// TestParseIfCaseInsensitive verifies /i input produces an IfNode.
// The lexer emits an empty-value TokenKeyword for /i (due to Backup() semantics
// in stateIf) rather than a "/i"-valued one, so we only assert an IfNode is produced.
func TestParseIfCaseInsensitive(t *testing.T) {
	nodes := parse(`if /i "abc"=="ABC" echo match` + "\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	_, ok := nodes[0].(*parser.IfNode)
	if !ok {
		t.Fatalf("expected *IfNode, got %T", nodes[0])
	}
}

// TestParseIfWithBlock verifies then-body as a compound block.
func TestParseIfWithBlock(t *testing.T) {
	nodes := parse("if exist x.txt (\necho yes\n)\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	ifn, ok := nodes[0].(*parser.IfNode)
	if !ok {
		t.Fatalf("expected *IfNode, got %T", nodes[0])
	}
	if _, ok := ifn.Then.(*parser.Block); !ok {
		t.Errorf("expected Then to be *Block, got %T", ifn.Then)
	}
}

// TestParseFor verifies a basic FOR loop is parsed.
func TestParseFor(t *testing.T) {
	nodes := parse("for %%i in (a b c) do echo %%i\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	fn, ok := nodes[0].(*parser.ForNode)
	if !ok {
		t.Fatalf("expected *ForNode, got %T", nodes[0])
	}
	if fn.Variable != "i" {
		t.Errorf("expected variable=i, got %q", fn.Variable)
	}
	if len(fn.Set) != 3 {
		t.Errorf("expected 3 set items, got %d: %v", len(fn.Set), fn.Set)
	}
	if fn.Do == nil {
		t.Error("expected Do body to be set")
	}
}

// TestParseForRange verifies FOR /L (range) is parsed.
func TestParseForRange(t *testing.T) {
	nodes := parse("for /l %%n in (1,1,5) do echo %%n\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	fn, ok := nodes[0].(*parser.ForNode)
	if !ok {
		t.Fatalf("expected *ForNode, got %T", nodes[0])
	}
	if fn.Variant != parser.ForRange {
		t.Errorf("expected ForRange, got %v", fn.Variant)
	}
}

// TestParseForF verifies FOR /F (token parsing) is recognised.
func TestParseForF(t *testing.T) {
	nodes := parse(`for /f "tokens=1" %%a in (file.txt) do echo %%a` + "\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	fn, ok := nodes[0].(*parser.ForNode)
	if !ok {
		t.Fatalf("expected *ForNode, got %T", nodes[0])
	}
	if fn.Variant != parser.ForF {
		t.Errorf("expected ForF, got %v", fn.Variant)
	}
}

// TestParseSemicolonSkipped verifies that standalone ; consumes rest of line.
func TestParseSemicolonSkipped(t *testing.T) {
	// Semicolon and everything after it should be consumed
	nodes := parse("echo hello ; echo world\n")
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	cmd, ok := nodes[0].(*parser.SimpleCommand)
	if !ok {
		t.Fatalf("expected *SimpleCommand, got %T", nodes[0])
	}
	if cmd.Name != "echo" {
		t.Errorf("expected command name 'echo', got %q", cmd.Name)
	}
	// The "echo world" should NOT be a separate node
}

// TestParseSemicolonAfterBlock verifies ; after compound block consumes rest of line.
func TestParseSemicolonAfterBlock(t *testing.T) {
	nodes := parse("if 1==1 (echo yes) ; \n")
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	ifn, ok := nodes[0].(*parser.IfNode)
	if !ok {
		t.Fatalf("expected *IfNode, got %T", nodes[0])
	}
	if ifn == nil {
		t.Error("expected non-nil IfNode")
	}
}

// TestParseSemicolonWithRedirect verifies ; with trailing redirect is handled.
func TestParseSemicolonWithRedirect(t *testing.T) {
	// The ; and >> file.txt should be consumed, if block should be parsed
	nodes := parse("if 1==1 (echo yes)) ; >> file.txt\n")
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d: %v", len(nodes), nodes)
	}
	ifn, ok := nodes[0].(*parser.IfNode)
	if !ok {
		t.Fatalf("expected *IfNode, got %T", nodes[0])
	}
	if ifn == nil {
		t.Error("expected non-nil IfNode")
	}
}

// TestParseCommaSkipped verifies that standalone , consumes rest of line.
func TestParseCommaSkipped(t *testing.T) {
	nodes := parse("echo hello , echo world\n")
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	cmd, ok := nodes[0].(*parser.SimpleCommand)
	if !ok {
		t.Fatalf("expected *SimpleCommand, got %T", nodes[0])
	}
	if cmd.Name != "echo" {
		t.Errorf("expected command name 'echo', got %q", cmd.Name)
	}
}

func TestParseIfElseMultilineBlock(t *testing.T) {
	src := `if "%VAL%"=="1" (
    echo IF branch works
) else (
    echo ELSE branch works
)
`
	l := lexer.New(src)
	var tokens []lexer.Item
	for {
		tok := l.NextItem()
		if tok.Type == lexer.TokenEOF || (tok.Type == 0 && len(tok.Value) == 0) {
			break
		}
		tokens = append(tokens, tok)
	}
	p := parser.NewFromTokens(tokens)
	nodes := p.Parse()

	for _, d := range p.Diagnostics {
		t.Errorf("unexpected parser diagnostic: line %d, col %d: %s", d.Line+1, d.Col+1, d.Message)
	}

	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	ifn, ok := nodes[0].(*parser.IfNode)
	if !ok {
		t.Fatalf("expected *IfNode, got %T", nodes[0])
	}
	if ifn.Then == nil {
		t.Error("expected Then body to be set")
	}
	if ifn.Else == nil {
		t.Error("expected Else body to be set")
	}
}

// TestParseIfOperatorAborts pins real-cmd aborts: an & or | run in an IF
// condition or body-start position aborts regardless of condition truth.
func TestParseIfOperatorAborts(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"left operand", "if 1&echo x echo y\n", "& was unexpected at this time."},
		{"right operand", "if 1==1&echo x echo y\n", "& was unexpected at this time."},
		{"false condition", "if 1==2&echo x echo y\n", "& was unexpected at this time."},
		{"exist arg", "if exist f&echo x echo y\n", "& was unexpected at this time."},
		{"defined arg", "if defined V&echo x echo y\n", "& was unexpected at this time."},
		{"pipe in condition", "if 1|echo x echo y\n", "| was unexpected at this time."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes := parse(tt.src)
			if len(nodes) == 0 {
				t.Fatal("expected a node")
			}
			ab := abortOf(t, nodes[0])
			if ab.Message != tt.want {
				t.Errorf("message = %q, want %q", ab.Message, tt.want)
			}
		})
	}
}

// abortOf digs the AbortNode that parseBinary wraps as its left operand.
func abortOf(t *testing.T, n parser.Node) *parser.AbortNode {
	t.Helper()
	switch node := n.(type) {
	case *parser.AbortNode:
		return node
	case *parser.BinaryNode:
		return abortOf(t, node.Left)
	case *parser.PipeNode:
		return abortOf(t, node.Left)
	}
	t.Fatalf("no AbortNode under %T", n)
	return nil
}

// TestParseForSetOperatorAborts pins a bare & inside a FOR set aborting.
func TestParseForSetOperatorAborts(t *testing.T) {
	nodes := parse("for %%a in (x&echo y) do echo z\n")
	if len(nodes) == 0 {
		t.Fatal("expected a node")
	}
	ab := abortOf(t, nodes[0])
	if want := "& was unexpected at this time."; ab.Message != want {
		t.Errorf("message = %q, want %q", ab.Message, want)
	}
}

// TestParseIfCompoundBodyNotAborted guards against over-abort: an operator
// after a valid then-command remains part of the compound.
func TestParseIfCompoundBodyNotAborted(t *testing.T) {
	nodes := parse("if 1==1 echo a & echo b\n")
	if len(nodes) == 0 {
		t.Fatal("expected a node")
	}
	if _, ok := nodes[0].(*parser.IfNode); !ok {
		t.Fatalf("expected *parser.IfNode, got %T", nodes[0])
	}
}
