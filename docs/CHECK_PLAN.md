# devflow — PLAN: Recuperación automática de sesión `gh` en codejob (PAT vía `--with-token`)

> Estado: Borrador para revisión · Objetivo: eliminar el device-flow del navegador
> (`Paste this code in browser: XXXX-XXXX`) durante codejob. Cuando la sesión de `gh`
> expira, recuperarla automáticamente con un PAT del keyring vía `gh auth login --with-token`
> — sin interacción, persistente, y arreglando `gh` globalmente (no solo codejob).
>
> ⚠️ Prescriptivo. Ver §5 (Invariantes) y §6 (Aceptación).

---

## 1. Problema (verificado en código)

El flujo de codejob ejecuta comandos `gh` que dependen de la sesión OAuth de `gh`:

- `gh pr view` (`codejob_state.go:171`), `gh pr merge` (`codejob_state.go:205`) en `MergeAndPublish`.
- `gh repo view` (`code_jules.go:397` vía `autoDetectOwnerRepo`) en el dispatch.
- `gh api user` (`github.go:75`), `gh repo view` (`github.go:88,115`).

Cuando la sesión OAuth de `gh` expira, esos comandos **disparan el device flow de GitHub**
(`Paste this code in browser: 014A-7C00`), interactivo por diseño, que bloquea el proceso
esperando input en el navegador y rompe el flujo a medias.

**Por qué `gh auth refresh` NO sirve:** usa el mismo device flow OAuth y vuelve a pedir el
código. No es silencioso.

**Camino correcto (elegido):** detectar la expiración con una llamada que NO abre navegador
(`gh api user`), y recuperar la sesión con `gh auth login --with-token`, que lee un PAT por
stdin, lo persiste en la config de `gh` y NO es interactivo. A partir de ahí TODOS los `gh`
funcionan — incluido el `gh` manual del usuario en la terminal.

### Infraestructura ya existente (reutilizable)

- `Keyring` (`keyring.go:16`) con `Set/Get/Delete` sobre `go-keyring`, servicio `"devflow"`.
- `JulesAuth.EnsureAPIKey` (`codejob_auth.go:51`) ya implementa "leer del keyring; si falta,
  pedir una vez al usuario y persistir" — plantilla exacta a copiar.
- `RunCommandWithStdin` (`executor.go:64`) — pasa input por stdin sin exponerlo en CLI/logs.
  Exactamente lo que `gh auth login --with-token` necesita.

## 2. Objetivo

Cuando un `gh` del flujo codejob falle por sesión expirada, recuperar la sesión
automáticamente desde un PAT del keyring (setup una sola vez, como el Jules key) y reintentar.
Cero device flow, cero navegador, cero comandos que el usuario tenga que escribir en el caso normal.

## 3. Diseño

### 3.1 `GitHubAuth` — gestor de PAT en keyring (nuevo `codejob_gh_auth.go`)

Espejo de `JulesAuth` (`codejob_auth.go`):

```go
const ghTokenKey = "github_pat"

type GitHubAuth struct{ kr *Keyring }

func NewGitHubAuth() (*GitHubAuth, error) { /* NewKeyring(), como JulesAuth */ }

// EnsureToken returns the PAT from the keyring; if absent, prompts once and persists.
func (a *GitHubAuth) EnsureToken() (string, error) {
    tok, err := a.kr.Get(ghTokenKey)
    if err == nil && tok != "" {
        return tok, nil
    }
    fmt.Fprintf(os.Stderr,
        "GitHub token not found. Create a fine-grained PAT (Contents + Pull requests: Read/Write) at %s\nEnter it now: ",
        termLink("https://github.com/settings/tokens", "https://github.com/settings/tokens"))
    tok = readSecret() // same secure stdin reader JulesAuth uses
    if tok == "" {
        return "", fmt.Errorf("no GitHub token provided")
    }
    if err := a.kr.Set(ghTokenKey, tok); err != nil {
        return "", err
    }
    return tok, nil
}

func (a *GitHubAuth) HasToken() bool { /* como HasKey */ }
func (a *GitHubAuth) Reset() error   { return a.kr.Delete(ghTokenKey) }
```

> Scope mínimo del PAT fine-grained: **Contents: Read/Write** y **Pull requests: Read/Write**.
> Documentar en CODEJOB.md.

### 3.2 `ensureGHSession` — detecta expiración y recupera (nuevo, en `github.go`)

```go
// ensureGHSession verifies the gh session and, if expired, restores it non-interactively
// from the keyring PAT via `gh auth login --with-token`. No-op when the session is healthy.
func ensureGHSession() error {
    // Cheap probe that NEVER opens a browser: fails fast if the session is invalid.
    if _, err := RunCommandSilent("gh", "api", "user", "--jq", ".login"); err == nil {
        return nil // session healthy
    }

    auth, err := NewGitHubAuth()
    if err != nil {
        return err
    }
    tok, err := auth.EnsureToken()
    if err != nil {
        return err
    }

    // Restore session non-interactively (token via stdin, never as CLI arg).
    if out, err := RunCommandWithStdin(tok, "gh", "auth", "login", "--with-token"); err != nil {
        return fmt.Errorf("gh auth restore failed (token invalid/expired?). Rotate with: codejob --reset-gh-token\n%s", strings.TrimSpace(out))
    }

    // Verify the restored session works.
    if _, err := RunCommandSilent("gh", "api", "user", "--jq", ".login"); err != nil {
        return fmt.Errorf("gh session still invalid after restore. Rotate with: codejob --reset-gh-token\n%w", err)
    }
    return nil
}
```

