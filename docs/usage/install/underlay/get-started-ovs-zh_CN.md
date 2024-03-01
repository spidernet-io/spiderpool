# Ovs-cni Quick Start

[**English**](./get-started-ovs.md) | **简体中文**

Spiderpool 可用作 Underlay 网络场景下提供固定 IP 的一种解决方案，本文将以 [Multus](https://github.com/k8snetworkplumbingwg/multus-cni)、[Ovs-cni](https://github.com/k8snetworkplumbingwg/ovs-cni) 、[Spiderpool](https://github.com/spidernet-io/spiderpool) 为例，搭建一套完整的 Underlay 网络解决方案，该方案能将可用的网桥公开为节点资源，供集群使用。

[`ovs-cni`](https://github.com/k8snetworkplumbingwg/ovs-cni) 是一个基于 Open vSwitch（OVS）的 Kubernetes CNI 插件，它提供了一种在 Kubernetes 集群中使用 OVS 进行网络虚拟化的方式。

## 先决条件

1. [安装要求](./../system-requirements-zh_CN.md)

2. 一个多节点的 Kubernetes 集群

3. [Helm 工具](https://helm.sh/docs/intro/install/)

4. 必须在主机上安装并运行 Open vSwitch，可参考[官方安装说明](https://docs.openvswitch.org/en/latest/intro/install/#installation-from-packages)

    以下示例是基于 Ubuntu 22.04.1。主机系统不同，安装方式可能不同。

    ```bash
    ~# sudo apt-get install -y openvswitch-switch
    ~# sudo systemctl start openvswitch-switch
    ```

5. 如果您使用如 Fedora、Centos 等 OS， 并且使用 NetworkManager 管理和配置网络，在以下场景时建议您需要配置 NetworkManager:

    * 如果你使用 Underlay 模式，`coordinator` 会在主机上创建 veth 接口，为了防止 NetworkManager 干扰 veth 接口, 导致 Pod 访问异常。我们需要配置 NetworkManager，使其不纳管这些 Veth 接口。

    * 如果你通过 `Iface`r 创建 Vlan 和 Bond 接口，NetworkManager 可能会干扰这些接口，导致 Pod 访问异常。我们需要配置 NetworkManager，使其不纳管这些 Veth 接口。

      ```shell
      ~# IFACER_INTERFACE="<NAME>"
      ~# cat > /etc/NetworkManager/conf.d/spidernet.conf <<EOF
      [keyfile]
      unmanaged-devices=interface-name:^veth*;interface-name:${IFACER_INTERFACE}
      EOF
      ~# systemctl restart NetworkManager
      ```

## 安装 Spiderpool

1. 安装 Spiderpool。

    ```bash
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    helm repo update spiderpool
    helm install spiderpool spiderpool/spiderpool --namespace kube-system --set multus.multusCNI.defaultCniCRName="ovs-conf" --set plugins.installOvsCNI=true
    ```

    > 如果未安装 ovs-cni, 可以通过 Helm 参数 '-set plugins.installOvsCNI=true' 安装它。
    >
    > 如果您是国内用户，可以指定参数 `--set global.imageRegistryOverride=ghcr.m.daocloud.io` 以帮助您快速的拉取镜像。
    >
    > 通过 `multus.multusCNI.defaultCniCRName` 指定 multus 默认使用的 CNI 的 NetworkAttachmentDefinition 实例名。如果 `multus.multusCNI.defaultCniCRName` 选项不为空，则安装后会自动生成一个数据为空的 NetworkAttachmentDefinition 对应实例。如果 `multus.multusCNI.defaultCniCRName` 选项为空，会尝试通过 /etc/cni/net.d 目录下的第一个 CNI 配置来创建对应的 NetworkAttachmentDefinition 实例，否则会自动生成一个名为 `default` 的 NetworkAttachmentDefinition 实例，以完成 multus 的安装。

2. 在每个节点上配置 Open vSwitch 网桥。

    创建网桥并配置网桥，以 `eth0` 为例。

    ```bash
    ~# ovs-vsctl add-br br1
    ~# ovs-vsctl add-port br1 eth0
    ~# ip addr add <IP地址>/<子网掩码> dev br1
    ~# ip link set br1 up
    ~# ip route add default via <默认网关IP> dev br1
    ```

    请把以上命令配置在系统行动脚本中，以在主机重启时能够生效

    创建后，可以在每个节点上查看到如下的网桥信息：

    ```bash
    ~# ovs-vsctl show
    ec16d9e1-6187-4b21-9c2f-8b6cb75434b9
        Bridge br1
            Port eth0
                Interface eth0
            Port br1
                Interface br1
                    type: internal
            Port veth97fb4795
                Interface veth97fb4795
        ovs_version: "2.17.3"
    ```

3. 创建 SpiderIPPool 实例。

    Pod 会从该 IP 池中获取 IP，进行 Underlay 的网络通讯，所以该 IP 池的子网需要与接入的 Underlay 子网对应。以下是创建相关的 SpiderIPPool 示例：

    ```shell
    cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderIPPool
    metadata:
      name: ippool-test
    spec:
      ips:
      - "172.18.30.131-172.18.30.140"
      subnet: 172.18.0.0/16
      gateway: 172.18.0.1
      multusName: 
      - kube-system/ovs-conf
    EOF
    ```

4. 验证安装：

    ```bash
    ~# kubectl get po -n kube-system |grep spiderpool
    spiderpool-agent-7hhkz                   1/1     Running     0              13m
    spiderpool-agent-kxf27                   1/1     Running     0              13m
    spiderpool-controller-76798dbb68-xnktr   1/1     Running     0              13m
    spiderpool-init                          0/1     Completed   0              13m

    ~# kubectl get sp ippool-test       
    NAME          VERSION   SUBNET          ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
    ippool-test   4         172.18.0.0/16   0                    10               false
    ~# 
    ```

5. Spiderpool 为简化书写 JSON 格式的 Multus CNI 配置，它提供了 SpiderMultusConfig CR 来自动管理 Multus NetworkAttachmentDefinition CR。如下是创建 Ovs SpiderMultusConfig 配置的示例：

    * 确认 ovs-cni 所需的网桥名称，本例子以 br1 为例:

    ```shell
    BRIDGE_NAME="br1"
    cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderMultusConfig
    metadata:
      name: ovs-conf
      namespace: kube-system
    spec:
      cniType: ovs
      ovs:
        bridge: "${BRIDGE_NAME}"
    EOF
    ```

## 创建应用

以下的示例 Yaml 中， 会创建 2 个副本的 Deployment，其中：

* `v1.multus-cni.io/default-network`：用于指定 Multus 的 NetworkAttachmentDefinition 配置，会基于它为应用创建一张默认网卡。

```shell
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      annotations:
        ipam.spidernet.io/ippool: |-
          {
            "ipv4": ["ippool-test"]
          }
        v1.multus-cni.io/default-network: kube-system/ovs-conf
      labels:
        app: test-app
    spec:
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchLabels:
                  app: test-app
              topologyKey: kubernetes.io/hostname
      containers:
      - name: test-app
        image: nginx
        imagePullPolicy: IfNotPresent
        ports:
        - name: http
          containerPort: 80
          protocol: TCP
EOF
```

SpiderIPPool 为应用分配了 IP，应用的 IP 将会自动固定在该 IP 范围内：

```bash
~# kubectl get po -l app=test-app -o wide
NAME                        READY   STATUS    RESTARTS   AGE     IP              NODE                 NOMINATED NODE   READINESS GATES
test-app-6f8dddd88d-hstg7   1/1     Running   0          3m37s   172.18.30.131   ipv4-worker          <none>           <none>
test-app-6f8dddd88d-rj7sm   1/1     Running   0          3m37s   172.18.30.132   ipv4-control-plane   <none>           <none>

~# kubectl get spiderippool
NAME          VERSION   SUBNET          ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
ippool-test   4         172.18.0.0/16   2                    2                false     false

~# kubectl get spiderendpoints
NAME                        INTERFACE   IPV4POOL      IPV4               IPV6POOL   IPV6   NODE
test-app-6f8dddd88d-hstg7   eth0        ippool-test   172.18.30.131/16                     ipv4-worker
test-app-6f8dddd88d-rj7sm   eth0        ippool-test   172.18.30.132/16                     ipv4-control-plane
```

测试 Pod 与 Pod 的通讯情况，以跨节点 Pod 为例：

```shell
~#kubectl exec -ti test-app-6f8dddd88d-hstg7 -- ping 172.18.30.132 -c 2

PING 172.18.30.132 (172.18.30.132): 56 data bytes
64 bytes from 172.18.30.132: seq=0 ttl=64 time=1.882 ms
64 bytes from 172.18.30.132: seq=1 ttl=64 time=0.195 ms

--- 172.18.30.132 ping statistics ---
2 packets transmitted, 2 packets received, 0% packet loss
round-trip min/avg/max = 0.195/1.038/1.882 ms
```
