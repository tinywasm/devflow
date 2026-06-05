# PLAN: Manejador unificado de credenciales (`SecretStore`)

> Estado: Aprobado para implementar · Fecha: 2026-06-05
> Objetivo: introducir **un único manejador de credenciales** (`SecretStore`) como pieza central
> de devflow, con resolución **`entorno → keyring`**, consumido por **todas** las herramientas
> (`gorelease`, `gopush`, `codejob`, …). Esto desbloquea su ejecución en CI/CD y nube sin
> interacción humana, manteniendo intacta la experiencia local actual.

> ⚠️ **Este documento es prescriptivo.** El agente que lo implemente debe ceñirse a las firmas,
> nombres, rutas e invariantes aquí definidos. Si algo no está especificado, **no se inventa**:
> se detiene y se pregunta. Ver §9 (Invariantes / Prohibiciones) y §11 (Criterios de aceptación).

---

## 1. Enfoque (leer antes que nada)

- **NO** es un trabajo por fases. **NO** se "arregla gorelease primero y luego se crea el manejador".
- Se construye **primero** el manejador `SecretStore` como componente central, y **a continuación**
  se migran los consumidores (`EnsureGitHubAuth`, `EnsureAPIKey`) para que lo usen.
- Una sola regla de precedencia, en un solo sitio, testeada de forma aislada. Sin lógica de
  resolución de credenciales duplicada por herramienta.

---

## 2. Problema actual

Cada herramienta resuelve sus credenciales por su cuenta y **todas atadas al keyring + modo
interactivo**, lo que impide ejecutarlas en CI/CD o nube (sin TTY, sin navegador, sin keyring):

| Consumidor | Secreto | Resolución actual | Bloqueante headless |
|---|---|---|---|
| `gorelease` / `gopush` | GitHub token | `github_auth.go:73` `EnsureGitHubAuth` → `NewKeyring()` (obligatorio) → si falta, **Device Flow** (navegador + polling) | navegador interactivo |
| `codejob` | Jules API key | `codejob_auth.go:53` `EnsureAPIKey` → keyring → si falta, **`term.ReadPassword`** | exige TTY humano |

Hechos relevantes del código:

- `keyring.go:55` `ensureKeyringAvailable()` intenta instalar `gnome-keyring`/`libsecret` con
  **`sudo apt/dnf/pacman`** si el keyring no existe → falla/cuelga en runners.
- **Ningún camino lee variables de entorno** (`GH_TOKEN`, `GITHUB_TOKEN`, `JULES_API_KEY`).
- `github_auth.go:75` y `codejob_auth.go:28` (`NewJulesAuth`) llaman `NewKeyring()` **de forma
  anticipada**, antes de comprobar si hay credencial en el entorno → el `sudo` se dispara aun
  cuando el secreto ya está disponible por env var.

---

## 3. Arquitectura objetivo

```
                 ┌──────────────────────────────┐
                 │        SecretStore           │   ← pieza central (nueva)
                 │  Get(name) → env → keyring   │
                 └──────────────┬───────────────┘
                                │ (keyring lazy: solo si falla el entorno)
          ┌─────────────────────┼─────────────────────┐
          │                     │                      │
   EnsureGitHubAuth      EnsureAPIKey (Jules)    (futuros consumidores)
   (github_auth.go)      (codejob_auth.go)
          │                     │
   gorelease / gopush       codejob
```

Regla única de resolución de `SecretStore.Get(name)`:

1. Probar las variables de entorno asociadas al secreto, en orden. La primera no vacía gana
   (`source = env`). **No se toca el keyring.**
2. Si ninguna env var está presente, inicializar el keyring **de forma perezosa** y leer su clave
   (`source = keyring`).
3. Si tampoco está en el keyring → `source = none` + error de "no encontrado" (el consumidor decide
   si lanza adquisición interactiva o falla).

El **modo interactivo** (Device Flow, `term.ReadPassword`) se mantiene **únicamente** como paso de
*adquisición cuando el secreto no existe en ningún backend Y hay TTY*. La **lectura** siempre pasa
por `SecretStore`.

---

## 4. Especificación del componente `SecretStore`

Crear un archivo nuevo: **`secret_store.go`** (paquete `devflow`).

### 4.1 Tipos

```go
// SecretSource indica de dónde se resolvió un secreto.
type SecretSource int

const (
    SourceNone    SecretSource = iota // no encontrado
    SourceEnv                         // variable de entorno
    SourceKeyring                     // keyring del sistema
)

// secretSpec describe cómo se resuelve un secreto lógico.
type secretSpec struct {
    keyringKey string   // clave en el keyring (compatibilidad con valores ya guardados)
    envKeys    []string // variables de entorno a probar, en orden de prioridad
}
```

