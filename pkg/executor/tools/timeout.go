package tools

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
	"golang.org/x/term"
)

const timeoutHelp = `Pauses command processing for the specified number of seconds.

TIMEOUT /T seconds [/NOBREAK]

  /T seconds  Specifies the number of seconds to wait (0–99999).
              Use -1 to wait indefinitely.
  /NOBREAK    Ignore key presses; wait the full duration.
`

func Timeout(p *processor.Processor, cmd *parser.SimpleCommand) error {
	// TIMEOUT /T <seconds> [/NOBREAK]
	seconds := 0
	noBreak := false
	args := cmd.Args
	for i := 0; i < len(args); i++ {
		arg := args[i]
		lower := strings.ToLower(arg)
		if lower == "/t" && i+1 < len(args) {
			seconds, _ = strconv.Atoi(args[i+1])
			i++
		} else if lower == "/nobreak" {
			noBreak = true
		} else if n, err := strconv.Atoi(arg); err == nil {
			seconds = n
		}
	}

	if seconds == 0 {
		return p.Success()
	}

	// Determine input source (prefer /dev/tty on Unix to allow interaction even if Stdin is redirected)
	var input io.Reader = p.Stdin
	var fd int = -1
	var tty *os.File

	if runtime.GOOS != "windows" {
		if f, err := os.OpenFile("/dev/tty", os.O_RDONLY, 0); err == nil {
			tty = f
			defer tty.Close()
			input = tty
			fd = int(f.Fd())
		}
	}

	if fd == -1 {
		if f, ok := input.(*os.File); ok {
			fd = int(f.Fd())
		}
	}

	if seconds < 0 {
		// /T -1 means wait indefinitely for a key press
		fmt.Fprint(p.Stdout, "Waiting for key press, press a key to continue ...")
		if fd != -1 && term.IsTerminal(fd) {
			if old, err := term.MakeRaw(fd); err == nil {
				defer term.Restore(fd, old) //nolint:errcheck
				io.ReadFull(input, make([]byte, 1)) //nolint:errcheck
			} else {
				io.ReadFull(input, make([]byte, 1)) //nolint:errcheck
			}
		} else {
			io.ReadFull(input, make([]byte, 1)) //nolint:errcheck
		}
		fmt.Fprintln(p.Stdout)
		return p.Success()
	}

	if noBreak {
		fmt.Fprintf(p.Stdout, "Waiting for %d seconds, press CTRL+C to quit ...\n", seconds)
		time.Sleep(time.Duration(seconds) * time.Second)
	} else {
		fmt.Fprintf(p.Stdout, "Waiting for %d seconds, press a key to continue ...\n", seconds)

		keyChan := make(chan struct{})
		go func() {
			// If we're in a terminal, we need to set raw mode to catch any key
			// but we can't easily do it from the goroutine because we need to
			// restore it in the main thread if the timeout expires.
			// So we assume the main thread handled raw mode if possible.
			io.ReadFull(input, make([]byte, 1)) //nolint:errcheck
			close(keyChan)
		}()

		var oldState *term.State
		if fd != -1 && term.IsTerminal(fd) {
			oldState, _ = term.MakeRaw(fd)
		}

		select {
		case <-keyChan:
			// Key pressed
		case <-time.After(time.Duration(seconds) * time.Second):
			// Timed out
		}

		if oldState != nil {
			term.Restore(fd, oldState) //nolint:errcheck
		}
	}

	return p.Success()
}
