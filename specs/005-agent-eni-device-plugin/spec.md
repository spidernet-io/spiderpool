# Feature Specification: Agent Network Resource Plugin

**Feature Branch**: `005-agent-eni-device-plugin`

**Created**: 2026-06-09

**Status**: Draft

**Input**: User description: "Implement a Spiderpool agent network resource plugin that advertises scheduler-facing network resources and supports webhook resource injection. Master NIC resources can be used outside provider mode. When provider mode is enabled, the same plugin can additionally advertise auxiliary elastic network interface capacity so Pods are not scheduled onto nodes without available auxiliary ENIs."

## Clarifications

### Session 2026-06-09

- Q: What does `spidernet.io/sub-eni` represent in node status after kubelet, node, or device-plugin restart? -> A: `spidernet.io/sub-eni` represents the current healthy schedulable total capacity reported through the Kubernetes device plugin resource model. It is not a real-time remaining/free slot counter. Kubernetes scheduling determines remaining schedulable capacity by subtracting already-bound Pod resource requests from the node's allocatable total.
- Q: How should restart recovery keep ENI slot scheduling correct? -> A: After kubelet or device-plugin restart, the device plugin must re-register with kubelet and report the healthy slot list again. Kubelet restores previously allocated device assignments from its device manager checkpoint, and new Pods requesting `spidernet.io/sub-eni` are not schedulable until the resource is registered and advertised again.
- Q: What source material defines this behavior? -> A: Kubernetes device plugin documentation states that a plugin reports devices to kubelet, kubelet advertises the resource through node status, unhealthy devices reduce allocatable while capacity remains unchanged, and free/unallocated resources must be derived from allocatable resources together with Pod resource allocation data. The kubelet device manager implementation also records Pod-device assignments in a checkpoint and notes that device plugin resource capacity can temporarily drop to zero until the plugin re-registers after kubelet restart.
- Q: How should the webhook decide whether to inject the ENI slot resource? -> A: Do not add a dedicated Pod annotation for ENI slot injection. When provider mode and `spiderpoolAgent.networkResourcePlugin.enabled` are enabled, and `resourceAdvertisement.subENI.rules` contains at least one rule, the existing Pod webhook must inspect the Pod's current Multus default-network and attachment-network annotations. If any referenced SpiderMultusConfig is VLAN type and its VLAN ID is nil, the webhook injects `spidernet.io/sub-eni` into the Pod resources unless the Pod already declares the same resource key.
- Q: How many ENI slot resources should the webhook inject when a Pod references multiple eligible VLAN SpiderMultusConfigs? -> A: If the Pod does not already declare `spidernet.io/sub-eni`, inject a quantity equal to the number of referenced eligible VLAN SpiderMultusConfigs.

### Session 2026-06-10

- Q: Which kubelet socket path should the network resource plugin use on current Kubernetes? -> A: Treat `{kubeletRootDir}/plugins_registry/` as the current kubelet plugin registration directory for external plugin registration, while recognizing that the v1beta1 device-plugin API constants still define `/var/lib/kubelet/device-plugins/` and `/var/lib/kubelet/device-plugins/kubelet.sock` for the legacy device-plugin gRPC service. The implementation plan must explicitly choose the registration mechanism and mount the matching host path, especially when kubelet uses a non-default root directory.
- Q: How should Spiderpool configure and select kubelet plugin paths for compatibility? -> A: Spiderpool must expose a configurable `kubeletRootDir`, mount both `{kubeletRootDir}/device-plugins` and `{kubeletRootDir}/plugins_registry` into the agent, prefer `{kubeletRootDir}/plugins_registry` when it exists, and fall back to `{kubeletRootDir}/device-plugins` when the new registration directory is absent.

### Session 2026-06-17

