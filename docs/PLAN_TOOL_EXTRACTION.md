← Índice: [PLAN.md](PLAN.md)

# Cambio 4 — `devflow` deja de ser una dependencia de librería del ecosistema

> Sub-plan del índice maestro [PLAN.md](PLAN.md).
>
> ⚠️ **Este plan NO es despachable todavía.** Su primer paso lo ejecuta el desarrollador en
> local (crear repos con `gonew`, que el agente no puede hacer). Las etapas marcadas
> **[AGENTE]** se despachan después, cuando los repos nuevos existan y estén publicados.

## 1. El diagnóstico, medido

`devflow` es un **módulo gordo**: un solo `package devflow` en la raíz que mezcla las
herramientas de desarrollo (`gopush`, `gotest`, `codejob`, `gonew`, `gorelease`, `badges`,
`devbackup`, `llmskill`, `goinstall`) con un puñado de utilidades genuinamente reutilizables.
Go no permite importar media función: **quien importa una sola utilidad se traga el módulo
entero y todo su `go.mod`**:

```
go-keyring, wizard, mcp, gorun, model, term, fetch, json, router, unixid, time, fmt,
dbus, wincred, shellescape
```

Por eso cada release de `devflow` obliga a actualizar repos que no usan ni una línea suya.

### Inventario real (verificado el 2026-07-13)

Seis repos declaran `github.com/tinywasm/devflow v0.4.46` en su `go.mod`. Esto es **todo** lo
que consumen de verdad en código Go:

| Repo | Tipo de dependencia | Símbolos que usa realmente |
|---|---|---|
| **server** | directa | `NewMarkDown` — **y nada más** |
| **client** | directa | `NewMarkDown`, `RunCommandInDir` — **y nada más** |
| **app** | directa | `FindProjectRoot`, `FolderWatcher`, `GitClient`, `GitHubAuthenticator`, `Go`, `GoModInterface`, `GoNew`, `NewFuture`, `NewGit`, `NewGitHub`, `NewGo`, `NewGoModHandler`, `NewGoNew`, `NewMockGitHubAuth`, `NewResolvedFuture`, `PushResult`, `ReplaceEntry` |
| **website** | **indirecta** (vía `client`) | ninguno |
| **deploy** | **indirecta** (vía `client`) | ninguno |
| **goflare** | **indirecta** (vía `client` y `server`) | ninguno |

**El hallazgo que decide el plan:** `server` y `client` arrastran los quince paquetes de
arriba **para llamar a dos funciones**. Y `website`, `deploy` y `goflare` heredan `devflow`
sin usarlo, solo por depender de `client`. Es decir: **cinco de los seis repos dejan de
depender de `devflow` extrayendo dos archivos.**

### Superficie extraíble: toda de stdlib pura

Verificado archivo por archivo — ninguno importa nada fuera de la stdlib de Go, así que los
repos nuevos nacen con `go.mod` sin dependencias:

| Archivo(s) en devflow | Imports | Consumido por |
|---|---|---|
| `markdown.go`, `markdown_extractor.go`, `markdown_updater.go` | `errors`, `fmt`, `strconv`, `strings`, `path/filepath` | server, client, y el propio `ReadPlanMeta` de devflow |
| `executor.go` | `fmt`, `os/exec`, `runtime`, `strings`, `time` | client, y **todo** devflow |
| `future.go` | *(ninguno)* | app |

## 2. Plan de corte — en dos oleadas, no en una

### Oleada A — barata y de máximo impacto (2 repos nuevos)

| Repo nuevo | Contenido | Libera a |
|---|---|---|
| `tinywasm/markdown` | `markdown.go` + `markdown_extractor.go` + `markdown_updater.go` | **server**, **client** |
| `tinywasm/command` | `executor.go` | **client**, y todo `devflow` por dentro |

Resultado: `server` y `client` borran `devflow` de su `go.mod` → **`website`, `deploy` y
`goflare` pierden la dependencia indirecta automáticamente**. Cinco repos limpios con dos
extracciones de stdlib pura.

### Por qué `command` y no `shell`

