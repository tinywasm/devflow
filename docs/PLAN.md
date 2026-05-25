> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan: tinywasm/devflow — Herramienta `gorelease`

## Context

`gopush` publica un módulo Go: tests → commit → tag → push → instala binarios locales →
actualiza dependientes. Lo que no hace es crear un **GitHub Release** con binarios
cross-platform para distribución pública.

`gorelease` extiende ese flujo: corre `gopush` completo (vía `g.Push()`), luego compila
los binarios para las plataformas objetivo y crea el GitHub Release con los assets.

### Relación con código existente

| Necesidad | Solución existente | Acción |
|---|---|---|
| Detectar y listar `cmd/` | `Go.Install()` — itera `cmd/` con `os.ReadDir` | Extraer `listCmdDirs(rootDir) []string` |
| Parsear args CLI (msg, tag, help) | `cmd/gopush/main.go` — lógica inline | Extraer `ParseCLIArgs(args []string) (msg, tag string, ok bool)` a `cli.go` |
| Correr gopush completo | `Go.Push(...)` | Llamar directamente |
| Crear GitHub Release | `GitHub` struct + `gh` CLI | Agregar `CreateRelease` a `github.go` |
| Auth GitHub | `GitHubAuth` + OAuth Device Flow | Sin cambios — reutilizar |
| Binarios temporales | `os.MkdirTemp` | Sin estado en repo, sin `.gitignore` |

## Múltiples `cmd/`

Si el repo tiene `cmd/gopush/`, `cmd/gotest/`, `cmd/gorelease/`, se compilan **todos**
para todas las plataformas:

```
cmd/gopush   × [linux/amd64, darwin/arm64, windows/amd64] = 3 binarios
cmd/gotest   × [linux/amd64, darwin/arm64, windows/amd64] = 3 binarios
cmd/gorelease × [linux/amd64, darwin/arm64, windows/amd64] = 3 binarios
→ 9 assets subidos al GitHub Release
```

Misma lógica que `Install()` — itera `cmd/`, filtra solo directorios.

## Plataformas objetivo

| GOOS    | GOARCH | Sufijo                     |
|---------|--------|----------------------------|
| linux   | amd64  | `<name>-linux-amd64`       |
| darwin  | arm64  | `<name>-darwin-arm64`      |
| windows | amd64  | `<name>-windows-amd64.exe` |

`CGO_ENABLED=0` obligatorio — cross-compilation sin CGO.

## Flujo completo

```
gorelease 'msg' [tag]
  └─ ParseCLIArgs → msg, tag
     └─ listCmdDirs("cmd/") → [gopush, gotest, ...]  ← error si vacío
        └─ g.Push(msg, tag, ...)   ← gopush completo
           └─ createdTag
              └─ os.MkdirTemp("", "gorelease-*")
                 └─ crossCompile(tmpDir, cmdDirs, targets)  ← todos × todas plataformas
                    └─ gh.CreateRelease(createdTag, assets)
                       └─ defer os.RemoveAll(tmpDir)
                          └─ print: ✅ Release → URL
```

Ver diagrama: [diagrams/GORELEASE_FLOW.md](diagrams/GORELEASE_FLOW.md)

## Casos de éxito y sus tests

### Caso 1: repo con un solo `cmd/name/`

**Input:** `gorelease 'feat: x'` en repo con `cmd/goflare/`
**Esperado:**
- gopush ejecuta tests, commit, tag (ej. v0.3.0), push
- 3 binarios compilados: `goflare-linux-amd64`, `goflare-darwin-arm64`, `goflare-windows-amd64.exe`
- `gh release create v0.3.0 --title v0.3.0 --notes "" goflare-linux-amd64 ...` ejecutado
- Output: `vet ✅, tests ✅, Tag: v0.3.0, Pushed ✅, Backup ✅\n✅ Release → <url>`

