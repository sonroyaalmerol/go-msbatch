package executor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

// exePrefix returns the command tokens from the MSBATCH_EXE_PREFIX
// environment variable, or nil when unset.
//
// MSBATCH_EXE_PREFIX is a space-separated command that is prepended to every
// .exe invocation on non-Windows hosts, e.g.:
//
//	MSBATCH_EXE_PREFIX=wine
//	MSBATCH_EXE_PREFIX=wine64
//	MSBATCH_EXE_PREFIX="wine --bottle /path/to/prefix"
//	MSBATCH_EXE_PREFIX="box64 wine"
//
// When unset (or empty), .exe files cannot be run and will produce an error.
func exePrefix() []string {
	v := os.Getenv("MSBATCH_EXE_PREFIX")
	if v == "" {
		return nil
	}
	return strings.Fields(v)
}

// runExternal is the default fallback executor. If the command resolves to a
// .bat or .cmd file it is run in-process by a child Processor (sharing the
// parent's environment and I/O). Otherwise the command is forwarded to the
// host OS via os/exec.
//
// On non-Windows systems, commands whose resolved name ends in .exe are
// dispatched through the prefix defined by MSBATCH_EXE_PREFIX; without it
// they fail immediately with a descriptive error.
//
// Argument handling differs between prefixed and native dispatch:
//   - Native: Windows-style paths in arguments are converted via MapPath and
//     glob patterns are expanded against the Unix filesystem.
//   - Prefixed (.exe): arguments are passed through unchanged. The Windows
//     binary resolves paths through its own Windows API calls (e.g. via Wine),
//     so converting them to Unix paths beforehand would break path handling.
func runExternal(p *processor.Processor, cmd *parser.SimpleCommand) error {
	cmdName := processor.MapPath(cmd.Name)

	// Determine early whether this is a prefixed .exe dispatch so that argument
	// handling can be chosen correctly before we build the args slice.
	isExe := runtime.GOOS != "windows" && strings.HasSuffix(strings.ToLower(cmdName), ".exe")

	// If the command resolves to a batch file, run it in-process.
	// (Batch files are never Wine candidates.)
	if batPath, ok := resolveBatchFile(cmdName); ok {
		// For batch files, map and glob-expand args as normal.
		var batArgs []string
		for _, arg := range cmd.Args {
			mapped := mapArg(arg)
			if strings.ContainsAny(mapped, "*?[") {
				if matches, err := filepath.Glob(mapped); err == nil && len(matches) > 0 {
					batArgs = append(batArgs, matches...)
					continue
				}
			}
			batArgs = append(batArgs, mapped)
		}
		return runBatchFile(p, batPath, batArgs)
	}

	if isExe {
		prefix := exePrefix()
		if len(prefix) == 0 {
			fmt.Fprintf(p.Stderr, "cannot execute '%s': no exe prefix configured (set MSBATCH_EXE_PREFIX, e.g. MSBATCH_EXE_PREFIX=wine)\n", cmd.Name)
			p.Env.Set("ERRORLEVEL", "9009")
			return nil
		}
		// Pass arguments to the Windows binary verbatim — no MapPath, no glob
		// expansion. The prefix tool (e.g. Wine) translates Windows paths internally.
		prefixArgs := append(append(prefix[1:], cmdName), cmd.Args...)
		return runOSCommand(p, prefix[0], prefixArgs, cmd.Name)
	}

	// Native Unix command — map paths and expand globs in arguments.
	var args []string
	for _, arg := range cmd.Args {
		mapped := mapArg(arg)
		if strings.ContainsAny(mapped, "*?[") {
			if matches, err := filepath.Glob(mapped); err == nil && len(matches) > 0 {
				args = append(args, matches...)
				continue
			}
		}
		args = append(args, mapped)
	}
	return runOSCommand(p, cmdName, args, cmd.Name)
}

