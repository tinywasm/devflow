---
name: form-codegen
description: Model authoring and form generation across tinywasm/model, tinywasm/orm (ormc), tinywasm/form, tinywasm/json, and tinywasm/dom. Use when creating or modifying models/DTOs, input widgets, ormc code generation, field validation, widget assignment, or form API design.
---

# Models + Forms: typed `model.Definition` → ormc → everything else

## The ONE pattern (source of truth)

A model is authored as a **typed `model.Definition` literal**. The var name
MUST end in `Model` — that is how `ormc` discovers it:

```go
import (
    "github.com/tinywasm/form/input"
    "github.com/tinywasm/model"
)

var LoginDataModel = model.Definition{
    Name: "login_data",
    Fields: model.Fields{
        {Name: "email",    Type: model.FieldText, NotNull: true, Widget: input.Email()},
        {Name: "password", Type: model.FieldText, NotNull: true, Widget: input.Password()},
    },
}

var ProductModel = model.Definition{
    Name: "product",
    Fields: model.Fields{
        {Name: "id",    Type: model.FieldInt,  DB: &model.FieldDB{PK: true, AutoInc: true}},
        {Name: "name",  Type: model.FieldText, NotNull: true, Permitted: model.Permitted{Minimum: 2}, Widget: input.Text()},
        {Name: "price", Type: model.FieldFloat, Widget: input.Number()},
    },
}
```

- **DB tables**: fields carry `DB: &model.FieldDB{...}` metadata.
- **Form-only DTOs** (login, filters, …): NO `DB` metadata — same pattern,
  just no table.
- **UI binding**: `Widget: input.Email()` etc. (typed expression from
  `tinywasm/form/input`). A field without `Widget` gets NO input in forms.

Then generate:

```bash
go generate ./...   # runs ormc (//go:generate ormc); install:
go install github.com/tinywasm/orm/cmd/ormc@latest
```

`ormc` parses the Definition literal (including the `Widget:` expressions)
and **generates the concrete Go struct** plus `ModelName()`, `Schema()`,
`Pointers()`, `Validate()`, `EncodeFields()`/`DecodeFields()` (typed codec)
and the `*List` type, into the generated `*_orm.go` file. **Never edit
generated files — they are overwritten on every run.**

## FORBIDDEN (these are the bugs this skill exists to prevent)

- ❌ **Struct tags as source of truth** (`input:"email"`, `db:"pk"` on
  hand-written structs). The Definition literal is typed; tags are the old,
  removed pattern.
- ❌ **Hand-writing the struct or any generated method**
  (`Schema`, `Pointers`, `ModelName`, `Validate`, `Encode/DecodeFields`).
- ❌ **Stdlib `encoding/json`** anywhere. Only `tinywasm/json`, which works
  exclusively through the generated typed codec — a hand-written DTO without
  generated `Encode/DecodeFields` cannot travel.
- ❌ **`form.RegisterInput` as a fix for empty forms.** `form.New` binds
  inputs ONLY via `Field.Widget` from the schema — there is no name matching.
  If a form renders without fields, the Definition is missing `Widget:`
  entries → fix the Definition and regenerate; never patch at the consumer.

## Library roles

| Library | Responsibility | Knows about |
|---|---|---|
| **model** | `Definition`, `Field`, `Fielder`, `Widget` iface, `Permitted`, `ValidateFields`, typed codec contracts | nothing else |
| **orm / ormc** | DB mapping + THE code generator (Definition → struct + methods) | model |
| **form** | `form.New(parentID, &Generated{})` — schema-driven form; SSR + reactive render | model, dom, form/input |
| **form/input** | Concrete widgets with validation (`Email`, `Password`, `Phone`, `Text`, `Number`, `Checkbox`, `Textarea`, …) | model |
| **json** | Zero-reflection codec over generated `Encode/DecodeFields` | model |
| **dom** | Pure HTML layout elements + signals. **NO form functions** (no `Input()`, `Form()`, …) | — |

## Validation flow (same code front + back)

```
model.ValidateFields(action, m)
  → per field: NotNull → Widget.Validate (semantic) → Permitted (chars/length)
```

Runs identically in WASM (`form.Validate()`) and on the server (after decode,
before persistence).

## How to instruct an agent in a PLAN.md

1. Write the `model.Definition` literal(s) — var name ending in `Model`,
   every form-visible field with an explicit `Widget:`.
2. Delete any hand-written struct the Definition replaces (ormc generates it;
   names would collide).
3. Run `go generate ./...` (ormc). Verify the generated schema carries the
   widgets and codecs.
4. Regression test: `form.New(id, &Generated{})` yields exactly the expected
   number of inputs — catches a future regeneration that loses widgets.

## Custom inputs

A project may define custom widgets (same `model.Widget` contract) and use
them in its Definitions like any `input.Xxx()`. They live with the consumer;
stdlib widgets live in `tinywasm/form/input`.