- Q: How should Helm values declare the agent network resource plugin, resource advertisement, and webhook injection? -> A: The Helm values must expose `spiderpoolAgent.networkResourcePlugin` as the top-level agent feature. It includes `enabled`, `kubeletRootDir`, and `devicePluginAffinity.nodeSelector`. Under `resourceAdvertisement`, `masterNIC` advertises `spidernet.io/<master>-nic` independently of provider mode, and `subENI` advertises `spidernet.io/sub-eni` only when provider mode is enabled. `masterNIC.rules` narrows physical NIC selection with optional Kubernetes label selector `nodeSelector`, `defaultMaxCount`, `includeInterfaces`, and `excludeInterfaces` shell-style glob patterns. `subENI.rules[]` lists sub-ENI advertisement rules; each rule's `defaultMaxCount` defines the advertised capacity and `nodeSelector` optionally limits which nodes advertise it.
- Q: How should the network resource plugin react when node labels that control advertising are updated? -> A: The spiderpool-agent must watch or otherwise reconcile relevant Node label changes and update `spidernet.io/sub-eni` and `spidernet.io/<master>-nic` advertisements automatically without requiring an agent restart.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Schedule Pods Only Where Required Network Resources Are Available (Priority: P1)

As a cluster operator, I need workloads that require specific master NICs or provider-mode auxiliary elastic network interfaces to request the matching Spiderpool network resources and be scheduled only onto nodes advertising those resources, so Pod startup does not fail after placement on an unsuitable node.

**Why this priority**: This is the primary user value. It prevents invalid scheduling decisions before allocation is attempted.

**Independent Test**: Configure master NIC resource advertisement for a mixed-NIC node pool and auxiliary ENI slot capacity for a provider-mode node pool, create Pods referencing eligible SpiderMultusConfigs, verify the webhook injects `spidernet.io/<master>-nic` and/or `spidernet.io/sub-eni` only when needed, and verify Pods are admitted only to nodes where Kubernetes resource accounting can satisfy the new requests.

**Acceptance Scenarios**:

1. **Given** a node advertises `spidernet.io/<master>-nic` for the master NIC required by a Pod's selected network, **When** the Pod is created, **Then** the Pod can be scheduled to that node and Kubernetes accounts for the request against that node.
2. **Given** no candidate node advertises the required `spidernet.io/<master>-nic` resource, **When** a Pod requests that master NIC resource, **Then** the Pod remains unscheduled instead of being placed on a node without the required master NIC.
3. **Given** provider mode is enabled and a node advertises enough `spidernet.io/sub-eni` allocatable capacity after existing Pod requests are considered, **When** a Pod requests one slot, **Then** the Pod can be scheduled to that node and Kubernetes accounts for the request against that node.
4. **Given** provider mode is enabled and all candidate nodes have no remaining schedulable `spidernet.io/sub-eni` capacity after existing Pod requests are considered, **When** a Pod requests one slot, **Then** the Pod remains unscheduled instead of being placed on a node that cannot allocate the interface.
5. **Given** the network resource plugin and webhook resource injection are enabled, **When** a Pod references one or more SpiderMultusConfigs that require a selected master NIC, **Then** the webhook injects the matching `spidernet.io/<master>-nic` resource unless the Pod already declares that resource.
6. **Given** provider mode is enabled and `resourceAdvertisement.subENI.rules` is non-empty, **When** a Pod references one or more VLAN SpiderMultusConfigs with nil VLAN ID through its existing Multus network annotations, **Then** the webhook injects `spidernet.io/sub-eni` with quantity equal to the number of eligible referenced SpiderMultusConfigs unless the Pod already declares that resource.

---

### User Story 2 - Keep Node Capacity Status Accurate (Priority: P2)

As a cluster operator, I need each node's advertised master NIC resources and provider-mode auxiliary ENI slot capacity to be visible through node status, so capacity planning and troubleshooting can be performed with standard cluster inspection workflows.

**Why this priority**: Operators need trustworthy capacity visibility to understand scheduling behavior and diagnose exhausted nodes.

**Independent Test**: Configure master NIC advertisement rules and a per-node maximum auxiliary ENI slot count, observe the reported node capacity, create/delete Pods requesting `spidernet.io/<master>-nic` and `spidernet.io/sub-eni`, and verify scheduling availability changes through Kubernetes resource accounting while node status continues to report the healthy schedulable total.

