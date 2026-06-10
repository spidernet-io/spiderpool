# Tasks: IaaS Provider HTTP Timeout

**Feature Branch**: `004-iaas-http-timeout`
**Plan**: [plan.md](./plan.md) | **Spec**: [spec.md](./spec.md)
**Inputs**: research.md, data-model.md, contracts/config-contract.md, quickstart.md

## Conventions

- Config field: Helm/YAML `iaasNetworkProvider.httpRequestTimeout`, Go
  `IaaSProviderConfig.HTTPRequestTimeout` (Go duration string).
- Shared constants live in `pkg/constant`: `DefaultCNIClientTimeout = 100s`,
  `IaaSTimeoutStaticLimit = 2m`, `DefaultIaaSProviderTimeout = 30s`.
- Tests use Ginkgo v2 + Gomega; non-suite `*_test.go` files MUST include a
  Ginkgo `Label(...)`.

---

## Phase 1: Setup

- [ ] T001 Verify the IaaS integration surfaces compile and tests pass before changes by running `go build ./... && go test ./pkg/iaas/... ./pkg/types/...` from the repo root

---

## Phase 2: Foundational (blocking prerequisites for all user stories)

**Purpose**: Shared types and constants every user story depends on. No user
story can be completed until these exist.

- [ ] T002 [P] Add shared timeout constants `DefaultCNIClientTimeout = 100 * time.Second`, `IaaSTimeoutStaticLimit = 2 * time.Minute`, and `DefaultIaaSProviderTimeout = 30 * time.Second` in `pkg/constant/` (e.g. a new `pkg/constant/timeout.go`), with English doc comments
- [ ] T003 [P] Add the `HTTPRequestTimeout string` field with tag `yaml:"httpRequestTimeout,omitempty"` to `IaaSProviderConfig` in `pkg/types/k8s.go`
- [ ] T004 Replace the literal `100*time.Second` with `constant.DefaultCNIClientTimeout` in `cmd/spiderpool/cmd/command_add.go` (CNI ADD plugin-to-agent timeout)
- [ ] T005 Replace the literal `100*time.Second` with `constant.DefaultCNIClientTimeout` in `cmd/spiderpool/cmd/command_delete.go` (CNI DEL plugin-to-agent timeout)

**Checkpoint**: Config type and constants exist; CNI ADD/DEL reference the shared constant. Project builds.

---

## Phase 3: User Story 1 - Configure Provider Timeout (Priority: P1)

**Goal**: Operators can set `iaasNetworkProvider.httpRequestTimeout` via Helm and
Spiderpool applies it to every provider allocate/release call (replacing the
hardcoded 30s).

**Independent Test**: Render the chart with a valid `httpRequestTimeout`, confirm
it appears in the ConfigMap, and confirm the IaaS HTTP client is constructed with
that timeout (unit test on `NewClient`).

### Tests for User Story 1

> Write/update these tests before the implementation is considered complete.

- [ ] T006 [P] [US1] Ginkgo/Gomega test that `NewClient` builds an `http.Client` whose `Timeout` equals the configured `HTTPRequestTimeout`, and defaults to `30s` when empty, in `pkg/iaas/client/client_test.go` (include `Label(...)`)
- [ ] T007 [P] [US1] Helm rendering test/verification that `--set iaasNetworkProvider.httpRequestTimeout=45s` renders `httpRequestTimeout: "45s"` into the agent/controller ConfigMap (add to existing chart test harness or `charts/spiderpool` rendering tests)

### Implementation for User Story 1

