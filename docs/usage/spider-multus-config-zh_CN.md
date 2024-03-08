# SpiderMultusConfig

**简体中文** | [**English**](./spider-multus-config.md)

## 介绍

Spiderpool 提供了 Spidermultusconfig CR 来自动管理 Multus NetworkAttachmentDefinition CR ，实现了对开源项目 Multus CNI 配置管理的扩展。

## SpiderMultusConfig 功能

Multus 是一个 CNI 插件项目，它通过调度第三方 CNI 项目，能够实现为 Pod 接入多张网卡。并且，Multus 可以通过 CRD 方式管理 CNI 配置，避免在每个主机上手动编辑 CNI 配置文件。但创建 Multus CR 时，需要手动书写 JSON 格式的 CNI 配置字符串。将会导致如下问题。

- 人为书写容易出现 JSON 格式错误，增加 Pod 启动失败的排障成本。

- CNI 种类多，并且它们的各个配置项也很多，不容易记忆，经常需要进行资料查阅，用户体验不友好。

Spidermultusconfig CR 基于 `spec` 中的定义自动生成 Multus CR，改进了如上问题，并且具备如下的一些特点：

- 误操作删除 Multus CR，Spidermultusconfig 将会自动重建；提升运维容错能力。

- 支持众多 CNI，如 Macvlan、IPvlan、Ovs、SR-IOV。

- 支持通过注解 `multus.spidernet.io/cr-name` 自定义 Multus CR 的名字。

- 支持通过注解 `multus.spidernet.io/cni-version` 自定义设置 CNI 的版本。

- 完善的 Webhook 机制，提前规避一些人为错误，降低后续排障成本。

- 支持 Spiderpool 的 CNI plugin：[ifacer](../reference/plugin-ifacer.md) 、[coordinator](../concepts/coordinator-zh_CN.md) ，提高了 Spiderpool 的 CNI plugin 的配置体验。

> 在已存在 Multus CR 实例时，创建与其同名 Spidermultusconfig CR ，Multus CR 实例将会被纳管，其配置内容将会被覆盖。如果不想发生被覆盖的情况，请避免创建与存量 Multus CR 实例同名的 Spidermultusconfig CR 实例或者在 Spidermultusconfig CR 中指定 `multus.spidernet.io/cr-name` 以更改自动生成的 Multus CR 的名字。

## 实施要求

1. 一套 Kubernetes 集群。

