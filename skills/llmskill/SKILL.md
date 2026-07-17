---
name: llmskill
description: Sync skills from tinywasm/devflow/skills to all installed LLM agents (Claude, Gemini). Run after creating or modifying any SKILL.md file in devflow/skills/.
---

# llmskill

`llmskill` is a CLI tool in `tinywasm/devflow` that synchronizes skill files from the devflow repository to all installed LLM agent configurations.

## When to Run

After creating or modifying any `SKILL.md` in `tinywasm/devflow/skills/<name>/`:

```bash
cd tinywasm/devflow && go install ./cmd/llmskill && llmskill -f
```

**The rebuild step is mandatory**: skills are EMBEDDED in the binary at
compile time (`//go:embed skills` in `llm_skill.go`) — `llmskill` does NOT
read the working directory. Running the old binary reinstalls the OLD
embedded skills and still prints "Skills updated". Always verify the change
landed:

```bash
grep -c "<some new phrase>" ~/.claude/skills/<name>/SKILL.md
```

## How It Works

1. Skills live in `tinywasm/devflow/skills/` (source of truth) and are
   embedded into the `llmskill` binary when it is built.
2. `llmskill` installs the embedded skills to `~/skills/`.
3. It then symlinks `~/skills/` from each detected LLM config dir
   (`~/.claude/skills/`, `~/.gemini/skills/`); if the target already exists
   as a real directory, it falls back to copying into it.

## Installation

`llmskill` is part of `github.com/tinywasm/devflow`.

**Step 1 — Install the binary:**

```bash
go install github.com/tinywasm/devflow/cmd/llmskill@latest
```

Or install all devflow binaries at once (requires the repo to be cloned first):

```bash
go install github.com/tinywasm/devflow/cmd/goinstall@latest && goinstall
```

**Step 2 — Clone the devflow repo** (required to EDIT skills — the binary
carries an embedded copy from build time):

```bash
git clone https://github.com/tinywasm/devflow
cd devflow
llmskill
```

To pick up local skill edits: `git pull` (or edit) + `go install ./cmd/llmskill` + `llmskill -f`.

## Usage

```bash
# Sync all installed LLMs
llmskill

# Sync only Claude
llmskill -l claude

# Force overwrite with backup
llmskill -f
```

## Skill File Format

Each skill is a folder with a single `SKILL.md`:

```
tinywasm/devflow/skills/
└── myskill/
    └── SKILL.md
```

`SKILL.md` frontmatter:
```markdown
---
name: myskill
description: One-line description used by the agent to decide when to apply this skill.
---

# Skill content here
```

## After Creating a New Skill

1. Write `SKILL.md` in `tinywasm/devflow/skills/<skillname>/`
2. Run `llmskill` from the shell
3. The skill is immediately active in all installed agents

## Important

- `llmskill` is a **local developer tool** — never include it in `PLAN.md` files sent to external agents.
- Skills are read-only context for agents — agents do not run `llmskill` themselves.
