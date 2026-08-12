# Implementation Plan: IaaS Provider Prewarm IP Pool Support

**Branch**: `006-iaas-prewarm-pool` | **Date**: 2026-08-05 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/006-iaas-prewarm-pool/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Add Spiderpool (open-source)-side support for IaaS-backed "prewarm" `SpiderIPPool`s
as described in the provider proposal (Draft v5, P0 scope only). No new
CRD is introduced. Concretely:

1. Two new well-known annotations on `SpiderIPPool` — `ipam.spidernet.io/iaas-provider`
   (marks a pool as IaaS-managed by a named vendor; supported list is currently
   `huaweicloud`) and `ipam.spidernet.io/pair-pool` (names its
   dual-stack sibling pool) — plus a synchronized label mirroring the first
   annotation (mutating webhook, following the existing `LabelIPPoolCIDR` sync
   precedent in `pkg/ippoolmanager/ippool_mutate.go`).
2. Validating webhook rules: supported-vendor whitelist for `iaas-provider`,
   plus pairing correctness — no self-reference, no
   same-IP-version pairing, v4-pool static capacity <= v6-pool static capacity
   when both pools already exist, and identical `nodeName`/`podAffinity` between
   paired pools.
3. A new cloud-neutral metadata structure on `SpiderIPPool` —
   `status.ipMetaData`, containing a pool-level `parentNic`, a `metadata`
   JSON string (decoded shape: primary-family address → paired `ipv6`, `mac`,
   `vlan`), provider-owned `observedGeneration`, and two observational
   counters (`readyIPCount`/`unreadyIPCount`). The external provider atomically
   publishes all four values after reconciling a pool generation. Presence of
   a decoded metadata entry IS per-IP readiness; there is no phase, failed-IP
   list, or `status.conditions`.
4. Two IPAM behavior changes: (a) automatic dual-stack pool completion during
   Pod pool-candidate resolution when a selected pool carries `pair-pool` and the
   opposite family wasn't explicitly requested (`pkg/ipam/pool_selections.go`);
   (b) per-IP metadata-aware, atomic pair-or-single allocation in
   `pkg/ippoolmanager/ippool_manager.go`'s `AllocateIP`, gated strictly behind the
   `iaas-provider` label so pools without it are entirely unaffected.

Technical approach: extend `SpiderIPPoolStatus`/`SpiderIPPoolSpec`-adjacent Go
types, regenerate CRD manifests/deepcopy, add mutating/validating webhook logic,
and add narrowly-scoped feature-gated IPAM paths. For v6, add informer-driven
metadata cache construction and generation gating. The draft v5 structural
metadata field must be cleared/migrated before applying the v6 string schema.

## Technical Context

**Language/Version**: Go 1.25 (per `go.mod`)

**Primary Dependencies**: `sigs.k8s.io/controller-runtime` (webhooks/manager),
`k8s.io/apimachinery`, existing
`pkg/ip` helpers (`FindAvailableIPs`, `IPsDiffSet`, `AssembleTotalIPs`), existing
`pkg/ippoolmanager` / `pkg/ipam` packages. (No `metav1.Condition` usage — this
feature deliberately adds no `status.conditions` field.)

**Storage**: Kubernetes CRD status/spec fields on the existing `SpiderIPPool`
resource (etcd via kube-apiserver); no new storage system.

**Testing**: Ginkgo v2 + Gomega unit tests under `pkg/ippoolmanager` and
`pkg/ipam` (matching `*_test.go` + `*_suite_test.go` + `Label(...)` convention,
e.g. `pkg/ippoolmanager/ippool_manager_suite_test.go`), run via
`make unittest-tests`. No new e2e suite is introduced by this plan (out of P0
scope per spec Assumptions); existing e2e for `SpiderIPPool` allocation must
continue to pass unmodified.

**Target Platform**: Linux Kubernetes clusters (Spiderpool agent/controller
pods), same as existing Spiderpool deployment target.

**Project Type**: Existing Kubernetes controller/webhook + CNI IPAM library
(single Go module, multi-package repo) — no new project/service is created.

