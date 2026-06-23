# PKGBUILD: `CGO_ENABLED=0` contradice `depends=('glibc')`

**Archivo:** `packaging/arch/PKGBUILD:16`

El paquete se compila con `export CGO_ENABLED=0` (statically linked, sin glibc), pero en las dependencias se declara `depends=('glibc')`. Si no se usa CGO, no hay dependencia real de glibc, por lo que este campo sobra o el build no está aprovechando bibliotecas de C del sistema de manera justificada.
