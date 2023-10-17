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

- Spiderpool's CNI plugins, including [ifacer](./ifacer.md) and [coordinator](../concepts/coordinator.md) are integrated, enhancing the overall configuration experience.

> It is important to note that when creating a SpiderMultusConfig CR with the same name as an existing Multus CR, the Multus CR instance will be managed by SpiderMultusConfig, and its configuration will be overwritten. To avoid overwriting existing Multus CR instances, it is recommended to either refrain from creating SpiderMultusConfig CR instances with the same name or specify a different name for the generated Multus CR using the `multus.spidernet.io/cr-name` annotation in the SpiderMultusConfig CR.

## Prerequisites

1. A ready Kubernetes cluster.

2. [Helm](https://helm.sh/docs/intro/install/) has been installed.

## Steps

### Install Spiderpool

Refer to [Installation](./readme.md) to install Spiderpool.

### Create CNI Configurations

SpiderMultusConfig CR supports various types of CNIs. The following sections explain how to create these configurations.

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

#### Other CNI Configurations

To create other types of CNI configurations, such as SR-IOV and OVS, refer to [SR-IOV](./install/underlay/get-started-sriov.md) and [Ovs](./install/underlay/get-started-ovs.md).

## Conclusion

SpiderMultusConfig CR automates the management of Multus NetworkAttachmentDefinition CRs, improving the experience of creating configurations and reducing operational costs.
