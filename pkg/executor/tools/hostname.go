package tools

import (
	"fmt"
	"os"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

func Hostname(p *processor.Processor, _ *parser.SimpleCommand) error {
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
