package devflow

import "github.com/tinywasm/command"

// GitHubClient defines the interface for GitHub operations.
// This allows mocking the GitHub dependency in tests.
type GitHubClient interface {
	SetLog(fn func(...any))
	GetCurrentUser() (string, error)
	RepoExists(owner, name string) (bool, error)
	CreateRepo(owner, name, description, visibility string) error
	DeleteRepo(owner, name string) error
	IsNetworkError(err error) bool
	GetHelpfulErrorMessage(err error) string
	CreateRelease(tag string, assets []string, targetRepo string) (string, error)
}

// GitHubAuthenticator defines the interface for GitHub authentication.
// This allows mocking authentication in tests.
type GitHubAuthenticator interface {
	EnsureGitHubAuth() error
	SetLog(fn func(...any))
}

// GitHubAuthHandler defines the interface for GitHub auth as a TUI handler.
type GitHubAuthHandler interface {
	GitHubAuthenticator
	Name() string
}

// GitClient defines the interface for Git operations.
type GitClient interface {
	CheckRemoteAccess() error
	Push(message, tag string) (PushResult, error)
	GetLatestTag() (string, error)
	SetLog(fn func(...any))
	SetShouldWrite(fn func() bool)
	SetRootDir(path string)
	GitIgnoreAdd(entry string) error
	GetConfigUserName() (string, error)
	GetConfigUserEmail() (string, error)
	InitRepo(dir string) error
	Add() error
	Commit(message string) (bool, error)
	CommitPaths(message string, paths ...string) (bool, error)
	CreateTag(tag string) (bool, error)
	PushWithTags(tag string) (bool, error)
	PushWithoutTags() (bool, error)
	HasPendingChanges() (bool, error)
	StatusPorcelain() (string, error)
	DiffShortStat() (string, error)
	GenerateNextTag() (string, error)
}

// FolderWatcher defines interface for adding/removing directories to watch
type FolderWatcher interface {
	AddDirectoriesToWatch(paths ...string) error
	RemoveDirectoriesFromWatcher(paths ...string) error
}

// Publisher defines the interface for publishing code changes.
type Publisher interface {
	Publish(message, tag string, skipTests, skipRace, skipDependents, skipBackup, skipTag, skipVerify bool) (PushResult, error)
}

// CodeJobDriver defines the contract for an external AI coding agent.
// Implementations: JulesDriver, (future: OllamaDriver, etc.)
// title is the human-readable job name (e.g. "owner/repo"), derived by CodeJob.
type CodeJobDriver interface {
	Name() string
	SetLog(fn func(...any))
	Send(prompt, title string) (string, error)
}

// SessionProvider is implemented by CodeJobDrivers that return a session ID
// after a successful Send(). CodeJob uses this to persist to .env.
type SessionProvider interface {
	SessionID() string
}

// GoModInterface defines interface for go.mod handling
type GoModInterface interface {
	NewFileEvent(fileName, extension, filePath, event string) error
	SetFolderWatcher(watcher FolderWatcher)
	Name() string
	SupportedExtensions() []string
	MainInputFileRelativePath() string
	UnobservedFiles() []string
	SetLog(fn func(...any))
	SetRootDir(path string)
	GetReplacePaths() ([]ReplaceEntry, error)
}

// BackupRunner defines the interface for backup operations.
// Allows mocking in tests to prevent real backup execution.
type BackupRunner interface {
	SetLog(fn func(...any))
	SetCommand(command string) error
	GetCommand() (string, error)
	Run() (string, error)
}

// Runner abstracts command execution (git, gh, etc.) for testing.
type Runner interface {
	Run(name string, args ...string) (string, error)
}

// RealRunner runs actual system commands.
type RealRunner struct{}

// Run executes the command using the command package.
func (RealRunner) Run(name string, args ...string) (string, error) {
	return command.Run(name, args...)
}
