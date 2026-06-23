# Función `copyTempFile` duplicada

Existen dos implementaciones para copiar un archivo:
- `internal/browser/detector.go:319-338`: `copyTempFile()` en paquete `browser`
- `internal/database/util.go:148-163`: `copyFile()` en paquete `database`

Ambas copian archivos, pero la versión en `database` es más sencilla y la de `browser` devuelve la ruta. Son funcionalmente equivalentes con mínimas diferencias y deberían consolidarse.
