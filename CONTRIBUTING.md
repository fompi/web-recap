# Contributing to web-recap

Thank you for your interest in contributing!

## Development

Requires Go 1.21+. To get started:

```bash
# Clone the repository
git clone https://github.com/robzolkos/web-recap.git
cd web-recap

# Run tests
go test -v ./...

# Build binary
make build
```

## Testing Requirements

This project aims to maintain near 100% test coverage across all packages. When submitting a pull request, ensure that:
1. Any new features or packages include comprehensive unit tests.
2. Any bug fixes include a regression test to prevent future occurrences.
3. You run `go test -cover ./...` locally and verify that test coverage remains optimal.

## Release Process

`CHANGELOG.md` and `VERSION` act as the single sources of truth for the project. 
To release a new version, **do not manually edit** `Makefile`, `main.go`, or packaging files. Instead:

1. Update `CHANGELOG.md` with the new version section under `## [Unreleased]`.
2. Run the automated bump script to synchronize all project files:
   ```bash
   go run scripts/bump.go <new_version>
   ```
3. Commit the changes.
