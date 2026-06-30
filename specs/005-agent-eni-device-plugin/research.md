# Research: Agent Network Resource Plugin

## Decision: Use Kubernetes device plugin extended resource semantics

`spidernet.io/sub-eni` will be advertised through the kubelet device plugin model as the node's current healthy schedulable total auxiliary ENI slot capacity.

**Rationale**: Kubernetes scheduler accounts for extended resources by subtracting already-bound Pod requests from `Node.status.allocatable`. Kubernetes device plugin documentation describes device plugins reporting devices to kubelet and kubelet advertising those resources in node status. The kubelet device manager code returns capacity/allocatable from healthy devices, stores Pod-device assignments in checkpoints, and allows capacity to temporarily drop to zero until the plugin re-registers after kubelet restart.

**Alternatives considered**:

- Update `Node.status.allocatable["spidernet.io/sub-eni"]` as a free-slot counter after every Pod allocation/release. Rejected because it conflicts with Kubernetes scheduler/resource accounting and creates status race conditions.
- Implement a custom scheduler extender/plugin. Rejected because extended resources already provide the required hard scheduling constraint.
- Use only admission webhook validation. Rejected because admission does not have a stable scheduler view of per-node remaining capacity.

## Decision: Reuse Pod resource injection instead of requiring users to write resources manually

Eligible provider-mode Pods will receive a `spidernet.io/sub-eni` resource limit through the existing Spiderpool Pod network/resource injection path. Injection is based on the Pod's existing Multus default-network and attachment-network annotations: when those annotations reference VLAN-type SpiderMultusConfigs with nil VLAN ID, the injected quantity equals the number of eligible referenced configs. Explicit user resource configuration remains respected and is not overwritten.

**Rationale**: `pkg/podmanager` already inspects Pod/Namespace network resource injection annotations, resolves SpiderMultusConfigs, injects Multus network annotations, and injects extended resources for RDMA/SR-IOV style resources. Reusing the same webhook keeps the user experience consistent while making ENI scheduling protection depend on the networks the Pod already selected.

**Alternatives considered**:

- Require every user Pod to manually set `resources.limits.spidernet.io/sub-eni`. Rejected because it is error-prone and inconsistent with existing resource injection behavior.
- Add a second mutating webhook. Rejected because the existing webhook already owns network-related Pod mutation.
- Add a dedicated ENI injection annotation. Rejected because the Pod's existing Multus annotations and referenced VLAN SpiderMultusConfigs already identify whether ENI slot scheduling protection is needed.

## Decision: Keep the device plugin responsible for scheduling capacity, not provider allocation

The device plugin will register slot devices and satisfy kubelet `Allocate` calls for accepted Pods. Provider IP/ENI attach and release behavior remains in the existing IPAM/IaaS allocation and release flow unless implementation discovers a provider API call that must be made at device allocation time.

**Rationale**: The existing `pkg/ipam/iaas.go` path already calls the IaaS provider with Pod identity, node name, IP, subnet, and parent NIC information during allocation/release. Keeping provider side effects there minimizes behavior changes and avoids splitting provider transaction ownership across CNI and device plugin paths.

**Alternatives considered**:

- Move provider ENI allocation into device plugin `Allocate`. Rejected for v1 planning because CNI/IPAM already owns network allocation state and rollback behavior.
- Treat the device plugin as a no-op capacity placeholder only. Rejected unless provider allocation remains fully handled elsewhere and tests prove no lifecycle gap.

## Decision: Model restart recovery around kubelet re-registration and checkpointed assignments

After kubelet, agent, or device-plugin restart, the agent device plugin must re-register and re-list healthy slot devices. Kubelet restores assigned device mappings from its device manager checkpoint. New Pods requiring `spidernet.io/sub-eni` must not be schedulable until kubelet receives the registered resource again.

**Rationale**: Kubernetes device plugin documentation expects plugins to detect kubelet restarts and re-register because kubelet deletes sockets under `/var/lib/kubelet/device-plugins` during startup. Kubelet stores device manager checkpoint data in `/var/lib/kubelet/device-plugins/kubelet_internal_checkpoint`.

**Alternatives considered**:

- Persist an independent Spiderpool allocation checkpoint for scheduler accounting. Rejected for scheduler-facing capacity because kubelet already owns device assignments for extended resources.
- Force node status to the configured maximum immediately on agent startup. Rejected because this could admit new Pods before kubelet has a registered resource and healthy device list.

## Decision: Add Helm values under the agent network resource plugin section

Add an optional `spiderpoolAgent.networkResourcePlugin` values section for enablement, `kubeletRootDir`, device plugin node affinity, and resource advertisement. `resourceAdvertisement.masterNIC` controls `spidernet.io/<master>-nic` advertisement and can work outside provider mode. `resourceAdvertisement.subENI` controls `spidernet.io/sub-eni` advertisement and is active only when provider mode is enabled. `resourceAdvertisement.subENI.rules[].defaultMaxCount` sets the advertised sub-ENI capacity, and `resourceAdvertisement.subENI.rules[].nodeSelector` optionally limits which nodes advertise it. Pod resource injection remains controlled by the existing `spiderpoolController.podResourceInject.enabled` setting.

