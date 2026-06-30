# Implementation Plan: Agent Network Resource Plugin

**Branch**: `005-agent-eni-device-plugin` | **Date**: 2026-06-17 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/005-agent-eni-device-plugin/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Add an optional Spiderpool agent network resource plugin for scheduler-facing network resource advertisement and webhook resource injection. The plugin advertises `spidernet.io/<master>-nic` resources with configurable virtual quantity for selected physical master NICs and can be used outside provider mode. When provider mode is enabled, the same plugin also advertises `spidernet.io/sub-eni` as the node's current healthy schedulable total auxiliary ENI slot capacity, not a free-slot counter. Eligible Pods must request the relevant resources so Kubernetes scheduler accounting prevents placement on unsuitable nodes. The agent integrates with existing IaaS provider allocation/release flows, Helm configuration, Pod resource injection patterns, kubelet plugin path selection, node observability, local Node label reconciliation, and restart recovery.

## Technical Context

**Language/Version**: Go 1.25, as declared in `go.mod`

**Primary Dependencies**: Kubernetes client-go/controller-runtime already used by Spiderpool; kubelet device plugin API from `k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1` using the repository's current `k8s.io/kubelet` dependency; existing Spiderpool managers in `pkg/ipam`, `pkg/iaas`, `pkg/podmanager`, `pkg/nodemanager`, and `pkg/multuscniconfig`; local Node informer/watch support for this agent's own Node object

**Storage**: Kubernetes Node status for scheduler-facing extended resource capacity; kubelet device manager checkpoint for Pod-device assignments; existing Spiderpool CRDs such as SpiderEndpoint for IP allocation state; local cached Node labels/annotations and in-memory desired device list state only

**Testing**: Ginkgo v2/Gomega unit and package tests, Helm template rendering checks for `spiderpoolAgent.networkResourcePlugin`, `resourceAdvertisement.subENI`, `resourceAdvertisement.masterNIC`, `kubeletRootDir`, and both plugin path mounts; focused webhook tests for resource injection; device plugin manager tests with fake kubelet/device-plugin clients; local Node reconcile tests for annotation/label updates; path-selection tests for `plugins_registry` preference and `device-plugins` fallback; and targeted e2e coverage for scheduling, dynamic config updates, non-provider master NIC behavior, provider-mode sub-ENI behavior, and restart scenarios

**Target Platform**: Linux Kubernetes worker nodes running spiderpool-agent as a DaemonSet

**Project Type**: Kubernetes networking agent, mutating webhook integration, Helm chart packaging, and provider-mode IPAM integration

**Performance Goals**: Agent startup and device plugin registration should advertise `spidernet.io/sub-eni` and selected `spidernet.io/<master>-nic` resources within 30 seconds in 95% of restart events after node components are ready. Node label updates that change the computed desired resource set should converge within 30 seconds in 95% of observed updates without restarting spiderpool-agent. Pod resource injection must add negligible admission overhead by reusing existing webhook resource injection flow. Allocation/release must not add extra Kubernetes API calls to the CNI hot path beyond existing provider-mode calls and device plugin RPCs.

**Constraints**: Preserve existing behavior when the feature is disabled or a Pod does not request Spiderpool network resources. Do not manually decrement `Node.status.allocatable["spidernet.io/sub-eni"]` after each allocation. Mount `{kubeletRootDir}/device-plugins` and `{kubeletRootDir}/plugins_registry` into spiderpool-agent only when `spiderpoolAgent.networkResourcePlugin.enabled=true`, defaulting `kubeletRootDir` to `/var/lib/kubelet`. At runtime prefer `plugins_registry` when present and fall back to `device-plugins` only when the preferred path is absent. Network resource advertising runs on every spiderpool-agent node by default because the default `spiderpoolAgent.networkResourcePlugin.devicePluginAffinity.nodeSelector` is empty and matches all nodes. Sub-ENI capacity comes from `spiderpoolAgent.networkResourcePlugin.resourceAdvertisement.subENI.rules[].defaultMaxCount`, and sub-ENI advertisement can be limited with `resourceAdvertisement.subENI.rules[].nodeSelector`. Master NIC advertisement is enabled by non-empty `spiderpoolAgent.networkResourcePlugin.resourceAdvertisement.masterNIC.rules`; matching rules select interfaces with shell-style glob patterns and set virtual capacity with `defaultMaxCount`. Master NIC advertisement can run outside provider mode; sub-ENI advertisement remains inactive unless provider mode is enabled. Avoid per-report Kubernetes API reads; reconcile from a local Node watch/cache and only notify kubelet when the computed resource set changes. Do not hardcode provider credentials or node-specific capacity outside Helm defaults.

