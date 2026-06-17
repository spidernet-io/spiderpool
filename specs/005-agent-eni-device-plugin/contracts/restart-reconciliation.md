# Contract: Restart Reconciliation

## Kubelet Restart

Expected behavior:

1. Kubelet removes device plugin sockets under the relevant kubelet plugin directory during startup.
2. spiderpool-agent detects the socket lifecycle change or failed stream and re-registers the network resource plugin.
3. Kubelet restores prior Pod-device assignments from its device manager checkpoint.
4. New Pods requesting `spidernet.io/sub-eni` remain unschedulable until kubelet advertises the resource again.

Acceptance signal: after kubelet and spiderpool-agent are ready, `spidernet.io/sub-eni` is visible in node allocatable as the healthy schedulable total within the feature's restart budget.

## Plugin Path Selection

Expected behavior:

1. The agent reads `kubeletRootDir` from rendered configuration.
2. It derives `{kubeletRootDir}/plugins_registry` and `{kubeletRootDir}/device-plugins`.
3. It selects `plugins_registry` when the directory exists.
4. It falls back to `device-plugins` only when `plugins_registry` is absent.
5. It logs the selected path and fallback reason.

Acceptance signal: path-selection tests cover default root, non-default root, preferred path present, and preferred path absent.

## spiderpool-agent or Device Plugin Restart

Expected behavior:

1. The new agent process starts the device plugin service before registration.
2. It registers the same resource name and stable slot IDs.
3. It reports the healthy slot list.
4. Kubelet keeps already assigned devices associated with active Pods and resumes allocation for later Pods.

Acceptance signal: active Pods keep their assigned resource; later Pods can schedule when capacity remains.

## Node Reboot

Expected behavior:

1. Existing running containers are restarted according to normal Kubernetes/runtime behavior.
2. spiderpool-agent starts and registers the resource after kubelet and the agent Pod are running.
3. Capacity is advertised from the configured slot total and current health.
4. Provider/IPAM cleanup or recovery follows existing Spiderpool lifecycle behavior.

Acceptance signal: the node never advertises a free-slot counter and never schedules more Pods requesting slots than the advertised total.

## Dynamic Node Metadata Reconciliation

Expected behavior:

- Increasing `resourceAdvertisement.subENI.rules[].defaultMaxCount` adds healthy slot IDs after agent configuration reconciliation.
- Decreasing `resourceAdvertisement.subENI.rules[].defaultMaxCount` lowers future schedulable capacity but must not break already-running Pods.
- A decrease below currently allocated slot count prevents new slot-consuming Pods until active requests fall below the new total.
- Changing labels so the node no longer matches `devicePluginAffinity.nodeSelector` causes the agent to stop advertising `spidernet.io/sub-eni` and `spidernet.io/<master>-nic` without restarting.
- Changing labels so the node matches `devicePluginAffinity.nodeSelector` causes the agent to resume advertising eligible resources without restarting.
- Label changes that alter matching `resourceAdvertisement.masterNIC.rules` cause the agent to recompute selected physical NIC resources and push an update only when the final resource set changes.
- Invalid `resourceAdvertisement.subENI.rules[].defaultMaxCount` values emit diagnostics and must not advertise an unsafe capacity.

Implementation contract:

1. The agent watches or caches only its own Node object.
2. It recomputes the desired resource set after relevant Node label changes.
3. It compares the new desired resource set with the last advertised set.
4. It notifies kubelet through the device plugin stream only when the resource set changed.
5. It may use a low-frequency resync as a fallback, but must not read the Node object from the Kubernetes API for every `ListAndWatch` report.

Acceptance signal: relevant Node label and annotation updates converge in kubelet-visible resources within 30 seconds in 95% of observed updates without restarting spiderpool-agent.
