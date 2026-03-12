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

1. **Installs Modular Skills**: Copies embedded Agent Skills to `~/tinywasm/skills/`.
2. **Detects installed LLMs**: Checks for `~/.claude/CLAUDE.md` and `~/.gemini/GEMINI.md`.
3. **Adds Reference Line**: Ensures your LLM config file contains:
   `Skills location: ~/tinywasm/skills/`
   This allows the LLM to discover and use all domain-specific skills.

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
Skills are installed in `~/tinywasm/skills/` even if no specific LLM config is found.

### Reference line not appearing
If you manually removed the reference line, run `llmskill -f` to re-add it.

## See Also

- [gotest](GOTEST.md) - Run tests with coverage and badges
- [gopush](GOPUSH.md) - Complete workflow: test + push + update
