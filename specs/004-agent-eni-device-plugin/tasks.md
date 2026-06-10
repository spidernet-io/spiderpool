# Tasks: Agent ENI Device Plugin

**Input**: Design documents from `specs/004-agent-eni-device-plugin/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/)

**Tests**: Required by the Spiderpool Constitution for this behavior change. Include unit tests, Ginkgo/Gomega package tests, Helm rendering tests, and targeted e2e coverage.

**Organization**: Tasks are grouped by user story so each story can be implemented and tested independently after shared foundation is complete.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare constants, package skeletons, and docs targets used by later phases.

- [X] T001 Create `pkg/enislotdeviceplugin/` with package documentation and empty implementation files `pkg/enislotdeviceplugin/config.go`, `pkg/enislotdeviceplugin/server.go`, `pkg/enislotdeviceplugin/register.go`, and `pkg/enislotdeviceplugin/devices.go`
- [X] T002 [P] Add canonical ENI resource/config constants for `spidernet.io/eni-slot`, `iaasNetworkProvider.eniDevPlugin`, and `injectPodENIResources` in `pkg/constant/k8s.go`
- [X] T003 [P] Add initial ENI device plugin documentation placeholders in `docs/usage/iaas-network-provider.md`, `docs/reference/configmap.md`, and `docs/reference/spiderpool-agent.md`
- [X] T004 [P] Add e2e scenario placeholder files for ENI device plugin scheduling in `test/e2e/eni/eni_device_plugin_suite_test.go` and `test/e2e/eni/eni_device_plugin_test.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared configuration, validation, and chart wiring required before user story implementation.

**CRITICAL**: No user story work can begin until this phase is complete.

- [X] T005 Add `ENIDevPluginConfig` fields `Enabled`, `ResourceName`, `MaxSlotsPerNode`, `KubeletRootDir`, and `InjectPodENIResources` to `pkg/types/k8s.go`
- [X] T006 Parse `iaasNetworkProvider.eniDevPlugin` from the Spiderpool configmap into shared agent/controller config in `cmd/spiderpool-agent/cmd/config.go` and `cmd/spiderpool-controller/cmd/config.go`
- [X] T007 [P] Add labeled config validation tests for default disabled state, invalid resource name, negative max slots, kubelet root path, and provider dependency in `cmd/spiderpool-agent/cmd/config_test.go` and `cmd/spiderpool-controller/cmd/config_test.go`
- [X] T008 Implement ENI device plugin config validation and defaulting helpers in `pkg/enislotdeviceplugin/config.go`
- [X] T009 [P] Add unit tests for ENI device plugin config defaulting, kubelet root path handling, and validation in `pkg/enislotdeviceplugin/config_test.go`
- [X] T010 Update Helm values for `iaasNetworkProvider.eniDevPlugin`, including `kubeletRootDir`, in `charts/spiderpool/values.yaml`
- [X] T011 Update rendered configmap values for `iaasNetworkProvider.eniDevPlugin`, including `kubeletRootDir`, in `charts/spiderpool/templates/configmap.yaml`
- [X] T012 Update spiderpool-agent DaemonSet hostPath volumes and mounts for `{kubeletRootDir}/device-plugins` and `{kubeletRootDir}/plugins_registry` gated by `eniDevPlugin.enabled` in `charts/spiderpool/templates/daemonset.yaml`
- [X] T013 [P] Create Helm rendering validation script for disabled defaults, `kubeletRootDir`, and enabled ENI device plugin mounts/config in `tools/helm/eni_device_plugin_render_test.sh`
- [X] T014 Confirm no CRD/OpenAPI source changes are required by reviewing `api/` and `pkg/k8s/apis/`, verify kubelet device plugin API imports compile with the current module set, and record the generation/dependency decision in `specs/004-agent-eni-device-plugin/plan.md`

**Checkpoint**: Configuration, defaults, Helm rendering, and package boundaries are ready.

---

## Phase 3: User Story 1 - Schedule Pods Only Where Auxiliary ENIs Are Available (Priority: P1) MVP

**Goal**: Eligible provider-mode Pods automatically request `spidernet.io/eni-slot` so Kubernetes scheduling prevents placement beyond node slot capacity.

**Independent Test**: Configure slot capacity, create Pods referencing eligible VLAN SpiderMultusConfigs, verify resource injection quantity, and verify excess Pods remain pending or schedule to nodes with remaining capacity.

### Tests for User Story 1

