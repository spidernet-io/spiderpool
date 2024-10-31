# Istio

**简体中文** | [**English**](./istio.md)

## 介绍

在 Istio 场景下，使用 Spiderpool 配置服务网格应用使用 Underlay 网络时，可能会出现流量无法被 istio 劫持的问题。这是因为：

1. 访问服务网格 Pod 的流量通过其 veth0 网卡(由 Spiderpool 创建)转发。流量随后会通过 istio 设置的 iptables redirect 规则，被劫持到 sidecar 容器中。但由于 iptables redirect 规则必须要求接收流量的网卡必须配置 IP 地址，否则该数据包会被内核沉默的丢弃。

2. 在默认情况下，Spiderpool 不会为使用 Underlay 网络的 Pod 的 veth0 网卡配置 IP 地址, 所以这会导致访问服务网格的流量被丢弃。

参考 [#Issue 3568](https://github.com/spidernet-io/spiderpool/issues/3568)。为了解决这个问题， Spiderpool 提供一个配置: `vethLinkAddress`，用于为 veth0 网卡配置一个 link-local 地址。

## 如何配置

1. 使用 Helm 安装 Spiderpool 时，可通过以下命令开启这个功能：

    ```shell
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    helm repo update spiderpool
    kubectl create namespace spiderpool
    helm install spiderpool spiderpool/spiderpool -n spiderpool --set coordinator.vethLinkAddress=169.254.100.1
    ```

    > - `vethLinkAddress` 必须是一个合法的 IP 地址。
    > - 如果您是中国用户，可以指定参数 `--set global.imageRegistryOverride=ghcr.m.daocloud.io` 来使用国内的镜像源。

2. 安装完成后，查看 Spidercoordinator 的配置，确保 `vethLinkAddress` 已配置正确：

    ```shell
    ~# kubectl  get spidercoordinators.spiderpool.spidernet.io default -o yaml
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

3. 如果您已经安装 Spiderpool, 您可以直接修改 Spidercoordinator 中关于 vethLinkAddress 的配置:

    ```shell
    kubectl patch spidercoordinators default --type='merge' -p '{"spec": {"vethLinkAddress": "169.254.100.1"}}'
    ```

4. 步骤 3 中是集群默认设置，如果您不希望整个集群默认都配置 vethLinkAddress，您可以为单个网卡配置：

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

## 验证

创建应用后，可查看 Pod 的 veth0 网卡是否正确配置 IP 地址：169.254.100.1

```shell
~# kubectl exec -it <pod-name> -n <namespace> -- ip addr show veth0
```
