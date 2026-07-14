---
PLAN: "fix: gopush cascade orders and typed outcome"
---

← Índice: [PLAN.md](PLAN.md)

# Cambio 1 — `gopush` no debe tocar un repo que está en manos del agente

> Sub-plan del índice maestro [PLAN.md](PLAN.md). Se despacha vía el flujo CodeJob.
> Ver skill: agents-workflow.

## 1. La corrección, en una frase

**Hoy la cascada de `gopush` edita el `go.mod` del dependiente y *después* pregunta si
tenía derecho a tocarlo.** Este plan invierte ese orden. Eso es todo.

Como efecto colateral se corrige un segundo bug que vive en el mismo sitio: un nodo que
se salta la publicación hoy **propaga su tag viejo** a sus dependientes como si lo
acabara de publicar.

---

## 2. Evidencia y causa raíz

Estado real observado en el repo `goflare-demo` el 2026-07-13:

- `.env` contiene `CODEJOB=jules:8992209174016991995` → hay un trabajo despachado al
  agente, todavía activo.
- `git status --porcelain` muestra ` M go.mod` y ` M go.sum` → el árbol quedó sucio
  **después** del despacho, sin commitear.

La causa está en `UpdateDependentModule` ([`go_handler.go`](../go_handler.go), el
procesador de cada nodo de la cascada). El orden de operaciones actual es:

1. `RemoveReplace` + `gomod.Save()` → **muta `go.mod`**
2. `go get <bump>` → **muta `go.mod` y `go.sum`**
3. `go mod tidy`, `go generate` → **mutan más**
4. **Recién aquí** `ResolvePublishAction(objectors, ctx)` decide `Skip` / `DepsOnly` / `None`
5. Con `ActionSkip` (sesión codejob activa) hace `return` y **deja el repo sucio**

El opositor `CodeJob.ObjectsToPublish` ([`codejob.go:60`](../codejob.go)) **sí** detecta
la sesión activa, pero se le pregunta demasiado tarde. El contrato lo dice literalmente y
la implementación lo incumple — en [`publish_objector.go:8`](../publish_objector.go):

```go
ActionSkip // do not touch the repo at all
```

### Consecuencia 1 — corrupción del loop de codejob

Cuando el agente termina, `codejob` ejecuta `CheckoutPRBranch`: ve el árbol sucio, hace
`git stash push`, cambia a la rama del PR y hace `git stash pop`. Como el plan que
`goflare-demo` tiene despachado **reescribe precisamente `go.mod`**, ese pop del `go.mod`
viejo de la cascada o bien conflictúa (stash colgado, árbol a medias), o bien se aplica
en silencio y mezcla versiones viejas sobre el `go.mod` que el agente acaba de reparar.
La segunda es peor: es inconsistencia silenciosa.

### Consecuencia 2 — la cascada propaga un tag rancio (bug independiente)

En `defaultCascadeProcessor` ([`cascade.go:257`](../cascade.go)), el valor de retorno de
un nodo saltado es el texto `"updated (…, push skipped)"`. Ese texto **no** contiene el
marcador `"deps only"`, así que la función cae hasta la última línea:

```go
return git.GetLatestTag()   // ← devuelve el tag VIEJO del módulo
```

`RunCascade` recibe una versión no vacía que no es "deps only", concluye que el nodo
**publicó**, lo mete en `publishedVersions` y **se lo pasa como bump a sus dependientes**
— que hacen `go get` de un tag que no contiene el cambio. Además lo reporta como
`published ✅`. Doble daño: el repo queda sucio *y* la cascada aguas abajo se envenena.

La raíz de este segundo bug es que el estado del nodo viaja como **texto libre** dentro
del valor de retorno, detectado con `strings.Contains`. La Etapa 1 lo sustituye por un
tipo.

### Qué NO se hace en este plan

- **No se añade ninguna variable a `.env`.** Las únicas claves del flujo siguen siendo
  `CODEJOB` y `CODEJOB_PR`, y el valor de `CODEJOB` sigue siendo `driver:sessionID`.
