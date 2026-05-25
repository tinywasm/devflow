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
| Parsear args CLI (msg, tag, help) | `cmd/codejob/main.go:parseArgs` — ya extraída, más completa | Mover a `cli.go` como `ParseCLIArgs` exportada |
| Correr gopush completo | `Go.Push(...)` | Llamar directamente |
| Crear GitHub Release | `GitHub` struct + `SecretRunner` | Agregar `CreateRelease` a `github.go` usando `getSecretRunner().Run(...)` |
| Auth GitHub | `GitHubAuth` + OAuth Device Flow | Sin cambios — reutilizar |
| Binarios temporales | `os.MkdirTemp` | Sin estado en repo, sin `.gitignore` |

## Múltiples `cmd/`

Si el repo tiene `cmd/gopush/`, `cmd/gotest/`, `cmd/gorelease/`, se compilan **todos**
para todas las plataformas:

```
cmd/gopush    × [linux/amd64, darwin/arm64, windows/amd64] = 3 binarios
cmd/gotest    × [linux/amd64, darwin/arm64, windows/amd64] = 3 binarios
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
              └─ tmpDir = os.MkdirTemp("", "gorelease-*")
                 defer os.RemoveAll(tmpDir)           ← registrado aquí, ejecuta al retornar
                 └─ CrossCompile(tmpDir, cmdDirs, DefaultTargets())
                    └─ gh.CreateRelease(createdTag, assets)
                       └─ print: ✅ Release → URL
```

Ver diagrama: [diagrams/GORELEASE_FLOW.md](diagrams/GORELEASE_FLOW.md)

## Firmas de funciones exportadas

```go
// gorelease.go
func (g *Go) Release(msg, tag string, gh *GitHub) error

// CrossCompile y DefaultTargets exportadas — necesarias para TestCrossCompile_NamingConvention
func CrossCompile(tmpDir string, cmds []string, targets []CrossTarget) ([]string, error)
func DefaultTargets() []CrossTarget

type CrossTarget struct{ GOOS, GOARCH string }

// campo inyectable en Go struct (go_handler.go) para tests
crossCompileFn func(tmpDir string, cmds []string, targets []CrossTarget) ([]string, error)
// nil = usa CrossCompile real

// github.go
func (gh *GitHub) CreateRelease(tag string, assets []string) (string, error)
// usa gh.getSecretRunner().Run("gh", args...) — igual que SetSecret usa RunWithStdin
```

## Ubicación y mocks de tests

**Ubicación:** `test/gorelease_test.go` — `package devflow_test`.

### Mocks disponibles (reutilizar sin crear nuevos)

| Mock | Definido en | Accesible desde gorelease_test.go |
|---|---|---|
| `MockGitClient` | `test/go_handler_test.go` | ✅ mismo package `devflow_test` |
| `MockPublisher` | `test/helpers_test.go` | ✅ mismo package `devflow_test` |
| `fakeRunner` | `test/github_secrets_test.go` | ✅ mismo package `devflow_test` |
| `newTestGitHub` | `test/github_secrets_test.go` | ✅ mismo package `devflow_test` |
| `MockGitHubAuth` | `mock_github_auth.go` (paquete `devflow`) | ✅ importado vía `devflow.NewMockGitHubAuth()` |

Todos los `_test.go` en `test/` compilan juntos como `package devflow_test` — tipos y
helpers definidos en cualquiera de ellos son visibles en los demás sin redeclarar.

### Nuevo mock necesario: `fakeCrossCompile`

Campo `crossCompileFn` en `Go` struct (definido en `go_handler.go`):

```go
// En go_handler.go — Go struct agrega:
crossCompileFn func(tmpDir string, cmds []string, targets []CrossTarget) ([]string, error)
```

`Release` lo usa: si `g.crossCompileFn != nil` → llama `g.crossCompileFn`; si no → `CrossCompile`.

En tests:
```go
goHandler.SetCrossCompileFn(func(tmpDir string, cmds []string, _ []devflow.CrossTarget) ([]string, error) {
    // crea archivos vacíos en tmpDir para simular binarios
    var assets []string
    for _, cmd := range cmds {
        p := filepath.Join(tmpDir, cmd+"-linux-amd64")
        os.WriteFile(p, []byte{}, 0644)
        assets = append(assets, p)
        // repetir para darwin-arm64 y windows-amd64.exe
    }
    return assets, nil
})
```

