package processor

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/sonroyaalmerol/go-msbatch/pkg/lexer"
	"github.com/sonroyaalmerol/go-msbatch/pkg/logging"
	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/pathutil"
)

type Processor struct {
	Env          *Environment
	Args         []string
	OriginalArgs []string
	Echo         bool
	ForVars      map[string]string
	Stdout       io.Writer
	Stdin        io.Reader
	Stderr       io.Writer
	RawStdout    io.Writer
	RawStderr    io.Writer
	Console      io.Writer
	Logger       *slog.Logger
	Trace        *logging.TraceLogger
	CurrentFile  string
	Nodes        []parser.Node
	PC           int
	Exited       bool
	ExitCode     int
	CallDepth    int
	DirStack     []string
	Executor     CommandExecutor
	Debugger     *Debugger
	FDs          map[int]*FDHandle

	echoBlockDepth int
}

// FDHandle is one open redirection target. Writers keep both the CRLF-translated and the raw stream, because duplicating into 1 or 2 must preserve the raw stream that external processes write to.
type FDHandle struct {
	W    io.Writer
	Raw  io.Writer
	R    io.Reader
	File *os.File
}

// FDStreams returns the streams currently bound to fd, reporting false when the handle was never opened.
func (p *Processor) FDStreams(fd int) (w, raw io.Writer, r io.Reader, ok bool) {
	switch fd {
	case 0:
		return nil, nil, p.Stdin, p.Stdin != nil
	case 1:
		return p.Stdout, p.RawStdout, nil, p.Stdout != nil
	case 2:
		return p.Stderr, p.RawStderr, nil, p.Stderr != nil
	}
	if h, exists := p.FDs[fd]; exists && h != nil {
		return h.W, h.Raw, h.R, true
	}
	return nil, nil, nil, false
}

// ExtraFiles maps handles 3..N onto exec.Cmd.ExtraFiles, whose entry i is the child's descriptor 3+i. Gaps are filled with os.DevNull so the numbering cannot shift. The returned func closes those placeholders.
func (p *Processor) ExtraFiles() ([]*os.File, func()) {
	noop := func() {}

	handleFile := func(fd int) *os.File {
		h, ok := p.FDs[fd]
		if !ok || h == nil {
			return nil
		}
		if h.File != nil {
			return h.File
		}
		if f, isFile := h.Raw.(*os.File); isFile {
			return f
		}
		if f, isFile := h.W.(*os.File); isFile {
			return f
		}
		return nil
	}

	maxFD := 0
	for fd := range p.FDs {
		if fd > maxFD && handleFile(fd) != nil {
			maxFD = fd
		}
	}
	if maxFD < 3 {
		return nil, noop
	}

	var placeholders []*os.File
	cleanup := func() {
		for _, f := range placeholders {
			f.Close()
		}
	}

	files := make([]*os.File, 0, maxFD-2)
	for fd := 3; fd <= maxFD; fd++ {
		if f := handleFile(fd); f != nil {
			files = append(files, f)
			continue
		}
		devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
		if err != nil {
			cleanup()
			return nil, noop
		}
		placeholders = append(placeholders, devNull)
		files = append(files, devNull)
	}
	return files, cleanup
}

// NewCRLF wraps w so bare LF line endings become CRLF, matching CMD console
// and redirected-text output. Existing CRLF pairs pass through unchanged.
type NewCRLF struct {
	w         io.Writer
	lastWasCR bool
}

func (c *NewCRLF) Write(b []byte) (int, error) {
	var out []byte
	for _, ch := range b {
		if ch == '\n' && !c.lastWasCR {
			out = append(out, '\r', '\n')
		} else {
			out = append(out, ch)
		}
		c.lastWasCR = ch == '\r'
	}
	_, err := c.w.Write(out)
	return len(b), err
}

func NewCRLFWriter(w io.Writer) *NewCRLF { return &NewCRLF{w: w} }

func New(env *Environment, args []string, exec CommandExecutor) *Processor {
	var originalArgs []string
	if len(args) > 1 {
		originalArgs = append([]string(nil), args[1:]...)
	}
	return &Processor{
		Env:          env,
		Args:         args,
		OriginalArgs: originalArgs,
		Echo:         true,
		ForVars:      make(map[string]string),
		Stdout:       os.Stdout,
		Stdin:        os.Stdin,
		Stderr:       os.Stderr,
		Console:      os.Stdout,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		Trace:        logging.GetTrace(),
		Executor:     exec,
		Debugger:     NewDebugger(),
	}
}

func (p *Processor) SetCurrentFile(filename string) {
	p.CurrentFile = filename
	p.Trace.File(filename)
}

