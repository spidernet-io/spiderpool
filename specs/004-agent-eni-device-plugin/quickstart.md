# Quickstart: Agent ENI Device Plugin

## Prerequisites

- Provider mode is configured through `iaasNetworkProvider.serverUrl`.
- Workloads that need auxiliary ENIs use the existing Spiderpool provider-mode network selection flow.
- The cluster allows spiderpool-agent to mount kubelet plugin directories on each node. Default installs use `/var/lib/kubelet/device-plugins` and `/var/lib/kubelet/plugins_registry`.

## Enable

Set Helm values:

```yaml
iaasNetworkProvider:
  serverUrl: "http://provider.example:8080"
  eniDevPlugin:
    enabled: true
    resourceName: spidernet.io/eni-slot
    maxSlotsPerNode: 8
    kubeletRootDir: /var/lib/kubelet
    injectPodENIResources: true
```

Install or upgrade Spiderpool with the updated chart.

## Verify Node Capacity

Check node allocatable resources:

```bash
kubectl get node <node> -o jsonpath='{.status.allocatable.spidernet\.io/eni-slot}{"\n"}'
```

Expected result: the value is the healthy schedulable total slot capacity, for example `8`.

## Verify Kubelet Plugin Paths

Confirm the agent DaemonSet mounts both paths derived from `kubeletRootDir`:

```bash
kubectl -n kube-system get ds spiderpool-agent -o yaml | grep -E "device-plugins|plugins_registry"
```

Expected result: both `{kubeletRootDir}/device-plugins` and `{kubeletRootDir}/plugins_registry` are mounted when `eniDevPlugin.enabled=true`. Agent logs should identify the selected path, preferring `plugins_registry` and falling back to `device-plugins` only when the preferred path is absent.

## Verify Pod Scheduling

Create enough eligible Pods to consume all slots on one node. A Pod that references two eligible VLAN SpiderMultusConfigs should contain:

```yaml
resources:
  limits:
    spidernet.io/eni-slot: "2"
```

Expected result: webhook injection uses the number of eligible referenced VLAN SpiderMultusConfigs as the resource quantity when the Pod does not already declare `spidernet.io/eni-slot`. Once existing Pod requests consume the node's advertised total, later Pods requesting `spidernet.io/eni-slot` remain pending or schedule to other nodes with remaining capacity.

## Verify Restart Recovery

Restart spiderpool-agent or kubelet on a test node.

Expected results:

- `spidernet.io/eni-slot` may disappear or become zero temporarily.
- The device plugin re-registers after components are ready.
- Node allocatable returns to the healthy schedulable total.
- Existing Pods do not cause the resource to be double-counted.
- New Pods are scheduled only when Kubernetes resource accounting shows remaining capacity.

## Troubleshooting

- If `spidernet.io/eni-slot` is missing, check spiderpool-agent logs for device plugin registration errors, selected kubelet plugin path, and fallback reason.
- If the node uses a non-default kubelet root, set `iaasNetworkProvider.eniDevPlugin.kubeletRootDir` to that root so both plugin directories are derived correctly.
- If Pods are pending, describe the Pod and node to confirm whether all advertised slots are consumed by already-bound Pod requests.
- Do not interpret `Node.status.allocatable["spidernet.io/eni-slot"]` as free slots. It is the total healthy schedulable capacity.
