package lsp

import "github.com/sonroyaalmerol/go-msbatch/pkg/executor"

// CommandHelp returns the help text for the named command, or empty string.
func CommandHelp(name string) string {
	return executor.CommandHelp(name)
}
