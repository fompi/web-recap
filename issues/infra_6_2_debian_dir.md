# Packaging dual: `debian/` en raíz, otros en `packaging/`

La carpeta `debian/` está en la raíz del proyecto, lo cual es un requisito técnico directo de herramientas como `dpkg-buildpackage`. Sin embargo, los paquetes de Arch y Fedora están correctamente aislados bajo el directorio `packaging/`.

Dado que esta estructura híbrida está "correcta convencionalmente" por limitaciones técnicas de dpkg, al menos debería documentarse explícitamente en el `README.md` o en la documentación interna para evitar confusión arquitectónica.
