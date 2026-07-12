# codejob Flow

Orchestrator for dispatching coding tasks to external AI agents and closing the loop.

```mermaid
flowchart TD
    A[codejob args] --> GH[Ensure GH Session<br/>PAT recovery]
    GH --> B{Message provided?}
    B -- No --> C{CODEJOB in .env?}
    C -- Yes --> D[Query agent session state]
    D --> E{PR ready?}
    E -- No --> F[Print status]
    E -- Yes --> G[CheckoutPRBranch:<br/>fetch, stash, checkout,<br/>verify, pop stash]
    G -- Success --> G1[HandleDone:<br/>rename PLAN → CHECK_PLAN,<br/>clean .env]
    G -- Failure --> G2[Print hints,<br/>CODEJOB preserved]
    G2 --> U
    G1 --> U
    C -- No --> C2{CODEJOB_PR in .env?}
    C2 -- Yes --> CP
    C2 -- No --> H{API key exists?}
    H -- No --> H1[Run setup wizard]
    H1 --> I
    H -- Yes --> I{PLAN.md exists?}
    I -- No --> J[Error: no plan]
    I -- Yes --> IV{Valid frontmatter?<br/>message: required}
    IV -- No --> JV[Error: invalid/missing<br/>plan frontmatter]
    IV -- Yes --> K[git commit + push<br/>skip tag, tests, deps, backup, verify]
    K --> L[Dispatch PLAN.md to agent]
    L --> M[Save session to .env]
    B -- Yes --> N{CODEJOB_PR in .env?}
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
plan's `CHECK_PLAN.md` frontmatter (`message:`, optional `tag:`), unless an
explicit CLI value overrides it. Because dispatch (`IV`) rejects a `PLAN.md`
without valid frontmatter, the message always exists by the time the loop closes
— the old generic `chore: merge agent PR` no longer occurs.

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

Every `docs/PLAN.md` must start with a frontmatter block; `message` is required
and becomes the close-loop commit message, `tag` is optional:

```markdown
---
message: "feat: topological dependency cascade and dirty-tree guard"
tag: v0.4.41
---
```

## Usage

```bash
codejob                        # dispatch PLAN.md, or check status / auto-merge a pending PR
                               #   (close-loop message taken from the plan frontmatter)
codejob 'commit message'       # close loop with an explicit message override
codejob 'commit message' v1.0  # close loop with explicit message + tag override
```
