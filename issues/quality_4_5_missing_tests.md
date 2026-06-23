# Sin tests para `output/` ni para `cmd/`

Los paquetes `internal/output` y `cmd/web-recap` no tienen ningún archivo de test. 

Funciones clave como los formatters de salida (Table, CSV, JSON, JSONLines) y la función `runQuery` del CLI no están testeadas, por lo que cambios en la visualización podrían romper de forma silenciosa el output esperado o la experiencia de la CLI.