### 4.2 Registro de secretos (única fuente de verdad)

```go
// Nombres lógicos de los secretos gestionados.
const (
    SecretGitHubToken = "github_token"
    SecretJulesAPIKey = "jules_api_key"
)

// secretRegistry mapea cada secreto lógico a su spec. Es la ÚNICA fuente de verdad
// para nombres de env var y claves de keyring. No duplicar estos literales en otros archivos.
var secretRegistry = map[string]secretSpec{
    SecretGitHubToken: {keyringKey: "github_token", envKeys: []string{"GH_TOKEN", "GITHUB_TOKEN"}},
    SecretJulesAPIKey: {keyringKey: "jules_api_key", envKeys: []string{"JULES_API_KEY"}},
}
```

> Las `keyringKey` (`github_token`, `jules_api_key`) **deben coincidir exactamente** con las
> constantes ya existentes (`githubTokenKey` en `github_auth.go:28`, `julesAPIKeyKey` en
> `codejob_auth.go:11`) para no invalidar credenciales ya guardadas por usuarios.

### 4.3 Struct y API pública

```go
// SecretStore resuelve credenciales con precedencia entorno → keyring.
// El keyring se inicializa de forma perezosa: NUNCA se toca si la credencial
// está disponible por variable de entorno (clave para CI/CD).
type SecretStore struct {
    log func(...any)
    kr  *Keyring // lazy; nil hasta el primer fallback a keyring
}

// NewSecretStore crea el manejador. NO inicializa el keyring (sin coste ni side effects).
func NewSecretStore() *SecretStore

// SetLog asigna el logger (propaga a Keyring cuando se inicialice).
func (s *SecretStore) SetLog(fn func(...any))

// Get resuelve el valor del secreto `name`. Devuelve el valor, su origen y error.
//   - Si name no está en secretRegistry → error (programación).
//   - Si se encuentra en env → (valor, SourceEnv, nil) SIN tocar keyring.
//   - Si no hay env pero sí keyring → (valor, SourceKeyring, nil).
//   - Si no está en ningún backend → ("", SourceNone, ErrSecretNotFound).
func (s *SecretStore) Get(name string) (string, SecretSource, error)

// Set persiste el valor en el keyring (único backend escribible).
// Usado por la adquisición interactiva tras obtener un secreto nuevo.
func (s *SecretStore) Set(name, value string) error

// Delete elimina el valor del keyring (p.ej. token inválido).
func (s *SecretStore) Delete(name string) error
```

```go
// ErrSecretNotFound se devuelve cuando un secreto conocido no está en ningún backend.
var ErrSecretNotFound = errors.New("secret not found in environment or keyring")
```

### 4.4 Comportamiento exacto de `Get`

1. `spec, ok := secretRegistry[name]`; si `!ok` → `fmt.Errorf("unknown secret %q", name)`.
2. Para cada `e` en `spec.envKeys`: si `strings.TrimSpace(os.Getenv(e)) != ""` → devolver ese valor
   con `SourceEnv`. **No inicializar keyring.**
3. Inicializar keyring perezosamente (`s.keyring()` → `NewKeyring()` una sola vez, cachear).
   Si `NewKeyring()` falla → devolver ese error (no envolver en `ErrSecretNotFound`).
4. `v, err := s.kr.Get(spec.keyringKey)`: si `err == nil && v != ""` → `SourceKeyring`.
   En cualquier otro caso → `("", SourceNone, ErrSecretNotFound)`.

`Set`/`Delete` resuelven `spec.keyringKey` vía registro e inicializan keyring perezosamente.

### 4.5 Detección de entorno interactivo (robustez headless)

Añadir helper en `secret_store.go`:

```go
// IsInteractive indica si hay un TTY donde solicitar credenciales al usuario.
func IsInteractive() bool {
    return term.IsTerminal(int(os.Stdin.Fd()))
}
```

Los consumidores la usan para **fallar rápido con mensaje accionable** en vez de colgarse cuando
falta un secreto y no hay TTY (ver §5).

---

## 5. Migración de los consumidores

### 5.1 `github_auth.go` — `GitHubAuth`

- `GitHubAuth` deja de crear keyring directamente; pasa a tener un `store *SecretStore`.
- `NewGitHubAuth()` crea `store: NewSecretStore()`. (Sin cambio de firma.)
- `EnsureGitHubAuth()` se reescribe así (lógica, no literal):