**Acceptance Scenarios**:

1. **Given** a node has selected physical master NICs, **When** the node agent becomes ready and the network resource plugin has registered, **Then** node status advertises `spidernet.io/<master>-nic` with the matching `defaultMaxCount` for each selected physical master NIC.
2. **Given** provider mode is enabled and a node has a configured maximum auxiliary ENI slot capacity, **When** the node agent becomes ready and the network resource plugin has registered, **Then** node status advertises the current healthy schedulable total for `spidernet.io/sub-eni`.
3. **Given** a Pod successfully receives an auxiliary ENI allocation, **When** node status is inspected, **Then** `spidernet.io/sub-eni` still represents the healthy schedulable total and the consumed slot is reflected by the Pod's resource request.
4. **Given** a Pod that held an auxiliary ENI is deleted and release completes, **When** scheduling is evaluated for later Pods, **Then** Kubernetes resource accounting makes the released slot available again without requiring `spidernet.io/sub-eni` to be rewritten as a free-slot counter.

---

### User Story 3 - Release Auxiliary ENI Capacity Reliably (Priority: P3)

As an application owner, I need auxiliary ENI capacity to be returned after my Pod exits, so future Pods are not blocked by stale capacity reservations.

**Why this priority**: Release correctness protects long-running cluster availability and avoids manual operator repair.

**Independent Test**: Create and delete Pods that request `spidernet.io/sub-eni` repeatedly and verify later Pods can be scheduled again once prior Pod requests no longer consume the node's schedulable slot capacity.

**Acceptance Scenarios**:

1. **Given** a Pod has been allocated an auxiliary ENI, **When** the Pod is deleted normally, **Then** the corresponding device allocation is released and another Pod can consume a `spidernet.io/sub-eni` request on that node.
2. **Given** allocation succeeds but Pod startup later fails, **When** cleanup is completed, **Then** the reserved auxiliary ENI slot no longer blocks future Pod scheduling.
3. **Given** release is retried after a transient failure, **When** the underlying capacity has already been returned, **Then** Kubernetes-visible scheduling capacity remains correct and is not double-counted.

### Edge Cases

