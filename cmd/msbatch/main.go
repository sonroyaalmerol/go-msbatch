package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/executor"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

func main() {
	if len(os.Args) > 1 {
		runFile(os.Args[1])
		return
	}
	runInteractive()
}

func runFile(filename string) {
	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	env := processor.NewEnvironment(true)
	proc := processor.New(env, os.Args[1:], executor.New())

	src := processor.Phase0ReadLine(string(content))
	nodes := processor.ParseExpanded(src)

	if err := proc.Execute(nodes); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}

func runInteractive() {
	env := processor.NewEnvironment(false)
	proc := processor.New(env, nil, executor.New())
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("Microsoft Windows [Version 10.0.19045.5442]")
	fmt.Println("(c) Microsoft Corporation. All rights reserved. (Go MS-Batch Implementation)")
	fmt.Println()

	for {
		pwd, _ := os.Getwd()
		fmt.Printf("%s> ", pwd)
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		nodes := processor.ParseExpanded(line)
		if err := proc.Execute(nodes); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}
}
