# devflow — PLAN: Completar rootDir en operaciones de archivo

> Estado: Borrador para revisión · Objetivo: eliminar el uso restante del CWD del proceso
> en operaciones de archivo/comando dentro de `devflow`, dejando solo `g.rootDir` como
> directorio de trabajo. Prerequisito completado: comandos de test ya usan `cmd.Dir`/`RunCommandInDir`
> (v0.4.30).
>
> ⚠️ Prescriptivo. Ver §5 (Invariantes) y §6 (Aceptación).

---

## 1. Contexto (verificado en código)

El dispatch anterior (v0.4.30) corrigió la ejecución de comandos en `gotest.go` y
`git_test_cache.go` para usar `g.rootDir` vía `RunCommandInDir` y `cmd.Dir`. Sin embargo,
quedan operaciones de archivo/comando que aún asumen el CWD del proceso:

| Sitio | Problema |
|-------|---------|
| `go_handler.go:33` `GoVersion()` | `os.ReadFile("go.mod")` — lee del CWD |
| `go_mod.go:412` `Verify()` | `RunCommand("go", "mod", "verify")` — sin `Dir` |
| `badges.go:81` `NewBadges` | `h.readmeFile = "README.md"` — escribe en CWD |
| `badges.go:108` `BuildBadges()` | genera SVGs en CWD |
| `badges.go:188` `UpdateReadme()` | `os.ReadFile(h.readmeFile)` — CWD |

Estas operaciones son invocadas desde `gotest.go` durante el full suite (`run_tests` sin args),
por lo que en el daemon MCP escriben en el directorio del daemon en lugar del proyecto.

## 2. Objetivo

Todas las operaciones de archivo dentro de `devflow` usan `g.rootDir` (o el path configurado).
El CWD del proceso queda libre de efectos secundarios del daemon.

## 3. Diseño

### 3.1 `GoVersion()` — `go_handler.go:32`

```go
// Cambiar:
data, err := os.ReadFile("go.mod")
// Por:
data, err := os.ReadFile(filepath.Join(g.rootDir, "go.mod"))
```

### 3.2 `Verify()` — `go_mod.go:406`

```go
// Cambiar:
output, err := RunCommand("go", "mod", "verify")
// Por:
output, err := RunCommandInDir(g.rootDir, "go", "mod", "verify")
```

`ModExists()` (`:361`) también debe verificar `filepath.Join(g.rootDir, "go.mod")`.

### 3.3 `Badges` — `badges.go`

`Badges` no tiene referencia a `rootDir`. Añadir campo `rootDir string` e inicializarlo:

```go
// NewBadges — añadir parámetro opcional o setter:
func (h *Badges) SetRootDir(dir string) { h.rootDir = dir }
```

- `h.readmeFile` pasa a ser path absoluto: `filepath.Join(h.rootDir, "README.md")`.
- `BuildBadges()` genera SVGs en `h.rootDir` en lugar de CWD.

En `gotest.go`, donde se construye `Badges` para el full suite, pasar `g.rootDir`:

```go
bh := NewBadges(...)
bh.SetRootDir(g.rootDir)
```

### 3.4 `exactCoverageFromProfile` — `gotest.go`

Verificar que `go tool cover` use `RunCommandInDir(g.rootDir, ...)` — si `profilePath` es
absoluto (generado con `os.CreateTemp`), el comando no necesita `Dir`; confirmar y documentar.

## 4. Pasos

1. `go_handler.go`: `GoVersion()` usa `filepath.Join(g.rootDir, "go.mod")`.
2. `go_mod.go`: `Verify()` usa `RunCommandInDir(g.rootDir, ...)`. `ModExists()` verifica path absoluto.
3. `badges.go`: añade `rootDir string` + `SetRootDir(dir string)`. `readmeFile`, `BuildBadges`, `UpdateReadme` usan `h.rootDir`.
4. `gotest.go`: llama `bh.SetRootDir(g.rootDir)` después de construir `Badges`.
5. Verificar `exactCoverageFromProfile`; documentar si ya es correcto.

## 5. Invariantes / prohibiciones

- **No** uses `os.Chdir` (race con el daemon).
- **No** cambies la firma pública de `GoVersion`, `Verify`, `NewBadges` más de lo necesario.
- `g.rootDir` por defecto es `"."` — comportamiento CLI existente sin cambio.
- **No** crees nuevas tools MCP aquí — este plan es solo corrección interna.

## 6. Aceptación

1. `GoVersion()` lee `go.mod` del `rootDir` configurado, no del CWD.
2. `Verify()` corre `go mod verify` en `rootDir`.
3. `UpdateReadme()` escribe en `<rootDir>/README.md`.
4. `go build ./... && go test ./...` verde.
5. Tests existentes de `devflow/test/` pasan sin cambios.

## 7. Tests

Añade a `devflow/test/`:

1. **GoVersion rootDir:** con un módulo temporal (`testCreateGoModule`), `SetRootDir(dir)`,
   aserta que `GoVersion()` no retorna error y lee la versión correcta.
2. **Badges rootDir:** con `SetRootDir(tmpDir)`, aserta que `UpdateReadme()` escribe en
   `tmpDir/README.md`, no en CWD.
3. **Verify rootDir:** con `SetRootDir(tmpModuleDir)`, aserta que `Verify()` no falla por
   CWD incorrecto.