**Performance Goals**: Zero added Kubernetes API calls and zero added
allocation-path latency for any `SpiderIPPool` that does NOT carry the new
`iaas-provider` label (gating is decided solely by the label, independent of
whether `ipMetaData` is populated; must be provably a
no-op branch skip). For IaaS pools, agent informer processing parses each
distinct metadata JSON revision once and publishes an immutable decoded map
snapshot; Pod allocation performs no JSON unmarshal and no extra API round
trip. A 64/1000-entry local benchmark showed that JSON-string storage with a
parsed cache reduced the simulated 1000-entry allocation cycle from ~2.11 ms
to ~0.54 ms, while JSON string without caching regressed it to ~5.29 ms.
Therefore the cache is part of the design, not an optional follow-up.

**Constraints**: No new CRD (reuse `SpiderIPPool`); no cloud API calls anywhere
in the Spiderpool codebase for this feature (the provider component is
external/private and out of scope); backward compatible CRD status/spec
additions only (additive fields, optional/pointer/omitempty so existing
manifests and older controllers remain valid); webhook validation additions
must not reject any pre-existing pool that lacks the new annotations. The one
exception is the unmerged/development-only v5 `ipMetaData.metadata` map →
string representation change; test-cluster draft data requires an explicit
clear/migration step before CRD rollout.

**Scale/Scope**: Covers both the proposal POC scale (~64 IPs) and large pools
up to at least 1000 metadata entries for serialization/cache validation.
Existing `MaxAllocatedIPs` and pool-size conventions remain unchanged.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Code quality and API compatibility**: Touches `pkg/k8s/apis/spiderpool.spidernet.io/v2beta1`
  (SpiderIPPool spec/status types + deepcopy), `pkg/ippoolmanager` (mutate,
  validate, manager/AllocateIP), `pkg/ipam` (pool_selections.go,
  allocate.go/selectByPod interaction), `charts/spiderpool/crds/spiderpool.spidernet.io_spiderippools.yaml`.
  Production-facing changes are additive and optional. The v6 revision changes
  the representation of the feature-branch-only draft metadata field and
  therefore requires migration/clearing of v5 test data. Pools without the new
  annotations/status keep their exact current behavior (spec FR-006, FR-011).
  Compatible with existing Helm values (no new values planned unless a
  provider-enablement toggle is later needed — none required for spiderpool-side
  P0 since gating is purely by annotation/status presence).
- **Testing standard**: New Ginkgo/Gomega unit tests required for: (1) mutating
  webhook label sync for `iaas-provider` annotation; (2) validating webhook
  vendor-whitelist and pairing
  rules (self-reference, same-version, capacity <=, nodeName/podAffinity match,
  not-yet-existing pair allowed); (3) `pool_selections.go` auto-completion of a
  paired pool; (4) `AllocateIP` per-IP metadata gating (generation match,
  parsed-cache availability, presence-in-metadata
  filtering, occupancy via `status.allocatedIPs`, atomic pair selection,
  single-family-from-paired-pool allocation, existing-order entry selection,
  and pass-through/no-op behavior for pools without the IaaS label). No e2e
  addition planned for P0; this is recorded as an explicit scope decision (not
  an untested-change exception) since the external provider component required
  to populate real metadata end-to-end is out of this repository.
- **User/operator consistency**: New annotation names
  (`ipam.spidernet.io/iaas-provider`, `ipam.spidernet.io/pair-pool`), synchronized
  label (`ipam.spidernet.io/iaas-provider`), and the new status field
  (`ipMetaData`, including `observedGeneration`) MUST follow the existing
  `ipam.spidernet.io/...` naming convention (`pkg/constant/k8s.go`) and
  existing validation error style (`field.Invalid`/`field.Forbidden` +
  `apierrors.NewInvalid`, per `pkg/ippoolmanager/ippool_validate.go` /
  `ippool_webhook.go`). Docs
  (English + Chinese, per repo convention) must be added/updated describing
  the new annotations, status shape, and pairing semantics.
