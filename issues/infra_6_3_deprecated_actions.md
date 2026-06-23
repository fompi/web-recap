# GitHub Actions: acciones deprecadas

El archivo `.github/workflows/release.yml` utiliza acciones oficiales que han sido marcadas como obsoletas por GitHub:

| Acción | Versión usada | Estado |
|--------|--------------|--------|
| `actions/create-release` | `v1` | **Deprecada**. Debería sustituirse por `softprops/action-gh-release` u otras alternativas. |
| `actions/upload-release-asset` | `v1` | **Deprecada**. También reemplazada por la misma acción moderna. |

Adicionalmente, se recomienda revisar si `actions/setup-go@v4` y `actions/cache@v3` pueden actualizarse a las últimas versiones disponibles.
