package tools

import (
	"fmt"
	"os"
	"slices"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

const hostnameHelp = `Displays the name of the current host.

HOSTNAME
`

func Hostname(p *processor.Processor, cmd *parser.SimpleCommand) error {
	if slices.Contains(cmd.Args, "/?") {
		fmt.Fprint(p.Stdout, hostnameHelp)
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	}
	h, err := os.Hostname()
	if err != nil {
		fmt.Fprintf(p.Stderr, "Could not determine hostname.\n")
		p.Env.Set("ERRORLEVEL", "1")
		return nil
	}
	fmt.Fprintln(p.Stdout, h)
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}