**Siete de las nueve funciones de `executor.go` no usan ningún shell**: llaman a
`exec.Command(name, args...)`, que ejecuta un binario con el `argv` ya troceado. No hay
`sh -c`, ni globs, ni `$VAR`, ni `;`/`|`. **Eso es la virtud del paquete, no un detalle**:
`Run("git", "commit", "-m", msg)` es inmune a inyección aunque `msg` traiga `; rm -rf /`,
precisamente porque no pasa por un shell. Un repo llamado `shell` instala el modelo mental
contrario e invita a que alguien escriba `shell.Run("rm -rf " + input)` esperando semántica
de shell.

Las dos funciones que **sí** abren un shell son, además, la parte muerta del archivo:
`RunShellCommand` no la llama **nadie** en todo el ecosistema, y `RunShellCommandAsync` tiene
**una** llamada (`devbackup.go:76`).

Descartados: `tinywasm/exec` (el nombre de paquete `exec` choca con `os/exec` dentro del
propio paquete y en cualquier llamador que también lo importe → alias forzado);
`tinywasm/run` (se confunde con el `tinywasm/gorun` ya existente); `tinywasm/proc` (sugiere
introspección de procesos).

### API del repo `command` (nombres definitivos)

Al mover el código se elimina el tartamudeo `command.RunCommand*` y se limpian dos deudas:

| Hoy en `devflow` | En `tinywasm/command` |
|---|---|
| `ExecCommand` (var de mockeo) | `Exec` |
| `RunCommand` | `Run` |
| `RunCommandSilent` | **ELIMINADA** — es literalmente `return RunCommand(...)`, un alias idéntico (su propio comentario dice *"or we can remove it"*). Sus ~15 llamadas pasan a `Run`. |
| `RunCommandInDir` | `RunInDir` |
| `RunCommandWithRetryInDir` | `RunWithRetry` |
| `RunCommandWithStdin` | `RunWithStdin` |
| `RunShellCommand` | **ELIMINADA** — cero llamadas en todo el ecosistema. |
| `RunShellCommandAsync` | `RunShellAsync` — la única que abre un shell, y su nombre lo dice. |

### Oleada B — `app` (decisión pendiente, NO la ejecutes aún)

`app` sí usa `devflow` en serio: es el gestor de proyectos Go (git, `go.mod`, GitHub,
`gonew`). Extraerlo es un repo `tinywasm/goproject` con `git_handler.go`, `go_mod.go`,
`github.go`, `github_auth.go`, `gonew.go` y `future.go`.

**Hay un nudo que resolver antes**, y por eso esta oleada no se planifica todavía: `app`
importa el tipo `devflow.Go` (`handler.go:42`) pero **solo lo usa para
`ModExistsInCurrentOrParent()` y `SetRootDir()`** — no para publicar. Sin embargo `go_handler.go`
es el cerebro de `gopush` (Push, cascada, opositores) y no se puede mover a `goproject` sin
arrastrar la cascada entera. Las salidas posibles (a decidir con datos, no aquí):

1. `app` deja de depender del tipo `Go` y usa una interfaz mínima propia (`ModExists`), con
   lo que `go_handler.go` se queda en `devflow`.
2. Se parte `go_handler.go` en un núcleo de consulta (`goproject`) y el publicador (`devflow`).

La opción 1 huele mejor (`app` solo necesita dos métodos), pero requiere mirar `GoNew`, que
recibe el `*Go` como colaborador. **Se planifica cuando la oleada A esté fusionada.**

## 3. Etapa 0 — ✅ HECHA (2026-07-13): repos creados y publicados

Ambos repos existen, están en verde y **no tienen ninguna dependencia**:

| Repo | Versión | Cobertura | API |
|---|---|---|---|
| `github.com/tinywasm/markdown` | **v0.0.2** | 91.7% | `markdown.New(rootDir, destination, writeFile) *Doc` → `InputPath`/`InputByte`/`InputEmbed`, `Extract`, `UpdateSection`, `Frontmatter`; y `markdown.ParseFrontmatter(content) (map[string]string, error)` |
| `github.com/tinywasm/command` | **v0.0.2** | 96.2% | `command.Run`, `RunInDir`, `RunWithStdin`, `RunWithRetry`, `RunShellAsync`; var `command.Exec` (seam de mockeo) |

Verificado descargándolos del proxy público desde un módulo limpio.

### Cambio de contrato del frontmatter — LÉELO ANTES DE LA ETAPA 1

