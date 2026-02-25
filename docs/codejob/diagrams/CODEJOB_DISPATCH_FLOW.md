# CodeJob Dispatch Flow

## Local — `codejob [path]`

```mermaid
flowchart TD
    A[codejob docs/PLAN.md] --> B{File exists<br/>and not empty?}
    B -->|No| BE[❌ Error: file not found or empty]
    B -->|Yes| GIT{Repo in sync<br/>with remote?}
    GIT -->|No| GITE[❌ Error: repo not in sync with remote<br/>Jules reads from GitHub not local]
    GIT -->|Yes| C{jules_api_key<br/>in keyring?}
    C -->|No| CE[❌ Error: run codejob init first]
    C -->|Yes| D[Build prompt reference<br/>Execute the implementation plan<br/>described in path]
    D --> E[POST to Jules API<br/>jules.googleapis.com/v1alpha/sessions]
    E --> F{Response}
    F -->|200 OK| G[→ Jules: SESSION_ID]
    G --> SAVE[Write CODEJOB=jules:ID to .env]
    SAVE --> DONE[✅ Jules session queued]
```

