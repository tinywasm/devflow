# GitGo - Automated Git Workflows for Go Projects

[![Go Version](https://img.shields.io/badge/Go-1.20+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

GitGo is a minimalist Go library that automates Git workflows for Go projects. It reimplements bash scripts like `pu.sh` and `gopu.sh` in a testable and reusable way.

## 🚀 Features

- **🔧 Installable CLIs**: `push` and `gopu` commands via `go install`
- **📦 Reusable Library**: Import and use in your own projects
- **🪵 Injectable Logger**: SetLogger for TUI integration
- **✅ ~40% Testable**: Basic coverage on critical functionality
- **🏷️ Automatic Tags**: Automatic semantic tag generation
- **🧪 Automated Testing**: Tests and race detection for Go projects
- **🚫 No Dependencies**: Only stdlib, no external dependencies

## 📦 Installation

```bash
# Install both commands
go install github.com/cdvelop/gitgo/cmd_push@latest
go install github.com/cdvelop/gitgo/cmd_gopu@latest

# Or use as a library
go get github.com/cdvelop/gitgo
```

## 🎯 Quick Usage

### `push` Command

Complete Git workflow: add → commit → tag → push

```bash
# Basic usage (message + auto tag)
push "feat: implement new feature"

# With specific tag
push "release: version 1.0.0" v1.0.0

# Commit only (no tag)
push "docs: update README"
```

### `gopu` Command

Specialized workflow for Go projects: verify → test → push → update dependencies

```bash
# Complete workflow
gopu "feat: new feature"

# Without tests (faster)
gopu --skip-tests "docs: update README"

# Without race detector
gopu --skip-race "refactor: cleanup"

# Without updating dependents
gopu --skip-update "fix: bug"

# With specific tag
gopu "release: major version" v2.0.0

# Search dependents without updating
gopu --search
```

## 📚 Usage as Library

```go
package main

import (
    "github.com/cdvelop/gitgo"
)

func main() {
    // Inject custom logger (optional)
    gitgo.SetLogger(func(v ...any) {
        // Custom logging
    })
    
    // Execute push workflow
    err := gitgo.WorkflowPush("feat: new feature", "")
    if err != nil {
        panic(err)
    }
}
```

### TUI Integration

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
    
    gitgo.WorkflowPush("my commit", "")
}
```

## 🗂️ Project Structure

```
gitgo/
├── cmd_push.go           # CLI push
├── cmd_gopu.go           # CLI gopu
├── logger.go             # Simple injectable logger
├── git_operations.go     # Git operations
├── go_operations.go      # Go operations
├── go_mod_update.go      # Dependents update
├── workflow_push.go      # Push workflow
├── workflow_gopu.go      # GoPU workflow
├── tag.go                # Tag logic (already existing)
├── *_test.go             # Tests (~40% coverage)
└── docs/
    ├── PROMPT_01_ARCHITECTURE.md
    ├── PROMPT_02_GIT_OPERATIONS.md
    ├── PROMPT_03_GO_OPERATIONS.md
    ├── PROMPT_04_PUSH_CMD.md
    ├── PROMPT_05_GOPU_CMD.md
    ├── PROMPT_06_TESTING.md
    └── PROMPT_07_LOGGER.md
```

## 📖 Detailed Documentation

Complete implementation documentation is in `docs/`:

1. **[PROMPT_01_ARCHITECTURE.md](docs/PROMPT_01_ARCHITECTURE.md)** - Architecture and design decisions
2. **[PROMPT_02_GIT_OPERATIONS.md](docs/PROMPT_02_GIT_OPERATIONS.md)** - Git operations
3. **[PROMPT_03_GO_OPERATIONS.md](docs/PROMPT_03_GO_OPERATIONS.md)** - Go operations and dependency updates
4. **[PROMPT_04_PUSH_CMD.md](docs/PROMPT_04_PUSH_CMD.md)** - Push command
5. **[PROMPT_05_GOPU_CMD.md](docs/PROMPT_05_GOPU_CMD.md)** - GoPU command
6. **[PROMPT_06_TESTING.md](docs/PROMPT_06_TESTING.md)** - Basic testing strategy
7. **[PROMPT_07_LOGGER.md](docs/PROMPT_07_LOGGER.md)** - Injectable logger

## 🔧 Public API

### Git Operations

```go
func GitAdd() error
func GitCommit(message string) error
func GitPush() error
func GitPushWithTags() error
func GitGenerateNextTag() (string, error)
func GitCreateTag(tag, message string) error
func GitHasChanges() (bool, error)
func GitHasUncommittedChanges() (bool, error)
func GitGetCurrentBranch() (string, error)
func GitGetLastCommit() (string, error)
func GitHasRemote() (bool, error)
func GitGetRemoteURL() (string, error)
func GitFetchTags() error
```

### Go Operations

```go
func GoTest() error
func GoTestRace() error
func GoModVerify() error
func GoModTidy() error
func GoGet(pkg string) error
func GoGetModulePath() (string, error)
func GoGetModuleName() (string, error)
func GoUpdateDependents(searchDir string) error
```

### Workflows

```go
func WorkflowPush(message, tag string) error
func WorkflowGoPU(message, tag string, skipTests, skipRace, skipUpdate bool) error
```

### Logger

```go
func SetLogger(fn LogFunc)
```

## 🔧 Development

### Build

```bash
# Local build
go build -o push cmd_push.go
go build -o gopu cmd_gopu.go

# Tests
go test -v

# Tests with coverage
go test -cover

# Tests with race detector
go test -race -v
```

## 🤝 Contributing

1. Fork the project
2. Create feature branch (`git checkout -b feature/amazing`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing`)
5. Open a Pull Request

## 📝 Conventions

- Commits following [Conventional Commits](https://www.conventionalcommits.org/)
- Tests for critical functionality (~40% coverage)
- GoDoc for public functions
- No external dependencies

## 📄 License

MIT License. See [LICENSE](LICENSE) for details.

## 🙏 Acknowledgments

Based on original bash scripts from [devscripts](https://github.com/cdvelop/devscripts)

## 🔗 Links

- [Implementation Documentation](docs/)

---

**Made to automate Git and Go workflows with simplicity**
