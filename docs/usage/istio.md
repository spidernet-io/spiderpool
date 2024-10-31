# Istio

**English** | [**简体中文**](./istio-zh_CN.md)

## Introduction

In the context of Istio, when using Spiderpool to configure the network for service mesh applications with an Underlay network, there may be issues where traffic cannot be intercepted by Istio. This is because:

1. Traffic accessing the service mesh Pod is forwarded through its veth0 network interface (created by Spiderpool). The traffic is then intercepted to the sidecar container through the iptables redirect rules set by Istio. However, since iptables redirect rules require the receiving network interface to be configured with an IP address, otherwise the packet will be silently dropped by the kernel.

2. By default, Spiderpool does not configure an IP address for the veth0 network interface of Pods using the Underlay network, which leads to the traffic accessing the service mesh being dropped.

Refer to [#Issue 3568](https://github.com/spidernet-io/spiderpool/issues/3568). To solve this problem, Spiderpool provides a configuration: `vethLinkAddress`, which is used to configure a link-local address for the veth0 network interface.

## How to Configure

1. When installing Spiderpool using Helm, you can enable this feature with the following command:

    ```shell
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    helm repo update spiderpool
    kubectl create namespace spiderpool
    helm install spiderpool spiderpool/spiderpool -n spiderpool --set coordinator.vethLinkAddress=169.254.100.1
    ```

    > - `vethLinkAddress` must be a valid IP address.
    > - If you are a user in China, you can specify the parameter `--set global.imageRegistryOverride=ghcr.m.daocloud.io` to use a domestic image source.

2. After installation, check the configuration of the Spidercoordinator to ensure that `vethLinkAddress` is configured correctly:

    ```shell
    ~# kubectl get spidercoordinators.spiderpool.spidernet.io default -o yaml
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderCoordinator
    metadata:
      creationTimestamp: "2024-10-30T08:31:09Z"
      finalizers:
      - spiderpool.spidernet.io
      generation: 7
      name: default
      resourceVersion: "195405"
      uid: 8bdceced-15db-497b-be07-81cbcba7caac
    spec:
      detectGateway: false
      detectIPConflict: false
      hijackCIDR:
      - 169.254.0.0/16
      podRPFilter: 0
      hostRPFilter: 0
      hostRuleTable: 500
      mode: auto
      podCIDRType: calico
      podDefaultRouteNIC: ""
      vethLinkAddress: 169.254.100.1
      podMACPrefix: ""
      tunePodRoutes: true
    status:
      overlayPodCIDR:
      - 10.222.64.0/18
      - 10.223.64.0/18
      phase: Synced
      serviceCIDR:
      - 10.233.0.0/18
    ```

3. If you have already installed Spiderpool, you can directly modify the configuration of `vethLinkAddress` in the Spidercoordinator:

    ```shell
    kubectl patch spidercoordinators default --type='merge' -p '{"spec": {"vethLinkAddress": "169.254.100.1"}}'
    ```

4. Step 3 is the default setting for the cluster. If you do not want the entire cluster to default to configuring `vethLinkAddress`, you can configure it for a single network interface:

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
        vethLinkAddress: 169.254.100.1
    EOF
    ```

## Verification

After creating the application, you can check whether the Pod's veth0 network interface is correctly configured with the IP address: 169.254.100.1

```shell
~# kubectl exec -it <pod-name> -n <namespace> -- ip addr show veth0
```
