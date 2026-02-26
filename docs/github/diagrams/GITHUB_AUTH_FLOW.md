# GitHub Authorization Flow in Devflow

This diagram illustrates how the `devflow` authentication system affects Git operations and why access errors occur in repositories belonging to unauthorized organizations, and how the auto-recovery loop handles them.

```mermaid
flowchart TD
    %% Initialization
    A[User executes push / gopush] --> B["devflow/github_auth.go <br/> (Requests Device Code)"]
    B --> C["Client ID: Ov23lij... <br/> (GitHub API)"]
    C --> D["Returns User Code <br/> (User opens browser)"]

    %% Authorization on GitHub
    D --> E{"User Authorizes devflow App"}
    E -->|If fully or partially authorized| F["Token Generated"]

    %% Local Configuration
    F --> G["EnsureGitHubAuth() <br/> (Saves Token to Keyring)"]
    G --> H["gh auth login --with-token"]
    H --> I["(gh CLI configured as <br/> credential helper for git)"]

    %% Push Error in Restricted Repo
    I --> J["User executes push <br/> (in org_restricted/repo)"]
    J --> K["devflow/git_handler.go <br/> CheckRemoteAccess()"]
    K --> L["Executes git ls-remote origin"]
    L --> M["git uses gh as helper <br/> (Sends devflow Token)"]
    M --> N{"GitHub verifies Token"}

    %% Final Scenarios
    N -->|Token WITHOUT permissions for org_restricted| O["Rejects Connection <br/> (git ls-remote fails)"]
    O --> P["devflow captures access error"]
    P --> Q{"Is it an auth error?"}
    Q -->|Yes — authRetrier != nil| R["Starts Auth flow again <br/> (Back to start)"]
    Q -->|No e.g. Network failure| S["devflow shows original error"]
    R -.-> B

    N -->|Token WITH permissions for org_permitted| T["Accepts Connection <br/> (Successful Push)"]
```

## Problem Analysis

The problem lies in the fact that the OAuth token obtained via `github_auth.go` (Device Flow) is used globally through the `gh` CLI (which acts as a `credential.helper` for git).
When the user authorizes the `devflow` OAuth application in the browser, it is very likely that explicit access to the corresponding organization has not been granted (often because it requires administrator approval).

Since `git` uses this token for any HTTPS/SSH operation towards GitHub, executing `git ls-remote origin` in a repository where the organization has not granted permissions results in GitHub denying access.

## Implementation

The auto-recovery is implemented via Dependency Injection in `git_handler.go`:

1. **`Git` struct** holds an optional `authRetrier GitHubAuthenticator` field.
2. **`SetAuthRetrier(a GitHubAuthenticator)`** is called by `cmd/push` and `cmd/gopush` at startup, injecting `NewGitHubAuth()`.
3. **`CheckRemoteAccess()`** — on detecting `"Authentication failed"` or `"Could not read from remote repository"`:
   - Calls `authRetrier.EnsureGitHubAuth()` (triggers Device Flow — browser opens).
   - Retries `git ls-remote origin` once.
   - If retry succeeds → continues push transparently.
   - If retry fails → returns the raw error without noise.

During re-authentication, the terminal shows: `🔑 Access denied. Restarting authentication...` followed by the Device Flow prompt, guiding the developer to approve organization access in the browser before continuing.
