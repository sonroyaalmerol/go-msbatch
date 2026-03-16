package lsp

import "strings"

// commandHelp maps lower-case command names to their help text.
// Built-in commands with full /? strings are included verbatim;
// others get a brief one-liner.
var commandHelp = map[string]string{
	// ---- tools with full help text ----
	"find": `Searches for a text string in a file or files.

FIND [/V] [/C] [/N] [/I] "string" [[path]filename[ ...]]

  /V        Displays all lines NOT containing the specified string.
  /C        Displays only the count of lines containing the string.
  /N        Displays line numbers with the displayed lines.
  /I        Ignores the case of characters when searching for the string.
  "string"  Specifies the text string to find.
  filename  Specifies a file or files to search.`,

	"sort": `Sorts input and writes results to the screen or a file.

SORT [/R] [[path]filename]

  /R          Reverses the sort order (Z to A, then 9 to 0).
  filename    Specifies the file to be sorted. If not specified,
              standard input is sorted.`,

	"tree": `Graphically displays the folder structure of a path.

TREE [path]

  path  Specifies the directory to display. Defaults to current directory.`,

	"xcopy": `Copies files and directory trees.

XCOPY source [destination] [/A | /M] [/D[:date]] [/P] [/S [/E]] [/V] [/W]
             [/C] [/I] [/Q] [/F] [/L] [/H] [/R] [/T] [/U]
             [/K] [/B] [/Y | /-Y] [/EXCLUDE:file1[+file2]...]

  /S    Copies subdirectories except empty ones.
  /E    Copies subdirectories including empty ones.
  /I    If destination doesn't exist, assume it is a directory.
  /Q    Quiet mode; suppresses per-file output.
  /L    List only; no copy performed.
  /Y    Suppress overwrite prompt.`,

	"robocopy": `Robust file copy utility.

ROBOCOPY source destination [file...] [options]

  /S      Copy subdirectories; exclude empty.
  /E      Copy subdirectories including empty.
  /MIR    Mirror a directory tree (equivalent to /E /PURGE).
  /PURGE  Delete destination files no longer in source.
  /XO     Exclude older files.
  /MT[:n] Multi-threaded copy (default 8 threads).
  /L      List only; no copy performed.`,

	"timeout": `Pauses command processing for the specified number of seconds.

TIMEOUT /T seconds [/NOBREAK]

  /T seconds  Seconds to wait (0-99999, or -1 to wait indefinitely).
  /NOBREAK    Ignore key presses.`,

	"where": `Displays the location of files matching a search pattern.

WHERE [/Q] name

  /Q    Quiet mode.
  name  Specifies the name of the file to find.`,

	"hostname": `Displays the name of the current host.

HOSTNAME`,

	"whoami": `Displays the current user name.

WHOAMI`,

	// ---- built-in commands ----
	"echo": `Displays messages or turns command echoing on or off.

ECHO [ON | OFF]
ECHO [message]

  ON | OFF  Turns command echoing on or off.
  message   Text to display.`,

	"set": `Displays, sets, or removes environment variables.

SET [variable=[string]]
SET /A expression
SET /P variable=[promptString]

  /A  Sets the variable to the result of a numeric expression.
  /P  Sets the variable to a line of input from the user.`,

	"cd": `Displays the name of or changes the current directory.

CD [/D] [path]
CHDIR [/D] [path]

  /D    Also change the current drive.
  path  Specifies the directory to change to.`,

	"chdir": `Displays the name of or changes the current directory.

CD [/D] [path]
CHDIR [/D] [path]`,

	"dir": `Displays a list of files and subdirectories in a directory.

DIR [path] [/options]`,

	"copy": `Copies one or more files to another location.

COPY [/Y | /-Y] source [+ source2...] [destination]

  /Y    Suppress overwrite prompt.
  /-Y   Prompt before overwriting.`,

	"move": `Moves files and renames files and directories.

MOVE [/Y | /-Y] source destination

  /Y    Suppress overwrite prompt.`,

	"del": `Deletes one or more files.

DEL [/P] [/F] [/S] [/Q] [/A:attrs] names
ERASE [/P] [/F] [/S] [/Q] [/A:attrs] names

  /P  Prompt before deleting.
  /F  Force delete read-only files.
  /S  Delete from all subdirectories.
  /Q  Quiet mode; no confirmation.`,

	"erase": `Deletes one or more files. Alias for DEL.`,

	"mkdir": `Creates a directory.

MKDIR [drive:]path
MD [drive:]path`,

	"md": `Creates a directory. Alias for MKDIR.`,

	"rmdir": `Removes a directory.

RMDIR [/S] [/Q] [drive:]path
RD [/S] [/Q] [drive:]path

  /S  Remove the directory tree.
  /Q  Quiet mode.`,

	"rd": `Removes a directory. Alias for RMDIR.`,

	"ren":    `Renames a file or files.\n\nREN [drive:][path]filename1 filename2`,
	"rename": `Renames a file or files. Alias for REN.`,

	"type": `Displays the contents of a text file.

TYPE [drive:][path]filename`,

	"cls": `Clears the screen.

CLS`,

	"ver": `Displays the shell version string.

VER`,

	"pause": `Suspends processing and displays a message.

PAUSE`,

	"exit": `Quits the shell or the current batch script.

EXIT [/B] [exitCode]

  /B        Exit the current batch script only (not the shell).
  exitCode  Numeric exit code to return.`,

	"goto": `Directs the batch program to a labelled line.

GOTO label
GOTO :EOF

  label  Specifies a text string used in the batch file as a label.
  :EOF   Transfers control to the end of the current file.`,

	"call": `Calls one batch program from another.

CALL [drive:][path]filename [batch-parameters]
CALL :label [arguments]

  :label  Call a subroutine defined by the given label.`,

	"if": `Performs conditional processing in batch programs.

IF [NOT] ERRORLEVEL number command
IF [NOT] string1==string2 command
IF [NOT] EXIST filename command
IF [/I] string1 op string2 command

  /I   Case-insensitive string comparison.
  op   EQU NEQ LSS LEQ GTR GEQ`,

	"for": `Runs a command for each item in a set.

FOR %%variable IN (set) DO command
FOR /D %%variable IN (set) DO command
FOR /R [[drive:]path] %%variable IN (set) DO command
FOR /L %%variable IN (start,step,end) DO command
FOR /F ["options"] %%variable IN (set) DO command`,

	"rem": `Records comments in a batch file.

REM [comment]`,

	"setlocal": `Begins localization of environment changes in a batch file.

SETLOCAL [ENABLEDELAYEDEXPANSION | DISABLEDELAYEDEXPANSION]`,

	"endlocal": `Ends localization of environment changes in a batch file.

ENDLOCAL`,

	"pushd": `Saves the current directory then changes it.

PUSHD [path]`,

	"popd": `Changes to the directory stored by the PUSHD command.

POPD`,

	"start": `Starts a separate window to run a specified program or command.

START ["title"] [/B] [/WAIT] [/D path] command [parameters]

  /B     Start application without creating a new window.
  /WAIT  Start application and wait for it to terminate.`,

	"mklink": `Creates a symbolic link.

MKLINK [[/D] | [/H] | [/J]] link target

  /D  Creates a directory symbolic link.
  /H  Creates a hard link.
  /J  Creates a directory junction.`,

	"color": `Sets the default console foreground and background colors.

COLOR [attr]

  attr  Two hex digits: background foreground (e.g. 0A = black on green).`,

	"title": `Sets the window title for a command prompt window.

TITLE [string]`,

	"path": `Displays or sets a search path for executable files.

PATH [path]
PATH ;   (clears the path)`,

	"prompt": `Changes the command prompt.

PROMPT [text]

  Common codes: $P=path, $G=>, $T=time, $D=date, $E=ESC, $$=$`,

	"more": `Displays output one screen at a time.

MORE [file]
command | MORE`,

	"assoc": `Displays or modifies file extension associations.

ASSOC [.ext[=[fileType]]]`,

	"ftype": `Displays or modifies file types used in file extension associations.

FTYPE [fileType[=[openCommandString]]]`,
}

// CommandHelp returns the help text for the named command, or empty string.
func CommandHelp(name string) string {
	return commandHelp[strings.ToLower(name)]
}
