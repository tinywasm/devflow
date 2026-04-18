---
name: tinywasm-app
description: Global MCP daemon for tinywasm/app — how to start the server, available tools, IDE auto-config, SSE logs, and diagnostics. Use when working with the tinywasm development environment.
---

# tinywasm/app — MCP Integration

`tinywasm/app` es el orquestador central del entorno de desarrollo. Expone un servidor MCP global persistente en el puerto `3030`.

## Cómo iniciar el MCP

```bash
tinywasm -mcp          # inicia el daemon global (MCP + SSE en :3030)
tinywasm               # abre el TUI cliente (se conecta al daemon en :3030)
```

El daemon persiste entre proyectos. El TUI solo es un visor SSE — Ctrl+C lo cierra sin detener el servidor.

## Configuración IDE (auto-gestionada)

Al iniciar, `app.ConfigureIDEs` escribe automáticamente la configuración MCP en:

| IDE | Archivo |
|-----|---------|
| VS Code | `~/.config/Code/User/mcp.json` o profiles |
| Claude Code | `~/.claude.json` |
| Antigravity | `~/.gemini/antigravity/mcp_config.json` |

Formato Claude Code (`~/.claude.json`):
```json
{
  "mcpServers": {
    "tinywasm": {
      "url": "http://localhost:3030/mcp",
      "type": "http"
    }
  }
}
```

## Auth

El daemon genera y persiste un API key en disco (`cfg.APIKeyPath`).
- `mcp.NewTokenAuthorizer(apiKey)` — activo cuando `apiKey != ""`
- `mcp.OpenAuthorizer()` — activo cuando no hay API key

El token va en el header: `Authorization: Bearer <apiKey>`

## Herramientas MCP registradas

### Global (daemon — siempre disponibles)

| Tool | Resource | Action | Descripción |
|------|----------|--------|-------------|
| `start_development` | `project` | `c` | Inicia/cambia proyecto activo en modo headless |

### Por proyecto (activos cuando hay proyecto corriendo)

| Tool | Resource | Action | Descripción |
|------|----------|--------|-------------|
| `app_rebuild` | `app` | `u` | Recompila WASM + recarga entorno |
| Tools de WasmClient | según módulo | — | Si WasmClient implementa `ToolProvider` |
| Tools de Browser | `browser` | — | Interacción con devbrowser |

### Registro dinámico de tools

```
ProjectToolProxy (mcp.ToolProvider)
    ├── SetActive(providers...)   // llamado al iniciar proyecto
    └── Tools()                   // delegado a providers activos
          ├── Handler.Tools()     → app_rebuild
          ├── WasmClient (si implementa ToolProvider)
          └── BrowserAdapter      → GetMCPTools()
```

## Flujo daemon/cliente

```
tinywasm -mcp  →  runDaemon()
    ├── crea mcp.Server (Auth + SSE)
    ├── registra daemonToolProvider  (start_development)
    ├── registra ProjectToolProxy    (vacío inicialmente)
    └── HTTP :3030
          ├── POST /mcp                → srv.HandleMessage (JSON-RPC 2.0)
          ├── POST /mcp               [tinywasm/state, tinywasm/action interceptados antes]
          ├── GET  /logs              → SSE log stream
          ├── GET  /tinywasm/state    → estado del proyecto activo
          └── POST /tinywasm/action   → keyboard webhooks (q, r, start, stop, restart)

tinywasm (sin flags)  →  clientMode
    ├── detecta daemon en :3030
    ├── conecta GET /logs (SSE)
    └── TUI viewer (Bubble Tea)
          ├── Ctrl+C → detach (daemon sigue corriendo)
          └── q      → POST /tinywasm/action {key:"q"} → detiene proyecto
```

## Endpoints HTTP

| Método | Ruta | Auth | Descripción |
|--------|------|------|-------------|
| POST | `/mcp` | Bearer | JSON-RPC 2.0 MCP + métodos `tinywasm/*` |
| GET | `/logs` | — | SSE stream de logs del proyecto activo |
| GET | `/tinywasm/state` | Bearer | Estado JSON del TUI activo |
| POST | `/tinywasm/action` | Bearer | Dispatch de acciones: `{key, value}` |
| GET | `/version` | — | Versión del daemon |

## SSE (streaming de notificaciones)

- Cambios en lista de tools → `notifications/tools/list_changed`
- Logs del proyecto → canal `"logs"` (consumido por TUI cliente)
- Notificaciones MCP → canal `"mcp"`

## Archivos clave

| Archivo | Rol |
|---------|-----|
| `daemon.go` | `runDaemon()`: inicia MCP global, gestiona auth+SSE+HTTP |
| `mcp_ide.go` | `ConfigureIDEs()`, `WriteMCPConfig()`: auto-config de IDEs |
| `mcp_registry.go` | `ProjectToolProxy`, `BrowserAdapter`, `buildProjectProviders()` |
| `mcp-tools.go` | `Handler.Tools()` → tool `app_rebuild` |
| `sse_adapter.go` | Adapta `tinywasm/sse` → `mcp.SSEPublisher` |
| `start.go` | Wiring: llama `ConfigureIDEs` + inicia daemon o cliente |

## Diagnóstico si el MCP no responde

```bash
# 1. Verificar que el daemon corre
curl http://localhost:3030/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"initialize","id":"1","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test","version":"0"}}}'

# 2. Si no responde, iniciar el daemon
tinywasm -mcp

# 3. Verificar configuración en Claude Code
cat ~/.claude.json | grep -A5 mcpServers
```
