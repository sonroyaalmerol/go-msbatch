package tools

import (
	"fmt"
	"os"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

const hostnameHelp = `Displays the name of the current host.

HOSTNAME
`

func Hostname(p *processor.Processor, cmd *parser.SimpleCommand) error {
	h, err := os.Hostname()
	if err != nil {
		fmt.Fprintf(p.Stderr, "Could not determine hostname.\n")
		p.Failure()
		return nil
	}
	fmt.Fprintln(p.Stdout, h)
	p.Success()
	return nil
}
