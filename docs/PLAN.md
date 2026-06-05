# PLAN: Integración de `gorelease` con secretos de GitHub (CI/CD)

> Estado: Propuesta · Fecha: 2026-06-05 · Alcance: autenticación de GitHub
> Objetivo: que `gorelease` (y cualquier comando que use `gh`) funcione **tanto en local**
> (keyring + Device Flow interactivo) **como en CI/CD** (token desde variable de entorno /
> secreto de GitHub Actions), sin configuración extra y sin romper el flujo actual.

---

## 1. Problema

Hoy la autenticación está acoplada al **keyring local** y al **Device Flow interactivo**,
lo que hace imposible ejecutar `gorelease` en un pipeline de CI/CD.

Cadena actual de llamadas:

1. `cmd/gorelease/main.go:63` → `devflow.NewGitHub(log)`.
2. `github.go:41` → `authenticator.EnsureGitHubAuth()`.
3. `github_auth.go:73` `EnsureGitHubAuth()`:
   - `NewKeyring()` es **obligatorio**.
   - Si no hay token guardado → `DeviceFlowAuth()`: abre el navegador y hace *polling*
     esperando que **un humano** pegue un código. Inviable sin TTY/navegador.
4. `keyring.go:55` `ensureKeyringAvailable()`: si no hay keyring (caso típico en runners),
   intenta **instalar `gnome-keyring`/`libsecret` con `sudo apt/dnf/pacman`** → falla o
   cuelga en CI.

**Ningún camino lee `GH_TOKEN` / `GITHUB_TOKEN` del entorno**, a pesar de que el `gh` CLI
que devflow usa internamente **ya respeta esas variables de forma nativa**. El keyring se
fuerza *antes* de dejar que `gh` actúe.

### 1.1 Restricción importante: publish cross-repo (privado → público)

`gorelease.go:111` `resolvePublishTarget()` deriva un repo público `<owner>/<folder>` cuando
el origen es privado y publica ahí con `--repo`. El **`GITHUB_TOKEN` automático de GitHub
Actions NO puede** crear releases en *otro* repositorio (está limitado al repo del workflow).
Para ese caso el usuario debe proveer un **PAT** (Personal Access Token) como secret. El plan
y la documentación deben dejarlo explícito.

---

## 2. Decisiones de diseño (confirmadas)

| Tema | Decisión |
|---|---|
| **Estrategia** | **Env var primero**: si `GH_TOKEN`/`GITHUB_TOKEN` está presente, se usa directamente y se **omite el keyring por completo**. Si no, se cae al Device Flow local actual. Detección automática, cero config. |
| **Variables** | `GH_TOKEN` y `GITHUB_TOKEN` (estándar de `gh` CLI y Actions). `GH_TOKEN` tiene prioridad sobre `GITHUB_TOKEN` (mismo orden que el `gh` CLI). |
| **Alcance** | Solo la **autenticación de GitHub** (`EnsureGitHubAuth` / `Keyring`). No se toca la Jules API key ni otros secretos en esta iteración. |
| **Workflow** | Se incluye un `.github/workflows/release.yml` de ejemplo (§5). |

---

## 3. Diseño de la solución

### 3.1 Principio

Introducir un **early-return por variable de entorno** al inicio de `EnsureGitHubAuth()`,
antes de tocar el keyring. Si hay token en el entorno:

1. No se instancia `NewKeyring()` (no se intenta instalar nada con `sudo`).
2. Se configura `gh` con ese token (o se confía en que `gh` lo lea solo — ver §3.3).
3. Se omite el Device Flow.

Esto mantiene **100% intacto** el flujo local (sin env vars → comportamiento idéntico al actual).

### 3.2 Cambios de código

#### a) `github_auth.go` — helper de lectura de entorno

```go
// githubTokenFromEnv returns a GitHub token from the environment, if present.
// Mirrors gh CLI precedence: GH_TOKEN wins over GITHUB_TOKEN.
// Returns ("", false) when neither is set (local interactive mode).
func githubTokenFromEnv() (string, bool) {
    for _, k := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
        if v := strings.TrimSpace(os.Getenv(k)); v != "" {
            return v, true
        }
    }
    return "", false
}
```

#### b) `github_auth.go` — early-return en `EnsureGitHubAuth()`

```go
func (a *GitHubAuth) EnsureGitHubAuth() error {
    // CI/CD path: token provided via environment → skip keyring & Device Flow.
    if token, ok := githubTokenFromEnv(); ok {
        a.log("🔑 Using GitHub token from environment (CI mode)")
        if err := a.configureGhWithToken(token); err != nil {
            return fmt.Errorf("failed to configure gh with env token: %w", err)
        }
        if _, err := RunCommandSilent("gh", "auth", "status"); err != nil {
            return fmt.Errorf("env GitHub token is invalid or lacks scopes: %w", err)
        }
        return nil
    }

    // Local interactive path (unchanged): keyring + Device Flow.
    kr, err := NewKeyring()
    // ... resto del código actual sin cambios ...
}
```

> **Nota sobre `configureGhWithToken` vs dejar que `gh` lea la env:** el `gh` CLI ya respeta
> `GH_TOKEN`/`GITHUB_TOKEN` automáticamente, así que el `gh auth login --with-token` podría ser
> innecesario. Sin embargo lo mantenemos para: (1) validar el token de forma temprana con un
> mensaje de error claro, y (2) cubrir el caso en que sólo `GITHUB_TOKEN` esté seteada (algunas
> versiones de `gh` priorizan distinto). Es idempotente y barato.

