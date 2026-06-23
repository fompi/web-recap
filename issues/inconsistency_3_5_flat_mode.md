# `--flat` flag y `mode=flat` string

**Archivo:** `cmd/web-recap/main.go:147-148` y `internal/database/ingest.go:39`

Hay un flag booleano `--flat` y un modo de ingesta (`--mode`). En `main.go`, el texto de `--mode` solo documenta `merged`, `split`, y `both`. Sin embargo, `ingest.go` sigue aceptando `mode="flat"` y lo convierte internamente a `mode="merged"` y `flat=true`. 

Son dos formas de activar lo mismo y no están cohesionadas. Si alguien pasa `-M flat --flat`, la redundancia no causa error pero evidencia una deuda técnica.
