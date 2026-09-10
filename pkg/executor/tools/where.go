package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/pathutil"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

const whereHelp = `Displays the location of files matching a search pattern.

WHERE [/Q] name

  /Q    Quiet mode; does not display file locations or error messages.
  name  Specifies the name of the file to find.
`

// lookPath honors the interpreter's PATH (batch SET PATH included) first.
func lookPath(p *processor.Processor, target string) (string, error) {
	if pathList, ok := p.Env.Get("PATH"); ok && pathList != "" {
		if resolved, err := pathutil.LookPathIn(pathList, target); err == nil {
			return resolved, nil
		}
	}
	return exec.LookPath(target)
}

// lookPathScript finds .cmd/.bat matches lacking the exec bit (Windows
// where lists them; LookPathIn requires executables).
func lookPathScript(p *processor.Processor, target string) (string, bool) {
	if strings.ContainsAny(target, `\/`) {
		return "", false
	}
	var dirs []string
	if pathList, ok := p.Env.Get("PATH"); ok {
		dirs = append(dirs, pathutil.SplitPathList(pathList)...)
	}
	dirs = append(dirs, filepath.SplitList(os.Getenv("PATH"))...)
	for _, dir := range dirs {
		if dir == "" {
			dir = "."
		}
		for _, ext := range []string{"", ".com", ".exe", ".bat", ".cmd"} {
			candidate := filepath.Join(pathutil.MapPath(dir), target+ext)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, true
			}
		}
	}
	return "", false
}

func Where(p *processor.Processor, cmd *parser.SimpleCommand) error {
	// WHERE [/Q] <name>
	if len(cmd.Args) == 0 {
		fmt.Fprintf(p.Stderr, "The syntax of the command is incorrect.\n")
		p.Failure()
		return nil
	}
	quiet := false
	target := ""
	for _, arg := range cmd.Args {
		if strings.ToLower(arg) == "/q" {
			quiet = true
		} else {
			target = arg
		}
	}
	path, err := lookPath(p, target)
	if err != nil {
		if script, ok := lookPathScript(p, target); ok {
			path = script
		} else {
			if !quiet {
				fmt.Fprintf(p.Stderr, "INFO: Could not find files for the given pattern(s).\n")
			}
			p.Failure()
			return nil
		}
	}
	if !quiet {
		fmt.Fprintln(p.Stdout, path)
	}
	p.Success()
	return nil
}