`tinywasm/markdown` parsea el frontmatter de forma **genérica**: devuelve
`map[string]string` y **no exige ninguna clave**. Decidir qué claves son obligatorias es
competencia de quien define el esquema del documento — o sea, de `devflow`, no de la librería.

Reparto de responsabilidades tras la extracción:

| Vive en `tinywasm/markdown` | Se queda en `devflow` (`frontmatter.go`) |
|---|---|
| `ParseFrontmatter` → `map[string]string` | `PlanMeta{Message, Tag}` |
| `Fence`, `ErrFrontmatterMissing`, `ErrFrontmatterUnclosed` (errores **estructurales**) | `ReadPlanMeta`, `ErrFrontmatterNoMessage`, `frontmatterHelp`, `ResolvePublishMessage` |

Es decir: `devflow.ParseFrontmatter` pasa a ser una **capa fina** sobre la de `markdown` que
mapea el `map` a `PlanMeta` y aplica la regla "`message` es obligatorio", envolviendo los
errores estructurales con el `frontmatterHelp` (que es texto específico de `docs/PLAN.md` y
por eso no viaja a la librería).

⚠️ Esto **se solapa con el [cambio 2](PLAN_FRONTMATTER_KEY.md)** (`message:` → `PLAN:`), que
toca exactamente esa capa. Haz uno u otro primero, pero no los despaches a la vez.

### Dos limpiezas ya aplicadas en el repo nuevo

- `RunShellCommand` (síncrona) **no existe** en `tinywasm/command`: no la llamaba nadie.
- El alias `BADGES` → `BADGES_SECTION` de `UpdateSection` **no existe**: era código muerto
  (ni siquiera el test lo usaba).

## 4. Etapa 1 — [AGENTE] `devflow` consume las librerías nuevas

**Archivos**: `go.mod`, `markdown.go`, `markdown_extractor.go`, `markdown_updater.go`,
`executor.go`, y todos los que llamen a esas funciones.

1. `go get github.com/tinywasm/markdown@v0.0.2 github.com/tinywasm/command@v0.0.2`
2. **Borra** de `devflow` los cuatro archivos extraídos y sus tests
   (`test/markdown_extractor_test.go`, `test/executor_test.go`, y los tests de markdown que se
   hayan llevado). Verificable: `ls markdown*.go executor.go` → no existe ninguno.
3. Sustituye cada llamada por la del paquete nuevo, **aplicando los renombrados de la tabla de
   §2**. Los puntos de uso interno son abundantes (`RunCommand*` se usa en casi todo el
   paquete) — resuélvelos con el compilador, no a ojo: `go build ./...` hasta que no quede
   ningún símbolo sin resolver.
   En particular, las ~15 llamadas a `RunCommandSilent` pasan **todas** a `command.Run`: la
   función desaparece, no se recrea un alias local.
   Verificable: `grep -rn "RunCommandSilent\|RunShellCommand\b" --include="*.go" .` → vacío.
4. `frontmatter.go` ([`frontmatter.go`](../frontmatter.go)) se reescribe como la **capa fina**
   descrita en §3: `ReadPlanMeta` usa `markdown.New(...).InputPath(...).Frontmatter()`, que
   devuelve un `map[string]string`, y `devflow.ParseFrontmatter` mapea ese `map` a `PlanMeta`
   exigiendo `message` y envolviendo los errores estructurales de `markdown` con
   `frontmatterHelp`. Los tests de frontmatter de `devflow` (en
   `test/markdown_extractor_test.go`) se conservan **con sus expectativas intactas**: siguen
   probando `PlanMeta` y el error de clave obligatoria, que ahora los produce esta capa.
5. `ExecCommand` es la **variable de mockeo** de los tests ([`executor.go:12`](../executor.go)).
   Al moverse pasa a llamarse `command.Exec`, y los tests de `devflow` que hoy hacen
   `devflow.ExecCommand = func(...)` pasan a `command.Exec = func(...)`. Es un cambio mecánico
   de paquete, **no** una nueva forma de mockear: no inventes una interfaz.
6. `devbackup.go:76` es el **único** llamador de la variante con shell: pasa a
   `command.RunShellAsync(...)`.

**Criterio**: `go test ./...` en verde y `go.mod` de `devflow` con `markdown` y `command` como
dependencias directas.

