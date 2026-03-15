package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

func Xcopy(p *processor.Processor, cmd *parser.SimpleCommand) error {
	// XCOPY <src> [dst] [/S] [/E] [/I] [/Y] ...
	recursive := false
	includeEmpty := false
	var srcs []string
	var dst string

	for _, arg := range cmd.Args {
		lower := strings.ToLower(arg)
		switch lower {
		case "/s":
			recursive = true
		case "/e":
			recursive = true
			includeEmpty = true
		case "/i", "/y", "/-y", "/q", "/f", "/h", "/r", "/k", "/n", "/o", "/x":
			// ignore
		default:
			if strings.HasPrefix(arg, "/") {
				continue
			}
			mapped := processor.MapPath(arg)
			if len(srcs) == 0 {
				srcs = append(srcs, mapped)
			} else if dst == "" {
				dst = mapped
			}
		}
	}

	if len(srcs) == 0 {
		fmt.Fprintf(p.Stderr, "The syntax of the command is incorrect.\n")
		p.Env.Set("ERRORLEVEL", "1")
		return nil
	}
	if dst == "" {
		dst, _ = os.Getwd()
	}

	count := 0
	for _, src := range srcs {
		if recursive {
			err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				rel, _ := filepath.Rel(src, path)
				target := filepath.Join(dst, rel)
				if info.IsDir() {
					if includeEmpty || path != src {
						os.MkdirAll(target, 0755)
					}
					return nil
				}
				os.MkdirAll(filepath.Dir(target), 0755)
				if err := copyFile(path, target); err != nil {
					return err
				}
				count++
				return nil
			})
			if err != nil {
				fmt.Fprintf(p.Stderr, "Error during copy.\n")
				p.Env.Set("ERRORLEVEL", "1")
				return nil
			}
		} else {
			matches, err := filepath.Glob(src)
			if err != nil || len(matches) == 0 {
				matches = []string{src}
			}
			for _, m := range matches {
				info, err := os.Stat(m)
				if err != nil {
					fmt.Fprintf(p.Stderr, "File not found - %s\n", m)
					continue
				}
				if info.IsDir() {
					continue
				}
				target := dst
				if dstInfo, err := os.Stat(dst); err == nil && dstInfo.IsDir() {
					target = filepath.Join(dst, filepath.Base(m))
				}
				if err := copyFile(m, target); err != nil {
					fmt.Fprintf(p.Stderr, "Access is denied.\n")
					p.Env.Set("ERRORLEVEL", "1")
					return nil
				}
				count++
			}
		}
	}

	fmt.Fprintf(p.Stdout, "%d File(s) copied\n", count)
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}
