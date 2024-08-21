# Ovs-cni Quick Start

[**English**](./get-started-ovs.md) | **简体中文**

Spiderpool 可用作 Underlay 网络场景下提供固定 IP 的一种解决方案，本文将以 [Multus](https://github.com/k8snetworkplumbingwg/multus-cni)、[Ovs-cni](https://github.com/k8snetworkplumbingwg/ovs-cni) 、[Spiderpool](https://github.com/spidernet-io/spiderpool) 为例，搭建一套完整的 Underlay 网络解决方案，该方案能将可用的网桥公开为节点资源，供集群使用。

[`ovs-cni`](https://github.com/k8snetworkplumbingwg/ovs-cni) 是一个基于 Open vSwitch（OVS）的 Kubernetes CNI 插件，它提供了一种在 Kubernetes 集群中使用 OVS 进行网络虚拟化的方式。

## 先决条件

1. 一个多节点的 Kubernetes 集群

2. [Helm 工具](https://helm.sh/docs/intro/install/)

3. 必须在主机上安装并运行 Open vSwitch，可参考[官方安装说明](https://docs.openvswitch.org/en/latest/intro/install/#installation-from-packages)

    以下示例是基于 Ubuntu 22.04.1。主机系统不同，安装方式可能不同。

    ```bash
    ~# sudo apt-get install -y openvswitch-switch
    ~# sudo systemctl start openvswitch-switch
    ```

4. 如果您使用如 Fedora、Centos 等 OS， 并且使用 NetworkManager 管理和配置网络，在以下场景时建议您需要配置 NetworkManager:

    * 如果你使用 Underlay 模式，`coordinator` 会在主机上创建 veth 接口，为了防止 NetworkManager 干扰 veth 接口, 导致 Pod 访问异常。我们需要配置 NetworkManager，使其不纳管这些 Veth 接口。

    * 如果你通过 `Ifacer` 创建 Vlan 和 Bond 接口，NetworkManager 可能会干扰这些接口，导致 Pod 访问异常。我们需要配置 NetworkManager，使其不纳管这些 Veth 接口。

      ```shell
      ~# IFACER_INTERFACE="<NAME>"
      ~# cat << EOF | > /etc/NetworkManager/conf.d/spidernet.conf
      > [keyfile]
      > unmanaged-devices=interface-name:^veth*;interface-name:${IFACER_INTERFACE}
      > EOF
      ~# systemctl restart NetworkManager
      ```

## 节点上配置 Open vSwitch 网桥

如下是创建并配置持久 OVS Bridge 的示例，本文中以 `eth0` 网卡为例，需要在每个节点上执行。

### Ubuntu 系统使用 netplan 持久化 OVS Bridge

如果您使用的是 Ubuntu 系统，可以参考本章节通过 netplan 配置 OVS Bridge。

1. 创建 OVS Bridge

    ```bash
    ~# ovs-vsctl add-br br1
    ~# ovs-vsctl add-port br1 eth0
    ~# ip link set br1 up
    ```

2. 在 /etc/netplan 目录下创建 12-br1.yaml 后，通过 `netplan apply` 生效。为确保在重启主机等场景下 br1 仍然可用，请检查 eth0 网卡是否也被 netplan 纳管。

    ```yaml: 12-br1.yaml
    network:
    version: 2
    renderer: networkd
    ethernets:
      br1:
      addresses:
        - "<IP地址>/<子网掩码>" # 172.18.10.10/16
    ```

3. 创建后，可以在每个节点上查看到如下的网桥信息：

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

    ~# ip a show br1
    208: br1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UNKNOWN group default qlen 1000
        link/ether 00:50:56:b4:5f:fd brd ff:ff:ff:ff:ff:ff
        inet 172.18.10.10/16 brd 172.18.255.255 scope global noprefixroute br1
          valid_lft forever preferred_lft forever
        inet6 fe80::4f28:8ef1:6b82:a9e4/64 scope link noprefixroute 
          valid_lft forever preferred_lft forever
    ```

### Fedora、Centos 等使用 NetworkManager 持久化 OVS Bridge

如果您使用如 Fedora、Centos 等 OS，推荐使用 NetworkManager 持久化 OVS Bridge。通过 NetworkManager 持久化 OVS Bridge 是一种不局限操作系统，更通用的一种方式。

1. 使用 NetworkManager 持久化 OVS Bridge，你需要安装 OVS NetworkManager 插件，示例如下：

    ```bash
    ~# sudo dnf install -y NetworkManager-ovs
    ~# sudo systemctl restart NetworkManager
    ```

2. 创建 ovs 网桥、端口和接口。

    ```bash
    ~# sudo nmcli con add type ovs-bridge conn.interface br1 con-name br1
    ~# sudo nmcli con add type ovs-port conn.interface br1-port master br1 con-name br1-port
    ~# sudo nmcli con add type ovs-interface slave-type ovs-port conn.interface br1 master br1-port con-name br1-int
    ```

3. 在网桥上创建另一个端口，并选择我们的物理设备中的 eth0 网卡作为其以太网接口，以便真正的流量可以在网络上流转。

    ```bash
    ~# sudo nmcli con add type ovs-port conn.interface ovs-port-eth0 master br1 con-name ovs-port-eth0
    ~# sudo nmcli con add type ethernet conn.interface eth0 master ovs-port-eth0 con-name ovs-port-eth0-int
    ```

4. 配置与激活 ovs 网桥。

    通过设置静态 IP 的方式配置网桥

    ```bash
    ~# sudo nmcli con modify br1-int ipv4.method static ipv4.address "<IP地址>/<子网掩码>" # 172.18.10.10/16
    ```

    激活网桥。

    ```bash
    ~# sudo nmcli con down "eth0"
    ~# sudo nmcli con up ovs-port-eth0-int
    ~# sudo nmcli con up br1-int
    ```

5. 创建后，可以在每个节点上查看到类似如下的信息。

    ```bash
    ~# nmcli c
    br1-int             dbb1c9be-e1ab-4659-8d4b-564e3f8858fa  ovs-interface  br1             
    br1                 a85626c1-2392-443b-a767-f86a57a1cff5  ovs-bridge     br1             
    br1-port            fe30170f-32d2-489e-9ca3-62c1f5371c6c  ovs-port       br1-port        
    ovs-port-eth0       a43771a9-d840-4d2d-b1c3-c501a6da80ed  ovs-port       ovs-port-eth0   
    ovs-port-eth0-int   1334f49b-dae4-4225-830b-4d101ab6fad6  ethernet       eth0         

    ~# ovs-vsctl show
    203dd6d0-45f4-4137-955e-c4c36b9709e6
        Bridge br1
            Port ovs-port-eth0
                Interface eth0
                    type: system
            Port br1-port
                Interface br1
                    type: internal
        ovs_version: "3.2.1"

    ~# ip a show br1
    208: br1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UNKNOWN group default qlen 1000
        link/ether 00:50:56:b4:5f:fd brd ff:ff:ff:ff:ff:ff
        inet 172.18.10.10/16 brd 172.18.255.255 scope global noprefixroute br1
          valid_lft forever preferred_lft forever
        inet6 fe80::4f28:8ef1:6b82:a9e4/64 scope link noprefixroute 
          valid_lft forever preferred_lft forever
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

2. 检查 `Spidercoordinator.status` 中的 Phase 是否为 Synced:

    ```shell
    ~# kubectl  get spidercoordinators.spiderpool.spidernet.io default -o yaml
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderCoordinator
    metadata:
      creationTimestamp: "2023-10-18T08:31:09Z"
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
      podMACPrefix: ""
      tunePodRoutes: true
    status:
      overlayPodCIDR:[]
      phase: Synced
      serviceCIDR:
      - 10.233.0.0/18
    ```

    如果状态为 `NotReady`,这将会阻止 Pod 被创建。目前 Spiderpool:
    * 优先通过查询 `kube-system/kubeadm-config` ConfigMap 获取集群的 Pod 和 Service 子网。 
    * 如果 `kubeadm-config` 不存在导致无法获取集群子网，那么 Spiderpool 会从 `Kube-controller-manager Pod` 中获取集群 Pod 和 Service 的子网。 如果您集群的 Kube-controller-manager 组件以 `systemd` 等方式而不是以静态 Pod 运行。那么 Spiderpool 仍然无法获取集群的子网信息。

    如果上面两种方式都失败，Spiderpool 会同步 status.phase 为 NotReady, 这将会阻止 Pod 被创建。我们可以手动创建 kubeadm-config ConfigMap，并正确配置集群的子网信息:

    ```shell
    export POD_SUBNET=<YOUR_POD_SUBNET>
    export SERVICE_SUBNET=<YOUR_SERVICE_SUBNET>
    cat << EOF | kubectl apply -f -
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: kubeadm-config
      namespace: kube-system
    data:
      ClusterConfiguration: |
        networking:
          podSubnet: ${POD_SUBNET}
          serviceSubnet: ${SERVICE_SUBNET}
    EOF
    ```

    一旦创建完成，Spiderpool 将会自动同步其状态。

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
