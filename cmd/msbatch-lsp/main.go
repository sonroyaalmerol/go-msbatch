// Command msbatch-lsp is a Language Server Protocol server for CMD/batch scripts.
//
// Usage:
//
//	msbatch-lsp
//
// The server communicates over stdin/stdout using the LSP JSON-RPC protocol.
// Configure your editor to launch this binary as the language server for
// .bat, .cmd, and .btm files.
package main

import (
	"fmt"
	"os"

	"github.com/sonroyaalmerol/go-msbatch/pkg/lsp"
)

func main() {
	if err := lsp.NewServer().Run(); err != nil {
		fmt.Fprintf(os.Stderr, "msbatch-lsp: %v\n", err)
		os.Exit(1)
	}
}
