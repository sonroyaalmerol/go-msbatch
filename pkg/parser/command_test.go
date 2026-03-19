package parser_test

import (
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
