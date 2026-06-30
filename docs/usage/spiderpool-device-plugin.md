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
    devicePluginAffinity:
      nodeSelector:
        matchExpressions:
          - key: node-role.kubernetes.io/control-plane
            operator: DoesNotExist
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

- `devicePluginAffinity.nodeSelector`: selects nodes that advertise Spiderpool network resources. Empty selector matches all nodes. Use `matchLabels` and `matchExpressions` operators such as `In`, `NotIn`, `Exists`, and `DoesNotExist` to express inclusion or exclusion.
- `nodeSelector`: Kubernetes label selector for nodes selected by the rule. Empty selector matches all nodes. Use `matchLabels` and `matchExpressions` operators such as `In`, `NotIn`, `Exists`, and `DoesNotExist` to express inclusion or exclusion.
- `defaultMaxCount`: total virtual capacity advertised for each selected master NIC. The default is `10000`.
- `includeInterfaces`: selects interface names with shell-style glob patterns such as `eth*` or `ens[0-9]`.
- `excludeInterfaces`: removes interfaces selected by the same rule and takes precedence over `includeInterfaces`.
- `masterNIC.rules[]`: enables master NIC resource advertisement when at least one rule is configured. Empty rules disable this advertisement.
- `networkResourcePlugin.enabled`: enables the overall Device Plugin feature. When disabled, no network resources are advertised.
- `spiderpoolController.podResourceInject.enabled`: enables the Pod webhook to read referenced SpiderMultusConfigs and inject the corresponding master NIC resource requests.

When `masterNIC.rules` is empty, Spiderpool does not advertise master NIC resources. When a rule omits `includeInterfaces`, it selects all discovered physical master NICs on matching nodes.

When resource injection is enabled, the webhook inspects the SpiderMultusConfigs referenced by `v1.multus-cni.io/default-network` and `k8s.v1.cni.cncf.io/networks`. References to ordinary NetworkAttachmentDefinitions that are not backed by SpiderMultusConfig are ignored. For Macvlan, IPvlan, VLAN, and IPoIB configurations, it injects `spidernet.io/<master>-nic: 1` into the first container's resource requests and limits. Duplicate master names are injected only once. If a configuration creates a bond from multiple master interfaces, the webhook injects one resource for every bond member so that the selected node must provide all of them.

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
  serverUrl: "http://iaas-network-provider.example.svc:80"

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

- `iaasNetworkProvider.serverUrl`: service address of IaaS Network Provider. Without provider mode, Sub-ENI scheduling does not take effect.
- `subENI.rules[]`: array of Sub-ENI resource advertisement rules. Empty rules disable Sub-ENI advertisement.
- `subENI.rules[].resourceName`: extended resource name advertised to Kubernetes. Keep the default `spidernet.io/sub-eni` unless you have a specific reason to change it.
- `subENI.rules[].defaultMaxCount`: default total auxiliary ENI capacity per node.
- `subENI.rules[].nodeSelector`: optional Kubernetes label selector. When set, only matching nodes advertise that Sub-ENI resource. It supports `matchLabels` and `matchExpressions`.
- `spiderpoolController.podResourceInject.enabled`: enables webhook injection of `spidernet.io/sub-eni` requests for eligible Pods.

When `spiderpoolController.podResourceInject.enabled=true`, the webhook automatically injects `spidernet.io/sub-eni` into a Pod when:

- IaaS Network Provider mode is enabled.
- The Pod references a VLAN SpiderMultusConfig without `vlanID`.
- The Pod does not already declare the resource.

The injected quantity equals the number of eligible VLAN SpiderMultusConfigs referenced by the Pod. See [IaaS Network Provider](./iaas-network-provider.md) for complete provider-mode configuration.

## Quick start

The following steps verify master NIC name scheduling only. For a Sub-ENI count scheduling quick start, see [IaaS Network Provider](./iaas-network-provider.md).

### 1. Prepare Helm values

Create `device-plugin-values.yaml`:

```yaml
spiderpoolAgent:
  networkResourcePlugin:
    enabled: true
    kubeletRootDir: /var/lib/kubelet
    devicePluginAffinity:
      nodeSelector: {}
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
- `devicePluginAffinity.nodeSelector` controls which nodes advertise Device Plugin resources. Leave it empty to match all nodes, or use `matchExpressions` to exclude nodes.
- Replace `eth1` in `masterNIC.rules` with the physical interface used for scheduling. Adjust `defaultMaxCount` only when the advertised virtual capacity should differ from `10000`.
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
      "spidernet.io/eth2-nic": "10k",
      "spidernet.io/sub-eni": "0"
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
      "spidernet.io/eth1-nic": "10k",
      "spidernet.io/sub-eni": "0"
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
- Resources can disappear temporarily after kubelet or spiderpool-agent restarts and return after Device Plugin registration completes.

### Master NIC resource is missing

- Run `ip link show` on the node and confirm the interface name exists.
- Check `masterNIC.rules`, `nodeSelector`, `includeInterfaces`, and `excludeInterfaces`.
- Check whether the node matches `devicePluginAffinity.nodeSelector`.
- Virtual interfaces and common CNI interfaces are not automatically advertised as physical master NICs.

### Pod remains Pending

```bash
kubectl describe pod <pod-name>
kubectl get events \
  --field-selector involvedObject.kind=Pod,involvedObject.name=<pod-name> \
  --sort-by=.metadata.creationTimestamp
```

- `Insufficient spidernet.io/<master>-nic`: no candidate node provides the requested master NIC resource.
- Pod does not contain `spidernet.io/<master>-nic`: confirm `podResourceInject.enabled=true`, `networkResourcePlugin.enabled=true`, and `masterNIC.rules` is not empty; verify that the Pod references a Macvlan, IPvlan, VLAN, or IPoIB SpiderMultusConfig with a non-empty `master`.
- If the network annotation contains an incorrect SpiderMultusConfig namespace or name, that reference is treated as an ordinary NetworkAttachmentDefinition and no master NIC resource is injected for it.
