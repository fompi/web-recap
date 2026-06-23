# Inconsistencia `--summary` / `--no-summary` flags

**Archivo:** `cmd/web-recap/main.go:135-136`

```go
sub.Flags().BoolVarP(&summary, "summary", "s", true, ...)
sub.Flags().BoolVarP(&noSummary, "no-summary", "S", false, ...)
```

Estos dos flags son mutuamente exclusivos, pero no se marcan como tales. Además, `--no-summary` es la negación de `--summary` — un solo flag con valor por defecto `true` bastaría. Pasar `--summary=false` equivale a `--no-summary`, creando confusión en la CLI.
