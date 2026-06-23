# Error silenciado en cierre de gzip

**Archivo:** `cmd/web-recap/main.go:335-337`

```go
defer func() {
    if closer != nil {
        closer.Close()
    }
...
}()
```

El error devuelto por `closer.Close()` se ignora. En implementaciones de compresión como gzip, el `Close()` escribe el trailer o finaliza los buffers pendientes. Si esto falla (por ejemplo, disco lleno), los datos resultantes pueden quedar corruptos y la herramienta saldría sin notificar de dicho fallo.
