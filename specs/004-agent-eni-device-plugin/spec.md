# Feature Specification: Agent ENI Device Plugin

**Feature Branch**: `005-agent-eni-device-plugin`

**Created**: 2026-06-09

**Status**: Draft

**Input**: User description: "Implement a feature based on the current provider integration. Add a device plugin to spiderpool-agent to assist auxiliary elastic network interface allocation and release. In provider mode, this should prevent Pods from being scheduled onto nodes without available auxiliary elastic network interfaces. A Helm values configuration, such as the maximum number of auxiliary elastic network interfaces available per node, should be synchronized into node status. When a Pod is created and scheduled, the system should determine whether the node has sufficient auxiliary elastic network interfaces. If capacity is available, it should call the device plugin to allocate one and then update the available count in node status."

## Clarifications

### Session 2026-06-09

- Q: What does `spidernet.io/eni-slot` represent in node status after kubelet, node, or device-plugin restart? -> A: `spidernet.io/eni-slot` represents the current healthy schedulable total capacity reported through the Kubernetes device plugin resource model. It is not a real-time remaining/free slot counter. Kubernetes scheduling determines remaining schedulable capacity by subtracting already-bound Pod resource requests from the node's allocatable total.
- Q: How should restart recovery keep ENI slot scheduling correct? -> A: After kubelet or device-plugin restart, the device plugin must re-register with kubelet and report the healthy slot list again. Kubelet restores previously allocated device assignments from its device manager checkpoint, and new Pods requesting `spidernet.io/eni-slot` are not schedulable until the resource is registered and advertised again.
- Q: What source material defines this behavior? -> A: Kubernetes device plugin documentation states that a plugin reports devices to kubelet, kubelet advertises the resource through node status, unhealthy devices reduce allocatable while capacity remains unchanged, and free/unallocated resources must be derived from allocatable resources together with Pod resource allocation data. The kubelet device manager implementation also records Pod-device assignments in a checkpoint and notes that device plugin resource capacity can temporarily drop to zero until the plugin re-registers after kubelet restart.
- Q: How should the webhook decide whether to inject the ENI slot resource? -> A: Do not add a dedicated Pod annotation for ENI slot injection. When provider mode and the ENI slot device plugin are enabled, the existing Pod webhook must inspect the Pod's current Multus default-network and attachment-network annotations. If any referenced SpiderMultusConfig is VLAN type and its VLAN ID is nil, the webhook injects `spidernet.io/eni-slot` into the Pod resources unless the Pod already declares the same resource key.
- Q: How many ENI slot resources should the webhook inject when a Pod references multiple eligible VLAN SpiderMultusConfigs? -> A: If the Pod does not already declare `spidernet.io/eni-slot`, inject a quantity equal to the number of referenced eligible VLAN SpiderMultusConfigs.

### Session 2026-06-10

- Q: Which kubelet socket path should the ENI slot device plugin use on current Kubernetes? -> A: Treat `{kubeletRootDir}/plugins_registry/` as the current kubelet plugin registration directory for external plugin registration, while recognizing that the v1beta1 device-plugin API constants still define `/var/lib/kubelet/device-plugins/` and `/var/lib/kubelet/device-plugins/kubelet.sock` for the legacy device-plugin gRPC service. The implementation plan must explicitly choose the registration mechanism and mount the matching host path, especially when kubelet uses a non-default root directory.
- Q: How should Spiderpool configure and select kubelet plugin paths for compatibility? -> A: Spiderpool must expose a configurable `kubeletRootDir`, mount both `{kubeletRootDir}/device-plugins` and `{kubeletRootDir}/plugins_registry` into the agent, prefer `{kubeletRootDir}/plugins_registry` when it exists, and fall back to `{kubeletRootDir}/device-plugins` when the new registration directory is absent.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Schedule Pods Only Where Auxiliary ENIs Are Available (Priority: P1)

As a cluster operator using provider mode, I need workloads that require auxiliary elastic network interfaces to request `spidernet.io/eni-slot` and be scheduled only onto nodes with enough remaining schedulable slot capacity, so Pod startup does not fail after placement on an unsuitable node.

**Why this priority**: This is the primary user value. It prevents invalid scheduling decisions before allocation is attempted.

**Independent Test**: Configure auxiliary ENI slot capacity for a mixed-capacity node pool, create Pods referencing eligible VLAN SpiderMultusConfigs, verify the webhook injects `spidernet.io/eni-slot` only when needed, and verify Pods are admitted only to nodes where allocatable slot capacity minus already-bound Pod requests can satisfy the new request.

**Acceptance Scenarios**:

1. **Given** provider mode is enabled and a node advertises enough `spidernet.io/eni-slot` allocatable capacity after existing Pod requests are considered, **When** a Pod requests one slot, **Then** the Pod can be scheduled to that node and Kubernetes accounts for the request against that node.
2. **Given** provider mode is enabled and all candidate nodes have no remaining schedulable `spidernet.io/eni-slot` capacity after existing Pod requests are considered, **When** a Pod requests one slot, **Then** the Pod remains unscheduled instead of being placed on a node that cannot allocate the interface.
3. **Given** a cluster contains nodes with different advertised `spidernet.io/eni-slot` allocatable totals, **When** multiple Pods request ENI slots, **Then** scheduling decisions respect each node's remaining schedulable capacity independently.
4. **Given** provider mode and the ENI slot device plugin are enabled, **When** a Pod references one or more VLAN SpiderMultusConfigs with nil VLAN ID through its existing Multus network annotations, **Then** the webhook injects `spidernet.io/eni-slot` with quantity equal to the number of eligible referenced SpiderMultusConfigs unless the Pod already declares that resource.

---

### User Story 2 - Keep Node Capacity Status Accurate (Priority: P2)

As a cluster operator, I need each node's auxiliary ENI slot capacity to be visible through node status, so capacity planning and troubleshooting can be performed with standard cluster inspection workflows.

**Why this priority**: Operators need trustworthy capacity visibility to understand scheduling behavior and diagnose exhausted nodes.

**Independent Test**: Configure a per-node maximum auxiliary ENI slot count, observe the reported node capacity, allocate and release Pods requesting `spidernet.io/eni-slot`, and verify scheduling availability changes through Kubernetes resource accounting while node status continues to report the healthy schedulable total.

**Acceptance Scenarios**:

1. **Given** a node has a configured maximum auxiliary ENI slot capacity, **When** the node agent becomes ready in provider mode and its device plugin has registered, **Then** node status advertises the current healthy schedulable total for `spidernet.io/eni-slot`.
2. **Given** a Pod successfully receives an auxiliary ENI allocation, **When** node status is inspected, **Then** `spidernet.io/eni-slot` still represents the healthy schedulable total and the consumed slot is reflected by the Pod's resource request.
3. **Given** a Pod that held an auxiliary ENI is deleted and release completes, **When** scheduling is evaluated for later Pods, **Then** Kubernetes resource accounting makes the released slot available again without requiring `spidernet.io/eni-slot` to be rewritten as a free-slot counter.

---

### User Story 3 - Release Auxiliary ENI Capacity Reliably (Priority: P3)

As an application owner, I need auxiliary ENI capacity to be returned after my Pod exits, so future Pods are not blocked by stale capacity reservations.

**Why this priority**: Release correctness protects long-running cluster availability and avoids manual operator repair.

**Independent Test**: Create and delete Pods that request `spidernet.io/eni-slot` repeatedly and verify later Pods can be scheduled again once prior Pod requests no longer consume the node's schedulable slot capacity.

**Acceptance Scenarios**:

1. **Given** a Pod has been allocated an auxiliary ENI, **When** the Pod is deleted normally, **Then** the corresponding device allocation is released and another Pod can consume a `spidernet.io/eni-slot` request on that node.
2. **Given** allocation succeeds but Pod startup later fails, **When** cleanup is completed, **Then** the reserved auxiliary ENI slot no longer blocks future Pod scheduling.
3. **Given** release is retried after a transient failure, **When** the underlying capacity has already been returned, **Then** Kubernetes-visible scheduling capacity remains correct and is not double-counted.

### Edge Cases

