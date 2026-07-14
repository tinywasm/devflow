---
PLAN: "fix: la cascada de gopush no toca repos con codejob activo y deja de propagar tags rancios"
---

> Este plan se despacha vía el flujo CodeJob. Ver skill: agents-workflow.

# PLAN — cola de ejecución de `tinywasm/devflow`


| Orden | Cambio | Asunto | Compuerta |
|-------|--------|--------|-----------|
| 1 | [PLAN_CASCADE_GUARD.md](PLAN_CASCADE_GUARD.md) | La cascada de `gopush` muta el `go.mod` del dependiente **antes** de preguntar si podía tocarlo, y un nodo saltado propaga su tag viejo. Invertir el orden + `CascadeOutcome` tipado + `gopush` rechaza publicar con `CODEJOB` activo + el flag `--no-cascade` que el diagrama promete y el CLI no tiene. | revisar completar lo que falte y continuar con el siguiente |
| 2 | [PLAN_FRONTMATTER_KEY.md](PLAN_FRONTMATTER_KEY.md) | Renombrar la clave de frontmatter `message:` → `PLAN:`. |revisar completar lo que falte y continuar con el siguiente   |
| 3 | [PLAN_CODEJOB_STATE.md](PLAN_CODEJOB_STATE.md) | Una sola variable de `.env` por manejador: `CODEJOB` y `CODEJOB_PR` se fusionan en `CODEJOB=<driver>:<fase>:<ref>`. | **Requiere el cambio 1** |