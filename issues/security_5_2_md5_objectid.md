# MD5 para deterministic ObjectIDs en MongoDB

**Archivo:** `internal/database/ingest.go:1262-1273`

Se usa la función criptográfica obsoleta MD5 para generar IDs determinísticos en MongoDB. MD5 tiene problemas conocidos de colisiones. Aunque el riesgo en un contexto de historial de navegador personal es bajo, las mejores prácticas sugieren evitar su uso por completo.

Se recomienda sustituirlo por SHA-256 (incluso si hay que truncarlo a 12 bytes para coincidir con el tamaño de un ObjectID de MongoDB).
