package tools

import (
	"fmt"
	"os/user"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

const whoamiHelp = `Displays the current user name.

WHOAMI
`

func Whoami(p *processor.Processor, cmd *parser.SimpleCommand) error {
	u, err := user.Current()
	if err != nil {
		fmt.Fprintf(p.Stderr, "Could not determine current user.\n")
		p.Failure()
		return nil
	}
	fmt.Fprintln(p.Stdout, u.Username)
	p.Success()
	return nil
}