// ProcessLine applies all expansion phases (0, 1, 4, 5) to a single source line
// and returns the fully-expanded result. It does NOT execute the result.
func (p *Processor) ProcessLine(src string) string {
	s := Phase0ReadLine(src)
	s = p.ExpandPhase1(s)
	s = p.ExpandPhase4(s)
	s = p.ExpandPhase5(s)
	return s
}

// ExpandPhase1 performs phase-1 percent expansion (%VAR%, %0..%9).
func (p *Processor) ExpandPhase1(s string) string {
	return Phase1PercentExpand(s, p.Env, p.Args, p.OriginalArgs)
}

// ExpandPhase4 performs phase-4 FOR variable expansion (%%i).
func (p *Processor) ExpandPhase4(s string) string {
	if len(p.ForVars) > 0 {
		return Phase4ForVarExpand(s, p.ForVars)
	}
	return s
}

// ExpandPhase5 performs phase-5 delayed expansion (!VAR!).
func (p *Processor) ExpandPhase5(s string) string {
	return Phase5DelayedExpand(s, p.Env)
}

// ParseExpanded lexes and parses an already-expanded line, returning the AST.
func ParseExpanded(line string) []parser.Node {
	bl := lexer.New(line)
	pr := parser.New(bl)
	return pr.Parse()
}

// EnterBatchEcho resets block echo suppression for a called batch file; the returned func restores the caller's depth.
func (p *Processor) EnterBatchEcho() func() {
	old := p.echoBlockDepth
	p.echoBlockDepth = 0
	return func() { p.echoBlockDepth = old }
}

func (p *Processor) ShouldEcho(n parser.Node) bool {
	return p.echoTraceable(n)
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

	// Use raw arguments to preserve original spacing.
	full := strings.Join(args, "")

	// CMD's echo skips the very first delimiter character if it's a standard one:
	// Space, Tab, Comma, Semicolon, Equal, or 0xA0.
	if len(full) > 0 {
		r := full[0]
		if r == ' ' || r == '\t' || r == ',' || r == ';' || r == '=' || r == '.' || r == ':' || r == '/' || r == '\xa0' {
			full = full[1:]
		}
	}

	lower := strings.ToLower(strings.TrimSpace(full))
	if lower == "on" {
		p.Echo = true
		return "", true
	}
	if lower == "off" {
		p.Echo = false
		return "", true
	}

	return full, false
}

func (p *Processor) SetErrorLevel(code int) {
	p.Env.SetErrorLevel(code)
	p.ExitCode = code
	p.Trace.ErrorLevel(code)
}

// Success is shorthand for SetErrorLevel(0).
func (p *Processor) Success() error {
	p.SetErrorLevel(0)
	return nil
}

// Failure is shorthand for SetErrorLevel(1).
func (p *Processor) Failure() error {
	p.SetErrorLevel(1)
	return nil
}

// FailureWithCode sets the ERRORLEVEL to code and returns nil.
func (p *Processor) FailureWithCode(code int) error {
	p.SetErrorLevel(code)
	return nil
}

// ShowHelp checks if "/?" or "-?" is present in cmd.Args. If so, it prints helpText,
// sets ERRORLEVEL to 0, and returns true.
func (p *Processor) ShowHelp(cmd *parser.SimpleCommand, helpText string) bool {
	for _, arg := range cmd.Args {
		if arg == "/?" || arg == "-?" {
			fmt.Fprint(p.Stdout, helpText)
			p.Success()
			return true
		}
	}
	return false
}

func (p *Processor) HandleSetBuiltin(name, value string) {
	if name != "" {
		if value == "" {
			p.Env.Delete(name)
		} else {
			p.Env.Set(name, value)
			p.Trace.SetVar(name, value)
		}
	}
}

// ExtractRawArgString joins all raw arguments into a single string and trims
// the initial CMD-style delimiter characters (Space, Tab, Comma, Semicolon,
// Equal, or 0xA0). This is used by built-in commands that need to process
// the entire remaining command line as a single literal string (like SET or
// ECHO).
func ExtractRawArgString(args []string) string {
	full := strings.Join(args, "")
	if len(full) > 0 {
		// CMD's parser skips exactly one leading delimiter run
		trimmed := strings.TrimLeft(full, " \t\v\f\xa0,;=")
		return trimmed
	}
	return ""
}

func (p *Processor) ExpandPrompt(prompt string) string {
	now := time.Now()
	pwd, _ := os.Getwd()

	drive := ""
	if winPwd := pathutil.ToWindowsPath(pwd); len(winPwd) >= 2 && winPwd[1] == ':' {
		drive = winPwd[:1]
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
