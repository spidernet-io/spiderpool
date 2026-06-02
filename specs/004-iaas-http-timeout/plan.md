# Implementation Plan: IaaS Provider HTTP Timeout

**Branch**: `004-iaas-http-timeout` | **Date**: 2026-06-02 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/004-iaas-http-timeout/spec.md`

## Summary

Spiderpool's IaaS provider HTTP client currently hardcodes a 30-second request
timeout in `pkg/iaas/client/client.go` (`NewClient`), and the configuration type
`IaaSProviderConfig` only exposes `serverUrl`. This feature adds a configurable
provider HTTP timeout (`iaasNetworkProvider.httpRequestTimeout`), surfaced
through the Helm value and the agent/controller ConfigMap, applied to every
provider allocation and release call. The timeout is validated at component startup: it MUST be positive and
strictly smaller than the static safety limit (2 minutes) and smaller than the
current CNI plugin-to-agent timeout (100 seconds, used by both CNI ADD and CNI
DEL in `cmd/spiderpool/cmd/command_add.go` and `command_delete.go`). At call
time the client also honors the parent `context.Context` deadline when one is
present, so the effective timeout is the smaller of the configured value and the
remaining context budget. The 100-second value is referenced from a shared
constant so that if the CNI client timeout later becomes configurable, the
validation compares against the configured value rather than a literal.

## Technical Context

**Language/Version**: Go 1.25 (per `go.mod`)

**Primary Dependencies**: standard library `net/http`, `context`, `time`;
`go.uber.org/zap` for logging; existing `pkg/iaas/client`, `pkg/types`,
`pkg/ipam`; Helm chart `charts/spiderpool`.

**Storage**: N/A (no persistent storage; configuration via ConfigMap/Helm values)

**Testing**: Ginkgo v2 + Gomega unit tests (`make unittest-tests`); Helm
rendering verification for chart changes.

**Target Platform**: Linux; Spiderpool Agent and Controller pods in Kubernetes.

**Project Type**: Kubernetes networking system (single Go repo: `cmd/`, `pkg/`,
`charts/`, `docs/`).

**Performance Goals**: No new hot-path cost. The change only sets an
`http.Client.Timeout` and performs O(1) startup validation. Provider calls
already exist on the IPAM ADD/DEL path; this feature bounds their latency rather
than adding work.

**Constraints**: Provider HTTP timeout MUST be `> 0`, `< 2m` (static safety
limit), and `< 100s` (current CNI plugin-to-agent timeout). At call time the
effective deadline is `min(configuredTimeout, remaining parent context budget)`.
Default value when IaaS integration is enabled but the timeout is unset: `30s`
(preserves current hardcoded behavior).

**Scale/Scope**: Small, focused change. Affected files: `pkg/types/k8s.go`,
`pkg/iaas/client/client.go`, agent/controller daemon wiring, a shared timeout
constant, `charts/spiderpool/values.yaml`, `charts/spiderpool/templates/configmap.yaml`,
`charts/spiderpool/README.md`, and docs (EN + ZH).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Code quality and API compatibility**: Adds an optional `httpRequestTimeout`
  field (`HTTPRequestTimeout`) to `IaaSProviderConfig` (`pkg/types/k8s.go`) and an
  optional Helm value `iaasNetworkProvider.httpRequestTimeout`. Both are additive and backward compatible: an
  empty/unset value preserves the existing 30s behavior. `NewClient` signature is
  extended to consume the configured timeout from the config struct (no new
  positional parameter), keeping the public function shape stable. The 100s CNI
  timeout literal in `command_add.go`/`command_delete.go` is replaced by a shared
  exported constant to remove duplication and enable future configurability.
- **Testing standard**: Unit tests (Ginkgo/Gomega) for: timeout parsing and
  validation (positive, `<2m`, `<100s`, zero/negative/unparsable, default), and
  for the client applying the configured timeout. Helm rendering test/verification
  that `iaasNetworkProvider.httpRequestTimeout` propagates into the ConfigMap. Generated test
  files include the required Ginkgo `Label(...)`.
- **User/operator consistency**: New Helm value documented in
  `charts/spiderpool/values.yaml` and `charts/spiderpool/README.md`; user docs
  updated in both English and Chinese. Validation errors name the configured
  value and the limit/budget it violated. Default and accepted duration format
  (Go duration string, e.g. `30s`, `1m30s`) documented.
- **Performance budget**: No performance-sensitive hot path is added. The change
  sets `http.Client.Timeout` once at client construction and performs O(1)
  validation at startup. Provider calls already run on the IPAM ADD/DEL path; this
  feature only bounds their duration.
- **Generated artifacts**: No CRD/OpenAPI/deepcopy/RBAC source definitions change,
  so `make manifests generate-k8s-api` / `make openapi-code-gen` are not required.
  Chart README parameter table is regenerated/updated to match `values.yaml`.

**Gate Result**: PASS — additive, backward-compatible config change with clear
validation and test coverage; no generated-artifact or performance-budget
violations.

## Project Structure

### Documentation (this feature)

```text
specs/004-iaas-http-timeout/
├── plan.md              # This file (/speckit.plan output)
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (config + validation contract)
└── tasks.md             # Phase 2 output (/speckit.tasks — NOT created here)
```

### Source Code (repository root)

```text
cmd/
├── spiderpool/cmd/command_add.go        # CNI ADD: 100s plugin-to-agent timeout (use shared const)
├── spiderpool/cmd/command_delete.go     # CNI DEL: 100s plugin-to-agent timeout (use shared const)
├── spiderpool-agent/cmd/daemon.go       # IaaS client creation + startup validation
└── spiderpool-controller/cmd/daemon.go  # IaaS client creation + startup validation

pkg/
├── types/k8s.go                         # IaaSProviderConfig: add HTTPRequestTimeout field
├── iaas/client/client.go                # NewClient: apply configured timeout; ValidateConfig: validate timeout
└── constant/ (or pkg/iaas/client)       # shared CNI plugin-to-agent timeout constant

charts/spiderpool/
├── values.yaml                          # iaasNetworkProvider.httpRequestTimeout
├── templates/configmap.yaml             # render timeout into ConfigMap
└── README.md                            # parameter table

docs/                                    # EN + ZH operator docs for the new value
```

**Structure Decision**: Single-repo Go layout. The feature reuses the existing
IaaS integration surfaces (`pkg/iaas/client`, `pkg/types`, the agent/controller
daemons, and the Helm chart) without introducing new packages, matching
Constitution Principle V and the repository layout in `AGENTS.md`.

## Complexity Tracking

> No Constitution Check violations. Section intentionally empty.
