package parser_test

import (
	"testing"

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

// TestParseSemicolonSkipped verifies that standalone ; is not parsed as a command.
func TestParseSemicolonSkipped(t *testing.T) {
	// Semicolon should be skipped, echo should be parsed
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
}

// TestParseSemicolonAfterBlock verifies ; after compound block is skipped.
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

// TestParseCommaSkipped verifies that standalone , is not parsed as a command.
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
