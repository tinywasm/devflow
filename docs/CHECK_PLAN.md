# Mejorar la Experiencia de Publicación de Código en el Ecosistema Go/Golang

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.
>
> Documento en español a pedido del mantenedor (análisis + hoja de ruta). Si una
> fase se despacha a un agente externo vía codejob, extraerla a su propio
> `docs/PLAN_<FASE>.md` autocontenido (idealmente en inglés).
>
> **Anti-footgun para el agente ejecutor**: este repo (`devflow`) es tooling de
> backend/CLI — usa stdlib de Go legítimamente (`os`, `strings`, `errors`,
> `net/http`). NO aplicar las reglas WASM del ecosistema (tinywasm/fmt, etc.)
> aquí. Los archivos `gopush` y `codejob` en la raíz del repo son binarios ELF
> compilados (artefactos), no código fuente — no tocarlos.

## 0. Contrato TDD — los tests y el diagrama YA describen el flujo objetivo

El contrato de esta mejora está **escrito y en rojo** (no compila hasta que se
implemente la API). El trabajo del agente es implementar la librería hasta que
estos tests pasen **sin modificar sus expectativas** (agregar tests nuevos sí
está permitido):

| Archivo | Contrato que fija |
|---|---|
| [test/dependents_guard_test.go](../test/dependents_guard_test.go) | Dirty-guard: `StatusPorcelain`, `CommitPaths`, `DiffShortStat`, `WorkTreeDirtyBeyond`; dependiente sucio → commit SOLO `go.mod`+`go.sum`, sin tag, jamás `git add .` |
| [test/commit_message_test.go](../test/commit_message_test.go) | `DepBump`, `BuildDepsCommitMessage` (golden), constantes `DepsCommitPrefix`/`CauseLinePrefix` |
| [test/cascade_test.go](../test/cascade_test.go) | `BuildDependentGraph` (cierre transitivo + topológico + ciclos + `MaxCascadeDepth`), `RunCascade` + `SetCascadeProcessFn`, `CascadeNode`/`CascadeReport`/estados |
| [test/go_handler_test.go](../test/go_handler_test.go) | `UpdateDependentModule` con 4º parámetro `rootCause`; push raíz agrega cuerpo `--shortstat`; mock ampliado del `GitClient` |

El flujo objetivo completo está diagramado en
[diagrams/GOPUSH_FLOW.md](diagrams/GOPUSH_FLOW.md) (pipeline principal +
procesamiento por nodo de la cascada). Tests y diagrama están alineados entre
sí; el código actual NO — esa brecha es exactamente lo que hay que implementar.

## 1. Contexto y problema

Cómo funciona hoy `gopush` con los dependientes (ver [GOPUSH.md](GOPUSH.md)):
al publicar un módulo (p.ej. `tinywasm/router`), busca en `..` los módulos que
lo requieren y en cada uno quita el `replace` local, hace `go get` de la
versión nueva + `go mod tidy`, corre sus tests y, si pasan, lo pushea.

**Esa mecánica es correcta y se conserva** — quitar el `replace`, el `go get`,
el `tidy` y los tests como puerta no son el problema. Los problemas están en
tres puntos específicos de CÓMO se commitea y hasta dónde llega esa cascada:

### P1 — El auto-push de dependientes arrastra trabajo en progreso (BUG, el más grave)

