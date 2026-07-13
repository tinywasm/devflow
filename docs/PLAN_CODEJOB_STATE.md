← Índice: [PLAN.md](PLAN.md) · Requiere: [PLAN_CASCADE_GUARD.md](PLAN_CASCADE_GUARD.md)

# Cambio 3 — Una sola variable de `.env` por manejador

> Sub-plan del índice maestro [PLAN.md](PLAN.md). Se despacha vía el flujo CodeJob.
> Ver skill: agents-workflow.
>
> ⚠️ **Compuerta**: este cambio reescribe la misma guarda que introduce el
> [cambio 1](PLAN_CASCADE_GUARD.md). No lo despaches hasta que aquel esté fusionado.

## 1. El problema y la recomendación

Hoy `codejob` esparce **dos** claves por el `.env`:

```
CODEJOB=jules:8992209174016991995          # sesión en curso
CODEJOB_PR=https://github.com/o/r/pull/1   # PR pendiente de revisar
```

Son **fases de una misma máquina de estados**, no dos datos independientes: nunca coexisten
(`HandleDone` borra la primera y escribe la segunda). Modelarlas como dos claves obliga a
todo consumidor a reconstruir mentalmente el estado a partir de qué clave está presente, y
cada consumidor lo hace a su manera — de ahí salió el footgun del cambio 1, donde una guarda
que mirase `CODEJOB_PR` bloquearía a `codejob` contra sí mismo.

### Recomendación: el estado va en el **valor**, no en el nombre de la clave

**Una clave por manejador. El manejador es dueño del formato de su valor.**

```
CODEJOB=jules:running:8992209174016991995
CODEJOB=jules:review:https://github.com/o/r/pull/1
```

Formato: `<driver>:<fase>:<ref>`, partido con `strings.SplitN(raw, ":", 3)` — los dos
puntos de la URL del PR quedan dentro de `ref`, que es el último campo. La `ref` es el
**único dato que la fase necesita**: el ID de sesión mientras corre, la URL del PR mientras
se revisa.

Por qué esta forma y no otras que descarté:

- **JSON en el valor** (`CODEJOB={"phase":"review",...}`): ilegible en un `.env`, con
  comillas que hay que escapar, y abre la puerta a meter más campos "porque cabe".
  Precisamente lo que queremos evitar.
- **Una clave por fase** (lo actual): es el problema.
- **Añadir `CODEJOB_STATE` junto a las otras dos**: más estado, no menos.

El valor es un **tipo**, no un string que cada quien parsea:

```go
type CodejobPhase string

const (
    PhaseRunning CodejobPhase = "running" // agent working; Ref = session ID
    PhaseReview  CodejobPhase = "review"  // PR open, pending merge; Ref = PR URL
)

// CodejobState is the single piece of state the codejob manager persists.
type CodejobState struct {
    Driver string       // "jules"
    Phase  CodejobPhase
    Ref    string       // session ID (running) or PR URL (review)
}

func ParseCodejobState(raw string) (CodejobState, error) // "" → zero value, no error
func (s CodejobState) String() string                    // "jules:review:https://…"
```

Y el manejador es el único que lo toca:

```go
func LoadCodejobState(env *DotEnv) (CodejobState, error)
func SaveCodejobState(env *DotEnv, s CodejobState) error
func ClearCodejobState(env *DotEnv) error   // borra CODEJOB (y el CODEJOB_PR heredado)
```

## 2. Migración de los `.env` que ya existen

Lectura tolerante, escritura estricta. `LoadCodejobState` reconoce los formatos viejos y los
convierte al vuelo; el siguiente `Save` ya escribe el formato nuevo y borra `CODEJOB_PR`.

| `.env` encontrado | Se interpreta como |
|---|---|
| `CODEJOB=jules:running:ID` (3 campos) | Formato nuevo, tal cual |
| `CODEJOB=jules:ID` (2 campos, sin fase conocida) | `{jules, running, ID}` — legado |
| `CODEJOB_PR=<url>` presente (con o sin `CODEJOB`) | `{jules, review, <url>}` — legado; **gana sobre `CODEJOB`** |
| Ninguna de las dos | Estado cero: no hay job |

`CODEJOB_PR` gana porque `HandleDone` escribe `CODEJOB_PR` **después** de borrar `CODEJOB`:
si por un fallo a medias quedaran las dos, la fase real es `review`.

La constante `EnvKeyCodejobPR` se conserva **solo** para leer el legado y borrarlo. Márcala
`// Deprecated: legacy key, read-only for migration.` y no la uses en ninguna escritura.

## 3. Contexto para el ejecutor

- `tinywasm/devflow` es **tooling de backend**: usa la stdlib de Go legítimamente. **NO**
  apliques aquí reglas del ecosistema WASM.
