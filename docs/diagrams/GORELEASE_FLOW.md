# gorelease Flow

Release-only workflow: reads an existing tag from git and creates a GitHub Release with
cross-compiled binaries. No tags are created, no commits made.

```mermaid
flowchart TD
    A[gorelease tag?] --> P[ParseReleaseArgs]
    P --> PH{help?}
    PH -- Yes --> USAGE[print usage\nExit 0]
    PH -- No --> B[listCmdDirs cmd/]
    B --> BE{vacío?}
    BE -- Yes --> BEE[Exit 1: no cmd/ found]
    BE -- No --> C{tag provided?}
    C -- Yes --> D[use explicit tag]
    C -- No --> E[git.GetLatestTag]
    E --> EE{empty?}
    EE -- Yes --> EE1[Exit 1: no tags found]
    EE -- No --> D
    D --> T[os.MkdirTemp gorelease-*]
    T --> F[crossCompile\ncmds × plataformas\nCGO_ENABLED=0]
    F --> F1[cmd1-linux-amd64\ncmd2-linux-amd64\n...]
    F --> F2[cmd1-darwin-arm64\ncmd2-darwin-arm64\n...]
    F --> F3[cmd1-windows-amd64.exe\ncmd2-windows-amd64.exe\n...]
    F1 & F2 & F3 --> FF{Build ok?}
    FF -- No --> FE[Exit 1: build error\ndefer RemoveAll]
    FF -- Yes --> R[resolvePublishTarget\nlee origin: owner + visibility]
    R --> RU{visibilidad\ndeterminada?}
    RU -- No --> G[gh release create tag\ntarget: origin\nassets N×3]
    RU -- Sí --> RV{origin público?}
    RV -- Sí --> G
    RV -- No, es privado --> RD[deriva owner/nombre-carpeta-local\nej: tinywasm/app]
    RD --> RC{existe y es público?}
    RC -- No --> RCE[Exit 1: no hay repo público\ndonde publicar]
    RC -- Sí --> G2[gh release create tag\n--repo owner/carpeta\nassets N×3]
    G & G2 --> GF{Release ok?}
    GF -- No --> GE[Exit 1: gh error\ndefer RemoveAll]
    GF -- Yes --> H[defer RemoveAll tmpDir]
    H --> I[✅ Release → URL]
```

## Resolución automática del repo de publicación

`gorelease` no recibe flags. Decide solo:

```
visibilidad indeterminada → publica en origin (fallback compatible)
origin público            → publica en origin (comportamiento clásico)
origin privado            → publica en  <owner-de-origin>/<nombre-carpeta-local>
                            (verifica que exista y sea público; si no, falla)
```

| Repo | Visibilidad | Rol |
|------|-------------|-----|
| `tinywasm/core` | Privado | fuente + tags + historial (= `origin`) |
| `tinywasm/app` | Público | releases con binarios (carpeta local = `app`) |

Estás físicamente en `~/Dev/Project/tinywasm/app`, `origin` es `tinywasm/core` (privado).
Como no se puede publicar un release útil en privado, `gorelease` deriva `tinywasm/app`
del owner de `origin` + el nombre de la carpeta local, y publica ahí.

## Output

```
✅ Release → https://github.com/tinywasm/app/releases/tag/v0.2.13
```

## With codejob `-release` flag

`codejob 'msg' -release` runs the normal close-loop (gopush), then calls `gorelease`:

```mermaid
flowchart TD
    A[codejob msg -release] --> P[ParseCLIArgs + detect -release flag]
    P --> B[MergeAndPublish]
    B --> C{RE_DISPATCH?}
    C -- Yes --> D[clean up, re-dispatch]
    C -- No --> E{-release flag?}
    E -- Yes --> F[releaseFn<br/>goHandler.ReleaseOnly tag]
    E -- No --> G[done]
    F --> H{Release ok?}
    H -- Yes --> G
    H -- No --> I[error but summary shown]
```

Tag creado por `gopush` en MergeAndPublish es usado inmediatamente por `gorelease`.
