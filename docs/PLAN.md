---
PLAN: "fix: refuse to tag a version already burned in the public Go checksum database"
EXECUTOR: jules
REVIEWER: none
---

> Este plan se despacha vía el flujo CodeJob. Ver skill: agents-workflow.

# Plan — `gopush` debe negarse a reutilizar una versión ya "quemada" en sum.golang.org

## El incidente que motiva este plan

Al intentar desplegar `veltylabs/iam` (2026-08-25), `go vet ./...` falló en
GitHub Actions (entorno limpio, sin `GOPRIVATE` configurado) con:

```
verifying github.com/tinywasm/rbac@v0.0.4: checksum mismatch
	downloaded: h1:yFJTFgXK+54iCPaaWayQCF6a1OWDxhHnoEDYFvrL7rM=
	go.sum:     h1:HKO7gzbCPvVd9r4X0viNYUh/IzzMUBQy4pwe0MmdAEA=

SECURITY ERROR
This download does NOT match an earlier download recorded in go.sum.
```

Localmente esto nunca se detectó porque `go env GOPRIVATE` incluye
`github.com/tinywasm` — Go evita tanto el proxy público
(`proxy.golang.org`) como el checksum database público (`sum.golang.org`)
para esos módulos, yendo directo a GitHub. En CI, sin esa variable, Go usa
la configuración por defecto (`GOPROXY=https://proxy.golang.org,direct`,
`GOSUMDB=sum.golang.org`), que sirve el checksum que tenga **grabado para
siempre** para `tinywasm/rbac@v0.0.4` — sea cual sea el contenido actual
del tag en GitHub.

**Confirmado, no es un bug en el código de tagging actual:**
`git.CreateTag` (`tinywasm/git/git_handler.go`) falla si el tag ya existe
localmente; `Push` valida `tag > latestTag` antes de comitear. No hay
evidencia de que el tag `v0.0.4` se haya movido dentro de esta sesión (el
commit local y remoto coinciden). La causa más probable: en algún punto
anterior (sesión previa, o un intento descartado), `v0.0.4` existió
brevemente con otro contenido, alguien lo consultó sin `GOPRIVATE` activo,
y el checksum quedó grabado en `sum.golang.org` de forma permanente — ese
servicio nunca actualiza una entrada ya escrita, por diseño (es su garantía
de seguridad contra ataques de sustitución).

**Mitigación inmediata ya aplicada** (fuera de este plan, en
`veltylabs/iam`): se taggeó `tinywasm/rbac@v0.0.5` sobre el mismo commit —
verificado limpio contra el proxy público real — y `iam` fue actualizado
para usarlo.

## El gap real: nada detecta esto ANTES de taggear

`gopush` (`devflow.Go.Push` → `gitmod.Git.Push`) no consulta el checksum
database público al elegir un número de versión. Cualquier repo del
ecosistema puede volver a "quemar" una versión sin que nadie lo note hasta
que un consumidor la construya en un entorno sin `GOPRIVATE` — típicamente
CI, el peor momento posible para descubrirlo.

## Diseño propuesto

### `SumDBClient`: nueva interfaz, en `tinywasm/git` (paquete `interface.go`)

```go
// SumDBClient reports whether a module version was ever indexed by the
// public Go checksum database (sum.golang.org). A version that appears
// there was consulted by someone, at some point, possibly with different
// content than what is about to be tagged now — reusing it risks the
// exact "SECURITY ERROR: checksum mismatch" failure this guards against.
type SumDBClient interface {
	Lookup(modulePath, version string) (burned bool, err error)
}
```

Añadir `IncrementTag(tag string) (string, error)` a la interfaz
`GitClient` (`tinywasm/git/interface.go`) — el método **ya existe** en el
struct concreto `*Git` (`git_handler.go`), solo falta exponerlo en la
interfaz para que `devflow.Go` pueda invocarlo a través de la abstracción
inyectable que ya usan sus tests (`MockGitClient`).

### Implementación real: `tinywasm/git/sumdb.go` (nuevo)

```go
package git

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// HTTPSumDB is the real SumDBClient, backed by sum.golang.org's lookup
// endpoint (https://go.dev/design/25530-sumdb#lookup).
type HTTPSumDB struct {
	Client *http.Client // nil = http.DefaultClient
}

func (s *HTTPSumDB) Lookup(modulePath, version string) (bool, error) {
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	u := fmt.Sprintf("https://sum.golang.org/lookup/%s@%s",
		url.PathEscape(modulePath), url.PathEscape(version))
	resp, err := client.Get(u)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil // never indexed — free to use
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("sumdb lookup %s@%s: unexpected status %d", modulePath, version, resp.StatusCode)
	}
	return true, nil
}

var _ = json.Marshal // placeholder if a richer response type is needed later
```

