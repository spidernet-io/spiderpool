# Implementation Plan: VLAN CNI Support and Namespace-Scoped Tenant Injection

**Branch**: `001-vlan-cni` | **Date**: 2026-04-21 | **Spec**: `/Users/cyclinder/Desktop/code/spiderpool/specs/001-vlan-cni/spec.md`
**Input**: Feature specification from `/Users/cyclinder/Desktop/code/spiderpool/specs/001-vlan-cni/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Extend Spiderpool in two connected areas: add native VLAN CNI support across CRD, webhook, informer, and docs; and enhance webhook-based auto injection so `cni.spidernet.io/network-resource-inject` can be resolved hierarchically for multi-tenant VLAN scenarios. The effective injection value is resolved by checking the Pod first and falling back to the Namespace only when the Pod does not define the annotation. Matching `SpiderMultusConfig` objects are then sorted deterministically before constructing the Multus annotation. Namespace lookups should use the existing namespace manager with cached reads to avoid per-request APIServer traffic.

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: Go, Kubernetes controller/webhook stack  
**Primary Dependencies**: controller-runtime manager/webhook machinery, Kubernetes client-go, Spiderpool CRDs, existing `podmanager` and `namespacemanager`, Multus CNI integration  
**Storage**: Kubernetes CRDs plus generated `NetworkAttachmentDefinition` resources; no new persistent storage  
**Testing**: Go unit tests under `pkg/**`, existing webhook and manager tests, and targeted integration/e2e coverage under repository test directories  
**Target Platform**: Linux Kubernetes clusters running Spiderpool, Multus, and optional RDMA-capable secondary CNIs
**Project Type**: Kubernetes networking controller/webhook project  
**Performance Goals**: Preserve current admission behavior while keeping auto-injection deterministic and avoiding direct per-request Namespace API reads  
**Constraints**: Must remain backward compatible, must not change the set of injected networks, must preserve explicit Pod override semantics, and should use cached Namespace reads through existing abstractions  
**Scale/Scope**: Changes affect `SpiderMultusConfig` schema/validation/translation plus pod webhook mutation behavior for VLAN and tenant-scoped network injection flows in the existing repository

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- No constitution file was found under `.specify/memory/constitution.md`, so repository-specific constitutional gates were unavailable.
- Default gates applied for this plan:
  - Preserve backward compatibility for existing CNI types and current Pod annotation behavior.
  - Prefer extending existing manager abstractions (`podmanager`, `namespacemanager`) over introducing new controllers or APIs.
  - Keep Namespace fallback semantics explicit and deterministic: `Pod` first, `Namespace` second.
  - Keep user-facing documentation synchronized with webhook behavior changes.

**Gate Result**: PASS. The design extends existing typed flows and manager abstractions without adding new APIs or long-lived state.

## Project Structure

### Documentation (this feature)

```text
specs/001-vlan-cni/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
└── tasks.md
```

### Source Code (repository root)

```text
cmd/
├── spiderpool-controller/cmd/
└── spiderpool-agent/cmd/

pkg/
├── constant/
├── k8s/apis/spiderpool.spidernet.io/v2beta1/
├── multuscniconfig/
├── namespacemanager/
└── podmanager/

docs/
└── usage/

test/
└── e2e/
```

**Structure Decision**: Use the existing single-repository Kubernetes controller layout. VLAN support is centered in CRD/constants/webhook/informer code under `pkg/k8s/apis/...` and `pkg/multuscniconfig/`. Namespace-scoped tenant default injection and precedence handling are centered in `pkg/podmanager/` and rely on `pkg/namespacemanager/`. No contracts artifact is required because the feature changes internal controller/webhook behavior rather than exposing a new public API.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|

