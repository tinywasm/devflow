---
name: agent-workflow
description: Multi-agent planning workflow with PLAN.md as master orchestrator, stage-driven execution, modular context files, and codejob dispatch. Use when creating plans for execution agents like Jules.
---

# CodeJob Agent Workflow

## When to create a PLAN.md vs edit directly

- **Edit `.md` files directly** (CREATION.md, SKILL.md, README.md, etc.) — documentation changes need no plan.
- **Create `docs/PLAN.md`** whenever the task involves modifying or creating Go code. The user reviews it before dispatching.
- `docs/PLAN.md` is ALWAYS at the **module root level** (next to `go.mod`), never inside sub-packages.

## Planning Process (Q&A First)

The planning agent MUST perform a conversational Q&A with the user before writing any PLAN.md:

1. Ask targeted questions and offer suggestions with justification.
2. Wait for user decisions on every architectural choice.
3. Review the actual code before asking — do not ask questions answerable by reading the repo.
4. Only write `docs/PLAN.md` once all decisions are resolved.

**The Q&A discussion stays in chat. `PLAN.md` contains only final resolutions.**

## PLAN.md Rules

- Acts as the entry point for an external agent that has **zero context** about this project.
- Must be fully self-contained: include all relevant constraints, interfaces, conventions, and examples inline.
- Link to relevant docs (`README.md`, `ARCHITECTURE.md`) but do not assume the agent will read them — repeat critical rules inline.
- Structure into clear, sequential execution steps.
- NEVER include `codejob`, `gopush`, or publishing commands — those are local developer tools, not agent instructions.

## Modular Stage Files

For complex features, use `PLAN.md` as a master checklist and break tasks into numbered stage files:

```
docs/
├── PLAN.md                    # Master orchestrator — index + checklist
├── PLAN_STAGE_1_MODELS.md     # Stage 1: data structures
├── PLAN_STAGE_2_CORE.md       # Stage 2: core logic
└── PLAN_STAGE_3_TESTS.md      # Stage 3: tests
```

Each stage file MUST include navigation at the top:
```
← [Stage 1](PLAN_STAGE_1_MODELS.md) | Next → [Stage 3](PLAN_STAGE_3_TESTS.md)
```

## Legacy Reference Code

When porting established logic, append snippets of the original code at the bottom of the relevant stage file. Explicitly tell the agent which logic to recycle and which dependencies to replace.

## Dispatching Work (`codejob`)

After the user approves `docs/PLAN.md`:

1. **Dispatch**: Run `codejob` (no args) — sends `docs/PLAN.md` to the agent.
2. **Review**: When the agent finishes, `codejob` renames `docs/PLAN.md` → `docs/CHECK_PLAN.md`, fetches changes, and switches to the agent's branch for local review.
3. **Resolve**:
   - **Approve**: Run `codejob 'commit message' [tag]` to merge the PR and publish via `gopush`.
   - **Iterate**: Create a new `docs/PLAN.md` and run `codejob` again. The tool merges the old PR and deletes `CHECK_PLAN.md`.

## TinyWasm-Specific Rules

These apply to all plans within the `tinywasm/*` ecosystem:

- **No standard library** in WASM-compiled packages: use `tinywasm/fmt` instead of `errors`, `strconv`, `strings`.
- **Value embedding only**: embed `dom.Element` as a value, never as a pointer (`*dom.Element`). Pointer embeds cause double heap allocation and GC pressure in TinyGo.
- **SSR split**: CSS, SVG icons, and heavy strings MUST live in `ssr.go` (`//go:build !wasm`). They must never reach the WASM binary.
- **No `front.go`**: WASM interactivity goes in the main component file via `OnMount()`. TinyGo eliminates it as dead code on SSR builds.
- **`docs/PLAN.md` at module root**: always next to `go.mod`, never inside sub-packages.
- **No `codejob`/`gopush` in plans**: those are local orchestration tools invisible to the agent.