**Nota para quien ejecute este plan:** verificar el formato de respuesta
real de `/lookup/` (es texto plano firmado, no JSON — la comprobación de
`resp.StatusCode` basta, no hace falta parsear el cuerpo). Ajustar el
placeholder de `encoding/json` si no termina haciendo falta — no dejar un
import sin uso real.

### `devflow.Go`: nuevo campo opcional + validación antes de taggear

En `go_handler.go`:

```go
type Go struct {
	// ... campos existentes ...
	sumdb gitmod.SumDBClient // nil = sin verificación, comportamiento actual
}

// SetSumDBClient enables the public-checksum-database guard before
// tagging. nil (never called) preserves the exact behavior this package
// had before this option existed.
func (g *Go) SetSumDBClient(c gitmod.SumDBClient) {
	g.sumdb = c
}
```

En `Push`, **antes** de la llamada a `g.git.Push(message, tag)` (la que hoy
recibe el `tag` parámetro tal cual, vacío o no):

```go
resolvedTag := tag
if g.sumdb != nil && modulePath != "" {
	var err error
	resolvedTag, err = g.resolveCleanTag(modulePath, tag)
	if err != nil {
		return gitmod.PushResult{}, err
	}
}
pushResult, err = g.git.Push(message, resolvedTag)
```

```go
// resolveCleanTag returns a version for modulePath free of public
// checksum-db conflicts.
//
//   - candidate == "" (auto-generate): resolves via g.git.GenerateNextTag(),
//     then keeps incrementing (g.git.IncrementTag) past any version already
//     burned — no caller intent is being overridden, any higher number
//     satisfies "give me the next version".
//   - candidate != "" (explicit): if THAT exact version is burned, FAILS
//     loudly instead of silently substituting a different one — silently
//     moving v0.0.4 to v0.0.5 when the caller asked for v0.0.4 specifically
//     would be its own kind of surprise.
//
// A lookup error (network down, sum.golang.org unreachable) is treated as
// "unknown, not burned" — see "Pregunta abierta" below before implementing
// this part as-is.
func (g *Go) resolveCleanTag(modulePath, candidate string) (string, error) {
	if candidate != "" {
		burned, err := g.sumdb.Lookup(modulePath, candidate)
		if err != nil {
			return candidate, nil // see open question
		}
		if burned {
			return "", fmt.Errorf(
				"tag %s for %s is already indexed in the public Go checksum database "+
					"with possibly different content — pick a different version",
				candidate, modulePath)
		}
		return candidate, nil
	}

	tag, err := g.git.GenerateNextTag()
	if err != nil {
		return "", err
	}
	for attempts := 0; attempts < 100; attempts++ {
		burned, err := g.sumdb.Lookup(modulePath, tag)
		if err != nil {
			return tag, nil // see open question
		}
		if !burned {
			return tag, nil
		}
		tag, err = g.git.IncrementTag(tag)
		if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("could not find a tag for %s free of public checksum-db conflicts after 100 attempts", modulePath)
}
```

### `cmd/gopush/main.go`: activar por defecto

Una vez implementado y probado, `gopush` (el binario, no la librería) debe
llamar `SetSumDBClient(&gitmod.HTTPSumDB{})` siempre — el punto de este
plan es que este incidente no vuelva a pasar, así que la protección debe
estar activa por defecto en el camino real de publicación, no detrás de un
flag opt-in que nadie active.

## Pregunta abierta — decidir antes de implementar

**¿Qué hacer si el lookup a `sum.golang.org` falla (red caída, timeout)?**

- **Opción A (la que el pseudocódigo arriba asume): fallar abierto.** Un
  error de red no bloquea el push — se trata como "no se pudo confirmar,
  se asume libre". Nunca se pierde la capacidad de publicar por una caída
  transitoria de un servicio externo.
- **Opción B: fallar cerrado.** Un error de red SÍ bloquea el push. Más
  estricto — nunca se publica sin la confirmación positiva de que la
  versión está limpia — pero convierte una caída de `sum.golang.org` en un
  incidente de publicación propio.

