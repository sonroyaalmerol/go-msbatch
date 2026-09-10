package parser_test

import (
	"strings"
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
)

// TestCommandNameTrimmed verifies command names are trimmed of whitespace.
func TestCommandNameTrimmed(t *testing.T) {
	nodes := parse("  dir  \n")
	for _, n := range nodes {
		if cmd, ok := n.(*parser.SimpleCommand); ok {
			if cmd.Name != "dir" {
				t.Errorf("expected trimmed name=dir, got %q", cmd.Name)
			}
			return
		}
	}
	t.Error("no SimpleCommand found")
}

// TestCommandArgsSpaceSplit verifies multiple args are split by spaces.
func TestCommandArgsSpaceSplit(t *testing.T) {
	nodes := parse("echo one two three\n")
	var cmd *parser.SimpleCommand
	for _, n := range nodes {
		if c, ok := n.(*parser.SimpleCommand); ok && c.Name == "echo" {
			cmd = c
			break
		}
	}
	if cmd == nil {
		t.Fatal("no echo command found")
	}
	if len(cmd.Args) != 3 {
		t.Errorf("expected 3 args, got %d: %v", len(cmd.Args), cmd.Args)
	}
}

// TestCommandWithFlags verifies slash-flags are collected as args.
func TestCommandWithFlags(t *testing.T) {
	nodes := parse("dir /w /b\n")
	var cmd *parser.SimpleCommand
	for _, n := range nodes {
		if c, ok := n.(*parser.SimpleCommand); ok && c.Name == "dir" {
			cmd = c
			break
		}
	}
	if cmd == nil {
		t.Fatal("no dir command found")
	}
	if len(cmd.Args) < 2 {
		t.Errorf("expected >= 2 args, got %d: %v", len(cmd.Args), cmd.Args)
	}
}

// TestCommandRedirectIn verifies < redirection is parsed correctly.
func TestCommandRedirectIn(t *testing.T) {
	nodes := parse("sort < input.txt\n")
	var cmd *parser.SimpleCommand
	for _, n := range nodes {
		if c, ok := n.(*parser.SimpleCommand); ok {
			cmd = c
			break
		}
	}
	if cmd == nil {
		t.Fatal("no command found")
	}
	if len(cmd.Redirects) == 0 {
		t.Fatal("expected redirect")
	}
	r := cmd.Redirects[0]
	if r.Kind != parser.RedirectIn {
		t.Errorf("expected RedirectIn, got %v", r.Kind)
	}
	if r.FD != 0 {
		t.Errorf("expected FD=0 for stdin, got %d", r.FD)
	}
}

// TestCommandRedirectFD verifies >&N FD duplication is parsed (phase 5.5).
func TestCommandRedirectFD(t *testing.T) {
	nodes := parse("echo hi >&2\n")
	var cmd *parser.SimpleCommand
	for _, n := range nodes {
		if c, ok := n.(*parser.SimpleCommand); ok && c.Name == "echo" {
			cmd = c
			break
		}
	}
	if cmd == nil {
		t.Fatal("no echo command found")
	}
	if len(cmd.Redirects) == 0 {
		t.Fatal("expected redirect")
	}
	r := cmd.Redirects[0]
	if r.Kind != parser.RedirectOutFD {
		t.Errorf("expected RedirectOutFD, got %v", r.Kind)
	}
}

// TestCommandVariableInArgs verifies variable tokens are concatenated into args.
func TestCommandVariableInArgs(t *testing.T) {
	nodes := parse("echo %PATH%\n")
	var cmd *parser.SimpleCommand
	for _, n := range nodes {
		if c, ok := n.(*parser.SimpleCommand); ok && c.Name == "echo" {
			cmd = c
			break
		}
	}
	if cmd == nil {
		t.Fatal("no echo command found")
	}
	if len(cmd.Args) == 0 {
		t.Error("expected at least one arg containing %PATH%")
	}
}