- `spiderpoolAgent.networkResourcePlugin.enabled=false`: no Spiderpool network resources are advertised and webhook resource injection for these resources remains inactive.
- `spiderpoolController.podResourceInject.enabled=false`: network resources may still be advertised, but the webhook must not automatically inject `spidernet.io/<master>-nic` or `spidernet.io/sub-eni` into Pods.
- Provider mode is disabled: master NIC resource advertisement and injection may still work, but auxiliary ENI capacity advertising, allocation, and injection must remain inactive.
- Configured per-node maximum capacity is zero: Pods requesting `spidernet.io/sub-eni` must not be schedulable to that node.
- A node matches a network resource plugin exclude selector: the spiderpool-agent on that node must not advertise `spidernet.io/sub-eni` or any `spidernet.io/<master>-nic` resources through the network resource plugin.
- A node uses sub-ENI advertisement: the agent must use the first matching `resourceAdvertisement.subENI.rules[]` entry for each resource name.
- `resourceAdvertisement.subENI.rules[].defaultMaxCount` is invalid: the agent must reject the configuration and avoid advertising an unsafe capacity.
- A node's relevant labels change while the spiderpool-agent is running: advertised network resources must converge to the new desired state without restarting the agent.
- A node becomes newly excluded while resources are advertised: the agent must stop advertising network resources on that node without requiring a restart.
- A previously excluded node no longer matches any exclude selector: the agent must resume advertising eligible network resources on that node without requiring a restart.
- A node has multiple matching NIC name rules: the final advertised NIC set must be deterministic and documented, with `excludeInterfaces` taking precedence over `includeInterfaces` within the selected rule evaluation.
- No NIC name rule matches a node: no master NIC resources are advertised on that node.
- A NIC name rule omits `nodeSelector`: that rule applies to all enabled nodes.
- A NIC name rule matches no physical NICs on a node: the node must still advertise its `spidernet.io/sub-eni` capacity, but no master NIC resource for that rule.
- Pod already declares `spidernet.io/sub-eni`: the webhook must not overwrite, duplicate, or increment the existing resource declaration.
- Pod references no VLAN SpiderMultusConfig with nil VLAN ID: the webhook must not inject `spidernet.io/sub-eni`.
- Pod references multiple VLAN SpiderMultusConfigs with nil VLAN ID: the webhook must count all eligible referenced configs and inject that total as the resource quantity when the Pod lacks the resource.
- Configured capacity changes while the node is running: node status must converge to the new healthy schedulable total without losing track of active Pod requests.
- Kubelet, node agent, or device plugin restarts after allocations exist: `spidernet.io/sub-eni` may temporarily be unavailable or zero until the device plugin re-registers and reports healthy slots; previously allocated Pod-device mappings must be recovered from kubelet-managed allocation state, and new Pods must not schedule until the resource is advertised again.
- Allocation or release returns a transient error: capacity visible to scheduling must not incorrectly admit additional Pods beyond the advertised total and already-bound requests.
- More Pods request auxiliary ENIs than the cluster can provide: excess Pods must remain pending with diagnosable scheduling information.
- Node status update conflicts or delays: the final reported `spidernet.io/sub-eni` total must converge without exceeding the configured maximum or dropping below zero.
- Clusters use a non-default kubelet root directory: both the device-plugin and plugin-registration host paths must be derived from the configured `kubeletRootDir` rather than assuming `/var/lib/kubelet`.
- The preferred `{kubeletRootDir}/plugins_registry` path is absent on a node: the agent must fall back to `{kubeletRootDir}/device-plugins` and emit diagnostics identifying the selected path.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST allow operators to enable or disable the Spiderpool agent network resource plugin through `spiderpoolAgent.networkResourcePlugin.enabled`.
- **FR-002**: System MUST allow operators to configure a Helm default maximum auxiliary ENI capacity per enabled node through `spiderpoolAgent.networkResourcePlugin.resourceAdvertisement.subENI.rules[].defaultMaxCount`.
- **FR-003**: System MUST publish `spidernet.io/sub-eni` in node status as the current healthy schedulable total capacity for auxiliary ENI slots.
- **FR-004**: System MUST require eligible Pods to request the relevant Spiderpool network resources so Kubernetes scheduling can reject placement when the node cannot satisfy the new Pod.
- **FR-005**: System MUST allocate an auxiliary ENI slot when kubelet asks the device plugin to satisfy a Pod's accepted `spidernet.io/sub-eni` request.
- **FR-006**: System MUST NOT rewrite `spidernet.io/sub-eni` as a real-time free-slot counter after each allocation; consumed capacity MUST be represented by Pod resource requests and kubelet device assignment state.
- **FR-007**: System MUST release reserved auxiliary ENI capacity when the owning Pod is deleted, fails before completion, or otherwise no longer needs the allocation.
- **FR-008**: System MUST make released auxiliary ENI slots available to future Pod requests without increasing `spidernet.io/sub-eni` above the healthy schedulable total.
- **FR-009**: System MUST prevent auxiliary ENI slot scheduling from exceeding the advertised total capacity, even under concurrent Pod creation.
- **FR-010**: System MUST preserve existing behavior for Pods that do not request Spiderpool network resources.
- **FR-011**: System MUST provide clear operator-visible diagnostics when auxiliary ENI capacity is exhausted, allocation fails, release fails, or node status cannot be updated.
- **FR-012**: System MUST reconcile `spidernet.io/sub-eni` after configuration changes, kubelet restarts, node agent restarts, device-plugin restarts, node reboots, and transient update conflicts.
- **FR-013**: System MUST preserve existing Spiderpool API, CRD, Helm, annotation, and webhook behavior unless an explicit compatibility exception is documented.
- **FR-014**: System MUST expose user/operator-facing names, defaults, validation errors, status fields, and examples consistently with existing Spiderpool conventions.
- **FR-015**: System MUST document that real-time free ENI slots, if exposed for troubleshooting, are a separate diagnostic value derived from the advertised total and active Pod requests or allocations, not the meaning of `spidernet.io/sub-eni`.
- **FR-016**: System MUST automatically inject Spiderpool network resources through the existing Pod webhook when `spiderpoolController.podResourceInject.enabled=true` and the corresponding advertised resource is enabled.
- **FR-017**: System MUST NOT inject, overwrite, duplicate, or increment a Spiderpool network resource when the Pod already declares the same resource key.
- **FR-018**: System MUST NOT require a dedicated ENI injection annotation on the Pod for this behavior.
- **FR-019**: System MUST inject `spidernet.io/sub-eni` quantity equal to the number of referenced eligible VLAN SpiderMultusConfigs when the Pod does not already declare the resource.
- **FR-020**: System MUST provide `spiderpoolAgent.networkResourcePlugin.kubeletRootDir` and derive kubelet plugin host paths from that value.
- **FR-021**: System MUST mount both `{kubeletRootDir}/device-plugins` and `{kubeletRootDir}/plugins_registry` into the spiderpool-agent when the network resource plugin is enabled.
- **FR-022**: System MUST select `{kubeletRootDir}/plugins_registry` when it exists and fall back to `{kubeletRootDir}/device-plugins` only when the preferred registration directory is absent.
- **FR-023**: System MUST document and validate the kubelet device-plugin registration path it selects, including operator-visible diagnostics for the selected path and fallback reason.
- **FR-024**: System MUST derive enabled sub-ENI advertised capacity from `spiderpoolAgent.networkResourcePlugin.resourceAdvertisement.subENI.rules[].defaultMaxCount`.
- **FR-024a**: System MUST allow operators to limit sub-ENI advertisement to nodes matching `spiderpoolAgent.networkResourcePlugin.resourceAdvertisement.subENI.rules[].nodeSelector`.
- **FR-025**: System MUST allow operators to select nodes for network resource advertising with Helm `spiderpoolAgent.networkResourcePlugin.devicePluginAffinity.nodeSelector`; nodes that do not match MUST NOT advertise `spidernet.io/sub-eni` or `spidernet.io/<master>-nic` resources through this plugin.
- **FR-026**: System MUST advertise one master NIC extended resource named `spidernet.io/<master>-nic` for each selected physical master NIC on enabled nodes.
- **FR-027**: System MUST advertise all physical master NICs by default when no matching `resourceAdvertisement.masterNIC.rules` restrict the node.
- **FR-028**: System MUST allow Helm `resourceAdvertisement.masterNIC.rules` to select nodes with optional Kubernetes label selector `nodeSelector` and narrow advertised master NIC resources with `includeInterfaces` and `excludeInterfaces` shell-style glob patterns.
- **FR-028a**: System MUST derive each advertised master NIC virtual capacity from `resourceAdvertisement.masterNIC.rules[].defaultMaxCount`, defaulting to `10000` when omitted.
- **FR-029**: System MUST apply `excludeInterfaces` before advertising selected master NIC resources so excluded physical NICs are not published even when they also match `includeInterfaces`.
- **FR-030**: System MUST treat a `resourceAdvertisement.masterNIC.rules` entry without `nodeSelector` as matching all enabled nodes.
- **FR-031**: System MUST automatically reconcile network resource advertisements when relevant Node labels change, without requiring a spiderpool-agent restart.
- **FR-032**: System MUST stop, resume, or update `spidernet.io/sub-eni` and `spidernet.io/<master>-nic` advertisements according to the latest Node labels and Helm NIC rules.
- **FR-033**: System MUST allow non-empty `resourceAdvertisement.masterNIC.rules` to enable master NIC advertisement outside provider mode.
- **FR-034**: System MUST keep `resourceAdvertisement.subENI` inactive unless provider mode is enabled, even when the network resource plugin itself is enabled.
- **FR-035**: System MUST NOT inject a Spiderpool network resource into a Pod when the corresponding advertised resource is disabled.

