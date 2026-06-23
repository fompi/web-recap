# Doble ordenación redundante

**Archivo:** `internal/database/query.go:42-45`, `cmd/web-recap/main.go:268`

`Query()` ordena las entries por timestamp **descendente**. Después, `main.go:268` llama a `SortEntriesDescending()` otra vez sobre el slice completo. Esto es redundante — la segunda llamada es innecesaria si solo hay un browser, pero necesaria si hay múltiples browsers mezclados. Sin embargo, la **primera** sort (dentro de `Query`) es la redundante, ya que las queries SQL ya traen `ORDER BY ... DESC`.
