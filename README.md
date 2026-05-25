# DevFlow
<img src="docs/img/badges.svg">
<img src="docs/img/badges.svg">

Complete Go development automation: project init, testing, versioning, updates, and backups. Single-line output optimized for AI agents and terminals.

## Commands

- **[gonew](docs/GONEW.md)** - Initialize new Go projects
- **[gotest](docs/GOTEST.md)** - Run tests, vet, race detection, coverage and badges
- **[gopush](docs/GOPUSH.md)** - Automated publish workflow: test + push + update dependents
- **[gorelease](docs/GORELEASE.md)** - Publish Go module + create GitHub Release with cross-platform binaries
- **[devbackup](docs/DEVBACKUP.md)** - Configure and execute automated backups
- **[badges](docs/BADGES.md)** - Generate SVG badges for README (test status, coverage, etc.)
- **[devllm](docs/LLMSKILL.md)** - Sync LLM configuration files from master template
- **[goinstall](docs/GOINSTALL.md)** - Install all devflow commands at once
- **[codejob](docs/CODEJOB.md)** - Send coding tasks to AI agents (Jules, etc.)

## Configuration

- **[GitHub Auth](docs/GITHUB.md)** - Configure GitHub authentication (OAuth, tokens, multi-account)

## Installation

```bash
# Install all commands at once (includes codejob and all other tools)
go install github.com/tinywasm/devflow/cmd/goinstall@latest && goinstall
```

Or install a single command — see each tool's doc linked above.

## Features

- **Intelligent push** - Auto-pulls with `--rebase` on non-fast-forward rejection
- **Zero config** - Auto-detects tests, project structure, WASM environments
- **Minimal output** - Single-line summaries for terminals and LLMs
- **Smart versioning** - Auto-increments tags, skips duplicates
- **Multi-account** - Switch GitHub orgs easily (cdvelop, veltylabs, tinywasm)
- **Dependency updates** - Auto-updates dependent modules in workspace
- **Full testing** - Combines vet, tests, race detection, and **exact weighted coverage** across all packages

## License

MIT