// mapArg applies MapPath to an argument only when it looks like a Windows path.
func mapArg(arg string) string {
	if strings.Contains(arg, "\\") || (len(arg) >= 2 && arg[1] == ':') {
		return processor.MapPath(arg)
	}
	return arg
}

// runOSCommand executes name with args via the host OS and updates ERRORLEVEL.
// displayName is used in error messages (the original un-mapped command name).
func runOSCommand(p *processor.Processor, name string, args []string, displayName string) error {
	c := exec.Command(name, args...)
	c.Stdout = p.Stdout
	c.Stderr = p.Stderr
	c.Stdin = p.Stdin
	c.Env = os.Environ()
	for k, v := range p.Env.Snapshot() {
		c.Env = append(c.Env, fmt.Sprintf("%s=%s", k, v))
	}

	if err := c.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			p.Env.Set("ERRORLEVEL", strconv.Itoa(exitErr.ExitCode()))
		} else {
			fmt.Fprintf(p.Stderr, "'%s' is not recognized as an internal or external command, operable program or batch file.\n", displayName)
			p.Env.Set("ERRORLEVEL", "9009")
		}
	} else {
		p.Env.Set("ERRORLEVEL", "0")
	}
	return nil
}

// resolveBatchFile checks whether name resolves to a .bat or .cmd file,
// searching the current directory and then the PATH.
// Returns the resolved path and true on success.
func resolveBatchFile(name string) (string, bool) {
	lower := strings.ToLower(name)
	isBatch := strings.HasSuffix(lower, ".bat") || strings.HasSuffix(lower, ".cmd")
	hasPathSep := strings.ContainsAny(name, "/\\")

	// Name already carries a batch extension — look for it directly.
	if isBatch {
		if _, err := os.Stat(name); err == nil {
			return name, true
		}
		return "", false
	}

	// Name carries a path separator but no batch extension — try .bat/.cmd.
	if hasPathSep {
		for _, ext := range []string{".bat", ".cmd"} {
			if candidate := name + ext; fileExists(candidate) {
				return candidate, true
			}
		}
		return "", false
	}

	// Bare name: check current directory first, then every PATH directory.
	for _, ext := range []string{".bat", ".cmd"} {
		if fileExists(name + ext) {
			return name + ext, true
		}
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		for _, ext := range []string{".bat", ".cmd"} {
			if candidate := filepath.Join(dir, name+ext); fileExists(candidate) {
				return candidate, true
			}
		}
	}
	return "", false
}

// runBatchFile executes a .bat/.cmd file in-process using a child Processor.
//
// The child shares the parent's Environment (so SET changes are visible to the
// caller, matching CMD's single-session behaviour), and inherits the parent's
// I/O streams and executor registry.
//
// EXIT /B in the child terminates the script and returns control to the caller.
// A plain EXIT propagates the Exited flag to the parent, ending the parent
// script as well (matching CMD's "exit the session" behaviour).
func runBatchFile(p *processor.Processor, batPath string, args []string) error {
	content, err := os.ReadFile(batPath)
	if err != nil {
		fmt.Fprintf(p.Stderr, "'%s' is not recognized as an internal or external command, operable program or batch file.\n", batPath)
		p.Env.Set("ERRORLEVEL", "9009")
		return nil
	}

	// %0 = script name, %1..%n = forwarded arguments.
	childArgs := append([]string{batPath}, args...)

	child := processor.New(p.Env, childArgs, p.Executor)
	child.Stdout = p.Stdout
	child.Stderr = p.Stderr
	child.Stdin = p.Stdin
	child.Echo = p.Echo

	src := processor.Phase0ReadLine(string(content))
	nodes := processor.ParseExpanded(src)

	execErr := child.Execute(nodes)

	// EXIT /B produces an EXIT_LOCAL sentinel — treat as a normal return.
	if execErr != nil && execErr.Error() == "EXIT_LOCAL" {
		return nil
	}

	// Plain EXIT: propagate the "exit the session" flag to the parent.
	if child.Exited {
		p.Exited = true
	}

	return execErr
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
