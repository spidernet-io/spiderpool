# Tasks: Agent Network Resource Plugin

**Input**: Design documents from `specs/005-agent-eni-device-plugin/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Required. The specification and plan require unit tests, Ginkgo/Gomega package tests, Helm rendering checks, path-selection tests, local Node reconcile tests, webhook tests, and targeted e2e coverage for scheduling, dynamic configuration, and restart behavior.

**Organization**: Tasks are grouped by user story so each story can be implemented and validated independently after the shared foundation is complete.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish source locations, constants, scaffolding, and verification entry points for the network resource plugin.

- [X] T001 Create `pkg/networkresourceplugin/` with package documentation and initial files `pkg/networkresourceplugin/config.go`, `pkg/networkresourceplugin/manager.go`, `pkg/networkresourceplugin/server.go`, `pkg/networkresourceplugin/register.go`, `pkg/networkresourceplugin/devices.go`, `pkg/networkresourceplugin/discovery.go`, and `pkg/networkresourceplugin/node_reconcile.go`
- [X] T002 Add canonical constants for `spiderpoolAgent.networkResourcePlugin`, `spidernet.io/sub-eni`, `spidernet.io/<master>-nic` suffix handling, and `spidernet.io/network-resource` in `pkg/constant/k8s.go`
- [X] T003 [P] Add network resource plugin e2e skeleton files `test/e2e/networkresourceplugin/network_resource_plugin_suite_test.go` and `test/e2e/networkresourceplugin/network_resource_plugin_test.go`
- [X] T004 [P] Rename or replace the stale Helm rendering helper with `tools/helm/network_resource_plugin_render_test.sh`
- [X] T005 [P] Record the dependency and generated-artifact decision for the current `k8s.io/kubelet` device plugin API in `specs/005-agent-eni-device-plugin/plan.md`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Implement shared config, Helm rendering, lifecycle wiring, kubelet path handling, and local Node metadata plumbing used by every user story.

**Critical**: No user story work should begin until this phase is complete.

- [X] T006 Implement `NetworkResourcePluginConfig`, `SubENIAdvertisementConfig`, `MasterNICAdvertisementConfig`, and NIC rule config structs with defaulting in `pkg/networkresourceplugin/config.go`
- [X] T007 [P] Add validation tests for `enabled`, `kubeletRootDir`, extended resource names, annotation keys, selector syntax, and glob patterns in `pkg/networkresourceplugin/config_test.go`
- [X] T008 Implement config validation and defaulting helpers in `pkg/networkresourceplugin/config.go`
- [X] T009 Parse `spiderpoolAgent.networkResourcePlugin` from rendered Spiderpool config into agent and controller config in `cmd/spiderpool-agent/cmd/config.go` and `cmd/spiderpool-controller/cmd/config.go`
- [X] T010 Wire network resource plugin config into agent and controller startup contexts in `cmd/spiderpool-agent/cmd/daemon.go` and `cmd/spiderpool-controller/cmd/daemon.go`
- [X] T011 Update Helm defaults for `spiderpoolAgent.networkResourcePlugin` in `charts/spiderpool/values.yaml`
- [X] T012 Render `spiderpoolAgent.networkResourcePlugin` into the Spiderpool configmap in `charts/spiderpool/templates/configmap.yaml`
- [X] T013 Mount `{kubeletRootDir}/device-plugins` and `{kubeletRootDir}/plugins_registry` only when `spiderpoolAgent.networkResourcePlugin.enabled=true` in `charts/spiderpool/templates/daemonset.yaml`
- [X] T014 Add local Node watch RBAC needed by spiderpool-agent in `charts/spiderpool/templates/role.yaml`
- [ ] T015 [P] Add Helm rendering assertions for disabled defaults, enabled configmap fields, non-default `kubeletRootDir`, both kubelet hostPath mounts, and local Node RBAC in `tools/helm/network_resource_plugin_render_test.sh`
- [X] T016 [P] Add kubelet plugin path selection tests for default root, non-default root, preferred `plugins_registry`, and fallback `device-plugins` in `pkg/networkresourceplugin/register_test.go`
- [X] T017 Implement kubelet path derivation, selected path diagnostics, and fallback behavior in `pkg/networkresourceplugin/register.go`
- [ ] T018 Implement manager lifecycle start/stop scaffolding and dependency injection for device plugin servers, local Node cache, and NIC discovery in `pkg/networkresourceplugin/manager.go`
- [X] T019 Start and stop the network resource plugin from spiderpool-agent lifecycle when `spiderpoolAgent.networkResourcePlugin.enabled=true` in `cmd/spiderpool-agent/cmd/daemon.go`
- [X] T020 Run `make chart-readme` after changing `charts/spiderpool/values.yaml` and include generated updates in `charts/spiderpool/README.md`

**Checkpoint**: Foundation ready. Config, chart rendering, lifecycle wiring, and path handling are in place.

---

## Phase 3: User Story 1 - Schedule Pods Only Where Required Network Resources Are Available (Priority: P1) MVP

**Goal**: Eligible Pods request `spidernet.io/<master>-nic` and/or `spidernet.io/sub-eni`, and Kubernetes can schedule them only onto nodes advertising the required resources.

**Independent Test**: Configure master NIC advertisement and provider-mode sub-ENI capacity, create Pods referencing eligible SpiderMultusConfigs, verify webhook-injected resources, and verify unsuitable nodes are rejected by scheduler resource accounting.

### Tests for User Story 1

- [ ] T021 [P] [US1] Add webhook tests for `spiderpoolController.podResourceInject.enabled=false`, disabled `resourceAdvertisement.masterNIC`, disabled `resourceAdvertisement.subENI`, and provider-mode sub-ENI gating in `pkg/podmanager/pod_webhook_internal_test.go`
- [ ] T022 [P] [US1] Add webhook tests for injecting `spidernet.io/sub-eni` with the count of eligible VLAN SpiderMultusConfigs and preserving user-declared resources in `pkg/podmanager/pod_webhook_internal_test.go`
- [ ] T023 [P] [US1] Add webhook tests for injecting concrete `spidernet.io/<master>-nic` resources and preserving user-declared master NIC resources in `pkg/podmanager/pod_webhook_internal_test.go`
- [ ] T024 [P] [US1] Add e2e coverage for Pods scheduling only to nodes advertising required master NIC and sub-ENI resources in `test/e2e/networkresourceplugin/network_resource_plugin_test.go`

### Implementation for User Story 1

- [ ] T025 [P] [US1] Extend Pod webhook configuration inputs with `spiderpoolController.podResourceInject.enabled`, `resourceAdvertisement.subENI`, and `resourceAdvertisement.masterNIC` in `pkg/podmanager/pod_webhook.go`
- [ ] T026 [P] [US1] Add helpers to resolve eligible VLAN SpiderMultusConfigs and compute `spidernet.io/sub-eni` quantity from Pod Multus annotations in `pkg/podmanager/utils.go`
- [ ] T027 [P] [US1] Add helpers to resolve the selected concrete master NIC resource name from Spiderpool network configuration in `pkg/podmanager/utils.go`
- [ ] T028 [US1] Implement Spiderpool network resource injection for `spidernet.io/sub-eni` and `spidernet.io/<master>-nic` without overwriting existing container resources in `pkg/podmanager/pod_webhook.go`
- [ ] T029 [US1] Wire controller config into the Pod mutating webhook path so injection follows `spiderpoolAgent.networkResourcePlugin` and provider-mode state in `cmd/spiderpool-controller/cmd/daemon.go`
- [ ] T030 [US1] Add operator-visible webhook diagnostics for disabled resources, unresolved master NICs, invalid config, and non-overwrite decisions in `pkg/podmanager/pod_webhook.go`
- [ ] T031 [US1] Document Pod resource injection behavior and examples in `docs/usage/iaas-network-provider.md` and `docs/usage/iaas-network-provider-zh_CN.md`

**Checkpoint**: User Story 1 is independently testable through webhook tests and scheduling e2e coverage.

---

## Phase 4: User Story 2 - Keep Node Capacity Status Accurate (Priority: P2)

**Goal**: Enabled nodes advertise selected physical master NIC resources and provider-mode auxiliary ENI slot totals through kubelet device plugin resources, with dynamic Node label and annotation reconciliation.

**Independent Test**: Configure NIC rules and per-node sub-ENI capacity, inspect node allocatable resources, update Node labels/annotations, and verify kubelet-visible resources converge without agent restart.

### Tests for User Story 2

- [ ] T032 [P] [US2] Add unit tests for physical NIC filtering, shell-style include/exclude matching, omitted `nodeSelector`, multiple matching rules, and default-all behavior in `pkg/networkresourceplugin/discovery_test.go`
- [ ] T033 [P] [US2] Add unit tests for sub-ENI default capacity and zero-capacity behavior in `pkg/networkresourceplugin/node_reconcile_test.go`
- [ ] T034 [P] [US2] Add device plugin server tests for `ListAndWatch` output for `spidernet.io/sub-eni` and `spidernet.io/<master>-nic` resources in `pkg/networkresourceplugin/server_test.go`
- [ ] T035 [P] [US2] Add local Node watch/reconcile tests for exclude selectors, NIC profile label changes, and no-op updates in `pkg/networkresourceplugin/node_reconcile_test.go`
- [ ] T036 [P] [US2] Add e2e coverage for node allocatable master NIC resources, sub-ENI totals, exclude labels, and dynamic updates in `test/e2e/networkresourceplugin/network_resource_plugin_test.go`

### Implementation for User Story 2

- [X] T037 [P] [US2] Implement physical NIC discovery and filtering of loopback, CNI/container virtual interfaces, bridge devices, and non-physical interfaces in `pkg/networkresourceplugin/discovery.go`
- [X] T038 [P] [US2] Implement master NIC rule matching, deterministic include/exclude evaluation, and resource name construction in `pkg/networkresourceplugin/discovery.go`
- [X] T039 [P] [US2] Implement stable sub-ENI slot device ID generation from effective capacity in `pkg/networkresourceplugin/devices.go`
- [X] T040 [P] [US2] Implement desired resource set computation from provider mode, `resourceAdvertisement`, Node labels, and NIC discovery in `pkg/networkresourceplugin/node_reconcile.go`
- [ ] T041 [US2] Implement local Node watch/cache reconciliation that observes only the current Node and recomputes desired resources on relevant metadata changes in `pkg/networkresourceplugin/node_reconcile.go`
- [ ] T042 [US2] Implement kubelet device plugin gRPC server `ListAndWatch`, health reporting, and resource update notification for desired device lists in `pkg/networkresourceplugin/server.go`
- [X] T043 [US2] Implement registration for the configured sub-ENI resource and selected master NIC resources with retry, socket cleanup, and path diagnostics in `pkg/networkresourceplugin/register.go`
- [ ] T044 [US2] Add manager coordination so device plugin streams are updated only when the computed resource set changes in `pkg/networkresourceplugin/manager.go`
- [ ] T045 [US2] Add logs, events, or metrics for advertised totals, selected master NICs, derived free slot diagnostics, excluded nodes, and reconcile decisions in `pkg/networkresourceplugin/manager.go` and `pkg/metric/metrics_eni.go`
- [ ] T046 [US2] Document node capacity status, NIC rule behavior, dynamic reconciliation, and troubleshooting in `docs/reference/spiderpool-agent.md` and `docs/reference/spiderpool-agent-zh_CN.md`

**Checkpoint**: User Story 2 is independently testable through device plugin tests, Node reconcile tests, Helm rendering, and node-status e2e coverage.

---

## Phase 5: User Story 3 - Release Auxiliary ENI Capacity Reliably (Priority: P3)

**Goal**: Auxiliary ENI slot assignments remain stable through allocation, release, cleanup retries, and restarts, while provider allocation ownership stays in the existing IPAM/IaaS flow.

**Independent Test**: Create and delete Pods that request `spidernet.io/sub-eni` repeatedly, restart kubelet or spiderpool-agent, and verify later Pods can schedule once prior Pod requests no longer consume capacity.

### Tests for User Story 3

- [ ] T047 [P] [US3] Add `Allocate` tests for known slot IDs, unknown slot rejection, repeated allocation calls, and empty successful runtime responses in `pkg/networkresourceplugin/server_test.go`
- [ ] T048 [P] [US3] Add restart reconciliation tests for socket removal, re-registration, stable slot IDs, and kubelet checkpoint assumptions in `pkg/networkresourceplugin/register_test.go`
- [ ] T049 [P] [US3] Add package or e2e tests for create/delete Pod cycles releasing `spidernet.io/sub-eni` scheduling capacity in `test/e2e/networkresourceplugin/network_resource_plugin_test.go`
- [ ] T050 [P] [US3] Add tests that provider allocation and release ownership remains in IPAM/IaaS paths in `pkg/ipam/iaas_test.go`

### Implementation for User Story 3

- [X] T051 [US3] Implement deterministic `Allocate` handling for known sub-ENI slot IDs and master NIC placeholder devices in `pkg/networkresourceplugin/server.go`
- [ ] T052 [US3] Ensure `Allocate` does not move provider attach or IP allocation ownership out of existing IPAM/IaaS flow in `pkg/ipam/iaas.go`
- [ ] T053 [US3] Implement kubelet restart detection, socket lifecycle handling, re-registration, and stable resource names after restart in `pkg/networkresourceplugin/register.go`
- [ ] T054 [US3] Ensure active Pod requests are not double-counted and decreasing configured capacity below active requests blocks only future scheduling in `pkg/networkresourceplugin/node_reconcile.go`
- [ ] T055 [US3] Add diagnostic logging for allocation, release assumptions, restart recovery, duplicate allocation avoidance, and unknown slot errors in `pkg/networkresourceplugin/server.go`
- [ ] T056 [US3] Document restart recovery, total-capacity semantics, release behavior, and troubleshooting in `docs/usage/iaas-network-provider.md` and `docs/usage/iaas-network-provider-zh_CN.md`

**Checkpoint**: User Story 3 is independently testable through allocation/restart tests and create/delete e2e coverage.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Synchronize docs, generated artifacts, quality gates, and end-to-end validation across all stories.

- [ ] T057 [P] Update config reference documentation for `spiderpoolAgent.networkResourcePlugin` values in `docs/reference/configmap.md` and `docs/reference/configmap-zh_CN.md`
- [ ] T058 [P] Update quickstart validation notes and test commands in `specs/005-agent-eni-device-plugin/quickstart.md`
- [X] T059 [P] Run focused package tests for `pkg/networkresourceplugin`, `pkg/podmanager`, `cmd/spiderpool-agent/cmd`, and `cmd/spiderpool-controller/cmd` from repository root `/root/cyclinder/spiderpool`
- [ ] T060 Run `make chart-readme-verify` from repository root `/root/cyclinder/spiderpool`
- [X] T061 Run `make gofmt` from repository root `/root/cyclinder/spiderpool`
- [ ] T062 Run `make lint-golang` from repository root `/root/cyclinder/spiderpool` or record a maintainer-approved exception with risk in `specs/005-agent-eni-device-plugin/tasks.md`
- [ ] T063 Run targeted network resource plugin e2e tests from repository root `/root/cyclinder/spiderpool` or record the required cluster prerequisites and deferred risk in `specs/005-agent-eni-device-plugin/tasks.md`
- [ ] T064 Review generated and source artifact impact for `api/`, `pkg/k8s/apis/`, `charts/spiderpool/crds/`, and `charts/spiderpool/README.md`, running generation targets only if source APIs changed

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately.
- **Foundational (Phase 2)**: Depends on Phase 1; blocks all user stories.
- **User Story 1 (Phase 3)**: Depends on Phase 2; MVP scope.
- **User Story 2 (Phase 4)**: Depends on Phase 2; can proceed in parallel with US1 after the foundation, but full scheduling demos benefit from US1 webhook injection.
- **User Story 3 (Phase 5)**: Depends on Phase 2; can proceed in parallel with US1/US2 after the foundation, but release demos benefit from US2 capacity advertisement.
- **Polish (Phase 6)**: Depends on the selected user stories being complete.

### User Story Dependencies

- **US1 (P1)**: Can start after Foundation. Delivers webhook resource requests and scheduling protection for MVP.
- **US2 (P2)**: Can start after Foundation. Delivers node-visible resource advertisement, capacity accuracy, NIC rules, and dynamic reconciliation.
- **US3 (P3)**: Can start after Foundation. Delivers allocation/release/restart robustness for auxiliary ENI capacity.

### Within Each User Story

- Tests should be added or updated before implementation is considered complete.
- Config/types before manager/server implementation.
- Device discovery and desired resource computation before kubelet stream updates.
- Webhook eligibility helpers before mutation wiring.
- Core implementation before docs and e2e validation.

---

## Parallel Opportunities

- T003, T004, and T005 can run in parallel after T001 and T002 are understood.
- T007, T015, and T016 can run in parallel while config and chart implementation are being developed.
- US1 test tasks T021 through T024 can run in parallel.
- US1 helper implementation tasks T025 through T027 can run in parallel before T028.
- US2 test tasks T032 through T036 can run in parallel.
- US2 implementation tasks T037 through T040 can run in parallel before T041 through T044.
- US3 test tasks T047 through T050 can run in parallel.
- Polish documentation and focused test tasks T057 through T059 can run in parallel after story implementation.

---

## Parallel Example: User Story 1

```bash
Task: "Add webhook tests for injection gates in pkg/podmanager/pod_webhook_internal_test.go"
Task: "Add webhook tests for sub-ENI quantity and non-overwrite in pkg/podmanager/pod_webhook_internal_test.go"
Task: "Add webhook tests for master NIC injection and non-overwrite in pkg/podmanager/pod_webhook_internal_test.go"
Task: "Add e2e coverage for scheduling resources in test/e2e/networkresourceplugin/network_resource_plugin_test.go"
```

## Parallel Example: User Story 2

```bash
Task: "Implement physical NIC discovery in pkg/networkresourceplugin/discovery.go"
Task: "Implement stable sub-ENI slot device IDs in pkg/networkresourceplugin/devices.go"
Task: "Implement desired resource set computation in pkg/networkresourceplugin/node_reconcile.go"
Task: "Add ListAndWatch tests in pkg/networkresourceplugin/server_test.go"
```

## Parallel Example: User Story 3

```bash
Task: "Add Allocate tests in pkg/networkresourceplugin/server_test.go"
Task: "Add restart reconciliation tests in pkg/networkresourceplugin/register_test.go"
Task: "Add provider ownership tests in pkg/ipam/iaas_test.go"
Task: "Add create/delete cycle e2e coverage in test/e2e/networkresourceplugin/network_resource_plugin_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1 setup.
2. Complete Phase 2 foundation.
3. Complete Phase 3 User Story 1.
4. Validate webhook injection and scheduler placement for Pods requiring Spiderpool network resources.
5. Stop and review before adding dynamic capacity and restart robustness.

### Incremental Delivery

1. Foundation: config, Helm rendering, lifecycle, path selection, and manager scaffolding.
2. US1: webhook resource injection and scheduling protection.
3. US2: node capacity advertisement, physical NIC rules, and dynamic reconciliation.
4. US3: allocation/release/restart behavior.
5. Polish: docs, generated artifacts, chart README, lint, focused tests, and e2e validation.

### Validation Commands

```bash
make chart-readme
make chart-readme-verify
make gofmt
make lint-golang
go test ./pkg/networkresourceplugin ./pkg/podmanager ./cmd/spiderpool-agent/cmd ./cmd/spiderpool-controller/cmd
```

---

## Notes

- All tasks use unchecked markdown checklist format and include exact file paths.
- `[P]` marks tasks that touch different files or can be prepared independently.
- User story tasks use `[US1]`, `[US2]`, or `[US3]` labels for traceability.
- Documentation tasks that touch `docs/` must update English and Chinese files together.
- When `charts/spiderpool/values.yaml` changes, run `make chart-readme` and include `charts/spiderpool/README.md`.
