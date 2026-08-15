---
PLAN: "feat: Git.Clone/Pull/Fetch — devflow no sabe traer un repo, solo empujarlo"
EXECUTOR: jules
REVIEWER: none
---

> Este plan se despacha con el flujo CodeJob. Ver skill: agents-workflow.

# Plan — `devflow.Git` solo sabe empujar

## El hueco

`Git` (`git_handler.go`) cubre bien el lado de escritura: `Add`, `Commit`,
`CommitPaths`, `Push`, `CreateTag`, `HasChanges`, `StatusPorcelain`,
`CheckRemoteAccess`, manejo de rama y upstream.

**No tiene ninguna operación de traída.** No existe `Clone`, ni `Pull`, ni
`Fetch`. Está construido sobre la suposición de que el repositorio ya está en
disco y de que esta máquina es su origen — cierta para `gopush`, que es de
donde salió, y falsa en cuanto un proceso automatizado tiene que mantener una
copia de trabajo de un repo del que no es autor.

## Por qué ahora

Un publicador que corre desatendido (el caso concreto: el CMS de una clínica
que publica su sitio público) necesita el ciclo completo:

```
si no existe la copia local  → Clone
si existe                    → Pull   (alguien pudo tocar el repo desde otro sitio)
escribir los ficheros        → (fuera de devflow)
Add + Commit + Push          → ya existe
```

Sin `Clone`/`Pull`, cada consumidor termina llamando a `exec.Command("git",
"clone", …)` por su cuenta — es decir, recreando localmente lo que esta
librería existe para poseer, que es exactamente lo que
`CONSTRUCTION_HARNESS.md` prohíbe ("un consumidor nunca recrea un símbolo que
falta; si una librería no expone lo que necesitas, párate y repórtalo").

## El arreglo

Tres métodos, sobre el mismo `g.run` que usan todos los demás:

```go
// Clone trae repoURL a la copia de trabajo. Si el destino ya contiene un
// repositorio, NO es un error: no hace nada y lo reporta, para que un
// publicador desatendido pueda llamar a Clone incondicionalmente al arrancar.
func (g *Git) Clone(repoURL string) (alreadyPresent bool, err error)

// Pull actualiza la copia de trabajo desde el upstream.
func (g *Git) Pull() error

// Fetch trae refs sin tocar el árbol de trabajo.
func (g *Git) Fetch() error
```

Detalles que no son opcionales:

- **`Clone` respeta `SetRootDir`**, como el resto del handler. Clona *en* ese
  directorio; no inventa una segunda convención de rutas.
- **`Clone` es idempotente por diseño.** Un publicador que arranca no debería
  tener que preguntar primero si ya clonó: llama a `Clone`, y si ya estaba,
  sigue. Devolver un error por "ya existe" obliga a cada consumidor a
  distinguir ese caso del fallo real, y ahí es donde se cuelan los `if
  strings.Contains(err.Error(), …)`.
- **`Pull` con un árbol sucio debe fallar con un error propio y reconocible**,
  no dejar el repo a medio mezclar. Un publicador desatendido tiene que poder
  distinguir "hay cambios locales sin publicar" de "no hay red". Mira cómo
  `Push` clasifica hoy sus fallos y sigue ese mismo criterio.
- **Autenticación: no inventes nada.** `Clone` usa el transporte que trae la
  URL. Si es SSH (`git@github.com:…`), la clave la resuelve el agente/el
  fichero de configuración de la máquina — que es justo lo que permite usar una
  *deploy key* acotada a un repo. `CheckRemoteAccess` y `SetAuthRetrier` ya
  existen para el camino HTTPS; no dupliques esa lógica ni fuerces un flujo de
  token en `Clone`.

## Restricciones

- Este repo es herramienta de backend: usa la biblioteca estándar
  legítimamente. No "arregles" esos imports.
- No cambies la firma ni el comportamiento de `Push`, `Add`, `Commit` ni
  `CommitPaths` — hay consumidores publicados (`gopush`, `codejob`).
- Sin carpetas `internal/`.
- Todo string repetido es una constante con nombre.

## Verificación

- `Clone` sobre un directorio vacío trae el repo; sobre uno que ya lo contiene
  devuelve `alreadyPresent == true` y `err == nil`, **sin tocar el árbol**.
- `Pull` con cambios locales sin commitear falla con el error propio, y el
  árbol queda exactamente como estaba.
- `Fetch` actualiza refs y **no** modifica el árbol de trabajo.
- Los tests usan repos locales de verdad (`git init` en un `t.TempDir()` y
  clonar por ruta de sistema de ficheros) — sin red y sin dobles: es una
  librería que envuelve al binario `git`, y un doble no probaría nada.
- Suite completa verde: `go build ./... && go vet ./... && go test ./...`.

## Etapas

| # | Alcance | Aceptación |
|---|---|---|
| 1 | `Fetch` y `Pull` | tests con repo local; `Pull` sucio falla limpio |
| 2 | `Clone` idempotente, respetando `SetRootDir` | segundo `Clone` no falla ni pisa nada |
