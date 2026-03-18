package tools

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

// Pkzip provides a compatibility layer for PKZIP using the host's 7-Zip (7z or 7za).
func Pkzip(p *processor.Processor, cmd *parser.SimpleCommand) error {
	return run7z(p, cmd, "a")
}

// Pkunzip provides a compatibility layer for PKUNZIP using the host's 7-Zip (7z or 7za).
func Pkunzip(p *processor.Processor, cmd *parser.SimpleCommand) error {
	return run7z(p, cmd, "x")
}

// Pkzipc provides a compatibility layer for PKZIPC using the host's 7-Zip (7z or 7za).
func Pkzipc(p *processor.Processor, cmd *parser.SimpleCommand) error {
	// PKZIPC is the command-line version of PKZIP.
	// We handle it similarly but may need to skip some specific flags.
	return run7z(p, cmd, "a")
}

func run7z(p *processor.Processor, cmd *parser.SimpleCommand, defaultMode string) error {
	words := cmd.Words()
	var z7Args []string
	mode := defaultMode
	var zipFile string
	var files []string
	var outDir string

	// 7z doesn't have a direct equivalent for every PKZIP flag.
	// We'll map the most common ones.
	for i := range words {
		word := words[i]
		lower := strings.ToLower(word)

		if strings.HasPrefix(word, "-") || strings.HasPrefix(word, "/") {
			flag := strings.TrimPrefix(strings.TrimPrefix(lower, "-"), "/")
			switch {
			case flag == "u":
				mode = "u" // update
			case flag == "n":
				// newer files only - 7z 'u' with some switches or just 'x' with overwrite mode
				// For pkunzip -n, it often means "newer". 7z -aoa (overwrite all) or -aos (skip)
				z7Args = append(z7Args, "-aos")
			case strings.HasPrefix(flag, "lev="):
				// compression level
				level := strings.TrimPrefix(flag, "lev=")
				z7Args = append(z7Args, "-mx="+level)
			case flag == "config":
				// ignore pkzipc config flag
			case strings.HasPrefix(flag, "archivedate="):
				// ignore
			case strings.HasPrefix(flag, "times="):
				// ignore
			default:
				// ignore unknown flags for now to avoid breaking 7z
			}
			continue
		}

		if zipFile == "" {
			zipFile = processor.MapPath(word)
		} else {
			// In PKUNZIP, the last argument might be an output directory if it ends in \
			if defaultMode == "x" && (strings.HasSuffix(word, "\\") || strings.HasSuffix(word, "/")) {
				outDir = processor.MapPath(word)
			} else {
				files = append(files, word)
			}
		}
	}

	// Find 7z, 7za, or 7zz
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
	finalArgs := []string{mode}
	finalArgs = append(finalArgs, z7Args...)
	finalArgs = append(finalArgs, zipFile)
	if outDir != "" {
		finalArgs = append(finalArgs, "-o"+outDir)
	}
	finalArgs = append(finalArgs, files...)

	p.Logger.Debug("running 7z compatibility layer", "exe", exe, "args", finalArgs)
	c := exec.Command(exe, finalArgs...)
	c.Stdout = p.Stdout
	c.Stderr = p.Stderr
	c.Stdin = p.Stdin
	c.Env = os.Environ() // Basic env inheritance

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