> `gh api user` es la sonda porque falla inmediatamente sin abrir navegador cuando no hay
> sesión válida (a diferencia de `gh pr view`/`gh repo view`, que pueden disparar el device flow).

### 3.3 Llamar `ensureGHSession` al inicio de ambos flujos

- `MergeAndPublish` (`codejob_state.go:161`): primera línea del cuerpo, antes de `gh pr view`.
- `Send` (`codejob.go`, antes de `autoDetectTitle()` en `:245`): antes del primer `gh repo view`.

Una sola llamada por flujo; si la sesión está sana, es solo el costo de un `gh api user` (~100ms).

### 3.4 Comando de rotación `--reset-gh-token` (CLI)

En `cmd/codejob`: flag que llama `NewGitHubAuth().Reset()` y vuelve a pedir el PAT. Evita que
el usuario tenga que adivinar cómo borrar el token del keyring cuando el PAT expira (~1 año).

## 4. Pasos

1. Crear `codejob_gh_auth.go` con `GitHubAuth` + `EnsureToken` + `HasToken` + `Reset` (espejo de JulesAuth).
2. Crear `ensureGHSession()` en `github.go` (sonda `gh api user` → restore vía `--with-token`).
3. Llamar `ensureGHSession()` al inicio de `MergeAndPublish` (`codejob_state.go:161`) y `Send` (`codejob.go`).
4. Añadir flag `--reset-gh-token` en `cmd/codejob`.
5. Docs: `CODEJOB.md` sección "Autenticación de GitHub"; `CODEJOB_FLOW.md` nodo de sesión.

## 5. Invariantes / prohibiciones

- **No** uses `gh auth login` (sin `--with-token`) ni `gh auth refresh` — ambos disparan el
  device flow interactivo (el bug que estamos eliminando).
- **No** pases el PAT como argumento CLI (fuga en logs/errores); usa solo `RunCommandWithStdin`.
- **No** uses `gh pr view`/`gh repo view` como sonda de salud — pueden abrir el navegador;
  la sonda debe ser `gh api user`.
- **No** llames `ensureGHSession` fuera del flujo codejob (no afecta gopush/badges/etc.).
- **No** sobreescribas la sesión si `gh api user` ya funciona (no-op cuando está sana).

## 6. Aceptación

1. Con sesión `gh` expirada y un PAT válido en el keyring, `codejob` y `codejob 'msg'`
   recuperan la sesión **sin abrir el navegador** y completan el flujo.
2. Sin PAT en el keyring (y sesión expirada), codejob pide el token **una sola vez** (como el
   Jules key), lo persiste, y restaura la sesión.
3. Con sesión `gh` sana, `ensureGHSession` es no-op (solo la sonda `gh api user`) y el flujo
   es idéntico al actual.
4. Tras `--with-token`, el `gh` manual del usuario en la terminal también queda autenticado.
5. Con PAT inválido/expirado, falla con `"...Rotate with: codejob --reset-gh-token"` — sin device flow.
6. `go build ./... && go test ./...` verde.

## 7. Tests

Añadir `test/gh_auth_test.go` (mockear `ExecCommand` en `executor.go:12` + keyring; sin `gh` real ni red):

1. **Sesión sana → no-op:** mock `gh api user` exitoso; aserta que `ensureGHSession` NO llama
   `gh auth login` y retorna nil.
2. **Expirada + PAT en keyring → restore:** mock `gh api user` falla la 1ª vez y ok la 2ª;
   keyring devuelve PAT; aserta que se invoca `gh auth login --with-token` con el PAT por stdin.
3. **Restore falla (PAT inválido):** ambos `gh api user` fallan; aserta error con
   `"Rotate with: codejob --reset-gh-token"` y que NO se llama `gh pr view`/`gh pr merge`.
4. **PAT ausente → prompt una vez:** keyring vacío; aserta que se invoca el lector de secreto
   una sola vez y se persiste (mock stdin).
5. **Integración con MergeAndPublish:** mock que la sonda falle; aserta que `ensureGHSession`
   corre ANTES de cualquier `gh pr view`/`gh pr merge`.

## 8. Decisión abierta

- ¿PAT clásico vs fine-grained? Recomendado **fine-grained** (scopes Contents + Pull requests,
  por-repo) por mínimo privilegio. Caveat: expira (~1 año) → rotación vía `--reset-gh-token`.
  Documentar el scope exacto en CODEJOB.md.
