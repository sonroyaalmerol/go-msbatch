package processor

import (
	"io"
	"os"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/lexer"
	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
)

// Processor applies the CMD.EXE parsing phases to a line of batch source and
// returns the expanded result ready for execution.
type Processor struct {
	Env     *Environment
	Args    []string // %0..%N positional arguments (batch mode)
	Echo    bool     // ECHO state (phase 3)
	ForVars map[string]string
	Stdout  io.Writer
	Stdin   io.Reader
	Stderr  io.Writer
}

// New creates a Processor.
func New(env *Environment, args []string) *Processor {
	return &Processor{
		Env:     env,
		Args:    args,
		Echo:    true,
		ForVars: make(map[string]string),
		Stdout:  os.Stdout,
		Stdin:   os.Stdin,
		Stderr:  os.Stderr,
	}
}

// ProcessLine applies phases 0-5 to a single source line and returns the
// fully-expanded line.  It does NOT execute the result.
func (p *Processor) ProcessLine(src string) string {
	// Phase 0: read-line normalisation (Ctrl-Z -> LF)
	s := Phase0ReadLine(src)

	// Phase 1: percent expansion
	s = Phase1PercentExpand(s, p.Env, p.Args)

	// Phase 4: FOR variable expansion
	if len(p.ForVars) > 0 {
		s = Phase4ForVarExpand(s, p.ForVars)
	}

	// Phase 5: delayed expansion (applied before execution per spec)
	s = Phase5DelayedExpand(s, p.Env)

	return s
}

// ProcessLineForVar applies phase-4 FOR variable expansion on top of
// ProcessLine output.  forVars maps single-char variable names to values.
func (p *Processor) ProcessLineForVar(src string, forVars map[string]string) string {
	expanded := p.ProcessLine(src)
	return Phase4ForVarExpand(expanded, forVars)
}

// ParseExpanded lexes and parses an already-expanded line, returning the AST.
func ParseExpanded(line string) []parser.Node {
	bl := lexer.New(line)
	pr := parser.New(bl)
	return pr.Parse()
}

// ExpandNode applies phases 1 and 5 to every string field in a SimpleCommand,
// returning a new SimpleCommand with expanded name and args.
func (p *Processor) ExpandNode(n *parser.SimpleCommand) *parser.SimpleCommand {
	out := &parser.SimpleCommand{
		Suppressed: n.Suppressed,
		Redirects:  n.Redirects,
	}
	out.Name = p.ProcessLine(n.Name)
	for _, a := range n.Args {
		out.Args = append(out.Args, p.ProcessLine(a))
	}
	return out
}

// ShouldEcho reports whether this command should be echoed (phase 3).
// A command suppressed by @ is never echoed.
func (p *Processor) ShouldEcho(n *parser.SimpleCommand) bool {
	if n.Suppressed {
		return false
	}
	return p.Echo
}

// HandleEchoBuiltin processes the "echo" builtin command, updating p.Echo and
// returning the text to print (empty string if it is a state-change command).
func (p *Processor) HandleEchoBuiltin(args []string) (output string, stateChanged bool) {
	if len(args) == 0 {
		if p.Echo {
			return "ECHO is on", false
		}
		return "ECHO is off", false
	}
	switch strings.ToLower(args[0]) {
	case "on":
		p.Echo = true
		return "", true
	case "off":
		p.Echo = false
		return "", true
	}
	return strings.Join(args, " "), false
}

// HandleSetBuiltin parses and applies a SET command.
// src is the raw text after "set " (i.e. "NAME=value" or "/a expr").
func (p *Processor) HandleSetBuiltin(name, value string) {
	if name != "" {
		p.Env.Set(name, value)
	}
}
