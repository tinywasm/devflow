# gopush Flow

Universal build+publish pipeline. Detects `go.mod` to choose between plain git push or full Go workflow.

```mermaid
flowchart TD
    A[gopush 'msg' tag] --> B{go.mod exists?}
    B -- No --> C[git add + commit + push<br/>tag if !skipTag]
    C --> M[Backup]
    M --> D[Done: print summary]
    B -- Yes --> E[Run gotest]
    E --> F{Tests pass?}
    F -- No --> G[Exit 1]
    F -- Yes --> H[git add + commit + push<br/>tag if !skipTag]
    H --> I{cmd/ exists?}
    I -- Yes --> J[Install binaries<br/>print each ✅ to console]
    I -- No --> K{skipDependents?}
    J --> K
    K -- No --> L{Dependents found?}
    L -- No --> M
    L -- Yes --> L0[Launch parallel workers<br/>print each result to console]
    L0 --> L1[Remove replace directive]
    L1 --> L2[go get + go mod tidy]
    L2 --> L3{Other replaces?}
    L3 -- No --> L4[gopush: with tests<br/>skip deps, backup]
    L4 --> L7{Tests pass?}
    L7 -- Yes --> L8[Print: 📦 dep ✅ updated]
    L7 -- No --> L9[Print: 📦 dep ❌ dirty state]
    L3 -- Yes --> L5[Print: 📦 dep ⏭ skip push]
    L8 --> L6{More dependents?}
    L9 --> L6
    L5 --> L6
    L6 -- Yes --> L1
    L6 -- No --> M
    K -- Yes --> M
    M --> D
```

## Output behavior

### Real-time console output (streaming, as each completes)

**Install** prints a single summary line:
```
✅ Installed: gotest, gopush, codejob
```

**Dependents** print one line per dependent (result only):
```
📦 deploy → skip (other replaces) ⏭
📦 mylib → updated ✅
📦 otherlib → tests failed ❌
```

### Final summary (single line, main package only)

The summary does NOT include install details or dependent results:
```
vet ✅, race ✅, tests ✅, coverage: 52.7%, Tag: v1.2.3, Pushed ✅, Backup ✅
```
