package devflow

import "strings"

// ParseCLIArgs parses command line arguments for devflow tools.
// It returns the message, tag, and whether help was requested.
func ParseCLIArgs(args []string) (message, tag string, isHelp bool) {
	if len(args) > 1 {
		arg := strings.ToLower(args[1])
		switch arg {
		case "help", "-help", "--help", "h", "-h", "?", "-?":
			return "", "", true
		}
		message = args[1]
	}
	if len(args) > 2 {
		tag = args[2]
	}
	return
}
