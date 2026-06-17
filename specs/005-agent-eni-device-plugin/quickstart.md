# Quickstart: Agent Network Resource Plugin

## Prerequisites

- Master NIC resource advertisement can run without provider mode.
- Auxiliary ENI resource advertisement requires provider mode configured through `iaasNetworkProvider.serverUrl`.
- Workloads that need auxiliary ENIs use the existing Spiderpool provider-mode network selection flow.
- The cluster allows spiderpool-agent to mount kubelet plugin directories on each node. Default installs use `/var/lib/kubelet/device-plugins` and `/var/lib/kubelet/plugins_registry`.

## Enable

Set Helm values:

```yaml
iaasNetworkProvider:
  serverUrl: "http://provider.example:8080"
spiderpoolAgent:
  networkResourcePlugin:
    enabled: true
    kubeletRootDir: /var/lib/kubelet
    devicePluginAffinity:
      nodeSelector:
        matchExpressions:
        - key: spidernet.io/network-resource
          operator: NotIn
          values:
          - "disabled"
    resourceAdvertisement:
      subENI:
        rules:
        - resourceName: spidernet.io/sub-eni
          defaultMaxCount: 8
          nodeSelector:
            matchLabels:
              key: value
      masterNIC:
        rules:
        - nodeSelector:
            matchLabels:
              spidernet.io/nic-profile: eth
          defaultMaxCount: 10000
          includeInterfaces:
          - "eth*"
        - defaultMaxCount: 5000
          includeInterfaces:
          - "ens[0-9]"
          excludeInterfaces:
          - "ens4"
```

Install or upgrade Spiderpool with the updated chart.

## Verify Node Capacity

Check node allocatable resources:

```bash
kubectl get node <node> -o jsonpath='{.status.allocatable.spidernet\.io/sub-eni}{"\n"}'
```

Expected result: the value is the healthy schedulable total slot capacity, for example `8`.

Check selected master NIC resources:

```bash
kubectl get node <node> -o jsonpath='{.status.allocatable}' | grep 'spidernet.io/'
```

Expected result: selected physical NICs are advertised as `spidernet.io/<master>-nic` resources with the matching rule's `defaultMaxCount`. Nodes that do not match `devicePluginAffinity.nodeSelector` do not advertise Spiderpool network resources.

## Verify Dynamic Node Updates

Exclude and re-enable a node:

```bash
kubectl label node <node> spidernet.io/network-resource=disabled --overwrite
kubectl label node <node> spidernet.io/network-resource-
```

Expected result: the node stops advertising Spiderpool network resources when excluded and resumes advertising eligible resources after the label is removed, without restarting spiderpool-agent.

## Verify Kubelet Plugin Paths

Confirm the agent DaemonSet mounts both paths derived from `kubeletRootDir`:

```bash
kubectl -n kube-system get ds spiderpool-agent -o yaml | grep -E "device-plugins|plugins_registry"
```

Expected result: both `{kubeletRootDir}/device-plugins` and `{kubeletRootDir}/plugins_registry` are mounted when `spiderpoolAgent.networkResourcePlugin.enabled=true`. Agent logs should identify the selected path, preferring `plugins_registry` and falling back to `device-plugins` only when the preferred path is absent.

## Verify Pod Scheduling

Create enough eligible Pods to consume all slots on one node. A Pod that references two eligible VLAN SpiderMultusConfigs should contain:

```yaml
resources:
  limits:
    spidernet.io/sub-eni: "2"
```

Expected result: webhook injection uses the number of eligible referenced VLAN SpiderMultusConfigs as the resource quantity when the Pod does not already declare `spidernet.io/sub-eni`. Once existing Pod requests consume the node's advertised total, later Pods requesting `spidernet.io/sub-eni` remain pending or schedule to other nodes with remaining capacity.

A Pod that requires a selected master NIC should contain the matching master NIC resource:

```yaml
resources:
  limits:
    spidernet.io/eth1-nic: "1"
```

Expected result: webhook injection adds the selected `spidernet.io/<master>-nic` resource when enabled and when the Pod does not already declare it. The scheduler places the Pod only on nodes advertising that master NIC resource.

## Verify Restart Recovery

Restart spiderpool-agent or kubelet on a test node.

Expected results:

- `spidernet.io/sub-eni` may disappear or become zero temporarily.
- The device plugin re-registers after components are ready.
- Node allocatable returns to the healthy schedulable total.
- Existing Pods do not cause the resource to be double-counted.
- New Pods are scheduled only when Kubernetes resource accounting shows remaining capacity.

## Troubleshooting

- If `spidernet.io/sub-eni` is missing, check spiderpool-agent logs for device plugin registration errors, selected kubelet plugin path, and fallback reason.
- If the node uses a non-default kubelet root, set `spiderpoolAgent.networkResourcePlugin.kubeletRootDir` to that root so both plugin directories are derived correctly.
- If node label changes do not affect advertised resources, check agent logs for local Node watch/reconcile diagnostics and whether the computed resource set actually changed.
- If Pods are pending, describe the Pod and node to confirm whether all advertised slots are consumed by already-bound Pod requests.
- Do not interpret `Node.status.allocatable["spidernet.io/sub-eni"]` as free slots. It is the total healthy schedulable capacity.
