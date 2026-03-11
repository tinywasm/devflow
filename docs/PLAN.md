# PLAN: Refactor gopush + codejob

## Development Rules

- **Mandatory DI:** No global state. Interfaces for external deps. Injection only in `cmd/*/main.go`.
- **Standard Library Only:** No external assertion libraries. Mocks for all I/O.
- **Max 500 lines per file.**
- **Testing:** Use `gotest` CLI. Install with `go install github.com/tinywasm/devflow/cmd/gotest@latest`.
- **Diagram-Driven Testing:** Flows in diagrams MUST have corresponding tests.

---

## Summary

Three changes:

1. **Delete `push` tool** — `gopush` absorbs its functionality (auto-detect `go.mod`).
2. **Decouple CodeJob from gopush** — `gopush` no longer dispatches CodeJob. The dependency inverts: `codejob` calls `gopush`.
3. **`codejob` unified API** — no subcommands (`init`/`done` removed). Message as argument implies "close loop". Publishes via `gopush` and conditionally updates dependents or re-dispatches.

---

## New Flow Diagrams

### gopush (universal)

> Diagram: [GOPUSH_FLOW.md](diagrams/GOPUSH_FLOW.md)

No CodeJob dispatch. gopush is purely build+publish. Two distinct paths: non-Go projects get a plain git push; Go projects get the full pipeline (tests → push → install → deps → backup). Dependent results print in real-time (one line per dep); final summary only reflects the main package.

---

### codejob (unified)

> Diagram: [CODEJOB_FLOW.md](diagrams/CODEJOB_FLOW.md)

Single entry point, behavior driven by arguments:
- **No args** → dispatch PLAN.md to agent (auto-setup if no API key) or check active session status.
- **With message** → close the loop: merge PR, publish via gopush. If new PLAN.md exists → re-dispatch. If not → full publish (with deps if Go).
- **With message + tag** → same as above with explicit tag.

Error if message provided but no pending PR (`CODEJOB_PR` in `.env`).

---

## Execution Steps

### Stage 1: Make gopush universal (delete push)

1. In `go_handler.go`: at the start of `Go.Push()`, detect `go.mod`. If missing, skip tests/install/deps and only run `git.Push()`.
2. Add `skipTag` bool parameter to `Go.Push()`. When true, skip tag generation/creation and call `git.PushWithoutTags()` after commit. When false, use existing `git.Push()` flow (commit + tag + push). This allows codejob to sync code without polluting the version history.
3. Remove `withCodeJob()` call from `git_handler.go` `Git.Push()`.
4. Remove CodeJob-related fields/imports from `Git` struct.
5. Delete `cmd/push/main.go` and its directory.
6. Update `cmd/gopush/main.go` to no longer inject CodeJob drivers into Git.
7. Delete `docs/PUSH.md`.

### Stage 2: codejob refactor (unified API)

1. Add a `Publisher` interface to `interface.go`:
   ```go
   type Publisher interface {
       Publish(message string, tag string) (PushResult, error)
   }
   ```
2. Refactor `cmd/codejob/main.go`:
   - Remove `init` and `done` subcommands.
   - New argument parsing: no args → dispatch/status. With args → close loop (message, optional tag).
   - Auto-detect missing API key → run setup wizard inline before dispatch.
   - Inject `Go` handler as Publisher.
3. In `codejob.go`: add `Publisher` field to `CodeJob`. Before `Send()`, call `Publisher.Publish()` (skip deps, backup, and tag).
4. Refactor `MergeAndPublish()`:
   - Accept Publisher. After merge+pull+cleanup, check for `PLAN.md`.
   - If PLAN.md exists → call Publisher.Publish (skip deps + tag) + dispatch to agent.
   - If no PLAN.md → call full gopush (with deps if go.mod exists, plain push otherwise).
   - Remove manual tag creation — delegate to gopush.
5. Delete `codejob_init.go` (wizard moves inline into codejob.go or cmd entry point).

### Stage 3: Dependent output refactor

1. In `go_handler.go` `UpdateDependentModule()`: print result per dependent in real-time via `consoleOutput` (e.g., `📦 mylib → ✅ updated to v1.2.3` or `📦 mylib → ❌ tests failed`).
2. In `go_handler.go` `Push()`: do NOT append dependent results to the summary slice. Final summary only contains main package result.
3. Change `skipTests=true` to `skipTests=false` in `UpdateDependentModule` recursive call (line 278). Keep `skipDeps=true, skipBackup=true`.

### Stage 4: Tests (diagram-driven)

