# PLAN: Fix gotest cache not invalidating on untracked files

## Problem

When running `gotest` (no arguments / full suite), the result is served from cache even when a
new, untracked test file has been added to the repo. This causes a silent false-positive: a
failing test is never executed and `gotest` reports success.

### Reproduction steps

1. Run `gotest` in a module — all tests pass → cache is saved.
2. Add a new `*_test.go` file that contains a failing test **without staging/committing it**
   (`git status` shows `?? new_test.go`).
3. Run `gotest` again → returns the cached success, the new test never runs.

Confirmed in `tinywasm/dom`: `test/uc_focus_preserve_test.go` (WASM, untracked) contains
`TestUpdate_PreservesActiveElementFocus` which fails, but `gotest` (no args) reports
`wasm ✅`.

## Root Cause

`GetGitState()` in [git_test_cache.go](../git_test_cache.go) builds the state key from:

```
commitHash + md5(git diff HEAD)
```

`git diff HEAD` only covers **tracked** files (staged + modified). It produces **no output**
for untracked files, so the state key is identical before and after adding an untracked file.
`IsCacheValid()` therefore returns `true` and the cached message is returned immediately.

```go
// git_test_cache.go:55 — does NOT include untracked files
diff, err := RunCommandSilent("git", "diff", "HEAD")
```

## Failing Test (regression guard)

`test/git_test_cache_test.go` → `TestTestCache_UntrackedFileInvalidatesCache`

Run it to confirm the bug:
```bash
go test -run TestTestCache_UntrackedFileInvalidatesCache ./test/
# --- FAIL: TestTestCache_UntrackedFileInvalidatesCache
# BUG: cache is still valid after adding an untracked .go file
```

## Fix

Update `GetGitState()` to also hash the list of **untracked `.go` files** using
`git ls-files --others --exclude-standard`. Including only `.go` files avoids false
invalidations from editor temp files, build artifacts, etc.

### Implementation (`git_test_cache.go`)

```go
func (tc *TestCache) GetGitState() (string, error) {
    commitHash, err := RunCommandSilent("git", "rev-parse", "HEAD")
    if err != nil {
        return "", fmt.Errorf("failed to get commit hash: %w", err)
    }
    commitHash = strings.TrimSpace(commitHash)

    // Tracked changes (staged + unstaged modifications)
    diff, _ := RunCommandSilent("git", "diff", "HEAD")

    // Untracked .go files — not covered by git diff HEAD
    untrackedRaw, _ := RunCommandSilent("git", "ls-files", "--others", "--exclude-standard")
    var goUntracked []string
    for _, f := range strings.Split(untrackedRaw, "\n") {
        f = strings.TrimSpace(f)
        if strings.HasSuffix(f, ".go") {
            goUntracked = append(goUntracked, f)
        }
    }
    sort.Strings(goUntracked)
    untrackedKey := strings.Join(goUntracked, "\n")

    combined := diff + "\x00" + untrackedKey
    diffHash := fmt.Sprintf("%x", md5.Sum([]byte(combined)))

    return commitHash + ":" + diffHash[:8], nil
}
```

`sort` import must be added.

## Acceptance Criteria

- `TestTestCache_UntrackedFileInvalidatesCache` passes after the fix.
- Existing cache tests (`TestTestCache_SaveAndValidate`, `TestTestCache_InvalidateCache`,
  `TestTestCache_GitState`) continue to pass.
- In `tinywasm/dom`: `gotest` (no args) reports `wasm ❌` while
  `test/uc_focus_preserve_test.go` is untracked and its test fails.

## Scope

- **1 file changed**: `devflow/git_test_cache.go` — only `GetGitState()`.
- No API changes; no other callers affected.
