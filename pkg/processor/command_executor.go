package processor

import "github.com/sonroyaalmerol/go-msbatch/pkg/parser"

// CommandExecutor handles dispatch of commands that are not part of the
// processor's own control-flow (goto, call, exit, setlocal, endlocal, shift).
//
// Implement this interface to provide custom built-in commands, or use
// internal/executor.New() for full CMD.EXE compatibility.
type CommandExecutor interface {
	ExecCommand(p *Processor, cmd *parser.SimpleCommand) error
}

// CommandExecutorFunc is an adapter that allows an ordinary function to be
// used as a CommandExecutor. If f has the appropriate signature,
// CommandExecutorFunc(f) is a CommandExecutor that calls f.
//
// This mirrors the http.HandlerFunc pattern from the standard library.
type CommandExecutorFunc func(p *Processor, cmd *parser.SimpleCommand) error

// ExecCommand calls f(p, cmd).
func (f CommandExecutorFunc) ExecCommand(p *Processor, cmd *parser.SimpleCommand) error {
	return f(p, cmd)
}
