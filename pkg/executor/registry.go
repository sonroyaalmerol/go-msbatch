// Package executor provides a CommandExecutor implementation for CMD.EXE
// built-in commands and external-process dispatch.
//
// Typical usage — full CMD.EXE compatibility:
//
//	proc.Executor = executor.New()
//
// To extend with custom commands while keeping all built-ins:
//
//	reg := executor.New()
//	reg.HandleFunc("greet", func(p *processor.Processor, cmd *parser.SimpleCommand) error {
//	    fmt.Fprintln(p.Stdout, "hello,", strings.Join(cmd.Args, " "))
//	    return nil
//	})
//	proc.Executor = reg
//
// To override a specific built-in:
//
//	reg := executor.New()
//	reg.HandleFunc("echo", myEcho)
//	proc.Executor = reg
//
// To build a completely custom executor with no built-ins:
//
//	reg := executor.NewEmpty()
//	reg.HandleFunc("print", myPrint)
//	proc.Executor = reg
package executor

import (
	"sort"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/executor/tools"
	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

// Registry dispatches commands to registered handlers by name (case-insensitive).
// Unrecognised commands are forwarded to the fallback executor if one is set.
//
// The zero value is valid but has no handlers and no fallback; commands are
// silently ignored. Use New to obtain a Registry pre-loaded with CMD.EXE built-ins,
// or NewEmpty for a blank slate.
type Registry struct {
	handlers map[string]processor.CommandExecutor
	fallback processor.CommandExecutor
}

// New returns a Registry pre-loaded with the full set of CMD.EXE built-in
// commands and an external-process fallback for unrecognised names.
func New() *Registry {
	r := NewEmpty()
	registerBuiltins(r)
	r.fallback = processor.CommandExecutorFunc(runExternal)
	return r
}

// NewEmpty returns a Registry with no registered handlers and no fallback.
func NewEmpty() *Registry {
	return &Registry{handlers: make(map[string]processor.CommandExecutor)}
}

// Handle registers h as the handler for the given command name.
// Names are matched case-insensitively. Registering the same name twice
// replaces the previous handler.
func (r *Registry) Handle(name string, h processor.CommandExecutor) {
	r.handlers[strings.ToLower(name)] = h
}

// HandleFunc registers fn as the handler for the given command name.
// It is shorthand for r.Handle(name, processor.CommandExecutorFunc(fn)).
func (r *Registry) HandleFunc(name string, fn func(*processor.Processor, *parser.SimpleCommand) error) {
	r.Handle(name, processor.CommandExecutorFunc(fn))
}

// SetFallback sets the executor used when no registered handler matches the
// command name. Pass nil to disable fallback (unrecognised commands are
// silently ignored).
func (r *Registry) SetFallback(h processor.CommandExecutor) {
	r.fallback = h
}

// Names returns a sorted list of all registered command names.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.handlers))
	for name := range r.handlers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ExecCommand looks up cmd.Name and calls the registered handler.
// Falls through to the fallback executor when the name is not registered.
func (r *Registry) ExecCommand(p *processor.Processor, cmd *parser.SimpleCommand) error {
	if h, ok := r.handlers[strings.ToLower(cmd.Name)]; ok {
		return h.ExecCommand(p, cmd)
	}
	if r.fallback != nil {
		return r.fallback.ExecCommand(p, cmd)
	}
	return nil
}

func registerBuiltins(r *Registry) {
	// ---- internal commands (native implementations) ----
	r.HandleFunc("echo", cmdEcho)
	r.HandleFunc("echo.", cmdEcho)
	r.HandleFunc("set", cmdSet)
	r.HandleFunc("cd", cmdCd)
	r.HandleFunc("chdir", cmdCd)
	r.HandleFunc("type", cmdType)
	r.HandleFunc("cls", cmdCls)
	r.HandleFunc("title", cmdTitle)
	r.HandleFunc("ver", cmdVer)
	r.HandleFunc("pause", cmdPause)
	r.HandleFunc("color", cmdColor)
	r.HandleFunc("pushd", cmdPushd)
	r.HandleFunc("popd", cmdPopd)
	r.HandleFunc("mkdir", cmdMkdir)
	r.HandleFunc("md", cmdMkdir)
	r.HandleFunc("rmdir", cmdRmdir)
	r.HandleFunc("rd", cmdRmdir)
	r.HandleFunc("del", cmdDel)
	r.HandleFunc("erase", cmdDel)
	r.HandleFunc("copy", cmdCopy)
	r.HandleFunc("move", cmdMove)
	r.HandleFunc("dir", cmdDir)
	r.HandleFunc("break", cmdBreak)
	r.HandleFunc("date", cmdDate)
	r.HandleFunc("time", cmdTime)
	r.HandleFunc("path", cmdPath)
	r.HandleFunc("prompt", cmdPrompt)
	r.HandleFunc("verify", cmdVerify)
	r.HandleFunc("vol", cmdVol)
	r.HandleFunc("assoc", cmdAssoc)
	r.HandleFunc("ftype", cmdFtype)
	r.HandleFunc("mklink", cmdMklink)
	r.HandleFunc("ren", cmdRen)
	r.HandleFunc("rename", cmdRen)
	r.HandleFunc("more", cmdMore)
	r.HandleFunc("start", cmdStart)
	// "exit" is handled directly by the processor's flow-control layer.

	// ---- external commands with native cross-platform implementations ----
	r.HandleFunc("hostname", tools.Hostname)
	r.HandleFunc("whoami", tools.Whoami)
	r.HandleFunc("timeout", tools.Timeout)
	r.HandleFunc("sort", tools.Sort)
	r.HandleFunc("where", tools.Where)
	r.HandleFunc("tree", tools.Tree)
	r.HandleFunc("find", tools.Find)
	r.HandleFunc("findstr", tools.Findstr)
	r.HandleFunc("xcopy", tools.Xcopy)
	r.HandleFunc("robocopy", tools.Robocopy)

}
