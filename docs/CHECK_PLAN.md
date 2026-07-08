# PLAN: devflow Bug-Fix Orchestrator

Status: **PROPOSED** — master plan coordinating two independent bug fixes. Execute the sub-plans in the order below.

## Development Rules

Compiled from project skills; these constraints apply to every sub-plan:

- **Documentation First:** Update the relevant plan and command doc (`docs/GOTEST.md`, `docs/GOPUSH.md`) before pushing code (`gopush`).
- **Test First:** Every fix starts with a failing test in `test/` that reproduces the real problem; only then change the code to make it pass.
- **SRP / file structure:** New behavior goes in its own small file, not appended to existing large files. One responsibility per type.
- **Dependency injection for exec:** Tests must override process creation via the exported hooks (`GoTestCmdFn`, `ExecCommand`) — never spawn real `go test` / `git` from unit tests.
- **No breaking CLI changes:** existing flags of `gotest`, `gopush`, `codejob` keep working.
- **Single source of truth:** `codejob` publishes through `Publisher.Publish` → `Go.Push` (gopush). Any publish-flow fix goes in `Go.Push` and its helpers — never duplicated in codejob.
- **English docs, mirrored chat.**

## Sub-plans and execution order

| # | Plan | Fixes | Status |
|---|------|-------|--------|
| 1 | [GOTEST_TIMEOUT_PLAN.md](GOTEST_TIMEOUT_PLAN.md) | Per-package timeout budget kills healthy tests (`gotest`) | PROPOSED |
| 2 | [GOPUSH_SELFDEP_PLAN.md](GOPUSH_SELFDEP_PLAN.md) | Publishing a module with same-repo submodules leaves it inconsistent (`gopush` / `codejob`) | PROPOSED |

**Order rationale:** fix #1 first. `gopush` runs `gotest` as its publish gate, and devflow's own suite currently trips the per-package timeout (`TestGoNewCreateLocalOnly` blamed at 0s elapsed) — publishing fix #2 would be blocked or flaky until the watchdog (or at minimum Phase 4 integration-tagging of GOTEST_TIMEOUT_PLAN) lands.

## Shared completion criteria

- Each sub-plan finishes with: its failing tests now green, full `gotest` green, docs updated, published via `gopush`.
- Final validation of the whole plan: republish `tinywasm/orm` (currently in the inconsistent state described in sub-plan 2) and verify the repo ends clean — single commit, tag includes submodule updates, `replace` preserved.
- Update the Status field here and in each sub-plan as work progresses (PROPOSED → IN PROGRESS → DONE).