**Scale/Scope**: Per-node slot count is a small integer from Helm default, defaulting to zero until configured. Master NIC resources are one extended resource per selected physical master NIC with virtual quantity from `resourceAdvertisement.masterNIC.rules[].defaultMaxCount`, defaulting to `10000`. For webhook-injected Pods, scheduler-facing slot quantity equals the number of eligible referenced VLAN SpiderMultusConfigs unless the Pod already declares the resource. Cluster scale should match current Spiderpool agent DaemonSet scale, with each agent watching only its own Node object for dynamic label changes.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Document how the plan satisfies the Spiderpool Constitution:

- **Code quality and API compatibility**: The feature stays within existing package boundaries: command wiring in `cmd/spiderpool-agent/cmd`, reusable network resource plugin logic in a new `pkg/networkresourceplugin` package, provider integration in existing `pkg/ipam`/`pkg/iaas` flow, Pod resource injection in `pkg/podmanager`, and Helm wiring in `charts/spiderpool`. It adds optional Helm/configmap fields and does not change existing CRD semantics or provider-mode defaults. Public behavior is backward compatible because the feature is disabled by default unless configured.
- **Testing standard**: Required coverage includes unit tests for configuration validation, slot list generation, NIC name rule matching, physical NIC filtering, local Node label reconciliation, and kubelet plugin path selection; Ginkgo/Gomega tests for resource injection; package tests for registration/restart/reconfiguration behavior; Helm rendering tests for values, both kubelet path mounts, and RBAC changes; and targeted e2e tests for scheduling capacity, dynamic Node metadata updates, and restart recovery.
- **User/operator consistency**: User-facing names must consistently use `spiderpoolAgent.networkResourcePlugin`, `resourceAdvertisement.subENI`, `resourceAdvertisement.masterNIC`, `spidernet.io/sub-eni`, `spidernet.io/<master>-nic`, "auxiliary ENI slot", "master NIC resource", `kubeletRootDir`, and Spiderpool network resource injection. Docs must clarify master NIC behavior outside provider mode, sub-ENI provider-mode gating, total schedulable capacity vs derived free capacity, restart behavior, dynamic Node label reconciliation, selected kubelet plugin path, and troubleshooting events.
- **Performance budget**: The feature touches CNI/provider allocation, kubelet device plugin RPC paths, and local Node watch/reconcile. Budget: no additional Kubernetes API calls in per-Pod CNI allocation beyond existing provider-mode lookups; no per-report API reads for Node metadata; each agent watches/caches only its own Node object; device plugin `Allocate` should be local and complete in under 100 ms p95 excluding provider API time; registration/re-advertisement within 30 seconds p95 after agent readiness; dynamic Node metadata changes converge within 30 seconds p95; no per-Pod Node status patching.
- **Generated artifacts**: Helm templates and values are source artifacts. If API or OpenAPI definitions are changed later, run the matching generation target. Current plan avoids CRD/OpenAPI changes unless implementation discovers a required user-facing API addition.

**Gate Result**: PASS. The design is optional, backward compatible, aligns with Kubernetes device plugin semantics, defines test coverage, and avoids manual generated artifact edits.

## Project Structure

### Documentation (this feature)

```text
specs/005-agent-eni-device-plugin/
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
|-- config.go              # parse network resource plugin config into agent context
`-- daemon.go              # start/stop the network resource plugin with agent lifecycle

pkg/networkresourceplugin/ # new package for network resource discovery, kubelet device plugin servers, device lists, health, restart registration, and local Node reconciliation
pkg/ipam/                  # existing provider allocate/release integration remains the allocation source of truth
pkg/iaas/                  # provider client contracts used by IPAM; extend only if ENI-specific provider calls are required
pkg/podmanager/            # existing mutating webhook resource injection pattern extended for Spiderpool network resources
pkg/constant/              # canonical resource name, annotations/labels, config keys if needed
pkg/metric/                # optional diagnostics for advertised total and derived free slots

charts/spiderpool/
|-- values.yaml
`-- templates/
    |-- configmap.yaml     # render feature config
    |-- daemonset.yaml     # mount kubelet device-plugin and plugin-registration directories derived from kubeletRootDir
    `-- role.yaml          # add only permissions required for local Node watch and planned diagnostics

