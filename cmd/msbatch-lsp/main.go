package main

import (
	"flag"
	"log"

	"github.com/sonroyaalmerol/go-msbatch/pkg/lsp"
)

func main() {
	debug := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()

	server := lsp.NewServer(*debug)
	if err := server.RunStdio(); err != nil {
		log.Fatal(err)
	}
}