**Test:** `TestGoRelease_SingleCmd`
```go
// Mock: RunCommand intercepta "gh release create" → verifica tag + 3 assets
// Mock: Go.Push → retorna PushResult{Tag: "v0.3.0"}
// Mock: listCmdDirs → ["goflare"]
// Mock: crossCompile → crea 3 archivos en tmpDir (no llama go build real)
// Assert: CreateRelease recibe tag="v0.3.0" y len(assets)==3
```

### Caso 2: repo con múltiples `cmd/`

**Input:** `gorelease 'chore: release'` en repo con `cmd/gopush/`, `cmd/gotest/`
**Esperado:**
- 6 binarios compilados (2 cmds × 3 plataformas)
- `gh release create` recibe 6 assets
- Output: `✅ Release → <url>`

**Test:** `TestGoRelease_MultipleCmd`
```go
// Mock: listCmdDirs → ["gopush", "gotest"]
// Mock: crossCompile → crea 6 archivos
// Assert: len(assets)==6
// Assert: assets contiene "gopush-linux-amd64" y "gotest-linux-amd64"
```

### Caso 3: tag explícito provisto

**Input:** `gorelease 'fix: bug' v1.0.0`
**Esperado:**
- gopush usa `v1.0.0` como tag (no auto-genera)
- Release creado con tag `v1.0.0`

**Test:** `TestGoRelease_ExplicitTag`
```go
// Assert: g.Push recibe tag="v1.0.0"
// Assert: CreateRelease recibe tag="v1.0.0"
```

### Caso 4: crossCompile — binario nombrado correctamente

**Test:** `TestCrossCompile_NamingConvention`
```go
// Crea tmpDir real, mock "go build" exitoso
// Assert: linux/amd64 → "<cmd>-linux-amd64"
// Assert: windows/amd64 → "<cmd>-windows-amd64.exe"
// Assert: darwin/arm64 → "<cmd>-darwin-arm64"
// Assert: CGO_ENABLED=0 en env de cada go build
```

### Caso 5: CreateRelease — args correctos a gh

**Test:** `TestCreateRelease_Args`
```go
// Mock RunCommand captura args
// Llama gh.CreateRelease("v1.0.0", []string{"/tmp/a", "/tmp/b"})
// Assert: args == ["release", "create", "v1.0.0", "--title", "v1.0.0", "--notes", "", "/tmp/a", "/tmp/b"]
// Assert: retorna URL del output de gh
```

## Casos de error y sus tests

| Caso | Error esperado | Test |
|---|---|---|
| Sin `cmd/` | `"no cmd/ found — gorelease requires at least one binary in cmd/"` | `TestGoRelease_NoCmdDir` |
| `cmd/` existe pero vacío | mismo error | `TestGoRelease_EmptyCmdDir` |
| `g.Push()` falla (tests fallan) | propaga error de gopush, no compila | `TestGoRelease_PushFails` |
| `go build` falla para una plataforma | error con GOOS/GOARCH en mensaje | `TestCrossCompile_BuildFails` |
| `gh release create` falla | error, tmpDir limpiado igualmente | `TestCreateRelease_GhFails` |

**Invariante de limpieza:** `defer os.RemoveAll(tmpDir)` garantiza que el directorio
temporal se elimina en todos los casos — éxito, error, panic. Verificable con
`TestGoRelease_TmpDirAlwaysCleanedUp`.

## Reutilización: `ParseCLIArgs` y `listCmdDirs`

### `ParseCLIArgs` (mover de `cmd/codejob/main.go` a `cli.go`)

`cmd/codejob/main.go` ya tiene `parseArgs` — es la implementación más limpia de las tres:

```go
func parseArgs(args []string) (message, tag string, isHelp bool)
```

`cmd/gopush/main.go` y `cmd/gotest/main.go` tienen variantes inline sin extraer y con
flags distintos (`-help` falta en gopush, `-?` falta en gotest). Son la misma lógica
con tres implementaciones divergentes.

**Acción:** mover `parseArgs` de `cmd/codejob/main.go` a `cli.go` como función exportada
`ParseCLIArgs` con los flags unificados del codejob (más completos):
`help`, `-help`, `--help`, `h`, `-h`, `?`, `-?`.

