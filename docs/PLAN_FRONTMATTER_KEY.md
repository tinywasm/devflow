← Índice: [PLAN.md](PLAN.md)

# Cambio 2 — La clave de frontmatter pasa de `message:` a `PLAN:`

> Sub-plan del índice maestro [PLAN.md](PLAN.md). Se despacha vía el flujo CodeJob.
> Ver skill: agents-workflow.

## 1. Qué y por qué

Todo `docs/PLAN.md` abre con un bloque de frontmatter cuya clave obligatoria hoy se llama
`message:`. El nombre es pobre: en un archivo llamado `PLAN.md`, dentro de un flujo llamado
`codejob`, la palabra "message" no dice de qué mensaje habla (¿el prompt al agente? ¿un
comentario?). En realidad es **el mensaje de commit con el que se cierra el loop**, y quien
lo escribe está describiendo *el plan*. Se renombra a `PLAN:`.

```markdown
---
PLAN: "feat: lo que implementa este plan"
tag: v0.2.0
---
```

`tag:` **no cambia**.

## 2. Ruptura limpia, sin fallback silencioso

**No se acepta `message:` como alias.** Un fallback silencioso dejaría dos claves válidas
para siempre y nadie migraría. En su lugar, el error es explícito y accionable: si el
frontmatter no trae `PLAN:`, el mensaje **nombra la clave vieja** para que el fix sea obvio
desde la terminal.

Los `docs/PLAN.md` de los demás repos del ecosistema quedan temporalmente rotos hasta que se
les cambie la clave. Es deliberado: el error los delata de uno en uno, con instrucciones.

## 3. Contexto para el ejecutor

- `tinywasm/devflow` es **tooling de backend**: usa la stdlib de Go legítimamente. **NO**
  apliques aquí reglas del ecosistema WASM.
- **TDD estricto**: escribe primero los tests, comprueba que fallan, y solo entonces
  implementa.
- **Cero strings mágicos**: los nombres de clave son constantes exportadas.
- No toques `cmd/`. Ejecuta los tests con `go test ./...`; **no ejecutes `gopush` ni
  `codejob`**.

## 4. Etapa 1 — El parser

**Archivos**: [`frontmatter.go`](../frontmatter.go), `test/merge_message_test.go`,
`test/markdown_extractor_test.go`.

### 4.1 Tests primero (rojo)

- `TestParseFrontmatter_PlanKey` — un bloque con `PLAN: "feat: x"` y `tag: v0.1.0` parsea a
  `PlanMeta{Message: "feat: x", Tag: "v0.1.0"}`.
- `TestParseFrontmatter_LegacyMessageKeyRejected` — un bloque que trae **solo** `message:`
  devuelve `ErrFrontmatterNoPlan`, y el texto del error contiene la palabra `message` (para
  que el usuario sepa qué renombrar).
- Los tests existentes de frontmatter que usan `message:` en sus fixtures pasan a usar
  `PLAN:`. Eso **no** es cambiar una expectativa: es actualizar el fixture al contrato nuevo.

### 4.2 Implementación (verde)

En `frontmatter.go`:

1. Constantes de clave (hoy son literales dentro del `switch`):
   ```go
   const (
       FrontmatterKeyPlan = "PLAN" // required: commit message used when closing the loop
       FrontmatterKeyTag  = "tag"  // optional: explicit version tag
       frontmatterKeyLegacyMessage = "message" // renamed to PLAN; only used to hint the fix
   )
   ```
2. El `switch key` reconoce `FrontmatterKeyPlan` y `FrontmatterKeyTag`. La clave
   `message` **ya no rellena nada** — solo se registra para el mensaje de error.
3. Renombra `ErrFrontmatterNoMessage` → `ErrFrontmatterNoPlan`, con este texto verbatim
   (más `frontmatterHelp`):
   ```go
   ErrFrontmatterNoPlan = errors.New("plan frontmatter: missing required 'PLAN:' field (the old 'message:' key was renamed — rename it in your plan)" + frontmatterHelp)
   ```
4. Actualiza el bloque `frontmatterHelp` para que el ejemplo use `PLAN:` y su tabla diga
   `PLAN  REQUIRED. The commit message used when the loop is closed.`
5. `ErrNoCloseLoopMessage`: su texto dice `add 'message:' to the plan frontmatter` → pasa a
   `add 'PLAN:' to the plan frontmatter`.

**El campo del struct `PlanMeta.Message` NO se renombra.** Es el nombre correcto para lo que
guarda (un mensaje de commit) y lo consume `ResolvePublishMessage`. Lo que se renombra es la
clave del documento, no el modelo interno.

## 5. Etapa 2 — Docs y skills (compuerta final)

Cambia **todas** las apariciones de la clave vieja. Verificable con
`grep -rn "message:" docs/ skills/` → solo puede quedar dentro de un texto que hable
explícitamente del renombrado.

| Archivo | Qué cambiar |
|---|---|
| [`docs/CODEJOB.md`](CODEJOB.md) | El bloque de ejemplo, la tabla de claves (`message` → `PLAN`) y el error de ejemplo. |
| [`docs/diagrams/CODEJOB_FLOW.md`](diagrams/CODEJOB_FLOW.md) | El nodo `IV{Valid frontmatter?<br/>message: required}` → `PLAN: required`; la sección "Plan frontmatter" y su ejemplo. |
| [`skills/plan-authoring/SKILL.md`](../skills/plan-authoring/SKILL.md) | La sección "Frontmatter — REQUIRED" y su tabla. |
| [`skills/agents-workflow/SKILL.md`](../skills/agents-workflow/SKILL.md) | Toda mención a `message:` en el frontmatter. |
| [`docs/PLAN.md`](PLAN.md) y este archivo | El frontmatter del propio índice usa `PLAN:`. |
| [`cmd/codejob/main.go`](cmd/codejob/main.go) y este archivo | El mensaje debe de usar `PLAN:`. |

## 6. Seguimiento local (NO lo hace el agente)

Tras fusionar este cambio, el desarrollador debe:

1. `llmskill` — sincronizar las skills modificadas a los agentes instalados.
2. Renombrar `message:` → `PLAN:` en el `docs/PLAN.md` de los repos que tengan uno pendiente.

## 7. Criterios de aceptación

1. `go test ./...` en verde.
2. `grep -rn '"message"' --include="*.go" .` → solo la constante
   `frontmatterKeyLegacyMessage`.
3. Un `PLAN.md` con `message:` en vez de `PLAN:` falla con un error que menciona ambas claves.
4. Sin cambios en `cmd/`.

## 8. Etapas

| # | Etapa | Archivos | Estado |
|---|-------|----------|--------|
| 1 | Parser: `PLAN:` obligatorio, `message:` rechazado con pista | `frontmatter.go`, `test/merge_message_test.go`, `test/markdown_extractor_test.go` | ☐ |
| 2 | Docs, diagramas y skills | `docs/CODEJOB.md`, `docs/diagrams/CODEJOB_FLOW.md`, `skills/*/SKILL.md` | ☐ |
