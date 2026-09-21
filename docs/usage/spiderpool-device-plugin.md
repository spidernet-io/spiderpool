# Spiderpool Device Plugin

**English** | [**简体中文**](./spiderpool-device-plugin-zh_CN.md)

## Background

In Spiderpool secondary networks, a SpiderMultusConfig uses `master` to identify the host physical interface that a Pod network must bind to, such as `eth1`, `ens5`, or a VLAN sub-interface. This is not just a configuration detail. It determines whether the Pod can complete secondary network setup on the target node at all.

Kubernetes' default scheduler mainly evaluates general-purpose resources such as CPU and memory. It does not know which host interface a Pod depends on, and it does not know how much cloud network capacity remains on each node for secondary networking. As a result, a Pod can be scheduled first and then fail later during CNI setup or cloud resource allocation.

These failures usually appear in two scenarios. First, physical interface naming or layout differs between nodes, for example when some nodes have `eth1` and others do not. Second, in public cloud environments, the number of ENIs (Elastic Network Interfaces) and auxiliary ENIs that can be attached to each node is usually limited by the instance type and cloud quota. If a Pod lands on a node without enough remaining ENI capacity, the Pod fails to obtain its network resource and cannot start. The failed attempt also generates unnecessary cloud API calls. Because some cloud platforms rate-limit these APIs, such calls can consume rate-limit capacity, delay later valid requests, and amplify failures during large Pod rollouts.

Spiderpool Device Plugin runs in `spiderpool-agent` and registers these network constraints as Kubernetes extended resources through the device plugin API. After a Pod requests these resources, the scheduler filters unsuitable nodes before Pod network setup begins. The Device Plugin only provides scheduling and kubelet admission constraints. It does not configure Pod interfaces or allocate and release cloud resources.

## Features

### Scheduling by master NIC name

Spiderpool discovers physical interfaces and advertises each selected interface to `Node.status.allocatable`:

```text
status:
  allocatable:
    spidernet.io/<master>-nic: 10000
```

For example, a node with `eth1` advertises:

```text
status:
  allocatable:
    spidernet.io/eth1-nic: 10000
```

A Pod that requests `spidernet.io/eth1-nic: 1` can only schedule to nodes that have `eth1` and advertise this resource. The default quantity `10000` comes from `masterNIC.rules[].defaultMaxCount` and is a virtual capacity representing NIC presence. It does not represent bandwidth, queue count, or a Pod limit.

This feature does not require IaaS Network Provider mode. It applies to Macvlan, IPvlan, VLAN, and other networks that require the target node to have the configured `master` interface.

How to configure:

```yaml
spiderpoolAgent:
  networkResourcePlugin:
    enabled: true
    resourceAdvertisement:
      masterNIC:
        rules:
          - nodeSelector:
              matchLabels:
                kubernetes.io/os: linux
            defaultMaxCount: 10000
            includeInterfaces:
              - "eth1"
              - "ens*"
            excludeInterfaces:
              - "ens10"

spiderpoolController:
  podResourceInject:
    enabled: true
```

What the configuration means:

- `nodeSelector`: Kubernetes label selector for nodes selected by the rule. Empty selector matches all nodes. Use `matchLabels` and `matchExpressions` operators such as `In`, `NotIn`, `Exists`, and `DoesNotExist` to express inclusion or exclusion.
- `defaultMaxCount`: total virtual capacity advertised for each selected master NIC. The default is `10000`.
- `includeInterfaces`: selects interface names with shell-style glob patterns such as `eth*` or `ens[0-9]`.
- `excludeInterfaces`: removes interfaces selected by the same rule and takes precedence over `includeInterfaces`.
- `masterNIC.rules[]`: enables master NIC resource advertisement when at least one rule is configured. Empty rules disable this advertisement.
- `networkResourcePlugin.enabled`: enables the overall Device Plugin feature. When disabled, no network resources are advertised.
- `spiderpoolController.podResourceInject.enabled`: enables the Pod webhook to read referenced SpiderMultusConfigs and inject the corresponding master NIC resource requests.

When `masterNIC.rules` is empty, Spiderpool does not advertise master NIC resources. When a rule omits `includeInterfaces`, it selects all discovered physical master NICs on matching nodes.

When resource injection is enabled, the webhook inspects the SpiderMultusConfigs referenced by `v1.multus-cni.io/default-network` and `k8s.v1.cni.cncf.io/networks`. References to ordinary NetworkAttachmentDefinitions that are not backed by SpiderMultusConfig are ignored. For Macvlan, IPvlan, VLAN, eni-vlan, and IPoIB configurations, it injects `spidernet.io/<master>-nic: 1` into the first container's resource requests and limits. Duplicate master names are injected only once. If a configuration creates a bond from multiple master interfaces, the webhook injects one resource for every bond member so that the selected node must provide all of them.

