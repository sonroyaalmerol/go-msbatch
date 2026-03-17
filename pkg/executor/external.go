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

	// On non-Windows, if we're asked to run "foo.exe", check first if a native
	// "foo" exists in the same location or in PATH. If it does, we run that
	// natively instead of using the exe prefix.
	if isExe {
		nativeName := cmdName[:len(cmdName)-4]
		// Check current directory first for bare names (CMD behavior)
		if !strings.ContainsAny(nativeName, "/\\") && fileExists(nativeName) {
			isExe = false
			cmdName = "./" + nativeName
		} else if nativePath, err := exec.LookPath(nativeName); err == nil {
			isExe = false
			cmdName = nativePath
		}
	}

	// Use Words() which groups RawArgs by true whitespace.
	cmdWords := cmd.Words()

	// If the command resolves to a batch file, run it in-process.
	// (Batch files are never Wine candidates.)
	if batPath, ok := resolveBatchFile(cmdName); ok {
		// For batch files, map and glob-expand args as normal.
		// Strip CMD/CRT quoting so %1 inside the called batch receives the
		// unquoted value (matching Windows CMD CALL semantics).
		var batArgs []string
		for _, arg := range cmdWords {
			mapped := mapArg(stripExeArg(arg))
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
		// Pass the original Windows path (cmd.Name) to the prefix tool, NOT the
		// Unix-mapped cmdName.  Wine resolves "C:\foo\app.exe" via WINEPREFIX/drive_c;
		// if we pass the Unix-mapped "/mnt/c/foo/app.exe" instead, Wine maps it to
		// Z:\ (its root drive) rather than C:\, breaking GetModuleFileName and any
		// relative path lookups the exe performs against its own location.
		//
		// Arguments are passed verbatim — no MapPath, no glob expansion.
		// Wine/the Windows binary handles its own path translation internally.
		prefixArgs := make([]string, 0, len(prefix)-1+1+len(cmdWords))
		prefixArgs = append(prefixArgs, prefix[1:]...)
		prefixArgs = append(prefixArgs, cmd.Name)
		prefixArgs = append(prefixArgs, cmdWords...)
		return runOSCommand(p, prefix[0], prefixArgs, cmd.Name)
	}

	// Native Unix command — map paths, expand globs, and strip CMD/CRT quoting.
	var args []string
	for _, arg := range cmdWords {
		mapped := mapArg(stripExeArg(arg))
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

// stripExeArg removes CMD/CRT-style quoting from an argument before it is
// passed to an external process via exec.Command.  On Windows the CRT does
// this automatically when building argv from the raw command line; on Linux
// we must do it ourselves.
//
// Rules (matching the Windows CRT argv parser):
//   - A '"' toggles quoting mode; quote characters themselves are dropped.
//   - Inside a quoted section, '\"' is a literal '"' (not a closing quote).
//   - Outside quoted sections characters are taken literally.
func stripExeArg(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '"' {
			i++ // consume opening "
			for i < len(s) {
				if s[i] == '\\' && i+1 < len(s) && s[i+1] == '"' {
					b.WriteByte('"')
					i += 2
				} else if s[i] == '"' {
					i++ // consume closing "
					break
				} else {
					b.WriteByte(s[i])
					i++
				}
			}
		} else {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
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

	// Build a deduplicated environment: start with the OS environment as the
	// baseline, then let batch-level SET variables override it.  This matches
	// Windows CMD behaviour where SET changes are visible to child processes
	// and take precedence over inherited values.  Without deduplication the OS
	// value (first entry) would win on Linux because getenv() returns the first
	// match, silently ignoring any SET PATH=… the batch script issued.
	envMap := make(map[string]string, len(os.Environ()))
	for _, kv := range os.Environ() {
		if k, _, ok := strings.Cut(kv, "="); ok {
			envMap[strings.ToUpper(k)] = kv // keep original casing in value
		} else {
			envMap[strings.ToUpper(kv)] = kv
		}
	}
	for k, v := range p.Env.Snapshot() {
		envMap[strings.ToUpper(k)] = fmt.Sprintf("%s=%s", k, v)
	}
	c.Env = make([]string, 0, len(envMap))
	for _, kv := range envMap {
		c.Env = append(c.Env, kv)
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
// Windows CMD semantics observed here:
//   - The child shares the parent's Environment so SET changes are visible to
//     the caller after return.
//   - Any SETLOCAL pushed by the called batch but not matched with ENDLOCAL is
//     automatically popped on exit (CMD auto-balances SETLOCAL on batch exit).
//   - Echo state is shared: if the called batch turns echo off, the caller's
//     echo state reflects that change after return.
//   - Errors internal to the called batch (e.g. GOTO to a missing label)
//     terminate only that batch; the parent continues executing.
//   - EXIT /B terminates only the called batch and returns control to the
//     caller.
//   - A plain EXIT propagates the Exited flag to the parent, ending the whole
//     session (matching CMD's "exit the session" behaviour).
func runBatchFile(p *processor.Processor, batPath string, args []string) error {
	content, err := os.ReadFile(batPath)
	if err != nil {
		fmt.Fprintf(p.Stderr, "'%s' is not recognized as an internal or external command, operable program or batch file.\n", batPath)
		p.Env.Set("ERRORLEVEL", "9009")
		return nil
	}

	// Record the SETLOCAL stack depth so we can auto-balance on exit.
	initialDepth := p.Env.StackDepth()

	// %0 = script name, %1..%n = forwarded arguments.
	childArgs := append([]string{batPath}, args...)

	child := processor.New(p.Env, childArgs, p.Executor)
	child.Stdout = p.Stdout
	child.Stderr = p.Stderr
	child.Stdin = p.Stdin
	// Echo state is inherited from the caller; changes inside the called batch
	// persist back to the caller (CMD global echo state behaviour).
	child.Echo = p.Echo
	// Treat the called batch as if it's one CALL level deep so that
	// EXIT /B returns to the caller rather than calling os.Exit.
	child.CallDepth = 1

	src := processor.Phase0ReadLine(string(content))
	nodes := processor.ParseExpanded(src)

	execErr := child.Execute(nodes)

	// Auto-balance SETLOCAL: pop any frames the called batch opened but did
	// not close with ENDLOCAL (CMD does this automatically on batch exit).
	for p.Env.StackDepth() > initialDepth {
		p.Env.Pop()
	}

	// Propagate echo state: changes made inside the called batch persist to
	// the caller just as they would in a real CMD session.
	p.Echo = child.Echo

	// EXIT /B produces an EXIT_LOCAL sentinel — treat as a normal return.
	// ERRORLEVEL was already set by the exit command before the sentinel was
	// raised, so we must not overwrite it here.
	if execErr != nil && execErr.Error() == "EXIT_LOCAL" {
		return nil
	}

	// Plain EXIT: propagate the "exit the session" flag to the parent.
	if child.Exited {
		p.Exited = true
		return nil
	}

	// Any other error (e.g. GOTO to a missing label) terminates only the
	// called batch — the parent continues.  Print the error and set
	// ERRORLEVEL, but do not propagate.
	if execErr != nil {
		fmt.Fprintf(p.Stderr, "%v\n", execErr)
		p.Env.Set("ERRORLEVEL", "1")
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
