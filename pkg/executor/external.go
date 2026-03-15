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

// runExternal is the default fallback: it resolves the command name through
// MapPath, expands glob patterns in arguments, and runs the command via
// os/exec, propagating the exit code into ERRORLEVEL.
func runExternal(p *processor.Processor, cmd *parser.SimpleCommand) error {
	cmdName := processor.MapPath(cmd.Name)

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