### Key Entities

- **Network Resource Plugin Configuration**: Operator-defined desired settings under `spiderpoolAgent.networkResourcePlugin`, including enablement, webhook resource injection, `kubeletRootDir`, device plugin node affinity, master NIC advertisement, and auxiliary ENI advertisement.
- **Auxiliary ENI Capacity Configuration**: Operator-defined desired capacity settings for nodes participating in provider-mode auxiliary ENI allocation, including enablement, default maximum per-node capacity, and optional node label selection.
- **Node Auxiliary ENI Status**: The node-visible record of current healthy schedulable auxiliary ENI slot capacity advertised as `spidernet.io/sub-eni`.
- **Pod Auxiliary ENI Request**: A Pod's declared request for one or more `spidernet.io/sub-eni` units that Kubernetes scheduling accounts against the node's advertised total. When injected by the webhook, the quantity equals the number of eligible VLAN SpiderMultusConfigs referenced by the Pod.
- **Eligible VLAN SpiderMultusConfig Reference**: A SpiderMultusConfig already referenced by the Pod's Multus default-network or attachment-network annotations, with VLAN CNI type and nil VLAN ID, used by the webhook to decide whether the Pod needs ENI slot scheduling protection.
- **Auxiliary ENI Allocation Record**: The association between a Pod and the auxiliary ENI capacity reserved or allocated for that Pod, used to release capacity reliably.
- **Master NIC Resource Advertisement Rule**: A Helm rule under `spiderpoolAgent.networkResourcePlugin.resourceAdvertisement.masterNIC.rules` that matches nodes and determines which physical master NIC interface names are advertised as `spidernet.io/<master>-nic` resources.
- **Master NIC Resource**: A scheduler-facing extended resource for a physical master NIC, advertised as `spidernet.io/<master>-nic` with configurable virtual quantity to constrain Pods by master NIC availability without modeling bandwidth.
- **Derived Free ENI Slot Count**: Optional troubleshooting information calculated from healthy schedulable total capacity and active Pod requests or allocations; it must not replace the scheduler-facing `spidernet.io/sub-eni` total.
- **Kubelet Root Directory Configuration**: The operator-configured kubelet root directory used to derive the agent's device-plugin and plugin-registration host path mounts.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In a test cluster with at least one unsuitable node and one suitable node, 100% of Pods requesting Spiderpool network resources are scheduled only to nodes whose advertised resources can satisfy the new request.
- **SC-002**: After kubelet, node agent, or device-plugin restart, `spidernet.io/sub-eni` is re-advertised as the healthy schedulable total within 30 seconds in 95% of observed restart events after the node components are ready.
- **SC-003**: Across 100 concurrent Pod creation attempts requesting `spidernet.io/sub-eni`, scheduled Pods on each node never exceed that node's advertised slot total.
- **SC-004**: After 50 create/delete Pod lifecycle repetitions on the same node, later Pods can again consume the released `spidernet.io/sub-eni` capacity with no stale reservations blocking scheduling.
- **SC-005**: Pods that do not request Spiderpool network resources continue to schedule and start with no observable behavior change.
- **SC-006**: Operators can identify the reason for auxiliary ENI scheduling or allocation failure from standard cluster-visible status or events in 100% of documented exhaustion and failure cases.
- **SC-007**: New installation configuration, node-visible status names, examples, and validation messages use consistent terminology across user-facing surfaces.
- **SC-008**: In webhook tests, 100% of Pods requiring enabled master NIC or auxiliary ENI resources receive the expected resource requests when absent, and 100% of Pods already declaring those resources remain unchanged.
- **SC-009**: In installation rendering tests, the agent mounts both kubelet plugin paths derived from `kubeletRootDir`, and path-selection tests verify preference for `plugins_registry` with fallback to `device-plugins` when the preferred path is absent.
- **SC-010**: In configuration tests, enabled provider-mode nodes use matching `resourceAdvertisement.subENI.rules[].defaultMaxCount`, and negative values fail validation.
- **SC-011**: In NIC advertisement tests, enabled nodes advertise `spidernet.io/<master>-nic` with matching `resourceAdvertisement.masterNIC.rules[].defaultMaxCount` for all selected physical NICs, nodes outside `devicePluginAffinity.nodeSelector` advertise none, and `resourceAdvertisement.masterNIC.rules` include/exclude patterns produce the documented NIC set.
- **SC-012**: In reconciliation tests, updates to relevant Node labels change the advertised `spidernet.io/sub-eni` and `spidernet.io/<master>-nic` resources within 30 seconds in 95% of observed updates without restarting spiderpool-agent.
- **SC-013**: In non-provider-mode tests, `resourceAdvertisement.masterNIC` can advertise and inject `spidernet.io/<master>-nic`, while `resourceAdvertisement.subENI` remains inactive.

