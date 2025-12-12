package gitgo

import (
	"fmt"
	"sync"
)

// LogFunc is the logging function type
type LogFunc func(v ...any)

// stats tracks usage statistics
type stats struct {
    mu           sync.Mutex
	bytesWritten int
}

var globalStats = &stats{}

// log is the internal logging function
var log LogFunc = func(v ...any) {
	msg := fmt.Sprint(v...)
    globalStats.mu.Lock()
	globalStats.bytesWritten += len(msg)
    globalStats.mu.Unlock()
	fmt.Println(msg)
}

// SetLogger configures the custom logging function
func SetLogger(fn LogFunc) {
	log = func(v ...any) {
		msg := fmt.Sprint(v...)
        globalStats.mu.Lock()
		globalStats.bytesWritten += len(msg)
        globalStats.mu.Unlock()
		fn(v...)
	}
}

// PrintSummary prints a minimal summary of execution usage
func PrintSummary() {
	// Minimal summary for MPC/LLM context efficiency
    globalStats.mu.Lock()
    bytes := globalStats.bytesWritten
    globalStats.mu.Unlock()
	fmt.Printf("\n--- Summary ---\nOutput size: %d bytes\n", bytes)
}
