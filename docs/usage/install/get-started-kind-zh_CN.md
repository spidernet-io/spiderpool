# Quick Start

[**English**](./get-started-kind.md) | **简体中文**

Kind 是一个使用 Docker 容器节点运行本地 Kubernetes 集群的工具。Spiderpool 提供了安装 Kind 集群的脚本，您可以使用它来部署符合您需求的集群，进行 Spiderpool 的测试与体验。

## 安装要求

* 获取 Spiderpool 稳定版本的代码到本地主机上，并进入 Spiderpool 工程的根目录。

    ```bash
    ~# LATEST_RELEASE_VERISON=$(curl -s https://api.github.com/repos/spidernet-io/spiderpool/releases | grep '"tag_name":' | grep -v rc | grep -Eo "([0-9]+\.[0-9]+\.[0-9])" | sort -r | head -n 1)
    ~# curl -Lo /tmp/$LATEST_RELEASE_VERISON.tar.gz https://github.com/spidernet-io/spiderpool/archive/refs/tags/v$LATEST_RELEASE_VERISON.tar.gz
    ~# tar -xvf /tmp/$LATEST_RELEASE_VERISON.tar.gz -C /tmp/
    ~# cd /tmp/spiderpool-$LATEST_RELEASE_VERISON
    ```
  
* 执行 `make dev-doctor`，检查本地主机上的开发工具是否满足部署 Kind 集群与 Spiderpool 的条件。

    构建 Spiderpool 环境需要具备 Kubectl、Kind、Docker、Helm、yq 工具。如果你的本机上缺少，请运行 `test/scripts/install-tools.sh` 来安装它们。

## 快速启动

如果您在中国大陆，安装时可以额外指定参数 `-e E2E_CHINA_IMAGE_REGISTRY=true` ，以帮助您更快的拉取镜像。

