package parser_test

import (
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/lexer"
	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
)

// parse is a test helper that lexes src and parses it.
func parse(src string) []parser.Node {
	bl := lexer.New(src)
	p := parser.New(bl)
	return p.Parse()
}

// firstNode returns the first node or nil.
func firstNode(nodes []parser.Node) parser.Node {
	if len(nodes) == 0 {
		return nil
	}
	return nodes[0]
}

// TestParseEmpty verifies that whitespace-only input produces no command nodes.
// Note: lexer.New panics on a completely empty string (returns nil), so we use "\n".
func TestParseEmpty(t *testing.T) {
	nodes := parse("\n")
	for _, n := range nodes {
		switch n.(type) {
		case *parser.CommentNode, *parser.LabelNode:
			// whitespace/comment/label tokens are acceptable
		default:
			t.Errorf("unexpected node type %T in whitespace-only input", n)
		}
	}
}

// TestParseSimpleCommand verifies a plain builtin command is parsed.
func TestParseSimpleCommand(t *testing.T) {
	nodes := parse("dir\n")
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	cmd, ok := nodes[0].(*parser.SimpleCommand)
	if !ok {
		t.Fatalf("expected *SimpleCommand, got %T", nodes[0])
	}
	if cmd.Name != "dir" {
		t.Errorf("expected name=dir, got %q", cmd.Name)
	}
}

// TestParseEchoWithArgs verifies that echo arguments are collected.
func TestParseEchoWithArgs(t *testing.T) {
	nodes := parse("echo hello world\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	cmd, ok := firstNode(nodes).(*parser.SimpleCommand)
	if !ok {
		t.Fatalf("expected *SimpleCommand, got %T", firstNode(nodes))
	}
	if cmd.Name != "echo" {
		t.Errorf("expected name=echo, got %q", cmd.Name)
	}
	if len(cmd.Args) == 0 {
		t.Error("expected args, got none")
	}
}

// TestParseAtSuppressor verifies the @ prefix sets Suppressed=true.
func TestParseAtSuppressor(t *testing.T) {
	nodes := parse("@echo off\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	cmd, ok := firstNode(nodes).(*parser.SimpleCommand)
	if !ok {
		t.Fatalf("expected *SimpleCommand, got %T", firstNode(nodes))
	}
	if !cmd.Suppressed {
		t.Error("expected Suppressed=true for @echo")
	}
}

// TestParseLabel verifies label parsing (phase 2: label token handling).
func TestParseLabel(t *testing.T) {
	nodes := parse(":start\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	lbl, ok := firstNode(nodes).(*parser.LabelNode)
	if !ok {
		t.Fatalf("expected *LabelNode, got %T", firstNode(nodes))
	}
	if lbl.Name != "start" {
		t.Errorf("expected label=start, got %q", lbl.Name)
	}
}

// TestParseRemComment verifies REM is parsed as a comment node.
func TestParseRemComment(t *testing.T) {
	nodes := parse("rem this is a comment\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	cmt, ok := firstNode(nodes).(*parser.CommentNode)
	if !ok {
		t.Fatalf("expected *CommentNode, got %T", firstNode(nodes))
	}
	_ = cmt.Text // just verify the type
}

// TestParseDoubleColonComment verifies :: is parsed as a comment node.
func TestParseDoubleColonComment(t *testing.T) {
	nodes := parse(":: this is a comment\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	_, ok := firstNode(nodes).(*parser.CommentNode)
	if !ok {
		t.Fatalf("expected *CommentNode, got %T", firstNode(nodes))
	}
}

// TestParsePipe verifies | produces a PipeNode (phase 5.3 pipe boundary).
func TestParsePipe(t *testing.T) {
	nodes := parse("dir | sort\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	pipe, ok := firstNode(nodes).(*parser.PipeNode)
	if !ok {
		t.Fatalf("expected *PipeNode, got %T", firstNode(nodes))
	}
	left, ok := pipe.Left.(*parser.SimpleCommand)
	if !ok {
		t.Fatalf("pipe.Left expected *SimpleCommand, got %T", pipe.Left)
	}
	if left.Name != "dir" {
		t.Errorf("expected left=dir, got %q", left.Name)
	}
}

// TestParseConcat verifies & produces a BinaryNode with NodeConcat.
func TestParseConcat(t *testing.T) {
	nodes := parse("echo a & echo b\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	bin, ok := firstNode(nodes).(*parser.BinaryNode)
	if !ok {
		t.Fatalf("expected *BinaryNode, got %T", firstNode(nodes))
	}
	if bin.Op != parser.NodeConcat {
		t.Errorf("expected NodeConcat op, got %v", bin.Op)
	}
}

// TestParseAndThen verifies && produces a BinaryNode with NodeAndThen.
func TestParseAndThen(t *testing.T) {
	nodes := parse("echo a && echo b\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	bin, ok := firstNode(nodes).(*parser.BinaryNode)
	if !ok {
		t.Fatalf("expected *BinaryNode, got %T", firstNode(nodes))
	}
	if bin.Op != parser.NodeAndThen {
		t.Errorf("expected NodeAndThen op, got %v", bin.Op)
	}
}

// TestParseOrElse verifies || produces a BinaryNode with NodeOrElse.
func TestParseOrElse(t *testing.T) {
	nodes := parse("fail || echo fallback\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	bin, ok := firstNode(nodes).(*parser.BinaryNode)
	if !ok {
		t.Fatalf("expected *BinaryNode, got %T", firstNode(nodes))
	}
	if bin.Op != parser.NodeOrElse {
		t.Errorf("expected NodeOrElse, got %v", bin.Op)
	}
}

// TestParseRedirectOut verifies > redirection is attached to a SimpleCommand.
func TestParseRedirectOut(t *testing.T) {
	nodes := parse("echo hi > out.txt\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	cmd, ok := firstNode(nodes).(*parser.SimpleCommand)
	if !ok {
		t.Fatalf("expected *SimpleCommand, got %T", firstNode(nodes))
	}
	if len(cmd.Redirects) == 0 {
		t.Fatal("expected at least one redirect")
	}
	r := cmd.Redirects[0]
	if r.Kind != parser.RedirectOut {
		t.Errorf("expected RedirectOut, got %v", r.Kind)
	}
	if r.Target != "out.txt" {
		t.Errorf("expected target=out.txt, got %q", r.Target)
	}
}

// TestParseRedirectAppend verifies >> is parsed as RedirectAppend.
func TestParseRedirectAppend(t *testing.T) {
	nodes := parse("echo hi >> out.txt\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	cmd, ok := firstNode(nodes).(*parser.SimpleCommand)
	if !ok {
		t.Fatalf("expected *SimpleCommand, got %T", firstNode(nodes))
	}
	if len(cmd.Redirects) == 0 {
		t.Fatal("expected at least one redirect")
	}
	if cmd.Redirects[0].Kind != parser.RedirectAppend {
		t.Errorf("expected RedirectAppend, got %v", cmd.Redirects[0].Kind)
	}
}

// TestParseMultipleCommands verifies multiple commands on separate lines.
func TestParseMultipleCommands(t *testing.T) {
	nodes := parse("echo a\necho b\n")
	if len(nodes) < 2 {
		t.Errorf("expected >= 2 nodes, got %d", len(nodes))
	}
}
