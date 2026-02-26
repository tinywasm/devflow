package devflow

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
	CreateTag(tag string) (bool, error)
	PushWithTags(tag string) (bool, error)
	HasPendingChanges() (bool, error)
}

// FolderWatcher defines interface for adding/removing directories to watch
type FolderWatcher interface {
	AddDirectoriesToWatch(paths ...string) error
	RemoveDirectoriesFromWatcher(paths ...string) error
}

// RepoSync checks whether the local repository is in sync with the remote.
// CodeJob uses this to refuse dispatch when local changes haven't been pushed yet.
type RepoSync interface {
	HasPendingChanges() (bool, error)
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
