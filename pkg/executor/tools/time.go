package tools

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

const timeHelp = `Displays or sets the system time.

TIME [/T | [time]]

  /T      Displays the current time without prompting for a new one.
  time    Sets the system time to the specified value (stub).

If used without parameters, TIME displays the current time and prompts for a
new one. Press ENTER to keep the same time.
`

const dateHelp = `Displays or sets the date.

DATE [/T | [date]]

  /T      Displays the current date without prompting for a new one.
  date    Sets the system date to the specified value (stub).

If used without parameters, DATE displays the current date and prompts for a
new one. Press ENTER to keep the same date.
`

func Time(p *processor.Processor, cmd *parser.SimpleCommand) error {
	// time/t or time /t
	isDisplayOnly := false
	for _, arg := range cmd.Args {
		if strings.EqualFold(arg, "/t") {
			isDisplayOnly = true
			break
		}
	}

	now := time.Now()
	// Windows CMD uses HH:mm:ss.ms
	timeStr := now.Format("15:04:05.00")

	if isDisplayOnly {
		fmt.Fprintln(p.Stdout, now.Format("15:04"))
		return p.Success()
	}

	if len(cmd.Args) > 0 {
		// Setting time is a stub for now as it requires root/admin.
		// We just ignore the argument and return success.
		return p.Success()
	}

	fmt.Fprintf(p.Stdout, "The current time is: %s\n", timeStr)
	fmt.Fprint(p.Stdout, "Enter the new time: ")

	if p.Stdin != nil {
		scanner := bufio.NewScanner(p.Stdin)
		if scanner.Scan() {
			// user entered something or just pressed enter
		}
	}

	return p.Success()
}

func Date(p *processor.Processor, cmd *parser.SimpleCommand) error {
	isDisplayOnly := false
	for _, arg := range cmd.Args {
		if strings.EqualFold(arg, "/t") {
			isDisplayOnly = true
			break
		}
	}

	now := time.Now()
	// Windows format: Mon 01/02/2006
	dateStr := now.Format("Mon 01/02/2006")

	if isDisplayOnly {
		fmt.Fprintln(p.Stdout, now.Format("01/02/2006"))
		return p.Success()
	}

	if len(cmd.Args) > 0 {
		// Setting date is a stub.
		return p.Success()
	}

	fmt.Fprintf(p.Stdout, "The current date is: %s\n", dateStr)
	fmt.Fprint(p.Stdout, "Enter the new date: (mm-dd-yy) ")

	if p.Stdin != nil {
		scanner := bufio.NewScanner(p.Stdin)
		if scanner.Scan() {
			// user entered something or just pressed enter
		}
	}

	return p.Success()
}
