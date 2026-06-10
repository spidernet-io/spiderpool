# Implementation Plan: Agent ENI Device Plugin

**Branch**: `005-agent-eni-device-plugin` | **Date**: 2026-06-09 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/004-agent-eni-device-plugin/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Add an optional Spiderpool agent device plugin for provider-mode auxiliary ENI scheduling. The plugin advertises `spidernet.io/eni-slot` as a Kubernetes extended resource where the advertised value is the node's current healthy schedulable total slot capacity, not a free-slot counter. Eligible Pods must request the resource so Kubernetes scheduler accounting prevents placement on exhausted nodes. The agent integrates with existing IaaS provider allocation/release flows, Helm configuration, Pod resource injection patterns, kubelet plugin path selection, node observability, and restart reconciliation.

## Technical Context

**Language/Version**: Go 1.25, as declared in `go.mod`

**Primary Dependencies**: Kubernetes client-go/controller-runtime already used by Spiderpool; kubelet device plugin API from `k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1` using the repository's current `k8s.io/kubelet` dependency; existing Spiderpool managers in `pkg/ipam`, `pkg/iaas`, `pkg/podmanager`, `pkg/nodemanager`, and `pkg/multuscniconfig`

**Storage**: Kubernetes Node status for scheduler-facing extended resource capacity; kubelet device manager checkpoint for Pod-device assignments; existing Spiderpool CRDs such as SpiderEndpoint for IP allocation state; optional in-memory agent reconciliation state only

**Testing**: Ginkgo v2/Gomega unit and package tests, Helm template rendering checks for `kubeletRootDir` and both plugin path mounts, focused webhook tests for resource injection, device plugin manager tests with fake kubelet/device-plugin clients, path-selection tests for `plugins_registry` preference and `device-plugins` fallback, and targeted e2e coverage for scheduling and restart scenarios

**Target Platform**: Linux Kubernetes worker nodes running spiderpool-agent as a DaemonSet

**Project Type**: Kubernetes networking agent, mutating webhook integration, Helm chart packaging, and provider-mode IPAM integration

**Performance Goals**: Agent startup and device plugin registration should advertise `spidernet.io/eni-slot` within 30 seconds in 95% of restart events after node components are ready. Pod resource injection must add negligible admission overhead by reusing existing webhook resource injection flow. Allocation/release must not add extra Kubernetes API calls to the CNI hot path beyond existing provider-mode calls and device plugin RPCs.

**Constraints**: Preserve existing provider-mode behavior when the feature is disabled or a Pod does not request auxiliary ENI slots. Do not manually decrement `Node.status.allocatable["spidernet.io/eni-slot"]` after each allocation. Mount `{kubeletRootDir}/device-plugins` and `{kubeletRootDir}/plugins_registry` into spiderpool-agent only when the feature is enabled, defaulting `kubeletRootDir` to `/var/lib/kubelet`. At runtime prefer `plugins_registry` when present and fall back to `device-plugins` only when the preferred path is absent. Do not hardcode provider credentials or node-specific capacity outside operator configuration.

**Scale/Scope**: Per-node slot count is a small integer configured by operators, defaulting to disabled. For webhook-injected Pods, scheduler-facing slot quantity equals the number of eligible referenced VLAN SpiderMultusConfigs unless the Pod already declares the resource. Cluster scale should match current Spiderpool agent DaemonSet scale.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Document how the plan satisfies the Spiderpool Constitution:

- **Code quality and API compatibility**: The feature stays within existing package boundaries: command wiring in `cmd/spiderpool-agent/cmd`, reusable device plugin logic in a new `pkg/enislotdeviceplugin` package, provider integration in existing `pkg/ipam`/`pkg/iaas` flow, Pod resource injection in `pkg/podmanager`, and Helm wiring in `charts/spiderpool`. It adds optional Helm/configmap fields and does not change existing CRD semantics or provider-mode defaults. Public behavior is backward compatible because the feature is disabled by default unless configured.
- **Testing standard**: Required coverage includes unit tests for configuration validation, slot list generation, and kubelet plugin path selection; Ginkgo/Gomega tests for resource injection; package tests for registration/restart reconciliation behavior; Helm rendering tests for values, both kubelet path mounts, and RBAC changes; and targeted e2e tests for scheduling capacity and restart recovery.
- **User/operator consistency**: User-facing names must consistently use `spidernet.io/eni-slot`, "auxiliary ENI slot", `iaasNetworkProvider.eniDevPlugin`, `kubeletRootDir`, and `injectPodENIResources`. Docs must clarify total schedulable capacity vs derived free capacity, restart behavior, selected kubelet plugin path, and troubleshooting events.
- **Performance budget**: The feature touches CNI/provider allocation and kubelet device plugin RPC paths. Budget: no additional Kubernetes API calls in per-Pod CNI allocation beyond existing provider-mode lookups; device plugin `Allocate` should be local and complete in under 100 ms p95 excluding provider API time; registration/re-advertisement within 30 seconds p95 after agent readiness; no per-Pod Node status patching.
- **Generated artifacts**: Helm templates and values are source artifacts. If API or OpenAPI definitions are changed later, run the matching generation target. Current plan avoids CRD/OpenAPI changes unless implementation discovers a required user-facing API addition.

