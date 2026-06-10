# Data Model: Agent ENI Device Plugin

## Auxiliary ENI Slot Configuration

Represents operator intent for provider-mode ENI slot scheduling.

**Fields**:

- `enabled`: Boolean. Default `false`. Enables the spiderpool-agent device plugin only when provider mode is configured.
- `resourceName`: String. Default `spidernet.io/sub-eni`. Must follow Kubernetes extended resource naming rules.
- `maxSlotsPerNode`: Integer. Default `0`. The node's configured maximum auxiliary ENI slot count. Must be `>= 0`.
- `injectPodENIResources`: Boolean. Default `true`. Allows Spiderpool's Pod mutation path to inject the ENI slot resource for eligible Pods.
- `kubeletRootDir`: String. Default `/var/lib/kubelet`. Used to derive kubelet plugin host path mounts and runtime plugin path selection.

**Validation rules**:

- `enabled=true` requires provider integration to be configured.
- `enabled=true` with `maxSlotsPerNode=0` is allowed but makes Pods requesting the resource unschedulable to that node.
- `resourceName` must remain stable across agent restarts and chart upgrades unless operators intentionally migrate workloads.
- `kubeletRootDir` must be an absolute host path.
- When enabled, the chart derives and mounts `{kubeletRootDir}/device-plugins` and `{kubeletRootDir}/plugins_registry`.

## Kubelet Plugin Path Selection

Represents the node-local path decision used by the agent device plugin.

**Fields**:

- `kubeletRootDir`: Configured kubelet root directory.
- `devicePluginPath`: Derived as `{kubeletRootDir}/device-plugins`.
- `pluginRegistrationPath`: Derived as `{kubeletRootDir}/plugins_registry`.
- `selectedPath`: The path chosen at agent startup or reconciliation.
- `selectionReason`: `preferred-present` or `fallback-preferred-absent`.

**Validation rules**:

- `pluginRegistrationPath` is preferred when it exists.
- `devicePluginPath` is used only when `pluginRegistrationPath` is absent.
- The selected path and fallback reason must be logged or otherwise diagnosable.

## ENI Slot Device

Represents one schedulable auxiliary ENI slot reported to kubelet.

**Fields**:

- `id`: Stable per-node slot identifier, for example `sub-eni-0`.
- `health`: Healthy or unhealthy according to the device plugin API.
- `resourceName`: The extended resource name, normally `spidernet.io/sub-eni`.

**Relationships**:

- Belongs to one node.
- May be assigned by kubelet to one Pod container at a time.

**State transitions**:

- `Configured` -> `Advertised`: Agent starts and registers healthy slots with kubelet.
- `Advertised` -> `Allocated`: Kubelet assigns a slot to a Pod that requested the resource.
- `Allocated` -> `Released`: Pod no longer requires the slot and kubelet frees the assignment.
- `Advertised` -> `Unavailable`: Plugin disconnects, kubelet restarts, or slot health becomes unhealthy.

## Node Auxiliary ENI Status

Represents Kubernetes-visible scheduling capacity.

**Fields**:

- `capacity[resourceName]`: Total slots reported by the device plugin, including unhealthy devices when kubelet models capacity that way.
- `allocatable[resourceName]`: Healthy schedulable total slots reported by kubelet for scheduling.

**Rules**:

- `allocatable[spidernet.io/sub-eni]` is not a real-time free counter.
- Remaining schedulable capacity is computed by Kubernetes scheduler from node allocatable minus already-bound Pod requests.
- Node status may temporarily omit or set the resource to zero while the plugin is not registered after restart.

## Pod Auxiliary ENI Request

Represents a Pod's declared need for auxiliary ENI capacity.

**Fields**:

- `resourceName`: `spidernet.io/sub-eni`.
- `quantity`: Integer. When injected by the webhook, the value equals the number of eligible VLAN SpiderMultusConfigs referenced by the Pod through Multus default-network or attachment-network annotations.
- `source`: Explicit user Pod resources or Spiderpool Pod resource injection.

**Validation rules**:

- Eligible provider-mode Pods must request the resource before scheduling.
- Existing user-provided resource limits must not be overwritten.
- Pods not requiring auxiliary ENIs must not receive the resource.
- When a Pod already declares `spidernet.io/sub-eni`, Spiderpool must not recalculate, overwrite, duplicate, or increment the declared quantity.

## Auxiliary ENI Allocation Record

Represents the allocation relationship needed to release provider resources safely.

**Fields**:

- `podNamespace`
- `podName`
- `podUID`
- `nodeName`
- `slotID`
- `ipAllocationDetails`
- `providerAllocationDetails`

**Relationships**:

- Kubelet owns the device assignment checkpoint for `slotID`.
- Spiderpool IPAM/SpiderEndpoint owns IP/provider allocation status.

**State transitions**:

- `Requested` -> `DeviceAssigned`: Kubelet assigned a slot during container creation.
- `DeviceAssigned` -> `ProviderAllocated`: Existing IPAM/IaaS flow completed provider allocation.
- `ProviderAllocated` -> `Released`: Pod deletion or failure cleanup released IP/provider state.
- `Released` -> `Reusable`: Slot can satisfy a later Pod request.

## Derived Free ENI Slot Count

Optional diagnostic value, not scheduler-facing.

**Fields**:

- `advertisedTotal`
- `activeRequested`
- `derivedFree`
- `lastCalculatedAt`

**Rules**:

- `derivedFree = advertisedTotal - activeRequested` or equivalent allocation-based calculation.
- Must never replace `Node.status.allocatable[spidernet.io/sub-eni]`.
- Used only for logs, metrics, events, or troubleshooting output.