- **TDD estricto**: los tests primero, en rojo, y luego la implementación.
- **Cero strings mágicos**: fases, claves y errores son constantes exportadas.
- No toques `cmd/`. Ejecuta los tests con `go test ./...`; **no ejecutes `gopush` ni
  `codejob`**.

## 4. Etapa 1 — El tipo y su persistencia

**Archivos**: [`codejob.go`](../codejob.go) (tipo + constantes + Load/Save/Clear),
`test/codejob_state_test.go`.

### 4.1 Tests primero (rojo)

- `TestParseCodejobState_NewFormat` — `"jules:review:https://github.com/o/r/pull/1"` →
  `{Driver: "jules", Phase: PhaseReview, Ref: "https://github.com/o/r/pull/1"}`. **La URL
  conserva sus dos puntos** (este es el test que fija el `SplitN(..., 3)`).
- `TestParseCodejobState_Empty` — `""` → valor cero, sin error.
- `TestParseCodejobState_Invalid` — `"basura"` → error `ErrInvalidCodejobState`.
- `TestLoadCodejobState_LegacySessionOnly` — `.env` con `CODEJOB=jules:S1` →
  `{jules, PhaseRunning, "S1"}`.
- `TestLoadCodejobState_LegacyPRWins` — `.env` con `CODEJOB=jules:S1` **y**
  `CODEJOB_PR=<url>` → `{jules, PhaseReview, <url>}`.
- `TestSaveCodejobState_WritesNewFormatAndDropsLegacy` — partiendo de un `.env` legado, tras
  `SaveCodejobState` el archivo tiene `CODEJOB=jules:review:<url>` y **ya no tiene**
  `CODEJOB_PR`.
- `TestClearCodejobState_RemovesBothKeys`.

### 4.2 Implementación (verde)

Tal como se especifica en §1 y §2. `ParseCodejobState` con `strings.SplitN(raw, ":", 3)`;
si hay 2 campos, es legado (`PhaseRunning`); si hay 3 y la fase no es `running` ni `review`,
devuelve `ErrInvalidCodejobState` (verbatim):

```go
var ErrInvalidCodejobState = errors.New("invalid CODEJOB value in .env: expected <driver>:<phase>:<ref>")
```

`SaveCodejobState` escribe `CODEJOB` **y** borra `EnvKeyCodejobPR` en la misma operación.

## 5. Etapa 2 — Migrar a los consumidores

**Archivos**: [`codejob.go`](../codejob.go), [`codejob_state.go`](../codejob_state.go),
[`go_handler.go`](../go_handler.go).

Sustituye toda lectura/escritura directa de las dos claves por el tipo. Los puntos exactos:

| Dónde | Hoy | Pasa a ser |
|---|---|---|
| `CodeJob.Run` — cerrar el loop | `env.Get(EnvKeyCodejobPR)` | `LoadCodejobState` + exigir `Phase == PhaseReview` |
| `CodeJob.Run` — comprobar estado | `env.Get(EnvKeyCodejob)` | `Phase == PhaseRunning` → `checkStatus(state)` |
| `CodeJob.Run` — auto-merge previo | `env.Get(EnvKeyCodejobPR)` | `Phase == PhaseReview` → `MergeAndPublish` |
| `CodeJob.checkStatus` | parsea `driver:sessionID` a mano con `SplitN` | recibe el `CodejobState` ya parseado; **borra ese parseo manual** |
| `CodeJob.Send` | `env.Set(EnvKeyCodejob, driver+":"+id)` | `SaveCodejobState(env, {driver, PhaseRunning, id})` |
| `HandleDone` | `env.Delete(CODEJOB)` + `env.Set(CODEJOB_PR, url)` | `SaveCodejobState(env, {driver, PhaseReview, prURL})` — una sola escritura |
| `MergeAndPublish` / `MergePR` | `env.Get/Delete(CODEJOB_PR)` | `LoadCodejobState` / `ClearCodejobState` |
| `IsEnvironmentValid` | mira las dos claves + `os.Getenv` | `LoadCodejobState(...).Phase != ""` o hay `docs/PLAN.md` |

`grep -rn "EnvKeyCodejobPR" --include="*.go" .` debe quedar reducido a su declaración
(marcada `Deprecated`) y a las dos líneas de migración/borrado dentro de `codejob.go`.

## 6. Etapa 3 — Las guardas (aquí está el footgun)

**Archivos**: [`go_handler.go`](../go_handler.go), [`codejob.go`](../codejob.go),
`test/go_handler_test.go`, `test/publish_objector_test.go`.

Con dos claves, "hay sesión activa" era "existe `CODEJOB`". Con una sola clave, **`CODEJOB`
existe en las dos fases** — así que preguntar por su presencia ya no significa lo mismo. Toda
guarda debe preguntar por la **fase**.

