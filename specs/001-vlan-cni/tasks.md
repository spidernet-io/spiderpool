# Tasks: VLAN CNI Support in Spiderpool

**Input**: Design documents from `specs/001-vlan-cni/`
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `quickstart.md`

## Format: `[ID] [P?] [Story] Description`

- `[P]`: Can run in parallel (different files, no blocking dependency)
- `[Story]`: User story label (`[US1]`, `[US2]`, `[US3]`)
- Include exact file paths in every task

## Phase 1: Setup

**Purpose**: Confirm generated artifacts and prepare implementation targets.

- [ ] T001 Review and align VLAN CNI implementation scope in `specs/001-vlan-cni/spec.md`, `specs/001-vlan-cni/plan.md`, and `specs/001-vlan-cni/data-model.md`
- [ ] T002 Inspect existing macvlan and ipvlan handling patterns in `pkg/k8s/apis/spiderpool.spidernet.io/v2beta1/spidermultus_types.go`, `pkg/multuscniconfig/multusconfig_informer.go`, `pkg/multuscniconfig/multusconfig_validate.go`, `pkg/podmanager/pod_webhook.go`, and `pkg/namespacemanager/namespace_manager.go`
- [ ] T003 Identify or add the VLAN CNI type constant in `pkg/constant/k8s.go` or the actual constant definition file used by existing CNI types

## Phase 2: Foundational Tasks

**Purpose**: Add shared schema and type support required by all user stories.

- [ ] T004 Add `vlan` support to `MultusCNIConfigSpec` and define `SpiderVlanCniConfig` with `master`, `vlanID`, `mtu`, `bond`, `rdmaResourceName`, and `ippools` in `pkg/k8s/apis/spiderpool.spidernet.io/v2beta1/spidermultus_types.go`
- [ ] T005 [P] Add CRD/OpenAPI validation tags for VLAN fields in `pkg/k8s/apis/spiderpool.spidernet.io/v2beta1/spidermultus_types.go`
- [ ] T006 Update generated deepcopy or related API helper artifacts for `pkg/k8s/apis/spiderpool.spidernet.io/v2beta1/spidermultus_types.go` in the corresponding generated files under `pkg/k8s/apis/spiderpool.spidernet.io/v2beta1/`
- [ ] T007 Add or update shared helper structures/functions needed for VLAN NAD generation with MTU propagation in `pkg/multuscniconfig/multusconfig_informer.go`

## Phase 3: User Story 1 - Basic VLAN network configuration (Priority: P1)

**Goal**: Users can create a `SpiderMultusConfig` with `cniType: vlan` and a single `vlan` block, and Spiderpool generates a correct VLAN NAD with optional MTU and IPPool settings.

**Independent Test Criteria**: Creating a single-master VLAN `SpiderMultusConfig` generates a valid `NetworkAttachmentDefinition` whose VLAN plugin config includes the correct `master`, `vlanId`, optional `mtu`, and Spiderpool IPAM settings.

- [ ] T008 [US1] Add `cniType: vlan` dispatch handling in `pkg/multuscniconfig/multusconfig_informer.go`
- [ ] T009 [US1] Implement single-master VLAN NAD generation with `master`, `vlanId`, optional `mtu`, and IPAM fields in `pkg/multuscniconfig/multusconfig_informer.go`
- [ ] T010 [US1] Ensure `vlan` spec fields are translated to NAD config without using macvlan runtime semantics in `pkg/multuscniconfig/multusconfig_informer.go`
- [ ] T011 [P] [US1] Add unit tests for single-master VLAN NAD generation, including MTU translation, in the informer test file associated with `pkg/multuscniconfig/multusconfig_informer.go`

## Phase 4: User Story 2 - Multi-NIC bond VLAN configuration (Priority: P2)

**Goal**: Users can configure multiple masters with bond settings, and Spiderpool generates an NAD plugin chain that prepares the bond first and then applies the VLAN plugin with optional MTU.

**Independent Test Criteria**: Creating a multi-master VLAN `SpiderMultusConfig` generates an NAD plugin chain ordered as `ifacer` then `vlan`, where the VLAN plugin uses the bond name as `master` and includes `mtu` when configured.

- [ ] T012 [US2] Implement multi-master VLAN translation to an `ifacer` + `vlan` plugin chain in `pkg/multuscniconfig/multusconfig_informer.go`
- [ ] T013 [US2] Propagate configured `bond.name`, `vlanId`, and optional `mtu` into the generated VLAN NAD chain in `pkg/multuscniconfig/multusconfig_informer.go`
- [ ] T014 [P] [US2] Add unit tests for multi-master bond VLAN NAD generation, including plugin order and MTU translation, in the informer test file associated with `pkg/multuscniconfig/multusconfig_informer.go`

## Phase 5: User Story 3 - RDMA-accelerated VLAN network and validation (Priority: P3)

**Goal**: Users can configure RDMA-enabled VLAN networks and receive correct webhook validation errors for invalid VLAN configurations.

**Independent Test Criteria**: Valid VLAN configs with `rdmaResourceName` pass validation, invalid VLAN configs are rejected with clear field errors, and resource injection semantics remain consistent with existing Spiderpool behavior.

- [ ] T015 [US3] Add VLAN-specific webhook validation for required `vlan` config presence, `master`, `vlanID` range, `bond` consistency, `mtu`, and mixed-config rejection in `pkg/multuscniconfig/multusconfig_validate.go`
- [ ] T016 [US3] Add RDMA-related validation for VLAN configuration in `pkg/multuscniconfig/multusconfig_validate.go`
- [ ] T017 [P] [US3] Add unit tests covering valid and invalid VLAN webhook validation cases in the test file associated with `pkg/multuscniconfig/multusconfig_validate.go`
- [ ] T018 [P] [US3] Add tests covering RDMA resource validation for VLAN config in the test file associated with `pkg/multuscniconfig/multusconfig_validate.go`