===  "创建 Spiderpool 单 CNI 集群"

    在该场景下，你可以通过简易运维，即可让应用可分配到固定的 Underlay IP 地址，同时 Pod 能够通过 Pod IP、clusterIP、nodePort 等方式通信。具体可参考 [一个或多个 underlay CNI 协同](../../concepts/arch-zh_CN.md#应用场景pod-接入一个-overlay-cni-和若干个-underlay-cni-网卡) 

    如下命令将创建一个 Macvlan 的单 CNI 集群，其中，通过 kube-proxy 实施 service 解析

    ```bash
    ~# make setup_singleCni_macvlan
    ```

===  "创建 Spiderpool 和 Calico 的双 CNI 集群"

    在这个场景下，你可以体验 Pod 具备双 CNI 网卡的效果。具体可参考 [underlay CNI 和 overlay CNI 协同](../../concepts/arch-zh_CN.md#应用场景pod-接入一个-overlay-cni-和若干个-underlay-cni-网卡) 

    如下命令将创建一个 Calico 为 main CNI ，并搭配 Spiderpool 为 POD 接入第二张 underlay 网卡，其中 Calico 基于 iptables datapath 工作，基于 kube-proxy 实现 service 解析。
    
    ```bash
    ~# make setup_dualCni_calico
    ```

===  "创建 Spiderpool 和 Cilium 的双 CNI 集群"

    在这个场景下，你可以体验 Pod 具备双 CNI 网卡的效果。具体可参考 [underlay CNI 和 overlay CNI 协同](../../concepts/arch-zh_CN.md#应用场景pod-接入一个-overlay-cni-和若干个-underlay-cni-网卡) 

    如下命令将创建一个 Cilium 为 main CNI ，并搭配 Spiderpool 为 POD 接入第二张 underlay 网卡，其中开启了 Cilium 的 eBPF 加速，并关闭了 kube-proxy 组件，基于 eBPF 实现 service 解析。
    
    > 确认操作系统 Kernel 版本号是是否 >= 4.9.17，内核过低时将会导致安装失败，推荐 Kernel 5.10+ 。

    ```bash
    ~# make setup_dualCni_cilium
    ```

## 验证安装

在 Spiderpool 工程的根目录下执行如下命令，为 kubectl 配置 Kind 集群的 KUBECONFIG。

```bash
~# export KUBECONFIG=$(pwd)/test/.cluster/spider/.kube/config
```

您可以看到类似如下的内容输出：

```bash
~# kubectl get nodes
NAME                   STATUS   ROLES           AGE     VERSION
spider-control-plane   Ready    control-plane   2m29s   v1.26.2
spider-worker          Ready    <none>          2m58s   v1.26.2

~# kubectl get po -n kube-system | grep spiderpool
NAME                                           READY   STATUS      RESTARTS   AGE                                
spiderpool-agent-4dr97                         1/1     Running     0          3m
spiderpool-agent-4fkm4                         1/1     Running     0          3m
spiderpool-controller-7864477fc7-c5dk4         1/1     Running     0          3m
spiderpool-controller-7864477fc7-wpgjn         1/1     Running     0          3m
spiderpool-init                                0/1     Completed   0          3m
```

Spiderpool 提供的快速安装 Kind 集群脚本会自动为您创建一个应用，以验证您的 Kind 集群是否能够正常工作，以下是应用的运行状态：

```bash
~# kubectl get po -l app=test-pod -o wide
NAME                       READY   STATUS    RESTARTS   AGE     IP             NODE            NOMINATED NODE   READINESS GATES
test-pod-856f9689d-876nm   1/1     Running   0          5m34s   172.18.40.63   spider-worker   <none>           <none>
```

## 部署应用

通过上述检查，Kind 集群一切正常。在本章节，将介绍在不同的环境下，如何去使用 Spiderpool 。

> Spiderpool 提供了 [Spidermultusconfig](../spider-multus-config-zh_CN.md) CR 来自动管理 Multus NetworkAttachmentDefinition CR ，实现了对开源项目 Multus CNI 配置管理的扩展。

===  "基于 Spiderpool 单 CNI 环境"

    获取集群的 Spidermultusconfig CR 与 IPPool CR。

    ```bash
    ~# kubectl get spidermultusconfigs.spiderpool.spidernet.io -A
    NAMESPACE     NAME              AGE
    kube-system   macvlan-vlan0     1h
    kube-system   macvlan-vlan100   1h
    kube-system   macvlan-vlan200   1h

    ~# kubectl get spiderippool
    NAME                VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
    default-v4-ippool   4         172.18.0.0/16             5                    253              true      
    default-v6-ippool   6         fc00:f853:ccd:e793::/64   5                    253              true      
    ...
    ```

    创建应用，以下命令会创建 1 个副本 Deployment，其中：

    - `v1.multus-cni.io/default-network`：通过它指定 Spidermultusconfig CR: `kube-system/macvlan-vlan0`，并通过该配置为应用创建一张由 Macvlan 配置网络的默认网卡 (eth0) 。
    
    - `ipam.spidernet.io/ippool`：用于指定 Spiderpool 的 IP 池，Spiderpool 会自动在该池中选择 IP 与应用的默认网卡形成绑定。
  
    ```shell
    cat <<EOF | kubectl create -f -
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: test-app
    spec:
      replicas: 1
      selector:
        matchLabels:
          app: test-app
      template:
        metadata:
          labels:
            app: test-app
          annotations:
            ipam.spidernet.io/ippool: |-
              {      
                "ipv4": ["default-v4-ippool"],
                "ipv6": ["default-v6-ippool"]
              }
            v1.multus-cni.io/default-network: kube-system/macvlan-vlan0
        spec:
          containers:
          - name: test-app
            image: alpine
            imagePullPolicy: IfNotPresent
            command:
            - "/bin/sh"
            args:
            - "-c"
            - "sleep infinity"
    EOF
    ```

    验证应用创建成功。

    ```shell
    ~# kubectl get po -owide
    NAME                        READY   STATUS    RESTARTS   AGE    IP              NODE            NOMINATED NODE   READINESS GATES
    test-app-7fdbb59666-4k5m7   1/1     Running       0          9s    172.18.40.223   spider-worker   <none>           <none>

    ~# kubectl exec -ti test-app-7fdbb59666-4k5m7 -- ip a
    ...
    3: eth0@if339: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue state UP 
        link/ether 0a:96:54:6f:76:b4 brd ff:ff:ff:ff:ff:ff
        inet 172.18.40.223/16 brd 172.18.255.255 scope global eth0
           valid_lft forever preferred_lft forever
    4: veth0@if11: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue state UP 
        link/ether 4a:8b:09:d9:4c:0a brd ff:ff:ff:ff:ff:ff
    ```

===  "基于 Spiderpool 和 Calico 的双 CNI 环境"

    获取集群的 Spidermultusconfig CR 与 IPPool CR

    ```bash
    ~# kubectl get spidermultusconfigs.spiderpool.spidernet.io -A
    NAMESPACE     NAME              AGE
    kube-system   calico            3m11s
    kube-system   macvlan-vlan0     2m20s
    kube-system   macvlan-vlan100   2m19s
    kube-system   macvlan-vlan200   2m19s

    ~# kubectl get spiderippool
    NAME                VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
    default-v4-ippool   4         172.18.0.0/16             1                    253              true
    default-v6-ippool   6         fc00:f853:ccd:e793::/64   1                    253              true
    ...
    ```

    创建应用，以下命令会创建一个具备两张网卡的 Deployment 应用，其中：

    - 默认网卡（eth0）由集群缺省 CNI Calico 配置。

    - `k8s.v1.cni.cncf.io/networks`：通过该注解额外为应用额外再创建一张由 Macvlan 配置网络的网卡 (net1) 。

    - `ipam.spidernet.io/ippools`：用于指定 Spiderpool 的 IP 池，Spiderpool 会自动在该池中选择 IP 与应用的 net1 网卡形成绑定。

    ```shell
    cat <<EOF | kubectl create -f -
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: test-app
    spec:
      replicas: 1
      selector:
        matchLabels:
          app: test-app
      template:
        metadata:
          labels:
            app: test-app
          annotations:
            ipam.spidernet.io/ippools: |-
              [{
                "interface": "net1",
                "ipv4": ["default-v4-ippool"],
                "ipv6": ["default-v6-ippool"]
              }]
            k8s.v1.cni.cncf.io/networks: kube-system/macvlan-vlan0
        spec:
          containers:
          - name: test-app
            image: alpine
            imagePullPolicy: IfNotPresent
            command:
            - "/bin/sh"
            args:
            - "-c"
            - "sleep infinity"
    EOF
    ```

    验证应用创建成功

    ```shell
    ~# kubectl get po -owide
    NAME                      READY   STATUS    RESTARTS   AGE   IP               NODE            NOMINATED NODE   READINESS GATES
    test-app-86dd478b-bv6rm   1/1     Running   0          12s   10.243.104.211   spider-worker   <none>           <none>

    ~# kubectl exec -ti test-app-7fdbb59666-4k5m7 -- ip a
    ...
    4: eth0@if148: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1480 qdisc noqueue state UP qlen 1000
        link/ether 1a:1e:e1:f3:f9:4b brd ff:ff:ff:ff:ff:ff
        inet 10.243.104.211/32 scope global eth0
    5: net1@if347: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue state UP 
        link/ether 56:b4:3d:a6:d2:d1 brd ff:ff:ff:ff:ff:ff
        inet 172.18.40.154/16 brd 172.18.255.255 scope global net1
    ```

===  "基于 Spiderpool 和 Cilium 的双 CNI 环境"

    获取集群的 Spidermultusconfig CR 与 IPPool CR

    ```bash
    ~# kubectl get spidermultusconfigs.spiderpool.spidernet.io -A
    NAMESPACE     NAME              AGE
    kube-system   cilium            5m32s
    kube-system   macvlan-vlan0     5m12s
    kube-system   macvlan-vlan100   5m17s
    kube-system   macvlan-vlan200   5m18s

    ~# kubectl get spiderippool
    NAME                VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
    default-v4-ippool   4         172.18.0.0/16             1                    253              true
    default-v6-ippool   6         fc00:f853:ccd:e793::/64   1                    253              true
    ...
    ```

    创建应用，以下命令会创建一个具备两张网卡的 Deployment 应用，其中：

    - 默认网卡（eth0）由集群缺省 CNI Cilium 配置。

    - `k8s.v1.cni.cncf.io/networks`：通过该注解额外为应用额外再创建一张由 Macvlan 配置网络的网卡 (net1) 。

    - `ipam.spidernet.io/ippools`：用于指定 Spiderpool 的 IP 池，Spiderpool 会自动在该池中选择 IP 与应用的 net1 网卡形成绑定。

    ```shell
    cat <<EOF | kubectl create -f -
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: test-app
    spec:
      replicas: 1
      selector:
        matchLabels:
          app: test-app
      template:
        metadata:
          labels:
            app: test-app
          annotations:
            ipam.spidernet.io/ippools: |-
              [{
                "interface": "net1",
                "ipv4": ["default-v4-ippool"],
                "ipv6": ["default-v6-ippool"]
              }]
            k8s.v1.cni.cncf.io/networks: kube-system/macvlan-vlan0
        spec:
          containers:
          - name: test-app
            image: alpine
            imagePullPolicy: IfNotPresent
            command:
            - "/bin/sh"
            args:
            - "-c"
            - "sleep infinity"
    EOF
    ```

    验证应用创建成功

    ```shell
    ~# kubectl get po -owide
    NAME                      READY   STATUS    RESTARTS   AGE   IP               NODE            NOMINATED NODE   READINESS GATES
    test-app-86dd478b-ml8d9   1/1     Running   0          58s   10.244.102.212   spider-worker   <none>           <none>

    ~# kubectl exec -ti test-app-7fdbb59666-4k5m7 -- ip a
    ...
    4: eth0@if148: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1480 qdisc noqueue state UP qlen 1000
        link/ether 26:f1:88:f9:7d:d7 brd ff:ff:ff:ff:ff:ff
        inet 10.244.102.212/32 scope global eth0
    5: net1@if347: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue state UP 
        link/ether ca:71:99:ec:ec:28 brd ff:ff:ff:ff:ff:ff
        inet 172.18.40.228/16 brd 172.18.255.255 scope global net1
    ```

现在您可以基于 Kind 测试与体验 Spiderpool 的[更多功能](../readme-zh_CN.md)。

## 卸载

* 卸载 Kind 集群

    执行 `make clean` 卸载 Kind 集群。

* 删除测试镜像

    ```bash
    ~# docker rmi -f $(docker images | grep spiderpool | awk '{print $3}')
    ~# docker rmi -f $(docker images | grep multus | awk '{print $3}')
    ```