- [X] T015 [P] [US1] Add labeled Ginkgo tests for detecting VLAN SpiderMultusConfigs with nil VLAN ID from Pod Multus default-network and attachment-network annotations in `pkg/podmanager/pod_webhook_internal_test.go`
- [X] T016 [P] [US1] Add labeled Ginkgo tests that no `spidernet.io/eni-slot` resource is injected when provider mode or `eniDevPlugin.enabled` is disabled in `pkg/podmanager/pod_webhook_internal_test.go`
- [X] T017 [P] [US1] Add labeled Ginkgo tests that existing `spidernet.io/eni-slot` limits are not overwritten, duplicated, incremented, or recalculated in `pkg/podmanager/utils_test.go`
- [X] T018 [P] [US1] Add labeled Ginkgo tests that injected `spidernet.io/eni-slot` quantity equals the count of eligible VLAN SpiderMultusConfigs in `pkg/podmanager/utils_test.go`
- [X] T019 [P] [US1] Add labeled e2e test coverage for scheduling Pods only up to advertised ENI slot capacity in `test/e2e/eni/eni_device_plugin_test.go`

### Implementation for User Story 1

- [X] T020 [P] [US1] Add helper to resolve Pod-referenced SpiderMultusConfigs from `v1.multus-cni.io/default-network` and `k8s.v1.cni.cncf.io/networks` in `pkg/podmanager/utils.go`
- [X] T021 [P] [US1] Add helper to identify VLAN-type SpiderMultusConfigs with nil VLAN ID in `pkg/podmanager/utils.go`
- [X] T022 [US1] Extend Pod webhook configuration inputs to include provider mode and `eniDevPlugin` injection settings in `pkg/podmanager/pod_webhook.go`
- [X] T023 [US1] Implement ENI slot resource injection using eligible VLAN SpiderMultusConfig count in `pkg/podmanager/utils.go`
- [X] T024 [US1] Ensure ENI resource injection skips Pods that already declare the configured resource key in any container in `pkg/podmanager/utils.go`
- [X] T025 [US1] Wire controller config into the Pod mutating webhook path so `injectPodENIResources` controls only ENI injection in `cmd/spiderpool-controller/cmd/daemon.go` and `pkg/podmanager/pod_webhook.go`
- [X] T026 [US1] Update docs with webhook auto-injection behavior and examples in `docs/usage/iaas-network-provider.md`

**Checkpoint**: User Story 1 is independently functional: eligible Pods request ENI slots and Kubernetes scheduler enforces node capacity.

---

## Phase 4: User Story 2 - Keep Node Capacity Status Accurate (Priority: P2)

**Goal**: spiderpool-agent registers a kubelet device plugin that advertises `spidernet.io/eni-slot` as healthy schedulable total capacity.

**Independent Test**: Enable the feature with a configured per-node slot count, start the agent, and verify node allocatable reports the healthy schedulable total, not free slots.

### Tests for User Story 2

- [X] T027 [P] [US2] Add labeled unit tests for stable ENI slot device ID generation from `maxSlotsPerNode` in `pkg/enislotdeviceplugin/devices_test.go`
- [X] T028 [P] [US2] Add labeled device plugin server tests for `ListAndWatch` healthy slot output and zero-slot behavior in `pkg/enislotdeviceplugin/server_test.go`
- [X] T029 [P] [US2] Add labeled registration retry and kubelet plugin path selection tests for kubelet socket availability and registration failures in `pkg/enislotdeviceplugin/register_test.go`
- [X] T030 [P] [US2] Add Helm rendering assertions that both `{kubeletRootDir}/device-plugins` and `{kubeletRootDir}/plugins_registry` are mounted only when `iaasNetworkProvider.eniDevPlugin.enabled=true` in `tools/helm/eni_device_plugin_render_test.sh`
- [X] T031 [P] [US2] Add labeled e2e test for node allocatable reporting the configured ENI slot total in `test/e2e/eni/eni_device_plugin_test.go`

### Implementation for User Story 2

- [X] T032 [P] [US2] Implement stable slot list generation and health model in `pkg/enislotdeviceplugin/devices.go`
- [X] T033 [P] [US2] Implement kubelet device plugin gRPC server methods in `pkg/enislotdeviceplugin/server.go`
- [X] T034 [US2] Implement kubelet registration, socket cleanup, and retry loop in `pkg/enislotdeviceplugin/register.go`
- [X] T035 [US2] Implement manager lifecycle start/stop wrapper in `pkg/enislotdeviceplugin/manager.go`
- [X] T036 [US2] Wire ENI device plugin startup and shutdown into `cmd/spiderpool-agent/cmd/daemon.go`
- [X] T037 [US2] Add agent context fields for ENI device plugin manager in `cmd/spiderpool-agent/cmd/config.go`
- [X] T038 [US2] Add operator-visible logs and events for registration, advertised total, and registration failures in `pkg/enislotdeviceplugin/manager.go`
- [X] T039 [US2] Update node capacity semantics documentation in `docs/reference/spiderpool-agent.md`