- **No se toca `CheckoutPRBranch` ni su stash/pop.** Ese mecanismo existe para absorber
  *WIP legítimo del humano*; una vez que la cascada deje de ensuciar el repo, ya no verá
  basura que no sea del usuario. Sus dos tests siguen en verde sin cambios.
- **No se repara `goflare-demo`** (es otro repo; se limpia a mano con
  `git checkout -- go.mod go.sum`).

---

## 3. Contexto para el ejecutor — léelo antes de tocar código

- `tinywasm/devflow` es **tooling de backend**: usa la stdlib de Go con toda legitimidad.
  **NO** apliques aquí reglas del ecosistema WASM — no "corrijas" imports de `fmt`,
  `strings`, `os/exec`, etc.
- **TDD estricto**: en cada etapa escribe primero los tests, comprueba que fallan
  (`go test ./test/ -run <Nombre>`), y solo entonces implementa hasta ponerlos en verde.
- **No cambies las expectativas de ningún test existente.** Los cuatro mocks de
  `SetCascadeProcessFn` y un `Errorf` de `TestUpdateDependentModule` necesitan un ajuste
  **mecánico** de tipos (§4.3) — eso no es cambiar una expectativa, es adaptar la firma.
  Todo lo demás debe seguir pasando tal cual, en particular:
  `TestUpdateDependentModule_DirtyTreeCommitsOnlyGoModAndSum`, `TestRunCascade_*`,
  `TestResolvePublishAction`, `TestCheckoutPRBranch_*`, `TestHandleDone_*`.
- **Cero strings mágicos**: cada mensaje de error nuevo es una constante exportada del
  paquete `devflow`. Prohibidos los literales en la lógica.
- **`cmd/` no se toca** en ninguna etapa. Toda la lógica es de librería, exportada y
  testeable.
- Ejecuta los tests con `go test ./...`. **No ejecutes `gopush` ni `codejob`** — son
  herramientas locales del desarrollador y se gestionan fuera de este plan.

---

## 4. Etapa 1 — Preguntar antes de mutar (y tipar el resultado de la cascada)

**Archivos**: [`cascade.go`](../cascade.go), [`go_handler.go`](../go_handler.go),
`test/cascade_test.go`, `test/dependents_guard_test.go`, `test/go_handler_test.go`.

### 4.1 El tipo nuevo (elimina los marcadores por substring)

En `cascade.go`, junto a las constantes `CascadeStatus*` que ya existen:

```go
// CascadeOutcome is the typed result of processing one node. It replaces the
// previous convention of encoding the status inside a free-form string.
type CascadeOutcome struct {
    Status  string // CascadeStatusPublished | CascadeStatusDepsOnly | CascadeStatusSkipped
    Version string // set only when Status == CascadeStatusPublished
    Reason  string // human-readable, e.g. "codejob session active"
}
```

Cambia las dos firmas que hoy devuelven `(string, error)`:

```go
type CascadeProcessFn func(node CascadeNode, bumps []DepBump, rootCause string) (CascadeOutcome, error)

func (g *Go) UpdateDependentModule(depDir string, bumps []DepBump, rootCause string) (CascadeOutcome, error)
```

**Prohibido** volver a usar `strings.Contains` sobre el resultado para deducir el estado.
Criterio verificable: `grep -rn "CascadeStatusDepsOnly)" --include="*.go" .` no debe
aparecer dentro de ningún `strings.Contains`.

### 4.2 Tests primero (rojo)

En `test/dependents_guard_test.go` — los tres comparten la misma forma: se captura el
`go.mod` y el `git status --porcelain` del fixture **antes** de la llamada, y se exige que
sean **idénticos byte a byte** después.

- `TestUpdateDependentModule_ActiveSessionLeavesRepoUntouched`
  Fixture: repo dependiente con `go.mod` (incluyendo un `replace` del módulo a
  actualizar), `go.sum` y un `.env` con `CODEJOB=jules:test-session`.
  Asserts: sin error; `go.mod` intacto (el `replace` sigue ahí); `git status` intacto;
  `outcome.Status == CascadeStatusSkipped`; `outcome.Reason == ObjectionCodejobSession`;
  `outcome.Version == ""`.
