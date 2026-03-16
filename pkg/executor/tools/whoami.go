package tools

import (
	"fmt"
	"os/user"
	"slices"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

const whoamiHelp = `Displays the current user name.

WHOAMI
`

func Whoami(p *processor.Processor, cmd *parser.SimpleCommand) error {
	if slices.Contains(cmd.Args, "/?") {
		fmt.Fprint(p.Stdout, whoamiHelp)
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	}
	u, err := user.Current()
	if err != nil {
		fmt.Fprintf(p.Stderr, "Could not determine current user.\n")
		p.Env.Set("ERRORLEVEL", "1")
		return nil
	}
	fmt.Fprintln(p.Stdout, u.Username)
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}
