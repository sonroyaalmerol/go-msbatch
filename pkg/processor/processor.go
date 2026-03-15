package processor

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/sonroyaalmerol/go-msbatch/pkg/lexer"
	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
)

// Processor applies the CMD.EXE parsing phases to a line of batch source and
// returns the expanded result ready for execution.
type Processor struct {
	Env      *Environment
	Args     []string // %0..%N positional arguments (batch mode)
	Echo     bool     // ECHO state (phase 3)
	ForVars  map[string]string
	Stdout   io.Writer
	Stdin    io.Reader
	Stderr   io.Writer
	Nodes    []parser.Node
	PC       int
	Exited   bool
	DirStack []string        // directory stack for PUSHD/POPD
	Executor CommandExecutor // handles non-flow-control command dispatch
}

// New creates a Processor. exec handles command dispatch; pass nil when only
// the parsing and expansion phases are needed (e.g. in tests).
func New(env *Environment, args []string, exec CommandExecutor) *Processor {
	return &Processor{
		Env:      env,
		Args:     args,
		Echo:     true,
		ForVars:  make(map[string]string),
		Stdout:   os.Stdout,
		Stdin:    os.Stdin,
		Stderr:   os.Stderr,
		Executor: exec,
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
	out.Name = strings.TrimSpace(p.ProcessLine(n.Name))
	for _, a := range n.Args {
		expanded := strings.TrimSpace(p.ProcessLine(a))
		if expanded != "" {
			out.Args = append(out.Args, expanded)
		}
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
	var filtered []string
	for _, a := range args {
		trimmed := strings.TrimSpace(a)
		if trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}

	if len(filtered) == 0 {
		if p.Echo {
			return "ECHO is on", false
		}
		return "ECHO is off", false
	}

	first := strings.ToLower(filtered[0])
	if first == "on" {
		p.Echo = true
		return "", true
	}
	if first == "off" {
		p.Echo = false
		return "", true
	}

	// JOIN original args to preserve spaces between words if they were intended
	// but we must be careful about leading spaces from TokenWhitespace.
	// Systematic way: join the filtered ones.
	return strings.Join(filtered, " "), false
}

// HandleSetBuiltin parses and applies a SET command.
// src is the raw text after "set " (i.e. "NAME=value" or "/a expr").
func (p *Processor) HandleSetBuiltin(name, value string) {
	if name != "" {
		p.Env.Set(name, value)
	}
}

// ExpandPrompt expands all $X codes in a PROMPT string.
// Supported codes (case-insensitive):
//
//	$$  →  $          $A  →  &        $B  →  |
//	$C  →  (          $D  →  date     $E  →  ESC (\x1B)
//	$F  →  )          $G  →  >        $H  →  backspace (\x08)
//	$L  →  <          $M  →  (empty on non-UNC drives)
//	$N  →  drive letter  $P  →  drive+path
//	$Q  →  =          $S  →  (space)  $T  →  time
//	$V  →  version    $_  →  newline
func (p *Processor) ExpandPrompt(prompt string) string {
	now := time.Now()
	pwd, _ := os.Getwd()

	// Drive letter: on Windows take from cwd; on Unix always empty.
	drive := ""
	if len(pwd) >= 2 && pwd[1] == ':' {
		drive = string(pwd[0])
	}

	errorlevel, _ := p.Env.Get("ERRORLEVEL")

	var sb strings.Builder
	runes := []rune(prompt)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '$' || i+1 >= len(runes) {
			sb.WriteRune(runes[i])
			continue
		}
		i++
		switch runes[i] {
		case '$':
			sb.WriteByte('$')
		case 'A', 'a':
			sb.WriteByte('&')
		case 'B', 'b':
			sb.WriteByte('|')
		case 'C', 'c':
			sb.WriteByte('(')
		case 'D', 'd':
			// Windows format: "Mon 01/02/2006"
			sb.WriteString(now.Format("Mon 01/02/2006"))
		case 'E', 'e':
			sb.WriteByte('\x1B')
		case 'F', 'f':
			sb.WriteByte(')')
		case 'G', 'g':
			sb.WriteByte('>')
		case 'H', 'h':
			sb.WriteByte('\x08')
		case 'L', 'l':
			sb.WriteByte('<')
		case 'M', 'm':
			// Remote name for mapped drives — empty on local/Unix drives.
			sb.WriteString("")
		case 'N', 'n':
			sb.WriteString(drive)
		case 'P', 'p':
			sb.WriteString(pwd)
		case 'Q', 'q':
			sb.WriteByte('=')
		case 'R', 'r':
			sb.WriteString(errorlevel)
		case 'S', 's':
			sb.WriteByte(' ')
		case 'T', 't':
			// Windows format: "15:04:05.00"
			sb.WriteString(now.Format("15:04:05.00"))
		case 'V', 'v':
			sb.WriteString("10.0.19045")
		case '_':
			sb.WriteByte('\n')
		default:
			// Unknown code — emit literally.
			sb.WriteByte('$')
			sb.WriteRune(runes[i])
		}
	}
	return sb.String()
}