2. 已安装 [Helm](https://helm.sh/docs/intro/install/)。

## 步骤

### 安装 Spiderpool

可参考 [安装](./readme-zh_CN.md) 安装 Spiderpool.

### 创建 CNI 配置

SpiderMultusConfig CR 支持的 CNI 类型众多，跟随下面章节了解，进行创建。

#### 节点网卡名称一致性

Multus 的 NetworkAttachmentDefinition CR 通过字段 `master` 指定节点上的网卡，当应用使用了该 CR 但应用的多个 Pod 副本被调度到了不同节点上，而有些节点上却不存在 `master` 所指定的网卡名，这将导致一些 Pod 副本无法正常运行。对此，可以参考本章节，将节点上的网卡名称一致化。

在本章节中将使用 udev 来更改节点的网卡名。udev 是 Linux 系统中用于设备管理的子系统，可以通过规则文件来定义设备的属性和行为。下列是通过 udev 更改节点的网卡名的步骤，您需要在每个要更改网卡名称的节点上执行以下操作：：

1. 确定需要变更网卡名称，您可以使用 `ip link show` 查看，并将网卡状态设置为 `down`，例如，本文中的 `ens256` 。

    ```bash
    # 通过 `ip link set` 命令将网卡状态设置为 down，避免在变更网卡名时因 "Device or resource busy" 而失败。
    ~# ip link set ens256 down

    ~# ip link show ens256
    4: ens256: <BROADCAST,MULTICAST> mtu 1500 qdisc mq state DOWN mode DEFAULT group default qlen 1000
        link/ether 00:50:56:b4:99:16 brd ff:ff:ff:ff:ff:ff
    ```

2. 创建 udev 规则文件：在 /etc/udev/rules.d/ 目录中创建一个新的规则文件，例如：`10-network.rules`，并编写 udev 规则，如下。

    ```shell
    ~# vim 10-network.rules
    SUBSYSTEM=="net", ACTION=="add", ATTR{address}=="<MAC地址>", NAME="<新网卡名>"

    # 在上述规则中，您需要将 <MAC地址> 替换为当前要修改网卡的 MAC 地址，将 <新网卡名> 替换为您希望设置的新网卡名。 例如：
    ~# cat 10-network.rules 
    SUBSYSTEM=="net", ACTION=="add", ATTR{address}=="00:50:56:b4:99:16", NAME="eth1"
    ```

3. 使 udev 守护进程重新加载配置文件

   ```bash
   ~# udevadm control --reload-rules
   ```

4. 触发所有设备的 add 事件，使配置生效

   ```bash
   ~# udevadm trigger -c add
   ```

5. 检查网卡名称变更成功。

   ```bash
   ~# ip link set eth1 up

   ~# ip link show eth1
   4: eth1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP mode DEFAULT group default qlen 1000
   link/ether 00:50:56:b4:99:16 brd ff:ff:ff:ff:ff:ff
   ```

注意：在更改网卡名之前，确保了解系统和网络的配置，理解更改可能对其他相关组件或配置产生的影响，并建议备份相关的配置文件和数据。另外，具体的更改步骤可能因 Linux 发行版（文中使用 Centos 7）而有所差异。

#### 创建 Macvlan 配置

如下是创建 Macvlan SpiderMultusConfig 配置的示例：

- master：在此示例用接口 `ens192` 作为 master 的参数。

```bash
MACVLAN_MASTER_INTERFACE="ens192"
MACVLAN_MULTUS_NAME="macvlan-$MACVLAN_MASTER_INTERFACE"

cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: ${MACVLAN_MULTUS_NAME}
  namespace: kube-system
spec:
  cniType: macvlan
  enableCoordinator: true
  macvlan:
    master:
    - ${MACVLAN_MASTER_INTERFACE}
EOF
```

使用如上配置，创建如下的 Macvlan SpiderMultusConfig，并且将基于它自动生成 Multus NetworkAttachmentDefinition CR，并将纳管其生命周期。

```bash
~# kubectl get spidermultusconfigs.spiderpool.spidernet.io -n kube-system
NAME             AGE
macvlan-ens192   26m

~# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system macvlan-ens192 -oyaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"spiderpool.spidernet.io/v2beta1","kind":"SpiderMultusConfig","metadata":{"annotations":{},"name":"macvlan-ens192","namespace":"kube-system"},"spec":{"cniType":"macvlan","enableCoordinator":true,"macvlan":{"master":["ens192"]}}}
  creationTimestamp: "2023-09-11T09:02:43Z"
  generation: 1
  name: macvlan-ens192
  namespace: kube-system
  ownerReferences:
  - apiVersion: spiderpool.spidernet.io/v2beta1
    blockOwnerDeletion: true
    controller: true
    kind: SpiderMultusConfig
    name: macvlan-ens192
    uid: 94bbd704-ff9d-4318-8356-f4ae59856228
  resourceVersion: "5288986"
  uid: d8fa48c8-0877-440d-9b66-88edd7af5808
spec:
  config: '{"cniVersion":"0.3.1","name":"macvlan-ens192","plugins":[{"type":"macvlan","master":"ens192","mode":"bridge","ipam":{"type":"spiderpool"}},{"type":"coordinator"}]}'
```

#### 创建 IPvlan 配置

如下是创建 IPvlan SpiderMultusConfig 配置的示例：

- master：在此示例用接口 `ens192` 作为 master 的参数。

- 使用 IPVlan 做集群 CNI 时，系统内核版本必须大于 4.2。

- 单个主接口不能同时被 Macvlan 和 IPvlan 所奴役。

```bash
IPVLAN_MASTER_INTERFACE="ens192"
IPVLAN_MULTUS_NAME="ipvlan-$IPVLAN_MASTER_INTERFACE"

cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: ${IPVLAN_MULTUS_NAME}
  namespace: kube-system
spec:
  cniType: ipvlan
  enableCoordinator: true
  ipvlan:
    master:
    - ${IPVLAN_MASTER_INTERFACE}
EOF
```

使用如上配置，创建如下的 IPvlan SpiderMultusConfig，并且将基于它自动生成 Multus NetworkAttachmentDefinition CR，并将纳管其生命周期。

```bash
~# kubectl get spidermultusconfigs.spiderpool.spidernet.io -n kube-system
NAME             AGE
ipvlan-ens192    12s

~# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system ipvlan-ens192 -oyaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"spiderpool.spidernet.io/v2beta1","kind":"SpiderMultusConfig","metadata":{"annotations":{},"name":"ipvlan-ens192","namespace":"kube-system"},"spec":{"cniType":"ipvlan","enableCoordinator":true,"ipvlan":{"master":["ens192"]}}}
  creationTimestamp: "2023-09-14T10:21:26Z"
  generation: 1
  name: ipvlan-ens192
  namespace: kube-system
  ownerReferences:
  - apiVersion: spiderpool.spidernet.io/v2beta1
    blockOwnerDeletion: true
    controller: true
    kind: SpiderMultusConfig
    name: ipvlan-ens192
    uid: accac945-9296-440e-abe8-6f6938fdb895
  resourceVersion: "5950921"
  uid: e24afb76-e552-4f73-bab0-8fd345605c2a
spec:
  config: '{"cniVersion":"0.3.1","name":"ipvlan-ens192","plugins":[{"type":"ipvlan","master":"ens192","ipam":{"type":"spiderpool"}},{"type":"coordinator"}]}'
```

### 创建 Sriov 配置

如下是创建 Sriov SpiderMultusConfig 配置的示例：

- 一个基础的例子

```bash
cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: sriov-demo
  namespace: kube-system
spec:
  cniType: sriov
  enableCoordinator: true
  sriov:
    resourceName: spidernet.io/sriov_netdeivce
    vlanID: 100
EOF
```

创建后，查看对应的 Multus NetworkAttachmentDefinition CR:

```shell
~# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system sriov-demo -o yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: sriov-demo
  namespace: kube-system
  annotations:
    k8s.v1.cni.cncf.io/resourceName: spidernet.io/sriov_netdeivce 
  ownerReferences:
  - apiVersion: spiderpool.spidernet.io/v2beta1
    blockOwnerDeletion: true
    controller: true
    kind: SpiderMultusConfig
    name: sriov-demo
    uid: b08ce054-1ae8-414a-b37c-7fd6988b1b8e
  resourceVersion: "153002297"
  uid: 4413e1fa-ce15-4acf-bce8-48b5028c0568
spec:
  config: '{"cniVersion":"0.3.1","name":"sriov-demo","plugins":[{"vlan":100,"type":"sriov","ipam":{"type":"spiderpool"}},{"type":"coordinator"}]}'
```

更多信息可参考 [Sriov-cni 使用](./install/underlay/get-started-sriov-zh_CN.md)

- 配置启用 RDMA 功能

```bash
cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: sriov-rdma
  namespace: kube-system
spec:
  cniType: sriov
  enableCoordinator: true
  sriov:
    enableRdma: true
    resourceName: spidernet.io/sriov_netdeivce
    vlanID: 100
EOF
```

创建后，查看对应的 Multus NetworkAttachmentDefinition CR:

```shell
~# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system sriov-rdma -o yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: sriov-rdma
  namespace: kube-system
  annotations:
    k8s.v1.cni.cncf.io/resourceName: spidernet.io/sriov_netdeivce 
  ownerReferences:
  - apiVersion: spiderpool.spidernet.io/v2beta1
    blockOwnerDeletion: true
    controller: true
    kind: SpiderMultusConfig
    name: sriov-rdma
    uid: b08ce054-1ae8-414a-b37c-7fd6988b1b8e
  resourceVersion: "153002297"
  uid: 4413e1fa-ce15-4acf-bce8-48b5028c0568
spec:
  config: '{"cniVersion":"0.3.1","name":"sriov-rdma","plugins":[{"vlan":100,"type":"sriov","ipam":{"type":"spiderpool"}},{"type":"rdma"},{"type":"coordinator"}]}'
```

- 配置 Sriov 网络带宽

我们可通过 SpiderMultusConfig 配置 Sriov 的网络带宽:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: sriov-bandwidth
  namespace: kube-system
spec:
  cniType: sriov
  enableCoordinator: true
  sriov:
    resourceName: spidernet.io/sriov_netdeivce
    vlanID: 100
    minTxRateMbps: 100
    MaxTxRateMbps: 1000
EOF
```

> `minTxRateMbps` 和 `MaxTxRateMbps` 配置此 CNI 配置文件的网络传输带宽范围为: [100,1000]

创建后，查看对应的 Multus NetworkAttachmentDefinition CR:

```shell
~# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system sriov-rdma -o yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: sriov-bandwidth
  namespace: kube-system
  annotations:
    k8s.v1.cni.cncf.io/resourceName: spidernet.io/sriov_netdeivce 
  ownerReferences:
  - apiVersion: spiderpool.spidernet.io/v2beta1
    blockOwnerDeletion: true
    controller: true
    kind: SpiderMultusConfig
    name: sriov-bandwidth
    uid: b08ce054-1ae8-414a-b37c-7fd6988b1b8e
spec:
  config: '{"cniVersion":"0.3.1","name":"sriov-bandwidth","plugins":[{"vlan":100,"type":"sriov","minTxRate": 100, "maxTxRate": 1000,"ipam":{"type":"spiderpool"}},{"type":"rdma"},{"type":"coordinator"}]}'
```

### Ifacer 使用配置

Ifacer 插件可以帮助我们在创建 Pod 时，自动创建 Bond 网卡 或者 Vlan 网卡，用于承接 Pod 底层网络。更多信息参考 [Ifacer](../reference/plugin-ifacer.md)。

#### **自动创建 Vlan 接口**

如果我们需要 Vlan 子接口承接 Pod 的底层网络，并且该接口在节点尚未被创建。我们可以在 Spidermultusconfig 中注入 vlanID 的配置，这样生成对应的 Multus NetworkAttachmentDefinition CR 时，就会注入
`ifacer` 插件对应的配置，该插件将会在 Pod 创建时，动态的在主机创建 Vlan 接口，用于承接 Pod 的底层网络。

下面我们以 CNI 为 IPVlan，IPVLAN_MASTER_INTERFACE 为 ens192，vlanID 为 100 为配置例子:

```shell
~# IPVLAN_MASTER_INTERFACE="ens192"
~# IPVLAN_MULTUS_NAME="ipvlan-$IPVLAN_MASTER_INTERFACE"
~# cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: ipvlan-ens192-vlan100
  namespace: kube-system
spec:
  cniType: ipvlan
  enableCoordinator: true
  ipvlan:
    master:
    - ${IPVLAN_MASTER_INTERFACE}
    vlanID: 100
EOF
```

当创建成功，查看对应的 Multus network-attachment-definition 对象:

```shell
~# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system macvlan-conf -o=jsonpath='{.spec.config}' | jq
{
  "cniVersion": "0.3.1",
  "name": "ipvlan-ens192-vlan100",
  "plugins": [
    {
      "type": "ifacer",
      "interfaces": [
        "ens192"
      ],
      "vlanID": 100
    },
    {
      "type": "ipvlan",
      "ipam": {
        "type": "spiderpool"
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

> `ifacer` 作为 CNI 链式调用顺序的第一个，最先被调用。 根据配置，`ifacer` 将基于 `ens192` 创建一个 VLAN tag 为 100 的子接口, 名为 ens192.100
>
> main CNI: IPVlan 的 master 字段的值为: `ens192.100`, 也就是通过 `ifacer` 创建的 VLAN 子接口: `ens192.100`
>
> 注意: 通过 `ifacer` 创建的网卡不是持久化的，重启节点或者人为删除将会被丢失。重启 Pod 会自动添加回来。

有时候网络管理员已经创建好 VLAN 子接口，我们不需要使用 `ifacer` 创建 Vlan 子接口 。我们可以直接配置 master 字段为: `ens192.100`，并且不配置 VLAN ID , 如下:

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

#### **自动创建 Bond 网卡**

如果我们需要 Bond 接口承接 Pod 的底层网络，并且该 Bond 接口在节点尚未被创建。我们可以在 Spidermultusconfig 中配置多个 master 接口，这样生成对应的 Multus NetworkAttachmentDefinition CR 时，就会注入
`ifacer` 插件对应的配置，该插件将会在 Pod 创建时，动态的在主机创建 Bond 接口，用于承接 Pod 的底层网络。

下面我们以 CNI 为 IPVlan，主机接口 ens192, ens224 为 slave 创建 Bond 接口为例子:

```shell
~# cat << EOF | kubectl apply -f - 
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: ipvlan-conf
  namespace: kube-system
spec:
  cniType: ipvlan
  macvlan:
    master:
    - ens192
    - ens224
    bond:
      name: bond0
      mode: 1
      options: ""
EOF
```

当创建成功，查看对应的 Multus network-attachment-definition 对象:

```shell
~# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system ipvlan-conf -o jsonpath='{.spec.config}' | jq
{
  "cniVersion": "0.3.1",
  "name": "ipvlan-conf",
  "plugins": [
    {
      "type": "ifacer",
      "interfaces": [
        "ens192"
        "ens224"
      ],
      "bond": {
        "name": "bond0",
        "mode": 1
      }
    },
    {
      "type": "ipvlan",
      "ipam": {
        "type": "spiderpool"
      },
      "master": "bond0",
      "mode": "bridge"
    },
    {
      "type": "coordinator",
    }
  ]
}
```

配置说明:

> `ifacer` 作为 CNI 链式调用顺序的第一个，最先被调用。 根据配置，`ifacer` 将基于 ["ens192","ens224"] 创建一个名为 `bond0` 的 bond 接口，mode 为 1(active-backup)。
>
> IPVlan 作为 main CNI，其 master 字段的值为: `bond0`， bond0 承接 Pod 的网络流量。
> 
> 创建 Bond 如果需要更高级的配置，可以通过配置 SpiderMultusConfig: macvlan-conf.spec.macvlan.bond.options 实现。 输入格式为: "primary=ens160;arp_interval=1",多个参数用";"连接

如果我们需要基于已创建的 Bond 网卡 bond0 创建 Vlan 子接口，以此 Vlan 子接口承接 Pod 的底层网络，可参考以下的配置:

```shell
~# cat << EOF | kubectl apply -f - 
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: ipvlan-conf
  namespace: kube-system
spec:
  cniType: ipvlan
  macvlan:
    master:
    - ens192
    - ens224
    vlanID: 100
    bond:
      name: bond0
      mode: 1
      options: ""
EOF
```

> 当使用以上配置创建 Pod，`ifacer` 会在主机上创建一张 bond 网卡  bond0 以及一张 Vlan 网卡 bond0.100 。

#### 其他 CNI 配置

创建其他 CNI 配置，如 Ovs: 参考 [创建 Ovs](./install/underlay/get-started-ovs-zh_CN.md)

## 总结

SpiderMultusConfig CR 自动管理 Multus NetworkAttachmentDefinition CR，提升了创建体验，降低了运维成本。
