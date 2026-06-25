# Cilium VXLAN 与 RDMA MTU

[**English**](./cilium-rdma-mtu.md) | **简体中文**

## 介绍

在同时包含 RDMA 节点和非 RDMA 节点的集群中，Pod 可能使用 Cilium 作为默认网络，并使用 Spiderpool 提供 Underlay 网络接口。当 Cilium 运行在 VXLAN 模式时，如果 RDMA 节点和非 RDMA 节点路径上的有效 MTU 不一致，跨节点 Pod 通信可能会异常。

这个问题通常表现为小包可以通信，但较大的数据包会被丢弃，例如 TLS 握手或证书下发相关流量。在已报告的场景中，Istio sidecar 可以从控制面完成服务发现，但与 Istiod 的 TLS 连接无法完成。根因是 MTU 黑洞：报文大小超过 VXLAN 封装后的安全传输大小，路径无法继续转发这些报文。

参考 [#Issue 5677](https://github.com/spidernet-io/spiderpool/issues/5677)。为了解决这个问题，Spiderpool 提供 `vethMTU` 配置，用于设置 coordinator 插件创建的 Pod `veth0` 网卡 MTU。默认值为 `1500`。在受影响的 Cilium VXLAN 与 RDMA 混合节点环境中，可将其设置为较小的值，例如 `1400`，以确保报文可以通过封装路径。

## 如何配置

1. 使用 Helm 安装 Spiderpool 时，配置默认 coordinator 的 `vethMTU`：

    ```shell
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    helm repo update spiderpool
    kubectl create namespace spiderpool
    helm install spiderpool spiderpool/spiderpool -n spiderpool --set coordinator.vethMTU=1400
    ```

    > - `vethMTU` 必须大于 `0`。
    > - 默认值为 `1500`。
    > - 如果您是中国用户，可以指定参数 `--set global.imageRegistryOverride=ghcr.m.daocloud.io` 来使用国内的镜像源。

2. 安装完成后，查看 SpiderCoordinator 配置：

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

3. 如果已经安装 Spiderpool，可以直接修改默认 SpiderCoordinator：

    ```shell
    kubectl patch spidercoordinators default --type='merge' -p '{"spec": {"vethMTU": 1400}}'
    ```

4. 步骤 3 是集群默认设置。如果只希望对某一个 Spiderpool 网络配置该 MTU，可在 `SpiderMultusConfig` 的 coordinator 配置中设置 `vethMTU`：

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

## 验证

创建应用后，查看 Pod 的 `veth0` 网卡是否使用了配置的 MTU：

```shell
~# kubectl exec -it <pod-name> -n <namespace> -- ip link show veth0
```

输出中应包含 `mtu 1400`：

```text
3: veth0@if123: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1400 qdisc noqueue state UP mode DEFAULT group default
```

然后验证之前异常的应用流量，例如 RDMA 节点与非 RDMA 节点上 Pod 之间的 TLS 访问。
