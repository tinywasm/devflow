# PLAN — gorelease: resolución automática del repo de publicación

Master orchestrator para implementar la publicación automática de releases en un repo
**público de distribución** cuando el `origin` es **privado**, sin flags ni variables de
entorno. La convención ES la configuración.

> Documento HOW. El WHAT/WHY abstracto vive en [GORELEASE.md](GORELEASE.md) y el flujo
> visual en [diagrams/GORELEASE_FLOW.md](diagrams/GORELEASE_FLOW.md).

---

## Development Rules (constraints copiadas para este PLAN)

- **Documentation First**: este PLAN y el diagrama se actualizan ANTES de tocar código.
- **TDD / Red first**: en esta etapa SOLO se escriben tests que reproducen el flujo
  esperado. NO se toca código de producción. Los tests definen el contrato; un agente de
  ejecución implementa hasta ponerlos en verde.
- **Dependency Injection**: la detección de visibilidad/owner DEBE pasar por el runner
  inyectable de `GitHub` (`gh.SecretRunner` vía `getSecretRunner()`), NO por el global
  `RunCommandSilent`. De lo contrario los tests no pueden simular `gh repo view`.
- **SRP**: la resolución del repo destino es una unidad separada de la creación del
  release. `CreateRelease` solo publica; `resolvePublishTarget` solo decide dónde.
- **Backward compatibility**: si la visibilidad del `origin` NO se puede determinar,
  el comportamiento debe caer al actual (publicar en `origin`, sin `--repo`). Solo se
  falla fuerte cuando se confirma `origin` privado y el repo público derivado no existe
  o no es público.

---

## Problema

Tras separar fuente (privada) de distribución (pública):

| Repo | Visibilidad | Rol |
|------|-------------|-----|
| `tinywasm/core` | Privado | fuente + tags + historial (= `origin`) |
| `tinywasm/app`  | Público | releases con binarios |

El directorio local `~/Dev/Project/tinywasm/app` **es** `core` (su `origin` apunta a
`tinywasm/core`). Hoy `gorelease` ejecuta `gh release create` sin `--repo`, por lo que
`gh` publica en `origin` → el release quedaría en **privado**, inservible para distribuir.

## Solución (convención, sin flags)

`gorelease` deduce el destino:

```
origin público  → publica en origin                (comportamiento clásico)
origin privado  → publica en <owner-origin>/<basename-carpeta-local>
                  verificando que exista y sea público; si no, falla
```

Estando en `~/Dev/Project/tinywasm/app` con `origin = tinywasm/core` (privado):
owner `tinywasm` + carpeta `app` → `tinywasm/app`.

---

## Diseño de implementación (HOW — para el agente de ejecución)

### 1. Nueva unidad de resolución
Función/método (nombre sugerido) `resolvePublishTarget(folderName string, gh *GitHub) (target string, err error)`:

1. `owner, _, visibility := gh.repoInfo("")` — consulta `gh repo view --json owner,name,visibility`
   del directorio actual (vía `gh.getSecretRunner()`).
2. Si la consulta falla (no JSON, sin `gh`, etc.) → `return "", nil` (fallback: origin).
3. Si `visibility == "PUBLIC"` → `return "", nil` (publica en origin).
4. Si privado → `candidate := owner + "/" + folderName`.
5. `vis := gh.repoInfo(candidate)` → si no existe o `!= "PUBLIC"` → `return "", fmt.Errorf(...)`.
6. Si público → `return candidate, nil`.

`folderName = filepath.Base(abs(g.rootDir))` (rootDir `.` ⇒ basename del cwd).

### 2. `CreateRelease` acepta destino opcional
`CreateRelease(tag string, assets []string, targetRepo string)`:
- Si `targetRepo != ""`, añade `--repo targetRepo` a los args de `gh release create`.
- Si `targetRepo == ""`, args idénticos a hoy (sin `--repo`).

### 3. `ReleaseOnly` orquesta
Tras cross-compile y antes de `CreateRelease`:
```go
target, err := resolvePublishTarget(filepath.Base(absRoot), gh)
if err != nil { return err }
url, err := gh.CreateRelease(tag, assets, target)
```

### 4. Helper de info de repo (DI)
`gh.repoInfo(repoRef string)` usa `gh.getSecretRunner().RunSilent("gh", "repo","view", [repoRef], "--json", "owner,name,visibility")`
y parsea `{owner:{login}, name, visibility}`. `repoRef==""` ⇒ repo del cwd.

### 5. Mejoras incorporadas

