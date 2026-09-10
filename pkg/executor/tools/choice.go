package tools

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
	"golang.org/x/term"
)

const choiceHelp = `Prompts the user to select one item from a list of choices.

CHOICE [/C [:]choices] [/N] [/CS] [/T timeout /D choice] [/M message]

  /C[:]choices  Specifies the list of choices to be created (default YN).
  /M message    Specifies the message to be displayed before the prompt.
  /N            Hides the list of choices, showing only the message.
  /CS           Enables case-sensitive choices (default is insensitive).
  /T timeout    Specifies the number of seconds to pause before defaulting.
  /D choice     Specifies the default choice after /T timeout.

ERRORLEVEL is set to the 1-based index of the selected choice, or 255 on
input error.
`

func Choice(p *processor.Processor, cmd *parser.SimpleCommand) error {
	choices := "YN"
	message := ""
	hideList := false
	caseSensitive := false
	timeout := 0
	defChoice := ""

	args := cmd.Args
	for i := 0; i < len(args); i++ {
		lower := strings.ToLower(args[i])
		value := ""
		hasValue := false
		if i+1 < len(args) {
			value = args[i+1]
			hasValue = true
		}
		switch lower {
		case "/c":
			if !hasValue {
				return choiceError(p, "/C")
			}
			choices = value
			i++
		case "/m":
			if !hasValue {
				return choiceError(p, "/M")
			}
			message = strings.Trim(value, `"`)
			i++
		case "/n":
			hideList = true
		case "/cs":
			caseSensitive = true
		case "/t":
			if !hasValue {
				return choiceError(p, "/T")
			}
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 {
				return choiceError(p, "/T")
			}
			timeout = n
			i++
		case "/d":
			if !hasValue {
				return choiceError(p, "/D")
			}
			defChoice = value
			i++
		default:
			return choiceError(p, args[i])
		}
	}

	if choices == "" {
		return choiceError(p, "/C")
	}

	index := func(s string) int {
		for j := 0; j < len(choices); j++ {
			if caseSensitive && string(choices[j]) == s ||
				!caseSensitive && strings.EqualFold(string(choices[j]), s) {
				return j + 1
			}
		}
		return 0
	}

	defIndex := 0
	if defChoice != "" {
		defIndex = index(defChoice)
		if defIndex == 0 {
			return choiceError(p, "/D")
		}
	}
	if timeout > 0 && defIndex == 0 {
		return choiceError(p, "/D")
	}

	display := func(s string) string {
		if caseSensitive {
			return s
		}
		return strings.ToUpper(s)
	}
	prompt := ""
	if message != "" {
		prompt = message + " "
	}
	if !hideList {
		_, _ = fmt.Fprintf(p.Stdout, "%s[%s]?", prompt, strings.Join(strings.Split(display(choices), ""), ","))
	} else if prompt != "" {
		_, _ = fmt.Fprint(p.Stdout, prompt)
	}

	readByte := choiceReader(p)

	pick := func(b byte) (int, bool) {
		if n := index(string(b)); n != 0 {
			_, _ = fmt.Fprintf(p.Stdout, "%s\n", display(string(b)))
			return n, true
		}
		return 0, false
	}

	if timeout > 0 {
		type read struct {
			b  byte
			ok bool
		}
		ch := make(chan read, 1)
		go func() {
			b, ok := readByte()
			ch <- read{b, ok}
		}()
		select {
		case r := <-ch:
			if r.ok {
				if n, ok := pick(r.b); ok {
					p.SetErrorLevel(n)
					return nil
				}
			} else {
				_, _ = fmt.Fprintf(p.Stdout, "%s\n", display(defChoice))
				p.SetErrorLevel(defIndex)
				return nil
			}
		case <-time.After(time.Duration(timeout) * time.Second):
			_, _ = fmt.Fprintf(p.Stdout, "%s\n", display(defChoice))
			p.SetErrorLevel(defIndex)
			return nil
		}
	}

	for {
		b, ok := readByte()
		if !ok {
			return choiceEOF(p)
		}
		if n, ok := pick(b); ok {
			p.SetErrorLevel(n)
			return nil
		}
	}
}

// choiceReader reads single bytes from the batch stdin, or the tty when no
// stdin is wired; a terminal stdin switches to raw single-key mode.
func choiceReader(p *processor.Processor) func() (byte, bool) {
	input := io.Reader(os.Stdin)
	if p.Stdin != nil {
		input = p.Stdin
	}
	if f, ok := input.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		if old, err := term.MakeRaw(int(f.Fd())); err == nil {
			return func() (byte, bool) {
				b, ok := readOne(input)
				if !ok {
					term.Restore(int(f.Fd()), old)
				}
				return b, ok
			}
		}
	}
	return func() (byte, bool) { return readOne(input) }
}

func readOne(input io.Reader) (byte, bool) {
	buf := make([]byte, 1)
	for {
		n, err := input.Read(buf)
		if n > 0 && buf[0] != '\r' && buf[0] != '\n' {
			return buf[0], true
		}
		if err != nil {
			return 0, false
		}
	}
}

func choiceError(p *processor.Processor, arg string) error {
	_, _ = fmt.Fprintf(p.Stderr, "ERROR: Invalid argument/option - '%s'.\n", arg)
	_, _ = fmt.Fprint(p.Stderr, "Type \"CHOICE /?\" for usage.\n")
	p.SetErrorLevel(255)
	return nil
}

func choiceEOF(p *processor.Processor) error {
	_, _ = fmt.Fprint(p.Stdout, "\n")
	_, _ = fmt.Fprintln(p.Stderr, "ERROR: The file is either empty or does not contain the valid choices.")
	p.SetErrorLevel(255)
	return nil
}