If the workload already declares a master NIC resource, the webhook preserves the user-provided value.

### Scheduling by Sub-ENI count

In provider mode, Spiderpool advertises the total auxiliary ENI slot capacity of a node to `Node.status.allocatable`:

```text
status:
  allocatable:
    spidernet.io/sub-eni: 10
```

`defaultMaxCount` defines the total per-node capacity advertised by each enabled agent.

When a Pod's `spidernet.io/sub-eni` request exceeds the remaining schedulable capacity, the scheduler does not place it on that node.

`Node.status.allocatable["spidernet.io/sub-eni"]` is the healthy total advertised by kubelet, not a remaining count. Kubernetes derives remaining capacity from this total and the resource requests of scheduled Pods.

How to configure:

```yaml
iaasNetworkProvider:
  enabled: true
  service:
    name: "iaas-network-provider"
    namespace: "iaas-network-provider-system"
    port: 443
  tls:
    caSecret: "iaas-network-provider-tls"

spiderpoolAgent:
  networkResourcePlugin:
    enabled: true
    resourceAdvertisement:
      subENI:
        rules:
          - resourceName: spidernet.io/sub-eni
            defaultMaxCount: 10
            nodeSelector:
              matchLabels:
                key: value

spiderpoolController:
  podResourceInject:
    enabled: true
```

What the configuration means:

- `iaasNetworkProvider.enabled` and `iaasNetworkProvider.service`: enable IaaS Network Provider mode and point Spiderpool to the provider Service (name/namespace/port). Without provider mode, Sub-ENI scheduling does not take effect.
- `subENI.rules[]`: array of Sub-ENI resource advertisement rules. Empty rules disable Sub-ENI advertisement. Each rule corresponds to one extended resource.
- `subENI.rules[].resourceName`: extended resource name advertised to Kubernetes. Defaults to `spidernet.io/sub-eni` and must follow the `<domain>/<resource>` qualified-name format. Different rules may advertise different resource names, for example one per pool mode.
- `subENI.rules[].defaultMaxCount`: total schedulable Sub-ENI capacity advertised on each node matched by the rule. Plan it against the instance type or parent NIC Sub-ENI limit and the actual cloud quota. It is a static number: it does not follow the pool's `readyIPCount` and is not a live remaining count queried from the cloud.
- `subENI.rules[].nodeSelector`: optional Kubernetes label selector. When set, only matching nodes advertise that Sub-ENI resource. It supports `matchLabels` and `matchExpressions`.
- `spiderpoolController.podResourceInject.enabled`: enables webhook injection of `spidernet.io/<master>-nic` requests for eligible Pods. Sub-ENI resources are never injected automatically, because the webhook cannot reliably determine at admission time whether a Pod will end up on a node-level pool or a global pool.

Rule matching semantics:

- When several rules share the same `resourceName`, a node uses the first rule it matches, in rule order. This lets you assign different capacities to different node groups, for example 16 for large instances and 4 for small ones.
- When a node matches rules with different `resourceName` values, all of those resources are advertised on the node at the same time. The scheduler accounts for each resource name independently; there is no shared-quota linkage between different resource names.

Pods that need Sub-ENI capacity scheduling must declare the resource request explicitly in their container resources. Extended resources require `requests` equal to `limits` with an integer value; a Pod with a single secondary NIC normally declares `1`, and a Pod with several secondary NICs declares the actual number of Sub-ENIs it occupies. Pods that do not declare the resource are not constrained by Sub-ENI capacity and the scheduler reserves nothing for them — make sure every workload that consumes Sub-ENIs declares it, otherwise the accounting is skewed. See [IaaS Network Provider](./iaas-network-provider.md) for complete provider-mode configuration.

#### Pool modes and node planning

The IaaS Network Provider supports two pool placement modes, and both consume the same physical Sub-ENI slots of the host:

- **Node-level pool**: pinned to a single node via `spec.nodeName`; the provider prewarms Sub-ENIs ahead of time, so Pods start fast. Suitable for workloads with a relatively stable scale that need fast Pod startup.
- **Global pool**: no `spec.nodeName`; Sub-ENIs are created on demand and reused through a sticky cache. Suitable for workloads with fluctuating replica counts that need elastic scaling.

You can dedicate different nodes to each mode, or let one node serve both. When both modes run on the same host, they share that host's total Sub-ENI capacity, so the advertised capacities must be planned within the host total.

The recommended deployment is to dedicate node groups per mode and advertise a distinct resource name for each group. Label the nodes first:

```bash
# node-level pool nodes
kubectl label node node1 spiderpool.io/node-pool=true --overwrite
# global pool nodes
kubectl label node node2 spiderpool.io/global-pool=true --overwrite
```

Then configure one rule per mode:

```yaml
spiderpoolAgent:
  networkResourcePlugin:
    enabled: true
    resourceAdvertisement:
      subENI:
        rules:
          - resourceName: spidernet.io/node-pool-sub-eni
            defaultMaxCount: 6
            nodeSelector:
              matchLabels:
                spiderpool.io/node-pool: "true"
          - resourceName: spidernet.io/global-pool-sub-eni
            defaultMaxCount: 4
            nodeSelector:
              matchLabels:
                spiderpool.io/global-pool: "true"
```

Workloads that use a node-level pool declare `spidernet.io/node-pool-sub-eni`, and workloads that use a global pool declare `spidernet.io/global-pool-sub-eni`. Dedicated node groups keep the two capacity budgets isolated: prewarm consumption on node-pool nodes can never squeeze the on-demand headroom of global-pool nodes.

The node labels only control resource advertisement and scheduling. They do not decide the pool mode: a SpiderIPPool with a non-empty `spec.nodeName` is a node-level prewarm pool, and a global pool must not set that field.

When a node must serve both modes, choose one of two accounting patterns:

- **Static split (two resource names)**: apply both labels to the node so it matches both rules; it then advertises both resources. Because different resource names are accounted independently, split the host total statically between the two capacities — for example, a host limit of 10 becomes `node-pool-sub-eni: 6` plus `global-pool-sub-eni: 4`. Never let the sum of advertised capacities exceed the physical Sub-ENI limit of the host. This pattern gives clear isolation, but capacity cannot flow between the modes.
- **Shared quota (one resource name)**: advertise a single resource such as `spidernet.io/sub-eni` with `defaultMaxCount` equal to the host total, and have workloads of both modes declare that same resource. Capacity then flexes between the modes on demand. Note that prewarming happens ahead of Pod creation: prewarmed Sub-ENIs occupy real slots before any Pod request is counted, so while a node-level pool is under-utilized the scheduler view is optimistic. Keep the prewarm size close to the expected concurrent Pod count to minimize the gap.

## Quick start

The following steps verify master NIC name scheduling only. For a Sub-ENI count scheduling quick start, see [IaaS Network Provider](./iaas-network-provider.md).

### 1. Prepare Helm values

Create `device-plugin-values.yaml`:

```yaml
spiderpoolAgent:
  networkResourcePlugin:
    enabled: true
    kubeletRootDir: /var/lib/kubelet
    resourceAdvertisement:
      masterNIC:
        rules:
          - defaultMaxCount: 10000
            includeInterfaces:
              - "eth1"

spiderpoolController:
  podResourceInject:
    enabled: true
```

Notes:

- `kubeletRootDir` must match the kubelet root directory on the nodes.
- Replace `eth1` in `masterNIC.rules` with the physical interface used for scheduling. Adjust `defaultMaxCount` only when the advertised virtual capacity should differ from `10000`. Use the rule's `nodeSelector` to limit which nodes advertise the resource; an empty selector matches all nodes.
- `podResourceInject.enabled` enables automatic injection of the master NIC resource from the referenced SpiderMultusConfig.

### 2. Install or update Spiderpool

For a new installation:

```bash
helm repo add spiderpool https://spidernet-io.github.io/spiderpool
helm repo update
helm install spiderpool spiderpool/spiderpool \
  --namespace kube-system \
  --create-namespace \
  --values device-plugin-values.yaml \
  --wait
```

For an existing installation:

```bash
helm upgrade spiderpool spiderpool/spiderpool \
  --namespace kube-system \
  --reuse-values \
  --values device-plugin-values.yaml \
  --wait
```

Change `spiderpool` and `kube-system` if the release uses different names.

### 3. Verify the installation

Confirm that spiderpool-agent is running:

```bash
kubectl get pod -n kube-system -l app.kubernetes.io/component=spiderpool-agent -o wide
```

Inspect the Spiderpool resources advertised by each node:

```bash
kubectl get nodes -o json | jq '[.items[] | {name: .metadata.name, allocatable: .status.allocatable}]'
```

Expected results:

```
[
  {
    "name": "spiderpool0522022016-control-plane",
    "allocatable": {
      "cpu": "56",
      "ephemeral-storage": "860377048Ki",
      "hugepages-1Gi": "0",
      "hugepages-2Mi": "0",
      "memory": "131885828Ki",
      "pods": "110",
      "spidernet.io/eth1-nic": "10k",
      "spidernet.io/eth2-nic": "10k"
    }
  },
  {
    "name": "spiderpool0522022016-worker",
    "allocatable": {
      "cpu": "56",
      "ephemeral-storage": "860377048Ki",
      "hugepages-1Gi": "0",
      "hugepages-2Mi": "0",
      "memory": "131885828Ki",
      "pods": "110",
      "spidernet.io/eth1-nic": "10k"
    }
  }
]
```

