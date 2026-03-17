package tools

import "strings"

// ToolHelp maps lower-case tool names to their help text.
var ToolHelp = map[string]string{
	"find":     findHelp,
	"findstr":  findstrHelp,
	"sort":     sortHelp,
	"tree":     treeHelp,
	"xcopy":    xcopyHelp,
	"robocopy": robocopyHelp,
	"timeout":  timeoutHelp,
	"where":    whereHelp,
	"hostname": hostnameHelp,
	"whoami":   whoamiHelp,
}

// Help returns the help text for the named tool, or an empty string.
func Help(name string) string {
	return ToolHelp[strings.ToLower(name)]
}
