# SpiderMultusConfig

**English** ｜ [**简体中文**](./spider-multus-config-zh_CN.md)

## Introduction

Spiderpool introduces the SpiderMultusConfig CR to automate the management of Multus NetworkAttachmentDefinition CR and extend the capabilities of Multus CNI configurations.

## SpiderMultusConfig Features

Multus is a CNI plugin project that enables Pods to access multiple network interfaces by leveraging third-party CNI plugins. While Multus allows management of CNI configurations through CRDs, manually writing JSON-formatted CNI configuration strings can lead to error:

- Human errors in writing JSON format may cause troubleshooting difficulties and Pod startup failures.

- There are numerous CNIs with various configuration options, making it difficult to remember them all, and users often need to refer to documentation, resulting in a poor user experience.

To address these issues, SpiderMultusConfig automatically generates the Multus CR based on its `spec`. It offers several features:

- In case of accidental deletion of a Multus CR, SpiderMultusConfig will automatically recreate it, improving operational fault tolerance.

- Support for various CNIs, such as Macvlan, IPvlan, Ovs, and SR-IOV.

- The annotation `multus.spidernet.io/cr-name` allows users to define a custom name for Multus CR.

- The annotation `multus.spidernet.io/cni-version` enables specifying a specific CNI version.

- A robust webhook mechanism is involved to proactively detect and prevent human errors, reducing troubleshooting efforts.

- Spiderpool's CNI plugins, including [ifacer](../reference/plugin-ifacer.md) and [coordinator](../concepts/coordinator.md) are integrated, enhancing the overall configuration experience.

> It is important to note that when creating a SpiderMultusConfig CR with the same name as an existing Multus CR, the Multus CR instance will be managed by SpiderMultusConfig, and its configuration will be overwritten. To avoid overwriting existing Multus CR instances, it is recommended to either refrain from creating SpiderMultusConfig CR instances with the same name or specify a different name for the generated Multus CR using the `multus.spidernet.io/cr-name` annotation in the SpiderMultusConfig CR.

## Prerequisites

1. A ready Kubernetes cluster.

