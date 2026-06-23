# El flag `--user` está duplicado

**Archivo:** `cmd/web-recap/main.go:133` y `main.go:152`

El flag `--user` (o `-u`) se registra iterando por los subcomandos (`dump`, `stats`, `ingest`) y luego se registra de forma independiente en `listCmd`. 

Ambos mapean a la misma variable global `userFlag`, lo cual funciona, pero causa una definición redundante en el código. Debería definirse en un único lugar en `rootCmd.PersistentFlags()`.