// TestCommandExternalName verifies external (non-builtin) commands are parsed.
func TestCommandExternalName(t *testing.T) {
	nodes := parse("myprogram.exe arg1\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	cmd, ok := nodes[0].(*parser.SimpleCommand)
	if !ok {
		t.Fatalf("expected *SimpleCommand, got %T", nodes[0])
	}
	if cmd.Name != "myprogram.exe" {
		t.Errorf("expected name=myprogram.exe, got %q", cmd.Name)
	}
}

// TestGotoLabel verifies goto produces a SimpleCommand with the label as arg.
func TestGotoLabel(t *testing.T) {
	nodes := parse("goto :end\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	cmd, ok := nodes[0].(*parser.SimpleCommand)
	if !ok {
		t.Fatalf("expected *SimpleCommand, got %T", nodes[0])
	}
	if cmd.Name != "goto" {
		t.Errorf("expected name=goto, got %q", cmd.Name)
	}
}

// TestCallLabel verifies call with a label target is parsed.
func TestCallLabel(t *testing.T) {
	nodes := parse("call :myFunc arg1\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	cmd, ok := nodes[0].(*parser.SimpleCommand)
	if !ok {
		t.Fatalf("expected *SimpleCommand, got %T", nodes[0])
	}
	if cmd.Name != "call" {
		t.Errorf("expected name=call, got %q", cmd.Name)
	}
}

// TestCommandRedirectAfterQuotedArg verifies redirect is parsed after a quoted argument.
func TestCommandRedirectAfterQuotedArg(t *testing.T) {
	nodes := parse("gawk \"BEGIN {print systime()}\" > timetemp.txt\n")
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	cmd, ok := nodes[0].(*parser.SimpleCommand)
	if !ok {
		t.Fatalf("expected *SimpleCommand, got %T", nodes[0])
	}
	if cmd.Name != "gawk" {
		t.Errorf("expected name=gawk, got %q", cmd.Name)
	}
	if len(cmd.Redirects) == 0 {
		t.Fatalf("expected redirect, got none. Args=%v, RawArgs=%v", cmd.Args, cmd.RawArgs)
	}
	r := cmd.Redirects[0]
	if r.Kind != parser.RedirectOut {
		t.Errorf("expected RedirectOut, got %v", r.Kind)
	}
	if r.Target != "timetemp.txt" {
		t.Errorf("expected target=timetemp.txt, got %q", r.Target)
	}
}

func TestCommandOperatorsDoNotCrossNewlines(t *testing.T) {
	nodes := parse("echo a &\necho b\n")
	if len(nodes) != 2 {
		t.Fatalf("got %d nodes, want 2", len(nodes))
	}
	for i, want := range []string{"echo", "echo"} {
		cmd, ok := nodes[i].(*parser.SimpleCommand)
		if !ok {
			t.Fatalf("node %d has type %T, want *parser.SimpleCommand", i, nodes[i])
		}
		if cmd.Name != want {
			t.Errorf("node %d name = %q, want %q", i, cmd.Name, want)
		}
	}
}

func TestCommandLeadingOperatorAborts(t *testing.T) {
	nodes := parse("& echo unreachable\n")
	if len(nodes) == 0 {
		t.Fatal("got no nodes")
	}
	binary, ok := nodes[0].(*parser.BinaryNode)
	if !ok {
		t.Fatalf("first node has type %T, want *parser.BinaryNode", nodes[0])
	}
	abort, ok := binary.Left.(*parser.AbortNode)
	if !ok {
		t.Fatalf("left node has type %T, want *parser.AbortNode", binary.Left)
	}
	if abort.Message != "& was unexpected at this time." {
		t.Errorf("message = %q", abort.Message)
	}
}

func TestCommandRawPreservesEscapedOperators(t *testing.T) {
	nodes := parse("echo left^&right\n")
	cmd, ok := nodes[0].(*parser.SimpleCommand)
	if !ok {
		t.Fatalf("first node has type %T, want *parser.SimpleCommand", nodes[0])
	}
	if cmd.Raw != "echo left^&right" {
		t.Errorf("raw = %q", cmd.Raw)
	}
}

func TestCommandRawPreservesCompoundStatements(t *testing.T) {
	tests := []struct {
		name   string
		source string
		raw    func(parser.Node) string
	}{
		{name: "if", source: "if %V%==1 echo yes\n", raw: func(n parser.Node) string { return n.(*parser.IfNode).Raw }},
		{name: "for", source: "for %%a in (1) do echo %%a\n", raw: func(n parser.Node) string { return n.(*parser.ForNode).Raw }},
		{name: "block", source: "(echo %V%)\n", raw: func(n parser.Node) string { return n.(*parser.Block).Raw }},
		{name: "binary", source: "echo %V% & echo tail\n", raw: func(n parser.Node) string { return n.(*parser.BinaryNode).Raw }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nodes := parse(tc.source)
			if len(nodes) != 1 {
				t.Fatalf("got %d nodes, want 1", len(nodes))
			}
			want := strings.TrimSuffix(tc.source, "\n")
			if got := tc.raw(nodes[0]); got != want {
				t.Errorf("raw = %q, want %q", got, want)
			}
		})
	}
}