```
1. v, src, err := a.store.Get(SecretGitHubToken)
2. si err == nil && v != "":
     - configureGhWithToken(v); validar con `gh auth status`.
     - si válido → return nil.
     - si inválido:
         · si src == SourceEnv → return error claro ("env GH_TOKEN/GITHUB_TOKEN inválido o sin scopes")
           (NO borrar nada, NO Device Flow: en CI no hay humano).
         · si src == SourceKeyring → a.store.Delete(SecretGitHubToken) y continuar al paso 3.
3. (secreto ausente o keyring inválido) Adquisición interactiva:
     - si !IsInteractive() → return error accionable:
         "no GitHub token found; set GH_TOKEN or GITHUB_TOKEN, or run locally to authenticate"
     - DeviceFlowAuth(...) [lógica ACTUAL sin cambios] → token
     - a.store.Set(SecretGitHubToken, token)
     - configureGhWithToken(token); return.
```

- `DeviceFlowAuth` **mantiene su lógica actual**; solo cambia que persiste con `store.Set` en lugar
  de `kr.Set`. Su firma `DeviceFlowAuth(kr *Keyring)` se ajusta a `DeviceFlowAuth()` usando
  `a.store` (verificar y actualizar llamadas internas).

### 5.2 `codejob_auth.go` — `JulesAuth`

- `JulesAuth` cambia el campo `kr *Keyring` por `store *SecretStore`.
- `NewJulesAuth() (*JulesAuth, error)`: **mantener la firma** (no romper llamadas), pero internamente
  usar `NewSecretStore()` (que **no** inicializa keyring) y devolver `nil` error.
- `HasKey()`: `_, _, err := a.store.Get(SecretJulesAPIKey); return err == nil`.
- `EnsureAPIKey()`:

```
1. v, _, err := a.store.Get(SecretJulesAPIKey)
2. si err == nil && v != "" → return v, nil
3. (ausente) si !IsInteractive() → return error accionable:
     "Jules API key not found; set JULES_API_KEY"
4. term.ReadPassword(...) [ACTUAL] → key; a.store.Set(SecretJulesAPIKey, key); return key.
```

### 5.3 `github.go` / `cmd/gorelease/main.go` / `git_handler.go`

- **Sin cambios de lógica.** `NewGitHub` sigue llamando `authenticator.EnsureGitHubAuth()`
  (`github.go:41`) y el auth-retrier de git (`git_handler.go:89`) hereda el nuevo comportamiento
  automáticamente.
- Verificar que no haya otras referencias directas a `NewKeyring()` desde los consumidores migrados.

---

## 6. Contrato de variables de entorno (documentar)

| Secreto | Env vars (prioridad) | Clave keyring | Notas |
|---|---|---|---|
| GitHub token | `GH_TOKEN`, luego `GITHUB_TOKEN` | `github_token` | Estándar de `gh` CLI y GitHub Actions. |
| Jules API key | `JULES_API_KEY` | `jules_api_key` | Para `codejob` headless. |

Nota cross-repo de `gorelease` (privado → público): el `GITHUB_TOKEN` automático de Actions solo
opera sobre el repo del workflow; para publicar en otro repo (`resolvePublishTarget` + `--repo`) se
necesita un **PAT** con scope `repo` expuesto como `GH_TOKEN`. devflow **solo lee** `GH_TOKEN`/
`GITHUB_TOKEN`; el nombre del secret en Actions (p.ej. `RELEASE_PAT`) es libre y se mapea a `GH_TOKEN`.

---

## 7. Workflows de ejemplo (para docs)

```yaml
# gorelease en CI
- name: Run gorelease
  run: go run github.com/tinywasm/devflow/cmd/gorelease@latest
  env:
    GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}   # cross-repo: usar ${{ secrets.RELEASE_PAT }}
```

```yaml
# codejob en CI
- name: Run codejob
  run: go run github.com/tinywasm/devflow/cmd/codejob@latest ...
  env:
    GH_TOKEN:       ${{ secrets.GITHUB_TOKEN }}
    JULES_API_KEY:  ${{ secrets.JULES_API_KEY }}
```

---

## 8. Plan de pruebas (obligatorio)

Crear `test/secret_store_test.go` (paquete de test externo, como el resto de `test/`).

1. **Precedencia env:** con `GH_TOKEN` y `GITHUB_TOKEN` seteadas → gana `GH_TOKEN`, `source==SourceEnv`.
2. **Trim:** env var con espacios/`"\n"` → se devuelve el valor recortado; env var solo-espacios
   se trata como ausente.
