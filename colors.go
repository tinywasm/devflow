package devflow

import "fmt"

const (
	ColorRed    = "\033[0;31m"
	ColorGreen  = "\033[0;32m"
	ColorYellow = "\033[0;33m"
	ColorCyan   = "\033[0;36m"
	ColorNone   = "\033[0m"
)

// PrintSuccess prints a success message in green.
func PrintSuccess(msg string) {
	fmt.Printf("%s%s%s\n", ColorGreen, msg, ColorNone)
}

// PrintWarning prints a warning message in yellow.
func PrintWarning(msg string) {
	fmt.Printf("%s%s%s\n", ColorYellow, msg, ColorNone)
}

// PrintError prints an error message in red.
func PrintError(msg string) {
	fmt.Printf("%sError: %s%s\n", ColorRed, msg, ColorNone)
}

// PrintInfo prints an informational message in cyan.
func PrintInfo(msg string) {
	fmt.Printf("%s%s%s\n", ColorCyan, msg, ColorNone)
}
