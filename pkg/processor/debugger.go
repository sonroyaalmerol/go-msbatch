package processor

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
)

type DebugMode int

const (
	DebugOff         DebugMode = iota
	DebugBreakpoints           // Break at REM BREAK / :: BREAK comments
	DebugStepMode              // Single-step mode
)

type Debugger struct {
	Enabled     bool
	Mode        DebugMode
	Breakpoints map[int]bool
	StepMode    bool
	Reader      *bufio.Reader
	LastCommand string
}

func NewDebugger() *Debugger {
	return &Debugger{
		Enabled:     false,
		Mode:        DebugOff,
		Breakpoints: make(map[int]bool),
		StepMode:    false,
		Reader:      bufio.NewReader(os.Stdin),
	}
}

func (d *Debugger) SetMode(mode DebugMode) {
	d.Mode = mode
	d.Enabled = (mode != DebugOff)
}

func (d *Debugger) IsBreakpoint(line int) bool {
	return d.Breakpoints[line]
}

func (d *Debugger) AddBreakpoint(line int) {
	d.Breakpoints[line] = true
}

func (d *Debugger) RemoveBreakpoint(line int) {
	delete(d.Breakpoints, line)
}

func (d *Debugger) ClearBreakpoints() {
	d.Breakpoints = make(map[int]bool)
}

func (d *Debugger) HasBreakpoints() bool {
	return len(d.Breakpoints) > 0
}

func IsBreakpointComment(n parser.Node) bool {
	comment, ok := n.(*parser.CommentNode)
	if !ok {
		return false
	}
	text := strings.ToUpper(strings.TrimSpace(comment.Text))
	words := strings.Fields(text)
	if len(words) == 0 {
		return false
	}
	return words[0] == "BREAK"
}

type DebugAction int

const (
	ActionContinue DebugAction = iota
	ActionStep
	ActionQuit
)

func (d *Debugger) Prompt(p *Processor, n parser.Node) DebugAction {
	pos := n.Pos()
	line := pos.Line + 1

	fmt.Fprintf(p.Stdout, "\n\u001b[36m\u001b[1m[DEBUG]\u001b[0m Breakpoint at %s:%d\n", p.CurrentFile, line)
	d.printSourceLine(p, n)
	fmt.Fprintf(p.Stdout, "\n")

	for {
		fmt.Fprintf(p.Stdout, "\u001b[33m(debug)\u001b[0m ")
		input, err := d.Reader.ReadString('\n')
		if err != nil {
			return ActionContinue
		}

		cmd := strings.TrimSpace(input)
		if cmd == "" {
			cmd = d.LastCommand
		} else {
			d.LastCommand = cmd
		}

		switch strings.ToLower(cmd) {
		case "", "c", "continue":
			d.StepMode = false
			return ActionContinue
		case "s", "step":
			d.StepMode = true
			return ActionStep
		case "n", "next":
			d.StepMode = false
			return ActionStep
		case "q", "quit", "exit":
			return ActionQuit
		case "h", "help", "?":
			d.printHelp(p)
		case "l", "list":
			d.listBreakpoints(p)
		case "v", "vars", "env":
			d.printVariables(p)
		case "p", "print":
			fmt.Fprintf(p.Stdout, "Usage: p <variable>\n")
		default:
			if strings.HasPrefix(strings.ToLower(cmd), "p ") ||
				strings.HasPrefix(strings.ToLower(cmd), "print ") {
				varName := strings.TrimSpace(cmd[2:])
				if strings.HasPrefix(strings.ToLower(cmd), "print ") {
					varName = strings.TrimSpace(cmd[6:])
				}
				d.printVariable(p, varName)
			} else if strings.HasPrefix(strings.ToLower(cmd), "b ") ||
				strings.HasPrefix(strings.ToLower(cmd), "break ") {
				var lineStr string
				if strings.HasPrefix(strings.ToLower(cmd), "b ") {
					lineStr = strings.TrimSpace(cmd[2:])
				} else {
					lineStr = strings.TrimSpace(cmd[6:])
				}
				d.addBreakpointFromInput(p, lineStr)
			} else if strings.HasPrefix(strings.ToLower(cmd), "d ") ||
				strings.HasPrefix(strings.ToLower(cmd), "delete ") {
				var lineStr string
				if strings.HasPrefix(strings.ToLower(cmd), "d ") {
					lineStr = strings.TrimSpace(cmd[2:])
				} else {
					lineStr = strings.TrimSpace(cmd[7:])
				}
				d.deleteBreakpointFromInput(p, lineStr)
			} else {
				fmt.Fprintf(p.Stdout, "Unknown command: %s (type 'h' for help)\n", cmd)
			}
		}
	}
}

