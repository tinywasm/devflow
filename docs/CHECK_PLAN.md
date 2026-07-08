# PLAN — codejob: branch checkout must be transactional (no state mutation on the wrong branch)

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

## The bug (observed in production, degrades the whole dev loop)

When a Jules session completes, `HandleDone` (`codejob_state.go`) is supposed
to leave the developer **positioned on the PR branch** with
`docs/PLAN.md → docs/CHECK_PLAN.md` renamed, ready for review. What actually
happens when the working tree is dirty (e.g. `go.mod`/`go.sum` drifted locally
while the agent worked — a routine occurrence, and the PR branch usually
touches those same files, so `git checkout` refuses):

```go
// codejob_state.go — HandleDone step 2 (current, WRONG)
if _, checkoutErr := RunCommandSilent("git", "checkout", branch); checkoutErr != nil {
    fmt.Printf("⚠️  Could not switch branch automatically — run manually:...")
}
// ...and then it CONTINUES: renames PLAN.md, deletes CODEJOB, sets CODEJOB_PR
```

The checkout failure is a **printed warning that scrolls past**, and every
subsequent state mutation (rename to `CHECK_PLAN.md`, `.env` update,
`.gitignore` update) is applied **while still on the default branch**. The
developer later opens `CHECK_PLAN.md`, reviews code that is NOT in the working
tree (it lives only on the un-checked-out PR branch), and can no longer tell
what state the project is in. This happened on `tinywasm/layout` (PR #10) and
`tinywasm/user`.

The same silent-checkout pattern exists in `MergeAndPublish` step 0
(`codejob_state.go`): `RunCommandSilent("git", "checkout", julesBranch)` with
the error **discarded** — if it fails, the subsequent dirty-tree commit
(`"review: corrections before merge"`) and `git push` land on the **default
branch** instead of the PR branch. That one can push wrong commits, not just
confuse.

## The fix — rule: **no state mutation unless positioned on the PR branch**

### Stage 1 — `HandleDone` becomes transactional and retryable

Rework `HandleDone` in `codejob_state.go`:

1. Extract a helper used by both flows (exported for tests):

```go
// CheckoutPRBranch fetches and hard-positions the working tree on the PR's
// head branch. A dirty working tree is handled, not feared: local drift is
// stashed with a labeled stash ("codejob: local drift before review") and
// re-applied after the switch; if re-applying conflicts, the stash is KEPT,
// the conflict files are listed, and an error is returned. Returns the branch
// name on success.
func CheckoutPRBranch(prURL string) (branch string, err error)
```

   Implementation notes:
   - `git fetch --all` first (keep, but a fetch failure before a checkout that
     needs the remote branch IS fatal here — return the error).
   - Resolve branch via `gh pr view <url> --json headRefName` (as today); empty
     branch = error.
   - If `git status --porcelain` is non-empty → `git stash push -u -m
     "codejob: local drift before review"`, then `git checkout <branch>`, then
     `git stash pop`; on pop conflict: leave the stash in place, return a
     descriptive error naming the stash and the conflicting files.
   - Verify the result: `git branch --show-current` must equal the PR branch —
     never trust the checkout exit code alone.

2. `HandleDone` calls `CheckoutPRBranch` **before any state mutation**. If it
   fails: print the actionable message (what to run, e.g. commit/stash and
   re-run `codejob`) and **return the error without touching**
   `docs/PLAN.md`, `.env`, or `.gitignore`. Because `EnvKeyCodejob` is only
   deleted on success, the next `codejob` run re-polls the finished session
   and retries `HandleDone` — the operation becomes **idempotent and
   retryable** instead of half-applied.
3. Only after a verified checkout: rename `PLAN.md → CHECK_PLAN.md`, delete
   `EnvKeyCodejob`, set `EnvKeyCodejobPR`, update `.gitignore` (existing
   logic, unchanged order).
4. Success output states the situation explicitly, e.g.:
   `🔀 On PR branch <branch> — review docs/CHECK_PLAN.md against this tree.`

### Stage 2 — `MergeAndPublish` uses the same guarantee

- Replace the silent step-0 checkout with `CheckoutPRBranch(prURL)`. If it
  fails → **abort with the error** (never commit/push "corrections" from an
  unverified branch).
- Keep the rest of the flow unchanged (dirty-tree commit → push → switch to
  default branch → merge → pull → cleanup → RE_DISPATCH detection).

### Stage 3 — tests (`gotest ./...`, never `go test`; tests live in `test/`)

Extend `test/codejob_state_test.go` (temp git repos, as the existing tests do):

- **Dirty-tree checkout succeeds:** repo on `main` with modified `go.mod` +
  a PR-like branch that also changes `go.mod` → `CheckoutPRBranch` lands on
  the branch, drift re-applied, correct branch reported.
- **Pop conflict:** drift conflicts with the branch → error returned, stash
  still present (`git stash list` non-empty), branch checked out, and —
  critically — `HandleDone` performed **no** rename and left `.env` intact.
- **HandleDone happy path:** on success, `CHECK_PLAN.md` exists, `CODEJOB`
  gone, `CODEJOB_PR` set, and `git branch --show-current` == PR branch.
- **HandleDone retryability:** first call fails (forced conflict), state
  untouched; after resolving, second call completes.
- **MergeAndPublish guard:** checkout failure aborts before any commit (no
  `"review: corrections before merge"` commit exists on any branch).

### Stage 4 — documentation AND flow diagrams (they must mirror the tests)

- **`docs/diagrams/CODEJOB_FLOW.md`** (mandatory — the diagram currently shows
  the buggy behavior as a single box: `HandleDone: fetch, checkout branch,
  rename PLAN → CHECK_PLAN, clean .env`). Redraw the `HandleDone` region so
  every branch in the mermaid graph corresponds 1:1 to a Stage-3 test case:
  - `CheckoutPRBranch`: fetch → resolve branch → dirty? → stash → checkout →
    verify `--show-current` → pop stash.
  - **Success path** → rename + `.env` mutations (only here).
  - **Failure path** (checkout/pop conflict) → stash kept, error surfaced,
    `CODEJOB` preserved → loops back to "run `codejob` again" (retryability).
  - `MergeAndPublish` step 0 now goes through the same `CheckoutPRBranch`
    node, with its abort edge (no "review: corrections" commit on failure).

  Rule: diagram edges and test names must stay in lockstep — a reviewer reads
  the diagram, finds the test that proves each edge.
- `docs/CODEJOB.md` (lifecycle doc) and `README.md` if it describes the loop:
  state the invariant — *"codejob never renames the plan or records the PR
  until the working tree is verifiably on the PR branch; on failure it is
  safe to just re-run `codejob`"*.
- Update the `agents-workflow` skill source
  (`devflow/skills/agents-workflow/SKILL.md`): its lifecycle mermaid shows
  `codejob renames PLAN.md…` — add the invariant to that step so plan-writing
  agents know a failed transition is retryable, not half-applied.

## Harness checklist (mandatory)

- All logic in the library (`codejob_state.go`); `cmd/codejob/main.go`
  untouched (thin main rule).
- No new string literals scattered: the stash label and user-facing hints are
  named constants next to the existing env-key constants.
- Errors are returned, never only printed; printing is additive to the
  returned error, not a substitute (the exact anti-pattern being removed).
- Exit codes: a failed `HandleDone`/`MergeAndPublish` must surface as non-zero
  through the existing error path (verify `cmd` already maps it — do not add
  logic in `main`).

## Acceptance criteria

1. `gotest ./...` green, including the five new cases.
2. Grep check: `RunCommandSilent("git", "checkout"` appears **only inside**
   `CheckoutPRBranch` — no other call site performs raw checkouts.
3. Reproduction scenario (dirty `go.mod` + PR touching `go.mod`) ends
   positioned on the PR branch with drift preserved, or fails loudly with
   zero state mutated — never the half-state observed in production.
4. Re-running `codejob` after a failed `HandleDone` completes the transition.
5. `docs/diagrams/CODEJOB_FLOW.md` reflects the new flow and every edge of the
   `HandleDone`/`CheckoutPRBranch` region maps to a named Stage-3 test (list
   the test name on or next to each edge/branch label).

## Stages

| Stage | File(s) | Action |
|---|---|---|
| 1 | `codejob_state.go` | `CheckoutPRBranch` helper + transactional `HandleDone` |
| 2 | `codejob_state.go` | `MergeAndPublish` step 0 uses the helper, aborts on failure |
| 3 | `test/codejob_state_test.go` | five scenarios above |
| 4 | `docs/diagrams/CODEJOB_FLOW.md`, `docs/CODEJOB.md`, `README.md`, `skills/agents-workflow/SKILL.md` | diagram redrawn in lockstep with Stage-3 tests + invariant documented |