Renombra `HasActiveCodejobSession(dir) bool` → `CodejobPhaseOf(dir) CodejobPhase` y ajusta a
los dos consumidores con reglas **distintas**:

| Guarda | Bloquea en | Motivo |
|---|---|---|
| **`Go.Push`** (raíz, del cambio 1) | `PhaseRunning` **y solo esa** | En `PhaseReview`, `MergeAndPublish` llama internamente a `publisher.Publish` para cerrar el loop. Si la guarda bloqueara `review`, **`codejob` se bloquearía a sí mismo** y sería imposible fusionar nada. |
| **`CodeJob.ObjectsToPublish`** (dependientes, en la cascada) | `PhaseRunning` **y** `PhaseReview` → `ActionSkip` | Aquí sí: durante `review` el árbol local está posicionado en la rama del PR, y una publicación de deps commitearía dentro del PR del agente. Este objetor **nunca** corre sobre el repo raíz, solo sobre dependientes, así que no puede auto-bloquear a `codejob`. |

Es una asimetría deliberada. Escríbela como comentario en ambas funciones.

### Tests (rojo primero)

- `TestGoPush_BlockedOnRunningPhase` — `.env` con `CODEJOB=jules:running:S1` → `Push` falla
  con `ErrPushBlockedActiveCodejob`.
- `TestGoPush_NotBlockedOnReviewPhase` — `.env` con `CODEJOB=jules:review:<url>` → `Push`
  **no** devuelve ese error. *(Sustituye a `TestGoPush_NotBlockedByCodejobPR` del cambio 1,
  que probaba lo mismo con el modelo viejo.)*
- `TestCodejobObjector_SkipsOnRunningAndReview` — el objetor devuelve `ActionSkip` en ambas
  fases, con `ObjectionCodejobSession`.
- `TestCodejobObjector_NoObjectionWhenNoState` — sin `CODEJOB`, y sin `docs/PLAN.md`, →
  `ActionNone`.
- Adapta `TestHasActiveCodejobSession` al nombre y tipo nuevos.

## 7. Etapa 4 — Docs y diagramas (compuerta final)

- [`docs/CODEJOB.md`](CODEJOB.md), sección "State Check & Cleanup": el bloque de `.env` pasa a
  mostrar **una** clave con sus dos fases, más la tabla de migración de §2 de este plan.
- [`docs/diagrams/CODEJOB_FLOW.md`](diagrams/CODEJOB_FLOW.md): los rombos
  `{CODEJOB in .env?}` y `{CODEJOB_PR in .env?}` se funden en uno solo:
  `{CODEJOB phase?}` con salidas `running` / `review` / `none`. El nodo `M` pasa a
  `Save state: CODEJOB=driver:running:id`; el `G1[HandleDone]` a
  `HandleDone: state → review, rename PLAN → CHECK_PLAN`.
- [`docs/diagrams/GOPUSH_FLOW.md`](diagrams/GOPUSH_FLOW.md): el rombo del cambio 1 pasa de
  `{Active CODEJOB session?}` a `{CODEJOB phase == running?}`. En la tabla de objetores,
  la fila de `CodeJob` pasa a `active session (phase running or review)`.

## 8. Criterios de aceptación

1. `go test ./...` en verde.
2. `grep -rn "EnvKeyCodejobPR" --include="*.go" .` → solo su declaración `Deprecated` y las
   líneas de migración/borrado en `codejob.go`. **Ninguna escritura.**
3. `grep -rn 'SplitN(val, ":", 2)' --include="*.go" .` → vacío (el parseo manual de
   `checkStatus` desapareció).
4. Un `.env` legado (`CODEJOB=jules:S1` + `CODEJOB_PR=<url>`) se carga como fase `review` y,
   tras el primer `Save`, queda migrado al formato nuevo.
5. Sin cambios en `cmd/`.

## 9. Etapas

| # | Etapa | Archivos | Estado |
|---|-------|----------|--------|
| 1 | `CodejobState` tipado + Load/Save/Clear + migración | `codejob.go`, `test/codejob_state_test.go` | ☐ |
| 2 | Migrar consumidores a una sola clave | `codejob.go`, `codejob_state.go`, `go_handler.go` | ☐ |
| 3 | Guardas por **fase** (asimetría raíz/dependiente) | `go_handler.go`, `codejob.go`, `test/go_handler_test.go`, `test/publish_objector_test.go` | ☐ |
| 4 | Docs y diagramas | `docs/CODEJOB.md`, `docs/diagrams/*` | ☐ |

Las etapas van **en orden**: la 2 necesita el tipo de la 1, la 3 necesita los consumidores de
la 2, y la 4 es la compuerta final.