## Assumptions

- Master NIC resource advertisement can be used outside provider-mode deployments; auxiliary ENI capacity applies only when provider mode is enabled.
- For webhook-injected Pods, the requested ENI slot quantity equals the number of referenced eligible VLAN SpiderMultusConfigs; user-declared `spidernet.io/sub-eni` quantities are respected as-is.
- The Helm `resourceAdvertisement.subENI.rules[].defaultMaxCount` value is the source of truth for advertised sub-ENI capacity, and `resourceAdvertisement.subENI.rules[].nodeSelector` controls which nodes advertise that capacity when set.
- Existing Pod selection conventions for provider-mode networking will be reused to identify Pods that require auxiliary ENIs.
- ENI slot resource injection is based on the Pod's existing Multus network annotations and the referenced SpiderMultusConfig properties, not on a new ENI-specific Pod annotation.
- Existing operator workflows for Helm configuration, node inspection, events, and troubleshooting remain the primary user-facing surfaces.
- The final Helm values shape is `spiderpoolAgent.networkResourcePlugin.enabled`, `spiderpoolAgent.networkResourcePlugin.kubeletRootDir`, `spiderpoolAgent.networkResourcePlugin.devicePluginAffinity.nodeSelector`, `spiderpoolAgent.networkResourcePlugin.resourceAdvertisement.subENI`, and `spiderpoolAgent.networkResourcePlugin.resourceAdvertisement.masterNIC`. Pod resource injection is controlled by `spiderpoolController.podResourceInject.enabled`.
- Kubernetes device plugin behavior is the governing model: device plugins advertise healthy extended-resource totals, kubelet records Pod-device assignments separately, and the scheduler calculates remaining capacity from node allocatable resources and already-bound Pod resource requests.
- Kubelet path handling must distinguish the current kubelet plugin registration directory `{kubeletRootDir}/plugins_registry/` from the v1beta1 device-plugin API path `{kubeletRootDir}/device-plugins/`; non-default kubelet root directories must be handled explicitly through `kubeletRootDir`.
- Default installations use `kubeletRootDir=/var/lib/kubelet` unless the operator overrides it.
- Network resource advertising runs on every spiderpool-agent node by default because `spiderpoolAgent.networkResourcePlugin.devicePluginAffinity.nodeSelector` defaults to an empty selector, which matches all nodes.
- Master NIC resource advertisement is disabled until `resourceAdvertisement.masterNIC.rules` contains at least one rule.
- Interface patterns in `includeInterfaces` and `excludeInterfaces` use shell-style glob matching, such as `eth*` or `ens[0-9]`.
- Node label changes are treated as dynamic configuration inputs for network resource advertising and should not require an agent restart to take effect.

## References

- Kubernetes Device Plugins documentation: https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/
- Kubernetes Device Plugins documentation, device plugin implementation and deployment sections: v1beta1 device-plugin service path `/var/lib/kubelet/device-plugins/` and registration socket `/var/lib/kubelet/device-plugins/kubelet.sock`
- Kubernetes kubelet defaults: `DefaultKubeletPluginsRegistrationDirName = "plugins_registry"` under the kubelet root directory
- Kubernetes v1.13 changelog: kubelet plugin registration directory moved from `{kubelet_root_dir}/plugins/` to `{kubelet_root_dir}/plugins_registry/`
- Kubernetes kubelet device manager implementation notes: https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/cm/devicemanager/manager.go
