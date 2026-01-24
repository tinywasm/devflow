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
	Push(message, tag string) (string, error)
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
	PushWithTags(tag string) error
}

// FolderWatcher defines interface for adding/removing directories to watch
type FolderWatcher interface {
	AddDirectoryToWatcher(path string) error
	RemoveDirectoryFromWatcher(path string) error
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
}
