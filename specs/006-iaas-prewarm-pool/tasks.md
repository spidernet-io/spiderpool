# Tasks: IaaS Provider Prewarm IP Pool Support

**Input**: Design documents from `/specs/006-iaas-prewarm-pool/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Spiderpool behavior changes require tests. This feature's risk is
concentrated in the admission webhook (mutating/validating) and the IPAM
allocation hot path, so Ginkgo/Gomega unit tests are REQUIRED for every new
behavior (per Constitution Principle II). No new e2e suite is added in this
phase (recorded as an explicit, low-risk scope decision in plan.md, since the
external private provider needed to populate real ledger data end-to-end is
outside this repository); `quickstart.md` documents a manual/kubectl
validation path instead.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Kubernetes APIs**: `pkg/k8s/apis/spiderpool.spidernet.io/v2beta1/`
- **Reusable Go logic**: `pkg/constant/`, `pkg/ippoolmanager/`, `pkg/ipam/`, `pkg/types/`
- **Helm/CRD packaging**: `charts/spiderpool/crds/`
- **User and developer documentation**: `docs/` (English + `zh_CN`)
- **Feature artifacts**: `specs/006-iaas-prewarm-pool/`

---

## Phase 1: Setup

**Purpose**: Confirm baseline repository state and quality gates before touching any code

- [X] T001 Confirm `make gofmt` and `make lint-golang` pass on a clean checkout of branch `006-iaas-prewarm-pool` (baseline, before any change)
- [X] T002 [P] Run `make unittest-tests` once on the unmodified branch to capture a baseline pass/fail snapshot for `pkg/ippoolmanager` and `pkg/ipam`
- [X] T003 [P] Review `docs/develop/proposal-iaas-ip-provider.md`, `specs/006-iaas-prewarm-pool/{spec.md,plan.md,data-model.md,research.md,contracts/spiderippool-iaas-extension.md}` together to confirm no drift before implementation starts

**Checkpoint**: Baseline confirmed clean; safe to start Foundational phase

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: CRD type/constant additions that every user story depends on. No user story work can begin until this phase is complete and `make manifests generate-k8s-api` has been run successfully.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T004 Add annotation/label constants `AnnoIPPoolIaas = AnnotationPre + "/iaas-pool"`, `AnnoIPPoolPairPool = AnnotationPre + "/pair-pool"`, `LabelIPPoolIaas = AnnoIPPoolIaas` in `pkg/constant/k8s.go` (next to existing `LabelIPPoolCIDR`/`AnnoSpiderSubnet` block)
- [X] T005 ~~Add `IaasIPAllocation` struct (`IPv4 *string`, `IPv6 *string`, `MAC string`, `VLANID *int32`, `Phase string` with `+kubebuilder:validation:Enum=Ready;NotReady;Releasing`, `LastError string`) and extend `IPPoolStatus` with `IaasIPs []IaasIPAllocation` and `Conditions []metav1.Condition` in `pkg/k8s/apis/spiderpool.spidernet.io/v2beta1/spiderippool_types.go` (per data-model.md §1.3)~~ **SUPERSEDED by T005-R below** (see Phase 5.1)
- [X] T006 Run `make manifests generate-k8s-api` to regenerate `charts/spiderpool/crds/spiderpool.spidernet.io_spiderippools.yaml` and `pkg/k8s/apis/spiderpool.spidernet.io/v2beta1/zz_generated.deepcopy.go`; verify the CRD diff is purely additive (no removed/renamed fields)
- [X] T007 [P] Add `FromIaasLedger bool` field to `types.AllocationResult` in `pkg/types/ip.go`
- [X] T008 Change `IPPoolManager.AllocateIP` interface signature in `pkg/ippoolmanager/ippool_manager.go:36` and its implementation at `ippool_manager.go:96` to `(ip *models.IPConfig, fromIaasLedger bool, err error)`, updating the implementation body to currently always return `false` for the new value (actual ledger logic lands in US3); update the single call site in `pkg/ipam/allocate.go:621` (`allocateIPFromCandidate`) to receive and store the new return value into `types.AllocationResult.FromIaasLedger` (built at `allocate.go:626-629`)
- [X] T009 [P] Add ledger helper scaffolding in `pkg/ippoolmanager/utils.go`: a function to parse/validate ledger entries (skip entries with neither `IPv4` nor `IPv6` set, per data-model.md validation rules) — implementation of the readiness-intersection selection itself is added in US3/Phase 5.1, but the shared parsing helper is foundational since both mutate/validate (US1) and AllocateIP (US3) need consistent capacity/entry parsing
- [X] T010 Define and document the performance budget check for this feature: zero added Kubernetes API calls and zero added latency for pools without the `iaas-pool` label (plan.md "Performance Goals") — add a short comment block at the top of the new code path in `AllocateIP` and `pool_selections.go` stating this invariant, to be verified in Polish phase

**Checkpoint**: Foundation ready — CRD types compile, deepcopy/manifests regenerated, `AllocationResult`/`AllocateIP` plumbing in place (defaulted to no-op). User story implementation can now begin.

---

## Phase 3: User Story 1 - Operator declares a node-pinned, paired prewarm pool (Priority: P1) 🎯 MVP

**Goal**: Recognize `SpiderIPPool`s marked as IaaS pools via annotation, keep a synchronized label, and validate `pair-pool` references (self-reference, same-version, capacity, selector-consistency rules) without requiring any IaaS provider component to be present.

**Independent Test**: Create/update `SpiderIPPool` objects with `iaas-pool`/`pair-pool` annotations (paired and unpaired, valid and invalid) via `kubectl`/webhook unit tests, and verify label sync + validation outcomes with zero dependency on ledger data or IPAM allocation code.

### Tests for User Story 1

- [X] T011 [P] [US1] Ginkgo/Gomega test in `pkg/ippoolmanager/ippool_mutate_test.go` (new file, following `ippool_webhook_test.go` conventions incl. `Label(...)`) for: annotation `iaas-pool=true` results in synchronized label; annotation removed/changed results in label correction; pool without the annotation is unaffected
- [X] T012 [P] [US1] Ginkgo/Gomega test in `pkg/ippoolmanager/ippool_validate_test.go` (new file) for: self-referential `pair-pool` rejected; same-IP-version pairing rejected; reference to non-existent pool allowed; v4-capacity > v6-capacity rejected; v4-capacity <= v6-capacity allowed; mismatched `nodeName`/`podAffinity` between existing paired pools rejected; matching selectors allowed; pool without `pair-pool` unaffected

### Implementation for User Story 1

- [X] T013 [US1] Implement `iaas-pool` annotation -> label sync in `mutateIPPool` in `pkg/ippoolmanager/ippool_mutate.go` (mirrors the existing `LabelIPPoolCIDR` block at `ippool_mutate.go:56-62`; annotation authoritative, label corrected/removed on mismatch) — depends on T004
- [X] T014 [US1] Implement pairing validation functions in `pkg/ippoolmanager/ippool_validate.go`: self-reference check, same-IP-version check (no API call needed), and — when the referenced pool exists via client `Get` — static-capacity comparison (v4 <= v6, using the `AssembleTotalIPs`/`IPsDiffSet` pattern from `validateIPPoolAvailableIPs` at `ippool_validate.go:97-156`) and `nodeName`/`podAffinity` equality check; return errors using the existing `field.Invalid`/`field.Forbidden` style — depends on T004, T009
- [X] T015 [US1] Wire the new pairing validation into `ValidateCreate`/`ValidateUpdate` entrypoints in `pkg/ippoolmanager/ippool_webhook.go` (alongside the existing `validateIPPoolSpec` call at `ippool_validate.go:43`/`:68`), returning `apierrors.NewInvalid(...)` on failure per existing convention — depends on T014
- [X] T016 [US1] Add/update English documentation page describing `ipam.spidernet.io/iaas-pool` and `ipam.spidernet.io/pair-pool` annotations, the synchronized label, and pairing validation rules, under `docs/` (choose the existing concepts/usage doc section that documents other `SpiderIPPool` annotations) — depends on T013, T014
- [X] T017 [US1] Add the synchronized Chinese (`zh_CN`) counterpart of the T016 documentation page in the same change, per repo bilingual-docs convention — depends on T016

**Checkpoint**: At this point, User Story 1 is fully functional and testable independently — pools can be declared, paired, and validated with zero dependency on IPAM allocation changes.

---

## Phase 4: User Story 2 - Automatic dual-stack pool completion for Pod IP requests (Priority: P1)

**Goal**: When a Pod's IP pool annotation selects only one side of a paired pool, automatically include the paired pool of the opposite IP family during candidate resolution, without requiring any user-facing annotation change.

**Independent Test**: Configure a Pod (or the IPAM candidate-resolution unit test path) referencing only a v4 pool with a valid `pair-pool` annotation to a v6 pool, and confirm the resolved candidate list includes both, while pools without pairing remain unaffected.

### Tests for User Story 2

- [X] T018 [P] [US2] Ginkgo/Gomega test in `pkg/ipam/pool_selections_test.go` (new file, following `ipam_suite_test.go` `Label("ipam", "unittest")` convention) for: single-family pool request with valid `pair-pool` auto-completes the opposite family; already-explicit dual-stack request produces no duplicate entries; pool without `pair-pool` is completely unaffected; wildcard-expanded pool carrying `pair-pool` still auto-completes correctly

### Implementation for User Story 2

- [X] T019 [US2] Implement auto-completion logic in `getPoolCandidates` in `pkg/ipam/pool_selections.go` (integrated near the existing `EnableIPv4`/`EnableIPv6` candidate-splitting logic at `pool_selections.go:139-182`): for each resolved candidate pool carrying `AnnoIPPoolPairPool`, if the opposite-family candidate list does not already contain the referenced pool, append it (read-only `Get`/informer lookup of the pool's own annotations, no cloud/API-heavy operation) — depends on T004
- [X] T020 [US2] Add/update the English documentation page (from T016) with a section describing automatic dual-stack completion behavior and its interaction with wildcard pool name matching, under `docs/` — depends on T019
- [X] T021 [US2] Synchronize the `zh_CN` documentation counterpart for T020 in the same change — depends on T020

**Checkpoint**: At this point, User Stories 1 AND 2 both work independently — Pods can request a single pool name and have the correct pair resolved automatically, entirely independent of ledger-based allocation gating (US3).

---

## Phase 5: User Story 3 - Per-IP readiness gating and atomic IP-pair allocation (Priority: P1)

**Goal**: Gate IP allocation from an IaaS pool (label `iaas-pool`) by per-IP prewarm-readiness (`status.iaasReadyIPs`), atomically allocate matched v4/v6 pairs sharing the same address, allow single-stack Pods to draw a single family from a paired pool without error, and skip the pre-existing synchronous IaaS-provider call for ledger-sourced IPs (FR-015).

> **Revision note**: This phase's design was revised after initial implementation, per user clarification. See **Phase 5.1** below for the superseding schema/algorithm; the task list here is kept for history but T005/T022/T023/T025/T029 are superseded. **Both Phase 5 and Phase 5.1 are further superseded by the v5 design in Phase 5.2** (`ipMetaData` map + `iaas-provider` annotation; `iaasReadyIPs`/`iaasFailedIPs`/`conditions` removed).

**Independent Test**: Populate an `iaas-pool`-labeled pool's `status.iaasReadyIPs`/`status.iaasFailedIPs`, run `AllocateIP` directly in a unit test, and verify: only addresses present in both `spec.ips`(-derived candidates) and `iaasReadyIPs` are selected; paired entries (same address in both v4/v6 sibling pools' ledgers) are allocated atomically; single-stack requests draw only their own family; pools without the `iaas-pool` label are entirely unaffected regardless of ledger content; and `FromIaasLedger` is set correctly so the `callIaaSAllocate` gating in `pkg/ipam/allocate.go` behaves per FR-015.

### Tests for User Story 3

- [X] T022 ~~Ginkgo/Gomega test in `pkg/ippoolmanager/ippool_manager_test.go` for `AllocateIP` ledger gating: only `Ready` + unclaimed (absent from `status.allocatedIPs`) entries are selected; `NotReady`/`Releasing` entries are skipped; entries already present in `status.allocatedIPs` are skipped; malformed entries (neither `IPv4` nor `IPv6` set) are skipped without failing the whole pool; when multiple entries qualify, selection follows the existing ascending-address order convention; pools with empty/absent `IaasIPs` fall through unchanged to the pre-existing algorithm (byte-for-byte same behavior/tests as before this feature)~~ **SUPERSEDED by T022-R in Phase 5.1**
- [X] T023 ~~Ginkgo/Gomega test in `pkg/ippoolmanager/ippool_manager_test.go` for atomic pair allocation and single-stack behavior: a dual-stack request against a paired-pool ledger entry returns both addresses from the same entry (never mixed across entries); a single-stack request against the same paired pool returns only the requested family with no error, leaving the other family available; `AllocateIP`'s new return value is `fromIaasLedger == true` only when the address came from the ledger (same file as T022 — sequential with it, not parallel, despite both being [US3])~~ **SUPERSEDED by T023-R in Phase 5.1**
- [X] T024 [P] [US3] Ginkgo/Gomega test in `pkg/ipam/allocate_test.go` (or a new `pkg/ipam/iaas_ledger_test.go` following the existing `iaas_test.go` `Label("ipam", "unittest")` convention) for FR-015: given `i.config.IaaSClient != nil` and a mix of `types.AllocationResult` with `FromIaasLedger` true/false, `callIaaSAllocate` is invoked only with the non-ledger subset, and is skipped entirely when all results are ledger-sourced; behavior is unchanged (all results passed through) for results with `FromIaasLedger == false`

### Implementation for User Story 3

- [X] T025 ~~Implement ledger-aware selection in `AllocateIP` in `pkg/ippoolmanager/ippool_manager.go` (integrated into the existing flow at `ippool_manager.go:167-238`, reusing the already-computed `allocatedRecords`/`usedIPs` from `convert.UnmarshalIPPoolAllocatedIPs`): when `len(ipPool.Status.IaasIPs) > 0`, iterate entries in ascending-address order, skip non-`Ready` phases and entries whose address(es) are already in `usedIPs`/`allocatedRecords`, and pick the first qualifying entry; when `len(ipPool.Status.IaasIPs) == 0`, fall through unchanged to the existing `FindAvailableIPs`-based code path — depends on T005, T008, T009~~ **SUPERSEDED by T025-R in Phase 5.1**
- [X] T026 [US3] Implement atomic pair vs. single-family selection: for a matched Ready ledger entry present in both a v4 pool's and its paired v6 pool's `iaasReadyIPs` lists, when the allocation request is for both families (dual-stack Pod), commit both addresses to the respective pool objects' `status.allocatedIPs`-equivalent bookkeeping; when the request is single-family, commit only the requested family's address, leaving the sibling pool's address available — depends on T025-R
- [X] T027 [US3] Set the new `fromIaasLedger` return value to `true` exactly when T025-R/T026 selected the returned IP via the readiness intersection; keep it `false` for pools without the `iaas-pool` label — depends on T025-R, T008
- [X] T028 [US3] Implement FR-015 gating in `pkg/ipam/allocate.go` (`allocateInStandardMode`, at the existing `i.config.IaaSClient != nil` block around `allocate.go:439-444`): filter `results` to exclude entries with `FromIaasLedger == true` before calling `i.callIaaSAllocate(ctx, pod, filteredResults)`; skip the call entirely when the filtered list is empty — depends on T007, T008, T027
- [X] T029 ~~Implement the "no available ready IP" failure path: when no qualifying `IaasIPs` entry exists for a pool with `len(IaasIPs) > 0`, return the existing `constant.ErrIPUsedOut`-class error unchanged, so normal multi-pool candidate fallback in `pkg/ipam/allocate.go` continues to apply exactly as it does today (no new blocking/retry behavior introduced) — depends on T025~~ **SUPERSEDED by T029-R in Phase 5.1**
- [X] T030 [US3] Add/update the English documentation page (from T016/T020) describing `status.iaasReadyIPs`/`status.iaasFailedIPs` shape, the `iaas-pool`-label-gated readiness-intersection semantics, atomic pairing, single-stack behavior, and the FR-015 synchronous-call-skip behavior, under `docs/` — depends on T025-R, T026, T028
- [X] T031 [US3] Synchronize the `zh_CN` documentation counterpart for T030 in the same change — depends on T030

**Checkpoint**: All user stories are now independently functional — pools can be declared and paired (US1), Pods auto-resolve dual-stack pairs (US2), and allocation is gated/atomic per the per-IP ledger with the redundant synchronous provider call skipped (US3), per the revised design in Phase 5.1.

---

## Phase 5.1: Ledger schema & selection-algorithm revision (supersedes parts of Phase 2 & Phase 5)

**Goal**: Replace the single `IaasIPs []IaasIPAllocation` (with `Phase`/`LastError`) ledger with two lists — `IaasReadyIPs`/`IaasFailedIPs` — and change the allocation gating condition from "ledger non-empty" to "pool carries the `iaas-pool` label", with selection becoming an intersection of the normal `spec.ips`-derived candidate set and `IaasReadyIPs`, rather than a replacement of `spec.ips`-based selection. See `data-model.md` §1.3 and `research.md` §5/§6 (revised) for the full rationale.

**Independent Test**: Same as Phase 5's Independent Test above (this phase replaces its implementation, not its acceptance criteria).

> **Revision note**: This phase is itself superseded by the v5 design in **Phase 5.2** below (two-list ledger → `ipMetaData` map; `iaas-pool` → `iaas-provider`; `conditions` removed). Kept for history.

- [X] T005-R Replace `IaasIPAllocation`/`Phase` with `IaasReadyIPAllocation` (`IPv4 *string`, `IPv6 *string`, `MAC string`, `VLANID *int32`) and `IaasFailedIPAllocation` (`IPv4 *string`, `IPv6 *string`) in `pkg/k8s/apis/spiderpool.spidernet.io/v2beta1/spiderippool_types.go`; replace `IPPoolStatus.IaasIPs` with `IaasReadyIPs []IaasReadyIPAllocation` and `IaasFailedIPs []IaasFailedIPAllocation` (per data-model.md §1.3) — depends on T004
- [X] T006-R Run `make manifests generate-k8s-api` to regenerate `charts/spiderpool/crds/spiderpool.spidernet.io_spiderippools.yaml` and `zz_generated.deepcopy.go` for the T005-R schema change; verify the diff only touches the ledger fields (no unrelated regressions) — depends on T005-R
- [X] T009-R Rework ledger helper functions in `pkg/ippoolmanager/utils.go` (`IsValidIaasIPAllocation`, `IsIaasIPAllocationClaimed`, `FindReadyIaasIPAllocation`, `IsIaasLedgerAddress`, `PrimaryAddress`, `IaasIPAllocationAddressForVersion`) for the new two-list shape; remove any `Phase`-based branching (`IsIaasIPAllocationReady` is no longer needed since list membership IS readiness) — depends on T005-R
- [X] T022-R [US3] Rewrite the Ginkgo/Gomega tests in `pkg/ippoolmanager/ippool_manager_test.go` for the revised `AllocateIP` behavior: for an `iaas-pool`-labeled pool, only addresses present in both the normal `spec.ips`-derived candidate set AND `status.iaasReadyIPs` are selected; addresses in `status.iaasFailedIPs` (or simply absent from `iaasReadyIPs`) are skipped; malformed ledger entries (neither `IPv4` nor `IPv6` set) are skipped without failing the whole pool; when multiple candidates qualify, selection follows the existing ascending-address order convention; pools **without** the `iaas-pool` label fall through unchanged to the pre-existing algorithm regardless of `iaasReadyIPs`/`iaasFailedIPs` content (byte-for-byte same behavior/tests as before this feature); a freshly-created `iaas-pool` with empty `iaasReadyIPs` returns `ErrIPUsedOut`, not a static fallback allocation — depends on T009-R
- [X] T023-R [US3] Rewrite the atomic-pair-allocation test: a dual-stack request against a paired pool where both sibling pools' `iaasReadyIPs` contain the same address returns that address for both families; a single-stack request against the same paired pool returns only the requested family with no error, leaving the sibling family available; `AllocateIP`'s `fromIaasLedger` return value is `true` only when the address was selected via the readiness intersection — depends on T022-R
- [X] T025-R [US3] Implement the revised selection algorithm in `AllocateIP` in `pkg/ippoolmanager/ippool_manager.go`: if the pool carries the `iaas-pool` label, compute the normal `spec.ips`-derived candidate set exactly as today (unchanged `FindAvailableIPs` call over `excludeIPs`/`reservedIPs`/`usedIPs`), then intersect it with addresses present in `status.iaasReadyIPs` (ascending order preserved), selecting the first match and copying its `MAC`/`VLANID` onto the resulting `IPConfig`; if the pool does not carry the label, ledger fields are never consulted (unchanged pre-existing behavior) — depends on T005-R, T008, T009-R
- [X] T029-R [US3] Implement the "no available ready IP" failure path: when the intersection computed in T025-R is empty for an `iaas-pool`-labeled pool (including a freshly-created pool with empty `iaasReadyIPs`), return the existing `constant.ErrIPUsedOut`-class error unchanged, so normal multi-pool candidate fallback in `pkg/ipam/allocate.go` continues to apply exactly as it does today — depends on T025-R
- [X] T032-R Update `specs/006-iaas-prewarm-pool/quickstart.md`/`contracts/spiderippool-iaas-extension.md` example payloads to the new two-list shape — **done**: quickstart/contracts/plan/research/data-model/test-plan all updated in this documentation pass; verified no remaining stray `iaasIPs`/`phase`-only references outside historical/superseded notes — depends on T005-R
- [X] T033-R Remove the now-unused `IaasIPAllocationPhaseReady`/`NotReady`/`Releasing` constants from `pkg/constant/k8s.go` if they were added; confirm no remaining references — depends on T005-R

---

## Phase 5.2: v5 revision — cloud-neutral `ipMetaData`, vendor annotation, no conditions (supersedes parts of Phase 2, 5 & 5.1)

**Goal**: Align the implementation with proposal Draft v5 / spec Session 2026-08-11 clarifications: (1) replace the `IaasReadyIPs`/`IaasFailedIPs` list ledgers and `Conditions` with a single cloud-neutral `status.ipMetaData` structure (`parentNic` + `metadata` map keyed by primary-family address, values carrying `ipv6`/`mac`/`vlan`, plus provider-written `readyIPCount`/`unreadyIPCount` counters); (2) replace the `ipam.spidernet.io/iaas-pool: "true"` annotation/label with `ipam.spidernet.io/iaas-provider: "<vendor>"` (supported vendor whitelist: `huaweicloud`); (3) keep the user-facing iaas-network-provider docs high-level (no prewarm internals). See `data-model.md` §1.3, `research.md` §1/§5/§6/§9, and `contracts/spiderippool-iaas-extension.md` (all revised) for the full design.

**Independent Test**: Same acceptance criteria as Phase 5/5.1 (readiness intersection, atomic pairing, non-IaaS pools unaffected), re-expressed against the new shape: populate `status.ipMetaData.metadata` on an `iaas-provider`-labeled pool and verify only metadata-keyed, unclaimed addresses are selected; verify `mac`/`vlan` propagation from the map entry; verify pools without the label ignore `ipMetaData` entirely.

- [X] T039 Replace constants in `pkg/constant/k8s.go`: `AnnoIPPoolIaas`/`LabelIPPoolIaas` (`/iaas-pool`) → `AnnoIPPoolIaasProvider`/`LabelIPPoolIaasProvider` (`/iaas-provider`); add a supported-vendor list (currently `huaweicloud`)
- [X] T040 Replace `IaasReadyIPAllocation`/`IaasFailedIPAllocation` and `IPPoolStatus.IaasReadyIPs`/`IaasFailedIPs`/`Conditions` with `IPMetaData` (`ParentNic string`, `Metadata map[string]IPMetadataEntry`, `ReadyIPCount *int64`, `UnreadyIPCount *int64`) and `IPMetadataEntry` (`IPv6 *string`, `MAC string`, `VLAN *int32`) in `pkg/k8s/apis/spiderpool.spidernet.io/v2beta1/spiderippool_types.go` (per data-model.md §1.3); drop the now-unused `metav1.Condition` import — depends on T039
- [X] T041 Run `make manifests generate-k8s-api` to regenerate the CRD YAML and `zz_generated.deepcopy.go` for the T040 schema change; verify the diff only touches the ipMetaData/conditions fields — depends on T040
- [X] T042 Update the mutating webhook label sync in `pkg/ippoolmanager/ippool_mutate.go` (+ `ippool_mutate_test.go`) for the `iaas-provider` annotation (value-mirroring: label value = annotation value, i.e. the vendor name) — depends on T039
- [X] T043 Add validating-webhook vendor-whitelist check in `pkg/ippoolmanager/ippool_validate.go` (+ tests): reject an `iaas-provider` annotation whose value is not a supported vendor (`huaweicloud`) — depends on T039
- [X] T044 Rework metadata helpers in `pkg/ippoolmanager/utils.go` (`IsIaaSPool`, `IsValidIaasIPAllocation`, `FindReadyIaasIPAllocation`, `IsIaasLedgerAddress`, etc.) for the map shape: candidate-set intersection against `Metadata` keys (O(1) map lookups), pair `ipv6` lookup from the entry value, malformed-key skip; update `ippool_manager.go`'s `genRandomIP`/`AllocateIP` (including the pair-pool "borrow primary pool's metadata" path) and copy `MAC`/`VLAN` from the map entry onto `IPConfig` — depends on T040
- [X] T045 Rewrite the affected Ginkgo tests in `pkg/ippoolmanager/ippool_manager_test.go` (and any `pkg/ipam` tests referencing the old fields) for the new shape: presence-in-metadata gating, absence = skipped, malformed keys skipped, non-labeled pools byte-for-byte unchanged, empty-metadata pool returns `ErrIPUsedOut` — depends on T044
- [X] T046 Update `docs/usage/iaas-network-provider.md` + `iaas-network-provider-zh_CN.md`: remove prewarm/ledger implementation detail (`iaasReadyIPs` shape, readiness-intersection mechanics); keep user-facing content high-level; note that a node-pool-oriented feature doc may be added separately — depends on T044
- [X] T047 Run `make gofmt`, `make lint-golang`, `make unittest-tests` on the full v5 changeset — gofmt/`go vet` clean, full `make unittest-tests` green (90/90 specs, 27 suites); `golangci-lint` step blocked by a pre-existing local v1-binary-vs-v2-config mismatch unrelated to this change — depends on T041, T042, T043, T044, T045

---

## Phase 5.3: v6 revision — JSON metadata cache and generation-safe publication

**Goal**: Reduce large-pool status processing overhead without adding stale
metadata races. Store the logical metadata map as a JSON string, decode it
once per authoritative revision into an immutable agent cache, and gate
allocation on provider-published `observedGeneration`.

**Independent Test**: For 64- and 1000-entry pools, verify metadata is parsed
once per changed revision (not once per Pod), unrelated `allocatedIPs` updates
reuse the snapshot, spec generation mismatch rejects allocation, and an atomic
provider status publication with matching observed generation restores
allocation. Corrupt JSON and cache-version mismatch must fail closed.

- [X] T048 Change `IPMetaData.Metadata` from structural map to JSON string and add `ObservedGeneration *int64` in the source API type; define the decoded map as an internal Go type
- [X] T049 Regenerate deepcopy and SpiderIPPool CRD schema with `make manifests generate-k8s-api`; document/perform migration or clearing of draft v5 structural metadata before applying the new schema — depends on T048
- [X] T050 Add an agent-side immutable metadata snapshot cache keyed by pool UID + observed generation, with content equality/hash reuse so unrelated resourceVersion changes do not reparse metadata — depends on T048
- [X] T051 Integrate the cache with IPPool informer Add/Update/Delete handling, including pure status Updates, startup replay, atomic replacement, deletion eviction, malformed-JSON failure state, and fail-closed cache miss/version mismatch — depends on T050
- [X] T052 Gate IaaS allocation on `status.ipMetaData.observedGeneration == metadata.generation`; return a retryable metadata-not-reconciled error before candidate selection when mismatched — depends on T048, T051
- [X] T053 Update allocation helpers/tests to consume decoded snapshots rather than the CRD field directly; retain candidate intersection, pair lookup, MAC/VLAN propagation, and non-IaaS fast path — depends on T051
- [X] T054 Update provider contract/quickstart and coordinate provider implementation so metadata, counters, and observedGeneration are published atomically after a complete trustworthy reconcile pass; partial per-IP failures are valid published outcomes — depends on T048
- [X] T055 Add benchmarks for 64/1000 entries covering outer marshal/unmarshal, DeepCopy, provider build, allocation with cache, and allocation without cache; enforce that no per-Pod metadata unmarshal is introduced — depends on T050
- [X] T056 Run targeted unit tests, generation, gofmt, build, and full relevant test suite; redeploy CRD/agent/controller and repeat spec-update concurrency and batch Pod restart tests — depends on T049, T052, T053, T054, T055

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Validation, documentation completeness, and quality-gate confirmation across all three stories

- [X] T032 [P] Run `make gofmt` and `make lint-golang` on the full changeset and fix any findings
- [X] T033 [P] Run `make unittest-tests` for the full changeset and confirm all new and pre-existing tests in `pkg/ippoolmanager` and `pkg/ipam` pass, including the Ginkgo `Label(...)` requirement check
- [X] T034 Run `make manifests generate-k8s-api` again after all type changes are finalized and verify `git diff` shows no unexpected manual drift in `charts/spiderpool/crds/spiderpool.spidernet.io_spiderippools.yaml` or `zz_generated.deepcopy.go`
- [X] T035 Manually execute `specs/006-iaas-prewarm-pool/quickstart.md` end-to-end against a dev cluster (simulating provider-written `status.iaasReadyIPs`/`status.iaasFailedIPs` via `kubectl patch --subresource=status`) and record results/deviations
- [X] T036 Validate the performance budget stated in `plan.md`: confirm (via code review and/or a targeted benchmark/log check) that pools without the `iaas-pool` label incur zero additional Kubernetes API calls or measurable latency change in `AllocateIP` and `getPoolCandidates`
- [X] T037 [P] Cross-check `specs/006-iaas-prewarm-pool/spec.md` Success Criteria (SC-001..SC-006) against the implemented behavior and note any gaps
- [X] T038 Review and finalize both English and `zh_CN` documentation pages added across T016/T017/T020/T021/T030/T031 for consistency of terminology (annotation/label/status field names) per spec FR-007/SC-006

---

## Phase 7: User Story 4 - Global pool mode: realtime allocation + sticky sub-ENI cache (Priority: P2)

**Goal**: Implement the Spiderpool-side half of `global-pool-design.md` (spec
US4 / FR-018–FR-025): decode metadata schema v2 (`{scope, parentNic, ips}`,
legacy shape accepted), recognize global pools (IaaS annotation + no
`spec.nodeName`), add the node-filtered cache-hit predicate, the cold-path
candidate ordering (unbound first, then idle-on-another-node), the
claim-then-RPC flow with claim rollback via the existing
`pkg/iaas/client` HTTP client, `detaching`-entry skipping, and the v6
metadata-reference exclusion for dynamic-sticky pairing. Everything
provider-side (schema v2 writing, watermark reclaim, flush discipline,
memory-authoritative RPC server) is external and out of scope.

**Independent Test**: Populate a global pool's metadata (schema v2,
`scope: ""`, entries with/without `node`, one `detaching`) in unit tests and
run `AllocateIP`/`AllocateIPPair` + the `pkg/ipam` allocation flow with a
mocked IaaS client: verify zero-RPC cache hits, cold-path ordering, claim
rollback on RPC failure, DEL touching only `allocatedIPs`, and byte-for-byte
unchanged node-level (US3) behavior.

**Prerequisite**: Phases 2–5.3 complete (metadata cache, `ipMetaData` v6
schema, pair machinery, `pkg/iaas/client`).

### Foundational for User Story 4

- [ ] T057 [US4] Upgrade the decoded metadata type and parser in `pkg/ippoolmanager/metadata_cache.go` to schema v2: decode `{"scope": "<nodeName>"|"", "parentNic": "<nic>", "ips": {addr: {ipv6, mac, vlan[, node][, detaching]}}}` into an internal struct carrying `Scope *string`, `ParentNic string`, and per-entry `Node`/`Detaching`; keep accepting the legacy flat shape (top-level address keys + reserved `parentNic` key) as a node-level pool; treat missing metadata or missing `scope` as not-yet-reconciled (fail closed, existing retryable error); validate node-level invariants (non-empty `scope` must equal `spec.nodeName`; per-entry `node` must not appear) and fail closed on violation (FR-018)
- [ ] T058 [P] [US4] Ginkgo/Gomega tests in `pkg/ippoolmanager/metadata_cache_test.go` for T057: v2 node-level decode, v2 global decode (entries with/without `node`, with `detaching`), legacy flat-shape decode, missing/empty `scope` handling, malformed JSON fail-closed, snapshot reuse semantics unchanged
- [ ] T059 [US4] Add global-pool recognition + placement helpers in `pkg/ippoolmanager/utils.go`: `IsGlobalIaaSPool(pool)` (IaaS annotation present AND `spec.nodeName` empty AND decoded `scope == ""`), `effectiveNode(snapshot, ip)` (`scope != "" ? scope : ips[ip].node`), and extend `FindReadyIPMetadata`/`FindReadyIPPairMetadata` with a `localNode` filter for global pools plus unconditional skipping of `detaching` entries in hit and candidate sets (FR-019/FR-020/FR-023)

### Tests for User Story 4

- [ ] T060 [P] [US4] Ginkgo/Gomega tests in `pkg/ippoolmanager/ippool_manager_test.go` for the global-pool hit path: entry `node == localNode` and unclaimed → selected with cached `{ipv6, mac, vlan}` and `fromIaasLedger == true`; entry on another node → not a hit; `detaching` entry → skipped; node-level pools unaffected regardless of per-entry `node` content (byte-for-byte US3 behavior)
- [ ] T061 [P] [US4] Ginkgo/Gomega tests for cold-path candidate ordering and pairing: unbound addresses (no entry, or entry without `node`) are preferred over idle-on-another-node addresses; v6 candidate set excludes every address referenced by an existing metadata `entry.ipv6` even when unclaimed; dual-stack cold path selects one free v4 + one free v6 and commits pair-or-nothing via the existing `AllocateIPPair` machinery (FR-021/FR-024)
- [ ] T062 [P] [US4] Ginkgo/Gomega tests in `pkg/ipam` (mocked `IaaSClient`) for the claim-then-RPC flow: RPC called only on cache miss (never on hit); RPC success configures the result from the response without waiting for metadata persistence; RPC failure rolls back the `allocatedIPs` claim and fails the ADD with a retryable error; CNI DEL performs no provider call for global-pool IPs (FR-021/FR-022)

### Implementation for User Story 4

- [ ] T063 [US4] Implement the hit-path and cold-path selection in `pkg/ippoolmanager/ippool_manager.go` (`AllocateIP`/`AllocateIPPair`/`genRandomIP`): for global pools, first evaluate the hit predicate over the snapshot; on miss, order candidates unbound-first then idle-on-another-node within the ordinary availability set (`spec.ips` ∖ `excludeIPs` ∖ reserved ∖ `allocatedIPs`); surface whether the selected IP was a hit (cached entry) or a miss (RPC required) to the caller — depends on T057, T059
- [ ] T064 [US4] Implement the synchronous cache-miss RPC in `pkg/ipam/allocate.go`: after the `allocatedIPs` claim commit, for global-pool miss results call the existing `pkg/iaas/client` `AllocateIPs` (request carries node name + pod ref per the current contract) and populate the result's `{ipv6, mac, vlan}` from the response; on RPC error, roll back the claim (existing release bookkeeping) and fail the ADD; keep FR-015 skip behavior for hit-sourced IPs; map provider typed errors (`CapacityExceeded`, `CloudThrottled`, ...) to retryable failures — depends on T063
- [ ] T065 [US4] Verify/adjust the CNI DEL and GC paths so global-pool IPs never trigger a provider release call (cache stickiness): extend the existing prewarm-pool IaaS-release skip (see `cbe82ab35`) to cover global pools — depends on T057, T059
- [ ] T066 [US4] Update `specs/006-iaas-prewarm-pool/quickstart.md` with a global-pool section (schema v2 example payloads with `scope: ""` + per-entry `node`, hit/miss walkthrough via `kubectl patch --subresource=status`) — depends on T057
- [ ] T067 [US4] Update English docs (`docs/usage/iaas-network-provider.md`) with a high-level global pool mode section (1 deploy : 1 pool model, sticky cache, no prewarming, watermark reclaim is provider-side) — keep internals out per the docs guidance — depends on T063, T064
- [ ] T068 [US4] Synchronize the `zh_CN` documentation counterpart for T067 in the same change — depends on T067

### Polish for User Story 4

- [ ] T069 [US4] Run `make gofmt`, `make lint-golang`, and `make unittest-tests` on the full US4 changeset; confirm all pre-existing US1–US3 tests pass unmodified (SC-011) and no CRD/deepcopy regeneration is needed (schema v2 lives inside the JSON string — `make manifests generate-k8s-api` diff must be empty)
- [ ] T070 [US4] Cross-check spec SC-009..SC-012 against the implemented behavior (zero-RPC hit, rollback on RPC failure, node-level regression, pair stickiness) and record results in `test-plan.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories (CRD types must exist and compile before any webhook/IPAM code referencing them can be written)
- **User Stories (Phase 3-5)**: All depend on Foundational phase completion
  - US1 (Phase 3) has no dependency on US2/US3 and can be fully completed and tested alone
  - US2 (Phase 4) depends only on Foundational (the `AnnoIPPoolPairPool` constant); it does NOT depend on US1's validation code, though in practice pairing validation (US1) should land first to avoid auto-completing invalid pairs in a real cluster — independently testable via unit tests regardless
  - US3 (Phase 5) depends only on Foundational (CRD status types, `AllocationResult.FromIaasLedger`, `AllocateIP` signature change); it does NOT depend on US2's auto-completion logic (an explicit dual-stack Pod request exercises the same ledger-gating code)