- Provider mode is disabled: auxiliary ENI capacity advertising and allocation behavior must remain inactive unless explicitly enabled.
- Configured per-node maximum capacity is zero: Pods requesting `spidernet.io/eni-slot` must not be schedulable to that node.
- Pod already declares `spidernet.io/eni-slot`: the webhook must not overwrite, duplicate, or increment the existing resource declaration.
- Pod references no VLAN SpiderMultusConfig with nil VLAN ID: the webhook must not inject `spidernet.io/eni-slot`.
- Pod references multiple VLAN SpiderMultusConfigs with nil VLAN ID: the webhook must count all eligible referenced configs and inject that total as the resource quantity when the Pod lacks the resource.
- Configured capacity changes while the node is running: node status must converge to the new healthy schedulable total without losing track of active Pod requests.
- Kubelet, node agent, or device plugin restarts after allocations exist: `spidernet.io/eni-slot` may temporarily be unavailable or zero until the device plugin re-registers and reports healthy slots; previously allocated Pod-device mappings must be recovered from kubelet-managed allocation state, and new Pods must not schedule until the resource is advertised again.
- Allocation or release returns a transient error: capacity visible to scheduling must not incorrectly admit additional Pods beyond the advertised total and already-bound requests.
- More Pods request auxiliary ENIs than the cluster can provide: excess Pods must remain pending with diagnosable scheduling information.
- Node status update conflicts or delays: the final reported `spidernet.io/eni-slot` total must converge without exceeding the configured maximum or dropping below zero.
- Clusters use a non-default kubelet root directory: both the device-plugin and plugin-registration host paths must be derived from the configured `kubeletRootDir` rather than assuming `/var/lib/kubelet`.
- The preferred `{kubeletRootDir}/plugins_registry` path is absent on a node: the agent must fall back to `{kubeletRootDir}/device-plugins` and emit diagnostics identifying the selected path.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST allow operators to enable or disable auxiliary ENI schedulable capacity behavior for provider-mode deployments.
- **FR-002**: System MUST allow operators to configure the maximum auxiliary ENI capacity available per node through installation configuration.
- **FR-003**: System MUST publish `spidernet.io/eni-slot` in node status as the current healthy schedulable total capacity for auxiliary ENI slots.
- **FR-004**: System MUST require eligible Pods to request `spidernet.io/eni-slot` so Kubernetes scheduling can reject placement when the node's advertised total minus already-bound Pod requests cannot satisfy the new Pod.
- **FR-005**: System MUST allocate an auxiliary ENI slot when kubelet asks the device plugin to satisfy a Pod's accepted `spidernet.io/eni-slot` request.
- **FR-006**: System MUST NOT rewrite `spidernet.io/eni-slot` as a real-time free-slot counter after each allocation; consumed capacity MUST be represented by Pod resource requests and kubelet device assignment state.
- **FR-007**: System MUST release reserved auxiliary ENI capacity when the owning Pod is deleted, fails before completion, or otherwise no longer needs the allocation.
- **FR-008**: System MUST make released auxiliary ENI slots available to future Pod requests without increasing `spidernet.io/eni-slot` above the healthy schedulable total.
- **FR-009**: System MUST prevent auxiliary ENI slot scheduling from exceeding the advertised total capacity, even under concurrent Pod creation.
- **FR-010**: System MUST preserve existing provider-mode behavior for Pods that do not request auxiliary ENIs.
- **FR-011**: System MUST provide clear operator-visible diagnostics when auxiliary ENI capacity is exhausted, allocation fails, release fails, or node status cannot be updated.
- **FR-012**: System MUST reconcile `spidernet.io/eni-slot` after configuration changes, kubelet restarts, node agent restarts, device-plugin restarts, node reboots, and transient update conflicts.
- **FR-013**: System MUST preserve existing Spiderpool API, CRD, Helm, annotation, and webhook behavior unless an explicit compatibility exception is documented.
- **FR-014**: System MUST expose user/operator-facing names, defaults, validation errors, status fields, and examples consistently with existing Spiderpool conventions.
- **FR-015**: System MUST document that real-time free ENI slots, if exposed for troubleshooting, are a separate diagnostic value derived from the advertised total and active Pod requests or allocations, not the meaning of `spidernet.io/eni-slot`.
- **FR-016**: System MUST automatically inject `spidernet.io/eni-slot` through the existing Pod webhook when provider mode and the ENI slot device plugin are enabled and the Pod currently references at least one VLAN-type SpiderMultusConfig whose VLAN ID is nil.
- **FR-017**: System MUST NOT inject, overwrite, duplicate, or increment `spidernet.io/eni-slot` when the Pod already declares the same resource key.
- **FR-018**: System MUST NOT require a dedicated ENI injection annotation on the Pod for this behavior.
- **FR-019**: System MUST inject `spidernet.io/eni-slot` quantity equal to the number of referenced eligible VLAN SpiderMultusConfigs when the Pod does not already declare the resource.
- **FR-020**: System MUST provide an installation configuration for `kubeletRootDir` and derive kubelet plugin host paths from that value.
- **FR-021**: System MUST mount both `{kubeletRootDir}/device-plugins` and `{kubeletRootDir}/plugins_registry` into the spiderpool-agent when the ENI slot device plugin is enabled.
- **FR-022**: System MUST select `{kubeletRootDir}/plugins_registry` when it exists and fall back to `{kubeletRootDir}/device-plugins` only when the preferred registration directory is absent.
- **FR-023**: System MUST document and validate the kubelet device-plugin registration path it selects, including operator-visible diagnostics for the selected path and fallback reason.

### Key Entities

