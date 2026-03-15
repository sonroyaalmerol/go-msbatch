package executor

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

// fileAssoc and fileTypes back the in-process ASSOC/FTYPE tables.
// These are not persisted to the Windows registry.
var (
	fileAssoc = map[string]string{} // ".ext"      -> file-type name
	fileTypes = map[string]string{} // type-name   -> open command
)

func cmdBreak(p *processor.Processor, _ *parser.SimpleCommand) error {
	// BREAK historically toggled extended Ctrl+C checking; it is now a no-op.
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdDate(p *processor.Processor, cmd *parser.SimpleCommand) error {
	// /T — display only (default behaviour here; setting the system date is unsupported).
	fmt.Fprintf(p.Stdout, "The current date is: %s\n", time.Now().Format("Mon 01/02/2006"))
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdTime(p *processor.Processor, cmd *parser.SimpleCommand) error {
	// /T — display only (setting the system time is unsupported).
	fmt.Fprintf(p.Stdout, "The current time is: %s\n", time.Now().Format("15:04:05.00"))
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdPath(p *processor.Processor, cmd *parser.SimpleCommand) error {
	if len(cmd.Args) == 0 {
		path, _ := p.Env.Get("PATH")
		fmt.Fprintf(p.Stdout, "PATH=%s\n", path)
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	}
	arg := strings.Join(cmd.Args, " ")
	if arg == ";" {
		p.Env.Set("PATH", "")
	} else {
		p.Env.Set("PATH", arg)
	}
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdPrompt(p *processor.Processor, cmd *parser.SimpleCommand) error {
	if len(cmd.Args) == 0 {
		p.Env.Set("PROMPT", "$P$G") // restore default
	} else {
		p.Env.Set("PROMPT", strings.Join(cmd.Args, " "))
	}
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdVerify(p *processor.Processor, cmd *parser.SimpleCommand) error {
	if len(cmd.Args) == 0 {
		state, _ := p.Env.Get("__VERIFY__")
		if state != "ON" {
			state = "OFF"
		}
		fmt.Fprintf(p.Stdout, "VERIFY is %s\n", state)
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	}
	switch strings.ToUpper(cmd.Args[0]) {
	case "ON":
		p.Env.Set("__VERIFY__", "ON")
	case "OFF":
		p.Env.Set("__VERIFY__", "OFF")
	default:
		fmt.Fprintf(p.Stderr, "You must specify ON or OFF.\n")
		p.Env.Set("ERRORLEVEL", "1")
		return nil
	}
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdVol(p *processor.Processor, _ *parser.SimpleCommand) error {
	pwd, _ := os.Getwd()
	drive := pwd
	if len(pwd) >= 2 && pwd[1] == ':' {
		drive = pwd[:2]
	}
	fmt.Fprintf(p.Stdout, " Volume in drive %s has no label.\n", drive)
	fmt.Fprintf(p.Stdout, " Volume Serial Number is 0000-0000\n")
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdAssoc(p *processor.Processor, cmd *parser.SimpleCommand) error {
	if len(cmd.Args) == 0 {
		for ext, ft := range fileAssoc {
			fmt.Fprintf(p.Stdout, "%s=%s\n", ext, ft)
		}
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	}
	arg := strings.Join(cmd.Args, " ")
	if before, after, ok := strings.Cut(arg, "="); ok {
		ext := strings.ToLower(strings.TrimSpace(before))
		if after == "" {
			delete(fileAssoc, ext)
		} else {
			fileAssoc[ext] = after
		}
	} else {
		ext := strings.ToLower(strings.TrimSpace(arg))
		ft, ok := fileAssoc[ext]
		if !ok {
			fmt.Fprintf(p.Stderr, "File association not found for extension %s\n", arg)
			p.Env.Set("ERRORLEVEL", "1")
			return nil
		}
		fmt.Fprintf(p.Stdout, "%s=%s\n", ext, ft)
	}
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdFtype(p *processor.Processor, cmd *parser.SimpleCommand) error {
	if len(cmd.Args) == 0 {
		for ft, openCmd := range fileTypes {
			fmt.Fprintf(p.Stdout, "%s=%s\n", ft, openCmd)
		}
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	}
	arg := strings.Join(cmd.Args, " ")
	if before, after, ok := strings.Cut(arg, "="); ok {
		ft := strings.TrimSpace(before)
		if after == "" {
			delete(fileTypes, ft)
		} else {
			fileTypes[ft] = after
		}
	} else {
		ft := strings.TrimSpace(arg)
		openCmd, ok := fileTypes[ft]
		if !ok {
			fmt.Fprintf(p.Stderr, "File type '%s' not found or no open command associated with it.\n", arg)
			p.Env.Set("ERRORLEVEL", "1")
			return nil
		}
		fmt.Fprintf(p.Stdout, "%s=%s\n", ft, openCmd)
	}
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdMklink(p *processor.Processor, cmd *parser.SimpleCommand) error {
	// MKLINK [[/D] | [/H] | [/J]] <Link> <Target>
	isHard := false
	var linkName, target string
	for _, arg := range cmd.Args {
		switch strings.ToLower(arg) {
		case "/d", "/j":
			// directory symlink / junction — os.Symlink handles both on Unix
		case "/h":
			isHard = true
		default:
			if linkName == "" {
				linkName = arg
			} else if target == "" {
				target = arg
			}
		}
	}
	if linkName == "" || target == "" {
		fmt.Fprintf(p.Stderr, "The syntax of the command is incorrect.\n")
		p.Env.Set("ERRORLEVEL", "1")
		return nil
	}
	linkPath := processor.MapPath(linkName)
	targetPath := processor.MapPath(target)
	var err error
	if isHard {
		err = os.Link(targetPath, linkPath)
	} else {
		err = os.Symlink(targetPath, linkPath)
	}
	if err != nil {
		fmt.Fprintf(p.Stderr, "Cannot create a file when that file already exists.\n")
		p.Env.Set("ERRORLEVEL", "1")
		return nil
	}
	if isHard {
		fmt.Fprintf(p.Stdout, "Hardlink created for %s <<===>> %s\n", linkName, target)
	} else {
		fmt.Fprintf(p.Stdout, "symbolic link created for %s <<===>> %s\n", linkName, target)
	}
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdRen(p *processor.Processor, cmd *parser.SimpleCommand) error {
	if len(cmd.Args) < 2 {
		fmt.Fprintf(p.Stderr, "The syntax of the command is incorrect.\n")
		p.Env.Set("ERRORLEVEL", "1")
		return nil
	}
	src := processor.MapPath(cmd.Args[0])
	newName := cmd.Args[1]
	matches, err := filepath.Glob(src)
	if err != nil || len(matches) == 0 {
		matches = []string{src}
	}
	for _, m := range matches {
		dst := filepath.Join(filepath.Dir(m), newName)
		if err := os.Rename(m, dst); err != nil {
			fmt.Fprintf(p.Stderr, "The system cannot find the file specified.\n")
			p.Env.Set("ERRORLEVEL", "1")
			return nil
		}
	}
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdMore(p *processor.Processor, cmd *parser.SimpleCommand) error {
	// Stub: outputs without interactive paging.
	if len(cmd.Args) == 0 {
		io.Copy(p.Stdout, p.Stdin) //nolint:errcheck
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	}
	for _, arg := range cmd.Args {
		if strings.HasPrefix(arg, "/") {
			continue
		}
		content, err := os.ReadFile(processor.MapPath(arg))
		if err != nil {
			fmt.Fprintf(p.Stderr, "The system cannot find the file specified.\n")
			p.Env.Set("ERRORLEVEL", "1")
			return nil
		}
		p.Stdout.Write(content) //nolint:errcheck
	}
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdStart(p *processor.Processor, cmd *parser.SimpleCommand) error {
	// START ["title"] [/B] [/WAIT] [/D path] [/MIN|/MAX|...] <command> [args...]
	if len(cmd.Args) == 0 {
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	}
	wait := false
	skipNext := false
	var cmdArgs []string
	for _, arg := range cmd.Args {
		if skipNext {
			skipNext = false
			continue
		}
		switch strings.ToLower(arg) {
		case "/wait":
			wait = true
		case "/d":
			skipNext = true
		case "/b", "/min", "/max", "/normal", "/high", "/realtime", "/abovenormal", "/belownormal", "/low":
			// ignore priority/window flags
		default:
			cmdArgs = append(cmdArgs, arg)
		}
	}
	if len(cmdArgs) == 0 {
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	}
	c := exec.Command(processor.MapPath(cmdArgs[0]), cmdArgs[1:]...)
	c.Stdout = p.Stdout
	c.Stderr = p.Stderr
	c.Stdin = p.Stdin
	if wait {
		if err := c.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				p.Env.Set("ERRORLEVEL", strconv.Itoa(exitErr.ExitCode()))
				return nil
			}
		}
	} else {
		c.Start() //nolint:errcheck
	}
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}
