package tools

import (
	"fmt"
	"os/user"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

func Whoami(p *processor.Processor, _ *parser.SimpleCommand) error {
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