Este plan no decide por el mantenedor — es exactamente el tipo de
trade-off (disponibilidad vs. certeza) que **CONSTRUCTION_HARNESS.md**
pide no resolver unilateralmente. Confirmar la opción antes de escribir el
código de producción; el test `TestPush_ExplicitTagBurnedInSumDB_Fails` en
`tinywasm/devflow/test/sumdb_test.go` (ya escrito, ver más abajo) no se ve
afectado por esta decisión — solo prueba el caso "lookup exitoso, versión
quemada".

## TDD — el test ya existe, en rojo

`tinywasm/devflow/test/sumdb_test.go` (ya escrito, commiteado con este
plan) prueba contra la API propuesta arriba y **hoy falla en compilación**
(`goHandler.SetSumDBClient undefined`) — es la especificación ejecutable
de este plan, no algo que quien lo ejecute deba adivinar:

- `TestPush_ExplicitTagBurnedInSumDB_Fails` — tag explícito quemado → error
  claro, `git.Push` nunca se llega a invocar con ese tag.
- `TestPush_AutoGeneratedTagBurnedInSumDB_Increments` — tag auto-generado
  quemado → incrementa hasta encontrar uno libre.
- `TestPush_SumDBClientNilPreservesCurrentBehavior` — sin
  `SetSumDBClient`, comportamiento idéntico al actual, cero llamadas
  nuevas.

También se extendió `MockGitClient` (mismo archivo de tests,
`go_handler_test.go`) con `IncrementTag` y el campo `nextTagOverride` —
necesarios para que el mock siga implementando `gitmod.GitClient` una vez
que gane el método nuevo, y para controlar `GenerateNextTag` en el segundo
test sin tocar los ~15 tests existentes que ya dependen del valor
hardcodeado `"v0.0.1"`.

## DRY — reusar, no duplicar

- `IncrementTag` **no se reimplementa**: ya existe en `*Git`
  (`git_handler.go:397`), correcto y ya probado
  (`TestGenerateNextTagWithOutOfOrderTags` y afines) — este plan solo lo
  expone en la interfaz `GitClient`, nunca copia su lógica.
- El mensaje de error de `resolveCleanTag` es la única fuente de verdad de
  ese texto — ningún otro punto del código debe repetirlo si se necesita
  en más de un lugar.
- `HTTPSumDB` vive en `tinywasm/git` (junto al resto de la mecánica git),
  no en `devflow` — `devflow` solo conoce la interfaz `SumDBClient`, igual
  patrón que ya usa para `GitClient`/`Runner`.

## Archivos a crear/modificar

```
tinywasm/git/interface.go        // modificar: +SumDBClient, +IncrementTag en GitClient
tinywasm/git/sumdb.go            // nuevo: HTTPSumDB
tinywasm/devflow/go_handler.go   // modificar: campo sumdb, SetSumDBClient, resolveCleanTag, uso en Push
tinywasm/devflow/cmd/gopush/main.go // modificar: activar por defecto
tinywasm/devflow/test/sumdb_test.go // YA EXISTE — no tocar salvo que la Opción A/B elegida requiera un test adicional para el caso de error de red
```

## Criterios de aceptación

- [ ] `go test ./...` en `tinywasm/git` y `tinywasm/devflow` — todo verde,
      incluyendo `sumdb_test.go` sin modificarlo (salvo un test adicional
      para la decisión de red, si aplica).
- [ ] `gopush` real (`cmd/gopush`), al publicar una versión ya quemada
      manualmente en un repo de prueba, se niega con el mensaje de
      `resolveCleanTag` — probado manualmente una vez, no en CI (requiere
      red real).
- [ ] `grep -rn "sum.golang.org" tinywasm/git/sumdb.go` → no vacío, y en
      ningún otro archivo del plan — un solo punto de contacto con el
      servicio externo.

## Fuera de alcance

- Verificar retroactivamente si alguna OTRA versión ya publicada en el
  ecosistema (más allá de las once verificadas manualmente el 2026-08-25:
  `jwt@v0.1.16`, `auth@v0.0.7`, `auth@v0.0.8`, `iam@v0.0.9`) está quemada —
  se verificó lo publicado en la sesión que originó este incidente, no el
  historial completo del ecosistema.
- Migrar repos que YA usan `gopush` sin este flag — este plan lo activa
  por defecto en `cmd/gopush`, no requiere cambios en ningún consumidor.