If resources are missing, inspect registration logs:

```bash
kubectl logs -n kube-system \
  -l app.kubernetes.io/component=spiderpool-agent \
  --tail=200 | grep "network resource plugin"
```

### 4. Verify master NIC name scheduling

Create a SpiderMultusConfig whose `master` is `eth1`:

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: master-nic-network
  namespace: default
spec:
  cniType: macvlan
  disableIPAM: true
  macvlan:
    master:
      - eth1
```

```bash
kubectl apply -f master-nic-network.yaml
```

Create a Pod that references this network. The Pod does not declare `spidernet.io/eth1-nic`; the webhook adds it automatically:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: master-nic-scheduling
  annotations:
    k8s.v1.cni.cncf.io/networks: default/master-nic-network
spec:
  containers:
    - name: test
      image: busybox:1.36
      command: ["sh", "-c", "sleep 3600"]
```

```bash
kubectl apply -f master-nic-pod.yaml
```

Watch timestamped Pod events:

```bash
kubectl get events \
  --field-selector involvedObject.kind=Pod,involvedObject.name=master-nic-scheduling \
  --sort-by=.metadata.creationTimestamp \
  -o custom-columns='TIME:.metadata.creationTimestamp,TYPE:.type,REASON:.reason,MESSAGE:.message' \
  --watch
```

A successful placement produces a `Scheduled` event. Check the selected node:

```bash
kubectl get pod master-nic-scheduling -o wide
```

Verify the injected request and confirm that the selected node advertises the resource:

```bash
kubectl get pod master-nic-scheduling \
  -o jsonpath='{.spec.containers[0].resources.requests.spidernet\.io/eth1-nic}{"\n"}'

NODE_NAME=$(kubectl get pod master-nic-scheduling -o jsonpath='{.spec.nodeName}')
kubectl get node "${NODE_NAME}" \
  -o jsonpath='{.status.allocatable.spidernet\.io/eth1-nic}{"\n"}'
```

The expected outputs are `1` for the Pod request and `10000` for the node capacity.

If no node advertises the resource, the Pod remains `Pending`, and Events report `FailedScheduling` and `Insufficient spidernet.io/eth1-nic`.

## Troubleshooting

### Nodes advertise no resources

Check configuration and component status:

```bash
helm get values spiderpool -n kube-system
kubectl get daemonset spiderpool-agent -n kube-system
kubectl logs -n kube-system -l app.kubernetes.io/component=spiderpool-agent --tail=200
```

- Confirm `networkResourcePlugin.enabled=true`.
- Confirm `kubeletRootDir` matches the node configuration.
- Confirm the agent mounts `{kubeletRootDir}/device-plugins` and `{kubeletRootDir}/plugins_registry`.
- Resources can disappear temporarily after kubelet or spiderpool-agent restarts and return after Device Plugin registration completes. New Pods cannot schedule to the node during that window.

### Sub-ENI resource is missing

- Sub-ENI resources are advertised only when IaaS Network Provider mode is enabled (`iaasNetworkProvider.enabled=true` with a valid Service configuration).
- Confirm `subENI.rules` is not empty and the node labels match the rule's `nodeSelector`.
- When several rules share one `resourceName`, only the first matching rule applies to a node; check the rule order if the capacity is unexpected.

### Master NIC resource is missing

- Run `ip link show` on the node and confirm the interface name exists.
- Check `masterNIC.rules`, `nodeSelector`, `includeInterfaces`, and `excludeInterfaces`.
- Virtual interfaces and common CNI interfaces are not automatically advertised as physical master NICs.

### Pod remains Pending

```bash
kubectl describe pod <pod-name>
kubectl get events \
  --field-selector involvedObject.kind=Pod,involvedObject.name=<pod-name> \
  --sort-by=.metadata.creationTimestamp
```

- `Insufficient spidernet.io/<master>-nic`: no candidate node provides the requested master NIC resource.
- `Insufficient spidernet.io/...sub-eni`: the requests of scheduled Pods already reach `defaultMaxCount` on every candidate node, or the declared resource name does not match any rule's `resourceName`. Check `kubectl describe node <node>` under `Allocated resources`. Being blocked at scheduling time is the intended behavior — no cloud API call is wasted.
- Pod does not contain `spidernet.io/<master>-nic`: confirm `podResourceInject.enabled=true`, `networkResourcePlugin.enabled=true`, and `masterNIC.rules` is not empty; verify that the Pod references a Macvlan, IPvlan, VLAN, eni-vlan, or IPoIB SpiderMultusConfig with a non-empty `master`.
- If the network annotation contains an incorrect SpiderMultusConfig namespace or name, that reference is treated as an ordinary NetworkAttachmentDefinition and no master NIC resource is injected for it.
