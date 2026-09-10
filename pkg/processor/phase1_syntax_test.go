package processor

import (
	"bytes"
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
)

func TestPhase1ReparsesCompoundSyntax(t *testing.T) {
	env := NewEnvironment(true)
	env.Set("V", "1&echo injected")
	proc := New(env, nil, nil)
	proc.Echo = false
	var stdout, stderr bytes.Buffer
	proc.Stdout = &stdout
	proc.Stderr = &stderr

	nodes := ParseExpanded("if %V%==1 echo yes\necho after\n")
	ifNode, ok := nodes[0].(*parser.IfNode)
	if !ok {
		t.Fatalf("first node has type %T, want *parser.IfNode", nodes[0])
	}
	if got := proc.ExpandPhase1(ifNode.Raw); got != "if 1&echo injected==1 echo yes" {
		t.Fatalf("expanded IF = %q", got)
	}
	if err := proc.Execute(nodes); err != nil {
		t.Fatal(err)
	}
	if got := stdout.String(); got != "" {
		t.Errorf("stdout = %q, want empty", got)
	}
	if got := stderr.String(); got != "& was unexpected at this time.\n" {
		t.Errorf("stderr = %q", got)
	}
	if proc.ExitCode != 255 {
		t.Errorf("exit code = %d, want 255", proc.ExitCode)
	}
}
