# Inconsistencia en `Browser.Name` vs `Browser.Type`

En `query.go:20` (`NewQuerier`), se pasa `string(b.Type)` como `browserName`:
```go
return NewChromeHandler(b.Path, string(b.Type), b.Profile), nil
```
Pero en `detector.go`, el `Name` se establece como "Google Chrome", "Brave", etc. (nombres legibles). Esto genera inconsistencia: los entries procedentes del detector normal llevan `Name="Google Chrome"` pero el querier usa `Type="chrome"`. En la práctica, el campo `Browser` en las entries siempre es el valor de `Type`, por lo que el campo `Name` del struct `Browser` nunca se aprovecha realmente en los resultados.
