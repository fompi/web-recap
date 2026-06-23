# SQL injection en table names

**Archivo:** `internal/database/ingest.go:209-215`

```go
func getBrowserSpecificTableName(browser string) string {
    b := strings.ToLower(browser)
    b = strings.ReplaceAll(b, " ", "_")
    b = strings.ReplaceAll(b, "-", "_")
    return "history_" + b  // ← si b = "chrome;DROP TABLE history--" ?
}
```

La sanitización reemplaza espacios y guiones, pero **no** escapa ni elimina otros caracteres especiales. Dado que el nombre del browser puede provenir de la entrada del usuario en la CLI (`--browser`), es posible inyectar código SQL mediante la declaración de tablas.

**Fix:** Validar explícitamente mediante una regex que el nombre de navegador solo contenga caracteres válidos (como `^[a-z0-9_]+$`) o validar contra una whitelist estricta.