- **Polish (Phase 6)**: Depends on all three user stories being complete
- **US4 / Global pool mode (Phase 7)**: Depends on Phases 2–5.3 and 6 being complete (metadata cache, `ipMetaData` v6 schema, pair machinery, `pkg/iaas/client`); node-level pool behavior must remain byte-for-byte unchanged (SC-011). Within Phase 7: T057→T058/T059 → tests T060–T062 [P] → implementation T063→T064, T065 → docs T066–T068 → polish T069–T070

### Within Each User Story

- Tests MUST be written before/alongside implementation and confirmed to fail first for the new behavior
- Webhook mutate/validate changes (US1) before docs referencing them
- CRD/type/constant plumbing (Foundational) before any manager/webhook/IPAM code that references the new fields
- Story complete (implementation + tests + docs) before moving to the next priority in a sequential (non-parallel-team) execution

### Parallel Opportunities

- T001-T003 (Setup) can run in parallel
- T007, T009, T010 (Foundational, different files) can run in parallel; T004-T006, T008 have a strict order (constants -> types -> generation -> interface change) and must not run in parallel with each other
- Once Foundational (Phase 2) completes, US1, US2, and US3 implementation can proceed in parallel by different developers since they touch disjoint files (`ippool_mutate.go`/`ippool_validate.go` vs. `pool_selections.go` vs. `ippool_manager.go`/`allocate.go`), with the caveat noted above that US1's validation should land first in practice to avoid validating against not-yet-correct pairing rules in a live cluster
- All test tasks marked [P] within a story can run in parallel (different test files)
- T032, T033, T037 in Polish can run in parallel

