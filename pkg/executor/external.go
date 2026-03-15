package executor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

// runExternal is the default fallback executor. If the command resolves to a
// .bat or .cmd file it is run in-process by a child Processor (sharing the
// parent's environment and I/O). Otherwise the command is forwarded to the
// host OS via os/exec.
func runExternal(p *processor.Processor, cmd *parser.SimpleCommand) error {
	cmdName := processor.MapPath(cmd.Name)

	// Resolve and expand glob patterns in arguments first.
	var args []string
	for _, arg := range cmd.Args {
		mapped := arg
		if strings.Contains(arg, "\\") || (len(arg) >= 2 && arg[1] == ':') {
			mapped = processor.MapPath(arg)
		}
		if strings.ContainsAny(mapped, "*?[") {
			if matches, err := filepath.Glob(mapped); err == nil && len(matches) > 0 {
				args = append(args, matches...)
				continue
			}
		}
		args = append(args, mapped)
	}

	// If the command resolves to a batch file, run it in-process.
	if batPath, ok := resolveBatchFile(cmdName); ok {
		return runBatchFile(p, batPath, args)
	}

	// Fall back to the host OS.
	c := exec.Command(cmdName, args...)
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
			fmt.Fprintf(p.Stderr, "'%s' is not recognized as an internal or external command, operable program or batch file.\n", cmd.Name)
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
