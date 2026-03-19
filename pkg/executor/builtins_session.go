package executor

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/executor/tools"
	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/pathutil"
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
	p.Success()
	return nil
}

func cmdPath(p *processor.Processor, cmd *parser.SimpleCommand) error {
	if len(cmd.Args) == 0 {
		path, _ := p.Env.Get("PATH")
		fmt.Fprintf(p.Stdout, "PATH=%s\n", path)
		p.Success()
		return nil
	}
	arg := processor.ExtractRawArgString(cmd.RawArgs)
	if arg == ";" {
		p.Env.Set("PATH", "")
	} else {
		p.Env.Set("PATH", arg)
	}
	p.Success()
	return nil
}

func cmdPrompt(p *processor.Processor, cmd *parser.SimpleCommand) error {
	if len(cmd.Args) == 0 {
		p.Env.Set("PROMPT", "$P$G") // restore default
	} else {
		arg := processor.ExtractRawArgString(cmd.RawArgs)
		p.Env.Set("PROMPT", arg)
	}
	p.Success()
	return nil
}

func cmdVerify(p *processor.Processor, cmd *parser.SimpleCommand) error {
	if len(cmd.Args) == 0 {
		state, _ := p.Env.Get("__VERIFY__")
		if state != "ON" {
			state = "OFF"
		}
		fmt.Fprintf(p.Stdout, "VERIFY is %s\n", state)
		p.Success()
		return nil
	}
	switch strings.ToUpper(cmd.Args[0]) {
	case "ON":
		p.Env.Set("__VERIFY__", "ON")
	case "OFF":
		p.Env.Set("__VERIFY__", "OFF")
	default:
		fmt.Fprintf(p.Stderr, "You must specify ON or OFF.\n")
		p.Failure()
		return nil
	}
	p.Success()
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
	p.Success()
	return nil
}

func cmdAssoc(p *processor.Processor, cmd *parser.SimpleCommand) error {
	if len(cmd.Args) == 0 {
		for ext, ft := range fileAssoc {
			fmt.Fprintf(p.Stdout, "%s=%s\n", ext, ft)
		}
		p.Success()
		return nil
	}
	arg := processor.ExtractRawArgString(cmd.RawArgs)
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
			p.Failure()
			return nil
		}
		fmt.Fprintf(p.Stdout, "%s=%s\n", ext, ft)
	}
	p.Success()
	return nil
}

func cmdFtype(p *processor.Processor, cmd *parser.SimpleCommand) error {
	if len(cmd.Args) == 0 {
		for ft, openCmd := range fileTypes {
			fmt.Fprintf(p.Stdout, "%s=%s\n", ft, openCmd)
		}
		p.Success()
		return nil
	}
	arg := processor.ExtractRawArgString(cmd.RawArgs)
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
			p.Failure()
			return nil
		}
		fmt.Fprintf(p.Stdout, "%s=%s\n", ft, openCmd)
	}
	p.Success()
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
		p.Failure()
		return nil
	}
	linkPath := pathutil.MapPath(linkName)
	targetPath := pathutil.MapPath(target)
	var err error
	if isHard {
		err = os.Link(targetPath, linkPath)
	} else {
		err = os.Symlink(targetPath, linkPath)
	}
	if err != nil {
		fmt.Fprintf(p.Stderr, "Cannot create a file when that file already exists.\n")
		p.Failure()
		return nil
	}
	if isHard {
		fmt.Fprintf(p.Stdout, "Hardlink created for %s <<===>> %s\n", linkName, target)
	} else {
		fmt.Fprintf(p.Stdout, "symbolic link created for %s <<===>> %s\n", linkName, target)
	}
	p.Success()
	return nil
}

func cmdRen(p *processor.Processor, cmd *parser.SimpleCommand) error {
	if len(cmd.Args) < 2 {
		fmt.Fprintf(p.Stderr, "The syntax of the command is incorrect.\n")
		p.Failure()
		return nil
	}
	srcPattern := cmd.Args[0]
	dstPattern := cmd.Args[1]
	src := pathutil.MapPath(srcPattern)

	matches := tools.GlobOrLiteral(src)

	dstHasWildcard := tools.HasWildcards(dstPattern)

	for _, m := range matches {
		dstName := dstPattern
		if dstHasWildcard {
			dstName = tools.SubstituteWildcard(filepath.Base(m), filepath.Base(srcPattern), dstPattern)
		}
		dst := filepath.Join(filepath.Dir(m), dstName)
		if err := os.Rename(m, dst); err != nil {
			fmt.Fprintf(p.Stderr, "The system cannot find the file specified.\n")
			p.Failure()
			return nil
		}
	}
	p.Success()
	return nil
}

func cmdMore(p *processor.Processor, cmd *parser.SimpleCommand) error {
	// Stub: outputs without interactive paging.
	if len(cmd.Args) == 0 {
		io.Copy(p.Stdout, p.Stdin) //nolint:errcheck
		p.Success()
		return nil
	}
	for _, arg := range cmd.Args {
		if strings.HasPrefix(arg, "/") {
			continue
		}
		content, err := os.ReadFile(pathutil.MapPath(arg))
		if err != nil {
			fmt.Fprintf(p.Stderr, "The system cannot find the file specified.\n")
			p.Failure()
			return nil
		}
		p.Stdout.Write(content) //nolint:errcheck
	}
	p.Success()
	return nil
}

func cmdStart(p *processor.Processor, cmd *parser.SimpleCommand) error {
	// START ["title"] [/B] [/WAIT] [/D path] [/MIN|/MAX|...] <command> [args...]
	if len(cmd.Args) == 0 {
		p.Success()
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
		p.Success()
		return nil
	}
	c := exec.Command(pathutil.MapPath(cmdArgs[0]), cmdArgs[1:]...)
	c.Stdout = p.Stdout
	c.Stderr = p.Stderr
	c.Stdin = p.Stdin
	if wait {
		if err := c.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				p.FailureWithCode(exitErr.ExitCode())
				return nil
			}
		}
	} else {
		c.Start() //nolint:errcheck
	}
	p.Success()
	return nil
}
