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

func Tree(p *processor.Processor, cmd *parser.SimpleCommand) error {
	root := "."
	for _, arg := range cmd.Args {
		if !strings.HasPrefix(arg, "/") {
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