#### c) `keyring.go` — sin cambios funcionales obligatorios

Como el early-return evita `NewKeyring()` en CI, no hace falta tocar el keyring. *(Opcional,
mejora de robustez): en `ensureKeyringAvailable()`, no intentar `sudo` cuando se detecte un
entorno no interactivo — fuera de alcance de esta iteración.)*

### 3.3 Flujo resultante

```
EnsureGitHubAuth()
  ├─ ¿GH_TOKEN o GITHUB_TOKEN en entorno?
  │     ├─ Sí → configurar gh + validar (gh auth status) → FIN  [CI/CD]
  │     └─ No → ↓
  └─ NewKeyring() → token guardado? → sí: validar / no: Device Flow  [LOCAL, actual]
```

---

## 4. Impacto y compatibilidad

- **Local sin cambios:** sin env vars, el comportamiento es idéntico (keyring + Device Flow).
- **`git_handler.go` (auth retrier):** `SetAuthRetrier` también llama `EnsureGitHubAuth()`
  (`git_handler.go:89`); se beneficia automáticamente del mismo early-return. Sin cambios extra.
- **Interfaz `GitHubAuthenticator` (`interface.go:18`):** sin cambios de firma → `MockGitHubAuth`
  sigue válido.
- **Documentación a actualizar:** `docs/GORELEASE.md` (sección Requirements/CI) y
  `docs/github/diagrams/GITHUB_AUTH_FLOW.md` (añadir la rama CI).

---

## 5. Workflow de GitHub Actions de ejemplo

Se añadirá como referencia en `docs/GORELEASE.md` (y opcionalmente `.github/workflows/release.yml`).

```yaml
name: release
on:
  push:
    tags: ["v*"]

permissions:
  contents: write   # necesario para crear releases en ESTE repo

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0   # gorelease necesita los tags (GetLatestTag)

      - uses: actions/setup-go@v5
        with:
          go-version: "stable"

      # gh CLI ya viene preinstalado en los runners de GitHub.
      - name: Run gorelease
        run: go run github.com/tinywasm/devflow/cmd/gorelease@latest
        env:
          # Caso A (mismo repo, público): basta el token automático.
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          # Caso B (privado → público, cross-repo): usar un PAT con scope `repo`
          # guardado como secret, p.ej. RELEASE_PAT, y sustituir la línea de arriba por:
          # GH_TOKEN: ${{ secrets.RELEASE_PAT }}
```

### 5.1 Sobre el PAT (caso privado → público)

- El `GITHUB_TOKEN` automático **solo** puede operar sobre el repo del workflow.
- Para el publish cross-repo (`resolvePublishTarget` → `--repo <owner>/<public>`), crear un
  **Fine-grained PAT** o **classic PAT** con scope `repo`, guardarlo como secret (p.ej.
  `RELEASE_PAT`) y exponerlo como `GH_TOKEN`.
- Documentar claramente ambos casos para evitar errores 403 confusos.

---

## 6. Plan de pruebas

1. **Unit `githubTokenFromEnv`:** precedencia `GH_TOKEN` > `GITHUB_TOKEN`, trim de espacios,
   ausencia → `("", false)`. (Sin tocar red ni keyring.)
2. **Unit `EnsureGitHubAuth` (rama CI):** inyectar `configureGhWithToken` / runner mock para
   verificar que con env var **no** se instancia keyring ni Device Flow.
   - *Nota:* requiere hacer `configureGhWithToken` y el `gh auth status` inyectables (p.ej. vía
     un campo función en `GitHubAuth`, igual que `SecretRunner` en `GitHub`). Pequeño refactor.
3. **Regresión local:** sin env vars, los tests existentes (`test/gorelease_*_test.go`,
   `test/cli_test.go`) deben seguir pasando sin cambios.
4. **Manual en CI:** un workflow de prueba con `GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}` sobre un
   repo público que crea un release de un tag de prueba.

---

## 7. Tareas (checklist de implementación)

- [ ] Añadir `githubTokenFromEnv()` en `github_auth.go`.
- [ ] Early-return por env var en `EnsureGitHubAuth()`.
- [ ] (Refactor menor) Hacer inyectables `configureGhWithToken` y la verificación `gh auth status`
      para poder testear la rama CI.
- [ ] Tests unitarios (§6.1, §6.2).
- [ ] Actualizar `docs/GORELEASE.md`: sección "CI/CD" + variables `GH_TOKEN`/`GITHUB_TOKEN` + nota PAT.
- [ ] Actualizar `docs/github/diagrams/GITHUB_AUTH_FLOW.md` con la rama CI.
- [ ] Añadir workflow de ejemplo (`.github/workflows/release.yml` o snippet en docs).
- [ ] Verificar regresión local (`go test ./...`).

---

## 8. Preguntas abiertas / decisiones futuras (fuera de alcance)

1. **Generalizar a otros secretos:** aplicar el mismo patrón env→keyring a la Jules API key
   (`codejob_auth.go`) y a cualquier secreto. Hoy queda fuera de alcance (decisión: "Solo auth GitHub").
2. **Endurecer `keyring.go`:** evitar el intento de `sudo apt/dnf/pacman` en entornos no
   interactivos / detectar `CI=true` para fallar rápido con mensaje claro.
3. **Soporte de `gh` no instalado en CI:** los runners de GitHub ya traen `gh`; si se ejecuta en
   otro CI (GitLab, self-hosted) habría que documentar la instalación de `gh` o evaluar usar la
   API REST directamente.