**Gate Result**: PASS. The design is optional, backward compatible, aligns with Kubernetes device plugin semantics, defines test coverage, and avoids manual generated artifact edits.

## Project Structure

### Documentation (this feature)

```text
specs/004-agent-eni-device-plugin/
|-- plan.md
|-- research.md
|-- data-model.md
|-- quickstart.md
|-- contracts/
|   |-- device-plugin-resource.md
|   |-- helm-values.md
|   |-- kubelet-plugin-paths.md
|   |-- pod-resource-injection.md
|   `-- restart-reconciliation.md
`-- tasks.md
```

### Source Code (repository root)

```text
cmd/spiderpool-agent/cmd/
|-- config.go              # parse auxiliary ENI slot config into agent context
`-- daemon.go              # start/stop the device plugin with agent lifecycle

pkg/enislotdeviceplugin/   # new package for kubelet device plugin server, slot list, health, restart registration
pkg/ipam/                  # existing provider allocate/release integration remains the allocation source of truth
pkg/iaas/                  # provider client contracts used by IPAM; extend only if ENI-specific provider calls are required
pkg/podmanager/            # existing mutating webhook resource injection pattern extended for spidernet.io/eni-slot
pkg/constant/              # canonical resource name, annotations/labels, config keys if needed
pkg/metric/                # optional diagnostics for advertised total and derived free slots

charts/spiderpool/
|-- values.yaml
`-- templates/
    |-- configmap.yaml     # render feature config
    |-- daemonset.yaml     # mount kubelet device-plugin and plugin-registration directories derived from kubeletRootDir
    `-- role.yaml          # add only permissions required by planned diagnostics

docs/usage/iaas-network-provider.md
docs/reference/configmap.md
docs/reference/spiderpool-agent.md
test/e2e/                  # targeted provider-mode scheduling and restart coverage
```

**Structure Decision**: Implement the kubelet-facing device plugin as a new reusable package and keep command wiring in `cmd/spiderpool-agent/cmd`. Reuse existing Pod webhook resource injection instead of introducing a scheduler plugin or controller. Keep provider allocation/release responsibility in existing IPAM/IaaS flow so the device plugin only gates scheduling and kubelet admission through `spidernet.io/eni-slot`.

## Complexity Tracking

No constitution violations require justification.

## Phase 0 Research Summary

See [research.md](./research.md). Key decisions:

- `spidernet.io/eni-slot` is a scheduler-facing total healthy capacity, not a remaining/free counter.
- Device plugin restart recovery relies on kubelet re-registration and kubelet device manager checkpoint behavior.
- Pod mutation should inject the extended resource only for Pods that reference eligible VLAN SpiderMultusConfigs while provider mode and `eniDevPlugin` are enabled; injected quantity equals the eligible reference count.
- Helm values should default `iaasNetworkProvider.eniDevPlugin.enabled` to disabled, default `kubeletRootDir` to `/var/lib/kubelet`, and mount both `{kubeletRootDir}/device-plugins` and `{kubeletRootDir}/plugins_registry` only when enabled.
- Kubelet plugin path selection should prefer `{kubeletRootDir}/plugins_registry` when present and fall back to `{kubeletRootDir}/device-plugins` when the preferred directory is absent.
- Dependency decision: use the repository's current `k8s.io/kubelet` dependency for the device plugin API instead of upgrading kubelet independently, unless the whole Kubernetes dependency set is upgraded together.

## Phase 1 Design Summary

See [data-model.md](./data-model.md), [quickstart.md](./quickstart.md), and [contracts/](./contracts/).

Design outputs define:

- Auxiliary ENI slot configuration, `kubeletRootDir`, and validation.
- Node status and device plugin resource contract for `spidernet.io/eni-slot`.
- Pod resource injection contract for eligible workloads.
- Restart reconciliation requirements for kubelet, agent, device-plugin, selected plugin path, and node reboot cases.

## Post-Design Constitution Check

- **Code quality and API compatibility**: PASS. Design isolates new logic under a package boundary, uses existing webhook/config patterns, and remains disabled by default.
- **Testing standard**: PASS. Unit, package, Helm rendering, and e2e coverage are identified for each changed behavior, including `kubeletRootDir` rendering and plugin path preference/fallback.
- **User/operator consistency**: PASS. Contracts and quickstart establish canonical names including `eniDevPlugin`, `kubeletRootDir`, and `injectPodENIResources`, and document the total-vs-free capacity distinction and plugin path fallback behavior.
- **Performance budget**: PASS. The design avoids per-Pod Node status writes and keeps scheduling capacity in Kubernetes extended-resource accounting.
- **Generated artifacts**: PASS. No generated CRD/OpenAPI edits are planned. Implementation review confirmed no changes under `api/`, `pkg/k8s/apis/`, or `charts/spiderpool/crds/` are required for the ENI device plugin configuration because it is Helm/configmap-driven rather than a CRD API extension. If implementation changes generated API sources later, generation targets must be run.

**Gate Result**: PASS. Ready for `/speckit-tasks`.
