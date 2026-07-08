# PLAN: gopush Same-Repo Submodule Handling

Status: **PROPOSED** — sub-plan 2 of [PLAN.md](PLAN.md). Read the orchestrator's Development Rules first; execute after [GOTEST_TIMEOUT_PLAN.md](GOTEST_TIMEOUT_PLAN.md).

## Problem

A library can contain submodules with their own `go.mod` that depend on the parent module itself (e.g. `tinywasm/orm` contains `ormcp/`, whose `go.mod` requires `github.com/tinywasm/orm`). When `gopush` (or `codejob`, which publishes through the same `Go.Push`) publishes a new tag:

1. `UpdateDependents` walks `..` and `FindDependentModules` treats `ormcp/` like any **external** dependent — it cannot tell that the dependent lives inside the repo just published.
2. `UpdateDependentModule` calls `RemoveReplace`, and `isLocalReplaceTarget` only preserves `=> .` — a parent-pointing `replace github.com/tinywasm/orm => ../` is **deleted**.
3. It then runs `go get` + `go mod tidy` + commit + push on the submodule — mutating the repo **after** the tag was created.

Result: the just-published repo is immediately inconsistent again — every publish re-dirties it, forever. Real evidence (current state of `tinywasm/orm` after publishing v0.9.25): uncommitted changes in `go.mod`, `go.sum`, `ormcp/go.mod`, `ormcp/go.sum`, and `ormcp/go.mod` has **no** `replace` left.

## Required behavior (same-repo submodules only)

For a submodule `go.mod` inside the repo being published that requires the parent module, three things must happen — **before** the publish commit, so the tag captures them:

1. **Version bump:** its `require <parent> <tag>` is updated to the tag about to be published.
2. **Replace preserved:** an existing local `replace <parent> => ../` (any relative path into the repo) is never removed.
3. **Replace added if missing:** if no such replace exists, add one, so the submodule's tests always build against the local (latest, unpublished) parent code.

External dependents keep today's behavior exactly (remove stale replace, `go get`, tidy, test, push).

## Design

### Detection

A dependent dir found by `FindDependentModules` is a *same-repo submodule* when it is located inside the published module's root (`strings.HasPrefix(depDir, rootDir+sep)` on absolute cleaned paths). Cross-check: its module path starts with `<parentModulePath>/`.

### Flow change in `Go.Push`

The next tag is already computed before commit (`git_handler.go`: `GenerateNextTag` / `IncrementTag`). New step between tests and the commit/tag:

```
tests pass
  → nextTag = GenerateNextTag()                     (existing logic, hoisted)
  → for each same-repo submodule requiring parent:
        EnsureReplace(parent, relPathToRoot)         (add if missing, keep if present)
        set require parent → nextTag                 (edit go.mod line; go mod tidy)
  → git add / commit / tag nextTag / push            (submodule changes ride the same commit)
  → UpdateDependents(...)                            (now skips same-repo submodules)
```

The `replace` makes the `require` version irrelevant for local builds, so pointing at a tag that exists only seconds later is safe; after the push the tag is live and external consumers of the submodule resolve normally.

### Code changes

- `go_mod.go` — new `EnsureReplace(modulePath, localPath string) bool` on `GoModHandler` (idempotent; respects single-line and block `replace (…)` syntax).
- New file `go_selfdep.go` — `(g *Go) syncInternalSubmodules(modulePath, nextTag string) error`: find, ensure replace, bump require, tidy. Called from `Push` pre-commit.
- `go_mod.go` `FindDependentModules` (or its caller `UpdateDependents`) — exclude dirs inside `g.rootDir`.
- `isLocalReplaceTarget` unchanged: external dependents' sibling-checkout replaces (`=> ../orm`) must still be removable; internal submodules simply never reach `UpdateDependentModule` anymore.

## Steps

### Phase 1 — Reproduce (TDD)

1. `test/go_selfdep_test.go`, failing first, using temp dirs (pattern of `testCreateGoModule`) with a parent module + `sub/go.mod` requiring it:
   - `FindDependentModules`/`UpdateDependents` currently returns/mutates the internal submodule → assert it must be excluded;
   - `RemoveReplace` flow deletes `=> ../` → assert preserved via the new path;
   - submodule without replace → assert `EnsureReplace` adds `replace parent => ../`;
   - submodule require bumped to `nextTag` **before** commit (inspect `go.mod` content at the mocked commit step via `ExecCommand`/`GoTestCmdFn` hooks — no real git/network).

### Phase 2 — Implement

2. `EnsureReplace` in `go_mod.go` (+ unit tests for line/block syntax, idempotency).
3. `go_selfdep.go` with `syncInternalSubmodules`; hoist next-tag computation in `Push` so it is known pre-commit; wire the call.
4. Exclusion of same-repo dirs in the dependents scan.

### Phase 3 — Repair & validate

5. Repair `tinywasm/orm`: ensure `ormcp/go.mod` gets its `replace github.com/tinywasm/orm => ../` back, then run the new `gopush` on orm — the repo must end **clean**: one commit, tag containing the submodule bump, replace intact, `ormcp` never pushed separately.
6. Regression: publish a module with a genuine external dependent and confirm behavior is unchanged.

### Phase 4 — Docs & release

7. Update `docs/GOPUSH.md` (new "Same-repo submodules" section) and `docs/CODEJOB.md` (note that it inherits the behavior via `Publish`). Publish devflow with `gopush`.

## Test strategy summary

| Case | Expected |
|---|---|
| `sub/go.mod` requires parent, has `replace => ../` | Replace kept, require bumped to next tag pre-commit, no separate push |
| `sub/go.mod` requires parent, no replace | Replace added, require bumped, no separate push |
| Same repo, submodule does not require parent | Untouched |
| External dependent with stale sibling replace | Today's behavior: replace removed, updated, tested, pushed |
| Publish twice in a row | Second run: "Nothing to push" — repo stays clean (the core bug) |
