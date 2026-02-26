# Devflow: Resilient Source ID Auto-Detection

This is the **Master Prompt (PLAN.md)** for execution agents working on `tinywasm/devflow`.

## Background
When a GitHub repository is transferred to a new organization or renamed (e.g., from `cdvelop/postgre` to `tinywasm/postgres`), the `gh repo view` command immediately returns the new URL/name. However, the Jules API relies on Google Cloud indexing, which has eventual consistency. This means Jules might return a `404 NOT_FOUND` for `sources/github/tinywasm/postgres` while the old `sources/github/cdvelop/postgre` is actually still active and valid in its database. 
We need Devflow to smartly detect this scenario by parsing `git remote -v` directly, extracting the local git configuration (which often retains the old URL until manually updated or serves as a solid fallback), and attempting a fallback query to Jules.

## Execution Steps

### 1. Extract Local Git Origin (`func getLocalGitOrigin()`)
- In `devflow/code_jules.go`, create a new helper function `func getLocalGitOrigin() (owner, repo string, err error)`.
- This function should execute `git remote -v` and parse the output for the `origin` fetch URL.
- It must successfully parse standard formats:
  - HTTPS: `https://github.com/cdvelop/postgre.git`
  - SSH: `git@github.com:cdvelop/postgre.git`
- Return the extracted `owner` and `repo` (stripping `.git`).

### 2. Refactor `autoDetectOwnerRepo` Fallback
- Modify the existing `autoDetectOwnerRepo()` in `code_jules.go`.
- Continue to use `gh repo view --json owner,name` as the primary, modern source of truth.
- Modify the return signature to return TWO sets of coordinates if they differ: `func getCandidateOrigins() ([]string, error)` (where the string is the full `sources/github/OWNER/REPO` ID).
- The array should always prioritize the `gh repo view` output.
- Secondarily, append the `getLocalGitOrigin()` output to the array **if and only if** it differs from the `gh` output.

### 3. Implement Fallback Retry Loop in `Send`
- Refactor `d.resolveSourceID()` to return the array of candidates `[]string`.
- Inside `d.Send(...)`, when querying the Jules API:
  - First, attempt the session creation (`doPost()`) using `candidates[0]`.
  - If Jules returns exactly `404 Not Found`, AND we have a `candidates[1]` (meaning the local git remote differs from the active GitHub location):
  - Log to the user: `Jules: 404 on <new-repo>, falling back to local git origin <old-repo> due to possible replication delay...`
  - Immediately retry the `doPost()` replacing the `sourceID` with `candidates[1]`.
  - If `candidates[1]` works, great! Keep using it for polling. If it also returns 404, fallback to the standard polling loop on `candidates[0]` as originally programmed, waiting for the indexer to catch up.

### 4. Ignore Local Configuration in Sync Checks
- In `devflow/git_handler.go` there is a function `HasPendingChanges() (bool, error)`.
- Currently, it executes `git status --porcelain` and if ANY file is untracked or modified, it blocks `codejob` dispatch and shows a warning (`Jules reads from GitHub, not the local filesystem`).
- Modify `HasPendingChanges` to filter out lines from the porcelain output that end with `.env` or `.gitignore`.
- If the only pending files are `.env` or `.gitignore`, it should return `false` (meaning the repo is clean enough to dispatch Jules).

### 5. Verification
- Add a new test in `test/code_jules_test.go` or similar to mock `git remote -v` and `gh repo view` exhibiting different names, asserting that `getCandidateOrigins()` returns both.
- Add a new test in `test/git_handler_test.go` to mock `git status --porcelain` returning only `.env` or `.gitignore` changes and ensure `HasPendingChanges()` returns `false`.
- Use `gotest` from the root of `devflow` to run all unit and WASM integration tests, enforcing that `tinywasm/fmt` is used appropriately if applicable to the test suite limits.
