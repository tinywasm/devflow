---
PLAN: "fix(gopush): no bloquear el bump de dependientes por replaces ajenos a la lib publicada; nombrar lib/subcarpeta en el reporte de cascada"
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 7233184041072467326
PR: https://github.com/tinywasm/devflow/pull/41
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan — dos bugs en el cascade de `gopush` que rompen el flujo de actualización de dependientes

Plan de ejecución para un agente. El **comportamiento objetivo** ya está descrito
en la documentación (que este plan actualiza y luego implementa):

- Comportamiento y uso: [`docs/GOPUSH.md`](../GOPUSH.md)
- Diagramas y mapa de pruebas: [`docs/diagrams/GOPUSH_FLOW.md`](../diagrams/GOPUSH_FLOW.md)

Implementa el código hasta que coincida con esos documentos (ya editados en §4.3)
y todas las pruebas de §5 pasen.

## 1. Objetivo

`gopush` publicó `tinywasm/widget` en `v0.6.2`. El cascade encontró 4 dependientes
y produjo:

```
📦 tests → skip (other replaces exist) ⏭
📦 components → skip (other replaces exist) ⏭
📦 layout → skip (other replaces exist) ⏭
📦 form → updated ✅
```

Dos bugs, ambos en `UpdateDependentModule` (`go_handler.go`) y su cadena de
objectors (`publish_objector.go`, `go_mod.go`):

- **Bug 1 — un `replace` ajeno bloquea TODO el nodo.** `components` tiene
  `replace github.com/tinywasm/widget => ../widget` (el que sí correspondía
  actualizar) **y además** `replace github.com/tinywasm/css => ../css` (una lib
  distinta, en desarrollo local simultáneo). El objector `GoModHandler` ve *cualquier*
  replace ajeno al lote publicado y devuelve `ActionSkip`: el nodo completo queda
  sin tocar, incluida la dependencia que sí se acababa de publicar. Resultado: el
  `require` de `widget` en `components` se queda pineado en `v0.6.0` mientras dure
  el desarrollo local paralelo de `css`. Cuando el desarrollador termine ese
  desarrollo y quite el replace de `widget`, Go no resuelve un error de compilación
  obvio — resuelve silenciosamente a `v0.6.0`, la versión vieja, no a la `v0.6.2`
  que en realidad se estuvo probando en local. El desarrollador tiene que notar el
  desfase y correr `go get` a mano para volver a alinear el `require` con lo que
  ya validó.
- **Bug 2 — el nombre reportado pierde la librería dueña.** `tests` en la salida
  de arriba es en realidad el `go.mod` propio de `ssr/tests/` (un submódulo interno
  que algunas libs usan para no contaminar el root con dependencias de test — ver
  `docs/GOPUSH.md` §"Internal submodules sync"). `UpdateDependentModule` calcula el
  nombre con `filepath.Base(depDir)`, que solo ve `tests` y no dice a qué librería
  pertenece. Con varias libs en la ruta de búsqueda que tienen ese patrón, la salida
  es ambigua.

## 2. Diagnóstico exacto (ya verificado, no re-investigar)

- Bug 1: `go_mod.go:351-357`, método `(*GoModHandler).ObjectsToPublish`:
  ```go
  func (m *GoModHandler) ObjectsToPublish(ctx PublishContext) (PublishAction, string) {
      m.SetRootDir(ctx.RepoDir)
      if m.HasOtherReplaces(ctx.ModulePaths...) {
          return ActionSkip, ObjectionOtherReplaces
      }
      return ActionNone, ""
  }
  ```
  `ctx.ModulePaths` son los módulos que se acaban de publicar en esta ola (p. ej.
  `github.com/tinywasm/widget`). `HasOtherReplaces` ya excluye esos de la
  comprobación (por eso un replace *sobre el propio módulo publicado* no cuenta) —
  el bug es que cualquier OTRO replace (a una lib no relacionada, p. ej. `css`)
  devuelve `ActionSkip`, y `ActionSkip` hace que `UpdateDependentModule` retorne en
  la línea 434-437 de `go_handler.go` **antes de tocar nada**, incluida la
  dependencia que sí debía actualizarse.
- Bug 2: `go_handler.go:407`:
  ```go
  depName := filepath.Base(depDir)
  ```
  Es la única fuente del nombre usado en las 7 líneas `📦 %s → ...` de esa función
  (líneas 402, 412, 421, 435, 464, 471, 478, 483, 491, 498, 507, 511, 514, 522, 526
  — todas pasan `depName`). Un dependiente cuyo `go.mod` vive en una subcarpeta de
  otra librería (p. ej. `.../tinywasm/ssr/tests/go.mod`) se reporta como `tests`,
  indistinguible de cualquier otro `tests/` en la ruta de búsqueda.