## Phase 6: User Story 4 - Namespace-scoped tenant default injection (Priority: P1)

**Goal**: Multi-tenant environments can set `cni.spidernet.io/network-resource-inject` on a Namespace so Pods inherit the tenant-default VLAN injection value without repeating Pod annotations.

**Independent Test Criteria**: A Pod without `cni.spidernet.io/network-resource-inject` but in a Namespace that defines it gets the expected auto-injected Multus networks, and Namespace lookup uses the cached manager path.

- [ ] T019 [US4] Add namespace-fallback resolution for `cni.spidernet.io/network-resource-inject` in `pkg/podmanager/pod_webhook.go` and/or `pkg/podmanager/utils.go`
- [ ] T020 [US4] Wire webhook namespace reads through `pkg/namespacemanager/namespace_manager.go` or the equivalent controller-initialized manager path using cached reads
- [ ] T021 [P] [US4] Add unit tests for Namespace-based fallback injection in the relevant test files under `pkg/podmanager/`

## Phase 7: User Story 5 - Pod overrides Namespace tenant default (Priority: P1)

**Goal**: When both Pod and Namespace define `cni.spidernet.io/network-resource-inject`, the Pod-specific value wins.

**Independent Test Criteria**: A Pod with its own injection annotation is mutated using the Pod value even when the Namespace defines a different tenant default.

- [ ] T022 [US5] Implement explicit Pod-over-Namespace precedence in `pkg/podmanager/pod_webhook.go` and/or `pkg/podmanager/utils.go`
- [ ] T023 [P] [US5] Add unit tests covering Pod override precedence and no-annotation no-op behavior in the relevant test files under `pkg/podmanager/`

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Complete documentation, consistency, and regression confidence.

- [ ] T024 Update English VLAN usage documentation, examples, MTU behavior, and Namespace-scoped injection behavior in `docs/usage/spider-multus-config.md`
- [ ] T025 [P] Update Chinese VLAN usage documentation, examples, MTU behavior, and Namespace-scoped injection behavior in `docs/usage/spider-multus-config-zh_CN.md`
- [ ] T026 [P] Add or update end-to-end or integration coverage for VLAN CNI and Namespace-based injection scenarios in the relevant test files under the repository test directories
- [ ] T027 Verify backward compatibility for existing non-VLAN CNI flows by updating or running affected regression tests in the relevant test files under the repository test directories
- [ ] T028 Review generated planning artifacts for terminology consistency (`vlan` vs `vlanConfig`, MTU support, Pod-vs-Namespace precedence, cache-based Namespace lookup) in `specs/001-vlan-cni/spec.md`, `specs/001-vlan-cni/plan.md`, `specs/001-vlan-cni/research.md`, `specs/001-vlan-cni/data-model.md`, and `specs/001-vlan-cni/quickstart.md`

## Dependencies

### Phase Dependencies

- Setup (Phase 1) must complete before Foundational Tasks (Phase 2)
- Foundational Tasks (Phase 2) must complete before User Stories (Phases 3-5)
- User Story 1 (Phase 3) should complete before User Story 2 (Phase 4) because multi-master VLAN reuses single VLAN generation patterns
- User Story 3 (Phase 5) depends on foundational schema support and can proceed in parallel with later US1/US2 test work once VLAN types exist
- User Story 4 (Phase 6) depends on the existing pod webhook injection path and namespace manager integration points
- User Story 5 (Phase 7) depends on User Story 4 annotation inheritance flow
- Polish (Phase 8) depends on completion of all targeted user stories

### User Story Dependencies

- **US1**: Depends on T004-T007
- **US2**: Depends on T004-T007 and benefits from T008-T010
- **US3**: Depends on T004-T006
- **US4**: Depends on the existing Pod webhook injection path and Namespace manager availability
- **US5**: Depends on US4

## Parallel Execution Examples

### User Story 1

- Run T009 and T011 in parallel after T008 is complete if the test file is separate from the implementation flow

### User Story 2

- Run T013 and T014 in parallel after T012 defines the plugin-chain structure

### User Story 3

- Run T017 and T018 in parallel after T015 and T016 establish validation behavior

### User Story 4

- Run T020 and T021 in parallel after T019 defines the namespace-fallback flow

### User Story 5

- Run T022 and T023 in parallel once the override precedence rules are finalized

### Polish

- Run T024 and T025 in parallel
- Run T026 and T027 in parallel if they touch separate test areas

## Implementation Strategy

### MVP First

1. Complete Phase 1 and Phase 2
2. Deliver **US1** as the MVP: single-master VLAN config with correct NAD generation and MTU translation
3. Add Namespace-scoped tenant default injection and Pod-over-Namespace precedence before broadening coverage
4. Validate single-master behavior before expanding to multi-master and RDMA validation

### Incremental Delivery

1. **Increment 1**: CRD/type support + single-master VLAN NAD generation
2. **Increment 2**: Multi-master bond-based VLAN chain generation
3. **Increment 3**: Namespace-scoped injection inheritance + Pod-over-Namespace precedence
4. **Increment 4**: Webhook validation + RDMA support
5. **Increment 5**: Docs and regression / e2e coverage

### Task Count Summary

- **Setup**: 3 tasks
- **Foundational**: 4 tasks
- **US1**: 4 tasks
- **US2**: 3 tasks
- **US3**: 4 tasks
- **US4**: 3 tasks
- **US5**: 2 tasks
- **Polish**: 5 tasks
- **Total**: 28 tasks
