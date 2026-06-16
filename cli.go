package devflow

import "strings"

// ParseCLIArgs parses command line arguments for devflow tools (codejob, gopush).
// It returns the message, tag, whether help was requested, and whether -release flag is present.
// Flags like -release and --release are detected and excluded from message/tag assignment.
func ParseCLIArgs(args []string) (message, tag string, isHelp, isRelease bool) {
	if len(args) > 1 {
		arg := strings.ToLower(args[1])
		switch arg {
		case "help", "-help", "--help", "h", "-h", "?", "-?":
			return "", "", true, false
		}
		message = args[1]
	}
	if len(args) > 2 {
		arg := args[2]
		// Don't assign -release or --release as tag
		if arg != "-release" && arg != "--release" {
			tag = arg
		}
	}
	// Scan all args for -release or --release flag
	for _, arg := range args[1:] {
		if arg == "-release" || arg == "--release" {
			isRelease = true
			break
		}
	}
	return
}

// ParseCodeJobArgs parses codejob CLI: codejob [message] [tag] [--reset-gh-token]
// Returns message, tag, isHelp, isRelease, and isResetGHToken.
func ParseCodeJobArgs(args []string) (message, tag string, isHelp, isRelease, isResetGHToken bool) {
	message, tag, isHelp, isRelease = ParseCLIArgs(args)

	for _, arg := range args[1:] {
		if arg == "--reset-gh-token" {
			isResetGHToken = true
			break
		}
	}
	return
}

// ParseReleaseArgs parses gorelease CLI: gorelease [tag]
// Returns tag (may be empty) and isHelp.
func ParseReleaseArgs(args []string) (tag string, isHelp bool) {
	if len(args) > 1 {
		arg := strings.ToLower(args[1])
		switch arg {
		case "help", "-help", "--help", "h", "-h", "?", "-?":
			return "", true
		}
		tag = args[1]
	}
	return
}
