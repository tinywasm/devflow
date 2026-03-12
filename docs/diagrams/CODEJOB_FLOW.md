# codejob Flow

Orchestrator for dispatching coding tasks to external AI agents and closing the loop.

```mermaid
flowchart TD
    A[codejob args] --> B{Message provided?}
    B -- No --> C{CODEJOB in .env?}
    C -- Yes --> D[Query agent session state]
    D --> E{PR ready?}
    E -- No --> F[Print status]
    E -- Yes --> G[HandleDone:<br/>fetch, checkout branch,<br/>rename PLAN → CHECK_PLAN,<br/>clean .env]
    C -- No --> C2{CODEJOB_PR in .env?}
    C2 -- Yes --> P
    C2 -- No --> H{API key exists?}
    H -- No --> H1[Run setup wizard]
    H1 --> I
    H -- Yes --> I{PLAN.md exists?}
    I -- No --> J[Error: no plan]
    I -- Yes --> K[gopush 'chore: sync'<br/>skip tag, deps, backup]
    K --> L[Dispatch PLAN.md to agent]
    L --> M[Save session to .env]
    B -- Yes --> N{CODEJOB_PR in .env?}
    N -- No --> O[Error: no pending PR]
    N -- Yes --> P[Merge PR + delete branch]
    P --> Q[git pull]
    Q --> R[Cleanup: delete CHECK_PLAN.md,<br/>clean .env]
    R --> S{PLAN.md exists?}
    S -- Yes --> K
    S -- No --> T[gopush 'msg' tag<br/>full: deps + backup]
    T --> U[Done]
    M --> U
    G --> U
```

## Usage

```bash
codejob                        # dispatch PLAN.md or check session status
codejob 'commit message'       # close loop: merge PR, publish, auto-tag
codejob 'commit message' v1.0  # close loop with specific tag
```
