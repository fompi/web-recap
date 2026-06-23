# `SKILL.md` desactualizado con respecto a CLI actual

El archivo `SKILL.md`, destinado a guiar a agentes de IA al trabajar con el CLI de web-recap, está ligeramente desfasado con la realidad del código fuente.

Específicamente:
- Sigue documentando `--tz` cuando el comando se actualizó a usar `--timezone`.
- Sigue documentando la ayuda antigua de `--compress` y no refleja la refactorización reciente que introduce `gzip (-z)`, `bzip2 (-zz)` y `xz (-zzz)`.

Mantener este archivo al día es importante para que agentes y usuarios no reciban información errónea sobre los flags.
