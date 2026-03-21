package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

type TraceMode int

const (
	TraceOff TraceMode = iota
	TraceOn
	TraceVerbose
)

type TraceLogger struct {
	enabled bool
	verbose bool
	writer  io.Writer
	indent  int
}

var globalTrace *TraceLogger

func InitTrace(mode TraceMode, w io.Writer) {
	if w == nil {
		w = os.Stderr
	}
	globalTrace = &TraceLogger{
		enabled: mode >= TraceOn,
		verbose: mode >= TraceVerbose,
		writer:  w,
	}
}

func GetTrace() *TraceLogger {
	if globalTrace == nil {
		globalTrace = &TraceLogger{}
	}
	return globalTrace
}

func (t *TraceLogger) Enabled() bool { return t.enabled }
func (t *TraceLogger) Verbose() bool { return t.verbose }
func (t *TraceLogger) Indent()       { t.indent++ }
func (t *TraceLogger) Dedent() {
	if t.indent > 0 {
		t.indent--
	}
}
func (t *TraceLogger) SetWriter(w io.Writer) { t.writer = w }

func (t *TraceLogger) prefix() string {
	return strings.Repeat("  ", t.indent)
}

func (t *TraceLogger) File(path string) {
	if !t.enabled {
		return
	}
	fmt.Fprintf(t.writer, "%s[%s]\n", t.prefix(), path)
}

func (t *TraceLogger) Line(lineNum int, content string) {
	if !t.enabled {
		return
	}
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return
	}
	fmt.Fprintf(t.writer, "%s%4d: %s\n", t.prefix(), lineNum, trimmed)
}

func (t *TraceLogger) Command(name string, args []string) {
	if !t.enabled {
		return
	}
	argStr := strings.Join(args, " ")
	if argStr != "" {
		fmt.Fprintf(t.writer, "%s> %s %s\n", t.prefix(), name, argStr)
	} else {
		fmt.Fprintf(t.writer, "%s> %s\n", t.prefix(), name)
	}
}

func (t *TraceLogger) CallLabel(label string, args []string) {
	if !t.enabled {
		return
	}
	argStr := strings.Join(args, " ")
	if argStr != "" {
		fmt.Fprintf(t.writer, "%sCALL :%s %s\n", t.prefix(), label, argStr)
	} else {
		fmt.Fprintf(t.writer, "%sCALL :%s\n", t.prefix(), label)
	}
}

func (t *TraceLogger) GotoLabel(label string) {
	if !t.enabled {
		return
	}
	fmt.Fprintf(t.writer, "%sGOTO :%s\n", t.prefix(), label)
}

func (t *TraceLogger) CallFile(path string, args []string) {
	if !t.enabled {
		return
	}
	argStr := strings.Join(args, " ")
	if argStr != "" {
		fmt.Fprintf(t.writer, "%sCALL %s %s\n", t.prefix(), path, argStr)
	} else {
		fmt.Fprintf(t.writer, "%sCALL %s\n", t.prefix(), path)
	}
}

func (t *TraceLogger) ReturnFromLabel() {
	if !t.enabled {
		return
	}
	fmt.Fprintf(t.writer, "%sRETURN\n", t.prefix())
}

func (t *TraceLogger) Exit(code int, isLocal bool) {
	if !t.enabled {
		return
	}
	if isLocal {
		fmt.Fprintf(t.writer, "%sEXIT /B %d\n", t.prefix(), code)
	} else {
		fmt.Fprintf(t.writer, "%sEXIT %d\n", t.prefix(), code)
	}
}

func (t *TraceLogger) ErrorLevel(code int) {
	if !t.verbose {
		return
	}
	fmt.Fprintf(t.writer, "%sERRORLEVEL=%d\n", t.prefix(), code)
}

func (t *TraceLogger) SetVar(name, value string) {
	if !t.verbose {
		return
	}
	if len(value) > 50 {
		value = value[:50] + "..."
	}
	fmt.Fprintf(t.writer, "%sSET %s=%s\n", t.prefix(), name, value)
}

func (t *TraceLogger) IfCondition(result bool, cond string) {
	if !t.verbose {
		return
	}
	fmt.Fprintf(t.writer, "%sIF %s => %t\n", t.prefix(), cond, result)
}

func (t *TraceLogger) RedirectWrite(target string) {
	if !t.enabled {
		return
	}
	fmt.Fprintf(t.writer, "%s> %s\n", t.prefix(), target)
}

func (t *TraceLogger) RedirectAppend(target string) {
	if !t.enabled {
		return
	}
	fmt.Fprintf(t.writer, "%s>> %s\n", t.prefix(), target)
}

func (t *TraceLogger) RedirectRead(target string) {
	if !t.enabled {
		return
	}
	fmt.Fprintf(t.writer, "%s< %s\n", t.prefix(), target)
}

func (t *TraceLogger) DeleteFile(target string) {
	if !t.enabled {
		return
	}
	fmt.Fprintf(t.writer, "%sDEL %s\n", t.prefix(), target)
}

func NewLoggerFromEnv() *slog.Logger {
	debug := os.Getenv("MSBATCH_DEBUG")
	if debug == "" || debug == "0" || strings.ToLower(debug) == "false" || strings.ToLower(debug) == "off" {
		return slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	output := os.Getenv("MSBATCH_DEBUG_FILE")
	var writer io.Writer = os.Stderr

	switch strings.ToLower(output) {
	case "stdout":
		writer = os.Stdout
	case "stderr", "":
		writer = os.Stderr
	default:
		f, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			writer = f
		}
	}

	return slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}
