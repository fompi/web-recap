# `HasColumn` usa `SELECT *` — ineficiente

**Archivo:** `internal/database/util.go:15`

```go
rows, err := db.Query("SELECT * FROM " + tableName + " LIMIT 0")
```

La comprobación de la existencia de una columna se hace haciendo una query que lee de la tabla. Aunque usa `LIMIT 0` para no traer filas, es más idiomático y seguro usar pragmas específicos de la base de datos (por ejemplo, `PRAGMA table_info(tableName)` en SQLite o `information_schema.columns` en PostgreSQL/MySQL). Además, la concatenación `+ tableName +` podría ser riesgosa si `tableName` no estuviera validado.
