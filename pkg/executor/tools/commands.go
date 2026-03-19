package tools

type CommandInfo struct {
	Name        string
	Aliases     []string
	Summary     string
	Syntax      string
	Description string
}

var Commands = []CommandInfo{
	{
		Name:    "CALL",
		Summary: "Calls one batch program from another",
		Syntax:  "CALL [drive:][path]filename [batch-parameters]\nCALL :label [arguments]",
		Description: "Calls another batch program or a label within the current script. " +
			"When calling another batch file, control returns to the calling script after the called script completes.",
	},
	{
		Name:    "CD",
		Aliases: []string{"CHDIR"},
		Summary: "Displays or changes the current directory",
		Syntax:  "CD [/D] [drive:][path]\nCD [..]",
		Description: "Displays the name of or changes the current directory. " +
			"Use /D to change drive and directory at the same time.",
	},
	{
		Name:    "CHDIR",
		Aliases: []string{"CD"},
		Summary: "Displays or changes the current directory",
		Syntax:  "CHDIR [/D] [drive:][path]\nCHDIR [..]",
		Description: "Same as CD. " +
			"Displays the name of or changes the current directory.",
	},
	{
		Name:    "CLS",
		Summary: "Clears the screen",
		Syntax:  "CLS",
		Description: "Clears the command prompt window. " +
			"All text in the console buffer is erased.",
	},
	{
		Name:    "COLOR",
		Summary: "Sets the console foreground and background colors",
		Syntax:  "COLOR [attr]",
		Description: "Sets the default console foreground and background colors. " +
			"Attr is two hex digits: first is background, second is foreground. " +
			"0=Black, 1=Blue, 2=Green, 3=Aqua, 4=Red, 5=Purple, 6=Yellow, 7=White, " +
			"8=Gray, 9=Light Blue, A=Light Green, B=Light Aqua, C=Light Red, " +
			"D=Light Purple, E=Light Yellow, F=Bright White.",
	},
	{
		Name:    "COPY",
		Summary: "Copies one or more files",
		Syntax:  "COPY [/Y|/-Y] source destination",
		Description: "Copies one or more files to another location. " +
			"Use /Y to suppress prompt on overwrite, /-Y to prompt. " +
			"Can concatenate files using + operator.",
	},
	{
		Name:    "DATE",
		Summary: "Displays or sets the system date",
		Syntax:  "DATE [/T | date]",
		Description: "Displays or sets the system date. " +
			"Use /T to display date without prompting for a new one.",
	},
	{
		Name:    "DEL",
		Aliases: []string{"ERASE"},
		Summary: "Deletes one or more files",
		Syntax:  "DEL [/P] [/F] [/S] [/Q] [/A[[:]attributes]] names",
		Description: "Deletes one or more files. " +
			"/P prompts before each deletion, /F forces read-only files, " +
			"/S deletes from subdirectories, /Q quiet mode.",
	},
	{
		Name:    "DIR",
		Summary: "Displays a list of files and subdirectories",
		Syntax:  "DIR [drive:][path][filename] [/A[[:]attributes]] [/B] [/S]",
		Description: "Displays a list of a directory's files and subdirectories. " +
			"/B uses bare format, /S displays in subdirectories.",
	},
	{
		Name:    "ECHO",
		Summary: "Displays messages or turns command echoing on/off",
		Syntax:  "ECHO [ON | OFF]\nECHO [message]\nECHO.",
		Description: "Displays messages, or turns command echoing on or off. " +
			"Use ECHO. to display a blank line. " +
			"ECHO without parameters shows current echo setting.",
	},
	{
		Name:    "ENDLOCAL",
		Summary: "Ends localization of environment changes",
		Syntax:  "ENDLOCAL",
		Description: "Ends localization of environment changes in a batch file. " +
			"Environment variables modified after SETLOCAL will be restored to their original values.",
	},
	{
		Name:    "ERASE",
		Aliases: []string{"DEL"},
		Summary: "Deletes one or more files",
		Syntax:  "ERASE [/P] [/F] [/S] [/Q] [/A[[:]attributes]] names",
		Description: "Same as DEL. " +
			"Deletes one or more files from disk.",
	},
	{
		Name:    "EXIT",
		Summary: "Quits the CMD.EXE program",
		Syntax:  "EXIT [/B] [exitCode]",
		Description: "Quits the CMD.EXE program (command interpreter) or the current batch script. " +
			"Use /B to exit only the batch script. " +
			"exitCode sets the ERRORLEVEL.",
	},
	{
		Name:    "FOR",
		Summary: "Runs a specified command for each file in a set",
		Syntax:  "FOR %%parameter IN (set) DO command\nFOR /L %%parameter IN (start,step,end) DO command\nFOR /F [\"options\"] %%parameter IN (set) DO command",
		Description: "Runs a specified command for each file in a set of files. " +
			"/L iterates over a numeric range, /F parses file contents or command output. " +
			"Use %% for batch files, % for command line.",
	},
	{
		Name:    "GOTO",
		Summary: "Directs the Windows command interpreter to a labeled line",
		Syntax:  "GOTO label\nGOTO :EOF",
		Description: "Directs cmd.exe to a labeled line in a batch program. " +
			":EOF transfers to the end of the current batch script. " +
			"Labels are defined with :labelname at the start of a line.",
	},
	{
		Name:    "IF",
		Summary: "Performs conditional processing in batch programs",
		Syntax:  "IF [NOT] ERRORLEVEL number command\nIF [NOT] string1==string2 command\nIF [NOT] EXIST filename command\nIF [NOT] DEFINED variable command",
		Description: "Performs conditional processing in batch programs. " +
			"Compares strings, checks file existence, checks ERRORLEVEL, " +
			"or checks if a variable is defined. Use NOT to negate condition.",
	},
	{
		Name:    "MD",
		Aliases: []string{"MKDIR"},
		Summary: "Creates a directory",
		Syntax:  "MD [drive:]path",
		Description: "Creates a directory. " +
			"Creates intermediate directories as needed. " +
			"MKDIR is a synonym for MD.",
	},
	{
		Name:    "MKDIR",
		Aliases: []string{"MD"},
		Summary: "Creates a directory",
		Syntax:  "MKDIR [drive:]path",
		Description: "Same as MD. " +
			"Creates a directory and any necessary parent directories.",
	},
	{
		Name:    "MOVE",
		Summary: "Moves files and renames files and directories",
		Syntax:  "MOVE [/Y|/-Y] [drive:][path]filename destination",
		Description: "Moves files and renames files and directories. " +
			"Use /Y to suppress prompt on overwrite, /-Y to prompt.",
	},
	{
		Name:    "PATH",
		Summary: "Displays or sets a search path for executable files",
		Syntax:  "PATH [[drive:]path[;...]]\nPATH ;",
		Description: "Displays or sets a search path for executable files. " +
			"PATH ; clears all search path settings. " +
			"PATH without parameters displays current path.",
	},
	{
		Name:    "PAUSE",
		Summary: "Suspends processing of a batch program",
		Syntax:  "PAUSE",
		Description: "Suspends processing of a batch program and displays " +
			"the message \"Press any key to continue...\" " +
			"Useful for allowing users to read output before the window closes.",
	},
	{
		Name:    "POPD",
		Summary: "Restores the previous value of the current directory",
		Syntax:  "POPD",
		Description: "Restores the previous value of the current directory saved by PUSHD. " +
			"Changes directory to the one most recently stored by PUSHD. " +
			"Removes that directory from the stack.",
	},
	{
		Name:    "PROMPT",
		Summary: "Changes the command prompt",
		Syntax:  "PROMPT [text]",
		Description: "Changes the cmd.exe command prompt. " +
			"Special codes: $P (path), $G (>), $L (<), $B (|), $T (time), $D (date), " +
			"$V (version), $N (drive), $_ (newline), $$ ($).",
	},
	{
		Name:    "PUSHD",
		Summary: "Stores the current directory for use by POPD",
		Syntax:  "PUSHD [path | ..]",
		Description: "Saves the current directory for use by the POPD command, " +
			"then changes to the specified directory. " +
			"Creates a stack of directories that POPD can pop from.",
	},
	{
		Name:    "RD",
		Aliases: []string{"RMDIR"},
		Summary: "Removes a directory",
		Syntax:  "RD [/S] [/Q] [drive:]path",
		Description: "Removes (deletes) a directory. " +
			"/S removes the directory tree including files, " +
			"/Q quiet mode, no confirmation prompt. " +
			"RMDIR is a synonym for RD.",
	},
	{
		Name:    "REM",
		Summary: "Records comments in a batch file",
		Syntax:  "REM [comment]\n:: [comment]",
		Description: "Records comments (remarks) in a batch file or CONFIG.SYS. " +
			"Lines starting with REM or :: are ignored. " +
			":: is slightly faster as it's not parsed as a command.",
	},
	{
		Name:    "REN",
		Aliases: []string{"RENAME"},
		Summary: "Renames a file or files",
		Syntax:  "REN [drive:][path]filename1 filename2",
		Description: "Renames a file or files. " +
			"Cannot specify a different drive or path for the new file. " +
			"RENAME is a synonym for REN.",
	},
	{
		Name:    "RENAME",
		Aliases: []string{"REN"},
		Summary: "Renames a file or files",
		Syntax:  "RENAME [drive:][path]filename1 filename2",
		Description: "Same as REN. " +
			"Renames a file, keeping it in the same directory.",
	},
	{
		Name:    "RMDIR",
		Aliases: []string{"RD"},
		Summary: "Removes a directory",
		Syntax:  "RMDIR [/S] [/Q] [drive:]path",
		Description: "Same as RD. " +
			"Removes an empty directory or with /S, removes directory tree.",
	},
	{
		Name:    "SET",
		Summary: "Displays, sets, or removes cmd environment variables",
		Syntax:  "SET [variable=[string]]\nSET /A expression\nSET /P variable=[promptString]",
		Description: "Displays, sets, or removes cmd.exe environment variables. " +
			"/A evaluates arithmetic expression, " +
			"/P prompts user for input. " +
			"SET without parameters displays all variables.",
	},
	{
		Name:    "SETLOCAL",
		Summary: "Begins localization of environment changes",
		Syntax:  "SETLOCAL [ENABLEEXTENSIONS | DISABLEEXTENSIONS]\nSETLOCAL [ENABLEDELAYEDEXPANSION | DISABLEDELAYEDEXPANSION]",
		Description: "Begins localization of environment changes in a batch file. " +
			"Changes to environment variables after SETLOCAL are local to the batch file. " +
			"Use ENDLOCAL to restore the original environment. " +
			"Delayed expansion (!var!) allows variables to be expanded at execution time.",
	},
	{
		Name:    "SHIFT",
		Summary: "Changes the position of replaceable parameters",
		Syntax:  "SHIFT [/n]",
		Description: "Changes the position of replaceable parameters in a batch file. " +
			"Each SHIFT moves %1 to %0, %2 to %1, etc. " +
			"Use /n to start shifting at argument n (0-8).",
	},
	{
		Name:    "START",
		Summary: "Starts a separate window to run a program or command",
		Syntax:  "START [\"title\"] [/D path] [/MIN] [/MAX] program [parameters]",
		Description: "Starts a separate window to run a specified program or command. " +
			"/MIN starts minimized, /MAX starts maximized, " +
			"/D sets starting directory.",
	},
	{
		Name:    "TIME",
		Summary: "Displays or sets the system time",
		Syntax:  "TIME [/T | time]",
		Description: "Displays or sets the system time. " +
			"Use /T to display time without prompting for a new one. " +
			"TIME without parameters displays current time and prompts for new time.",
	},
	{
		Name:    "TITLE",
		Summary: "Sets the window title for a CMD.EXE session",
		Syntax:  "TITLE [string]",
		Description: "Sets the window title for the command prompt window. " +
			"The title appears in the window's title bar and taskbar button.",
	},
	{
		Name:    "TYPE",
		Summary: "Displays the contents of a text file",
		Syntax:  "TYPE [drive:][path]filename",
		Description: "Displays the contents of a text file or files. " +
			"Outputs file contents to stdout without paging.",
	},
	{
		Name:        "VER",
		Summary:     "Displays the Windows version",
		Syntax:      "VER",
		Description: "Displays the Windows version number.",
	},
	{
		Name:    "VERIFY",
		Summary: "Tells cmd whether to verify that files are written correctly",
		Syntax:  "VERIFY [ON | OFF]",
		Description: "Tells cmd.exe whether to verify that your files are written correctly to a disk. " +
			"VERIFY without parameters displays current setting.",
	},
	{
		Name:        "VOL",
		Summary:     "Displays a disk volume label and serial number",
		Syntax:      "VOL [drive:]",
		Description: "Displays the disk volume label and serial number, if they exist.",
	},
	{
		Name:    "XCOPY",
		Summary: "Copies files and directory trees",
		Syntax:  "XCOPY source [destination] [/A | /M] [/D[:date]] [/P] [/S [/E]] [/V] [/W] [/Y|/-Y]",
		Description: "Copies files and directory trees. " +
			"/S copies directories and subdirectories except empty ones, " +
			"/E copies directories and subdirectories including empty ones, " +
			"/Y suppresses overwrite prompt.",
	},
}

var commandMap map[string]*CommandInfo
var commandNames []string

func init() {
	commandMap = make(map[string]*CommandInfo)
	seen := make(map[string]bool)

	for i := range Commands {
		cmd := &Commands[i]
		name := cmd.Name
		if !seen[name] {
			seen[name] = true
			commandNames = append(commandNames, name)
		}
		commandMap[name] = cmd
		for _, alias := range cmd.Aliases {
			commandMap[alias] = cmd
			if !seen[alias] {
				seen[alias] = true
				commandNames = append(commandNames, alias)
			}
		}
	}
}

func GetCommand(name string) *CommandInfo {
	return commandMap[name]
}

func GetCommandNames() []string {
	return commandNames
}

func GetPrimaryCommandNames() []string {
	var names []string
	for i := range Commands {
		names = append(names, Commands[i].Name)
	}
	return names
}

func GetCommandDocumentation(name string) string {
	cmd := GetCommand(name)
	if cmd == nil {
		return ""
	}
	doc := "**" + cmd.Name + "** - " + cmd.Summary + "\n\n**Syntax:**\n" + cmd.Syntax + "\n\n**Description:**\n" + cmd.Description
	return doc
}
