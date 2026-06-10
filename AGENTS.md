# Repository Guidelines

## Project Structure & Module Organization

Spiderpool is a Go Kubernetes networking project. Main binaries live in `cmd/`, reusable packages in `pkg/`, and Kubernetes APIs, generated clients, and OpenAPI specs in `api/`. Helm packaging is under `charts/spiderpool/`; container build assets are in `images/`. End-to-end assets and cluster scripts live in `test/`, documentation in `docs/`, design/spec work in `specs/`, and shared automation in `tools/` and `contrib/`. Avoid editing `vendor/` directly unless dependency vendoring is the explicit task.

## Build, Test, and Development Commands

- `make build-bin`: build Spiderpool binaries into the local output path.
- `make install-bin`: install built binaries.
- `make build_image`: build Docker images with buildx using the current commit version.
- `make build_docker_image`: local Docker fallback when buildx has pull issues.
- `make dev-doctor`: verify Go and required e2e tools such as Docker, kubectl, kind, and p2ctl.
- `make gofmt`: run `go fmt` on Go packages.
- `make lint-golang`: run format checks, lock checks, `go vet`, and `golangci-lint`.
- `make manifests generate-k8s-api`: regenerate CRDs/RBAC/webhooks and deepcopy code.
- `make openapi-code-gen`: regenerate OpenAPI clients from `api/v1/*/openapi.yaml`.
- `make chart-readme`: regenerate `charts/spiderpool/README.md` from `charts/spiderpool/values.yaml`.

## Coding Style & Naming Conventions

Use Go 1.25 as declared in `go.mod`. Keep Go code `gofmt`/`gofumpt` clean and satisfy `.golangci.yaml` linters: `govet`, `errcheck`, `staticcheck`, `ineffassign`, and `errorlint`. Package names are lowercase and directory-oriented, for example `pkg/ippoolmanager` and `pkg/workloadendpointmanager`. Tests use `_test.go`; suite files follow `*_suite_test.go`.

All code, comments, generated text reviewed before commit, and project documentation must be English-first. Use English for identifiers, comments, logs, errors, examples, and primary documentation text unless a file is explicitly localized.

## Testing Guidelines

Unit tests use Ginkgo v2 and Gomega. Run `make unittest-tests` for package and command tests; it also checks that non-suite test files include a Ginkgo `Label(...)`. For e2e work, build or pull images first, then use targets such as `make e2e_init_spiderpool` and `make e2e_test_spiderpool`. Narrow e2e runs with `E2E_GINKGO_LABELS=smoke` or `GINKGO_OPTION="--label-filter=CaseLabel"`.

## Commit & Pull Request Guidelines

History uses short imperative subjects with optional scopes, such as `fix: ...`, `test: ...`, `CI: ...`, `charts: ...`, and release bumps. Keep commits focused and sign them when following the contribution docs (`git commit -s`). PRs should link issues with `Fixes #...`, state unit or e2e coverage, mention docs impact, include reviewer notes when needed, and fill the release-note block with either content or `NONE`. Apply one release label: `release/none`, `release/bug`, or `release/feature`.

## Agent-Specific Instructions

Before changing generated Kubernetes or OpenAPI files, update the source definitions and run the matching generation or verify target. Do not revert unrelated local changes; this repository may contain concurrent contributor work.

When adding or modifying files under `docs/`, update both English and Chinese documentation in the same change. Keep English as the primary/source version, and keep the corresponding Chinese localized file or section synchronized with equivalent content.

When changing `charts/spiderpool/values.yaml`, run `make chart-readme` and include the generated `charts/spiderpool/README.md` changes in the same commit. Use `make chart-readme-verify` to check that the generated README is in sync.

<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan
<!-- SPECKIT END -->