- [ ] T008 [P] [US1] Add `httpRequestTimeout: "30s"` with `@param` documentation under `iaasNetworkProvider` in `charts/spiderpool/values.yaml`
- [ ] T009 [P] [US1] Render `httpRequestTimeout: {{ (.Values.iaasNetworkProvider).httpRequestTimeout | default "30s" | quote }}` under `iaasNetworkProvider` in `charts/spiderpool/templates/configmap.yaml`
- [ ] T010 [US1] In `pkg/iaas/client/client.go` `NewClient`, parse `cfg.HTTPRequestTimeout` (defaulting empty to `constant.DefaultIaaSProviderTimeout`) and set it as `http.Client.Timeout`, removing the hardcoded `30 * time.Second` (depends on T002, T003)
- [ ] T011 [US1] Update the `iaasNetworkProvider` parameter table in `charts/spiderpool/README.md` to include `iaasNetworkProvider.httpRequestTimeout`

**Checkpoint**: Configured timeout flows Helm → ConfigMap → client and bounds every provider call.

---

## Phase 4: User Story 2 - Prevent Unsafe Timeout Values (Priority: P1)

**Goal**: Spiderpool rejects provider timeout values that are not safely below the
parent budget: at startup the value MUST be `> 0`, `< 2m`, and `< 100s` (the
current CNI plugin-to-agent timeout shared by ADD and DEL).

**Independent Test**: Run `ValidateConfig` with valid and unsafe values and confirm
acceptance/rejection; start the agent/controller with an unsafe value and confirm
startup fails with a clear error.

### Tests for User Story 2

- [ ] T012 [P] [US2] Ginkgo/Gomega table test for `ValidateConfig` covering empty (default ok), `30s` (ok), `0s`/negative (error), unparsable (error), `>= 2m` (error), `>= 100s` (error), and `ServerURL == ""` (always nil) in `pkg/iaas/client/client_test.go` (include `Label(...)`)
- [ ] T013 [P] [US2] Test confirming both CNI ADD and CNI DEL share `constant.DefaultCNIClientTimeout` (value-equality assertion) so the validation budget matches both flows (SC-003), in `cmd/spiderpool/cmd/command_test.go` or `pkg/constant` test

### Implementation for User Story 2

- [ ] T014 [US2] Extend `ValidateConfig` in `pkg/iaas/client/client.go` to validate `HTTPRequestTimeout` when `ServerURL != ""`: parse, `> 0`, `< constant.IaaSTimeoutStaticLimit`, `< constant.DefaultCNIClientTimeout`; return errors that name the configured value and the violated limit (depends on T002, T003)
- [ ] T015 [US2] In `cmd/spiderpool-agent/cmd/daemon.go`, make IaaS config validation failure fatal (currently a warning at lines ~83-85) when `ServerURL != ""`, so an unsafe timeout aborts startup
- [ ] T016 [US2] In `cmd/spiderpool-controller/cmd/daemon.go`, make IaaS config validation failure fatal (currently a warning at lines ~86-88) when `ServerURL != ""`, mirroring the agent

**Checkpoint**: Unsafe values are rejected at startup; ADD/DEL share one validated budget.

---

## Phase 5: User Story 3 - Diagnose Timeout Decisions (Priority: P2)

**Goal**: Operators get clear, distinct messages for provider timeout, parent-budget
rejection, invalid configuration, and provider errors.

**Independent Test**: Trigger each case (slow provider, exhausted context budget,
invalid config) and confirm the messages distinguish the outcomes and include the
configured value and the relevant budget.

### Tests for User Story 3

- [ ] T017 [P] [US3] Ginkgo/Gomega test that the client returns a distinct "parent budget exhausted" error (without issuing the request) when `ctx.Deadline()` is already passed, in `pkg/iaas/client/client_test.go` (include `Label(...)`)
- [ ] T018 [P] [US3] Test that a provider-timeout failure produces a message identifying a provider-interaction timeout (distinct from validation/budget errors), in `pkg/iaas/client/client_test.go`

### Implementation for User Story 3

- [ ] T019 [US3] In `pkg/iaas/client/client.go` `AllocateIPs` and `releaseSingleIP`, before `http.NewRequestWithContext`, check `ctx.Deadline()`; if `time.Until(deadline) <= 0` return a clear "parent budget exhausted" error without calling the provider (effective bound becomes `min(httpRequestTimeout, remaining budget)`)
- [ ] T020 [US3] Ensure timeout/transport errors from `httpClient.Do` are wrapped with an operator-facing message that identifies a provider-interaction timeout, and that validation errors (US2) name the configured value and limit, keeping the four outcomes distinct (FR-012, SC-005)

