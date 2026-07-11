# gopush Flow

Universal build+publish pipeline. Detects `go.mod` to choose between plain git
push or the full Go workflow. Dependents are updated through a **transitive
cascade coordinator** (topological order, one commit+tag per module per wave).

> **Target flow.** The executable contract lives in the test suite — each
> diagram section links the tests that lock it. Implementation must make those
> tests pass without changing their expectations.

## Contract → tests

| Contract | Locked by |
|---|---|
| Dirty-guard per node: pathspec-limited commit (`go.mod`+`go.sum` only), no tag, never `git add .`/`-A` | [`TestUpdateDependentModule_DirtyTreeCommitsOnlyGoModAndSum`](../../test/dependents_guard_test.go) |
| Git primitives: `StatusPorcelain`, `CommitPaths`, `DiffShortStat` (diff vs HEAD, staged or not), `WorkTreeDirtyBeyond` | [`test/dependents_guard_test.go`](../../test/dependents_guard_test.go) |
| Graph: transitive closure, topological order, single node per module, cycle = error, `MaxCascadeDepth = 10` | [`TestBuildDependentGraph_*`](../../test/cascade_test.go) |
| Wave semantics: one call per node with ALL published bumps, failure cuts only its branch, partial updates allowed, deps-only does not propagate, skipped when zero bumps | [`TestRunCascade_*`](../../test/cascade_test.go) |
| Deps commit format: `deps:` title, `cause:` line propagating the root message, bump list | [`TestBuildDepsCommitMessage`](../../test/commit_message_test.go) |
| Root push: user title intact + `--shortstat` body | [`TestGoPush_AppendsShortStatBody`](../../test/go_handler_test.go) |
| `UpdateDependentModule` carries `rootCause` (4th parameter) | [`TestUpdateDependentModule`](../../test/go_handler_test.go) |

## Main pipeline

```mermaid
flowchart TD
    A[gopush 'msg' tag] --> B{go.mod exists?}
    B -- No --> C[git add + commit + push<br/>tag if !skipTag]
    C --> M[Backup]
    M --> D[Done: print summary]
    B -- Yes --> V{skipVerify?}
    V -- No --> VR[go mod verify]
    VR --> VF{Verify pass?}
    VF -- No --> VE[Exit 1: actionable error<br/>unknown revision / checksum / go.sum]
    VF -- Yes --> E[Run gotest]
    V -- Yes --> E
    E --> F{Tests pass?}
    F -- No --> G[Exit 1]
    F -- Yes --> H[git add + commit + push<br/>title = user msg, body = diff --shortstat<br/>tag if !skipTag]
    H --> I{cmd/ exists?}
    I -- Yes --> J[Install binaries<br/>print each ✅ to console]
    I -- No --> K{skipDependents or --no-cascade?}
    J --> K
    K -- Yes --> M
    K -- No --> W[BuildDependentGraph<br/>transitive closure + topological sort]
    W --> WC{Cycle or levels > MaxCascadeDepth?}
    WC -- Yes --> WE[Error: abort cascade<br/>nothing published from it]
    WE --> M
    WC -- No --> L0[RunCascade: per topological level,<br/>parallel workers within the level]
    L0 --> RPT[Print CascadeReport table]
    RPT --> M
```

- Shortstat body: computed **before** staging, so its contract is
  `git diff HEAD --shortstat` (staged or not) — a `--cached`-only
  implementation returns empty at message-build time
  ([`TestGitDiffShortStat`](../../test/dependents_guard_test.go)).
- Graph rules: cycles abort with an explicit error before anything is
  published; `MaxCascadeDepth = 10` topological levels
  ([`TestBuildDependentGraph_CycleIsAnError`, `TestBuildDependentGraph_DepthLimit`](../../test/cascade_test.go)).

## Per-node cascade processing

Each dependent node is processed **exactly once** per wave
([`TestRunCascade_DiamondProcessesNodeOnceWithAllBumps`](../../test/cascade_test.go)),
receiving the bumps of ALL its in-cascade dependencies published in this wave.
A node with zero available bumps (every upstream failed or published nothing)
is **skipped**; a node with some failed upstreams is still processed with the
bumps that did publish — partial updates are safe: the module simply stays on
the old version of the failed dependency
([`TestRunCascade_FailureCutsOnlyItsBranch`](../../test/cascade_test.go)).

```mermaid
flowchart TD
    N[Node: dependent module<br/>+ bumps from published upstreams] --> N2[Remove replace of published deps<br/>go get all bumps + go mod tidy + go generate]
    N2 --> N1{CODEJOB active in .env?}
    N1 -- Yes --> NS1[⏭ updated, push skipped<br/>report: skipped]
    N1 -- No --> N3{Other replaces?}
    N3 -- Yes --> NS2[⏭ updated, push skipped<br/>report: skipped]
    N3 -- No --> N4[Run gotest]
    N4 -- fail --> NF[❌ revert go.mod/go.sum<br/>report: failed — branch cut]
    N4 -- pass --> N5{Dirty beyond go.mod/go.sum?<br/>.env/.gitignore ignored}
    N5 -- Yes --> N6[CommitPaths: ONLY go.mod+go.sum<br/>deps msg + cause, push WITHOUT tag<br/>report: deps-only ⚠ — no propagation]
    N5 -- No --> N7[Commit deps msg + cause<br/>tag + push with tags<br/>report: published ✅]
    N7 --> N8[Published version feeds<br/>next topological level]
```

Guard rails:

- **`git add .` (or `-A`, or any path beyond `go.mod`/`go.sum`) never runs on a
  dependent** — a dirty tree (e.g. a repo with WIP like `tinywasm/sse`) only
  ever gets a pathspec-limited `git add go.mod go.sum`. Developer WIP is never
  swept into a deps commit
  ([`TestUpdateDependentModule_DirtyTreeCommitsOnlyGoModAndSum`](../../test/dependents_guard_test.go)).
- The dirty check uses `WorkTreeDirtyBeyond` — `.env` and `.gitignore` are
  always ignored, same rule as `HasPendingChanges`
  ([`TestWorkTreeDirtyBeyond`](../../test/dependents_guard_test.go)).
- Nodes skipped by CODEJOB/other-replaces keep today's semantics: `go.mod` is
  updated locally, push is skipped, nothing propagates downstream.
- **Commit message** is deterministic, built by `BuildDepsCommitMessage`
  ([`TestBuildDepsCommitMessage`](../../test/commit_message_test.go)):
  ```
  deps: update router to v0.1.3

  cause: feat: rutas con parámetros opcionales   ← root gopush message, propagated

  - github.com/tinywasm/router v0.1.2 → v0.1.3
  ```

## Output behavior

### Real-time console output (streaming, as each completes)

**Install** prints a single summary line:
```
✅ Installed: gotest, gopush, codejob
```

**Cascade nodes** print one line per node (result only):
```
📦 mylib → published v0.3.1 ✅
📦 sse → deps only (dirty tree) ⚠
📦 otherlib → tests failed ❌
📦 leaflib → skipped (no published upstreams) ⏭
```

### Cascade report (end of cascade)

```
📦 Cascade report
  mylib    → published v0.3.1 ✅
  sse      → deps only (dirty tree) ⚠
  otherlib → failed: tests ❌
  leaflib  → skipped (upstream failed) ⏭
```

### Final summary (single line, main package only)

The summary does NOT include install details or cascade results:
```
vet ✅, race ✅, tests ✅, coverage: 52.7%, Tag: v1.2.3, Pushed ✅, Backup ✅
```
