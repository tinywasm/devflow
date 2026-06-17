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

## Herramientas MCP registradas (16 tools — registro estático al arrancar)

Todas las tools se registran **una sola vez al iniciar el daemon**. No hay `list_changed`.

### Daemon tools (3) — siempre disponibles

| Tool | Descripción |
|------|-------------|
| `start_development` | Inicia/cambia proyecto activo en modo headless |
| `app_info` | URL, dir público, modo WASM, módulos del proyecto activo |
| `app_get_logs` | Logs recientes con filtro de sección (e.g. `BUILD`, `SERVER`) |

### Browser tools (13) — visibles desde el arranque, "no listo" sin proyecto activo

| Tool | Descripción |
|------|-------------|
| `browser_navigate` | Navega a una URL |
| `browser_click_element` | Click en un selector CSS |
| `browser_fill_element` | Rellena un campo de texto |
| `browser_swipe_element` | Gesto swipe en un elemento |
| `browser_evaluate_js` | Ejecuta JavaScript y retorna resultado |
| `browser_get_content` | Obtiene HTML/texto de la página |
| `browser_get_console` | Logs de consola del browser |
| `browser_get_errors` | Errores JS de la página |
| `browser_get_network_logs` | Peticiones de red |
| `browser_get_performance` | Métricas de rendimiento |
| `browser_inspect_element` | Atributos y estilos de un elemento |
| `browser_screenshot` | Captura pantalla |
| `browser_emulate_device` | Emula viewport de dispositivo |

### Arquitectura de registro (daemon, desde v0.5.28)

```
daemon startup
    ├── browser singleton = cfg.BrowserFactory(...)   // una sola instancia de larga vida
    ├── daemonToolProvider (dtp)
    │     ├── start_development
    │     ├── app_get_logs
    │     └── app_info
    ├── BrowserAdapter{dtp.browser}                   // delega a browser.GetMCPTools()
    └── mcp.NewServer([..cfg.McpToolHandlers, dtp, BrowserAdapter])

start_development RPC
    └── runProjectLoop() → onProjectReady(h)
          ├── dtp.setActiveHandler(h)   // app_info / app_get_logs operativos
          └── browser.OpenBrowser(...)  // browser_* operativos
```

> **No existe `ProjectToolProxy`** (eliminado en v0.5.28). El browser singleton expone sus
> tools desde el arranque; retornan error "not ready" hasta que un proyecto está activo.

## Flujo daemon/cliente

```
tinywasm -mcp  →  runDaemon()
    ├── crea mcp.Server (Auth + SSE)
    ├── registra daemonToolProvider + BrowserAdapter (estático)
    └── HTTP :3030
          ├── POST /mcp                → srv.HandleMessage (JSON-RPC 2.0)
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

- Logs del proyecto → canal `"logs"` (consumido por TUI cliente)
- Notificaciones MCP → canal `"mcp"`
- No se emite `notifications/tools/list_changed` (registro estático)

## Archivos clave

| Archivo | Rol |
|---------|-----|
| `daemon.go` | `runDaemon()`: inicia MCP global, gestiona auth+SSE+HTTP; `daemonToolProvider` |
| `mcp_ide.go` | `ConfigureIDEs()`, `WriteMCPConfig()`: auto-config de IDEs |
| `mcp_registry.go` | `BrowserAdapter`, `buildProjectProviders()` (modo standalone) |
| `sse_publisher.go` | Ring buffer `[100]LogEntry`; `RecentLogs(section, limit)` |
| `section-build.go` | Emite `✅ build ok` / `❌ build failed` en sección BUILD |
| `start.go` | Wiring modo standalone: llama `ConfigureIDEs` + `buildProjectProviders` |

## Diagnóstico si el MCP no responde

```bash
# 1. Verificar que el daemon corre
curl http://localhost:3030/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"initialize","id":"1","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test","version":"0"}}}'

# 2. Ver tools disponibles (reemplaza <TOKEN> con TINYWASM_API_KEY del .env)
curl http://localhost:3030/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <TOKEN>" \
  -d '{"jsonrpc":"2.0","method":"tools/list","id":"1","params":{}}'

# 3. Si no responde, iniciar el daemon
tinywasm -mcp

# 4. Verificar configuración en Claude Code
cat ~/.claude.json | grep -A5 mcpServers
```
