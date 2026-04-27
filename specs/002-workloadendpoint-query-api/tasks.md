# Tasks: WorkloadEndpoint Query API with MAC Address

**Input**: Design documents from `/specs/002-workloadendpoint-query-api/`  
**Prerequisites**: plan.md, spec.md, data-model.md, contracts/openapi.yaml

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Repository root**: `/Users/cyclinder/Desktop/code/spiderpool/`
- **CRD types**: `pkg/k8s/apis/spiderpool.spidernet.io/v2beta1/`
- **API spec**: `api/v1/agent/`
- **Agent cmd**: `cmd/spiderpool-agent/cmd/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: No setup needed - project is already initialized. Proceed to foundational tasks.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: CRD schema change and OpenAPI spec extension that MUST be complete before user story implementation

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

### CRD Schema Extension

- [ ] T001 [P] Add `mac` field to `IPAllocationDetail` struct in `pkg/k8s/apis/spiderpool.spidernet.io/v2beta1/spiderendpoint_types.go`
- [ ] T002 Regenerate deepcopy code: `make generate` or `controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./pkg/k8s/apis/..."`
- [ ] T003 Verify CRD schema is valid and backward compatible

### OpenAPI Spec Extension

- [ ] T004 Extend `api/v1/agent/openapi.yaml` with GET `/workloadendpoint` endpoint definition
- [ ] T005 Extend `api/v1/agent/openapi.yaml` with `WorkloadEndpointStatus` and `InterfaceDetail` definitions
- [ ] T006 Extend `api/v1/agent/openapi.yaml` with `mac` field in `IpConfig` definition (for IPAM request)
- [ ] T007 Run `swagger generate` to update generated client/server code

**Checkpoint**: Foundation ready - CRD has mac field, OpenAPI spec extended, generated code updated

---

## Phase 3: User Story 1 - Query Pod Network Allocation Details (Priority: P1) 🎯 MVP

**Goal**: Implement GET `/v1/workloadendpoint` API that returns Pod network allocation details via Unix Socket

**Independent Test**: Query API for existing Pod and verify response contains expected interface details with IP, optional VLAN, and optional MAC fields

### Implementation for User Story 1

- [ ] T008 [P] Implement `GetWorkloadendpoint` handler in `cmd/spiderpool-agent/cmd/ipam.go` (or new file)
- [ ] T009 [P] Implement parameter validation (podNamespace, podName required) in handler
- [ ] T010 Implement SpiderEndpoint lookup by Pod namespace/name in handler
- [ ] T011 Implement response transformation from SpiderEndpoint to WorkloadEndpointStatus
- [ ] T012 Implement field omission logic (exclude mac/vlan when not set)
- [ ] T013 Add error handling for 404 (SpiderEndpoint not found)
- [ ] T014 Add error handling for 400 (missing/invalid parameters)
- [ ] T015 Wire handler to router in `cmd/spiderpool-agent/cmd/unix_server.go`

**Checkpoint**: User Story 1 complete - API returns Pod network details, handles errors correctly

---

## Phase 4: User Story 2 - MAC Address Recording for External Integration (Priority: P2)

**Goal**: Accept MAC address in IPAM request and record to SpiderEndpoint

**Independent Test**: Simulate IPAM request with MAC provided, verify MAC appears in SpiderEndpoint and subsequent API queries

### Implementation for User Story 2

- [ ] T016 [P] Add `mac` field to `IpamAddArgs` model (swagger generated)
- [ ] T017 [P] Add `mac` field to `IPConfig` model (swagger generated)
- [ ] T018 Update IPAM allocation handler to extract MAC from request parameters in `cmd/spiderpool-agent/cmd/ipam.go`
- [ ] T019 Update IP allocation recording to include MAC when provided in `pkg/podmanager/` or IPAM package
- [ ] T020 Add MAC address format validation before recording

**Checkpoint**: User Story 2 complete - MAC accepted in IPAM request and recorded to SpiderEndpoint

---

## Phase 5: User Story 3 - Backward Compatibility (Priority: P3)

**Goal**: Ensure existing API consumers continue to function without modification

**Independent Test**: Verify existing Spiderflat component can query API and parse responses without changes

### Implementation for User Story 3

- [ ] T021 [P] Add unit test for field omission (mac/vlan not present when unset) in `pkg/podmanager/` tests
- [ ] T022 [P] Add integration test for backward compatibility (old client parses new response) in test/e2e or test/integration
- [ ] T023 Verify response serialization omits empty fields (not null/empty string)

**Checkpoint**: User Story 3 complete - Existing clients work unchanged

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, validation, and final integration

- [ ] T024 [P] Update CRD documentation with new `mac` field description
- [ ] T025 [P] Add API usage example to `docs/` (reference quickstart.md content)
- [ ] T026 Add logging for API queries (request/response with trace ID)
- [ ] T027 Add metrics for API query latency and error rates (if metrics framework exists)
- [ ] T028 Final code review and formatting (`make lint` or `gofmt`)

**Checkpoint**: Feature complete - All user stories functional, documented, production-ready

---

## Dependency Graph

```
Phase 2 (Foundational)
├── T001 (CRD mac field)
├── T002 (deepcopy regen) ── depends on T001
├── T003 (CRD verify) ── depends on T002
├── T004-T006 (OpenAPI extend) ── can parallel with T001-T003
└── T007 (swagger generate) ── depends on T004-T006

Phase 3 (US1 - Query API) ── depends on Phase 2 complete
├── T008-T009 (handler, validation) ── parallel
├── T010-T012 (lookup, transform, omit) ── depends on T008
└── T013-T015 (error handling, wiring) ── depends on T010-T012

Phase 4 (US2 - MAC Recording) ── depends on Phase 2 complete
├── T016-T017 (model updates) ── parallel
├── T018-T019 (handler update, recording) ── depends on T016-T017
└── T020 (validation) ── depends on T018

Phase 5 (US3 - Backward Compat) ── depends on Phase 3 complete
├── T021-T023 (tests) ── parallel

Phase 6 (Polish) ── depends on all above
└── T024-T028 ── parallel
```

---

## Parallel Execution Examples

### Maximum Parallelism Setup

```bash
# Phase 2 - Run in parallel terminals:
# Terminal 1: CRD changes
vim pkg/k8s/apis/spiderpool.spidernet.io/v2beta1/spiderendpoint_types.go

# Terminal 2: OpenAPI changes (simultaneously)
vim api/v1/agent/openapi.yaml

# After both complete:
make generate
swagger generate
```

### Per-Story Development

**US1 Developer** can work on handler logic while **US2 Developer** updates models:
- US1 uses mock/stub MAC data
- US2 provides real MAC storage
- Integration happens at checkpoint

---

## Implementation Strategy

**MVP First**: Deliver User Story 1 (Query API) as standalone feature:
- API works with existing SpiderEndpoint data (no MAC field yet)
- Provides immediate value for IP/VLAN visibility
- Can be deployed independently

**Incremental Delivery**:
1. Phase 2: Foundation (CRD + OpenAPI) - 1-2 days
2. Phase 3: US1 (Query API) - 2-3 days → MVP ready
3. Phase 4: US2 (MAC Recording) - 1-2 days
4. Phase 5: US3 (Backward Compat) - 1 day
5. Phase 6: Polish - 1 day

**Total Estimate**: 6-10 days for full feature
