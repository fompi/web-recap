# CI no incluye compilación de Linux ARM64

El archivo Makefile no genera un binario para `linux-arm64`, y el workflow de release de GitHub Actions tampoco lo sube como asset del release. 

Dado que el `PKGBUILD` de Arch explícitamente soporta `aarch64`, es incoherente y poco conveniente no distribuir el binario ARM64 precompilado junto al resto de arquitecturas en los releases de GitHub.