## 3. Decisiones tomadas (defaults fijados, ya no son preguntas)

| # | Decisión |
|---|---|
| Bug 1 — acción | Un replace ajeno al lote publicado ya NO produce `ActionSkip`. Pasa a `ActionDepsOnly`: la lib publicada SÍ se actualiza (replace removido si aplica, `go get`, `go mod tidy`, tests), se commitea `go.mod`+`go.sum`, pero **sin tag y sin propagar** aguas abajo — el repo sigue dependiendo de código local no publicado en otra parte, así que no es candidato a release. El replace ajeno (p. ej. `css`) queda intacto, no se toca. |
| Bug 1 — alcance | Un replace sobre el MISMO módulo que se está publicando sigue sin objetar nada (ya excluido hoy vía `ctx.ModulePaths`); eso no cambia. Solo cambia qué pasa cuando el replace es sobre un módulo distinto. |
| Bug 1 — sesión CODEJOB activa | Sin cambios: sigue siendo `ActionSkip` (repo NO tocado). Es un objector distinto (`CodeJob`, no `GoModHandler`) y no se toca en este plan. |
| Bug 2 — nombre mostrado | `dependentDisplayName(depDir)` sube desde `depDir` buscando el primer directorio con `.git` (raíz del repo real) y devuelve la ruta relativa al padre de esa raíz — p. ej. `ssr/tests` para `.../ssr/tests`, `form` para `.../form` (idéntico al comportamiento actual en el caso común). Si no encuentra `.git` en 20 niveles (o llega a la raíz del filesystem), cae a `filepath.Base(depDir)` — mismo comportamiento de hoy, así que ningún test existente que no cree `.git` se rompe. |
| Docs | `docs/GOPUSH.md` y `docs/diagrams/GOPUSH_FLOW.md` son el contrato objetivo (ver §4.3) — ya quedan editados en este plan; el código debe hacerlos ciertos. |

## 4. Cambios por archivo

### 4.1 `go_mod.go` — Bug 1

Reemplazar el cuerpo de `ObjectsToPublish` (línea 351-357):

```go
func (m *GoModHandler) ObjectsToPublish(ctx PublishContext) (PublishAction, string) {
	m.SetRootDir(ctx.RepoDir)
	if m.HasOtherReplaces(ctx.ModulePaths...) {
		return ActionDepsOnly, ObjectionOtherReplaces
	}
	return ActionNone, ""
}
```

Único cambio: `ActionSkip` → `ActionDepsOnly`. `HasOtherReplaces` y
`ObjectionOtherReplaces` no cambian. No tocar `RemoveReplace` ni
`isLocalReplaceTarget` — ese código ya hace lo correcto (preserva replaces locales
que no son del módulo publicado).

### 4.2 `go_handler.go` — Bug 2

Añadir esta función nueva justo antes de `UpdateDependentModule` (antes de la
línea 406, después del comentario existente de `reportFail`):

```go
// dependentDisplayName returns a name that disambiguates dependents that live
// in a subfolder with their own go.mod (e.g. "ssr/tests") from top-level
// dependents (e.g. "form"). It walks up from depDir to the nearest ".git"
// directory — the repo root — and returns the path relative to that root's
// parent. When no repo root is found (e.g. in tests without a real git repo)
// it falls back to the last path component, matching the previous behavior.
func dependentDisplayName(depDir string) string {
	dir := depDir
	for i := 0; i < 20; i++ {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			parent := filepath.Dir(dir)
			rel, err := filepath.Rel(parent, depDir)
			if err == nil {
				return filepath.ToSlash(rel)
			}
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Base(depDir)
}
```

Y cambiar la línea 407 de:

```go
	depName := filepath.Base(depDir)
```

a:

```go
	depName := dependentDisplayName(depDir)
```

`os` y `filepath` ya están importados en este archivo — no hace falta tocar el
bloque de imports. No tocar ninguna otra línea de `UpdateDependentModule`: las 7+
llamadas a `g.consoleOutput(fmt.Sprintf("📦 %s → ...", depName, ...))` ya usan la
variable `depName`, así que heredan el fix automáticamente.

### 4.3 Documentación (contrato objetivo — editar así, el código de §4.1/4.2 lo hace cierto)

**`docs/GOPUSH.md`** — en la sección "For Go Projects", paso 8, reemplazar la línea:

```
   - **Guard check**: If the dependent has an active `CODEJOB` session or other local `replace`s, it is **skipped** (the repo is NOT touched at all: no `go.mod` write, no `go get`, no tests).
```