**gopush tests** — existing: `test/go_handler_test.go` → `TestGoPush`.

1. **Update `TestGoPush`**: verify it still passes with the new `go.mod` detection and `skipTag` logic.
2. **Add `TestGoPush_NoGoMod`**: create temp dir WITHOUT `go.mod`. Verify gopush only calls `git.Push()` + backup, skips tests/install/deps.
3. **Add `TestGoPush_SkipTag`**: verify that when `skipTag=true`, `PushWithoutTags()` is called instead of `Push()`.
4. **Add `TestGoPush_DependentOutput`**: verify dependents print via `consoleOutput` (real-time) and do NOT appear in the returned summary string.
5. **Add `TestGoPush_DependentTestFailure`**: verify that when a dependent's tests fail, it prints `❌` via consoleOutput and continues to next dependent.
6. Update `MockGitClient` if new interface methods are added.

**codejob tests** — existing: `test/codejob_test.go`, `test/codejob_state_test.go`, `test/codejob_dispatch_test.go`.

7. **Update `TestCodeJobSend*`**: verify `Publisher.Publish()` is called before dispatch (skip tag, deps, backup).
8. **Add `TestCodeJob_NoArgs_AutoSetup`**: verify setup wizard runs when API key missing.
9. **Update `TestMergeAndPublish*`**: verify it calls gopush (full with tag+deps) when no PLAN.md, and gopush (skip tag+deps) + re-dispatch when PLAN.md exists.
10. **Add `TestCodeJob_MessageWithoutPR`**: verify error when message provided but no `CODEJOB_PR` in `.env`.
11. **Remove `TestDispatchCodeJob_*`**: `DispatchCodeJob()` function is deleted in Stage 6.

### Stage 5: Cleanup

1. Remove `DispatchCodeJob()` function from `codejob.go` (no longer called from git).
2. Remove `withCodeJob()` method from `git_handler.go`.
3. Update `docs/GOPUSH.md` documenting universal behavior (go.mod detection, skipTag).
4. Update `docs/CODEJOB.md` documenting unified API (no init/done subcommands).
5. Delete old diagrams (replaced by `docs/diagrams/`):
   - `docs/codejob/diagrams/CODEJOB_DISPATCH_FLOW.md` → move to `docs/codejob/DRIVER_JULES.md`
   - `docs/codejob/diagrams/CODEJOB_INIT_FLOW.md`
   - `docs/codejob/diagrams/CODEJOB_STATE_FLOW.md`
   - `docs/codejob/diagrams/GOPUSH_CODEJOB_FLOW.md`
   - `docs/codejob/diagrams/JULES_WORKFLOW.md` (keep only if still relevant)
6. Update `README.md`: remove `push` references, update diagram links to `docs/diagrams/`.
7. Update any cross-references in `docs/CODEJOB.md` and `docs/GOPUSH.md` pointing to old diagram paths.

---

## Files to modify

| File | Action |
|---|---|
| `cmd/push/` | DELETE (entire directory) |
| `cmd/gopush/main.go` | Update (remove CodeJob injection) |
| `cmd/codejob/main.go` | Update (unified API, inject Publisher) |
| `git_handler.go` | Remove `withCodeJob()`, CodeJob fields |
| `go_handler.go` | Add go.mod detection, skipTag, skipDeps |
| `codejob.go` | Add Publisher field, call before Send |
| `codejob_state.go` | Refactor MergeAndPublish to use Publisher |
| `codejob_init.go` | DELETE (wizard moves inline) |
| `interface.go` | Add Publisher interface |
| `docs/PUSH.md` | DELETE |
| `docs/GOPUSH.md` | Update (universal behavior) |
| `docs/CODEJOB.md` | Update (unified API) |
| `docs/codejob/diagrams/CODEJOB_DISPATCH_FLOW.md` | MOVE → `docs/codejob/DRIVER_JULES.md` |
| `docs/codejob/diagrams/CODEJOB_INIT_FLOW.md` | DELETE |
| `docs/codejob/diagrams/CODEJOB_STATE_FLOW.md` | DELETE |
| `docs/codejob/diagrams/GOPUSH_CODEJOB_FLOW.md` | DELETE |
| `docs/codejob/diagrams/JULES_WORKFLOW.md` | DELETE (if obsolete) |
| `README.md` | Remove push refs, update diagram links |
| `test/codejob_test.go` | Update (Publisher, unified API) |
| `test/codejob_state_test.go` | Update (MergeAndPublish refactor) |
| `test/codejob_dispatch_test.go` | Update or DELETE |
| `test/go_handler_test.go` | Update + add new tests |