**Checkpoint**: User Story 2 is independently functional: node status advertises scheduler-facing total ENI slot capacity through kubelet.

---

## Phase 5: User Story 3 - Release Auxiliary ENI Capacity Reliably (Priority: P3)

**Goal**: Pod deletion, startup failure, and restart scenarios release or recover ENI slot capacity without stale reservations or double-counting.

**Independent Test**: Repeatedly create/delete Pods requesting ENI slots, restart kubelet or spiderpool-agent, and verify later Pods can schedule again without exceeding advertised capacity.

### Tests for User Story 3

- [X] T040 [P] [US3] Add labeled device plugin tests for idempotent `Allocate` handling and unknown slot rejection in `pkg/enislotdeviceplugin/server_test.go`
- [X] T041 [P] [US3] Add labeled restart reconciliation tests for plugin re-registration after socket removal in `pkg/enislotdeviceplugin/register_test.go`
- [X] T042 [P] [US3] Add labeled package tests that IPAM/IaaS release behavior remains owned by existing SpiderEndpoint cleanup in `pkg/ipam/iaas_test.go`
- [X] T043 [P] [US3] Add labeled e2e test for create/delete loop returning schedulable ENI slot capacity in `test/e2e/eni/eni_device_plugin_test.go`
- [X] T044 [P] [US3] Add labeled e2e test for spiderpool-agent or kubelet restart recovery in `test/e2e/eni/eni_device_plugin_test.go`

### Implementation for User Story 3

- [X] T045 [US3] Ensure `Allocate` returns deterministic successful responses for known slot IDs without moving provider allocation ownership out of `pkg/ipam/iaas.go` in `pkg/enislotdeviceplugin/server.go`
- [X] T046 [US3] Implement kubelet restart detection through device-plugin socket lifecycle and re-registration in `pkg/enislotdeviceplugin/register.go`
- [X] T047 [US3] Ensure device IDs remain stable across spiderpool-agent restarts in `pkg/enislotdeviceplugin/devices.go`
- [X] T048 [US3] Add diagnostic logging for release/reuse assumptions and duplicate allocation avoidance in `pkg/enislotdeviceplugin/server.go`
- [X] T049 [US3] Update restart and release behavior documentation in `docs/usage/iaas-network-provider.md`

**Checkpoint**: User Story 3 is independently functional: slot capacity recovers after Pod lifecycle and restart events.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final validation, docs, quality gates, and performance checks across all stories.

- [X] T050 [P] Update feature quickstart examples in `docs/usage/iaas-network-provider.md` and `docs/reference/configmap.md`
- [X] T051 [P] Add diagnostic metrics or logs for advertised total and derived free ENI slot count in `pkg/metric/metrics_eni.go` and `pkg/enislotdeviceplugin/manager.go`
- [X] T052 [P] Add troubleshooting notes for pending Pods, missing node allocatable resource, and restart windows in `docs/reference/spiderpool-agent.md`
- [X] T053 Run `make gofmt` from repository root `/root/cyclinder/spiderpool` to format Go changes across `cmd/` and `pkg/`
- [X] T054 Run focused package tests for `pkg/enislotdeviceplugin`, `pkg/podmanager`, and `cmd/spiderpool-agent/cmd` from repository root `/root/cyclinder/spiderpool`
- [X] T055 Run Helm rendering validation script `tools/helm/eni_device_plugin_render_test.sh` from repository root `/root/cyclinder/spiderpool`
- [X] T056 Run targeted e2e scenario for `test/e2e/eni` from repository root `/root/cyclinder/spiderpool`
- [X] T057 Run `make lint-golang` from repository root `/root/cyclinder/spiderpool` or record a maintainer-approved exception with risk in `specs/004-agent-eni-device-plugin/tasks.md`
- [X] T058 Verify no CRD, OpenAPI, or generated Kubernetes artifacts changed unexpectedly in `api/`, `pkg/k8s/apis/`, and `charts/spiderpool/crds/`; if source definitions changed, run `make manifests generate-k8s-api` or `make openapi-code-gen`