por:

```
   - **Guard check**: If the dependent has an active `CODEJOB` session, it is **skipped** (the repo is NOT touched at all: no `go.mod` write, no `go get`, no tests). If it has local `replace`s for OTHER modules (unrelated to the ones just published), the bump still lands: `go.mod`/`go.sum` are updated, tested and committed, but **without a tag** and without propagating to further dependents (deps-only) — replaces on unrelated modules are left untouched.
```

**`docs/diagrams/GOPUSH_FLOW.md`**:

1. En la tabla "Contract → tests", reemplazar la fila:
   ```
   | Other replaces protection: `UpdateDependentModule` does NOT touch the repo at all | [`TestUpdateDependentModule_OtherReplacesLeavesRepoUntouched`](../../test/dependents_guard_test.go) |
   ```
   por:
   ```
   | Other replaces (unrelated modules) go deps-only: bump lands, no tag, no propagation | [`TestUpdateDependentModule_OtherReplacesGoesDepsOnly`](../../test/dependents_guard_test.go) |
   ```

2. En la tabla del objector chain ("Publish-objector chain" / "Per-node cascade
   processing"), reemplazar la fila:
   ```
   | `GoModHandler` | `go.mod` has other local `replace`s | `Skip` |
   ```
   por:
   ```
   | `GoModHandler` | `go.mod` has local `replace`s for modules NOT in this wave's bumps | `DepsOnly` |
   ```

3. En el diagrama mermaid de "Per-node cascade processing", el nodo `N1` dice
   `N1{action == Skip?<br/>session active / other replaces}` — cambiar a:
   ```
   N1{action == Skip?<br/>codejob session active}
   ```
   y el nodo `N5` dice `N5{action == DepsOnly?<br/>dirty tree / PLAN.md pending}` —
   cambiar a:
   ```
   N5{action == DepsOnly?<br/>dirty tree / PLAN.md pending / other replaces}
   ```

4. En "Guard rails", reemplazar el bullet:
   ```
   - **`Skip` nodes (active `CODEJOB` session, other replaces): the repo is NOT
     touched at all** — no `go.mod` write, no `go get`, no tests. Nothing propagates
     downstream.
   ```
   por:
   ```
   - **`Skip` nodes (active `CODEJOB` session): the repo is NOT
     touched at all** — no `go.mod` write, no `go get`, no tests. Nothing propagates
     downstream.
   ```
   y ampliar el bullet de `DepsOnly` (el que empieza con
   "`DepsOnly` nodes run tests as a gate...") agregando al final del párrafo:
   ```
   A repo with a local `replace` for a module OTHER than the ones just published
   (e.g. actively developing two sibling libraries locally at once) also lands here:
   the specific bump it was waiting for is applied and tested, but the release is
   withheld because the repo still depends on unpublished local code elsewhere —
   the unrelated `replace` itself is left untouched.
   ```

5. En "Output behavior" → "Real-time console output", agregar una línea de
   ejemplo mostrando el formato lib/subcarpeta, justo debajo de la línea
   `📦 leaflib → skipped (no published upstreams) ⏭`:
   ```
   📦 ssr/tests → updated ✅
   ```

No tocar `docs/CODEJOB.md` ni `docs/diagrams/CODEJOB_FLOW.md`: el objector
`CodeJob` (sesión activa / `PLAN.md` pendiente) no cambia en este plan.

## 5. Mapa de pruebas (TDD)

Editar primero los tests (rojo), luego el código de §4.1/4.2 (verde). Ningún test
nuevo necesita red, git o go toolchain reales — todo vía `command.Exec` inyectado
(mismo patrón que `TestUpdateDependentModule_DirtyTreeCommitsOnlyGoModAndSum`) o
vía el camino de early-skip que no ejecuta comandos (mismo patrón que
`TestUpdateDependentModule_ActiveSessionLeavesRepoUntouched`).

### 5.1 `test/gomod_handler_test.go` — `TestGoModHandler_ObjectsToPublish`

Bloque actual (líneas ~177-186):

```go
	// with another replace -> ActionSkip
	content2 := content + "replace github.com/other/lib => ../other\n"
	os.WriteFile(gomodPath, []byte(content2), 0644)
	m = devflow.NewGoModHandler() // fresh load
	action, reason = m.ObjectsToPublish(ctx)
	if action != devflow.ActionSkip {
		t.Errorf("expected ActionSkip, got %v (%s)", action, reason)
	}
	if reason != devflow.ObjectionOtherReplaces {
		t.Errorf("expected %q, got %q", devflow.ObjectionOtherReplaces, reason)
	}
```

Reemplazar por:

```go
	// with another replace unrelated to the module being published -> ActionDepsOnly
	// (the bump must still land in go.mod; only the tag/propagation is withheld)
	content2 := content + "replace github.com/other/lib => ../other\n"
	os.WriteFile(gomodPath, []byte(content2), 0644)
	m = devflow.NewGoModHandler() // fresh load
	action, reason = m.ObjectsToPublish(ctx)
	if action != devflow.ActionDepsOnly {
		t.Errorf("expected ActionDepsOnly, got %v (%s)", action, reason)
	}
	if reason != devflow.ObjectionOtherReplaces {
		t.Errorf("expected %q, got %q", devflow.ObjectionOtherReplaces, reason)
	}
```

### 5.2 `test/dependents_guard_test.go` — renombrar y reescribir `TestUpdateDependentModule_OtherReplacesLeavesRepoUntouched`

Esta función (líneas ~343-366) queda obsoleta por el propio cambio de contrato:
ya NO puede afirmar "repo untouched" porque ahora el repo SÍ se toca (deps-only).
Borrar la función completa y reemplazarla por esta (nombre nuevo,
`TestUpdateDependentModule_OtherReplacesGoesDepsOnly`), que sigue el mismo patrón
de mocking de `command.Exec` que `TestUpdateDependentModule_DirtyTreeCommitsOnlyGoModAndSum`
(arriba en el mismo archivo) pero con árbol de trabajo limpio, para aislar que la
única objeción activa es el replace ajeno:

```go
func TestUpdateDependentModule_OtherReplacesGoesDepsOnly(t *testing.T) {
	tmp := t.TempDir()
	depDir := filepath.Join(tmp, "myapp")
	os.MkdirAll(depDir, 0755)
	gomodContent := "module github.com/test/myapp\n\ngo 1.20\n\nrequire github.com/test/mylib v0.0.0\nrequire github.com/test/other v0.0.0\nreplace github.com/test/other => ../other\n"
	os.WriteFile(filepath.Join(depDir, "go.mod"), []byte(gomodContent), 0644)

	var mu sync.Mutex
	var gitCalls [][]string
	originalExec := command.Exec
	defer func() { command.Exec = originalExec }()
	command.Exec = func(name string, args ...string) *exec.Cmd {
		joined := strings.Join(args, " ")
		switch name {
		case "git":
			mu.Lock()
			gitCalls = append(gitCalls, args)
			mu.Unlock()
			switch {
			case strings.HasPrefix(joined, "status --porcelain"):
				// Clean tree: the only objection in play must be the
				// unrelated replace, not a dirty working tree.
				return exec.Command("echo", "")
			case strings.HasPrefix(joined, "diff"):
				// go.mod will change once bumped -> "there ARE changes to commit"
				return exec.Command("false")
			default:
				return exec.Command("true")
			}
		case "go":
			if joined == "version" {
				return exec.Command("echo", "go version go1.20 linux/amd64")
			}
			return exec.Command("true") // get / tidy / generate / list
		case "gotest":
			return exec.Command("echo", "tests ok")
		}
		return originalExec(name, args...)
	}

	mockGit := &MockGitClient{}
	g := newGoHandlerWithMockBackup(t, mockGit)
	g.SetConsoleOutput(func(string) {})
	g.SetRetryConfig(time.Millisecond, 1)

	outcome, err := g.UpdateDependentModule(depDir, []devflow.DepBump{{ModulePath: "github.com/test/mylib", NewVersion: "v0.0.1"}}, "feat: test")
	if err != nil {
		t.Fatalf("other-replaces path must succeed as deps-only, got error: %v", err)
	}
	if outcome.Status != devflow.CascadeStatusDepsOnly {
		t.Errorf("expected status %s, got %+v", devflow.CascadeStatusDepsOnly, outcome)
	}
	if outcome.Reason != devflow.ObjectionOtherReplaces {
		t.Errorf("expected reason %s, got %s", devflow.ObjectionOtherReplaces, outcome.Reason)
	}

	mu.Lock()
	defer mu.Unlock()
	var sawTagCreation, sawPush bool
	for _, args := range gitCalls {
		if len(args) == 0 {
			continue
		}
		if args[0] == "tag" {
			sawTagCreation = true
		}
		if args[0] == "push" {
			sawPush = true
			for _, a := range args[1:] {
				if a == "--tags" {
					sawTagCreation = true
				}
			}
		}
	}
	if sawTagCreation {
		t.Error("deps-only outcome must never create/push a tag")
	}
	if !sawPush {
		t.Errorf("expected a push of the deps-only commit, git calls: %v", gitCalls)
	}
}
```

`sync`, `strings`, `exec`, `command`, `time` ya están importados en este archivo
(los usa `TestUpdateDependentModule_DirtyTreeCommitsOnlyGoModAndSum`). No agregar
imports nuevos.

Buscar cualquier otra referencia a `TestUpdateDependentModule_OtherReplacesLeavesRepoUntouched`
en el repo (`grep -rn OtherReplacesLeavesRepoUntouched .`) y actualizarla — no debe
quedar ninguna.

### 5.3 `test/dependents_guard_test.go` — test nuevo para el nombre lib/subcarpeta

Agregar esta función nueva (puede ir después de
`TestUpdateDependentModule_ActiveSessionLeavesRepoUntouched`, reutiliza el mismo
camino de skip por sesión CODEJOB activa porque no requiere mockear `git`/`go`,
solo importa el `depName` calculado):

```go
func TestUpdateDependentModule_DisplayNameIncludesParentRepo(t *testing.T) {
	tmp := t.TempDir()
	repoDir := filepath.Join(tmp, "ssr")
	depDir := filepath.Join(repoDir, "tests")
	os.MkdirAll(depDir, 0755)
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755) // marks repoDir as the repo root
	gomodContent := "module github.com/test/ssr/tests\n\ngo 1.20\n\nrequire github.com/test/mylib v0.0.0\n"
	os.WriteFile(filepath.Join(depDir, "go.mod"), []byte(gomodContent), 0644)
	planDir := filepath.Join(depDir, "docs")
	_ = os.MkdirAll(planDir, 0755)
	_ = os.WriteFile(filepath.Join(planDir, "PLAN.md"), []byte("---\nPLAN: test\nSTATUS: running\n---\n"), 0644)

	var lines []string
	g, _ := devflow.NewGo(&MockGitClient{})
	g.SetConsoleOutput(func(s string) { lines = append(lines, s) })

	outcome, err := g.UpdateDependentModule(depDir, []devflow.DepBump{{ModulePath: "github.com/test/mylib", NewVersion: "v0.0.1"}}, "feat: test")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if outcome.Status != devflow.CascadeStatusSkipped {
		t.Fatalf("expected status %s, got %s", devflow.CascadeStatusSkipped, outcome.Status)
	}

	found := false
	for _, l := range lines {
		if strings.Contains(l, "ssr/tests") {
			found = true
		}
		if strings.Contains(l, "📦 tests →") {
			t.Errorf("display name lost the parent repo, printed the ambiguous form: %q", l)
		}
	}
	if !found {
		t.Errorf("expected a console line naming %q, got: %v", "ssr/tests", lines)
	}
}
```

## 6. Criterios de aceptación (Definition of Done)

1. `gotest` verde (vet, tests, race) en todo el repo.
2. `TestGoModHandler_ObjectsToPublish` (§5.1), `TestUpdateDependentModule_OtherReplacesGoesDepsOnly`
   (§5.2) y `TestUpdateDependentModule_DisplayNameIncludesParentRepo` (§5.3) existen
   y pasan; se editaron/crearon antes que su código.
3. `grep -rn "OtherReplacesLeavesRepoUntouched" .` no devuelve nada (test viejo
   completamente renombrado/reemplazado, no duplicado).
4. `grep -rn "filepath.Base(depDir)" go_handler.go` no devuelve nada (reemplazado
   por `dependentDisplayName(depDir)`).
5. `docs/GOPUSH.md` y `docs/diagrams/GOPUSH_FLOW.md` reflejan exactamente el texto
   de §4.3 (sin restos de la semántica vieja "other replaces → skip").
6. Ningún test existente que no crea un directorio `.git` cambia de resultado
   (`dependentDisplayName` cae a `filepath.Base` sin `.git`, igual que antes).

## 7. Fuera de alcance

- El objector `CodeJob` (sesión activa / `PLAN.md` pendiente) y el objector `Git`
  (árbol sucio) no cambian: siguen `ActionSkip` y `ActionDepsOnly` respectivamente,
  sin tocar.
- `syncInternalSubmodules` (`go_selfdep.go`) — el mensaje de log
  `"Syncing internal submodule: %s"` (línea 42) usa `filepath.Base(subDir)` con el
  mismo problema de ambigüedad, pero es un log interno del propio repo publicado
  (paso 3 de `gopush`, antes del cascade), no el reporte de dependientes externos
  que motivó este plan. Si se quiere el mismo fix ahí, es un plan aparte.
- No se toca `RunCascade`/`printCascadeReport` (`cascade.go`): esa tabla ya
  imprime `ModulePath` (el import path completo, sin ambigüedad), no `depName`.