**A — Cacheo de la consulta del origin.** `repoInfo("")` devuelve owner Y visibility en una
sola llamada. La resolución NO debe volver a consultar el origin: reutiliza el owner ya
obtenido para construir el candidato. Solo se hace una 2ª llamada (`repoInfo(candidate)`)
cuando el origin es privado. Justificación: `gh` tiene latencia de red y en
`codejob -release` esto corre en el loop de cierre; se evita una llamada redundante.

**C — Matriz de distribución completa.** `DefaultTargets()` debe cubrir la audiencia
heterogénea de una distribución pública de binarios. Hoy: linux/amd64, darwin/arm64,
windows/amd64. Añadir **`linux/arm64`** (servidores ARM, CI, Raspberry) y **`darwin/amd64`**
(Macs Intel). Matriz final: linux/{amd64,arm64}, darwin/{arm64,amd64}, windows/amd64.
Necesario para que `tinywasm/installer` no haga 404 en esas plataformas.

## Mejoras de release (seguridad y calidad)

**#1 — Checksums (SHA256). SEGURIDAD.** Hoy `CreateRelease` sube binarios crudos sin
verificación de integridad; el installer los hace ejecutables sin comprobar nada → un
release comprometido = ejecución de código arbitrario. `gorelease` debe generar
`checksums.txt` (SHA256 de cada asset) e incluirlo como asset del release.
- Impl: tras cross-compile, calcular SHA256 de cada binario, escribir `checksums.txt` en el
  tmpDir, añadirlo a `assets` antes de `CreateRelease`.
- Test (live): `TestReleaseOnly_UploadsChecksums` 🔴 — asierta `checksums.txt` en los args.
- Consumo: `tinywasm/installer` lo verifica antes de `chmod +x` (ver su PLAN).

**#2 — Inyección de versión en el binario. CORRECTITUD.** `Install()` ya inyecta
`-ldflags=-X main.Version=<tag>` ([go_handler.go:451]), pero `CrossCompile` (el path de
`gorelease`) compila con `go build` plano → el binario distribuido NO reporta su versión, y
el `Verify` del installer (`strings.Contains(out, version)`) queda sin base real.
`CrossCompile` debe inyectar la **misma** variable `main.Version=<tag>` que `Install`.
- Seam a crear: pasar el `tag` a `CrossCompile` (cambia firma) o helper puro
  `crossBuildArgs(version, pkg, outputPath) []string`. Test de referencia (no live por
  requerir el seam):
  ```go
  // test que el agente de ejecución activa al crear el helper:
  func TestCrossBuildArgs_InjectsVersionAndStrips(t *testing.T) {
      args := devflow.CrossBuildArgs("v0.3.0", "./cmd/tinywasm", "/tmp/out")
      joined := strings.Join(args, " ")
      // #2 versión + #3 flags de tamaño/reproducibilidad
      mustContain(t, joined, "-X main.Version=v0.3.0")
      mustContain(t, joined, "-trimpath")
      mustContain(t, joined, "-s -w")
  }
  ```

**#3 — Flags de build: `-ldflags="-s -w" -trimpath`. CALIDAD.** `CrossCompile` solo usa
`CGO_ENABLED=0`. Añadir `-s -w` (quita símbolos debug, ~25-30% menos tamaño) y `-trimpath`
(elimina rutas locales del binario → reproducible, sin fugas de `/home/...`). Se implementa
junto con #2 en el mismo `crossBuildArgs` (mismo test de referencia).

**#4 — Un solo release, el tag más alto. YA IMPLEMENTADO + bloqueado.** `git.GetLatestTag`
([git_handler.go:289]) ya ordena por `version:refname` descendente → el semver más alto;
`ReleaseOnly` crea **un** release por invocación, no uno por tag. Se bloquea contra
regresión con `TestReleaseOnly_PublishesSingleReleaseForHighestTag` 🟢 (1 release create,
target = tag más alto). No requiere cambio de código.

## Nota: source-zip del release público

Como el código vive en `core` (privado) y el release se publica en `app` (público, solo
README), el "Source code (zip)" que GitHub adjunta automáticamente al release contendrá el
README, no el código. Es irrelevante para distribución solo-binarios — nadie usa ese enlace.
No requiere acción. (El `go install` roto ya se corrigió en el README de `app`.)

---

## Estrategia de tests (lo que se escribe AHORA)

Archivo: `test/gorelease_publish_test.go`. Se maneja todo a través de la firma pública
existente `ReleaseOnly(tag, gh)` y se asierta sobre los args finales de `gh release create`.
Se inyecta un `scriptedRunner` (helper de test, implementa `devflow.SecretRunner`) que
responde distinto según el comando.