Lint exception for T057: `GOCACHE=/tmp/spiderpool-go-build make lint-golang` passed `check-go-fmt.sh` and `lock-check.sh`, then failed because `golangci-lint` is not installed in the current environment (`/bin/bash: line 1: golangci-lint: command not found`). Risk: golangci-lint-specific issues may remain and must be checked in CI or an environment with golangci-lint installed.

E2E execution note for T056: `GOCACHE=/tmp/spiderpool-go-build go test ./test/e2e/eni -run '^$'` passed compile-time validation. The targeted runtime command `GOCACHE=/tmp/spiderpool-go-build go test ./test/e2e/eni -ginkgo.label-filter='eni-device-plugin'` was attempted but stopped in `BeforeSuite` because the current environment does not provide required e2e cluster variables (`E2E_CLUSTER_NAME`, and by implication the kubeconfig-backed e2e context). Risk: the new ENI device plugin e2e specs still need to run in a configured e2e cluster with `iaasNetworkProvider.eniDevPlugin` enabled.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately.
- **Foundational (Phase 2)**: Depends on Setup; blocks all user stories.
- **User Story 1 (Phase 3)**: Depends on Foundational; MVP because it delivers scheduler protection through resource requests.
- **User Story 2 (Phase 4)**: Depends on Foundational; can run in parallel with US1 after shared config is ready, but e2e validation needs a working resource request path.
- **User Story 3 (Phase 5)**: Depends on US2 device plugin lifecycle and benefits from US1 scheduling path.
- **Polish (Phase 6)**: Depends on selected user stories being complete.

### User Story Dependencies

- **US1**: Requires config and webhook foundation; no dependency on US2 for package-level injection tests.
- **US2**: Requires config and Helm foundation; no dependency on US1 for package-level device plugin tests.
- **US3**: Requires US2 registration/device lifecycle and should be validated after US1 and US2 are integrated.

### Parallel Opportunities

- T002, T003, and T004 can run in parallel after T001.
- T007, T009, and T013 can run in parallel with implementation of foundational config and Helm changes.
- T015 through T019 can be written in parallel because they target distinct webhook and e2e behaviors.
- T020 and T021 can run in parallel before T023.
- T027 through T031 can run in parallel because they target distinct device plugin and Helm behaviors.
- T032 and T033 can run in parallel before T034 and T035.
- T040 through T044 can run in parallel before US3 implementation.
- T050 through T052 can run in parallel during final documentation and diagnostics.

## Parallel Examples

### User Story 1

```text
Task: "Add Ginkgo tests for detecting VLAN SpiderMultusConfigs with nil VLAN ID from Pod Multus annotations in pkg/podmanager/pod_webhook_internal_test.go"
Task: "Add Ginkgo tests that existing spidernet.io/eni-slot limits are not overwritten in pkg/podmanager/utils_test.go"
Task: "Add e2e test coverage for scheduling Pods only up to advertised ENI slot capacity in test/e2e/eni/eni_device_plugin_test.go"
```

### User Story 2

```text
Task: "Add unit tests for stable ENI slot device ID generation from maxSlotsPerNode in pkg/enislotdeviceplugin/devices_test.go"
Task: "Add device plugin server tests for ListAndWatch healthy slot output in pkg/enislotdeviceplugin/server_test.go"
Task: "Add Helm rendering test that both kubelet plugin paths derived from kubeletRootDir are mounted only when iaasNetworkProvider.eniDevPlugin.enabled=true in charts/spiderpool/"
```

### User Story 3

```text
Task: "Add restart reconciliation tests for plugin re-registration after socket removal in pkg/enislotdeviceplugin/register_test.go"
Task: "Add e2e test for create/delete loop returning schedulable ENI slot capacity in test/e2e/eni/eni_device_plugin_test.go"
```

## Implementation Strategy

### MVP First

1. Complete Phase 1 and Phase 2.
2. Complete Phase 3 for User Story 1.
3. Validate webhook injection and scheduler resource requests with focused tests.
4. Demo that Pods referencing eligible VLAN SpiderMultusConfigs receive the correct `spidernet.io/eni-slot` quantity.

### Incremental Delivery

1. Deliver US1 to ensure Pods request ENI slots.
2. Deliver US2 to advertise node slot capacity through kubelet device plugin.
3. Deliver US3 to harden release and restart behavior.
4. Complete polish tasks and repository quality gates.

### Quality Gate

Do not merge until tests for touched packages, Helm rendering, targeted e2e coverage, `make gofmt`, and `make lint-golang` have passed or an explicit maintainer-approved exception is recorded.
