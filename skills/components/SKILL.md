---
name: components
description: TinyWasm component creation standards for tinywasm/components. Use when creating or reviewing UI components: file structure, embedding rules, CSS tokens, SSR split, icon registration, and PLAN.md workflow.
---

# TinyWasm Component Creation

## Module

`github.com/tinywasm/components` — located at `tinywasm/components/`.

Each component lives in its own sub-package: `tinywasm/components/<name>/`.

## File Structure (per component)

```
tinywasm/components/
└── mycomponent/
    ├── mycomponent.go        # Struct, Render(), OnMount()
    ├── mycomponent.css       # Scoped styles
    ├── mycomponent_test.go   # Tests (no build tag — backend by default)
    └── ssr.go                # //go:build !wasm — CSS embed, RenderCSS(), IconSvg()
```

No `front.go`. WASM interactivity lives in the main file via `OnMount()`.

## Struct Definition — Embedding Rule

Always embed `dom.Element` as a **value**, never as a pointer.

```go
// ✅ CORRECT
type MyComponent struct {
    dom.Element
    Title string
}

// ❌ WRONG — double heap allocation, nil-guard boilerplate, GC pressure in TinyGo
type MyComponent struct {
    *dom.Element
}
```

Why: TinyGo (final WASM compiler) has a simple GC. Fewer heap objects = fewer pauses.
Value embedding = 1 allocation instead of 2, better cache locality, zero nil panic risk.

## Render Pattern

```go
package mycomponent

import "github.com/tinywasm/dom"

type MyComponent struct {
    dom.Element
    Title string
}

func (c *MyComponent) Render() *dom.Element {
    return dom.Div().
        Class("mycomponent").
        Text(c.Title)
}

// OnMount is called by tinywasm/dom after HTML is injected into the DOM.
// No build tag needed — TinyGo eliminates it as dead code in SSR builds.
func (c *MyComponent) OnMount() {
    if el, ok := dom.Get(c.GetID()); ok {
        el.On("click", func(e dom.Event) { /* ... */ })
    }
}
```

## SSR File (`ssr.go`)

```go
//go:build !wasm

package mycomponent

import _ "embed"

//go:embed mycomponent.css
var css string

func (c *MyComponent) RenderCSS() string { return css }

func (c *MyComponent) IconSvg() map[string]string {
    return map[string]string{
        // Only internal SVG content — no wrapping <svg> tag.
        // Default viewBox: 0 0 16 16. Include viewBox="..." in string to override.
        // MANDATORY: always use fill="currentColor" (or stroke="currentColor") so
        // the icon color can be controlled from CSS via the parent's color property.
        "mycomponent-icon": `<path fill="currentColor" d="..."/>`,
    }
}
```

`IconSvg()` MUST be in `ssr.go`. SVG strings must never reach the WASM binary.
All paths/shapes MUST use `fill="currentColor"` or `stroke="currentColor"` — never hardcode colors in SVG.

`RenderCSS()` ships component-scoped CSS only — it goes to `assetmin`'s `middle` slot. Components MUST NOT declare a `RootCSS()` function: theme `:root` tokens are global state owned by the app or `tinywasm/dom`'s default theme. A third-party `RootCSS()` is silently ignored by `assetmin` with a warning (single-override rule).

**Icon chain: `IconSvg()` → sprite inline en HTML → `<svg><use href="#id">` en `Render()` → CSS controla color/tamaño.**

El framework inyecta el sprite directamente en el `<body>` del HTML. No hay URL `/assets/icons.svg` — el sprite existe solo inline. `href="#id"` siempre resuelve sin request de red.

En `Render()`, referenciar el icono con `<svg><use>`:
```go
dom.Svg(dom.Use().Attr("href", "#mycomponent-icon")).Class("mycomponent-icon")
```

En `mycomponent.css`, solo apariencia — nunca referenciar el sprite por URL:
```css
.mycomponent-icon {
    width: 1em;
    height: 1em;
    fill: currentColor;
}
```

## CSS Conventions

- Class prefix: component name — `mycomponent-*`
- Colors: `var(--color-primary)`, `var(--color-secondary)`, etc.
- Spacing: `var(--mag-pri)`, `var(--mag-sec)`, `var(--mag-cua)`
- Never hardcode values — always use CSS custom properties **without fallback**. Fallbacks break reusability: the component stops inheriting the theme.
- A component MUST NOT define `:root { … }` — that block is owned by the app's `RootCSS()` (or `tinywasm/dom`'s default fallback). Components only consume tokens, never declare them.
- No form-related CSS — use `tinywasm/form` for that.

Theme tokens are defined in `tinywasm/dom/theme.css` and injected into `<head>` by `assetmin` via `dom.RootCSS()`. Available tokens:
```
--color-primary, --color-secondary, --color-tertiary, --color-quaternary
--color-gray, --color-selection, --color-hover, --color-success, --color-error
--menu-width-collapsed, --menu-width-expanded
--title-height, --content-height, --controls-height
--mag-pri, --mag-sec, --mag-cua
```

## No Standard Library in WASM Packages

Use only tinywasm ecosystem modules — never `errors`, `strconv`, `strings`, `time`, `net/http`, `encoding/json`.

| Stdlib | Tinywasm replacement | Docs |
|--------|---------------------|------|
| `errors`, `fmt`, `strconv`, `strings`, `path/filepath` | `github.com/tinywasm/fmt` | [pkg.go.dev](https://pkg.go.dev/github.com/tinywasm/fmt) |
| `encoding/json` | `github.com/tinywasm/json` — zero reflection, requires `fmt.Fielder` (generated by `ormc`) | [pkg.go.dev](https://pkg.go.dev/github.com/tinywasm/json) |
| `time` | `github.com/tinywasm/time` — uses JS `Date` in WASM; `time.Now()` crashes TinyGo | [pkg.go.dev](https://pkg.go.dev/github.com/tinywasm/time) |
| `net/http` | `github.com/tinywasm/fetch` — uses browser `fetch` API in WASM | [pkg.go.dev](https://pkg.go.dev/github.com/tinywasm/fetch) |

## CSS-First Interactivity

Prefer CSS-only solutions (`:checked`, `:focus-within`, `:hover`, `~` sibling selector) over JS/WASM event handlers. Use `OnMount()` only when CSS cannot handle the interaction.

## Tests

```go
func TestMyComponent_Render(t *testing.T) {
    c := &MyComponent{Title: "Hello"}
    html := c.Render().RenderHTML()
    if !strings.Contains(html, "mycomponent") {
        t.Error("expected mycomponent class")
    }
}
```

## When to Create a PLAN.md

Any code change (new component, modifying existing) requires `docs/PLAN.md` at the module root for user review before dispatching to an external agent. Documentation-only changes (`.md` files) can be edited directly.

See `agents-workflow` skill for the full PLAN.md and codejob workflow.
