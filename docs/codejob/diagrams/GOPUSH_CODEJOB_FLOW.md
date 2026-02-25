# gopush + CodeJob Integrated Flow

## Overview

When `docs/PLAN.md` exists, `gopush` automatically dispatches the task
to Jules after a successful push. No separate `codejob` invocation is needed.

```mermaid
flowchart TD
    A[gopush 'commit msg'] --> B[GoHandler.Push]
    B --> C[go verify]
    C --> D[gotest]
    D --> E[Git.Push<br/>add → commit → tag → push]
    E --> F[go install cmd/*]
    F --> G[UpdateDependents]
    G --> H[backup async]
    H --> I{Push succeeded?}
    I -->|No| IE[❌ Print error and exit 1]
    I -->|Yes| J{PLAN.md<br/>exists locally?}
    J -->|No| JO[✅ Print push summary]
    J -->|Yes| K[CodeJob.Send]
    K --> L[git pre-flight<br/>HasPendingChanges = false<br/>push guarantees clean state]
    L --> M[POST to Jules API<br/>jules.googleapis.com/v1alpha/sessions]
    M --> N{Jules response}
    N -->|200 OK| O[✅ Push summary<br/>✅ Jules session queued<br/>Jules opens a PR when done]
    N -->|Error| P[✅ Push summary<br/>⚠️ Jules dispatch failed]
```

## Why the pre-flight always passes after gopush

`CodeJob.Send()` calls `git.HasPendingChanges()` before dispatching.
After `gopush` completes successfully:

- `git status --porcelain` → empty (all changes committed)
- `IsAheadOfRemote()` → false (just pushed)

There is no race condition — `GoHandler.Push()` is synchronous.