`UpdateDependentModule` ([go_handler.go:296](../go_handler.go#L296)) termina
llamando `depHandler.Push(...)` que ejecuta `g.git.Add()` = `git add .`
([git_handler.go:234](../git_handler.go#L234)). Si el dependiente tiene cambios
sin commitear (caso actual: `tinywasm/sse` con mucho WIP), **todo ese trabajo se
commitea y pushea** con el mensaje `deps: update router to vX.Y.Z`.

Consecuencias: (a) se publica trabajo a medias sin consentimiento; (b) el
mensaje de commit **miente** — dice "deps" pero contiene features/refactors. El
problema de "mensajes vagos" que percibe el mantenedor es en gran parte este
bug: para un bump puro de dependencia, `deps: update router to v0.1.3` es un
mensaje exacto, no vago.

### P2 — La cascada se corta en el primer nivel

En [go_handler.go:383](../go_handler.go#L383) el push del dependiente se invoca
con `skipTag=true` y `skipDependents=true`:

```go
_, err = depHandler.Push(commitMsg, "", true, true, true, true, true, false, "")
//                                  skipTests, skipRace, skipDependents, skipBackup, skipTag ...
```

- **Sin tag**: el dependiente queda commiteado/pusheado pero sin versión nueva.
  Sus propios consumidores no pueden hacer `go get` de nada — la actualización
  transitiva es imposible.
- **Sin recursión**: los dependientes del dependiente nunca se enteran.

### P3 — Mensajes de commit en cascadas: tedio y trazabilidad

Con cascada recursiva, cada repo intermedio necesita un mensaje. Escribirlos a
mano anula la automatización; dejarlos genéricos rompe el seguimiento. Hoy el
único mensaje automático es `deps: update %s to %s` (go_handler.go:382) —
correcto pero sin contexto de *por qué* (qué cambió río arriba).

## 2. Decisiones de diseño (pros / contras / justificación)

### A. Protección del árbol de trabajo sucio (resuelve P1) — sin configuración

| Opción | Pros | Contras |
|---|---|---|
| A1: skip total si hay cambios fuera de `go.mod`/`go.sum` | Cero riesgo, trivial | El bump queda pendiente; habrá que repetir el `go get` a mano |
| A2: commit **solo por pathspec** (`git add go.mod go.sum` + commit + push, sin tag, sin cascada) | El bump avanza, el WIP queda intacto, el mensaje es veraz | Los tests corren con el WIP presente: si el WIP rompe tests, se hace skip (comportamiento actual) |

**Decisión: A2 con degradación a skip.** Si el árbol está sucio (ignorando
`.env`/`.gitignore`, misma regla que `HasPendingChanges`,
[git_handler.go:547](../git_handler.go#L547)): correr tests; si pasan,
commitear únicamente `go.mod`+`go.sum`, pushear sin tag y **no** cascadear
desde ese repo (no hay versión nueva que propagar). Si fallan, revertir
`go.mod`/`go.sum` y skip. Si el árbol está limpio: flujo completo (tag +
cascada, ver C).

Justificación clave: **la protección es automática e intuitiva** — un repo con
trabajo a medias está sucio por definición, así que queda protegido sin que el
desarrollador tenga que acordarse de nada. Se evaluó y **descartó** una clave
explícita `GOPUSH_HOLD` en `.env` (decisión del mantenedor 2026-07-11: poco
intuitivo, nadie la usaría). El estado del árbol ES la señal de intención.

### B. Cascada transitiva (resuelve P2)

| Diseño | Descripción | Pros | Contras |
|---|---|---|---|
| B1: recursión local | Cambiar `skipTag`/`skipDependents` a `false` en la llamada interna | 1 línea | Sin control de ciclos; workers anidados (5×5×…); un módulo que depende de 2 publicados recibe 2 commits+2 tags; output ilegible. **Descartado** |
| B2: coordinador BFS por olas | Procesar nivel 1, luego 2, con set de visitados | Controla ciclos | Módulo alcanzable por 2 rutas puede perder un bump o duplicar tags |
| B3: **cierre transitivo + orden topológico** | Resolver TODO el grafo primero (`FindDependentModules` ya existe, [go_mod.go:616](../go_mod.go#L616)), ordenar topológicamente, procesar cada módulo **una sola vez** con todos los bumps de sus deps ya publicadas en esta ola | Mínimo nº de tags (1 por módulo); mensajes multi-bump precisos; determinista; paraleliza dentro de cada nivel | Más código; duración total mayor (niveles secuenciales) |

**Decisión: B3.** Es la única que produce exactamente un commit+tag por módulo
por ola. Reglas fijadas por `test/cascade_test.go`:

- Un nodo se procesa **sí y solo sí** tiene ≥1 bump disponible (alguna de sus
  deps en cascada publicó versión nueva). Con todas sus upstreams fallidas o
  deps-only → `skipped`. Actualización parcial es segura: el módulo simplemente
  se queda en la versión vieja de la dep fallida.
- `const MaxCascadeDepth = 10` niveles topológicos — exceder es error.
- Ciclo en el grafo → error nombrando el ciclo, sin publicar nada de él.
- Flag `--no-cascade` en el CLI recupera el comportamiento de un solo nivel.
- Por nodo, el bump de `go.mod` ocurre PRIMERO (semántica actual: un skip deja
  el módulo actualizado localmente, solo omite el push). Guards en orden tras
  el bump: `CODEJOB` activo → skip; otros `replace` → skip; tests fallan →
  failed + revert de `go.mod`/`go.sum` (rama cortada); árbol sucio → deps-only
  (A2). Ver el diagrama por nodo en
  [diagrams/GOPUSH_FLOW.md](diagrams/GOPUSH_FLOW.md).
- Fallos cortan **solo su rama**; el reporte final (`CascadeReport`) lista cada
  módulo con su estado: `published` / `deps-only` / `skipped` / `failed`.

### C. Mensajes de commit (resuelve P3) — programático, sin IA

**¿Se puede resumir programáticamente sin IA? ¿Existe librería?** Respuesta
honesta: **no existe** librería Go madura que genere mensajes semánticos de
commits desde diffs arbitrarios sin IA — las heurísticas por rutas/extensiones
producen títulos pobres o falsos. Lo que sí existe y sirve: `git diff
--shortstat` (resumen cuantitativo exacto) y `golang.org/x/exp/apidiff`
(cambios de API exportada entre versiones — pospuesto como mejora futura).

**La clave del caso concreto**: en una cascada el contenido semántico del
commit **se conoce por construcción** — no hay nada que adivinar. Formato
golden fijado por `TestBuildDepsCommitMessage`:

```
deps: update router to v0.1.3

cause: feat: rutas con parámetros opcionales   ← mensaje del gopush raíz, propagado

- github.com/tinywasm/router v0.1.2 → v0.1.3
```

La propagación del **mensaje raíz** (`cause:`) por toda la cascada es la mejora
de trazabilidad más barata y valiosa: leyendo el log de cualquier dependiente
se sabe qué motivó el bump — determinista, instantáneo, offline.

Para el push raíz, el título sigue siendo humano (el autor sabe qué hizo) y
gopush agrega como cuerpo el `--shortstat` del diff staged
(`TestGoPush_AppendsShortStatBody`).

**IA descartada** (decisión del mantenedor 2026-07-11): se evaluó un endpoint
configurable por variable de entorno con fallback y quedó fuera del alcance.
No agregar integración con LLMs a este flujo.

### D. Interacción con codejob

`codejob` queda cubierto sin cambios de diseño: usa `Publisher.Publish` con
skips para el sync pre-dispatch ([codejob.go:245](../codejob.go#L245)) y el
guard `HasActiveCodejobSession` ya excluye repos con sesión activa. El
coordinador de cascada **reutiliza** esos guards, no los duplica. El cierre
`codejob 'msg'` → `MergeAndPublish` → `Push` completo adquiere la cascada
automáticamente.

## 3. Fases de implementación

Orden por riesgo/valor: primero el bug que corrompe historia (P1), después
trazabilidad (P3), después la cascada (P2) que ya nace protegida.

### Fase 1 — Dirty-guard y primitivas git (seguridad)

**Archivos**: `git_handler.go`, `go_handler.go`, `interface.go`.
**Tests que deben pasar**: `test/dependents_guard_test.go` completo,
`TestUpdateDependentModule` (firma nueva).

1. `git_handler.go` — métodos nuevos en `*Git` (y en la interfaz `GitClient`
   de `interface.go`; el mock de test ya los implementa):
   - `StatusPorcelain() (string, error)` — `git status --porcelain`.
   - `CommitPaths(message string, paths ...string) (bool, error)` —
     `git add <paths>` + commit; retorna `false, nil` si esos paths no tienen
     cambios; **jamás** `git add .`.
   - `DiffShortStat() (string, error)` — `git diff HEAD --shortstat` (cambios
     vs HEAD, staged o no; vacío en árbol limpio). **No usar `--cached`**: el
     shortstat se calcula ANTES de que el flujo haga `git add`, así que un
     diff solo-staged siempre estaría vacío en ese momento.
2. `go_handler.go` — `func WorkTreeDirtyBeyond(git GitClient, allowed ...string) (bool, error)`:
   true si el porcelain contiene entradas fuera de `allowed`, ignorando
   siempre `.env` y `.gitignore`.
3. `UpdateDependentModule(depDir, modulePath, version, rootCause string)` —
   nuevo 4º parámetro. Tras bump+tests, decidir por estado del árbol:
   - sucio → `CommitPaths(msg, "go.mod", "go.sum")` + `PushWithoutTags()`,
     sin tag, resultado/console `deps only (dirty tree) ⚠`;
   - limpio → commit+tag+push completo;
   - tests fallan → revertir `go.mod`/`go.sum` (`git checkout -- go.mod go.sum`)
     y skip (rama cortada).

### Fase 2 — Mensajes programáticos con causa propagada

**Archivos**: `commit_message.go`, `go_handler.go`.
**Tests que deben pasar**: `TestBuildDepsCommitMessage`,
`TestDepsCommitConstants`, `TestGoPush_AppendsShortStatBody`, y las aserciones
de mensaje en `TestUpdateDependentModule_DirtyTreeCommitsOnlyGoModAndSum`.

1. `type DepBump struct{ ModulePath, OldVersion, NewVersion string }` y
   `func BuildDepsCommitMessage(bumps []DepBump, rootCause string) string`
   (formato golden en el test; `OldVersion` vacío se omite; sin bumps → `""`).
2. Constantes exportadas `DepsCommitPrefix = "deps: "` y
   `CauseLinePrefix = "cause: "` — prohibido repetir los literales en lógica.
3. Push raíz (proyecto Go, camino con tag): cuerpo del commit = título del
   usuario + `\n\n` + `DiffShortStat()` si no está vacío.

### Fase 3 — Cascada topológica

**Archivos**: `cascade.go` (nuevo), `go_handler.go` (delegación),
`cmd/gopush/main.go` (flag `--no-cascade`), `docs/GOPUSH.md`.
**Tests que deben pasar**: `test/cascade_test.go` completo.

1. `cascade.go`:
   ```go
   type CascadeNode struct{ Dir, ModulePath string; DependsOn []string }
   type CascadeEntry struct{ ModulePath, Status, Detail string }
   type CascadeReport struct{ Entries []CascadeEntry }
   // estados como constantes: CascadeStatusPublished, CascadeStatusDepsOnly,
   // CascadeStatusSkipped, CascadeStatusFailed
   const MaxCascadeDepth = 10

   func (g *Go) BuildDependentGraph(rootModule, searchPath string) ([]CascadeNode, error)
   func (g *Go) SetCascadeProcessFn(fn func(node CascadeNode, bumps []DepBump, rootCause string) (publishedVersion string, err error))
   func (g *Go) RunCascade(rootModule, rootVersion, rootCause, searchPath string) CascadeReport
   ```
   - El procesador real (bump+tidy+generate+tests+commit+tag+push, con los
     guards de Fase 1) es la función por defecto; `SetCascadeProcessFn` es el
     seam de test — sin red, git ni toolchain en los tests de cascada.
   - Paralelismo con el semáforo existente **dentro** de cada nivel topológico
     ([go_mod.go:594](../go_mod.go#L594)); niveles secuenciales.
   - Imprimir `CascadeReport` como tabla al final (formato en
     [diagrams/GOPUSH_FLOW.md](diagrams/GOPUSH_FLOW.md) → "Cascade report").
2. `Push` paso 6 delega en `RunCascade` pasando el mensaje del usuario como
   `rootCause`. `UpdateDependents(modulePath, version, searchPath)` conserva su
   firma de 3 argumentos como wrapper de compatibilidad (rootCause vacío) —
   `test/coverage_test.go:92` la usa.
3. CLI: `--no-cascade` filtrado en `main.go` igual que `--skip-race`
   (cmd delgado: solo parseo; la decisión vive en la librería).
4. Actualizar `docs/GOPUSH.md` al flujo nuevo (el diagrama ya está actualizado).

## 4. Checklist de calidad (obligatorio en cada fase)

- **Sin strings hardcodeados**: toda clave, prefijo de mensaje, límite y estado
  es constante exportada en la librería; literales en lógica prohibidos.
- **`cmd/` delgado**: `cmd/gopush/main.go` solo parsea flags, inyecta y hace
  print/exit; cada decisión nueva es función exportada de la librería.
- **Sin duplicación librería/cmd**: el cmd consume las constantes exportadas.
- **CLI no interactivo por defecto**: stdout = datos, stderr = diagnóstico,
  exit codes deterministas (contrato existente, no regresionar).
- **Dependencias inyectables**: git vía `GitClient`, procesador de cascada vía
  `SetCascadeProcessFn`, backup vía `BackupRunner`; nada de red real en tests.
- **No modificar las expectativas de los tests del contrato** (sección 0);
  agregar tests adicionales sí está permitido.

## 5. Riesgos y mitigaciones

| Riesgo | Mitigación |
|---|---|
| Tormenta de tags: cada gopush raíz genera N tags patch en el ecosistema | Es el modelo actual asumido (patch bumps automáticos); B3 garantiza 1 tag/módulo/ola; `--no-cascade` da freno manual |
| Un bug publicado se propaga a todo el ecosistema en minutos | Tests como puerta en CADA salto; el reporte final muestra el alcance; revertir = publicar fix y la misma cascada lo propaga |
| Cascada larga bloquea la terminal | Aceptable v1 (output en streaming ya existe); si duele, mover a modo detach como el backup |
| WIP con API vieja pasa tests con la dep nueva y el bump commiteado confunde al retomar | El commit deps-only es mínimo y reversible (`git revert` de 2 archivos); el reporte lo marca `deps only (dirty tree) ⚠` |
| Repo "limpio pero no listo" se publica en cascada | Sin mecanismo de hold explícito (descartado por poco intuitivo): la convención del ecosistema es que main limpio = publicable; si un repo no está listo, su trabajo vive sin commitear o en rama, y el dirty-guard/branch lo protege |

## 6. Etapas

| Etapa | Contenido | Depende de | Estado |
|---|---|---|---|
| 0 | Tests del contrato + diagrama alineados | — | ✅ hecho (2026-07-11) |
| 1 | Dirty-guard + `CommitPaths`/`StatusPorcelain`/`DiffShortStat` + `WorkTreeDirtyBeyond` | 0 | ✅ hecho |
| 2 | `DepBump` + `BuildDepsCommitMessage` + `cause:` + cuerpo `--shortstat` | 0 (paralela a 1) | ✅ hecho |
| 3 | Cascada topológica (`cascade.go`) + `--no-cascade` + `GOPUSH.md` | 1 y 2 (gate) | ✅ hecho |

**Resumen ejecutivo**: el "problema de sse" es un bug de `git add .` (Fase 1,
la más urgente y pequeña) y se resuelve sin configuración: el estado sucio del
árbol es la señal; los mensajes de cascada no necesitan IA — son derivables
exactamente y ganan trazabilidad propagando la causa raíz (Fase 2); la cascada
recursiva es viable solo con coordinador topológico y guards, nunca con
recursión ingenua (Fase 3).

**Notas de implementación (Jules)**:
- Se implementaron todas las fases exitosamente.
- **Verificación**: Los tests de contrato (`dependents_guard_test.go`, `commit_message_test.go`, `cascade_test.go`, `go_handler_test.go`) pasan al 100%.
- **Ambiente**: Se encontró una discrepancia de versión de Go (1.25.2 en `go.mod` vs 1.24.3 en el sandbox). Se usó 1.24 temporalmente para validación de tests.
- **Regresiones**: Algunos tests pre-existentes fallan en el sandbox debido a problemas con el `keyring` (headless environment), lo cual es independiente de los cambios realizados en este plan.
