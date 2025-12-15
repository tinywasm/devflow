# GoNew Command Specification

## Overview
Convert bash workflow `gonewproject.sh` to Go implementation as `cmd/gonew` following devflow architecture patterns.

## Core Requirements

### Command Execution
**All external command execution MUST use `executor.go` utilities:**
- `RunCommand(name, args...)` - Execute commands with error handling
- `RunCommandSilent(name, args...)` - Execute without output logging
- `RunShellCommand(command)` - Cross-platform shell command execution

Examples:
- Git commands: `RunCommand("git", "init")`
- Go commands: `RunCommand("go", "mod", "init", modulePath)`
- GitHub CLI: `RunCommand("gh", "api", "user", "--jq", ".login")`

### Workflow
1. **Validate inputs** - Validate repo name (strict rules, error if invalid), validate description (required, max 350 chars)
2. **Check availability** - Both local directory and remote repository must be available
3. **Create remote** - GitHub repository via gh CLI (fallback to local-only if unavailable)
4. **Initialize local** - Clone (if remote) OR git init + files (if local-only)
5. **Generate files** - README, LICENSE (MIT), .gitignore, {repo}.go, go.mod
6. **Initial commit** - Commit all files (both remote and local-only modes)
7. **Tag creation** - Use `Git.GenerateNextTag()` (returns v0.0.1 for new repos)
8. **Push** - Push commits and tag (if remote available)
9. **Output** - Single-line summary: `✅ Created: my-repo [local+remote] v0.0.1`

### File Generation Templates
- **README.md**: `# {repo-name}\n\n{description}`
- **LICENSE**: MIT license with git config user.name and current year
- **.gitignore**: Go-specific (binaries, test artifacts, coverage files)
- **{repo}.go**: Basic struct + New() constructor (bash script pattern)
- **go.mod**: `module github.com/{username}/{repo-name}`

### Naming Conventions
- **Repo name**: Strict validation (alphanumeric, dashes, underscores only - error if invalid)
- **File name**: `{repo-name}.go` (use validated repo name)
- **Struct name**: CamelCase conversion (e.g., `my-repo` → `type MyRepo struct{}`)
- **Variable**: First letter lowercase of struct name (e.g., `m := &MyRepo{}`)
- **Main branch**: `main`

### Error Handling & Fallback
- **Both unavailable**: Abort with clear error
- **Remote unavailable** (network, gh CLI issues): Create local-only with helpful message
- **Git config missing**: Fail with error, suggest `git config --global user.name "Name"`
- **GitHub errors**: Provide actionable guidance (install gh, run `gh auth login`, etc.)

## Architecture

### New Components

#### 1. GitHub Handler (`github.go`)
```go
type GitHub struct {
    log func(...any)
}

// NewGitHub creates handler and verifies gh CLI availability
func NewGitHub() (*GitHub, error) // Returns error if gh not installed or not authenticated

// Core methods
func (gh *GitHub) GetCurrentUser() (string, error)
func (gh *GitHub) RepoExists(owner, name string) (bool, error)
func (gh *GitHub) CreateRepo(name, description, visibility string) error
func (gh *GitHub) IsNetworkError(err error) bool
func (gh *GitHub) GetHelpfulErrorMessage(err error) string
func (gh *GitHub) SetLog(fn func(...any))
```

#### 2. Project Templates (`project_templates.go`)
```go
// File generation functions
func GenerateREADME(repoName, description, targetDir string) error
func GenerateLicense(ownerName, targetDir string) error // MIT only
func GenerateGitignore(targetDir string) error
func GenerateHandlerFile(packageName, targetDir string) error
func ValidateRepoName(name string) error // Returns error if invalid chars (only alphanumeric, dash, underscore allowed)
func ValidateDescription(desc string) error
```

#### 3. GoNew Orchestrator (`gonew.go`)
```go
type GoNew struct {
    git    *Git
    github *GitHub
    goH    *Go  // 'go' is reserved keyword
    log    func(...any)
}

type NewProjectOptions struct {
    Name        string // Required, must be valid (alphanumeric, dash, underscore only)
    Description string // Required, max 350 chars
    Visibility  string // "public" or "private" (default: "public")
    Directory   string // Supports ~/path, ./path, /abs/path (default: ./{Name})
}

// NewGoNew creates orchestrator (all handlers must be initialized)
func NewGoNew(git *Git, github *GitHub, goHandler *Go) *GoNew

// Create executes full workflow with remote (or local-only fallback)
func (gn *GoNew) Create(opts NewProjectOptions) (string, error)

// AddRemote adds GitHub remote to existing local project
// Validates project structure (go.mod, git repo), reads description from README.md
// If remote already configured, returns info message without changes
func (gn *GoNew) AddRemote(projectPath, visibility string) (string, error)

func (gn *GoNew) SetLog(fn func(...any))
```

### Modified Components

#### `git_handler.go` (Refactor + Add methods)
```go
// NewGit verifies git installation and returns error if not available
func NewGit() (*Git, error) // REFACTOR: Change signature from NewGit() *Git

// Configuration management
func (g *Git) GetConfigUserName() (string, error)
func (g *Git) GetConfigUserEmail() (string, error)
func (g *Git) SetUserConfig(name, email string) error // Public for programmatic use

// Repository initialization (for local-only mode)
func (g *Git) InitRepo(dir string) error // git init + set branch main
```

#### `go_mod.go` (Refactor + Add methods)
```go
// NewGo verifies Go installation and returns error if not available
func NewGo(gitHandler *Git) (*Go, error) // REFACTOR: Change signature from NewGo(*Git) *Go

// Module initialization
func (g *Go) ModInit(modulePath, targetDir string) error
func (g *Go) DetectGoExecutable() (string, error)
```

