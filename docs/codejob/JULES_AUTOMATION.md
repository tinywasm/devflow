# Jules Skill

Configure and use the Jules AI agent driver for CodeJob.

## Overview

The Jules driver connects CodeJob to the Jules API, allowing you to dispatch tasks described in `docs/PLAN.md` to an automated AI software engineer.

## Requirements

- **Jules API Key**: Get one at [jules.google.com](https://jules.google.com).
- **GitHub CLI**: `gh` must be installed and authenticated.

## Automatic Setup

The first time you run `codejob`, it will automatically prompt you for your API key if it's not found in your system keyring.

```bash
codejob
# Jules API Key not found. Get yours at https://jules.google.com/settings/api
# Enter it now: ***********
```

The key is stored securely using the system keyring service (Keychain on macOS, Secret Service on Linux, Credential Locker on Windows).

## Manual Initialization

If you want to initialize the key before dispatching your first task, you can use the internal wizard:

```bash
codejob
```

Wait, `codejob init` was removed. Just run `codejob` and it will prompt you if needed.

## Configuration Fields

If using the library directly, you can provide explicit configuration:

```go
cfg := devflow.JulesConfig{
    APIKey:      "optional-override-key",
    SourceID:    "sources/github/owner/repo",
    StartBranch: "main",
}
driver := devflow.NewJulesDriver(cfg)
```

- **APIKey**: Your Jules API key. Defaults to the one stored in the keyring.
- **SourceID**: The Jules source identifier. Auto-detected via `gh repo view`.
- **StartBranch**: The branch Jules should start from. Auto-detected via `git branch`.

## Workflow

1. **Dispatch**: `codejob` sends `docs/PLAN.md` to Jules.
2. **Persistence**: A session ID is saved to `.env` as `CODEJOB=jules:ID`.
3. **Poll**: Subsequent runs of `codejob` check the session status.
4. **Pull Request**: When Jules is done, a PR is created on GitHub.
5. **Review**: `codejob` fetches the new branch and renames your plan to `docs/CHECK_PLAN.md`.
6. **Merge**: After reviewing, run `codejob 'message'` to merge and publish.

## See Also

- [CodeJob](CODEJOB.md) - General orchestrator documentation
- [gopush](GOPUSH.md) - Universal publish workflow