Exportar setter `SetCrossCompileFn` para que sea accesible desde `package devflow_test`.

### Helper de test: `testCreateCmdDirs`

```go
// testCreateCmdDirs crea un módulo Go temporal con subdirectorios en cmd/.
// cmds vacío → crea el módulo pero NO crea cmd/ (caso "sin cmd/").
// withEmptyCmd=true → crea cmd/ vacío sin subdirectorios (caso "cmd/ vacío").
func testCreateCmdDirs(t *testing.T, cmds ...string) (dir string, cleanup func())
```

Internamente crea `go.mod` mínimo para que `NewGo` no falle. Ubicación: `test/helpers_test.go`.

Para el caso "cmd/ vacío":
```go
dir, cleanup := testCreateCmdDirs(t)   // sin cmd/
os.MkdirAll(filepath.Join(dir, "cmd"), 0755)  // cmd/ existe pero vacío
```

## Casos de éxito y sus tests

Todos en `test/gorelease_test.go`, `package devflow_test`.

### Caso 1: repo con un solo `cmd/name/`

**Test:** `TestGoRelease_SingleCmd`
```go
dir, cleanup := testCreateCmdDirs(t, "goflare")
defer cleanup()
defer testChdir(t, dir)()

mockGit := &MockGitClient{createdTag: "v0.3.0"}
goHandler, _ := devflow.NewGo(mockGit)
goHandler.SetCrossCompileFn(fakeCrossCompileFn(t))  // no llama go build real

fake := &fakeRunner{output: "https://github.com/org/repo/releases/tag/v0.3.0"}
gh := newTestGitHub(fake)

err := goHandler.Release("feat: x", "", gh)
// Assert: err == nil
// Assert: fake.lastArgs[0:3] == ["release", "create", "v0.3.0"]
// Assert: cantidad de paths de assets en fake.lastArgs == 3
```

### Caso 2: repo con múltiples `cmd/`

**Test:** `TestGoRelease_MultipleCmd`
```go
dir, cleanup := testCreateCmdDirs(t, "gopush", "gotest")
defer cleanup()
defer testChdir(t, dir)()

mockGit := &MockGitClient{createdTag: "v0.1.0"}
goHandler, _ := devflow.NewGo(mockGit)
goHandler.SetCrossCompileFn(fakeCrossCompileFn(t))

fake := &fakeRunner{output: "https://github.com/.../v0.1.0"}
gh := newTestGitHub(fake)

err := goHandler.Release("chore: release", "", gh)
// Assert: err == nil
// Assert: cantidad de assets en fake.lastArgs == 6  (2 cmds × 3 plataformas)
// Assert: algún arg contiene "gopush-linux-amd64"
// Assert: algún arg contiene "gotest-linux-amd64"
```

### Caso 3: tag explícito provisto

**Test:** `TestGoRelease_ExplicitTag`
```go
// mismo setup mínimo con un cmd/
err := goHandler.Release("fix: bug", "v1.0.0", gh)
// Assert: fake.lastArgs contiene "v1.0.0" en posición del tag
// Assert: MockGitClient.Push fue llamado con tag="v1.0.0"
```

Requiere exponer `lastPushTag` en `MockGitClient` o capturarlo vía `PushFn`.

### Caso 4: CrossCompile — naming convention

**Test:** `TestCrossCompile_NamingConvention`

Único test que llama `CrossCompile` real (usa `go build` stdlib — no TinyGo).
Necesita un módulo temporal mínimo con `cmd/mytool/main.go` y `go.mod`.

```go
repoDir, cleanup := testCreateCmdDirs(t, "mytool")
defer cleanup()

tmpDir, _ := os.MkdirTemp("", "gorelease-naming-*")
defer os.RemoveAll(tmpDir)

// CrossCompile debe recibir repoDir para saber dónde está ./cmd/mytool
assets, err := devflow.CrossCompile(tmpDir, []string{"mytool"}, devflow.DefaultTargets(), repoDir)
// Assert: err == nil
// Assert: algún asset termina en "mytool-linux-amd64"
// Assert: algún asset termina en "mytool-darwin-arm64"
// Assert: algún asset termina en "mytool-windows-amd64.exe"
```

