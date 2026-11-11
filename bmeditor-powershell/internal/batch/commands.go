package batch

import "sort"

// Command is a command a batch file runs: one of cmd's own, or a program
// Windows has.
type Command struct {
	Name     string
	Syntax   string
	Doc      string
	Internal bool // cmd's own
	Keyword  bool // part of a statement (do, else, exist...)
}

var commands = []Command{
	// cmd's own
	{Name: "assoc", Internal: true, Syntax: "assoc [.ext[=[fileType]]]", Doc: "Shows or changes the file type of an extension."},
	{Name: "break", Internal: true, Syntax: "break", Doc: "Does nothing on Windows; kept for DOS."},
	{Name: "call", Internal: true, Syntax: "call [drive:][path]file [args]\ncall :label [args]", Doc: "Runs another batch file and comes back, or runs a label of this file as a subroutine until goto :eof or exit /b."},
	{Name: "cd", Internal: true, Syntax: "cd [/d] [drive:][path]", Doc: "Shows or changes the current folder. /d changes the drive too. Same as chdir."},
	{Name: "chdir", Internal: true, Syntax: "chdir [/d] [drive:][path]", Doc: "Shows or changes the current folder. Same as cd."},
	{Name: "chcp", Internal: false, Syntax: "chcp [codepage]", Doc: "Shows or sets the console's code page: chcp 65001 for UTF-8."},
	{Name: "cls", Internal: true, Syntax: "cls", Doc: "Clears the console."},
	{Name: "color", Internal: true, Syntax: "color [attr]", Doc: "Sets the console's colours: two hex digits, background then text (color 0a)."},
	{Name: "copy", Internal: true, Syntax: "copy [/y|/-y] [/b|/a] source [+ source...] [destination]", Doc: "Copies files, or joins them with +. /y overwrites without asking."},
	{Name: "date", Internal: true, Syntax: "date [/t | date]", Doc: "Shows or sets the date. /t only shows it. %DATE% has it too."},
	{Name: "del", Internal: true, Syntax: "del [/p] [/f] [/s] [/q] [/a[[:]attrs]] names", Doc: "Deletes files. /s in subfolders too, /q without asking, /f read-only ones too. Same as erase."},
	{Name: "dir", Internal: true, Syntax: "dir [drive:][path][file] [/a[[:]attrs]] [/b] [/s] [/o[[:]order]]", Doc: "Lists a folder. /b bare names, /s with subfolders, /a:d folders only, /o:n by name."},
	{Name: "echo", Internal: true, Syntax: "echo [on | off]\necho message\necho.", Doc: "Prints a message, or turns the echo of commands on or off. echo. prints an empty line. @echo off at the top hides the commands."},
	{Name: "endlocal", Internal: true, Syntax: "endlocal", Doc: "Ends the changes to the environment that setlocal started: variables set since are restored."},
	{Name: "erase", Internal: true, Syntax: "erase [/p] [/f] [/s] [/q] names", Doc: "Deletes files. Same as del."},
	{Name: "exit", Internal: true, Syntax: "exit [/b] [exitCode]", Doc: "Quits cmd, or with /b only this batch file or subroutine, setting ERRORLEVEL to exitCode."},
	{Name: "for", Internal: true, Syntax: "for %%v in (set) do command\nfor /d %%v in (set) do command\nfor /r [path] %%v in (set) do command\nfor /l %%v in (start,step,end) do command\nfor /f [\"options\"] %%v in (file | \"string\" | 'command') do command", Doc: "Runs a command for each item: files of a set, folders (/d), files under a tree (/r), numbers (/l), or lines and their tokens (/f: \"tokens=1,2 delims=, skip=1 usebackq\"). In a batch file the variable is %%v; ~ modifiers work on it: %%~nxv."},
	{Name: "ftype", Internal: true, Syntax: "ftype [fileType[=[openCommandString]]]", Doc: "Shows or changes the command that opens a file type."},
	{Name: "goto", Internal: true, Syntax: "goto label\ngoto :eof", Doc: "Jumps to :label in this file. goto :eof ends the file, or the subroutine call started."},
	{Name: "if", Internal: true, Syntax: "if [not] errorlevel number command\nif [not] string1==string2 command\nif [not] exist file command\nif [not] defined variable command\nif [/i] string1 compare-op string2 command\nif ... (command) else (command)", Doc: "Runs a command on a condition. compare-op: equ, neq, lss, leq, gtr, geq. /i ignores case. else must be on the line of the closing parenthesis."},
	{Name: "md", Internal: true, Syntax: "md [drive:]path", Doc: "Makes a folder, with the folders above it. Same as mkdir."},
	{Name: "mkdir", Internal: true, Syntax: "mkdir [drive:]path", Doc: "Makes a folder, with the folders above it. Same as md."},
	{Name: "mklink", Internal: true, Syntax: "mklink [[/d] | [/h] | [/j]] link target", Doc: "Makes a symbolic link: to a file, or with /d to a folder; /h a hard link, /j a junction."},
	{Name: "move", Internal: true, Syntax: "move [/y | /-y] [drive:][path]file[,...] destination", Doc: "Moves or renames files and folders."},
	{Name: "path", Internal: true, Syntax: "path [[drive:]path[;...][;%PATH%]]\npath ;", Doc: "Shows or sets the search path of programs (%PATH%)."},
	{Name: "pause", Internal: true, Syntax: "pause", Doc: "Waits for a key: Press any key to continue... pause >nul waits without the message."},
	{Name: "popd", Internal: true, Syntax: "popd", Doc: "Goes back to the folder pushd left."},
	{Name: "prompt", Internal: true, Syntax: "prompt [text]", Doc: "Changes cmd's prompt."},
	{Name: "pushd", Internal: true, Syntax: "pushd [path | ..]", Doc: "Goes to a folder and remembers the current one for popd. A network path gets a drive letter. pushd \"%~dp0\" goes to the batch file's folder."},
	{Name: "rd", Internal: true, Syntax: "rd [/s] [/q] [drive:]path", Doc: "Removes a folder; /s with everything in it, /q without asking. Same as rmdir."},
	{Name: "rem", Internal: true, Syntax: "rem [comment]", Doc: "A comment. :: is one too, but not inside parentheses."},
	{Name: "ren", Internal: true, Syntax: "ren [drive:][path]file newName", Doc: "Renames files. Same as rename."},
	{Name: "rename", Internal: true, Syntax: "rename [drive:][path]file newName", Doc: "Renames files. Same as ren."},
	{Name: "rmdir", Internal: true, Syntax: "rmdir [/s] [/q] [drive:]path", Doc: "Removes a folder. Same as rd."},
	{Name: "set", Internal: true, Syntax: "set [variable=[string]]\nset \"variable=string\"\nset /a variable=expression\nset /p variable=[prompt]", Doc: "Shows, sets or removes environment variables. /a computes an integer expression, /p reads a line typed. set \"name=value\" keeps trailing spaces out."},
	{Name: "setlocal", Internal: true, Syntax: "setlocal [enabledelayedexpansion | disabledelayedexpansion] [enableextensions | disableextensions]", Doc: "Keeps changes to the environment local until endlocal or the end of the file. enabledelayedexpansion lets !var! read a variable as it is when the line runs, inside loops and blocks."},
	{Name: "shift", Internal: true, Syntax: "shift [/n]", Doc: "Moves the arguments down: %2 becomes %1. /n starts at the nth."},
	{Name: "start", Internal: true, Syntax: "start [\"title\"] [/d path] [/min | /max] [/wait] [/b] [command | program | file] [args]", Doc: "Starts a program or opens a file in a window of its own. The first quoted argument is the title: start \"\" \"C:\\path with spaces\\app.exe\". /wait waits for it, /b uses this console."},
	{Name: "time", Internal: true, Syntax: "time [/t | time]", Doc: "Shows or sets the time. /t only shows it. %TIME% has it too."},
	{Name: "title", Internal: true, Syntax: "title [string]", Doc: "Sets the console window's title."},
	{Name: "type", Internal: true, Syntax: "type [drive:][path]file", Doc: "Prints a text file."},
	{Name: "ver", Internal: true, Syntax: "ver", Doc: "Prints the Windows version."},
	{Name: "verify", Internal: true, Syntax: "verify [on | off]", Doc: "Kept for DOS. verify >nul sets ERRORLEVEL to 0."},
	{Name: "vol", Internal: true, Syntax: "vol [drive:]", Doc: "Shows a disk's label and serial number."},
	// Parts of statements
	{Name: "do", Keyword: true, Syntax: "for %%v in (set) do command", Doc: "What for runs for each item."},
	{Name: "else", Keyword: true, Syntax: "if condition (command) else (command)", Doc: "What if runs when the condition is false. On the line of the closing parenthesis."},
	{Name: "in", Keyword: true, Syntax: "for %%v in (set) do command", Doc: "The set a for goes through."},
	{Name: "not", Keyword: true, Syntax: "if not condition command", Doc: "Turns an if's condition around."},
	{Name: "exist", Keyword: true, Syntax: "if [not] exist file command", Doc: "True when the file or folder is there. if exist \"dir\\\" is true only for a folder."},
	{Name: "defined", Keyword: true, Syntax: "if [not] defined variable command", Doc: "True when the variable is set."},
	{Name: "errorlevel", Keyword: true, Syntax: "if [not] errorlevel number command", Doc: "True when ERRORLEVEL, the last exit code, is number or more. %ERRORLEVEL% has its value."},
	{Name: "equ", Keyword: true, Syntax: "if a equ b", Doc: "Equal."},
	{Name: "neq", Keyword: true, Syntax: "if a neq b", Doc: "Not equal."},
	{Name: "lss", Keyword: true, Syntax: "if a lss b", Doc: "Less than."},
	{Name: "leq", Keyword: true, Syntax: "if a leq b", Doc: "Less than or equal."},
	{Name: "gtr", Keyword: true, Syntax: "if a gtr b", Doc: "Greater than."},
	{Name: "geq", Keyword: true, Syntax: "if a geq b", Doc: "Greater than or equal."},
	{Name: "nul", Keyword: true, Syntax: "command >nul 2>&1", Doc: "The device that drops what is written to it."},
	{Name: "enabledelayedexpansion", Keyword: true, Syntax: "setlocal enabledelayedexpansion", Doc: "Lets !var! read a variable as it is when the line runs."},
	{Name: "enableextensions", Keyword: true, Syntax: "setlocal enableextensions", Doc: "Turns on cmd's command extensions (on by default)."},
	// Programs of Windows
	{Name: "attrib", Syntax: "attrib [+r|-r] [+a|-a] [+s|-s] [+h|-h] [path] [/s [/d]]", Doc: "Shows or changes file attributes: read-only, archive, system, hidden."},
	{Name: "choice", Syntax: "choice [/c choices] [/n] [/cs] [/t timeout /d choice] [/m text]", Doc: "Asks for one key of a set; ERRORLEVEL is its place in /c (choice /c yn /m \"Go on?\")."},
	{Name: "clip", Syntax: "command | clip", Doc: "Puts what is piped in on the clipboard."},
	{Name: "curl", Syntax: "curl [options] url", Doc: "Transfers data with URLs: curl -L -o file url downloads."},
	{Name: "find", Syntax: "find [/v] [/c] [/n] [/i] \"string\" [path]", Doc: "Finds lines with a string; /v without it, /c counts, /i ignores case."},
	{Name: "findstr", Syntax: "findstr [/s] [/i] [/r] [/c:\"string\"] [/n] [/m] strings [files]", Doc: "Finds strings or regular expressions (/r) in files or input. /i ignores case, /s in subfolders, /n with line numbers, /m only file names."},
	{Name: "forfiles", Syntax: "forfiles [/p path] [/m mask] [/s] [/c command] [/d [+|-]date]", Doc: "Runs a command for each file found, such as those older than some days (/d -30)."},
	{Name: "icacls", Syntax: "icacls name [/grant[:r] sid:perm] [/remove sid] [/t] [/c] [/q]", Doc: "Shows or changes the access control lists of files and folders."},
	{Name: "ipconfig", Syntax: "ipconfig [/all | /release | /renew | /flushdns]", Doc: "Shows the network configuration."},
	{Name: "more", Syntax: "command | more\nmore file", Doc: "Shows output a screen at a time."},
	{Name: "msbuild", Syntax: "msbuild [project] [/t:target] [/p:name=value]", Doc: "Builds a .NET or C++ project."},
	{Name: "net", Syntax: "net [use | user | localgroup | start | stop | share | session] ...", Doc: "Manages network drives, users, groups and services: net session >nul 2>&1 succeeds only as an administrator."},
	{Name: "ping", Syntax: "ping [-n count] [-w timeout] host", Doc: "Sends echo requests. ping -n 3 127.0.0.1 >nul waits about two seconds."},
	{Name: "powershell", Syntax: "powershell [-NoProfile] [-ExecutionPolicy Bypass] [-File script.ps1 | -Command \"...\"]", Doc: "Runs Windows PowerShell; pwsh runs PowerShell 7."},
	{Name: "pwsh", Syntax: "pwsh [-NoProfile] [-File script.ps1 | -Command \"...\"]", Doc: "Runs PowerShell 7."},
	{Name: "reg", Syntax: "reg query|add|delete|copy|export|import key [/v name] [/t type] [/d data] [/f]", Doc: "Reads and changes the registry."},
	{Name: "robocopy", Syntax: "robocopy source destination [files] [/e] [/mir] [/xo] [/r:n] [/w:n] [/mt[:n]] [/log:file]", Doc: "Copies folders robustly: /e with subfolders, /mir mirrors (deletes extra files too). Exit codes below 8 mean success."},
	{Name: "sc", Syntax: "sc [query | start | stop | config | create | delete] service ...", Doc: "Manages Windows services."},
	{Name: "schtasks", Syntax: "schtasks /create | /delete | /query | /run | /end ...", Doc: "Manages scheduled tasks."},
	{Name: "shutdown", Syntax: "shutdown [/s | /r | /l | /a] [/t seconds] [/f]", Doc: "Shuts down (/s), restarts (/r) or logs off (/l); /a cancels."},
	{Name: "sort", Syntax: "sort [/r] [file]", Doc: "Sorts lines; /r in reverse."},
	{Name: "taskkill", Syntax: "taskkill [/f] [/t] (/pid id | /im image)", Doc: "Ends processes: /im app.exe by name, /pid by id, /f forcibly, /t with their children."},
	{Name: "tasklist", Syntax: "tasklist [/fi \"filter\"] [/fo table|list|csv] [/nh]", Doc: "Lists running processes: tasklist /fi \"imagename eq app.exe\"."},
	{Name: "tar", Syntax: "tar -c|-x|-t [-z] -f archive [files]", Doc: "Makes (-c), unpacks (-x) or lists (-t) archives, zip too."},
	{Name: "timeout", Syntax: "timeout /t seconds [/nobreak]", Doc: "Waits some seconds, or until a key. timeout /t 5 /nobreak >nul waits five seconds whatever is pressed."},
	{Name: "tree", Syntax: "tree [drive:][path] [/f] [/a]", Doc: "Draws the folder tree; /f with files."},
	{Name: "where", Syntax: "where [/r dir] [/q] pattern", Doc: "Finds programs on PATH: where git. /q only sets ERRORLEVEL."},
	{Name: "whoami", Syntax: "whoami [/user | /groups | /priv]", Doc: "Prints the current user."},
	{Name: "wmic", Syntax: "wmic alias [where clause] get properties", Doc: "Queries Windows Management Instrumentation (deprecated: PowerShell's Get-CimInstance does it now)."},
	{Name: "xcopy", Syntax: "xcopy source [destination] [/s] [/e] [/i] [/y] [/h] [/d[:date]]", Doc: "Copies files and folders; /e with empty folders, /i treats the destination as a folder, /y overwrites. robocopy does more."},
}