- `TestUpdateDependentModule_OtherReplacesLeavesRepoUntouched`
  Igual, pero la objeción viene de `GoModHandler`: un `replace` hacia **otro** módulo
  local distinto del que se actualiza, y **sin** `.env`. Mismos asserts, con
  `Reason == ObjectionOtherReplaces`.
- `TestUpdateDependentModule_UpToDateLeavesRepoUntouched`
  Fixture ya en la versión destino y **sin** ningún `replace`. Asserts: sin error;
  `go.mod` y `git status` intactos; `outcome.Status == CascadeStatusSkipped`.
  (Hoy este camino hace `Save()` antes de comprobar la versión y puede dejar el `go.mod`
  reescrito sin commitear — otra fuga de árbol sucio, §4.4 paso 3.)

En `test/cascade_test.go`:

- `TestRunCascade_SkippedNodeDoesNotPropagate`
  Grafo A → B → C. Con `SetCascadeProcessFn`, el mock devuelve para B
  `CascadeOutcome{Status: CascadeStatusSkipped, Reason: ObjectionCodejobSession}`.
  Asserts: B se reporta con `Status == CascadeStatusSkipped`; **C se reporta
  `CascadeStatusSkipped` con detalle `"no upstream bumps"`** — es decir, de B **no** salió
  ninguna versión hacia abajo. Este test es el que blinda el bug del tag rancio.

### 4.3 Ajuste mecánico de los tests existentes

Al cambiar las firmas hay que adaptar (solo la construcción del valor de retorno y su
lectura, **nunca** los asserts):

- Los cuatro mocks de `SetCascadeProcessFn` en `test/cascade_test.go`
  (`TestRunCascade_ChainPropagatesBumpsAndCause`,
  `TestRunCascade_DiamondProcessesNodeOnceWithAllBumps`,
  `TestRunCascade_FailureCutsOnlyItsBranch`,
  `TestRunCascade_DepsOnlyNodeDoesNotPropagate`): pasan a devolver
  `CascadeOutcome{Status: CascadeStatusPublished, Version: "v1.0.1"}` o
  `CascadeOutcome{Status: CascadeStatusDepsOnly}` en lugar del string equivalente.
- `TestUpdateDependentModule` en `test/go_handler_test.go`: su `t.Errorf(..., result)`
  usa `%s` sobre lo que ahora es un struct — cámbialo a `%+v`. Sus dos asserts reales
  (error contiene `go get failed`, el `replace` fue eliminado) **no cambian**.

### 4.4 Implementación (verde)

Reordena `UpdateDependentModule` así:

```
1. os.Stat(go.mod)                        ← igual que hoy
2. Construir gomod + git rooteados en depDir y RESOLVER LA OBJECIÓN   ← MOVIDO AQUÍ
       if action == ActionSkip → return CascadeOutcome{Skipped, Reason: reason}, nil
          (el repo NO se toca: ni Save, ni go get, ni tests)
3. Comprobar si ya está en la versión destino Y no hay replace que quitar
       → return CascadeOutcome{Skipped, Reason: "already up-to-date"}, nil   (repo intacto)
4. Mutar: RemoveReplace + Save + go get + go mod tidy + go generate
5. gotest (gate)  → si falla: revertir go.mod/go.sum y devolver error
6. Commit según la acción YA resuelta en el paso 2 (DepsOnly / None)
```

Detalles que el ejecutor **no** debe improvisar:

- Los tres opositores (`gomod`, `git`, `CodeJob{}`) son **de solo lectura**, por eso es
  correcto consultarlos antes de mutar. `HasOtherReplaces(ctx.ModulePath)` ya excluye el
  `replace` del propio módulo en actualización ([`go_mod.go:217`](../go_mod.go)), así que
  su veredicto es idéntico antes y después de la mutación. Lo mismo vale para el opositor
  `Git`: `WorkTreeDirtyBeyond(git, "go.mod", "go.sum")` ignora justamente los dos archivos
  que la mutación tocaría.
- **Resuelve la acción UNA sola vez** (paso 2) y **reutiliza** `action` y `reason` en el
  paso 6. No vuelvas a llamar a `ResolvePublishAction`.
