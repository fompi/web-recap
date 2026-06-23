# MongoDB `insertedCount` inflado con MatchedCount

**Archivo:** `internal/database/ingest.go:1129`

El código realiza:
```go
insertedCount += int(res.UpsertedCount) + int(res.MatchedCount)
```

`MatchedCount` incluye documentos que existían y **no** fueron modificados (si no cambian campos). Sumar `MatchedCount` infla la cuenta reportada al usuario, que podría creer que se han ingestado/actualizado nuevos documentos. Debería usar `ModifiedCount` si el objetivo es contar inserciones + actualizaciones reales.