- **Performance budget**: Explicit budget stated above (zero added API calls or
  latency for non-IaaS pools; decode once per metadata revision, O(1) direct
  key lookup where applicable, no per-Pod metadata unmarshal or extra API calls
  for IaaS pools). This is the IPAM allocation hot path (`AllocateIP`,
  `getPoolCandidates`), so this budget is mandatory per Constitution Principle IV.
- **Generated artifacts**: `SpiderIPPoolSpec`/`SpiderIPPoolStatus` Go type
  changes require `make manifests generate-k8s-api` (regenerates CRD YAML under
  `charts/spiderpool/crds/` and `zz_generated.deepcopy.go`) before the change is
  complete. No OpenAPI (`api/v1/*/openapi.yaml`) changes are anticipated since
  this feature does not touch the spiderpool-agent/controller HTTP API surface,
  but this must be re-confirmed once field additions are finalized in Phase 1.

**Gate Result**: PASS — non-IaaS and pre-feature surfaces remain
additive/backward-compatible; the isolated draft-v5 migration is documented,
testing plan matches the risk level (webhook + IPAM hot path), performance
budget is explicit and required by Principle IV, and the generation target is
identified up front. No complexity exceptions required.

## Project Structure

### Documentation (this feature)

```text
specs/006-iaas-prewarm-pool/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command) — CRD field contract
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
pkg/k8s/apis/spiderpool.spidernet.io/v2beta1/
├── spiderippool_types.go       # + IPMetaData status field (parentNic, metadata JSON string, observedGeneration, ready/unready counters); no Conditions
└── zz_generated.deepcopy.go    # regenerated via `make generate-k8s-api`

pkg/constant/
└── k8s.go                      # + AnnoIPPoolIaasProvider, AnnoIPPoolPairPool, LabelIPPoolIaasProvider constants + supported-vendor list (huaweicloud)

pkg/ippoolmanager/
├── ippool_mutate.go             # + iaas-provider annotation -> label sync (mirrors LabelIPPoolCIDR pattern)
├── ippool_validate.go           # + vendor whitelist + pairing validation rules (self-ref, version, capacity, nodeName/podAffinity match)
├── ippool_manager.go            # + AllocateIP per-IP metadata gating + atomic pair selection; interface signature gains a `fromIaasLedger bool` return value (FR-015)
├── metadata_cache.go            # + immutable parsed metadata snapshots keyed by pool UID + observedGeneration
└── utils.go                     # + decoded metadata helper(s): ready/unclaimed entry lookup, pair lookup

pkg/ipam/
├── pool_selections.go            # + auto-complete paired pool into candidate list
├── allocate.go                    # + skip callIaaSAllocate for metadata-sourced IPs (FR-015); selectByPod untouched (nodeName/podAffinity AND semantics already sufficient per spec)
└── iaas.go                        # callIaaSAllocate call-site gating only; no change to its cloud API logic itself

pkg/types/
└── ip.go                          # + AllocationResult.FromIaasLedger bool (propagates metadata-origin flag from AllocateIP to the FR-015 gating check)

charts/spiderpool/crds/
└── spiderpool.spidernet.io_spiderippools.yaml   # regenerated CRD schema (status.ipMetaData)

docs/
├── usage/ (or concepts/) new/updated page(s) describing iaas-provider/pair-pool annotations (English)
└── zh_CN equivalent page(s), synchronized per repo convention

# Docs guidance: the user-facing iaas-network-provider docs MUST stay
# high-level and MUST NOT expose prewarm/ipMetaData internals; a separate
# node-pool-oriented feature doc may be added later.

specs/006-iaas-prewarm-pool/    # feature plan, research, contracts, and tasks
```

**Structure Decision**: No new top-level package or binary. All changes land in
existing packages (`pkg/k8s/apis/.../v2beta1`, `pkg/constant`,
`pkg/ippoolmanager`, `pkg/ipam`) plus generated CRD/deepcopy artifacts and docs,
matching the proposal's explicit design principle of reusing `SpiderIPPool` with
zero new CRDs and confining spiderpool-side changes to "two points" (pool
resolution auto-completion, and per-IP-metadata-aware atomic allocation).

## Complexity Tracking

> No Constitution Check violations identified; this section is intentionally
> left without entries.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | — | — |
