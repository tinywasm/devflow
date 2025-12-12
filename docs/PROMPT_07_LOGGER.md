# PROMPT 07: Simple Logger

## Context
Minimalist logger as a function variable. No interfaces, no levels, no complex configuration. Compatible with injection from TUI.

## File: logger.go

```go
package gitgo

import "fmt"

// LogFunc is the logging function type
type LogFunc func(v ...any)

// log is the internal logging function
var log LogFunc = fmt.Println

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
```

## Usage in Code

### Internal logging examples

```go
// In git_operations.go
func GitAdd() error {
    log("git add .")

    cmd := exec.Command("git", "add", ".")
    return cmd.Run()
}

func GitCommit(message string) error {
    log("git commit -m", message)

    cmd := exec.Command("git", "commit", "-m", message)
    return cmd.Run()
}

// In workflow_push.go
func WorkflowPush(message, tag string) error {
    log("Workflow Push:", message)

    // ... operations

    if err := GitAdd(); err != nil {
        log("Error:", err)
        return err
    }

    // ... more operations
}
```

## TUI Integration

### From external application

```go
package main

import (
    "github.com/cdvelop/gitgo"
    "your-tui-framework/logger"
)

func main() {
    // Inject TUI logger
    gitgo.SetLogger(func(v ...any) {
        logger.Info(v...)
    })

    // Use gitgo normally
    gitgo.WorkflowPush("my commit", "")
}
```

### Silent Logger

```go
package main

import "github.com/cdvelop/gitgo"

func main() {
    // No output
    gitgo.SetLogger(func(v ...any) {})

    gitgo.WorkflowPush("quiet commit", "")
}
```

### Logger to file

```go
package main

import (
    "github.com/cdvelop/gitgo"
    "log"
    "os"
)

func main() {
    file, _ := os.Create("gitgo.log")
    defer file.Close()

    fileLogger := log.New(file, "", log.LstdFlags)

    gitgo.SetLogger(func(v ...any) {
        fileLogger.Println(v...)
    })

    gitgo.WorkflowPush("logged commit", "")
}
```

## Logger Testing

### logger_test.go

```go
package gitgo

import (
    "testing"
)

func TestDefaultLogger(t *testing.T) {
    // By default should be fmt.Println
    // (difficult to test without capturing stdout)

    // Only verify that it doesn't panic
    defer func() {
        if r := recover(); r != nil {
            t.Fatal("Default logger panicked")
        }
    }()

    log("test message")
}

func TestSetLogger(t *testing.T) {
    // Capture logs
    var logged []any

    customLog := func(v ...any) {
        logged = append(logged, v...)
    }

    SetLogger(customLog)

    log("test", "message", 123)

    if len(logged) != 3 {
        t.Errorf("Expected 3 logged items, got %d", len(logged))
    }

    if logged[0] != "test" {
        t.Errorf("Expected 'test', got %v", logged[0])
    }

    if logged[1] != "message" {
        t.Errorf("Expected 'message', got %v", logged[1])
    }

    if logged[2] != 123 {
        t.Errorf("Expected 123, got %v", logged[2])
    }
}

func TestSetLoggerNil(t *testing.T) {
    // Silence
    SetLogger(func(v ...any) {})

    // Should not panic
    defer func() {
        if r := recover(); r != nil {
            t.Fatal("Silent logger panicked")
        }
    }()

    log("this should be silent")
}

func TestLoggerConcurrency(t *testing.T) {
    // Basic concurrency test
    done := make(chan bool)

    SetLogger(func(v ...any) {
        // Concurrent logging
    })

    for i := 0; i < 10; i++ {
        go func(n int) {
            log("concurrent", n)
            done <- true
        }(i)
    }

    for i := 0; i < 10; i++ {
        <-done
    }
}
```

## Features

### ✅ Advantages
- Simple (3 lines of code)
- No dependencies
- Injectable from TUI
- Configurable at runtime
- Zero overhead when silent

### ❌ Does Not Include
- Levels (Debug, Info, Error)
- Automatic formatting
- Timestamps
- Colors
- Multiple outputs

### 💡 Decision
Minimalist logger is sufficient because:
1. Real output is handled by TUI
2. We don't need complex logs
3. CLI binaries are ephemeral
4. Clean integration with external systems

## Usage in Binaries

### cmd_push.go
```go
package main

import (
    "flag"
    "github.com/cdvelop/gitgo"
    "log"
)

func main() {
    flag.Parse()

    args := flag.Args()

    // Simple logger for CLI
    gitgo.SetLogger(func(v ...any) {
        log.Println(v...)
    })

    // ... rest of code
}
```

### cmd_gopu.go
```go
package main

import (
    "flag"
    "github.com/cdvelop/gitgo"
    "log"
)

func main() {
    // ... flags

    // Logger with prefix
    gitgo.SetLogger(func(v ...any) {
        log.Println("[gopu]", v)
    })

    // ... rest
}
```

## Summary

- **Single file**: logger.go
- **Lines of code**: ~15
- **Dependencies**: fmt (stdlib)
- **Injectable**: Yes (SetLogger)
- **Thread-safe**: No (unnecessary)
- **Overhead**: Zero when silent

Simple, effective, integrable.
