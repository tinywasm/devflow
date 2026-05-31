# PLAN: Mock DevBackup en Tests

## Problema

Cada vez que se ejecutan los tests de `devflow`, el proceso de backup real se dispara
(FreeFileSync u otro comando configurado en `DEV_BACKUP`). Esto ocurre porque:

1. `Go.backup` es un campo de tipo concreto `*DevBackup` — no hay forma de inyectar un mock.
2. `DevBackup.Run()` lee `DEV_BACKUP` del entorno o `.bashrc` y ejecuta el comando real.
3. Tests en `coverage_test.go` llaman `goHandler.Push(...)` con `skipBackup=false`:
   - Línea 29: `Push("msg", "v0.0.1", true, true, false, **false**, false, false, "")`
   - Líneas 45, 51, 147: igualmente con `skipBackup=false`
4. `go_handler_test.go:200` y `:545` también usan `skipBackup=false`.
5. Resultado: 5 alertas de disco USB cuando el disco de respaldo no está conectado.

## Causa Raíz

`Go.backup` usa el tipo concreto `*DevBackup` en lugar de una interfaz, impidiendo la inyección de dependencia en tests.

```go
// go_handler.go — situación actual (problemática)
type Go struct {
    backup *DevBackup  // tipo concreto, no mockeable
    ...
}
```

## Solución

Introducir interfaz `BackupRunner`, cambiar el campo `backup` a esa interfaz, y proveer `MockDevBackup` para los tests.

---

## Pasos de Ejecución

### Paso 1 — Agregar interfaz `BackupRunner` a `interface.go`

**Archivo:** `devflow/interface.go`

Agregar al final del archivo:

```go
// BackupRunner defines the interface for backup operations.
// Allows mocking in tests to prevent real backup execution.
type BackupRunner interface {
    SetLog(fn func(...any))
    SetCommand(command string) error
    GetCommand() (string, error)
    Run() (string, error)
}
```

---

### Paso 2 — Cambiar campo `backup` en `Go` struct a la interfaz

**Archivo:** `devflow/go_handler.go`

```go
// Antes
backup *DevBackup

// Después
backup BackupRunner
```

El `NewGo()` sigue creando `NewDevBackup()` (implementa la interfaz automáticamente), sin cambio en comportamiento de producción.

---

### Paso 3 — Agregar método `SetBackup` a `Go` struct

**Archivo:** `devflow/go_handler.go`

Agregar después de `SetLog`:

```go
// SetBackup replaces the backup runner (used in tests to inject a mock).
func (g *Go) SetBackup(b BackupRunner) {
    g.backup = b
}
```

---

### Paso 4 — Crear `MockDevBackup` en archivo de mock existente o nuevo

**Archivo:** `devflow/mock_devbackup.go` (nuevo, en package `devflow`)

```go
package devflow

// MockDevBackup is a no-op BackupRunner for use in tests.
type MockDevBackup struct {
    RunCalled    int
    RunResult    string
    RunErr       error
    CommandStored string
}

func (m *MockDevBackup) SetLog(_ func(...any))           {}
func (m *MockDevBackup) SetCommand(cmd string) error     { m.CommandStored = cmd; return nil }
func (m *MockDevBackup) GetCommand() (string, error)     { return m.CommandStored, nil }
func (m *MockDevBackup) Run() (string, error) {
    m.RunCalled++
    return m.RunResult, m.RunErr
}
```

---

### Paso 5 — Inyectar `MockDevBackup` en los tests afectados

**Archivo:** `devflow/test/helpers_test.go`

Agregar helper de construcción:

```go
// newGoHandlerWithMockBackup creates a Go handler with a no-op backup mock.
func newGoHandlerWithMockBackup(t *testing.T, git devflow.GitClient) *devflow.Go {
    t.Helper()
    h, err := devflow.NewGo(git)
    if err != nil {
        t.Fatalf("NewGo: %v", err)
    }
    h.SetBackup(&devflow.MockDevBackup{})
    return h
}
```

**Archivos a actualizar — reemplazar `devflow.NewGo(mockGit)` por `newGoHandlerWithMockBackup(t, mockGit)`:**

| Archivo | Líneas aproximadas |
|---|---|
| `test/coverage_test.go` | Todas las llamadas a `NewGo` |
| `test/go_handler_test.go` | Líneas 153, 197, y otras que usen `NewGo` directamente |
| `test/async_test.go` | Construcción del handler |

---

### Paso 6 — Verificar

```bash
cd devflow && go test ./test/... -run TestGoPush -v
cd devflow && go test ./test/... -v 2>&1 | grep -i backup
```

No debe aparecer ningún proceso de FreeFileSync ni alerta de disco.

---

## Archivos Modificados

| Archivo | Tipo de cambio |
|---|---|
| `devflow/interface.go` | Agregar interfaz `BackupRunner` |
| `devflow/go_handler.go` | Campo `backup BackupRunner`, método `SetBackup` |
| `devflow/mock_devbackup.go` | Nuevo — `MockDevBackup` |
| `devflow/test/helpers_test.go` | Agregar `newGoHandlerWithMockBackup` |
| `devflow/test/coverage_test.go` | Usar mock helper |
| `devflow/test/go_handler_test.go` | Usar mock helper donde `skipBackup=false` |
| `devflow/test/async_test.go` | Usar mock helper si aplica |

## Sin Cambio de Comportamiento en Producción

- `NewGo()` sigue inicializando `NewDevBackup()` — la interfaz la implementa automáticamente.
- `gopush` CLI no cambia.
- Solo los tests inyectan el mock.