3. **Env ausente → no keyring:** inyectar un keyring falso/contador y verificar que con env var
   presente **no** se invoca el keyring (clave anti-`sudo` en CI).
4. **Secreto desconocido:** `Get("nope")` → error de "unknown secret".
5. **No encontrado:** sin env y sin keyring → `ErrSecretNotFound`, `source==SourceNone`.
6. **Consumidores (rama no interactiva):** con `IsInteractive()==false` y secreto ausente,
   `EnsureGitHubAuth`/`EnsureAPIKey` devuelven el error accionable y **no** intentan Device Flow /
   `ReadPassword`.

> Para los puntos 3 y 6 puede ser necesario inyectar dependencias (un campo función o una pequeña
> interfaz de keyring) en `SecretStore`/consumidores, igual que `SecretRunner` en `GitHub`. Hacerlo
> de forma mínima y consistente con el patrón existente.

**Regresión:** sin env vars y con TTY, el flujo local debe ser idéntico al actual. Todos los tests
existentes (`test/gorelease_*_test.go`, `test/cli_test.go`, `test/llm_skill_test.go`, etc.) deben
seguir pasando sin modificación.

---

## 9. Invariantes y prohibiciones (NO inventar)

- **NO** cambiar el `keyringService = "devflow"` ni las claves `github_token` / `jules_api_key`.
- **NO** modificar la lógica del Device Flow (`requestDeviceCode`, `pollForToken`, `openBrowser`,
  `configureGhWithToken`) ni del `term.ReadPassword`; solo cambia desde dónde se leen/persisten.
- **NO** introducir backends nuevos (Vault, AWS, `.env`/`dotenv.go`) en esta entrega. El registro
  queda preparado para ello, pero su implementación está **fuera de alcance**.
- **NO** cambiar firmas públicas existentes salvo las explícitamente indicadas en §5
  (`DeviceFlowAuth`). `NewGitHubAuth`, `NewJulesAuth`, `EnsureGitHubAuth`, `EnsureAPIKey`,
  `HasKey`, y las interfaces de `interface.go` mantienen su firma.
- **NO** añadir flags de CLI nuevos (`--ci`, etc.): la detección es automática (env → keyring,
  + `IsInteractive`).
- **NO** ejecutar `sudo`/instalaciones cuando la credencial venga del entorno (garantizado por la
  inicialización perezosa del keyring).
- Mantener el estilo del código existente (logging vía `log func(...any)`, manejo de errores con
  `fmt.Errorf("...: %w", err)`).

---

## 10. Checklist de implementación (orden recomendado)

- [ ] Crear `secret_store.go`: tipos, `secretRegistry`, `SecretStore` (Get/Set/Delete lazy),
      `ErrSecretNotFound`, `IsInteractive`.
- [ ] Tests `test/secret_store_test.go` (§8.1–§8.5).
- [ ] Migrar `github_auth.go` (`GitHubAuth.store`, `EnsureGitHubAuth`, `DeviceFlowAuth`) según §5.1.
- [ ] Migrar `codejob_auth.go` (`JulesAuth.store`, `EnsureAPIKey`, `HasKey`) según §5.2.
- [ ] Añadir tests de rama no interactiva (§8.6).
- [ ] `go build ./... && go test ./...` en verde (regresión incluida).
- [ ] Docs: `docs/GORELEASE.md` (sección CI/CD), `docs/CODEJOB.md` (env `JULES_API_KEY`),
      `docs/github/diagrams/GITHUB_AUTH_FLOW.md` (rama entorno), y workflows de ejemplo (§7).
- [ ] Verificar que no quedan llamadas directas a `NewKeyring()` en los consumidores migrados.

---

## 11. Criterios de aceptación (Definition of Done)

1. `SecretStore.Get` resuelve `entorno → keyring` con la precedencia de §4.2 y **no toca el keyring
   cuando hay env var** (verificado por test).
2. `gorelease` se ejecuta de principio a fin en CI con solo `GH_TOKEN` en el entorno, sin keyring,
   sin navegador y sin `sudo`.
3. `codejob` resuelve la Jules API key y el token de GitHub desde el entorno en CI, sin TTY.
4. En local (con TTY, sin env vars) el comportamiento es **idéntico** al actual: Device Flow para
   GitHub y `ReadPassword` para Jules, persistiendo en keyring.
5. Cuando falta un secreto y no hay TTY, el proceso falla con un mensaje accionable que nombra la
   variable de entorno a definir (no se cuelga ni intenta instalar keyring).
6. `go test ./...` pasa, incluyendo los tests nuevos y los existentes sin modificar.
