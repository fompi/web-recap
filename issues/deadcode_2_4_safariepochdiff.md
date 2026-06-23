# `safariEpochDiff` definido como `const` duplicada

En `internal/database/safari.go`, la constante `safariEpochDiff = 978307200` se declara **dos veces** (líneas 123 y 134) como constante local dentro del mismo bloque `if/else`. Debería ser una constante a nivel de función o paquete. También está definida en `util.go`.
