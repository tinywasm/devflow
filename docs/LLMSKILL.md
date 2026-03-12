# llmskill - LLM Configuration Sync

Synchronizes LLM configuration files and installs modular Agent Skills.

## Installation

```bash
go install github.com/tinywasm/devflow/cmd/llmskill@latest
```

## Usage

### Sync all installed LLMs and update skills
```bash
llmskill
```

### Sync specific LLM
```bash
llmskill -l claude
llmskill --llm gemini
```

### Force refresh (reinstalls all skills)
```bash
llmskill -f
llmskill --force
```

## How it works

1. **Installs Modular Skills**: Copies embedded Agent Skills to `~/skills/`.
2. **Detects installed LLMs**: Checks for `~/.claude/` and `~/.gemini/` directories.
3. **Creates Symlinks**: Creates a symlink from `~/.claude/skills/` (and others) to the shared `~/skills/` directory.
   This allows LLMs to natively discover and use all domain-specific skills without any text-based configuration.

## Supported Skills

The skills are divided by domain to minimize context usage:

- `core-principles`: SRP, DI, Framework-less development.
- `testing`: gotest, gopush, WASM dual testing.
- `documentation`: doc standards, diagrams, readme indexing.
- `wasm`: tinywasm MCP, frontend Go compatibility.
- `codejob-agent-workflow`: PLAN.md orchestrator, stage-driven execution.
- `dev-protocols`: Language rules, justification, Claude plan mirroring.

## Library Usage

```go
import "github.com/tinywasm/devflow"

llm := devflow.NewLLM()

// Sync all and install skills
summary, err := llm.Sync("", false)
if err != nil {
    log.Fatal(err)
}
fmt.Println(summary)
```

## Troubleshooting

### "No LLMs detected"
Skills are installed in `~/skills/` even if no specific LLM config is found.

### Skills not appearing in LLM
Ensure the LLM directory exists (`~/.claude` or `~/.gemini`). If symlinks are broken, run `llmskill -f` to force a refresh.

## See Also

- [gotest](GOTEST.md) - Run tests with coverage and badges
- [gopush](GOPUSH.md) - Complete workflow: test + push + update