**Checkpoint**: All three user stories independently functional and diagnosable.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T021 [P] Add/update English operator docs for `iaasNetworkProvider.httpRequestTimeout` (name, default `30s`, duration format, valid/invalid examples, the `< parent budget` requirement) under `docs/`
- [ ] T022 [P] Add/update the synchronized Chinese localized docs for the same content under `docs/` (per AGENTS.md EN+ZH requirement)
- [ ] T023 Run `make gofmt` and `make lint-golang` and fix any findings
- [ ] T024 Run `make unittest-tests` and ensure all new/updated tests pass with required Ginkgo labels
- [ ] T025 Run the `quickstart.md` Helm-render and negative-validation checks to confirm operator-facing behavior

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies.
- **Foundational (Phase 2)**: Depends on Setup. BLOCKS all user stories (T002/T003 provide the constant + field used everywhere).
- **User Story 1 (Phase 3)**: Depends on Foundational.
- **User Story 2 (Phase 4)**: Depends on Foundational. T014 reuses `ValidateConfig`; independent of US1 but typically lands together.
- **User Story 3 (Phase 5)**: Depends on Foundational. Builds on the client call sites; independently testable.
- **Polish (Phase 6)**: Depends on all targeted user stories.

### Key task dependencies

- T002, T003 → T004, T005, T010, T014 (constant + field consumed)
- T010 (NewClient applies timeout) → independent of T014 but both touch `client.go`
- T014 (ValidateConfig) → T015, T016 (daemons rely on validation being authoritative)
- T019, T020 share `client.go` with T010/T014; sequence edits to that file to avoid conflicts.

### Parallel Opportunities

- T002 and T003 can run in parallel (different files).
- US1: T006, T007 (tests) parallel; T008, T009 (chart) parallel.
- US2: T012, T013 (tests) parallel.
- US3: T017, T018 (tests) parallel.
- Polish: T021, T022 (EN/ZH docs) parallel.
- Note: edits within `pkg/iaas/client/client.go` (T010, T014, T019, T020) touch the same file — do NOT mark those [P]; sequence them.

---

## Parallel Example: User Story 1

```bash
# Tests (different files) in parallel:
Task: "NewClient timeout unit test in pkg/iaas/client/client_test.go"
Task: "Helm rendering test for httpRequestTimeout in charts/spiderpool tests"

# Chart edits (different files) in parallel:
Task: "Add httpRequestTimeout to charts/spiderpool/values.yaml"
Task: "Render httpRequestTimeout in charts/spiderpool/templates/configmap.yaml"
```

---

## Implementation Strategy

### MVP First (User Story 1 + 2)

US1 and US2 are both P1 and together form the safety-critical MVP: configurable
timeout (US1) is only safe with validation (US2).

1. Phase 1 Setup → Phase 2 Foundational.
2. Phase 3 (US1) → validate config flows to the client.
3. Phase 4 (US2) → validate unsafe values are rejected at startup.
4. STOP and VALIDATE: render chart + start components with valid/invalid values.

### Incremental Delivery

1. Foundational ready (constant + field, CNI uses constant).
2. US1 → configurable timeout applied. Demo Helm render + client behavior.
3. US2 → startup validation. Demo rejection of unsafe values.
4. US3 → diagnostics. Demo distinct error messages.
5. Polish → EN/ZH docs, lint, tests, quickstart verification.

---

## Notes

- [P] = different files, no dependency on incomplete tasks.
- Tests precede or accompany implementation per the Constitution; verify they fail first where applicable.
- Backward compatible: empty/unset `httpRequestTimeout` defaults to `30s`; empty `serverUrl` disables IaaS and skips validation.
- No CRD/OpenAPI/deepcopy generation required; only the chart README table is regenerated/updated.