## Local-Only Mode Workflow

When remote creation fails (network, gh unavailable, etc.):

1. Create target directory
2. `git init` + set branch to `main`
3. Generate all files (README, LICENSE, .gitignore, handler.go, go.mod)
4. `git add .`
5. `git commit -m "Initial commit"`
6. `git tag v0.0.1`
7. Output: `⚠️ Created: my-repo [local only] v0.0.1 - gh unavailable`

User can later add remote:
```bash
gonew add-remote my-repo [--visibility=public]
```

## CLI Design

### Main Command
```bash
# Create new project
gonew <repo-name> <description> [flags]

# Examples
gonew my-project "A sample Go project"
gonew my-lib "Go library" --visibility=private
gonew ~/Dev/my-tool "CLI tool" --local-only

# Add remote to existing local project
gonew add-remote <project-path> [--visibility=public]
```

### Flags
- `--visibility=public|private` (default: public)
- `--local-only` Skip remote creation entirely
- `--license=MIT` (default: MIT, future: more types)

### Output Examples
- Success: `✅ Created: my-repo [local+remote] v0.0.1`
- Local only: `⚠️ Created: my-repo [local only] v0.0.1 - run 'gonew add-remote' when ready`
- Error: `❌ Failed: directory ./my-repo already exists`

## Implementation Phases

### Phase 1: Core Handlers Refactoring & Extension
- **REFACTOR** `NewGit()` to return `(*Git, error)` with git installation verification
- **REFACTOR** `NewGo()` to return `(*Go, error)` with Go installation verification
- Implement `GitHub` handler with `NewGitHub() (*GitHub, error)` verification
- Add `ModInit`, `DetectGoExecutable` to `go_mod.go`
- Add `InitRepo`, `SetUserConfig`, config getters to `git_handler.go`
- Add `SetLog()` to all handlers for debugging support

### Phase 2: Templates & Orchestration
- Implement `project_templates.go` with all generators (ValidateRepoName, CamelCase conversion)
- Create `GoNew` orchestrator with full workflow
- Implement conflict checking (local + remote)
- Implement fallback logic (local-only mode)
- Implement `AddRemote` with validation and README parsing

### Phase 3: CLI
- Create `cmd/gonew/main.go`
- Parse arguments and flags
- Path expansion (~, relative, absolute)
- Implement `add-remote` subcommand with validation

### Phase 4: Testing
- Unit tests for each handler (with mocks for GitHub)
- Integration tests (local-only mode)
- Path handling tests (various formats)
- Error handling tests
- CamelCase conversion tests for various repo names

## Key Decisions Summary

| Topic | Decision |
|-------|----------|
| License | MIT default, extensible via parameter |
| Handler template | Minimal: struct + New() only, CamelCase naming |
| VCS provider | GitHub default, interface-based for extensibility |
| Tag generation | Reuse `Git.GenerateNextTag()` |
| Remote creation | Optional, graceful fallback to local-only |
| Directory conflicts | Abort if local OR remote exists |
| Git config | Required (user.name, user.email), fail if missing |
| Main branch | `main` |
| Output | Single-line summary only (devflow style) |
| Dependencies | All handlers verify in New(), return (*Handler, error) - **REFACTOR REQUIRED** |
| Logging | All handlers support SetLog() for debugging |
| .gitignore | Auto-generated for Go projects |
| Description | Required, max 350 chars (GitHub limit) |
| Repo name validation | Strict: alphanumeric, dash, underscore only - error if invalid |
| Struct naming | CamelCase conversion (my-repo → MyRepo) |
| Variable naming | First letter lowercase of struct (MyRepo → m) |
| Add-remote | Validates project, reads README, skip if remote exists |

## Success Criteria

1. ✅ Creates complete Go project (local + remote or local-only)
2. ✅ Generates: README, LICENSE, .gitignore, handler.go, go.mod
3. ✅ Initial commit + tag v0.0.1 in both modes
4. ✅ Single-line output matching devflow style
5. ✅ Repository name strict validation (error on invalid chars)
6. ✅ Graceful fallback when remote unavailable
7. ✅ Helpful error messages with actionable guidance
8. ✅ Cross-platform support (Windows, Linux, macOS)
9. ✅ Path flexibility (~/path, ./path, /abs/path)
10. ✅ `add-remote` subcommand for upgrading local projects
11. ✅ All handlers verify dependencies on creation
12. ✅ Unit test coverage >80%

## References

### Bash Scripts (Source)
All referenced bash scripts are available in `docs/scripts-ref/` for implementation reference:

- **Main workflow**: [gonewproject.sh](scripts-ref/gonewproject.sh) - Main entry point
- **Remote creation**: [repocreate.sh](scripts-ref/repocreate.sh) - GitHub repo creation via gh CLI
- **Go initialization**: [gomodinit.sh](scripts-ref/gomodinit.sh) - go mod init + handler file generation
- **Repo setup**: [repoexistingsetup.sh](scripts-ref/repoexistingsetup.sh) - Tag creation
- **Utilities**: [functions.sh](scripts-ref/functions.sh), [gitutils.sh](scripts-ref/gitutils.sh), [githubutils.sh](scripts-ref/githubutils.sh), [licensecreate.sh](scripts-ref/licensecreate.sh)

### Go Patterns
- Handlers: `git_handler.go`, `go_handler.go`
- **Command Execution**: `executor.go` - **REQUIRED** for all external commands (git, go, gh, etc.)

### Script Flow (gonewproject.sh)
