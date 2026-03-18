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

TREE [path] [/F]

  path  Specifies the directory to display. Defaults to current directory.
  /F    Display the names of the files in each folder.
`

func Tree(p *processor.Processor, cmd *parser.SimpleCommand) error {
	root := "."
	showFiles := false
	for _, arg := range cmd.Args {
		lower := strings.ToLower(arg)
		isFlag := strings.HasPrefix(arg, "/") && !strings.ContainsRune(arg[1:], '/')
		if !isFlag {
			root = processor.MapPath(arg)
		} else if lower == "/f" {
			showFiles = true
		}
	}
	fmt.Fprintln(p.Stdout, root)
	printTree(p.Stdout, root, "", showFiles)
	p.Success()
	return nil
}

func printTree(w io.Writer, dir, prefix string, showFiles bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var dirs, files []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e)
		} else if showFiles {
			files = append(files, e)
		}
	}

	all := append(dirs, files...)
	for i, e := range all {
		isLast := i == len(all)-1
		connector, childPrefix := "├───", prefix+"│   "
		if isLast {
			connector, childPrefix = "└───", prefix+"    "
		}
		fmt.Fprintf(w, "%s%s%s\n", prefix, connector, e.Name())
		if e.IsDir() {
			printTree(w, filepath.Join(dir, e.Name()), childPrefix, showFiles)
		}
	}
}
