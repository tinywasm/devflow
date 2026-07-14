# codejob Flow

Orchestrator for dispatching coding tasks to external AI agents and closing the loop.

```mermaid
flowchart TD
    A[codejob args] --> GH[Ensure GH Session<br/>PAT recovery]
    GH --> B{Message provided?}
    B -- No --> C{CODEJOB phase?}
    C -- running --> D[Query agent session state]
    D --> E{PR ready?}
    E -- No --> F[Print status]
    E -- Yes --> G[CheckoutPRBranch:<br/>fetch, stash, checkout,<br/>verify, pop stash]
    G -- Success --> G1[HandleDone:<br/>rename PLAN → CHECK_PLAN,<br/>state → review]
    G -- Failure --> G2[Print hints,<br/>state preserved]
    G2 --> U
    G1 --> U
    C -- review --> CP
    C -- none --> H{API key exists?}
    H -- No --> H1[Run setup wizard]
    H1 --> I
    H -- Yes --> I{PLAN.md exists?}
    I -- No --> J[Error: no plan]
    I -- Yes --> IV{Valid frontmatter?<br/>PLAN: required}
    IV -- No --> JV[Error: invalid/missing<br/>plan frontmatter]
    IV -- Yes --> K[git commit + push<br/>skip tag, tests, deps, backup, verify]
    K --> L[Dispatch PLAN.md to agent]
    L --> M[Save state: CODEJOB=driver:running:id]
    B -- Yes --> N{CODEJOB phase == review?}
    N -- No --> O[Error: no pending PR]
    N -- Yes --> CP[CheckoutPRBranch]
    CP -- Failure --> O
    CP -- Success --> P[Merge PR + delete branch]
    P --> Q[git pull]
    Q --> RM[Read CHECK_PLAN.md frontmatter<br/>message + tag]
    RM --> R[Cleanup: delete CHECK_PLAN.md,<br/>clean .env]
    R --> S{PLAN.md exists?}
    S -- Yes --> K
    S -- No --> T[gopush: msg/tag =<br/>CLI value or plan frontmatter<br/>full: deps + backup]
    T --> U[Done]
    M --> U
```

The close-loop commit message is **not** hardcoded: it comes from the finished
plan's `CHECK_PLAN.md` frontmatter (`PLAN:`, optional `TAG:`), unless an
explicit CLI value overrides it. Because dispatch (`IV`) rejects a `PLAN.md`
without valid frontmatter, the message always exists by the time the loop closes
— the old generic `chore: merge agent PR` no longer occurs.

> El árbol sucio que `CheckoutPRBranch` absorbe con stash/pop es ahora, por construcción,
> únicamente WIP del desarrollador: la cascada de `gopush` no toca un repo con sesión
> activa, y `gopush` se niega a publicar en él.

## Traceability (Test Map)

| Diagram Edge / Branch | Test Name |
|---|---|
| CheckoutPRBranch: stash/pop success | `TestCheckoutPRBranch_DirtyTreeSuccess` |
| CheckoutPRBranch: pop conflict | `TestCheckoutPRBranch_PopConflict` |
| HandleDone: Success path | `TestHandleDone_HappyPath` |
| HandleDone: Failure path (retryable) | `TestHandleDone_Retryability` |
| MergeAndPublish: Checkout failure | `TestMergeAndPublish_Guard` |
| Dispatch rejects invalid plan frontmatter (`IV -- No`) | `test/codejob_test.go` |
| Close-loop message = plan frontmatter unless CLI override | `TestResolvePublishMessage_*` (`test/merge_message_test.go`) |

## Plan frontmatter

Every `docs/PLAN.md` must start with a frontmatter block; `PLAN` is required
and becomes the close-loop commit message, `TAG` is optional:

```markdown
---
PLAN: "feat: topological dependency cascade and dirty-tree guard"
TAG: v0.4.41
---
```

## Usage

```bash
codejob                        # dispatch PLAN.md, or check status / auto-merge a pending PR
                               #   (close-loop message taken from the plan frontmatter)
codejob 'commit message'       # close loop with an explicit message override
codejob 'commit message' v1.0  # close loop with explicit message + tag override
```
