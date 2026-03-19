package tools

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/pathutil"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

type pkzipMode int

const (
	pkzipAdd pkzipMode = iota
	pkzipExtract
	pkzipUpdate
)

func Pkzip(p *processor.Processor, cmd *parser.SimpleCommand) error {
	return run7z(p, cmd, pkzipAdd)
}

func Pkunzip(p *processor.Processor, cmd *parser.SimpleCommand) error {
	return run7z(p, cmd, pkzipExtract)
}

func Pkzipc(p *processor.Processor, cmd *parser.SimpleCommand) error {
	return run7zipc(p, cmd)
}

func run7zipc(p *processor.Processor, cmd *parser.SimpleCommand) error {
	words := cmd.Words()
	mode := pkzipAdd

	for _, word := range words {
		lower := strings.ToLower(word)
		if strings.HasPrefix(word, "-") || strings.HasPrefix(word, "/") {
			flag := strings.TrimPrefix(strings.TrimPrefix(lower, "-"), "/")
			if flag == "extract" || flag == "x" || flag == "e" {
				mode = pkzipExtract
				break
			}
		}
	}

	return run7z(p, cmd, mode)
}

func run7z(p *processor.Processor, cmd *parser.SimpleCommand, defaultMode pkzipMode) error {
	words := cmd.Words()
	var z7Args []string
	mode := defaultMode
	var zipFile string
	var files []string
	var outDir string
	var recurse bool
	var overwrite bool
	var storePaths bool

	for i := range words {
		word := words[i]
		lower := strings.ToLower(word)

		if strings.HasPrefix(word, "-") || strings.HasPrefix(word, "/") {
			flag := strings.TrimPrefix(strings.TrimPrefix(lower, "-"), "/")
			switch {
			case flag == "u":
				mode = pkzipUpdate
			case flag == "n":
				if defaultMode == pkzipExtract {
					z7Args = append(z7Args, "-aos")
				}
			case flag == "o":
				overwrite = true
			case flag == "r":
				recurse = true
			case flag == "p":
				storePaths = true
			case flag == "d":
				if defaultMode == pkzipExtract {
					z7Args = append(z7Args, "-y")
				}
			case strings.HasPrefix(flag, "lev="):
				level := strings.TrimPrefix(flag, "lev=")
				z7Args = append(z7Args, "-mx="+level)
			case flag == "config":
			case strings.HasPrefix(flag, "archivedate="):
			case strings.HasPrefix(flag, "times="):
			case strings.HasPrefix(flag, "s"):
			case strings.HasPrefix(flag, "extract"):
				mode = pkzipExtract
			case flag == "add":
				mode = pkzipAdd
			}
			continue
		}

		if zipFile == "" {
			zipFile = pathutil.MapPath(word)
		} else {
			if defaultMode == pkzipExtract && (strings.HasSuffix(word, "\\") || strings.HasSuffix(word, "/")) {
				outDir = pathutil.MapPath(word)
			} else {
				files = append(files, word)
			}
		}
	}

	exe, err := exec.LookPath("7z")
	if err != nil {
		exe, err = exec.LookPath("7za")
		if err != nil {
			exe, err = exec.LookPath("7zz")
			if err != nil {
				fmt.Fprintln(p.Stderr, "7z, 7za, or 7zz not found on PATH. Please install 7-Zip.")
				p.FailureWithCode(9009)
				return nil
			}
		}
	}

	var modeStr string
	switch mode {
	case pkzipAdd:
		modeStr = "a"
	case pkzipExtract:
		modeStr = "x"
	case pkzipUpdate:
		modeStr = "u"
	}

	finalArgs := []string{modeStr}

	if recurse && (mode == pkzipAdd || mode == pkzipUpdate) {
		finalArgs = append(finalArgs, "-r")
	}

	if storePaths && (mode == pkzipAdd || mode == pkzipUpdate) {
	}

	if overwrite && mode == pkzipExtract {
		finalArgs = append(finalArgs, "-y")
		finalArgs = append(finalArgs, "-aoa")
	}

	finalArgs = append(finalArgs, z7Args...)
	finalArgs = append(finalArgs, zipFile)

	if outDir != "" {
		finalArgs = append(finalArgs, "-o"+outDir)
	}

	if recurse && (mode == pkzipAdd || mode == pkzipUpdate) && len(files) > 0 {
		for i, f := range files {
			if !strings.Contains(f, "*") && !strings.Contains(f, "?") {
				files[i] = f + "/*"
			}
		}
	}

	finalArgs = append(finalArgs, files...)

	cwd, _ := os.Getwd()
	p.Logger.Debug("running 7z compatibility layer", "exe", exe, "args", finalArgs, "cwd", cwd)
	c := exec.Command(exe, finalArgs...)
	c.Stdout = p.Stdout
	c.Stderr = p.Stderr
	c.Stdin = p.Stdin
	c.Env = os.Environ()

	if err := c.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			p.SetErrorLevel(exitErr.ExitCode())
		} else {
			fmt.Fprintf(p.Stderr, "Error executing %s: %v\n", exe, err)
			p.Failure()
		}
	} else {
		p.Success()
	}

	return nil
}
