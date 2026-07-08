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
    I -- Yes --> K[git commit + push<br/>skip tag, tests, deps, backup, verify]
    K --> L[Dispatch PLAN.md to agent]
    L --> M[Save session to .env]
    B -- Yes --> N{CODEJOB_PR in .env?}
    N -- No --> O[Error: no pending PR]
    N -- Yes --> CP[CheckoutPRBranch]
    CP -- Failure --> O
    CP -- Success --> P[Merge PR + delete branch]
    P --> Q[git pull]
    Q --> R[Cleanup: delete CHECK_PLAN.md,<br/>clean .env]
    R --> S{PLAN.md exists?}
    S -- Yes --> K
    S -- No --> T[gopush 'msg' tag<br/>full: deps + backup]
    T --> U[Done]
    M --> U
```

## Traceability (Test Map)

| Diagram Edge / Branch | Test Name |
|---|---|
| CheckoutPRBranch: stash/pop success | `TestCheckoutPRBranch_DirtyTreeSuccess` |
| CheckoutPRBranch: pop conflict | `TestCheckoutPRBranch_PopConflict` |
| HandleDone: Success path | `TestHandleDone_HappyPath` |
| HandleDone: Failure path (retryable) | `TestHandleDone_Retryability` |
| MergeAndPublish: Checkout failure | `TestMergeAndPublish_Guard` |

## Usage

```bash
codejob                        # dispatch PLAN.md or check session status
codejob 'commit message'       # close loop: merge PR, publish, auto-tag
codejob 'commit message' v1.0  # close loop with specific tag
```