## 5. Etapa 2 — ⚠️ CÓDIGO MIGRADO, PENDIENTE DE PUBLICAR

**El cambio ya está hecho en el árbol de trabajo de `server` y `client`** (2026-07-13), con
`go vet` y `go test ./...` en verde en ambos, y **`devflow` ya no aparece en ninguno de los dos
`go.mod`**. Lo que falta es **publicar**, y está deliberadamente bloqueado.

| Repo | Cambio aplicado | Estado |
|---|---|---|
| `server` | `devflow.NewMarkDown` → `markdown.New` (`generator.go:49`) | ✅ migrado, tests verdes · ⏸ **sin publicar** (tag actual v0.2.29) |
| `client` | `devflow.NewMarkDown` → `markdown.New` (`generator.go:58`); `devflow.RunCommandInDir` → `command.RunInDir` (`generator.go:109`) | ✅ migrado, tests verdes · ⏸ **sin publicar** (tag actual v0.6.23) |
| `website`, `deploy`, `goflare` | Ninguno | La línea `devflow // indirect` cae sola con un `go mod tidy` tras publicar `server`/`client` |

### 🔒 Por qué NO se publicó — compuerta real

Publicar `server` o `client` dispara la **cascada de dependientes** de `gopush`, que llega
transitivamente hasta **`goflare-demo`** (`goflare-demo` → `goflare` → `client`/`server`). Y
`goflare-demo` tiene ahora mismo una sesión activa:

```
CODEJOB=jules:8992209174016991995     # trabajo despachado, en curso
 M go.mod                              # árbol ya sucio por una cascada anterior
```

Con el bug del [cambio 1](PLAN_CASCADE_GUARD.md), la cascada le **mutaría el `go.mod` otra
vez** antes de decidir saltárselo, justo mientras el agente trabaja sobre un PR que reescribe
ese mismo archivo. Sería reproducir a propósito la corrupción que el cambio 1 arregla.

Tampoco hay escape manual: **`gopush` no implementa `--no-cascade`** pese a que el diagrama lo
dibuja. Esa carencia se salda en la etapa 3 del cambio 1.

**Publica `server` y `client` con `gopush` en cuanto el [cambio 1](PLAN_CASCADE_GUARD.md) esté
fusionado y reinstalado.** No los publiques antes, y no los publiques con `git` a mano para
esquivar la cascada: los dependientes se quedarían sin actualizar y el problema solo se
aplazaría.

## 6. Criterios de aceptación (oleada A completa)

1. `tinywasm/markdown` y `tinywasm/command` publicados, cada uno con un `go.mod` **sin
   dependencias**.
2. `devflow` compila y pasa `go test ./...` consumiéndolos; ya no contiene `markdown*.go` ni
   `executor.go`.
3. `grep -rn "tinywasm/devflow" server/go.mod client/go.mod` → vacío.
4. Tras un `go mod tidy`, `website`, `deploy` y `goflare` ya no listan `devflow` ni como
   indirecta.
5. Ninguna herramienta CLI (`gopush`, `gotest`, `codejob`, `gonew`, `gorelease`, `badges`,
   `devbackup`, `llmskill`, `goinstall`) cambia de comportamiento.

## 7. Etapas

| # | Etapa | Dónde | Estado |
|---|-------|-------|--------|
| 0 | `gonew` crea `tinywasm/markdown` y `tinywasm/command`; mover código; publicar | local | ✅ **v0.0.2 ambos** (2026-07-13) |
| 1 | `devflow` importa ambas y borra el código movido | **[AGENTE]** `devflow` | ☐ despachable |
| 2 | `server` y `client` sueltan `devflow` | otros repos | ✅ código migrado y verde · ⏸ **publicación bloqueada por el [cambio 1](PLAN_CASCADE_GUARD.md)** |
| 3 | `tidy` en `website`, `deploy`, `goflare` | otros repos | ☐ (automático tras publicar la etapa 2) |
| B | Planificar la extracción de `goproject` para `app` | pendiente de decisión (§2) | ☐ |

Con la etapa 0 cerrada, la **etapa 1 ya es despachable** como trabajo de agente dentro de
`devflow`. La etapa 2 está escrita pero **no publicada**: espera al cambio 1 (§5).
