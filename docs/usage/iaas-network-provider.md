# IaaS Network Provider

**English** | [**简体中文**](./iaas-network-provider-zh_CN.md)

## Introduction

Spiderpool can integrate with a generic IaaS Network Provider. When Spiderpool allocates or releases Pod IP addresses, it calls the configured provider to bind or unbind the corresponding IaaS-side IP resources on a cloud platform, and configures the Pod interface with the MAC address and VLAN ID returned by the cloud.

This is useful for public cloud or private cloud environments where an IP address assigned by Spiderpool must also be registered, bound, or programmed in an external cloud network system before the Pod can use it. Typical use cases include:

- Allocating auxiliary IP resources (sub-ENIs) from a cloud platform.
- Binding an IP to a node, ENI, auxiliary network interface, or VLAN sub-interface.
- Returning cloud-specific attributes such as the Pod interface MAC address and VLAN ID to Spiderpool.
- Releasing the IaaS-side IP binding when Spiderpool releases the Pod IP.

The IaaS Network Provider is an HTTP service. Spiderpool only defines the API contract and does not depend on a specific cloud vendor implementation. An IaaS-backed `SpiderIPPool` works in one of two placement modes:

- **Node-level pool**: pinned to a single node via `spec.nodeName`; the provider prewarms sub-ENIs on that node ahead of time, so Pods start fast without any synchronous cloud call.
- **Global pool**: no `spec.nodeName`; one pool serves a workload whose Pods spread across many nodes, allocating in realtime with a sticky sub-ENI cache.

For the design details behind these modes, the allocation call flow, the timeout model, and the provider API contract, see [IaaS Network Provider Architecture](../concepts/iaas-network-provider.md).

Terminology used in this document:

- ENI: Elastic Network Interface
- Sub-ENI: Secondary Elastic Network Interface
- VLAN: Virtual Local Area Network

## Prerequisites

Before installing, prepare the following:

1. **An IaaS Network Provider deployment.** The provider implements the [Spiderpool provider API contract](../concepts/iaas-network-provider.md#api-contract) and is exposed through a Kubernetes Service. It must be installed **before** Spiderpool, because at install/upgrade time Helm looks up the provider TLS Secret to snapshot its CA certificate.

2. **IaaS-side network resources.** Platform administrators need to:

    - Create a VPC subnet and bind it to the node's elastic network interface. For example, bind the VPC subnet `172.91.0.0/24` to the physical NIC `eth1` on the nodes.
    - Confirm the maximum number of auxiliary ENIs that can be bound per node; it is used for the Sub-ENI scheduling capacity below.
    - It is recommended that the extension elastic network interfaces on each node do not have IP addresses configured, to avoid communication issues caused by inconsistent return paths.

3. **VLAN CNI.** Provider mode uses [vlan-cni](https://github.com/spidernet-io/vlan-cni) — a VLAN CNI plugin developed by Spiderpool based on the upstream community cni-plugin project — to create VLAN sub-interfaces for Pods with the VLAN ID and MAC address allocated by the cloud. It is shipped in the Spiderpool plugins image and installed via `plugins.installVlanCNI`.

4. **Dual-stack planning (optional).** Dual-stack Pods are supported through *paired pools*: an IPv4 pool and an IPv6 pool referencing each other via the `ipam.spidernet.io/pair-pool` annotation, so both families are provisioned atomically on one sub-ENI. Plan the IPv6 subnet together with the IPv4 subnet if you need dual-stack.

## Install and configure Spiderpool

### Helm values

Create `iaas-values.yaml`:

```yaml
ipam:
  enableGatewayDetection: false
  enableIPConflictDetection: false

plugins:
  installVlanCNI: true

iaasNetworkProvider:
  enabled: true
  service:
    name: "iaas-network-provider"
    namespace: "iaas-network-provider-system"
    port: 8443
  tls:
    caSecret: "iaas-network-provider-tls"
  httpRequestTimeout: "50s"

spiderpoolController:
  podResourceInject:
    enabled: true

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
      subENI:
        rules:
          - resourceName: spidernet.io/sub-eni
            defaultMaxCount: 256
```

What the configuration means:

- `iaasNetworkProvider.enabled`: turns IaaS Network Provider integration on (default `false`). When disabled, all other `iaasNetworkProvider` settings are ignored.
- `iaasNetworkProvider.service`: the Kubernetes Service (name/namespace/port) of the provider. When enabled, `service.name` must be non-empty (default `iaas-network-provider`).
- `iaasNetworkProvider.tls`: the connection uses one-way TLS — Spiderpool verifies the provider serving certificate. At install/upgrade time Helm looks up the provider TLS Secret (`tls.caSecret` in `service.namespace`) and copies only its `ca.crt` into a local Secret `iaas-provider-ca`. For GitOps or `helm template` (where `lookup` is unavailable), set `tls.ca` (base64 PEM CA bundle) explicitly; it takes precedence over the lookup. `tls.insecureSkipVerify=true` skips verification and is only a gradual-rollout fallback. If the provider is uninstalled and reinstalled (new CA), re-run `helm upgrade` on Spiderpool to refresh the snapshot.
- `iaasNetworkProvider.httpRequestTimeout`: how long Spiderpool waits for a single provider call. The default `50s` covers the provider worst case; see the [timeout model](../concepts/iaas-network-provider.md#http-request-timeout-model) before changing it.
- `plugins.installVlanCNI` must be enabled: provider mode configures Pod interfaces through VLAN CNI.
- `ipam.enableGatewayDetection` and `ipam.enableIPConflictDetection` must be disabled. Unlike the traditional order of calling CNI first and IPAM afterwards, this mode must call IPAM first to obtain the IaaS network attributes before CNI configures the Pod network, so gateway detection and IP conflict detection cannot work.
- `spiderpoolAgent.networkResourcePlugin` enables device-plugin resource advertisement so the scheduler can constrain placement:
    - `subENI.rules[]` advertises `spidernet.io/sub-eni` with per-node auxiliary ENI capacity (`defaultMaxCount`, set to the real per-node sub-ENI limit). Empty rules disable Sub-ENI advertisement. An optional `nodeSelector` (supporting `matchLabels` and `matchExpressions`) limits which nodes advertise the resource.
    - `masterNIC.rules[]` advertises `spidernet.io/<master>-nic` on nodes that own the physical NIC selected by `includeInterfaces`/`excludeInterfaces` (shell-style globs, e.g. `eth*`; exclusion wins), so workloads only land on nodes that actually have the NIC named by the SpiderMultusConfig `master` field. `defaultMaxCount` (default `10000`) is a virtual capacity that only indicates NIC existence. See [Spiderpool Device Plugin](./spiderpool-device-plugin.md) for details.
- `spiderpoolController.podResourceInject.enabled` lets the webhook inject the `spidernet.io/<master>-nic` request into eligible Pods automatically. `spidernet.io/sub-eni` is **never** injected automatically — users must declare it on the Pod for the scheduler to enforce ENI capacity.

### Install

```bash
helm upgrade --install spiderpool spiderpool/spiderpool \
  --namespace kube-system \
  --values iaas-values.yaml \
  --wait
```

### Verify the installation

Check that the feature is active:

```bash
# 1. The ConfigMap renders a non-empty provider service name only when enabled
kubectl get configmap spiderpool-conf -n kube-system -o yaml | grep -A3 iaasNetworkProvider

# 2. The agent log shows the IaaS client is initialized
kubectl logs -n kube-system -l app.kubernetes.io/component=spiderpool-agent | grep "IaaS"
```

Expect `iaasNetworkProvider.service.name` to be non-empty and the agent log to contain `IaaS provider configured and client created successfully`. If you see `IaaS provider configuration validation failed`, verify the `iaasNetworkProvider.service` and `iaasNetworkProvider.tls` values.

Check that nodes advertise the scheduling resources:

```bash
kubectl get nodes -o custom-columns='NAME:.metadata.name,SUB_ENI:.status.allocatable.spidernet\.io/sub-eni,MASTER_NIC:.status.allocatable.spidernet\.io/eth1-nic'
```

Matching nodes show `SUB_ENI=256` and `MASTER_NIC=10000`; nodes that do not satisfy the rules show `<none>`.

## Create a SpiderMultusConfig

Provider mode uses a VLAN SpiderMultusConfig. The VLAN ID is allocated dynamically by the cloud, so **do not set `vlanID`** — vlan-cni queries the local spiderpool-agent through a Unix socket during Pod creation for the VLAN ID and MAC address allocated from the IaaS, then creates the VLAN sub-interface in the Pod network namespace accordingly.

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: iaas-vlan-config
  namespace: spiderpool
spec:
  cniType: vlan
  vlan:
    master:
      - eth1
    ippools:
      ipv4:
        - pool-node1        # reference the SpiderIPPool(s) created below
```

Notes:

- `master` is required and must match the physical NIC name on the target nodes, as well as the NIC selected by `masterNIC.rules[].includeInterfaces` (here `eth1`). Keep the name consistent across candidate nodes, or rely on [master NIC scheduling](./spiderpool-device-plugin.md) to keep workloads off nodes without it.
- Never set `vlanID` in the `vlan` section; a statically configured VLAN ID would conflict with the cloud-allocated one and break Pod networking.

## Create SpiderIPPools

An IaaS pool is a normal `SpiderIPPool` plus the `ipam.spidernet.io/iaas-provider` annotation (the value names your provider). `subnet` must match the VPC subnet on the cloud platform. The placement mode is derived from the pool shape: with `spec.nodeName` it is a node-level pool, without it a global pool. The mode is fixed for the pool's lifetime — the webhook rejects adding or removing `spec.nodeName` afterwards.

### Node-level pool

A node-level pool is pinned to exactly one node (the webhook rejects multiple `nodeName` entries) and **requires** the `ipam.spidernet.io/parent-nic` annotation, naming the single guest-OS parent NIC of the pool on that node:

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: pool-node1
  annotations:
    ipam.spidernet.io/iaas-provider: huaweicloud
    ipam.spidernet.io/parent-nic: eth1
spec:
  ipVersion: 4
  subnet: 172.91.0.0/24
  gateway: 172.91.0.1
  ips:
    - 172.91.0.100-172.91.0.120
  nodeName:
    - node1
```

After creation, check:

1. **Marker label synced**: the mutating webhook mirrors the `iaas-provider` annotation into a label of the same name.

    ```bash
    kubectl get spiderippool pool-node1 -o jsonpath='{.metadata.labels}'
    ```

2. **Parent NIC published**: the spiderpool-agent on the pool's node resolves the annotated NIC's MAC address locally and publishes both to `status.parentNic`, from which the provider reads the cloud-side parent port MAC.

    ```bash
    kubectl get spiderippool pool-node1 -o jsonpath='{.status.parentNic}'
    # {"mac":"fa:16:3e:11:22:33","name":"eth1"}
    ```

3. **Prewarm completed**: a node-level pool is strictly prewarm-only — Spiderpool only allocates addresses that the provider has already prepared and published into `status.ipMetaData`. Wait until the provider flushes metadata entries before starting Pods; otherwise Pod creation fails with an IP-used-out error and kubelet keeps retrying until prewarmed addresses appear.

    ```bash
    kubectl get spiderippool pool-node1 -o jsonpath='{.status.ipMetaData}'
    ```

### Global pool

A global pool sets **no** `spec.nodeName` and serves one workload whose Pods spread across nodes. The `parent-nic` annotation is optional: with it, the allocation path resolves the parent NIC MAC directly by name on the Pod's node; without it, the MAC falls back to the SpiderMultusConfig `master` interface.

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: pool-global
  annotations:
    ipam.spidernet.io/iaas-provider: huaweicloud
    ipam.spidernet.io/parent-nic: eth1   # optional; the NIC must carry this name on every node
spec:
  ipVersion: 4
  subnet: 172.91.0.0/24
  gateway: 172.91.0.1
  ips:
    - 172.91.0.121-172.91.0.180
  podAffinity:
    matchLabels:
      app: my-app
```

A global pool needs **no prewarming** — the first Pod on a node triggers a synchronous provider call to create the sub-ENI, and later Pods on that node reuse it from the cache with no cloud call. `podAffinity` is recommended so the pool is dedicated to one workload. Also check the marker label after creation as above; `status.parentNic` is never published on global pools.

### Dual-stack paired pools (optional)

For dual-stack, create a v4 and a v6 pool referencing each other with `ipam.spidernet.io/pair-pool`, so one sub-ENI carries both families atomically:

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: pool-node1-v4
  annotations:
    ipam.spidernet.io/iaas-provider: huaweicloud
    ipam.spidernet.io/pair-pool: pool-node1-v6
    ipam.spidernet.io/parent-nic: eth1
spec:
  ipVersion: 4
  subnet: 172.91.0.0/24
  ips: ["172.91.0.100-172.91.0.120"]
  nodeName: ["node1"]
---
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: pool-node1-v6
  annotations:
    ipam.spidernet.io/iaas-provider: huaweicloud
    ipam.spidernet.io/pair-pool: pool-node1-v4
    ipam.spidernet.io/parent-nic: eth1
spec:
  ipVersion: 6
  subnet: fd00:172:91::/112
  ips: ["fd00:172:91::100-fd00:172:91::120"]
  nodeName: ["node1"]
---
```

The webhook validates the pairing (mutual references, matching shape, v6 capacity covering v4). For a paired set, `status.parentNic` and prewarm metadata live on the primary (v4) pool.

## Create Pods

### Pod using a node-level pool

The Pod references the VLAN SpiderMultusConfig through the Multus annotation and declares one `spidernet.io/sub-eni` request so the scheduler enforces auxiliary ENI capacity. The webhook injects `spidernet.io/eth1-nic` automatically when `podResourceInject` is enabled:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: iaas-demo
  annotations:
    k8s.v1.cni.cncf.io/networks: spiderpool/iaas-vlan-config
    ipam.spidernet.io/ippool: '{"ipv4": ["pool-node1"]}'
spec:
  containers:
    - name: demo
      image: busybox:1.36
      command: ["sh", "-c", "sleep 3600"]
      resources:
        requests:
          spidernet.io/sub-eni: "1"
        limits:
          spidernet.io/sub-eni: "1"
```

Watch the scheduling events while the Pod starts:

```bash
kubectl get events \
  --field-selector involvedObject.kind=Pod,involvedObject.name=iaas-demo \
  --sort-by=.metadata.creationTimestamp --watch
```

When capacity is available the events show `Scheduled`; if the combined `sub-eni` requests exceed node capacity, excess Pods stay `Pending` with `FailedScheduling` and `Insufficient spidernet.io/sub-eni`.

### Deployment using a global pool

A multi-replica workload spreading across nodes fits the global pool. Match the pool's `podAffinity` labels:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
      annotations:
        k8s.v1.cni.cncf.io/networks: spiderpool/iaas-vlan-config
        ipam.spidernet.io/ippool: '{"ipv4": ["pool-global"]}'
    spec:
      containers:
        - name: app
          image: busybox:1.36
          command: ["sh", "-c", "sleep 3600"]
          resources:
            requests:
              spidernet.io/sub-eni: "1"
            limits:
              spidernet.io/sub-eni: "1"
```

The first replica on each node triggers one synchronous provider call; subsequent replicas (and future Pods after the earlier ones are deleted) reuse the node's cached sub-ENIs and start without any cloud call.

## Verify the result

After the Pods are running:

```bash
# Pod got an IP from the pool, with the cloud-allocated MAC and VLAN on its interface
kubectl get pod iaas-demo -o wide
kubectl exec iaas-demo -- ip addr show

# The declared and injected scheduling resources
kubectl get pod iaas-demo -o jsonpath='{.spec.containers[0].resources.requests}'

# The pool records the allocation
kubectl get spiderippool pool-node1 -o jsonpath='{.status.allocatedIPCount}'

# The SpiderEndpoint tracks the Pod's allocation details
kubectl get spiderendpoint iaas-demo -o yaml
```

## Troubleshooting

- **Feature not active**: confirm `iaasNetworkProvider.enabled=true` and `iaasNetworkProvider.service.name` is non-empty; check the agent log for `IaaS provider configuration validation failed`.
- **Pod pending with `Insufficient spidernet.io/sub-eni` (or `<master>-nic`)**: confirm `subENI.rules`/`masterNIC.rules` are not empty, check `defaultMaxCount`, `nodeSelector`, `includeInterfaces`/`excludeInterfaces`, and run `ip link show` on the node to confirm the physical NIC exists. If a Pod misses the injected `<master>-nic` request, check `podResourceInject.enabled`; the `sub-eni` request must always be declared by the user.
- **Node-level pool Pod fails with IP-used-out**: the pool has not been prewarmed (or all prewarmed entries are in use) — check `status.ipMetaData` and the provider logs. Pod creation recovers automatically once metadata entries appear.
- **Provider call timeout**: see the [timeout model and error messages](../concepts/iaas-network-provider.md#http-request-timeout-model).
- **Allocation/release semantics** (synchronous allocation, idempotent release, parent NIC MAC lookup): see [special scenario handling](../concepts/iaas-network-provider.md#special-scenario-handling).