---

## Parallel Example: User Story 1

```bash
# Launch both new test files for User Story 1 together:
Task: "Ginkgo/Gomega test for iaas-pool label sync in pkg/ippoolmanager/ippool_mutate_test.go"
Task: "Ginkgo/Gomega test for pairing validation rules in pkg/ippoolmanager/ippool_validate_test.go"
```

## Parallel Example: User Story 3

```bash
# Launch the independent (different-file) test task for User Story 3 in parallel with T022/T023
# (T022 and T023 share pkg/ippoolmanager/ippool_manager_test.go and run sequentially):
Task: "Ginkgo/Gomega test for AllocateIP ledger gating in pkg/ippoolmanager/ippool_manager_test.go (T022)"
Task: "Ginkgo/Gomega test for FR-015 synchronous-call skip in pkg/ipam/iaas_ledger_test.go (T024, parallel with T022/T023)"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: User Story 1 (pool declaration + pairing validation)
4. **STOP and VALIDATE**: Confirm pools can be declared/paired/validated via unit tests and manual `kubectl apply`, with zero IPAM behavior change yet
5. Deploy/demo if ready — this alone lets the (future/external) provider component start reconciling against a stable, validated pool contract

### Incremental Delivery

1. Complete Setup + Foundational -> Foundation ready (CRD types, constants, plumbing)
2. Add User Story 1 -> Test independently -> Deploy/Demo (MVP!)
3. Add User Story 2 -> Test independently -> Deploy/Demo (Pods can now omit the paired pool name)
4. Add User Story 3 -> Test independently -> Deploy/Demo (allocation actually gates on real ledger readiness and skips the redundant sync provider call)
5. Add User Story 4 -> Test independently -> Deploy/Demo (global pools: realtime first-use allocation + sticky sub-ENI cache reuse; requires the external provider's schema v2 / Allocate RPC / watermark reclaim counterpart)
6. Each story adds value without breaking previous stories — pools without the new annotations/status are unaffected at every step (FR-006/FR-011), and node-level prewarm pools are unaffected by US4 (FR-019/SC-011)

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together (single PR, since it's a small, tightly-coupled set of type/constant/plumbing changes)
2. Once Foundational is done:
   - Developer A: User Story 1 (`pkg/ippoolmanager/ippool_mutate.go`, `ippool_validate.go`, `ippool_webhook.go`)
   - Developer B: User Story 2 (`pkg/ipam/pool_selections.go`)
   - Developer C: User Story 3 (`pkg/ippoolmanager/ippool_manager.go`, `pkg/ipam/allocate.go`)
3. Stories complete and integrate independently; recommend landing US1 slightly ahead of US2/US3 in the merge order (not a hard code dependency, but avoids validating pairing rules against pools that IPAM is already treating as auto-completed/gated)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable per its own Ginkgo test file(s)
- Verify new tests fail before implementing (TDD per Constitution Principle II)
- Commit after each task or logical group
- Stop at any checkpoint to validate a story independently
- FR-015 (skipping the pre-existing synchronous `callIaaSAllocate`) was discovered during planning by reading `pkg/ipam/allocate.go:439` and `pkg/ipam/iaas.go`, and confirmed with the user before being added to spec.md/plan.md/research.md; it is folded into User Story 3 (T024, T028) since it is the natural conclusion of the ledger-gated allocation path
- Avoid: vague tasks, same-file conflicts, cross-story dependencies that break independence
