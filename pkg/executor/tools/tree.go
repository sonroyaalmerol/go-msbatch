package tools

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

const treeHelp = `Graphically displays the folder structure of a path.

TREE [path]

  path  Specifies the directory to display. Defaults to current directory.
`

func Tree(p *processor.Processor, cmd *parser.SimpleCommand) error {
	for _, a := range cmd.Args {
		if a == "/?" {
			fmt.Fprint(p.Stdout, treeHelp)
			p.Env.Set("ERRORLEVEL", "0")
			return nil
		}
	}
	root := "."
	for _, arg := range cmd.Args {
		// Only treat as a flag if it looks like a short Windows-style flag
		// (starts with "/" but has no further "/" in the body).  Unix absolute
		// paths like /tmp/foo also start with "/" and must be treated as paths.
		isFlag := strings.HasPrefix(arg, "/") && !strings.ContainsRune(arg[1:], '/')
		if !isFlag {
			root = processor.MapPath(arg)
			break
		}
	}
	fmt.Fprintln(p.Stdout, root)
	printTree(p.Stdout, root, "")
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func printTree(w io.Writer, dir, prefix string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for i, e := range entries {
		isLast := i == len(entries)-1
		connector, childPrefix := "├───", prefix+"│   "
		if isLast {
			connector, childPrefix = "└───", prefix+"    "
		}
		fmt.Fprintf(w, "%s%s%s\n", prefix, connector, e.Name())
		if e.IsDir() {
			printTree(w, filepath.Join(dir, e.Name()), childPrefix)
		}
	}
}
