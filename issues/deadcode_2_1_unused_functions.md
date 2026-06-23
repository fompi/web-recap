# Funciones exportadas no utilizadas

| Función | Archivo | Análisis |
|---------|---------|----------|
| `NewDetector()` | `internal/browser/detector.go:21-23` | Solo se usa `NewDetectorForUser()`. No hay ningún call site. |
| `GetBrowser()` | `internal/browser/detector.go:186-207` | No se invoca en ninguna parte del codebase. |
| `GetFirefoxProfilePath()` | `internal/browser/paths.go:124-179` | Función exportada de 55 líneas sin ningún call site. Código muerto. |