func (d *Debugger) printHelp(p *Processor) {
	help := `
Debugger commands:
  c, continue    Continue execution until next breakpoint
  s, step        Step into next line (stay in step mode)
  n, next        Execute next line and continue
  q, quit        Exit the script
  v, vars        Show all environment variables
  p <var>        Print value of a specific variable
  b <line>       Add breakpoint at line number
  d <line>       Delete breakpoint at line number
  l, list        List all breakpoints
  h, help        Show this help
`
	fmt.Fprintf(p.Stdout, "%s", help)
}

func (d *Debugger) printSourceLine(p *Processor, n parser.Node) {
	pos := n.Pos()
	line := pos.Line + 1

	var desc string
	switch node := n.(type) {
	case *parser.SimpleCommand:
		desc = fmt.Sprintf("%s %s", node.Name, strings.Join(node.Words(), " "))
	case *parser.LabelNode:
		desc = ":" + node.Name
	case *parser.CommentNode:
		desc = "REM " + node.Text
	case *parser.IfNode:
		desc = "IF ..."
	case *parser.ForNode:
		desc = "FOR ..."
	case *parser.Block:
		desc = "( ... )"
	default:
		desc = fmt.Sprintf("[%T]", n)
	}

	fmt.Fprintf(p.Stdout, "  %d: %s\n", line, strings.TrimSpace(desc))
}

func (d *Debugger) printVariables(p *Processor) {
	snapshot := p.Env.Snapshot()
	var keys []string
	for k := range snapshot {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Fprintf(p.Stdout, "Environment variables:\n")
	for _, k := range keys {
		fmt.Fprintf(p.Stdout, "  %s=%s\n", k, snapshot[k])
	}

	if len(p.ForVars) > 0 {
		fmt.Fprintf(p.Stdout, "\nFOR variables:\n")
		for k, v := range p.ForVars {
			fmt.Fprintf(p.Stdout, "  %%%%{%s}=%s\n", k, v)
		}
	}
}

func (d *Debugger) printVariable(p *Processor, name string) {
	if val, ok := p.Env.Get(name); ok {
		fmt.Fprintf(p.Stdout, "  %s=%s\n", name, val)
		return
	}
	if val, ok := p.ForVars[name]; ok {
		fmt.Fprintf(p.Stdout, "  %%%%{%s}=%s\n", name, val)
		return
	}
	fmt.Fprintf(p.Stdout, "  Variable '%s' not defined\n", name)
}

func (d *Debugger) addBreakpointFromInput(p *Processor, lineStr string) {
	lineNum, err := strconv.Atoi(lineStr)
	if err != nil {
		fmt.Fprintf(p.Stdout, "Invalid line number: %s\n", lineStr)
		return
	}
	if lineNum < 1 {
		fmt.Fprintf(p.Stdout, "Line number must be positive: %d\n", lineNum)
		return
	}
	d.AddBreakpoint(lineNum)
	fmt.Fprintf(p.Stdout, "Breakpoint added at line %d\n", lineNum)
}

func (d *Debugger) deleteBreakpointFromInput(p *Processor, lineStr string) {
	lineNum, err := strconv.Atoi(lineStr)
	if err != nil {
		fmt.Fprintf(p.Stdout, "Invalid line number: %s\n", lineStr)
		return
	}
	if d.Breakpoints[lineNum] {
		d.RemoveBreakpoint(lineNum)
		fmt.Fprintf(p.Stdout, "Breakpoint removed at line %d\n", lineNum)
	} else {
		fmt.Fprintf(p.Stdout, "No breakpoint at line %d\n", lineNum)
	}
}

func (d *Debugger) listBreakpoints(p *Processor) {
	if len(d.Breakpoints) == 0 {
		fmt.Fprintf(p.Stdout, "No breakpoints set.\n")
		return
	}
	fmt.Fprintf(p.Stdout, "Breakpoints:\n")
	var lines []int
	for l := range d.Breakpoints {
		lines = append(lines, l)
	}
	sort.Ints(lines)
	for _, l := range lines {
		fmt.Fprintf(p.Stdout, "  line %d\n", l)
	}
}
