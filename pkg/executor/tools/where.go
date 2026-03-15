package tools

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

func Where(p *processor.Processor, cmd *parser.SimpleCommand) error {
	// WHERE [/Q] <name>
	if len(cmd.Args) == 0 {
		fmt.Fprintf(p.Stderr, "The syntax of the command is incorrect.\n")
		p.Env.Set("ERRORLEVEL", "1")
		return nil
	}
	quiet := false
	target := ""
	for _, arg := range cmd.Args {
		if strings.ToLower(arg) == "/q" {
			quiet = true
		} else {
			target = arg
		}
	}
	path, err := exec.LookPath(target)
	if err != nil {
		if !quiet {
			fmt.Fprintf(p.Stderr, "INFO: Could not find files for the given pattern(s).\n")
		}
		p.Env.Set("ERRORLEVEL", "1")
		return nil
	}
	if !quiet {
		fmt.Fprintln(p.Stdout, path)
	}
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}
