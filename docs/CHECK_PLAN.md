# Plan: Mejorar el Comportamiento de Ayuda de `codejob`

## Problema

Actualmente, cuando los agentes LLM intentan ver la ayuda de `codejob` ejecutando variaciones como `codejob -help`, el argumento `-help` no es reconocido como una solicitud de ayuda. En su lugar, el sistema lo interpreta como el parámetro `message` (el mensaje de commit para cerrar un ciclo). Si existe una sesión activa (`CODEJOB_PR` en `.env`), esto provoca que se realice una publicación o merge accidental usando "-help" como mensaje de commit.

Además, si `codejob` se ejecuta sin argumentos en un entorno "vacío" (sin el archivo `docs/PLAN.md` ni variables de sesión en `.env`), el programa simplemente lanza un error ("prompt file not found") en lugar de mostrar de forma proactiva la ayuda de uso para guiar al usuario o agente.

## Solución Propuesta

1. **Ampliar la captura de argumentos de ayuda en `parseArgs` (`cmd/codejob/main.go`):**
   - Modificar la función `parseArgs` para que reconozca una lista exhaustiva de banderas de ayuda: `help`, `-help`, `--help`, `h`, `-h`, `?`, `-?`.
   - Con esto se evita que variaciones erróneas comunes se interpreten como un `message` de commit.

2. **Validar el "Entorno Activo" antes de ejecutar (`cmd/codejob/main.go`):**
   - Si no se proveen argumentos (`message == ""`), verificar si el entorno justifica la ejecución de `CodeJob`. Un entorno válido tiene al menos una de estas condiciones:
     - Existe un PR pendiente para hacer merge (`CODEJOB_PR` en `.env`).
     - Existe una sesión activa en progreso (`CODEJOB` en `.env`).
     - Existe un archivo de prompt listo para ser despachado (`docs/PLAN.md`).
   - Si ninguna de estas condiciones se cumple, interceptar la ejecución, mostrar la ayuda de uso (`showHelp()`) y terminar. Esto evita delegar la validación al `job.Run()` que solo devolvería un error técnico poco útil.

3. **Mejorar el mensaje de uso (`showHelp`):**
   - Actualizar el texto mostrado en `showHelp()` para listar las nuevas opciones soportadas.

## Tareas (Checklist)

- [ ] Modificar `parseArgs` en `cmd/codejob/main.go` añadiendo la captura de banderas extra.
- [ ] Crear la lógica de `isEnvironmentValid()` en `cmd/codejob/main.go` utilizando `devflow.NewDotEnv(".env")` y `devflow.DefaultIssuePromptPath`.
- [ ] Integrar esta validación en el bloque principal (`main()`) para disparar `showHelp()` cuando sea necesario.
- [ ] Actualizar la cadena impresa en `showHelp()` (`help, --help, -help, -h, h, ?, -?`).
