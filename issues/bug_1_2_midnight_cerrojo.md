# End-date midnight "cerrojo" implícito inconsistente

**Archivos:** `chrome.go:74-77`, `firefox.go:117-120`, `safari.go:130-133`

Los tres handlers contienen esta lógica:
```go
if endDate.Hour() == 0 && endDate.Minute() == 0 && endDate.Second() == 0 {
    endTimestamp += 86400
}
```
Esto añade implícitamente 24 horas si la hora final es medianoche. Sin embargo:
- No considera nanosegundos (una fecha con nanosegundos != 0 pero hora 00:00:00 se extiende incorrectamente).
- No está documentado en el man page ni en `--help`.
- Si el usuario pasa `--to "2026-06-20"` (= medianoche), el comportamiento real es `--to "2026-06-21 00:00:00"`, lo cual puede sorprender.