**Rationale**: Master NIC scheduling is useful outside provider mode, so the top-level configuration must live under the agent rather than the provider integration section. Keeping the plugin under `spiderpoolAgent.networkResourcePlugin` makes the user-facing feature describe network resource advertisement and injection rather than the kubelet device-plugin implementation detail.

**Alternatives considered**:

- Keep the feature under the provider integration section. Rejected because master NIC scheduling does not require provider mode.
- Add a top-level device-plugin-only section. Rejected because it exposes the implementation mechanism rather than the network resource scheduling feature.
- Reuse RDMA shared device plugin values. Rejected because ENI slots are implemented inside spiderpool-agent, not as a separate third-party DaemonSet.
- Expose separate full paths for device plugin and plugin registration directories. Rejected because a single `kubeletRootDir` keeps configuration aligned with kubelet and avoids inconsistent path combinations.
- Configure every node's sub-ENI capacity in Helm. Rejected because large clusters would require long, brittle node-name lists.

## Decision: Advertise master NIC resources from physical NIC discovery and node NIC name rules

The network resource plugin will advertise `spidernet.io/<master>-nic` for each selected physical master NIC on enabled nodes. `resourceAdvertisement.masterNIC.rules[].defaultMaxCount` controls the advertised virtual quantity and defaults to `10000`. Helm `resourceAdvertisement.masterNIC.rules` can narrow the set for matching nodes with optional Kubernetes label selector `nodeSelector`, `includeInterfaces`, and `excludeInterfaces` shell-style glob patterns. A rule without `nodeSelector` matches all enabled nodes. When rules match, include results are combined and `excludeInterfaces` removes matches before advertising.

**Rationale**: Operators need master NIC scheduling without modeling bandwidth or queue count. A large default virtual quantity avoids accidental quantity exhaustion while still requiring the resource key to exist on the scheduled node. Rule-driven enablement keeps the default installation quiet, while glob rules handle NIC naming patterns without enumerating every interface.

**Alternatives considered**:

- Require exact NIC name lists for every node. Rejected because it does not scale with many nodes or changing NIC inventories.
- Use regular expressions. Rejected for v1 because shell-style globs match the requested examples (`eth*`, `ens[0-9]`) and are easier to document and validate.
- Reuse `spidernet.io/sub-eni` only. Rejected because it cannot express master NIC placement constraints.

## Decision: Reconcile local Node metadata changes instead of polling per report

Each spiderpool-agent will read/cache only its own Node object, recompute desired network resources when relevant labels change, and notify kubelet only when the computed resource set changes. A low-frequency resync may be used as a fallback, but `ListAndWatch` must not perform a Kubernetes API GET for every report.

**Rationale**: Node labels and annotations are dynamic operator inputs for excluding nodes, changing per-node sub-ENI capacity, and selecting NIC rules. Watching only the local Node keeps API load proportional to node count and avoids cluster-wide watches in every agent. Comparing computed results before notifying kubelet avoids unnecessary device-list churn.

**Alternatives considered**:

- Require agent restart after Node label changes. Rejected because operators need runtime reconfiguration.
- Poll the Node object before every device-plugin update. Rejected because it creates avoidable API load and couples kubelet reporting to apiserver latency.
- Watch all Nodes from every agent. Rejected because each agent only needs its own Node metadata.

## Decision: Mount both kubelet plugin directories and select at runtime

When enabled, spiderpool-agent will mount both `{kubeletRootDir}/device-plugins` and `{kubeletRootDir}/plugins_registry`. Runtime path selection prefers `{kubeletRootDir}/plugins_registry` when it exists and falls back to `{kubeletRootDir}/device-plugins` when the preferred directory is absent. The selected path and fallback reason must be visible in logs or events.

**Rationale**: Kubernetes current kubelet plugin registration uses `plugins_registry` under the kubelet root, while the v1beta1 device-plugin API and documentation still expose the legacy `/var/lib/kubelet/device-plugins` service socket. Mounting both paths gives compatibility across clusters and lets implementation validate the real node filesystem before choosing.

**Alternatives considered**:

- Mount only `plugins_registry`. Rejected because the v1beta1 device-plugin API and existing kubelet package constants still use the device plugin socket path.
- Mount only `device-plugins`. Rejected because it ignores the newer plugin registration directory and non-default kubelet root deployments.
- Hardcode `/var/lib/kubelet`. Rejected because kubelet root is configurable and the chart must support non-default roots.

## Decision: Expose derived free slot count only as diagnostics

If Spiderpool exposes free ENI slots, it should do so as a metric/event/log or optional diagnostic status, derived from advertised total and active Pod requests/allocations. It must not replace `spidernet.io/sub-eni`.

**Rationale**: Operators need troubleshooting visibility, but scheduler-facing node status must keep Kubernetes extended-resource semantics.

**Alternatives considered**:

- Do not expose derived free slot count. Acceptable for MVP, but weaker for troubleshooting.
- Store derived free count in Node allocatable. Rejected for correctness.