var byName = func() map[string]Command {
	m := map[string]Command{}
	for _, c := range commands {
		m[c.Name] = c
	}
	return m
}()

// Lookup finds a command by name, in any case, with or without .exe.
func Lookup(name string) (Command, bool) {
	n := []rune{}
	for _, r := range name {
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		n = append(n, r)
	}
	s := string(n)
	if c, ok := byName[s]; ok {
		return c, true
	}
	if len(s) > 4 && s[len(s)-4:] == ".exe" {
		c, ok := byName[s[:len(s)-4]]
		return c, ok
	}
	return Command{}, false
}

// Commands are all the commands, sorted by name.
func Commands() []Command {
	out := append([]Command{}, commands...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// EnvVars are the variables Windows and cmd set, which a batch file reads
// without setting them.
var EnvVars = []struct{ Name, Doc string }{
	{"CD", "The current folder."},
	{"DATE", "Today's date, as the system's settings write it."},
	{"TIME", "The time now."},
	{"RANDOM", "A random number from 0 to 32767."},
	{"ERRORLEVEL", "The exit code of the last command."},
	{"CMDCMDLINE", "The command line that started cmd."},
	{"CMDEXTVERSION", "The version of cmd's command extensions."},
	{"PATH", "Where programs are looked for."},
	{"PATHEXT", "The extensions of programs: .COM;.EXE;.BAT;.CMD..."},
	{"TEMP", "The user's folder for temporary files."},
	{"TMP", "The user's folder for temporary files."},
	{"USERPROFILE", "The user's folder: C:\\Users\\name."},
	{"USERNAME", "The user's name."},
	{"USERDOMAIN", "The user's domain."},
	{"COMPUTERNAME", "The computer's name."},
	{"APPDATA", "The user's roaming application data: C:\\Users\\name\\AppData\\Roaming."},
	{"LOCALAPPDATA", "The user's local application data: C:\\Users\\name\\AppData\\Local."},
	{"PROGRAMDATA", "Application data for all users: C:\\ProgramData."},
	{"PROGRAMFILES", "C:\\Program Files."},
	{"PROGRAMFILES(X86)", "C:\\Program Files (x86)."},
	{"SYSTEMROOT", "Windows' folder: C:\\Windows."},
	{"WINDIR", "Windows' folder: C:\\Windows."},
	{"SYSTEMDRIVE", "Windows' drive: C:."},
	{"HOMEDRIVE", "The drive of the user's home."},
	{"HOMEPATH", "The user's home on HOMEDRIVE."},
	{"OS", "Windows_NT."},
	{"PROCESSOR_ARCHITECTURE", "AMD64, ARM64 or x86."},
	{"NUMBER_OF_PROCESSORS", "How many logical processors there are."},
	{"COMSPEC", "cmd.exe's path."},
	{"PUBLIC", "The public user's folder: C:\\Users\\Public."},
}

// Args are the argument forms: %0 to %9, %* and the ~ modifiers.
var Args = []struct{ Text, Doc string }{
	{"~dp0", "The batch file's folder, with a trailing backslash."},
	{"~f0", "The batch file's full path."},
	{"~nx0", "The batch file's name with its extension."},
	{"~n0", "The batch file's name without its extension."},
	{"0", "The batch file as it was called."},
	{"1", "The first argument."},
	{"~1", "The first argument without its quotes."},
	{"2", "The second argument."},
	{"*", "All the arguments."},
}