- Paso 3: `RemoveReplace` devuelve un `bool`. Solo llama a `Save()` si devolvió `true`.
  Si el módulo ya está en la versión destino **y** no se quitó ningún replace, el repo
  queda literalmente intacto.
- El texto `"updated (…, push skipped)"` desaparece.
  Verificable: `grep -rn "push skipped" --include="*.go" .` → vacío.

En `cascade.go`:

- `defaultCascadeProcessor`: propaga el `CascadeOutcome` de `UpdateDependentModule` tal
  cual. Solo cuando **todos** los bumps salieron con `Status == CascadeStatusPublished`
  consulta `git.GetLatestTag()` para rellenar `Version`. **Nunca** llames a
  `GetLatestTag()` en los caminos `Skipped` ni `DepsOnly` — esa llamada es exactamente el
  origen del tag rancio.
- `RunCascade`: registra la `CascadeEntry` a partir de `outcome.Status`, y solo escribe en
  `publishedVersions[node.ModulePath]` cuando `Status == CascadeStatusPublished` (y
  `Version != ""`). `Skipped` y `DepsOnly` no propagan nada.
  El `Detail` de la entrada sale de `outcome.Reason` (o de `outcome.Version` si publicó).

---

## 5. Etapa 2 — `gopush` se niega a publicar un repo con `CODEJOB` activo

Es el guardarraíl directo: si hay un trabajo despachado, el repo está en manos del
agente, y publicar movería la rama base por debajo de él (haciendo que el PR conflictúe
al fusionar).

**Archivos**: [`go_handler.go`](../go_handler.go), `test/go_handler_test.go`.

### 5.1 Tests primero (rojo)

- `TestGoPush_BlockedByActiveCodejobSession`
  Fixture: repo con `.env` conteniendo `CODEJOB=jules:test-session` y un cambio pendiente
  cualquiera. `Push(...)` devuelve un error cuyo mensaje contiene
  `ErrPushBlockedActiveCodejob`, y **no se crea ningún commit** (`git log` igual que
  antes de la llamada).
- `TestGoPush_NotBlockedByCodejobPR`
  Fixture con `.env` que tiene **solo** `CODEJOB_PR=https://github.com/o/r/pull/1` (sin
  `CODEJOB`). `Push(...)` **no** devuelve el error de bloqueo (puede fallar por otras
  razones del fixture, pero su mensaje no contiene `ErrPushBlockedActiveCodejob`).

### 5.2 Implementación (verde)

Constante exportada en `go_handler.go` (texto verbatim):

```go
// ErrPushBlockedActiveCodejob is returned by Push when the repo has an active codejob
// session: publishing would move the base branch under the agent.
const ErrPushBlockedActiveCodejob = "gopush blocked: active codejob session (CODEJOB in .env) — the repo is under agent control; run 'codejob' to check status and close the loop before publishing"
```

En `Go.Push`, justo **después** de `ValidateCommitMessage` y **antes** de
`HasPendingChanges` (para cubrir por igual el pipeline Go y el universal no-Go):

```go
if HasActiveCodejobSession(g.rootDir) {
    return PushResult{}, errors.New(ErrPushBlockedActiveCodejob)
}
```

Usa `errors.New`, **no** `fmt.Errorf` — `go vet` marca el format string no constante.

**Anti-footgun crítico**: la guarda mira **solo** `CODEJOB`, jamás `CODEJOB_PR`.
`codejob` borra `CODEJOB` en `HandleDone` y deja `CODEJOB_PR` durante la fase de revisión;
en esa fase `MergeAndPublish` llama internamente a `publisher.Publish`, que es este mismo
`Push`. Si la guarda mirase `CODEJOB_PR`, **`codejob` se bloquearía a sí mismo al cerrar
el loop**. `HasActiveCodejobSession` ([`go_handler.go:53`](../go_handler.go)) ya tiene la
semántica correcta y su test lo fija (`TestHasActiveCodejobSession`) — úsala tal cual, no
escribas una comprobación nueva.

