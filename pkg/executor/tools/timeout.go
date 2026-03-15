package tools

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

func Timeout(p *processor.Processor, cmd *parser.SimpleCommand) error {
	// TIMEOUT /T <seconds> [/NOBREAK]
	seconds := 0
	args := cmd.Args
	for i, arg := range args {
		lower := strings.ToLower(arg)
		if lower == "/t" && i+1 < len(args) {
			seconds, _ = strconv.Atoi(args[i+1])
		} else if lower == "/nobreak" {
			// ignore — we never wait for keypress
		} else if n, err := strconv.Atoi(arg); err == nil {
			seconds = n
		}
	}
	if seconds < 0 {
		// /T -1 means wait indefinitely for a keypress — stub: skip
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	}
	if seconds > 0 {
		fmt.Fprintf(p.Stdout, "Waiting for %d seconds, press a key to continue ...\n", seconds)
		time.Sleep(time.Duration(seconds) * time.Second)
	}
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}
