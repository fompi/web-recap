# Sentinel errors declarados pero no usados

Los siguientes errores centinela están declarados pero nunca son retornados ni comprobados en el código:

| Error | Archivo |
|-------|---------|
| `ErrDatabaseLocked` | `internal/browser/errors.go:11` |
| `ErrDatabaseError` | `internal/database/errors.go:8` |
