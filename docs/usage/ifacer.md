# CNI meta-plugin: ifacer

**English** | [**简体中文**](./ifacer-zh_CN.md)

## Introduction

When Pods use VLAN networks, network administrators may need to manually configure various VLAN or Bond interfaces on the nodes in advance. This process can be tedious and error-prone. Spiderpool provides a CNI meta-plugin called `ifacer`.
This plugin dynamically creates VLAN sub-interfaces or Bond interfaces on the nodes during Pod creation, based on the provided `ifacer` configuration, greatly simplifying the configuration workload. In the following sections, we will delve into this plugin.

## Feature

- Support dynamic creation of VLAN sub-interfaces
- Support dynamic creation of Bond interfaces

> The VLAN/Bond interfaces created by this plugin will be lost when the node restarts, but they will be automatically recreated upon the Pod restarts
> Deleting existed VLAN/Bond interfaces is not supported
> Configuring the address of VLAN/Bond interfaces during creation is not supported

## Prerequisite

There are no specific requirements including Kubernetes or Kernel versions for using this plugin. During the installation of Spiderpool, the plugin will be automatically installed in the `/opt/cni/bin/` directory on each host. You can verify by checking for the presence of the `ifacer` binary in that directory on each host.

## How to use

We will provide two examples to illustrate how to use `ifacer`. But before that, we need to create two IP pools for testing:

- vlan100: dynamically create VLAN sub-interfaces

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

- vlan200：dynamically create Bond interfaces

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

### Dynamically create VLAN sub-interfaces

Generate a Multus network-attachment-definition configuration for `ifacer` using `SpiderMultusConfig`:

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

After creation, you can view the corresponding Multus network-attachment-definition:

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

> `ifacer` is the first one to be invoked in the CNI chain. As per the provided configuration, `ifacer` creates a sub-interface with a VLAN tag of 100 based on `ens192`.
> main CNI: Macvlan's master field is `ens192.100`, namely the VLAN sub-interface created by `ifacer`

`ifacer` is not required if the VLAN sub-interface has already been created on the node. In that case, you can directly configure the master field as `ens192.100` without specifying a VLAN ID, as shown below:

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

Next, create a Pod for testing:

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

Upon successful creation of the Pod, we can verify that a VLAN sub-interface named `ens192.100` has been created on the node:

```shell
[root@controller1 ~]# kubectl  get po -o wide | grep nginx
nginx-5d6dc85ff4-gdvkh   1/1     Running   0          32s    172.100.0.163    worker1       <none>           <none>
[root@worker1 ~]# ip l show type vlan
135508: ens192.100@ens192: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP mode DEFAULT group default
    link/ether 00:50:56:b4:3c:82 brd ff:ff:ff:ff:ff:ff
```

After testing, the Pod shows normal connectivity in various aspects.

### Dynamically create Bond interfaces

Generate a Multus network-attachment-definition configuration for `ifacer` using `SpiderMultusConfig`:

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

Upon successful creation, view the associated Multus network-attachment-definition:

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

Configuration Explanation:

> `ifacer` is the first one to be invoked in the CNI chain. As per the provided configuration, `ifacer` creates a bond interface named `bond0` using ["ens192","ens160"], with the mode of 1 (active-backup). Additionally, a VLAN sub-interface named `bond0.200` is created based on `bond0`, with a VLAN tag of 200.
> main CNI: Macvlan's master field is `bond0.200`
> If advanced configuration options are required for bond creation, you can achieve this by configuring `SpiderMultusConfig: macvlan-conf.spec.macvlan.bond.options`. The input format should be as follows: "primary=ens160;arp_interval=1". Multiple parameters can be connected using ";" as a delimiter.

Create a Pod for testing:

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

Upon successful creation of the Pod, we can verify that a Bond interface named `bond0` and a VLAN sub-interface named `bond0.100` have been created on the node:

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

After testing, the Pod shows normal connectivity in various aspects.