**Relación con la Etapa 1**: dentro de la cascada esta guarda es redundante (la Etapa 1
ya impide llegar a `Push` en un nodo con sesión activa). Es intencional: la Etapa 1 protege
la cascada, la Etapa 2 protege al humano que ejecuta `gopush` a mano. **No borres ninguna
de las dos** por parecer duplicadas.

---

## 6. Etapa 3 — El flag `--no-cascade` que el diagrama promete y el CLI no tiene

[`GOPUSH_FLOW.md`](diagrams/GOPUSH_FLOW.md) documenta un rombo
`{skipDependents or --no-cascade?}`, pero **`cmd/gopush/main.go` no implementa ese flag**:
`grep -rn "no-cascade" cmd/gopush/main.go` → vacío. La librería sí acepta el parámetro
(`Go.Push(..., skipDependents, ...)`); simplemente no hay forma de activarlo desde la línea de
comandos.

Esto no es cosmético. El 2026-07-13 hizo falta publicar `server` y `client` mientras
`goflare-demo` tenía una sesión `CODEJOB` activa, y **no hubo forma segura de hacerlo con la
herramienta**: cualquier `gopush` habría arrastrado la cascada hasta ese repo. Un escape
manual (`--no-cascade`) es la válvula que faltaba.

**Archivos**: `cmd/gopush/main.go`, [`cli.go`](../cli.go), `test/cli_test.go`.

### 6.1 Tests primero (rojo)

- `TestParseCLIArgs_NoCascadeFlag` — `gopush 'msg' --no-cascade` devuelve el mensaje `msg`
  **sin** el flag entre los argumentos posicionales, y señala que la cascada va desactivada.
  Sigue el patrón que ya usa `--skip-race` / `-R` en `cmd/gopush/main.go`: el flag se filtra de
  `os.Args` **antes** de `ParseCLIArgs`, para no contaminar los posicionales.
- `TestParseCLIArgs_NoCascadeAbsent` — sin el flag, la cascada queda activada (comportamiento
  actual, por defecto).

### 6.2 Implementación (verde)

En `cmd/gopush/main.go`, junto al filtrado que ya existe para `--skip-race`:

```go
if arg == "--no-cascade" {
    noCascade = true
    continue
}
```

y pásalo como el parámetro `skipDependents` de `Go.Push` (que **ya existe** — no cambies su
firma). Añade la línea al bloque `usage()`:

```
    --no-cascade   Publish this module only; do not update dependent modules
```

**Anti-footgun**: `--no-cascade` es un escape manual, **no** un sustituto de las guardas de las
etapas 1 y 2. Las guardas protegen del error; el flag sirve para cuando el desarrollador
*decide* no propagar. No hagas que una implique la otra.

## 7. Etapa 4 — Diagramas y documentación (compuerta final)

Los diagramas son el contrato; deben reflejar exactamente lo implementado.

### [`docs/diagrams/GOPUSH_FLOW.md`](diagrams/GOPUSH_FLOW.md)

1. **Pipeline principal**: tras `A[gopush 'msg' tag]`, inserta el rombo
   `{Active CODEJOB session?}`. Rama `Yes → Exit 1: gopush blocked (repo under agent control)`;
   rama `No` continúa al flujo actual.
2. **Diagrama por nodo**: invierte el orden. `resolvePublishAction` (nodo `NR`) pasa a ser
   **el primer nodo**, antes del de actualización (`N2`). La rama `Skip` sale directamente
   de `NR` a `⏭ repo untouched — nothing mutated, no tests, no propagation`.
   Elimina el estado "updated, push skipped": ya no existe.
3. **Guard rails**: reescribe el bullet de Skip →
   `Skip nodes (active CODEJOB session, other replaces): the repo is NOT touched at all — no go.mod write, no go get, no tests. Nothing propagates downstream.`
   Añade uno nuevo →
   `A skipped or deps-only node never contributes a version to the next topological level (it used to leak its stale tag as if freshly published).`
4. **Tabla "Contract → tests"**: añade filas para
   `TestUpdateDependentModule_ActiveSessionLeavesRepoUntouched`,
   `TestUpdateDependentModule_OtherReplacesLeavesRepoUntouched`,
   `TestUpdateDependentModule_UpToDateLeavesRepoUntouched`,
   `TestRunCascade_SkippedNodeDoesNotPropagate` y
   `TestGoPush_BlockedByActiveCodejobSession`.
   Añade también la fila del contrato tipado: *"Node result is a typed `CascadeOutcome`;
   status is never inferred from substrings"*.

