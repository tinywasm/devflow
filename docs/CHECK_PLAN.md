---
message: "fix: la cascada de gopush no toca repos con codejob activo y deja de propagar tags rancios"
---

> Este plan se despacha vía el flujo CodeJob. Ver skill: agents-workflow.

# PLAN — cola de ejecución de `tinywasm/devflow`

> Si te han dicho *"ejecuta el plan descrito en docs/PLAN.md"*, ejecuta **únicamente el
> primer cambio de la tabla que no esté cerrado**. Cada sub-plan es autocontenido.

| Orden | Cambio | Asunto | Compuerta |
|-------|--------|--------|-----------|
| 1 | [PLAN_CASCADE_GUARD.md](PLAN_CASCADE_GUARD.md) | La cascada de `gopush` muta el `go.mod` del dependiente **antes** de preguntar si podía tocarlo, y un nodo saltado propaga su tag viejo. Invertir el orden + `CascadeOutcome` tipado + `gopush` rechaza publicar con `CODEJOB` activo + el flag `--no-cascade` que el diagrama promete y el CLI no tiene. | 🔴 **URGENTE — despachar ya** |
| 2 | [PLAN_FRONTMATTER_KEY.md](PLAN_FRONTMATTER_KEY.md) | Renombrar la clave de frontmatter `message:` → `PLAN:`. | Solapa con el 4 |
| 3 | [PLAN_CODEJOB_STATE.md](PLAN_CODEJOB_STATE.md) | Una sola variable de `.env` por manejador: `CODEJOB` y `CODEJOB_PR` se fusionan en `CODEJOB=<driver>:<fase>:<ref>`. | **Requiere el cambio 1** |
| 4 | [PLAN_TOOL_EXTRACTION.md](PLAN_TOOL_EXTRACTION.md) | `devflow` es un módulo gordo: quien importa una función mínima arrastra keyring, wizard, mcp, gorun… `server` y `client` lo hacen para llamar a **dos funciones**. Consumir `tinywasm/markdown` y `tinywasm/command` y borrar el código movido. | Despachable · su publicación **espera al cambio 1** |

## Grafo de dependencias

```mermaid
flowchart LR
    C1[1. Cascade guard<br/>🔴 corrupción activa] --> C3[3. Estado codejob<br/>una variable por manejador]
    C1 -.desbloquea la publicación de.-> P[server + client<br/>ya migrados, sin publicar]
    C2[2. Frontmatter PLAN:] <-.solapan en<br/>frontmatter.go.-> C4[4. Extracción de herramientas]
```

- **1 es la única urgencia, y ahora además bloquea trabajo terminado.** Hay corrupción
  ocurriendo hoy (`goflare-demo` tiene el árbol sucio con una sesión activa), y por eso
  `server` y `client` — ya migrados y en verde — **no se pueden publicar**: la cascada de
  `gopush` llegaría hasta `goflare-demo` y le volvería a mutar el `go.mod` mientras el agente
  trabaja sobre él. Tampoco hay escape manual, porque `--no-cascade` no existe en el CLI.
- **3 depende de 1** porque reescribe la misma guarda que introduce el cambio 1
  (`HasActiveCodejobSession`). Despacharlos a la vez provocaría un conflicto de merge
  garantizado.
- **2 y 4 se solapan**: ambos reescriben `frontmatter.go` (el 2 renombra la clave; el 4 lo
  convierte en una capa fina sobre `tinywasm/markdown`). Cualquier orden vale, pero **no los
  despaches a la vez**.
- La etapa local del 4 (`gonew` + publicar) **ya está hecha**: `tinywasm/markdown` v0.0.2 y
  `tinywasm/command` v0.0.2, ambos con cero dependencias.

## Trabajo terminado que espera al cambio 1

| Repo | Estado | Publicar cuando |
|---|---|---|
| `server` | `markdown.New` en lugar de `devflow`; `devflow` fuera del `go.mod`; tests verdes | el cambio 1 esté fusionado y `gopush` reinstalado |
| `client` | `markdown.New` + `command.RunInDir`; `devflow` fuera del `go.mod`; tests verdes | ídem |

## Por qué existe esta cola

Los cuatro cambios atacan la misma enfermedad desde ángulos distintos: **`devflow` acumuló
responsabilidades y estado sin límites claros**. El cambio 1 arregla un límite roto entre
`gopush` y `codejob`; el 3 reduce el estado que `codejob` esparce por el `.env`; el 4
reduce la superficie que `devflow` impone al resto del ecosistema; y el 2 es deuda de
nomenclatura en el contrato que une plan y commit.