2. [Helm](https://helm.sh/docs/intro/install/) has been installed.

## Steps

### Install Spiderpool

Refer to [Installation](./readme.md) to install Spiderpool.

### Create CNI Configurations

SpiderMultusConfig CR supports various types of CNIs. The following sections explain how to create these configurations.

#### Node NIC name consistency

Multus's NetworkAttachmentDefinition CR specifies the NIC on the node through the field `master`. When an application uses this CR but multiple Pod copies of the application are scheduled to different nodes, and the NIC name specified by `master` does not exist on some nodes, This will cause some Pod replicas to not function properly. In this regard, you can refer to this chapter to make the NIC names on the nodes consistent.

In this chapter, udev will be used to change the NIC name of the node. udev is a subsystem used for device management in Linux systems. It can define device attributes and behaviors through rule files. The following are the steps to change the node NIC name through udev. You need to do the following on each node where you want to change the NIC name:

1. Confirm that the NIC name needs to be changed. You can use the `ip link show` to view it and set the NIC status to `down`, for example, `ens256` in this article.

    ```bash
    # Use the `ip link set` command to set the NIC status to down to avoid failure due to "Device or resource busy" when changing the NIC name.
    ~# ip link set ens256 down

    ~# ip link show ens256
    4: ens256: <BROADCAST,MULTICAST> mtu 1500 qdisc mq state DOWN mode DEFAULT group default qlen 1000
        link/ether 00:50:56:b4:99:16 brd ff:ff:ff:ff:ff:ff
    ```

2. Create a udev rule file: Create a new rule file in the /etc/udev/rules.d/ directory, for example: `10-network.rules`, and write the udev rule as follows.

    ```shell
    ~# vim 10-network.rules
    SUBSYSTEM=="net", ACTION=="add", ATTR{address}=="<MAC address>", NAME="<New NIC name>"

    # In the above rules, you need to replace <MAC address> with the MAC address of the current NIC you want to modify, and replace <new NIC name> with the new NIC name you want to set. For example:
    ~# cat 10-network.rules 
    SUBSYSTEM=="net", ACTION=="add", ATTR{address}=="00:50:56:b4:99:16", NAME="eth1"
    ```

3. Cause the udev daemon to reload the configuration file.

    ```bash
    ~# udevadm control --reload-rules
    ```

4. Trigger the add event of all devices to make the configuration take effect.

    ```bash
    ~# udevadm trigger -c add
    ```

5. Check that the NIC name has been changed successfully.

    ```bash
    ~# ip link set eth1 up

    ~# ip link show eth1
    4: eth1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP mode DEFAULT group default qlen 1000
    link/ether 00:50:56:b4:99:16 brd ff:ff:ff:ff:ff:ff
    ```

Note: Before changing the NIC name, make sure to understand the configuration of the system and network, understand the impact that the change may have on other related components or configurations, and it is recommended to back up related configuration files and data.

The exact steps may vary depending on the Linux distribution (Centos 7 is used in this article).

#### Create Macvlan Configurations

Here is an example of creating Macvlan SpiderMultusConfig configurations:

- master: `ens192` is used as the master interface parameter.

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

Create the Macvlan SpiderMultusConfig using the provided configuration. This will automatically generate the corresponding Multus NetworkAttachmentDefinition CR and manage its lifecycle.

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

- Customize the MTU size of the pod

```shell
~# cat << EOF | kubectl apply -f - 
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: macvlan-mtu
  namespace: kube-system
spec:
  cniType: macvlan
  macvlan:
    master:
    - ens192
    mtu: 1480
EOF
```

> The maximum MTU size of the Pod should not exceed the MTU of the host's network interface. If necessary, you need to modify the MTU of the host's network interface.

you can refer to as follow manifest. When created, view the corresponding Maltus Netwalk-Atahement-De Finity object:

```shell
~# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system macvlan-mtu -oyaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  generation: 1
  name: macvlan-mtu
  namespace: kube-system
  ownerReferences:
  - apiVersion: spiderpool.spidernet.io/v2beta1
    blockOwnerDeletion: true
    controller: true
    kind: SpiderMultusConfig
    name: macvlan-ens192
    uid: 94bbd704-ff9d-4318-8356-f4ae59856228
spec:
  config: '{"cniVersion":"0.3.1","name":"macvlan-ens192","plugins":[{"type":"macvlan","mtu": 1480,"master":"ens192","mode":"bridge","ipam":{"type":"spiderpool"}},{"type":"coordinator"}}'
```

#### Create IPvlan Configurations

Here is an example of creating IPvlan SpiderMultusConfig configurations:

- master: `ens192` is used as the master interface parameter.

- When using IPVlan as the cluster's CNI, the kernel version must be higher than 4.2.

- A single main interface cannot be used by both Macvlan and IPvlan simultaneously.

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

Create the IPvlan SpiderMultusConfig using the provided configuration. This will automatically generate the corresponding Multus NetworkAttachmentDefinition CR and manage its lifecycle.

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

- Customize the MTU size of the pod

```shell
~# cat << EOF | kubectl apply -f - 
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: ipvlan-mtu
  namespace: kube-system
spec:
  cniType: ipvlan
  ipvlan:
    master:
    - ens192
    mtu: 1480
EOF
```

Note: The maximum MTU size of the Pod should not exceed the MTU of the host's network interface. If necessary, you need to modify the MTU of the host's network interface.

you can refer to as follow manifest. When created, view the corresponding Maltus Netwalk-Atahement-De Finity object:

```shell
~# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system ipvlan-mtu -oyaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  generation: 1
  name: ipvlan-mtu
  namespace: kube-system
  ownerReferences:
  - apiVersion: spiderpool.spidernet.io/v2beta1
    blockOwnerDeletion: true
    controller: true
    kind: SpiderMultusConfig
    name: ipvlan-ens192
    uid: 94bbd704-ff9d-4318-8356-f4ae59856228
spec:
  config: '{"cniVersion":"0.3.1","name":"ipvlan-ens192","plugins":[{"type":"ipvlan","mtu": 1480,"master":"ens192","mode":"bridge","ipam":{"type":"spiderpool"}},{"type":"coordinator"}}'
```

### Create Sriov Configuration

Here is an example of creating Sriov SpiderMultusConfig configuration:

- Basic example

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

After creation, check the corresponding Multus NetworkAttachmentDefinition CR:

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

For more information, refer to [sriov-cni usage](./install/underlay/get-started-sriov.md)

- Enable RDMA feature

```shell
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

After creation, check the corresponding Multus NetworkAttachmentDefinition CR:

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

- Configure Sriov-CNI Network Bandwidth

You can configure the network bandwidth of Sriov through SpiderMultusConfig:

```shell
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

> minTxRateMbps and maxTxRateMbps configure the transmission bandwidth range for pods created with this configuration: [100,1000].

After creation, check the corresponding Multus NetworkAttachmentDefinition CR:

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
  config: '{"cniVersion":"0.3.1","name":"sriov-bandwidth","plugins":[{"vlan":100,"type":"sriov","min_tx_rate": 100, "max_tx_rate": 1000,"ipam":{"type":"spiderpool"}},{"type":"rdma"},{"type":"coordinator"}]}'
```

- Configure the mtu size of the Sriov VF

```bash
cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: sriov-mtu
  namespace: kube-system
spec:
  cniType: sriov
  enableCoordinator: true
  sriov:
    resourceName: spidernet.io/sriov_netdeivce
    vlanID: 100
    mtu: 8000
EOF
```


Note: The maximum MTU size of the Pod should not exceed the MTU of the PF's network interface. If necessary, you need to modify the MTU of the host's network interface.

After creation, check the corresponding Multus NetworkAttachmentDefinition CR:

```shell
~# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system sriov-mtu -o yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: sriov-mtu
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
  config: '{"cniVersion":"0.3.1","name":"sriov-bandwidth","plugins":[{"vlan":100,"type":"sriov","ipam":{"type":"spiderpool"}},{"type":"rdma"},{"type":"coordinator"},{"type":"tuning","mtu":8000}]}'
```

### Ifacer Configurations

The Ifacer plug-in can help us automatically create a Bond NIC or VLAN NIC when creating a pod to undertake the pod's underlying network. For more information, refer to [Ifacer](../reference/plugin-ifacer.md).

#### Ifacer create vlan interface

If you need a VLAN sub-interface to take over the underlying network of the pod, and the interface has not yet been created on the node. You can inject the configuration of the vlanID in Spidermultusconfig
so that when the corresponding Multus NetworkAttachmentDefinition CR is generated, it will be injected The `ifacer` plug-in will dynamically create a VLAN interface on the host when the pod is created,
which is used to undertake the pod's underlay network.

The following is an example of CNI as IPVlan, IPVLAN_MASTER_INTERFACE as ens192, and vlanID as 100.

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

When the Spidermultuconfig object is created, view the corresponding Multus network-attachment-definition object:

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

> `ifacer` is called first in the CNI chaining sequence. Depending on the configuration, `ifacer` will create a sub-interface with a VLAN tag of 100 named ens192.100 based on `ens192`.
>
> main CNI: The value of the master field of IPVlan is: `ens192.100`, which is the VLAN sub-interface created by 'ifacer': `ens192.100`.
>
> Note: The NIC created by `ifacer` is not persistent, and will be lost if the node is restarted or manually deleted. Restarting the pod is automatically added back.

Sometimes the network administrator has already created the VLAN sub-interface, and you don't need to use `ifacer` to create the VLAN sub-interface. You can directly configure the master
field as: `ens192.100` and not configure the VLAN ID, as follows:

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

#### Ifacer create bond interface

If you need a bond interface to take over the underlying network of the pod, and the bond interface has not yet been created on the node. You can configure multiple master interfaces in
Spidermultusconfig so that the corresponding Multus NetworkAttachmentDefinition CR is generated and injected The `ifacer'` plug-in will dynamically create a bond interface on the host
when the pod is created, which is used to undertake the underlying network of the pod.

The following is an example of CNI as IPVlan, host interface ens192, and ens224 as slave to create a bond interface:

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

When the Spidermultuconfig object is created, view the corresponding Multus network-attachment-definition object:

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

Configuration description:

> `ifacer` is called first in the CNI chaining sequence. Depending on the configuration, `ifacer` will create a bond interface named 'bond0' based on ["ens192","ens224"] with mode 1 (active-backup).
>
> main CNI: The value of the master field of IPvlan is: `bond0`, bond0 takes over the network traffic of the pod.
>
> Create a Bond If you need a more advanced configuration, you can do so by configuring SpiderMultusConfig: macvlan-conf.spec.macvlan.bond.options. The input format is: "primary=ens160; arp_interval=1", use ";" for multiple parameters.

If you need to create a VLAN sub-interface based on the created BOND NIC: bond0, so that the VLAN sub-interface undertakes the underlying network of the pod, you can refer to the following configuration:

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

> When creating a pod with the above configuration, `ifacer` will create a bond NIC bond0 and a VLAN NIC bond0.100 on the host.

#### Other CNI Configurationsi

To create other types of CNI configurations, such OVS, refer to [Ovs](./install/underlay/get-started-ovs.md).

#### ChainCNI Configuration

If you need to add ChainCNI to the CNI configuration, for example, you need to use the tuning plugin to configure the system kernel parameters of the pod (such as net.core.somaxconn, etc.) or mtu size of the pod. This can be achieved with the following configuration (MacVlan CNI as an example):

1. Configure the sysctl kernel parameter

Create a SpiderMultusConfig CR:

```shell
~# cat << EOF | kubectl apply -f - 
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: macvlan-conf
  namespace: kube-system
spec:
  cniType: macvlan
  macvlan:
    master:
    - ens192
  chainCNIJsonData:
  - |
    {
        "type": "tuning",
        "sysctl": {
          "net.core.somaxconn": "4096"
        }
    }
EOF
```

Note that every element of Shanknick Senda must be a legitimate Ethan string, you can refer to as follow manifest. When created, view the corresponding Maltus Netwalk-Atahement-De Finity object:

```shell
~# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system macvlan-ens192 -oyaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  generation: 1
  name: macvlan-conf
  namespace: kube-system
  ownerReferences:
  - apiVersion: spiderpool.spidernet.io/v2beta1
    blockOwnerDeletion: true
    controller: true
    kind: SpiderMultusConfig
    name: macvlan-ens192
    uid: 94bbd704-ff9d-4318-8356-f4ae59856228
spec:
  config: '{"cniVersion":"0.3.1","name":"macvlan-ens192","plugins":[{"type":"macvlan","master":"ens192","mode":"bridge","ipam":{"type":"spiderpool"}},{"type":"coordinator"},{"type":"tuning", "sysctl": {"net.core.somaxconn": "4096"}}]}'
```

## Conclusion

SpiderMultusConfig CR automates the management of Multus NetworkAttachmentDefinition CRs, improving the experience of creating configurations and reducing operational costs.
