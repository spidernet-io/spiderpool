# 多网卡 Pod 注解

[**English**](./multi-interfaces-annotation.md) | **简体中文**

当使用 [Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni) 为 Pod 分配多张网卡时，Spiderpool 支持为每张网卡指定 IP 池。

该功能通过注解 `ipam.spidernet.io/subnets` 和 `ipam.spidernet.io/ippools` 实现。

## 快速开始

本示例将创建两个 Multus CNI 配置对象和两个 underlay 子网，然后运行一个拥有两张网卡、且两张网卡的 IP 分别位于不同子网的 Pod。

### 安装 Spiderpool

参考[安装指南](./install/get-started-kind-zh_CN.md)安装 Spiderpool。

### 配置 Multus

本示例使用 Macvlan 作为主 CNI，创建两个 network-attachment-definition，需要确认以下参数：

* 确认 Macvlan 所需的宿主机父接口。本示例以宿主机的 ens192 和 ens224 网卡为例，创建供 Pod 使用的 Macvlan 子接口。

* 为了使用 Veth 插件实现 clusterIP 通信，需要确认集群 Service 的 serviceIP CIDR，例如可以使用命令 `kubectl -n kube-system get configmap kubeadm-config -oyaml | grep service` 查看。

```shell
~# kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/multus-conf.yaml

~# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system
NAME                  AGE
macvlan-conf-ens192   20s
macvlan-conf-ens224   22s
```

## 通过子网实现多网卡

创建两个子网，分别为不同的网卡提供 IP 地址。

```shell
~# kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/different-segment-ipv4-subnets.yaml

~# kubectl get spidersubnet
NAME                 VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT
subnet-test-ens192   4         10.6.0.1/16   0                    10
subnet-test-ens224   4         10.7.0.1/16   0                    10
```

下面的示例 Yaml 会创建一个 2 副本的 Deployment：

```shell
~# kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/subnet-test-deploy.yaml
```

最终，Deployment 创建后，Spiderpool 会从指定的子网中随机选取 IP，创建两个固定 IP 池，分别绑定到 Deployment 中 Pod 的两张网卡上。

```bash
~# kubectl get spiderippool
NAME                                 VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
auto-test-app-v4-eth0-b1a361c7e9df   4         10.6.0.1/16   2                    3                false     false
auto-test-app-v4-net1-b1a361c7e9df   4         10.7.0.1/16   2                    3                false     false

~# kubectl get spiderippool auto-test-app-v4-eth0-b1a361c7e9df -o jsonpath='{.spec.ips}'
["10.6.168.171-10.6.168.173"]

~# kubectl get spiderippool auto-test-app-v4-net1-b1a361c7e9df -o jsonpath='{.spec.ips}'
["10.7.168.171-10.7.168.173"]

~# kubectl get po -l app=test-app -o wide
NAME                        READY   STATUS    RESTARTS   AGE   IP             NODE    NOMINATED NODE   READINESS GATES
test-app-6f4594ff67-fkqbw   1/1     Running   0          40s   10.6.168.172   node2   <none>           <none>
test-app-6f4594ff67-gwlx8   1/1     Running   0          40s   10.6.168.173   node1   <none>           <none>

~# kubectl exec -ti test-app-6f4594ff67-fkqbw -- ip a
3: eth0@if2: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether ae:fa:5e:d9:79:11 brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 10.6.168.172/16 brd 10.6.255.255 scope global eth0
       valid_lft forever preferred_lft forever
4: veth0@if13: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether 26:6f:22:91:22:f9 brd ff:ff:ff:ff:ff:ff link-netnsid 0
5: net1@if3: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether d6:4b:c2:6a:62:0f brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 10.7.168.173/16 brd 10.7.255.255 scope global net1
       valid_lft forever preferred_lft forever
```

以下命令展示了 Pod 中的多网卡路由信息。Veth 插件会自动协调多张网卡之间的策略路由，解决多网卡之间的通信问题。