Nota: la firma de `CrossCompile` recibe `repoDir` para ejecutar `go build -o ... ./cmd/<name>`
desde el directorio correcto. Actualizar firma en Stage 3.

### Caso 5: CreateRelease — args correctos a gh

**Test:** `TestCreateRelease_Args`
```go
fake := &fakeRunner{output: "https://github.com/org/repo/releases/tag/v1.0.0"}
gh := newTestGitHub(fake)

url, err := gh.CreateRelease("v1.0.0", []string{"/tmp/a", "/tmp/b"})
// Assert: err == nil
// Assert: fake.lastArgs == ["release", "create", "v1.0.0",
//                           "--title", "v1.0.0", "--notes", "",
//                           "/tmp/a", "/tmp/b"]
// Assert: url == "https://github.com/org/repo/releases/tag/v1.0.0"
```

`CreateRelease` usa `gh.getSecretRunner().Run("gh", args...)` — `fakeRunner.Run` captura
los args sin invocar `gh` real.

## Casos de error y sus tests

Todos en `test/gorelease_test.go`.

| Caso | Error esperado | Setup |
|---|---|---|
| Sin `cmd/` | contiene `"no cmd/ found"` | `testCreateCmdDirs(t)` — sin args, sin cmd/ |
| `cmd/` vacío | mismo error | `testCreateCmdDirs(t)` + `os.MkdirAll(dir+"/cmd", 0755)` |
| `g.Push()` falla | propaga error, `crossCompileFn` no se llama | `MockGitClient{pushErr: errors.New("tests failed")}` |
| `CrossCompile` falla | error con detalle GOOS/GOARCH | `SetCrossCompileFn` retorna error |
| `gh release create` falla | error, tmpDir eliminado | `fakeRunner{err: errors.New("unauthorized")}` |

**Invariante de limpieza — `TestGoRelease_TmpDirAlwaysCleanedUp`:**
```go
// SetCrossCompileFn retorna error
// Captura el tmpDir creado internamente (inyectar hook o leer del error)
// Assert: tmpDir no existe después del error
```

## Reutilización: `ParseCLIArgs` y `listCmdDirs`

### `ParseCLIArgs` (mover de `cmd/codejob/main.go` a `cli.go`)

`cmd/codejob/main.go` ya tiene `parseArgs` — es la implementación más limpia de las tres.
`cmd/gopush/main.go` y `cmd/gotest/main.go` tienen variantes inline con flags distintos
(`-help` falta en gopush, `-?` falta en gotest).

**Acción:** mover a `cli.go` como `ParseCLIArgs` exportada con el superconjunto de flags:
`help`, `-help`, `--help`, `h`, `-h`, `?`, `-?` → `isHelp=true`.

Reemplazar las tres implementaciones inline. Eliminar `parseArgs` de `cmd/codejob/main.go`.

### `listCmdDirs` (nuevo, `go_handler.go`)

```go
// listCmdDirs retorna los nombres de los subdirectorios en cmd/.
// Retorna slice vacío (no error) si cmd/ no existe.
func listCmdDirs(rootDir string) ([]string, error)
```

`Install()` y `Release()` usan `listCmdDirs` — sin duplicar `os.ReadDir`.

## Archivos a crear/editar

### Stage 1 — `cli.go` (nuevo) + `test/cli_test.go` (nuevo) + refactor `cmd/*/main.go`

Mover `parseArgs` → `ParseCLIArgs` exportada en `cli.go`.
Tests en `test/cli_test.go` (`package devflow_test`).
Reemplazar inline en `cmd/codejob`, `cmd/gopush`, `cmd/gotest`.

### Stage 2 — `go_handler.go` (editar)

- Extraer `listCmdDirs` de `Install()`.
- Agregar campo `crossCompileFn` a `Go` struct.
- Agregar `SetCrossCompileFn(fn)` setter exportado.

### Stage 3 — `gorelease.go` (nuevo)

