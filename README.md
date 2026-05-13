# Go Project Template

> Other references
>
> - https://github.com/thockin/go-build-template/tree/master
> - https://peter.bourgon.org/go-best-practices-2016/

## Getting Started

Follow below steps after creating a new repository from this template

- [ ] **Initialize Go module:**

  ```bash
  go mod init github.com/YOUR_USERNAME/YOUR_PROJECT_NAME
  go mod tidy
  ```

- [ ] **Update app name** in:

  - [ ] [Taskfile.yml](./Taskfile.yml) - `APP_NAME` variable
  - [ ] [Dockerfile](./Dockerfile) - Binary name and labels
  - [ ] [main.go](./main.go) - `appName` constant
  - [ ] [.goreleaser.yml](./.goreleaser.yml) - `project_name` and `binary` name
  - [ ] [README.md](./README.md) - Title and description

- [ ] **Update main file location** (if not using root `main.go`):

  - [ ] [Taskfile.yml](./Taskfile.yml) - `MAIN_FILE` variable
  - [ ] [.goreleaser.yml](./.goreleaser.yml) - `main` field under `builds`

- [ ] **Configure Homebrew release** (optional):

  > **Note:** GitHub's default `GITHUB_TOKEN` has limited permissions for tap repositories. See [GoReleaser docs](https://goreleaser.com/errors/resource-not-accessible-by-integration/).

  - [ ] Add `RELEASE_TOKEN` in repository secrets and update in [release workflow](./.github/workflows/release.yml)
  - [ ] Update [release workflow](./.github/workflows/release.yml) to use the new token
  - [ ] Update [.goreleaser.yml](./.goreleaser.yml) `brews` section with your tap repository details

- [ ] **Clean up:** Delete this checklist and update README with project documentation

## Build & Test

- Using [Taskfile](https://taskfile.dev/)

_Install Taskfile: [Installation Guide](https://taskfile.dev/docs/installation)_

```bash
# Available tasks
task --list
task: Available tasks for this project:
* all:                   Run comprehensive checks: format, lint, security and test
* build:                 Build the application binary for the current platform
* build-docker:          Build Docker image
* build-platforms:       Build the application binaries for multiple platforms and architectures
* fmt:                   Formats all Go source files
* install:               Install required tools and dependencies
* lint:                  Run static analysis and code linting using golangci-lint
* run:                   Runs the main application
* security:              Run security vulnerability scan
* test:                  Runs all tests in the project      (aliases: tests)
* vet:                   Examines Go source code and reports suspicious constructs
```

- Build with [goreleaser](https://goreleaser.com/)

_Install GoReleaser: [Installation Guide](https://goreleaser.com/install/)_

```bash
# Build locally
goreleaser release --snapshot --clean
...
```
