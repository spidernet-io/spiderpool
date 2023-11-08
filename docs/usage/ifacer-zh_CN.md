# CNI meta-plugin: ifacer

[**English**](./ifacer.md) | **简体中文**

## 介绍

当 Pod 使用 VLAN 网络时，我们可能需要网络管理员提前在节点配置各种 VLAN 或 Bond 接口, 这一过程需要人工参与，配置繁琐且易出错。Spiderpool 提供一个名为 `ifacer`的 CNI meta-plugin,
该插件可在创建 Pod 时，根据提供的 `ifacer` 插件配置, 动态的在节点创建 VLAN 子接口 或 Bond 接口，极大的简化了网络管理员的配置工作。下面我们将会详细介绍这个插件。

## 功能

该插件支持:

- 支持动态的创建 VLAN 子接口
- 支持动态的创建 Bond 接口

> 通过该插件创建的 VLAN/Bond 接口，当节点重启时会丢失，但 Pod 重启后会自动创建
> 插件不支持删除已创建的 VLAN/Bond 接口
> 插件不支持在创建时配置 VLAN/Bond 接口的地址

## 使用要求

使用该插件无特殊的限制: 包括 Kubernetes、Kernel 版本等。安装 Spiderpool 时，会自动安装该插件到每个主机的 `/opt/cni/bin/` 路径下, 您可以通过检测每个主机该路径下是否存在 `ifacer` 二进制确认安装成功。

## 如何使用

我们通过两个例子来展示如何使用 `ifacer` 插件。在介绍例子之前，我们需要创建两个 IP 池用于测试:

- vlan100：动态创建 VLAN 子接口

```shell
[root@controller1 ~]# cat << EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: vlan100
spec:
  gateway: 172.100.0.1
  ips:
  - 172.100.0.100-172.100.0.200
  subnet: 172.100.0.0/16
  vlan: 100
EOF
```

- vlan200：动态创建 Bond 接口

```shell
[root@controller1 ~]# cat << EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: vlan200
spec:
  gateway: 172.200.0.1
  ips:
  - 172.200.0.100-172.200.0.200
  subnet: 172.200.0.0/16
  vlan: 200
EOF
```

### 动态创建 VLAN 子接口

我们通过 `SpiderMultusConfig` 为 `ifacer`生成一个对应的 Multus network-attachment-definition 配置:

```shell
[root@controller1 ~]# cat << EOF | kubectl apply -f - 
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: macvlan-conf
  namespace: kube-system
spec:
  cniType: macvlan
  coordinator:
    mode: underlay
  macvlan:
    master:
    - ens192
    vlanID: 100
    ippools:
      ipv4: 
      - vlan100
EOF
spidermultusconfig.spiderpool.spidernet.io/macvlan-conf created
```

当创建成功，查看对应的 Multus network-attachment-definition 对象:

```shell
[root@controller1 ~]# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system macvlan-conf -o=jsonpath='{.spec.config}' | jq
{
  "cniVersion": "0.3.1",
  "name": "macvlan-conf1",
  "plugins": [
    {
      "type": "ifacer",
      "interfaces": [
        "ens192"
      ],
      "vlanID": 100
    },
    {
      "type": "macvlan",
      "ipam": {
        "type": "spiderpool",
        "default_ipv4_ippool": ["vlan100"]
      },
      "master": "ens192.100",
      "mode": "bridge"
    },
    {
      "type": "coordinator",
    }
  ]
}
```

从配置可以看到:

> `ifacer` 作为 CNI 链式调用顺序的第一个，最先被调用。 根据配置，`ifacer` 将基于 `ens192` 创建一个 VLAN tag 为: 100 的子接口
> main CNI: Macvlan 的 master 字段的值为: `ens192.100`, 也就是通过 `ifacer` 创建的 VLAN 子接口: `ens192.100`

如果节点上已经创建好 VLAN 子接口，不需要使用 `ifacer` 。我们可以直接配置 master 字段为: `ens192.100`，并且不配置 VLAN ID 即可, 如下:

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: macvlan-conf
  namespace: kube-system
spec:
  cniType: macvlan
  macvlan:
    master:
    - ens192.100
    ippools:
      ipv4: 
        - vlan100
```

接下来我们创建测试的业务 Pod:

```shell
[root@controller1 ~]# cat << EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      annotations:
        v1.multus-cni.io/default-network: macvlan-conf
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx
        imagePullPolicy: IfNotPresent
        ports:
        - name: http
          containerPort: 80
          protocol: TCP
