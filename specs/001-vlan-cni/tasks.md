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
- [ ] T002 Inspect existing macvlan and ipvlan handling patterns in `pkg/k8s/apis/spiderpool.spidernet.io/v2beta1/spidermultus_types.go`, `pkg/multuscniconfig/multusconfig_informer.go`, and `pkg/multuscniconfig/multusconfig_validate.go`
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

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Complete documentation, consistency, and regression confidence.

- [ ] T019 Update English VLAN usage documentation, examples, and MTU behavior in `docs/usage/spider-multus-config.md`
- [ ] T020 [P] Update Chinese VLAN usage documentation, examples, and MTU behavior in `docs/usage/spider-multus-config-zh_CN.md`
- [ ] T021 [P] Add or update end-to-end or integration coverage for VLAN CNI scenarios, including MTU-related expectations, in the relevant test files under the repository test directories
- [ ] T022 Verify backward compatibility for existing non-VLAN CNI flows by updating or running affected regression tests in the relevant test files under the repository test directories
- [ ] T023 Review generated planning artifacts for terminology consistency (`vlan` vs `vlanConfig`, MTU support) in `specs/001-vlan-cni/spec.md`, `specs/001-vlan-cni/plan.md`, `specs/001-vlan-cni/research.md`, `specs/001-vlan-cni/data-model.md`, and `specs/001-vlan-cni/quickstart.md`

## Dependencies

### Phase Dependencies

- Setup (Phase 1) must complete before Foundational Tasks (Phase 2)
- Foundational Tasks (Phase 2) must complete before User Stories (Phases 3-5)
- User Story 1 (Phase 3) should complete before User Story 2 (Phase 4) because multi-master VLAN reuses single VLAN generation patterns
- User Story 3 (Phase 5) depends on foundational schema support and can proceed in parallel with later US1/US2 test work once VLAN types exist
- Polish (Phase 6) depends on completion of all targeted user stories

### User Story Dependencies

- **US1**: Depends on T004-T007
- **US2**: Depends on T004-T007 and benefits from T008-T010
- **US3**: Depends on T004-T006

## Parallel Execution Examples

### User Story 1

- Run T009 and T011 in parallel after T008 is complete if the test file is separate from the implementation flow

### User Story 2

- Run T013 and T014 in parallel after T012 defines the plugin-chain structure

### User Story 3

- Run T017 and T018 in parallel after T015 and T016 establish validation behavior

### Polish

- Run T019 and T020 in parallel
- Run T021 and T022 in parallel if they touch separate test areas

## Implementation Strategy

### MVP First

1. Complete Phase 1 and Phase 2
2. Deliver **US1** as the MVP: single-master VLAN config with correct NAD generation and MTU translation
3. Validate single-master behavior before expanding to multi-master and RDMA validation

### Incremental Delivery

1. **Increment 1**: CRD/type support + single-master VLAN NAD generation
2. **Increment 2**: Multi-master bond-based VLAN chain generation
3. **Increment 3**: Webhook validation + RDMA support
4. **Increment 4**: Docs and regression / e2e coverage

### Task Count Summary

- **Setup**: 3 tasks
- **Foundational**: 4 tasks
- **US1**: 4 tasks
- **US2**: 3 tasks
- **US3**: 4 tasks
- **Polish**: 5 tasks
- **Total**: 23 tasks
