# Plan — Sync del arnés embebido en cada `AGENTS.md` (fuente única, cero deriva)

> Herramienta de `tinywasm/devflow` que mantiene el **bloque canónico del arnés**
> embebido e idéntico dentro del `AGENTS.md` de cada repositorio, desde una **fuente
> única**, sin editar N repos a mano y sin que se desincronicen con el tiempo.

---

## Reglas de Desarrollo

Las reglas del arnés viven en el **`AGENTS.md` de la raíz de esta librería** — que
esta misma herramienta genera y mantiene (se hace *dogfooding*: devflow es el primer
consumidor). Este PLAN no las repite; describe solo el *cómo*.

---

## El problema

El arnés debe estar **embebido, en inglés, dentro de cada `AGENTS.md`**, porque cada
librería es un repositorio independiente: un agente que trabaja solo en ese repo no
tiene acceso a ningún documento global. Una referencia local (p.ej. "ver
`ARNES_DE_CONSTRUCCION.md`") es un enlace roto para ese agente.

Pero embeber a mano el mismo bloque en decenas de `AGENTS.md` **garantiza deriva**:
con el tiempo cada copia se edita por separado y dejan de ser el mismo arnés — justo
lo que el arnés prohíbe (una sola forma, no duplicar). La solución: **una fuente
única** y una herramienta que la **propaga embebida** por marcadores, de forma
idempotente. DRY en el origen, autocontenido en cada destino.

---

## Diseño

### 1. Fuente única del bloque canónico

Un archivo con el bloque del arnés en inglés vive **una sola vez** dentro de devflow
y se incrusta en el binario con `//go:embed` (mismo patrón que las skills):

```
devflow/agents/HARNESS.md      # texto canónico del arnés, en inglés
```

```go
//go:embed agents/HARNESS.md
var harnessBlock string
```

Editar el arnés = editar **este** archivo. Nada más.

### 2. Convención de marcadores en `AGENTS.md`

El bloque se inserta en una región delimitada por comentarios; todo lo de fuera es
contenido propio de cada librería (reglas específicas, testing, etc.) y no se toca:

```markdown
<!-- BEGIN tinywasm-harness — generado por devflow; no editar dentro -->
… bloque canónico del arnés …
<!-- END tinywasm-harness -->
```

Reglas de la inserción (idempotente):

- Si existen los marcadores → se reemplaza **solo** lo que hay entre ellos.
- Si no existen → se insertan justo después del título `# Agent Guide — …` (o al
  inicio si no hay título), con los marcadores.
- Si `AGENTS.md` no existe → se crea con título + bloque.
- Correr la herramienta dos veces seguidas no produce diferencias (idempotencia).

Este es el mismo mecanismo de marcadores que ya usa el tool `badges` sobre el
`README.md`, reutilizado — no se inventa uno nuevo.

### 3. Punto de ejecución — que nadie tenga que "acordarse"

El arnés dice: un paso obligatorio que haya que recordar es un hueco. Por eso la
sincronización **no** es un comando suelto que se te olvida correr:

- **Paso pre-publicación en `gopush`.** Antes de cada publicación, `gopush`
  reinyecta el bloque en el `AGENTS.md` del repo. Así, cualquier repo que se publique
  queda al día automáticamente; la deriva se auto-corrige en cada release.
- **Modo bajo demanda** (secundario): una invocación explícita para reinyectar sin
  publicar (útil en un barrido puntual del ecosistema).

---

## Pasos de implementación

1. Crear `devflow/agents/HARNESS.md` con el bloque canónico del arnés en inglés
   (los 7 principios + "por qué arnés y no manual" + la prueba de fuego).
2. Función `SyncHarness(agentsPath string) (changed bool, err error)`: lee el
   `AGENTS.md`, reemplaza/inserta la región entre marcadores con `harnessBlock`,
   escribe solo si cambió. Reutilizar el helper de marcadores de `badges`.
3. Wire en `gopush`: llamar `SyncHarness` en la fase pre-publicación; si cambia el
   `AGENTS.md`, incluirlo en el commit de release.
4. Exponer el modo bajo demanda (subcomando o flag) para barridos.
5. *Dogfooding*: generar el `AGENTS.md` de devflow con la propia herramienta.

---

## Estrategia de pruebas y criterios de aceptación

- **Idempotencia:** correr `SyncHarness` dos veces sobre el mismo archivo → segunda
  corrida `changed == false`, sin diff. (Test directo, patrón del test de `badges`.)
- **Preserva lo propio:** un `AGENTS.md` con reglas específicas fuera de los
  marcadores conserva ese contenido intacto tras la sincronización.
- **Inserción limpia:** sobre un `AGENTS.md` sin marcadores, la primera corrida los
  añade tras el título; sobre uno inexistente, lo crea.
- **Una sola fuente:** el texto del arnés aparece exactamente una vez en el código
  fuente de devflow (`agents/HARNESS.md`); ningún otro archivo lo hardcodea.
- **Cero referencias locales en el resultado:** el `AGENTS.md` resultante no contiene
  ningún enlace a documentos fuera de su propio repositorio.
