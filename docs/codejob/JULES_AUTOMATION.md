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

State lives in the `docs/PLAN.md` frontmatter (not `.env`), so each step is a
commit and the loop can also run in GitHub Actions.

1. **Dispatch**: `codejob` sends `docs/PLAN.md` to the `EXECUTOR` (Jules) and
   writes `STATUS: running` + `SESSION` to the frontmatter.
2. **Pull Request**: When Jules is done, a PR is created on GitHub.
3. **Review**: if a `REVIEWER` is set, it posts a native review on the PR;
   otherwise `STATUS: review` awaits you.
4. **Merge**: you merge the PR (locally or from web) → `gopush` publishes
   (tag-only) and `docs/PLAN.md` is deleted.

See [CODEJOB.md](../CODEJOB.md) for the full state model and the cloud setup.

## See Also

- [CodeJob](CODEJOB.md) - General orchestrator documentation
- [gopush](GOPUSH.md) - Universal publish workflow
