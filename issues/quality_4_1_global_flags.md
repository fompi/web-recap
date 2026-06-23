# Variables globales mutables (flags)

**Archivo:** `cmd/web-recap/main.go:21-38`

Todos los flags del programa (ej. `fromFlag`, `toFlag`, `timezone`, `summary`) están definidos como variables globales en el paquete `main`. 

Esto es un anti-patrón que dificulta escribir tests para la CLI e impide ejecutar múltiples comandos de forma aislada sin que el estado de una ejecución afecte a la otra. Cobra soporta `cmd.Flags().GetString()` y el paso de structs de configuración, que son mejores prácticas.