| Test | Escenario | Aserción | Estado HOY |
|------|-----------|----------|------------|
| `TestReleaseOnly_PrivateOrigin_PublishesToDerivedPublicRepo` | origin `tinywasm/core` PRIVATE, carpeta `app`, `tinywasm/app` PUBLIC | args de release create contienen `--repo tinywasm/app` | 🔴 RED |
| `TestReleaseOnly_PrivateOrigin_DerivedRepoNotPublic_Errors` | origin PRIVATE, derivado existe pero PRIVATE | `ReleaseOnly` retorna error con "public" | 🔴 RED |
| `TestReleaseOnly_PrivateOrigin_DerivedRepoMissing_Errors` | origin PRIVATE, derivado no existe (`gh repo view` error) | `ReleaseOnly` retorna error; no llama release create | 🔴 RED |
| `TestReleaseOnly_PublicOrigin_PublishesToOrigin` | origin PUBLIC | args NO contienen `--repo` (publica en origin) | 🟢 GREEN |
| `TestReleaseOnly_VisibilityUndetermined_FallsBackToOrigin` | `gh repo view` no-JSON / falla | args sin `--repo` (no rompe flujo clásico) | 🟢 GREEN |
| `TestReleaseOnly_ReleaseCreateFails_PropagatesError` | `gh release create` falla | `ReleaseOnly` propaga el error | 🟢 GREEN |
| `TestReleaseOnly_UploadsChecksums` | release público | args incluyen `checksums.txt` (mejora #1) | 🔴 RED |
| `TestDefaultTargets_CoversDistributionMatrix` | targets | linux/{amd64,arm64}, darwin/{arm64,amd64}, windows/amd64 (mejora C) | 🔴 RED |
| `TestReleaseOnly_PublishesSingleReleaseForHighestTag` | un release | 1 sola llamada release create, tag más alto (mejora #4) | 🟢 GREEN |
| `TestCrossBuildArgs_InjectsVersionAndStrips` (referencia) | build args | `-X main.Version`, `-trimpath`, `-s -w` (mejoras #2/#3) | ⏸ requiere seam |

Los 🔴 RED requieren la resolución nueva → pasan a GREEN tras implementar el diseño.
Los 🟢 GREEN fijan la **compatibilidad hacia atrás**: la implementación no debe romperlos.

### Impacto en tests existentes
`TestReleaseOnly_*` actuales usan `fakeRunner` con salida única (una URL). Con la
resolución, la primera llamada será `gh repo view` y el `fakeRunner` devolverá la URL →
parseo JSON falla → fallback a origin (regla de backward compatibility) → las aserciones
actuales (`release`/`create`/tag + assets, sin `--repo`) **siguen pasando**. No requieren
cambios.

---

## Checklist de ejecución

- [x] Actualizar `diagrams/GORELEASE_FLOW.md` (resolución automática, sin flags)
- [x] Crear este `PLAN.md`
- [x] Escribir tests RED en `test/gorelease_publish_test.go`
- [x] Escribir test RED de targets (`gorelease_targets_test.go`, matriz completa)
- [x] Escribir tests de mejoras #1/#4 (`UploadsChecksums` RED, `PublishesSingleReleaseForHighestTag` GREEN)
- [ ] Implementar `repoInfo` + `resolvePublishTarget` (DI vía runner, mejora A: una sola consulta al origin)
- [ ] Extender `CreateRelease` con `targetRepo` (añade `--repo` cuando el destino difiere de origin)
- [ ] Conectar en `ReleaseOnly`
- [ ] Añadir `linux/arm64` + `darwin/amd64` a `DefaultTargets()` (mejora C)
- [ ] **#1** Generar `checksums.txt` (SHA256) e incluirlo como asset (seguridad)
- [ ] **#2/#3** `crossBuildArgs`: inyectar `-X main.Version=<tag>` + `-trimpath -ldflags=-s -w`; activar `TestCrossBuildArgs_InjectsVersionAndStrips`
- [x] **#4** Bloqueado: un release para el tag más alto (ya implementado en `GetLatestTag`)
- [x] Corregir README de `tinywasm/app`: quitar `go install`, dejar solo descarga de binario (riesgo B)
- [ ] Actualizar `GORELEASE.md` (sección "Distribución privada → pública" + checksums + versión embebida)
- [ ] Actualizar el help de `cmd/gorelease/main.go`: explicar publicación automática (origin privado → repo público derivado) y checksums
- [ ] `gotest` verde

## Related

- [GORELEASE.md](GORELEASE.md) — WHAT/WHY del comando
- [diagrams/GORELEASE_FLOW.md](diagrams/GORELEASE_FLOW.md) — flujo visual
- [GOPUSH.md](GOPUSH.md) · [CODEJOB.md](CODEJOB.md) — crean el tag que consume gorelease