docs/usage/iaas-network-provider.md
docs/reference/configmap.md
docs/reference/spiderpool-agent.md
test/e2e/                  # targeted provider-mode scheduling and restart coverage
```

**Structure Decision**: Implement the agent network resource plugin as a new reusable package and keep command wiring in `cmd/spiderpool-agent/cmd`. Reuse existing Pod webhook resource injection instead of introducing a scheduler plugin or controller. Keep provider allocation/release responsibility in existing IPAM/IaaS flow so the plugin gates scheduling and kubelet admission through extended resources while provider allocation remains in the existing IPAM path.

## Complexity Tracking

No constitution violations require justification.

## Phase 0 Research Summary

See [research.md](./research.md). Key decisions:

- `spidernet.io/sub-eni` is a scheduler-facing total healthy capacity, not a remaining/free counter.
- Device plugin restart recovery relies on kubelet re-registration and kubelet device manager checkpoint behavior.
- Pod mutation should inject Spiderpool network resources only when the existing controller `podResourceInject.enabled` setting and the corresponding advertised resource are enabled; existing user-declared resources are respected.
- Helm values should default `spiderpoolAgent.networkResourcePlugin.enabled` to disabled, default `kubeletRootDir` to `/var/lib/kubelet`, and mount both `{kubeletRootDir}/device-plugins` and `{kubeletRootDir}/plugins_registry` only when enabled.
- Helm values should expose `spiderpoolAgent.networkResourcePlugin.devicePluginAffinity.nodeSelector`, `spiderpoolAgent.networkResourcePlugin.resourceAdvertisement.subENI.rules[].defaultMaxCount`, `resourceAdvertisement.subENI.rules[].nodeSelector`, and `resourceAdvertisement.masterNIC.rules` with `nodeSelector`, `defaultMaxCount`, `includeInterfaces`, and `excludeInterfaces`.
- Master NIC advertisement should be driven by non-empty `resourceAdvertisement.masterNIC.rules`; `excludeInterfaces` takes precedence over `includeInterfaces`, patterns use shell-style glob matching, `defaultMaxCount` controls virtual capacity, and master NIC advertisement can be used outside provider mode.
- `resourceAdvertisement.subENI` should remain inactive unless provider mode is enabled.
- Dynamic Node labels and annotations should be watched through the local Node cache/informer and reconciled only when the computed desired resource set changes, avoiding per-report API reads.
- Kubelet plugin path selection should prefer `{kubeletRootDir}/plugins_registry` when present and fall back to `{kubeletRootDir}/device-plugins` when the preferred directory is absent.
- Dependency decision: use the repository's current `k8s.io/kubelet` dependency for the device plugin API instead of upgrading kubelet independently, unless the whole Kubernetes dependency set is upgraded together.

## Phase 1 Design Summary

See [data-model.md](./data-model.md), [quickstart.md](./quickstart.md), and [contracts/](./contracts/).

Design outputs define:

- Network resource plugin configuration, webhook injection, node exclusion selectors, NIC name rules, `kubeletRootDir`, and validation.
- Node status and device plugin resource contract for `spidernet.io/sub-eni` and `spidernet.io/<master>-nic`.
- Pod resource injection contract for eligible workloads.
- Restart and dynamic reconfiguration requirements for kubelet, agent, device-plugin, selected plugin path, local Node metadata changes, and node reboot cases.

## Post-Design Constitution Check

- **Code quality and API compatibility**: PASS. Design isolates new logic under a package boundary, uses existing webhook/config patterns, and remains disabled by default.
- **Testing standard**: PASS. Unit, package, Helm rendering, and e2e coverage are identified for each changed behavior, including `spiderpoolAgent.networkResourcePlugin`, node exclusion selectors, NIC name rules, local Node metadata reconciliation, `kubeletRootDir` rendering, provider-mode sub-ENI gating, non-provider master NIC behavior, and plugin path preference/fallback.
- **User/operator consistency**: PASS. Contracts and quickstart establish canonical names including `spiderpoolAgent.networkResourcePlugin`, `resourceAdvertisement.subENI`, `resourceAdvertisement.masterNIC`, and `kubeletRootDir`, and document the total-vs-free capacity distinction, master NIC resources, dynamic Node metadata reconciliation, and plugin path fallback behavior.
- **Performance budget**: PASS. The design avoids per-Pod Node status writes, avoids per-report API reads, watches only the local Node object, and keeps scheduling capacity in Kubernetes extended-resource accounting.
- **Generated artifacts**: PASS. No generated CRD/OpenAPI edits are planned. Implementation review confirmed no changes under `api/`, `pkg/k8s/apis/`, or `charts/spiderpool/crds/` are required for the network resource plugin configuration because it is Helm/configmap-driven rather than a CRD API extension. If implementation changes generated API sources later, generation targets must be run.

**Gate Result**: PASS. Ready for `/speckit-tasks`.
