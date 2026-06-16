# Cilium VXLAN and RDMA MTU

**English** | [**简体中文**](./cilium-rdma-mtu-zh_CN.md)

## Introduction

In a cluster that has both RDMA-capable nodes and non-RDMA nodes, Pods may use Cilium as the default network and use Spiderpool for an Underlay network interface. When Cilium runs in VXLAN mode, traffic between Pods on RDMA and non-RDMA nodes can fail if the effective MTU is different on the two paths.

This problem is commonly observed as small packets working while larger packets, such as TLS handshake or certificate-delivery traffic, are dropped. In the reported case, the Istio sidecar could discover the control plane, but TLS connections to Istiod could not complete. The root cause is an MTU black hole: packets larger than the safe encapsulated size are sent, but the path cannot forward them after VXLAN encapsulation.

Refer to [#Issue 5677](https://github.com/spidernet-io/spiderpool/issues/5677). To solve this problem, Spiderpool provides `vethMTU`, which configures the MTU of the Pod `veth0` device created by the coordinator plugin. The default value is `1500`. In affected Cilium VXLAN and RDMA mixed-node environments, set it to a smaller value such as `1400` so packets can pass through the encapsulated path.

## How to Configure

1. When installing Spiderpool using Helm, configure the default coordinator `vethMTU`:

    ```shell
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    helm repo update spiderpool
    kubectl create namespace spiderpool
    helm install spiderpool spiderpool/spiderpool -n spiderpool --set coordinator.vethMTU=1400
    ```

    > - `vethMTU` must be greater than `0`.
    > - The default value is `1500`.
    > - If you are a user in China, you can specify the parameter `--set global.imageRegistryOverride=ghcr.m.daocloud.io` to use a domestic image source.

2. After installation, check the SpiderCoordinator configuration:

    ```shell
    ~# kubectl get spidercoordinators.spiderpool.spidernet.io default -o yaml
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderCoordinator
    metadata:
      name: default
    spec:
      detectGateway: false
      detectIPConflict: false
      hijackCIDR:
      - 169.254.0.0/16
      hostRPFilter: 0
      hostRuleTable: 500
      mode: auto
      podCIDRType: cilium
      podDefaultRouteNIC: ""
      podMACPrefix: ""
      podRPFilter: 0
      tunePodRoutes: true
      vethLinkAddress: ""
      vethMTU: 1400
    status:
      phase: Synced
    ```

3. If Spiderpool is already installed, patch the default SpiderCoordinator:

    ```shell
    kubectl patch spidercoordinators default --type='merge' -p '{"spec": {"vethMTU": 1400}}'
    ```

4. Step 3 changes the cluster default. If you only want to configure one Spiderpool network, set `vethMTU` in the `SpiderMultusConfig` coordinator section:

    ```shell
    MACVLAN_MASTER_INTERFACE="eth0"
    cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderMultusConfig
    metadata:
      name: macvlan-conf
      namespace: kube-system
    spec:
      cniType: macvlan
      macvlan:
        master:
        - ${MACVLAN_MASTER_INTERFACE}
      coordinator:
        vethMTU: 1400
    EOF
    ```

## Verification

After creating the application, check whether the Pod `veth0` device uses the configured MTU:

```shell
~# kubectl exec -it <pod-name> -n <namespace> -- ip link show veth0
```

The output should contain `mtu 1400`:

```text
3: veth0@if123: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1400 qdisc noqueue state UP mode DEFAULT group default
```

Then verify application traffic that previously failed, such as TLS access between Pods on RDMA and non-RDMA nodes.
