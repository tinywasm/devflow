---
name: agents-workflow
description: Multi-agent planning workflow with PLAN.md as master orchestrator, stage-driven execution, modular context files, and codejob dispatch. Use when creating plans for execution agents like Jules.
---

# CodeJob Agent Workflow

> How to write the actual content of a `PLAN.md` (structure, precision level,
> quality checklist, TinyWasm-specific rules) is a separate domain: see skill
> **plan-authoring**. This skill covers the process around it — when to
> create one, the Q&A gate, dispatch, review, and local execution.

## The Planning Agent's Role in This Workflow

The agent reading this skill (Claude, Gemini, or any other installed LLM) acts **only as a planning and documentation agent**:

- Only edits `.md` files. Never executes code, shell commands, or compilers unless the user explicitly requests it.
- Never renames, moves, or deletes `PLAN.md` or `CHECK_PLAN.md`. The lifecycle of those files is managed automatically by `codejob`. **Single exception:** when the user decides a plan runs LOCALLY, it is renamed to `docs/LAST_PLAN_EXECUTED.md` — see "Local Execution Flow" below.
- Never applies a code fix directly when it affects more than 1 file: write a new `PLAN.md` and let codejob dispatch it.

## When to Create `PLAN.md` vs Edit Directly

- **Edit `.md` files directly** (SKILL.md, README.md, ARCHITECTURE.md, etc.) — documentation changes need no plan.
- **Create `docs/PLAN.md`** whenever the task involves modifying or creating Go code. The user reviews it before dispatching. Content rules: skill **plan-authoring**.
- `docs/PLAN.md` is ALWAYS at the **module root level** (next to `go.mod`), never inside sub-packages.

### Never clobber an existing plan — `PLAN.md` becomes an execution queue

Before writing `docs/PLAN.md`, check whether one already exists with a pending plan:

1. **Existing plan found** → copy its full content to a descriptive file: `docs/PLAN_<TOPIC>.md` (topic in SCREAMING_SNAKE, e.g. `PLAN_KIND_UNIFICATION_INPUTSCHEMA.md`).
2. Write the new plan to its own `docs/PLAN_<NEW_TOPIC>.md`.
3. Rewrite `docs/PLAN.md` as an **execution queue**. The dispatch message the agent receives is always *"execute the plan described in docs/PLAN.md"*, so the queue MUST open with an explicit instruction that resolves that message to all of them:

   ```markdown
   # PLAN — execution queue for `<module>`

   > If you were told to "execute the plan described in docs/PLAN.md", execute
   > **ALL the plans below, in order (top to bottom)**. Each plan is
   > self-contained; finish one (its acceptance criteria green) before starting
   > the next. Never mix changes from one plan into another.

   | Order | Plan | Subject |
   |-------|------|---------|
   | 1 | [PLAN_<TOPIC>.md](PLAN_<TOPIC>.md) | ... |

   After completing all plans, run `gotest ./...` one final time: everything green.
   ```

4. **No existing plan** → write the plan directly in `docs/PLAN.md` (single-topic case; it stays dispatchable by `codejob` as-is).

### Local Execution Flow — `LAST_PLAN_EXECUTED.md`

Not every plan is dispatched. When the user decides an existing `docs/PLAN.md`
will be executed **locally** (immediately, in-session, instead of via codejob):

1. **Rename `docs/PLAN.md` → `docs/LAST_PLAN_EXECUTED.md` at that moment.**
   This frees `docs/PLAN.md` (no clash with codejob's rename/delete lifecycle
   or with other plans queued later) and marks the plan as locally owned.
2. Execute the work with `LAST_PLAN_EXECUTED.md` as the spec.
3. It is **committed together with its implementation on `gopush`** (the only
   publish path) — the executed spec lands in git history next to the code it
   produced.
4. On the NEXT local execution in the same repo: **overwrite the content** of
   the existing `docs/LAST_PLAN_EXECUTED.md` with the new plan — never create
   numbered variants (`LAST_PLAN_EXECUTED_2.md`). Git history preserves every
   previous version, so the repo keeps a detailed, commit-anchored record of
   what was done and when.
5. Its content ROTATES: other documents may point to it only as "the most
   recent locally executed plan" — never cite its sections or rely on
   specific content (same staleness rule as `PLAN.md` references).

### Master plan naming — never overwrite `MASTER_PLAN.md`

Multi-repo orchestrators at the monorepo root use a **descriptive name consistent with the task**: `docs/<TOPIC>_MASTER_PLAN.md` (e.g. `SIZE_OPTIMIZATION_MASTER_PLAN.md`, `MCP_DAEMON_HARDENING_MASTER_PLAN.md`). A bare `docs/MASTER_PLAN.md` likely already exists from a previous wave — never overwrite or reuse it for a new topic.

## Planning Process (Q&A First)

The planning agent MUST perform a conversational Q&A with the user before writing any `PLAN.md`:

1. Read the relevant code before asking — do not ask questions the code already answers.
2. **Investigate prior art before proposing a mechanism**: the repo's git history, predecessor repos (pre-split origins), and settled decision records (ARCHITECTURE/DESIGN). When a decision record rejects an approach, verify its scope — it may have rejected a *different variant* than the one under consideration (e.g. "never execute the user's package" does not cover executing dependency packages).
3. **Never present a single take-it-or-leave-it proposal for an architectural choice.** Offer at least two candidate approaches with honest trade-offs and a recommendation, and wait for the user's decision. Writing a plan around an un-offered choice invalidates the plan.
4. Only write `docs/PLAN.md` once all decisions are resolved.

**The Q&A stays in chat. `PLAN.md` contains only final resolutions.**

## Plans Are Ephemeral — Rationale Lives in Permanent Docs

`PLAN.md` is deleted when the loop closes (renamed `CHECK_PLAN.md`, then removed by `codejob`). Consequences:

- Anything that must outlive execution — decision rationale, rejected alternatives, contracts — goes to **permanent docs** (`ARCHITECTURE.md`, `DESIGN.md`) **before dispatch** (documentation-first). The plan's documentation stage then says **VERIFY the docs against the implementation**, never "create" them.
- **Permanent docs (README, ARCHITECTURE, DESIGN, SPECS, diagrams) must NEVER link to or cite `PLAN.md`/`CHECK_PLAN.md`**, including section references like "PLAN §8" — they are guaranteed dead references. If a doc written ahead of implementation needs an interim marker, use a self-deleting note — `STATUS (remove this note when X lands): …` — and make its removal an explicit task in the plan.

## Plan Lifecycle

```mermaid
flowchart TD
    A[Planning agent writes<br/>docs/PLAN.md] --> B[User runs codejob<br/>dispatches to Jules]
    B --> C[Jules opens PR<br/>on a branch]
    C --> D[User runs codejob<br/>no args]
    D --> E[codejob renames<br/>PLAN.md to CHECK_PLAN.md<br/>sets CODEJOB_PR in .env<br/>ON SUCCESSFUL CHECKOUT]
    E --> F[User asks planning agent<br/>to review CHECK_PLAN.md]
    F --> G{Implementation<br/>correct?}
    G -->|yes| H[Planning agent or user runs<br/>codejob commit msg]
    G -->|errors| I[Planning agent writes<br/>new docs/PLAN.md<br/>with the fix]
    I --> B
    H --> J[codejob: merge PR<br/>gopush + tests<br/>delete CHECK_PLAN.md]
```

### Key rules for the planning agent when reviewing `CHECK_PLAN.md`

`CHECK_PLAN.md` is the **original `PLAN.md` renamed automatically by `codejob`** after Jules opens a PR. It is the spec of what was supposed to be implemented.

When the user asks the planning agent to review a `CHECK_PLAN.md`:

1. **Read `CHECK_PLAN.md`** to understand what was planned (stages, expected outputs, criteria).
2. **Inspect the actual code** in the repo to verify each stage was executed correctly.
3. **Verify documentation** — this is mandatory, agents frequently skip it:
   - `docs/API.md` updated if public API changed (new functions, types, signatures).
   - `docs/ARCHITECTURE.md` updated if design or structure changed.
   - `README.md` updated if usage examples or install instructions are affected.
   - `docs/SKILL.md` updated if the library's usage conventions changed.
   - Any doc explicitly listed as a deliverable in `CHECK_PLAN.md` must exist and be accurate.
   - If documentation is missing or stale → write a new `docs/PLAN.md` with only the doc fixes.
4. **Run or instruct tests** if needed (`gotest ./...`).
5. **If everything is correct (code + docs):** run `codejob 'commit message'` to close the loop (or tell the user to run it if they prefer).
6. **If something is missing or broken:** write a new `docs/PLAN.md` with the specific fix. Do NOT edit code directly.

The planning agent **never**:
- Renames, moves, or deletes `PLAN.md` or `CHECK_PLAN.md` — managed by `codejob` (sole exception: the rename to `LAST_PLAN_EXECUTED.md` when the user opts for local execution — see "Local Execution Flow"). The codejob rename only happens after a successful branch checkout; if it fails, it is safe to re-run `codejob`.
- Runs `gopush` directly — `codejob` calls it internally.
- Applies multi-file code fixes directly — always via a new `PLAN.md`.

The planning agent **runs `codejob` when the user says "despacha"** (dispatch). This sends `docs/PLAN.md` to the execution agent (Jules):

```bash
codejob   # dispatches docs/PLAN.md to Jules
```

The `codejob 'commit message'` form (close loop / publish) can be run by **the planning agent or the user**:

```bash
codejob 'commit message'        # merge PR + gopush + delete CHECK_PLAN.md
codejob 'commit msg' v0.2.0     # same with explicit tag
```

## Error Handling After Agent Execution

When `gotest` fails or the agent reports errors:

| Scenario | Planning agent's action |
|---|---|
| Error in 1 file | Write new `PLAN.md` with the exact fix (include code) |
| Error in 2+ files | Write new self-contained `PLAN.md` with all changes |
| Design logic error | Q&A with user → new `PLAN.md` with resolved decision |

In all cases: the planning agent **does not execute** the fix directly. It only writes the `PLAN.md`.
