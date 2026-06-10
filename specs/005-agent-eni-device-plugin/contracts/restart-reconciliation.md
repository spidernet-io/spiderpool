# Contract: Restart Reconciliation

## Kubelet Restart

Expected behavior:

1. Kubelet removes device plugin sockets under the relevant kubelet plugin directory during startup.
2. spiderpool-agent detects the socket lifecycle change or failed stream and re-registers the ENI slot device plugin.
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

## Configuration Change

Expected behavior:

- Increasing `maxSlotsPerNode` adds healthy slot IDs after agent reconciliation.
- Decreasing `maxSlotsPerNode` lowers future schedulable capacity but must not break already-running Pods.
- A decrease below currently allocated slot count prevents new slot-consuming Pods until active requests fall below the new total.