Luego reemplazar las tres implementaciones inline por la llamada a `ParseCLIArgs`.

**Help flags unificados** (superconjunto de los tres actuales):
`help`, `-help`, `--help`, `h`, `-h`, `?`, `-?` → `isHelp=true`.

### `listCmdDirs` (nuevo, `go_handler.go`)

Extrae la lógica de detección de `cmd/` de `Install()`:

```go
// listCmdDirs retorna los nombres de los subdirectorios en cmd/.
// Retorna slice vacío si cmd/ no existe.
func listCmdDirs(rootDir string) ([]string, error)
```

`Install()` y `crossCompile()` usan `listCmdDirs` — sin duplicar `os.ReadDir`.

## Archivos a crear/editar

### Stage 1 — `cli.go` (nuevo) + refactor de `cmd/*/main.go`

Mover `parseArgs` de `cmd/codejob/main.go` → `ParseCLIArgs` exportada en `cli.go`.
Agregar tests en `cli_test.go`.
Reemplazar implementaciones inline en `cmd/gopush/main.go` y `cmd/gotest/main.go`
por la llamada a `ParseCLIArgs`. Eliminar `parseArgs` inline de `cmd/codejob/main.go`.

### Stage 2 — `go_handler.go` (editar)

Extraer `listCmdDirs` de `Install()`. `Install()` llama `listCmdDirs` internamente.

### Stage 3 — `gorelease.go` (nuevo)

Funciones:
- `(g *Go) Release(msg, tag string) error` — orquesta todo
- `crossCompile(tmpDir string, cmds []string, targets []crossTarget) ([]string, error)`
- `crossTarget` struct: `{GOOS, GOARCH string}`

### Stage 4 — `github.go` (editar)

Agregar `CreateRelease(tag string, assets []string) (url string, error)`.

### Stage 5 — `cmd/gorelease/main.go` (nuevo)

CLI entry point usando `ParseCLIArgs`. Sin lógica de parsing inline.

### Stage 6 — `docs/diagrams/GORELEASE_FLOW.md` (actualizar)

Diagrama Mermaid actualizado con múltiples cmd/ y limpieza garantizada.

## Stages Summary

| # | Archivo | Acción |
|---|---|---|
| 1 | `cli.go` | Nuevo — `ParseCLIArgs` compartido |
| 2 | `go_handler.go` | Extraer `listCmdDirs` de `Install()` |
| 3 | `gorelease.go` | Nuevo — `Release`, `crossCompile`, `crossTarget` |
| 4 | `github.go` | Agregar `CreateRelease` |
| 5 | `cmd/gorelease/main.go` | Nuevo — entry point sin parsing inline |
| 6 | `docs/diagrams/GORELEASE_FLOW.md` | Actualizar con flujo completo |

## Tests: `ParseCLIArgs` (`cli_test.go`)

`TestParseArgs_HelpFlags` — todos los flags de ayuda retornan `isHelp=true`:
`["help"]`, `["-help"]`, `["--help"]`, `["h"]`, `["-h"]`, `["?"]`, `["-?"]`.

`TestParseArgs_MessageOnly` — `["fix: bug"]` → `msg="fix: bug"`, `tag=""`, `isHelp=false`.

`TestParseArgs_MessageAndTag` — `["fix: bug", "v1.2.3"]` → `msg="fix: bug"`, `tag="v1.2.3"`.

`TestParseArgs_Empty` — `[]` → `msg=""`, `tag=""`, `isHelp=false` (el main decide mostrar usage).

## Verification

```bash
gotest
```

Tests unitarios sin binarios reales: mocks de `RunCommand`, `Go.Push`, `listCmdDirs`.
`TestCrossCompile_NamingConvention` puede usar `go build` real si TinyGo no es
requerido (los binarios de devflow son stdlib).
`TestGoRelease_TmpDirAlwaysCleanedUp` verifica que el tmpDir no existe después de
cualquier camino de ejecución.
