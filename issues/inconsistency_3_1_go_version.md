# Versión de Go en CI workflows muy desactualizada

| Fuente | Versión Go |
|--------|-----------|
| `go.mod` | `go 1.25.5` |
| `test.yml` | `'1.21', '1.22'` |
| `release.yml` | `'1.22'` |

El `go.mod` requiere `go 1.25.5` pero los CI workflows testean con Go 1.21 y 1.22, que están por detrás. Es muy probable que el CI falle al compilar. Deberían actualizarse a `'1.25', '1.26'` como mínimo.
