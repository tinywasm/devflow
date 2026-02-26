# Implementation: `codejob done`

## Development Rules
- **Single Responsibility Principle (SRP):** Every file (CSS, Go, JS) must have a single, well-defined purpose.
- **Mandatory Dependency Injection (DI):** No Global State. Main structs must hold interfaces. `cmd/<app_name>/main.go` is the ONLY place where "Real" implementations are injected.
- **Testing Runner (`gotest`):** For Go tests, ALWAYS use the globally installed `gotest` CLI command.
- **Standard Library Only:** NEVER use external assertion libraries. Use only the standard `testing`, `net/http/httptest`, and `reflect` APIs.
- **Documentation First:** Update documentation before coding.

## Steps

### Step 1 — `codejob_state.go`: Modify `JulesSessionState()`
Update signature to `(msg, prURL string, done bool, err error)`.

### Step 2 — `codejob_state.go`: Modify `HandleDone()`
Add `prURL string` parameter and write `CODEJOB_PR` to `.env`.

### Step 3 — `codejob_state.go`: Add `MergePR()` function
Implement PR merging via `gh` CLI, deletion of `docs/CHECK_PLAN.md`, and cleanup of `.env`.

### Step 4 — `cmd/codejob/main.go`: Add `done` subcommand
Handle `done` case in CLI switch.

### Step 5 — `cmd/codejob/main.go`: Update `runQueryState()`
Pass `prURL` to `HandleDone`.

### Step 6 — `cmd/codejob/main.go`: Add `runDone()`
CLI handler for `MergePR()`.

### Step 7 — `test/codejob_state_test.go`: Add Tests
1. `TestJulesSessionState_ReturnsPRURL`
2. `TestJulesSessionState_EmptyURLWhenWorking`
3. `TestHandleDone_PersistsPRURL`
4. `TestMergePR_NoPRURL`