```go
func (g *Go) Release(msg, tag string, gh *GitHub) error
func CrossCompile(tmpDir string, cmds []string, targets []CrossTarget, repoDir string) ([]string, error)
func DefaultTargets() []CrossTarget
type CrossTarget struct{ GOOS, GOARCH string }
```

`Release` usa `g.crossCompileFn` si no es nil, sino `CrossCompile`.

### Stage 4 — `github.go` (editar)

```go
func (gh *GitHub) CreateRelease(tag string, assets []string) (string, error)
// usa gh.getSecretRunner().Run("gh", "release", "create", tag, "--title", tag, "--notes", "", assets...)
```

### Stage 5 — `cmd/gorelease/main.go` (nuevo)

Entry point con `ParseCLIArgs`. Construye `*GitHub` con `devflow.NewGitHub(log)`.
Llama `goHandler.Release(msg, tag, gh)`.

### Stage 6 — `cmd/goinstall/main.go` (editar)

`goinstall` llama `goHandler.Install("")` que itera `cmd/` automáticamente.
`gorelease` quedará incluido sin cambios en código — pero verificar que el README
de `goinstall` lista el nuevo comando.

### Stage 7 — `docs/diagrams/GORELEASE_FLOW.md` (ya existe)

Verificar que refleja `defer` registrado en creación de tmpDir, no como paso secuencial.

### Stage 8 — `docs/GORELEASE.md` (nuevo)

Misma estructura que `docs/GOPUSH.md`: Usage, Arguments, Behavior, Output, Examples,
Exit codes. Enlace a `diagrams/GORELEASE_FLOW.md`.

### Stage 9 — `README.md` (editar)

```markdown
- **[gorelease](docs/GORELEASE.md)** - Publish Go module + create GitHub Release with cross-platform binaries
```

## Stages Summary

| # | Archivo | Acción |
|---|---|---|
| 1 | `cli.go` + `test/cli_test.go` + 3 `cmd/*/main.go` | `ParseCLIArgs` exportada; eliminar inline |
| 2 | `go_handler.go` | `listCmdDirs` + campo `crossCompileFn` + `SetCrossCompileFn` |
| 3 | `gorelease.go` | `Release`, `CrossCompile`, `DefaultTargets`, `CrossTarget` |
| 4 | `github.go` | `CreateRelease` vía `getSecretRunner().Run` |
| 5 | `cmd/gorelease/main.go` | Entry point |
| 6 | `cmd/goinstall/main.go` | Verificar — `Install()` ya itera cmd/, no requiere cambios de código |
| 7 | `docs/diagrams/GORELEASE_FLOW.md` | Verificar consistencia con implementación |
| 8 | `docs/GORELEASE.md` | Documentación del comando |
| 9 | `README.md` | Agregar `gorelease` a la tabla de comandos |

## Tests: `ParseCLIArgs` (`test/cli_test.go`, `package devflow_test`)

`TestParseArgs_HelpFlags` — todos los flags retornan `isHelp=true`:
`["help"]`, `["-help"]`, `["--help"]`, `["h"]`, `["-h"]`, `["?"]`, `["-?"]`.

`TestParseArgs_MessageOnly` — `["fix: bug"]` → `msg="fix: bug"`, `tag=""`, `isHelp=false`.

`TestParseArgs_MessageAndTag` — `["fix: bug", "v1.2.3"]` → `msg="fix: bug"`, `tag="v1.2.3"`.

`TestParseArgs_Empty` — `[]` → `msg=""`, `tag=""`, `isHelp=false`.

## Verification

```bash
gotest
```

- Tests de `ParseCLIArgs`: sin I/O, puramente unitarios.
- Tests de `Release`: mocks vía `MockGitClient` + `SetCrossCompileFn` + `fakeRunner` en `SecretRunner`. Sin red, sin `gh` real.
- `TestCrossCompile_NamingConvention`: llama `go build` real (stdlib, ~1s). Requiere Go en PATH.
- `TestCreateRelease_Args`: `fakeRunner` captura args sin invocar `gh`.
- `TestGoRelease_TmpDirAlwaysCleanedUp`: verifica limpieza de tmpDir en camino de error.
