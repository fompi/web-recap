# `ingest.go` tiene demasiadas líneas (>1300)

**Archivo:** `internal/database/ingest.go`

El archivo tiene más de 1300 líneas y concentra **toda** la lógica de ingesta en un solo lugar: 
- Creación de tablas SQL (3 drivers × 3 modos)
- Query builders para INSERTs de los 4 tipos de navegadores
- Lógica de inserción bulk para SQLite/MySQL/Postgres
- Lógica de inserción para MongoDB

Debería refactorizarse y separarse en componentes más cohesivos (ej: `ingest_sql.go`, `ingest_mongo.go`, etc.).