- **Auxiliary ENI Capacity Configuration**: Operator-defined desired capacity settings for nodes participating in provider-mode auxiliary ENI allocation, including enablement and maximum per-node capacity.
- **Node Auxiliary ENI Status**: The node-visible record of current healthy schedulable auxiliary ENI slot capacity advertised as `spidernet.io/eni-slot`.
- **Pod Auxiliary ENI Request**: A Pod's declared request for one or more `spidernet.io/eni-slot` units that Kubernetes scheduling accounts against the node's advertised total. When injected by the webhook, the quantity equals the number of eligible VLAN SpiderMultusConfigs referenced by the Pod.
- **Eligible VLAN SpiderMultusConfig Reference**: A SpiderMultusConfig already referenced by the Pod's Multus default-network or attachment-network annotations, with VLAN CNI type and nil VLAN ID, used by the webhook to decide whether the Pod needs ENI slot scheduling protection.
- **Auxiliary ENI Allocation Record**: The association between a Pod and the auxiliary ENI capacity reserved or allocated for that Pod, used to release capacity reliably.
- **Derived Free ENI Slot Count**: Optional troubleshooting information calculated from healthy schedulable total capacity and active Pod requests or allocations; it must not replace the scheduler-facing `spidernet.io/eni-slot` total.
- **Kubelet Root Directory Configuration**: The operator-configured kubelet root directory used to derive the agent's device-plugin and plugin-registration host path mounts.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In a test cluster with at least one exhausted node and one available node, 100% of Pods requesting `spidernet.io/eni-slot` are scheduled only to nodes whose advertised total minus already-bound Pod requests can satisfy the new request.
- **SC-002**: After kubelet, node agent, or device-plugin restart, `spidernet.io/eni-slot` is re-advertised as the healthy schedulable total within 30 seconds in 95% of observed restart events after the node components are ready.
- **SC-003**: Across 100 concurrent Pod creation attempts requesting `spidernet.io/eni-slot`, scheduled Pods on each node never exceed that node's advertised slot total.
- **SC-004**: After 50 create/delete Pod lifecycle repetitions on the same node, later Pods can again consume the released `spidernet.io/eni-slot` capacity with no stale reservations blocking scheduling.
- **SC-005**: Pods that do not request auxiliary ENIs continue to schedule and start with no observable behavior change in provider-mode deployments.
- **SC-006**: Operators can identify the reason for auxiliary ENI scheduling or allocation failure from standard cluster-visible status or events in 100% of documented exhaustion and failure cases.
- **SC-007**: New installation configuration, node-visible status names, examples, and validation messages use consistent terminology across user-facing surfaces.
- **SC-008**: In webhook tests, 100% of Pods referencing eligible VLAN SpiderMultusConfigs receive `spidernet.io/eni-slot` with quantity equal to the eligible reference count when absent, and 100% of Pods already declaring that resource remain unchanged.
- **SC-009**: In installation rendering tests, the agent mounts both kubelet plugin paths derived from `kubeletRootDir`, and path-selection tests verify preference for `plugins_registry` with fallback to `device-plugins` when the preferred path is absent.

## Assumptions

- The feature applies only to provider-mode deployments and is disabled or inert outside that mode.
- For webhook-injected Pods, the requested ENI slot quantity equals the number of referenced eligible VLAN SpiderMultusConfigs; user-declared `spidernet.io/eni-slot` quantities are respected as-is.
- Per-node maximum capacity is treated as the source of truth for advertised capacity; actual provider-side limits are expected to be reflected by operator configuration or provider integration checks.
- Existing Pod selection conventions for provider-mode networking will be reused to identify Pods that require auxiliary ENIs.
- ENI slot resource injection is based on the Pod's existing Multus network annotations and the referenced SpiderMultusConfig properties, not on a new ENI-specific Pod annotation.
- Existing operator workflows for Helm configuration, node inspection, events, and troubleshooting remain the primary user-facing surfaces.
- Kubernetes device plugin behavior is the governing model: device plugins advertise healthy extended-resource totals, kubelet records Pod-device assignments separately, and the scheduler calculates remaining capacity from node allocatable resources and already-bound Pod resource requests.
- Kubelet path handling must distinguish the current kubelet plugin registration directory `{kubeletRootDir}/plugins_registry/` from the v1beta1 device-plugin API path `{kubeletRootDir}/device-plugins/`; non-default kubelet root directories must be handled explicitly through `kubeletRootDir`.
- Default installations use `kubeletRootDir=/var/lib/kubelet` unless the operator overrides it.

## References

- Kubernetes Device Plugins documentation: https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/
- Kubernetes Device Plugins documentation, device plugin implementation and deployment sections: v1beta1 device-plugin service path `/var/lib/kubelet/device-plugins/` and registration socket `/var/lib/kubelet/device-plugins/kubelet.sock`
- Kubernetes kubelet defaults: `DefaultKubeletPluginsRegistrationDirName = "plugins_registry"` under the kubelet root directory
- Kubernetes v1.13 changelog: kubelet plugin registration directory moved from `{kubelet_root_dir}/plugins/` to `{kubelet_root_dir}/plugins_registry/`
- Kubernetes kubelet device manager implementation notes: https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/cm/devicemanager/manager.go