### [`docs/diagrams/CODEJOB_FLOW.md`](diagrams/CODEJOB_FLOW.md)

El flujo de `codejob` **no cambia** — no hay nodos nuevos. Añade bajo el diagrama una nota
corta explicando por qué ya no hace falta ninguna verificación de deriva ahí:

> El árbol sucio que `CheckoutPRBranch` absorbe con stash/pop es ahora, por construcción,
> únicamente WIP del desarrollador: la cascada de `gopush` no toca un repo con sesión
> activa, y `gopush` se niega a publicar en él.

### [`docs/CODEJOB.md`](CODEJOB.md) y [`docs/GOPUSH.md`](GOPUSH.md)

- `CODEJOB.md`: en la tabla "`codejob` vs `gopush`", añade una línea indicando que
  `gopush` **falla** si hay una sesión `CODEJOB` activa, y que la salida es cerrar el loop
  con `codejob`.
- `GOPUSH.md`: alinea la descripción de la semántica `Skip` de la cascada (repo intacto) y
  documenta el rechazo del pipeline raíz.

4. **`GOPUSH_FLOW.md`**: el rombo `K{skipDependents or --no-cascade?}` deja de ser una promesa
   vacía — ahora el flag existe. Documenta `--no-cascade` en la sección de uso.

---

## 8. Criterios de aceptación

1. `go test ./...` en verde, con los tests nuevos y **todos** los existentes pasando.
2. `grep -rn "push skipped" --include="*.go" .` → vacío.
3. `grep -rn "EnvKeyCodejob" --include="*.go" .` sigue devolviendo **solo**
   `EnvKeyCodejob` y `EnvKeyCodejobPR` — ninguna clave nueva de `.env`, ningún cambio en
   el formato `driver:sessionID`.
4. Ninguna deducción de estado por substring: no existe ningún `strings.Contains` sobre el
   resultado de un nodo de cascada.
5. `GetLatestTag()` no se invoca en los caminos `Skipped` ni `DepsOnly` de
   `defaultCascadeProcessor`.
6. `gopush 'msg' --no-cascade` publica el módulo sin tocar ningún dependiente.
7. Sin cambios en `codejob.go` ni en `codejob_state.go`. El único archivo de `cmd/` que se toca
   es `cmd/gopush/main.go`, y solo para filtrar el flag nuevo (la lógica sigue en la librería).

## 9. Etapas

| # | Etapa | Archivos | Estado |
|---|-------|----------|--------|
| 1 | Preguntar antes de mutar + `CascadeOutcome` tipado | `cascade.go`, `go_handler.go`, `test/cascade_test.go`, `test/dependents_guard_test.go`, `test/go_handler_test.go` | ☐ |
| 2 | `gopush` rechaza con `CODEJOB` activo | `go_handler.go`, `test/go_handler_test.go` | ☐ |
| 3 | Flag `--no-cascade` (el diagrama ya lo promete) | `cmd/gopush/main.go`, `cli.go`, `test/cli_test.go` | ☐ |
| 4 | Diagramas y docs | `docs/diagrams/GOPUSH_FLOW.md`, `docs/diagrams/CODEJOB_FLOW.md`, `docs/CODEJOB.md`, `docs/GOPUSH.md` | ☐ |

Las etapas 1, 2 y 3 son independientes (cualquier orden). La 4 es una **compuerta final**:
requiere las tres anteriores terminadas.

## 10. Qué desbloquea este cambio

`server` y `client` **ya tienen migrado el código** para soltar `devflow` (ver
[PLAN_TOOL_EXTRACTION.md](PLAN_TOOL_EXTRACTION.md) §5), pero **no se han publicado**: hacerlo
hoy con `gopush` arrastraría la cascada hasta `goflare-demo`, que tiene una sesión `CODEJOB`
activa, y le volvería a ensuciar el `go.mod`. En cuanto este cambio esté fusionado, esa
publicación es segura.
