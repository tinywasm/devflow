# gopush + CodeJob Integrated Flow

## Overview

When `docs/PLAN.md` exists, `gopush` automatically dispatches the task
to Jules after a successful push. No separate `codejob` invocation is needed.

CodeJob dispatch is embedded inside `GoHandler.Push()` (step 8), so it runs
regardless of whether the caller is the `gopush` CLI or a library consumer.

```mermaid
flowchart TD
    A[gopush 'commit msg'] --> B[GoHandler.Push]
    B --> C[go verify]
    C --> D[gotest]
    D --> E[Git.Push<br/>add → commit → tag → push]
    E --> F[go install cmd/*]
    F --> G[UpdateDependents]
    G --> H[backup async]
    H --> I{PLAN.md exists?<br/>No active CODEJOB session?}
    I -->|No| IO[✅ Return push summary]
    I -->|Yes| J[git pre-flight<br/>HasPendingChanges = false<br/>push guarantees clean state]
    J --> K[POST to Jules API<br/>jules.googleapis.com/v1alpha/sessions]
    K --> L{Jules response}
    L -->|200 OK| O[✅ Return push summary<br/>+ Jules session queued<br/>Jules opens a PR when done]
    L -->|Error| P[✅ Return push summary<br/>+ ⚠️ CodeJob warning in summary]
```

## Why the pre-flight always passes

`CodeJob.Send()` calls `git.HasPendingChanges()` before dispatching.
After step 3 (`git.Push()`) completes:

- `git status --porcelain` → empty (all changes committed)
- `IsAheadOfRemote()` → false (just pushed)

There is no race condition — `GoHandler.Push()` is synchronous.

## Why dispatch is inside GoHandler.Push() (not in the CLI)

Previously `TryDispatch` was called only in the `cmd/gopush` and `cmd/push` CLI entry
points. This meant that library consumers calling `goHandler.Push()` directly would
never trigger CodeJob. Embedding the dispatch as step 8 of `GoHandler.Push()` ensures
the behavior is consistent regardless of the call site.

Errors from dispatch are now included in the Push summary string (visible to all callers)
instead of being silently swallowed.