EOF
```

当 Pod 成功的被创建，我们可以查看该节点一个名为 `ens192.100` 的 VLAN 子接口被成功创建:

```shell
[root@controller1 ~]# kubectl  get po -o wide | grep nginx
nginx-5d6dc85ff4-gdvkh   1/1     Running   0          32s    172.100.0.163    worker1       <none>           <none>
[root@worker1 ~]# ip l show type vlan
135508: ens192.100@ens192: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP mode DEFAULT group default
    link/ether 00:50:56:b4:3c:82 brd ff:ff:ff:ff:ff:ff
```

经过测试，Pod 各种连通性正常。

### 动态的创建 Bond 接口

我们通过 `SpiderMultusConfig` 为 `ifacer` 生成一个对应的 Multus network-attachment-definition 配置:

```shell
[root@controller1 ~]# cat << EOF | kubectl apply -f - 
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: macvlan-conf
  namespace: kube-system
spec:
  cniType: macvlan
  coordinator:
    mode: underlay
  macvlan:
    master:
    - ens192
    - ens160
    vlanID: 200
    ippools:
      ipv4:
      - vlan200
    bond:
      name: bond0
      mode: 1
      options: ""
EOF
spidermultusconfig.spiderpool.spidernet.io/macvlan-conf created
```

当创建成功，查看对应的 Multus network-attachment-definition 对象:

```shell
[root@controller1 ~]# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system macvlan-conf -o jsonpath='{.spec.config}' | jq
{
  "cniVersion": "0.3.1",
  "name": "macvlan-conf",
  "plugins": [
    {
      "type": "ifacer",
      "interfaces": [
        "ens192"
        "ens160"
      ],
      "vlanID": 200,
      "bond": {
        "name": "bond0",
        "mode": 1
      }
    },
    {
      "type": "macvlan",
      "ipam": {
        "type": "spiderpool",
        "default_ipv4_ippool": ["vlan100"]
      },
      "master": "bond0.200",
      "mode": "bridge"
    },
    {
      "type": "coordinator",
    }
  ]
}
```

配置说明:

> `ifacer` 作为 CNI 链式调用顺序的第一个，最先被调用。 根据配置，`ifacer` 将基于 ["ens192","ens160"] 创建一个名为 `bond0` 的 bond 接口，mode 为 1(active-backup)。然后再基于 `bond0` 创建名为 `bond0.200`的 VLAN 子接口, VLAN tag 为 200
> main CNI: Macvlan 的 master 字段的值为: `bond0.200`
> 创建 Bond 如果需要更高级的配置，可以通过配置 SpiderMultusConfig: macvlan-conf.spec.macvlan.bond.options 实现. 输入格式为: "primary=ens160;arp_interval=1",多个参数用";"连接

接下来我们创建测试的业务 Pod:

```shell
[root@controller1 ~]# cat << EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      annotations:
        v1.multus-cni.io/default-network: macvlan-conf
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx
        imagePullPolicy: IfNotPresent
        ports:
        - name: http
          containerPort: 80
          protocol: TCP
EOF
```

当 Pod 成功的被创建，我们可以查看该节点一个名为 `bond0` 的 Bond 接口 和 `bond0.100` 的 VLAN 子接口 被成功创建:

```shell
[root@controller1 ~]# kubectl  get po -o wide | grep nginx
nginx-1c8wcv2sg5-jcwla   1/1     Running   0          32s    172.200.0.163    worker1       <none>           <none>
[root@worker1 ~]# ip --details link show type bond
135510: bond0: <BROADCAST,MULTICAST,MASTER,UP> mtu 1500 qdisc noqueue state UP mode DEFAULT group default qlen 1000
    link/ether 00:50:56:b4:8f:14 brd ff:ff:ff:ff:ff:ff promiscuity 1
    bond mode active-backup miimon 0 updelay 0 downdelay 0 use_carrier 1 arp_interval 1 arp_validate none arp_all_targets any primary ens192 primary_reselect always fail_over_mac none xmit_hash_policy layer2 resend_igmp 1 num_grat_arp 1 all_slaves_active 0 min_links 0 lp_interval 1 packets_per_slave 1 lacp_rate slow ad_select stable tlb_dynamic_lb 1 addrgenmode eui64 numtxqueues 16 numrxqueues 16 gso_max_size 65536 gso_max_segs 65535
[root@worker1 ~]# ip link show type vlan
135508: bond0.100@ens192: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP mode DEFAULT group default
    link/ether 00:50:56:b4:3c:82 brd ff:ff:ff:ff:ff:ff
```

经过测试，Pod 各种连通性正常。
