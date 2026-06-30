# Data Model: Agent Network Resource Plugin

## Network Resource Plugin Configuration

Represents operator intent for Spiderpool network resource advertisement and webhook injection.

**Fields**:

- `enabled`: Boolean. Default `false`. Enables the spiderpool-agent network resource plugin.
- `kubeletRootDir`: String. Default `/var/lib/kubelet`. Used to derive kubelet plugin host path mounts and runtime plugin path selection.
- `devicePluginAffinity.nodeSelector`: Kubernetes label selector. Default empty selector. Matching nodes advertise Spiderpool network resources.
- `resourceAdvertisement.subENI`: Auxiliary ENI resource advertisement settings.
- `resourceAdvertisement.masterNIC`: Master NIC resource advertisement settings.

**Validation rules**:

- `enabled=false` disables all network resource advertisement and webhook injection for these resources.
- Pod resource injection is controlled by the existing Spiderpool controller `podResourceInject.enabled` setting.
- `kubeletRootDir` must be an absolute host path.
- When enabled, the chart derives and mounts `{kubeletRootDir}/device-plugins` and `{kubeletRootDir}/plugins_registry`.
- `devicePluginAffinity.nodeSelector` uses Kubernetes label selector semantics.

## Auxiliary ENI Resource Advertisement

Represents provider-mode sub-ENI resource advertisement.

**Fields**:

- `enabled`: Boolean. Default `false`. Enables `spidernet.io/sub-eni` advertisement when provider mode is enabled.
- `subENI`: Array. Sub-ENI resource advertisement rules.
- `rules[].resourceName`: String. Default `spidernet.io/sub-eni`. Must follow Kubernetes extended resource naming rules.
- `rules[].defaultMaxCount`: Integer. Default `0`. Auxiliary ENI slot count advertised for matching enabled nodes. Must be `>= 0`.
- `rules[].nodeSelector`: Kubernetes label selector. Default `{}`. Optional selector limiting which nodes advertise the rule's sub-ENI resource.

**Validation rules**:

- `enabled=true` is active only when provider integration is configured.
- `enabled=true` with `rules[].defaultMaxCount=0` is allowed but makes Pods requesting the resource unschedulable on nodes matching that entry.
- `rules[].resourceName` must remain stable across agent restarts and chart upgrades unless operators intentionally migrate workloads.
- `rules[].nodeSelector` must follow Kubernetes label selector semantics.

## Node Network Resource Metadata

Represents dynamic Node labels that affect local network resource advertising.

**Fields**:

- `labels`: Node labels used by `devicePluginAffinity.nodeSelector`, `resourceAdvertisement.subENI.rules[].nodeSelector`, and `resourceAdvertisement.masterNIC.rules[*].nodeSelector`.

**Validation rules**:

- Label changes are dynamic inputs and must not require an agent restart.

## Master NIC Name Rule

Represents a Helm rule that selects which physical master NICs are advertised on matching nodes.

**Fields**:

- `nodeSelector`: Optional Kubernetes label selector. When omitted, the rule matches all enabled nodes.
- `defaultMaxCount`: Integer. Default `10000`. Virtual capacity advertised for each selected master NIC. Must be `>= 0`.
- `includeInterfaces`: Optional list of shell-style glob patterns. When omitted in a matching rule, all physical NIC names are initially included.
- `excludeInterfaces`: Optional list of shell-style glob patterns removed from the included set.

**Rules**:

- If `resourceAdvertisement.masterNIC.rules` is empty, master NIC advertisement is disabled.
- If no `resourceAdvertisement.masterNIC.rules` match the local Node, no master NIC resources are advertised.
- If one or more rules match, include results are combined.
- `excludeInterfaces` takes precedence over `includeInterfaces`.
- Only physical master NICs may be advertised; loopback, CNI/container virtual interfaces, bridge devices, and other non-physical interfaces must be filtered out before advertisement.

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

## Auxiliary ENI Slot Device

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

## Master NIC Device

Represents one selected physical master NIC resource reported to kubelet.

**Fields**:

- `interfaceName`: Physical master NIC name.
- `resourceName`: `spidernet.io/<master>-nic`, where `<master>` is `interfaceName`.
- `quantity`: Virtual quantity from `resourceAdvertisement.masterNIC.rules[].defaultMaxCount`, defaulting to `10000`.
- `health`: Healthy or unhealthy according to the device plugin API.

**Relationships**:

- Belongs to one enabled node.
- Exists only when the NIC is selected by default physical NIC discovery or matching `resourceAdvertisement.masterNIC.rules`.

**State transitions**:

- `Selected` -> `Advertised`: The agent reports the master NIC resource to kubelet.
- `Advertised` -> `Removed`: Node labels, NIC rules, device discovery, or exclude selectors no longer select the NIC.
- `Advertised` -> `Unavailable`: Plugin disconnects, kubelet restarts, or NIC health becomes unhealthy.

## Node Auxiliary ENI Status

Represents Kubernetes-visible scheduling capacity.

**Fields**:

- `capacity[resourceName]`: Total slots reported by the device plugin, including unhealthy devices when kubelet models capacity that way.
- `allocatable[resourceName]`: Healthy schedulable total slots reported by kubelet for scheduling.

**Rules**:

- `allocatable[spidernet.io/sub-eni]` is not a real-time free counter.
- Remaining schedulable capacity is computed by Kubernetes scheduler from node allocatable minus already-bound Pod requests.
- Node status may temporarily omit or set the resource to zero while the plugin is not registered after restart.
- `spidernet.io/<master>-nic` resources represent selected physical NIC presence with configurable virtual quantity, not bandwidth or queue count.

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
