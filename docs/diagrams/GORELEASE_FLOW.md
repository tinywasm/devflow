# gorelease Flow

Extensión de [GOPUSH_FLOW.md](GOPUSH_FLOW.md): corre el pipeline completo de gopush
y luego crea un GitHub Release con binarios cross-platform para todos los `cmd/`.

```mermaid
flowchart TD
    A[gorelease 'msg' tag] --> P[ParseCLIArgs]
    P --> PH{help / sin args?}
    PH -- Sí --> USAGE[print usage\nExit 0]
    PH -- No --> B[listCmdDirs cmd/]
    B --> BE{vacío?}
    BE -- Sí --> BEE[Exit 1: no cmd/ found]
    BE -- No --> C[g.Push msg tag\ngopush completo]
    C --> CF{Push ok?}
    CF -- No --> CE[Exit 1: propagar error]
    CF -- Yes --> D[createdTag]
    D --> T[os.MkdirTemp gorelease-*]
    T --> E[crossCompile\ncmds × plataformas\nCGO_ENABLED=0]
    E --> E1[cmd1-linux-amd64\ncmd2-linux-amd64\n...]
    E --> E2[cmd1-darwin-arm64\ncmd2-darwin-arm64\n...]
    E --> E3[cmd1-windows-amd64.exe\ncmd2-windows-amd64.exe\n...]
    E1 & E2 & E3 --> EF{Build ok?}
    EF -- No --> EE[Exit 1: build error\ndefer RemoveAll]
    EF -- Yes --> F[gh release create tag\nassets N×3]
    F --> FF{Release ok?}
    FF -- No --> FE[Exit 1: gh error\ndefer RemoveAll]
    FF -- Yes --> G[defer RemoveAll tmpDir]
    G --> H[✅ Release → URL]
```

## Output

```
vet ✅, race ✅, tests ✅, coverage: 71%, Tag: v0.2.13, Pushed ✅, Backup ✅
✅ Release → https://github.com/tinywasm/goflare/releases/tag/v0.2.13
```

Tag ya aparece en la primera línea (gopush summary) — no se repite en la segunda.
