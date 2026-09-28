# AGENTS.md

## Lints

Lints must run before every commit. From the repo root:

```bash
task all   # format, lint, security, and test
```

Or individually:

```bash
task fmt    # gofmt
task lint   # golangci-lint
task vet    # go vet
task test   # go test ./...
```

A commit that fails any of these is not done. Fix lint issues in the same commit as the change that caused them.
