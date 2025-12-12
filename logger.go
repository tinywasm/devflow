package gitgo

import "fmt"

// LogFunc is the logging function type
type LogFunc func(v ...any)

// log is the internal logging function
var log LogFunc = func(v ...any) {
	fmt.Println(v...)
}

// SetLogger configures the custom logging function
// Useful for TUI integration or silent logging
//
// Example:
//   gitgo.SetLogger(func(v ...any) {
//       // Custom logging
//   })
//
//   gitgo.SetLogger(func(v ...any) {}) // Silence
func SetLogger(fn LogFunc) {
	log = fn
}