```bash
~# kubectl exec -ti test-app-6f4594ff67-fkqbw -- ip rule show
0:  from all lookup local
32764:  from 10.7.168.173 lookup 100
32765:  from all to 10.7.168.173/16 lookup 100
32766:  from all lookup main
32767:  from all lookup default

~# kubectl exec -ti test-app-6f4594ff67-fkqbw -- ip r show main
default via 10.6.0.1 dev eth0

~# kubectl exec -ti test-app-6f4594ff67-fkqbw -- ip route show table 100
default via 10.7.0.1 dev net1
10.6.168.123 dev veth0 scope link
10.7.0.0/16 dev net1 proto kernel scope link src 10.7.168.173
10.96.0.0/12 via 10.6.168.123 dev veth0
```

## 通过 IPPool 实现多网卡

创建两个 IPPool，分别为不同的网卡提供 IP 地址。

```shell
~# kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/different-segment-ipv4-ippools.yaml

~# kubectl get spiderippool
NAME                   VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
ippool-test-ens192     4         10.6.0.1/16   0                    5                false     false
ippool-test-ens224     4         10.7.0.1/16   0                    5                false     false
```

下面的示例 Yaml 会创建一个 1 副本的 Deployment：

```shell
~# kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/ippool-test-deploy.yaml
```

最终，Deployment 创建后，Spiderpool 会从两个指定的 IPPool 中随机选取 IP，分别绑定到两张网卡上。

```bash
~# kubectl get spiderippool
NAME                   VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
ippool-test-ens192     4         10.6.0.1/16   1                    5                false     false
ippool-test-ens224     4         10.7.0.1/16   1                    5                false     false

~# kubectl get po -l app=ippool-test-app -o wide
NAME                               READY   STATUS    RESTARTS   AGE     IP             NODE    NOMINATED NODE   READINESS GATES
ippool-test-app-65f646574c-mpr47   1/1     Running   0          6m18s   10.6.168.175   node2   <none>           <none>

~#  kubectl exec -ti ippool-test-app-65f646574c-mpr47 -- ip a
...
3: eth0@tunl0: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue
    link/ether 2a:ca:ce:06:1e:91 brd ff:ff:ff:ff:ff:ff
    inet 10.6.168.175/16 brd 10.6.255.255 scope global eth0
       valid_lft forever preferred_lft forever
4: veth0@if15: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue
    link/ether 86:ba:6f:97:ae:1b brd ff:ff:ff:ff:ff:ff
5: net1@eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue
    link/ether f2:12:b5:8c:ff:4f brd ff:ff:ff:ff:ff:ff
    inet 10.7.168.177/16 brd 10.7.255.255 scope global net1
       valid_lft forever preferred_lft forever
```

以下命令展示了 Pod 中的多网卡路由信息。Veth 插件会自动协调多张网卡之间的策略路由，解决多网卡之间的通信问题。

```bash
~# kubectl exec -ti ippool-test-app-65f646574c-mpr47 -- ip rule show
0: from all lookup local
32764: from 10.7.168.177 lookup 100
32765: from all to 10.7.168.177/16 lookup 100
32766: from all lookup main
32767: from all lookup default

~# kubectl exec -ti ippool-test-app-65f646574c-mpr47 -- ip r show main
default via 10.6.0.1 dev eth0

~# kubectl exec -ti ippool-test-app-65f646574c-mpr47 -- ip r show table 100
default via 10.7.0.1 dev net1
10.6.168.123 dev veth0 scope link
10.7.0.0/16 dev net1 scope link  src 10.7.168.177
10.96.0.0/12 via 10.6.168.123 dev veth0
```

## 清理

清理相关资源，以便您可以再次运行本教程。

```bash
kubectl delete \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/multus-conf.yaml \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/different-segment-ipv4-subnets.yaml \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/subnet-test-deploy.yaml \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/different-segment-ipv4-ippools.yaml \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/basic/custom-dual-ippool-deploy.yaml \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/ippool-test-deploy.yaml \
--ignore-not-found=true
```
