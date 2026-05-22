---
name: form-codegen
description: Unified form validation architecture across tinywasm/fmt, tinywasm/dom, tinywasm/form, tinywasm/json, and tinywasm/orm. Use when working on input widgets, ormc code generation, field validation, widget assignment, or form API design.
---

# Form + Codegen Unified Architecture

## Overview

Form creation and validation uses the same code on frontend (WASM) and backend (server). The `fmt.Widget` interface is the bridge — it carries validation logic AND UI rendering capability.

**One way to create forms: `form.New(parentID, &struct)` (schema-driven).** No declarative API.

## Library Roles

| Library | Responsibility | Knows About |
|---|---|---|
| **fmt** | Field, Permitted, Widget interface, ValidateFields() | Nothing else |
| **dom** | HTML layout elements (Div, H1, Button, Nav...). NO form functions | fmt only |
| **form** | Schema-driven form creation via `form.New()` | fmt, dom, form/input |
| **form/input** | Concrete widgets with validation (Email, Text, etc.) | fmt only |
| **json** | Zero-reflection JSON codec using Fielder | fmt only |
| **orm** | DB mapping + ormc code generator | fmt only (generated code imports form/input) |

## Key Design Rule

**dom does NOT have form functions.** No Email(), Form(), Password(), Select(), Textarea(), Input() in dom. These are form concerns with validation — they live in form/input as widgets. dom only provides pure HTML layout elements.

## Widget Assignment: Go Type Defaults + Explicit Override

ormc assigns widgets based on Go type defaults. No Resolver, no aliases, no name matching.

**Only structs with `// ormc:form` or `// ormc:formonly` directive get widgets.**

| Directive | DB/ORM | Widgets |
|---|---|---|
| _(none)_ | yes | no |
| `// ormc:form` | yes | yes |
| `// ormc:formonly` | no | yes |

### Defaults by Go type

| Go type | Default Widget |
|---|---|
| `string` | `input.Text()` |
| `int`, `int64`, etc. | `input.Number()` |
| `float32`, `float64` | `input.Number()` |
| `bool` | `input.Checkbox()` |

### Tag override

- `input:"email"` → uses `input.Email()` instead of default
- `input:"textarea,required,min=5"` → override + Permitted modifiers
- `input:"required,min=5"` → keep default widget + modifiers only
- `input:"-"` → excluded from form (no widget)

## Validation Flow (Same Code Front + Back)

```
fmt.ValidateFields(action, model)
  → for each field in Schema():
    → field.Validate(value)
      → 1. NotNull check (required)
      → 2. Widget.Validate(value) — semantic (email format, date format)
      → 3. Permitted.Validate(field, value) — characters, length
```

Runs identically in:
- **Frontend:** form.Validate() in WASM
- **Backend:** after orm.Insert() or manual validation

## Example

```go
// ormc:form
type User struct {
    ID        string  `db:"pk"`              // string → input.Text()
    Email     string  `input:"email"`        // override → input.Email()
    Bio       string  `input:"textarea"`     // override → input.Textarea()
    Age       int     `db:"not_null"`        // int → input.Number()
    Active    bool                           // bool → input.Checkbox()
    SecretKey string  `input:"-"`            // excluded from form
}
```

## Custom Inputs (web/input/)

Projects can define custom widgets in `web/input/` that override stdlib inputs. ormc discovers them via AST scanning. Custom takes priority over stdlib.

## Cross-Library Execution Order

When making changes that span libraries:

1. **dom** first (remove form functions — unblocks clean separation)
2. **fmt** if interface changes needed (currently stable)
3. **form/input** (currently complete — Permitted cleanup done)
4. **orm** (pending: widget assignment + tag support)
5. **json** only if codec changes needed (currently stable)

## ormc Usage in PLAN.md (for agents)

`ormc` is a CLI code generator. **Agents must never write `Schema()`, `Pointers()`, `ModelName()`, or `Validate()` by hand** — these are always generated.

### Installation

```bash
go install github.com/tinywasm/orm/cmd/ormc@latest
```

### What ormc generates (per directive)

| Directive | Generated methods |
|---|---|
| `// ormc:form` | `ModelName()`, `Schema()` (with widgets), `Pointers()`, `Validate()`, `*List` type |
| `// ormc:formonly` | `Schema()` (with widgets), `Pointers()`, `Validate()`, `*List` type — NO `ModelName()` |
| _(none)_ | `ModelName()`, `Schema()` (no widgets), `Pointers()`, `Validate()`, `*List` type |

Output file: `model_orm.go` in the same package. **Never edit `model_orm.go` — it is overwritten on each `ormc` run.**

### How to instruct an agent

In `PLAN.md`, always say:

1. Add the struct with the correct directive to `model.go` (or the appropriate model file).
2. Run `ormc` from the module root.
3. Do NOT write `Schema()`, `Pointers()`, `ModelName()`, or `Validate()` manually.

Example instruction in a plan:

```
Add to `modules/foo/model.go`:

    // ormc:form
    type Foo struct {
        ID   int64
        Name string
    }

Then run:

    ormc

ormc will generate Schema(), Pointers(), ModelName(), Validate() in model_orm.go.
Do NOT write these methods manually.
```

### ID field convention

`ormc` auto-detects `ID` as primary key (auto-increment). No `db:"pk"` tag needed for the standard `ID int64` pattern. Only add `db:` tags to override defaults.

## Plans Location

- dom: `tinywasm/dom/docs/PLAN.md` — pending: remove form functions
- form: `tinywasm/form/docs/PLAN.md` — complete
- orm: `tinywasm/orm/docs/PLAN.md` — pending execution
- Flow diagram: `tinywasm/orm/docs/diagrams/ORMC_FLOW.md`
