# SpiderSubnet

**English** ｜ [**简体中文**](./spider-subnet-zh_CN.md)

## Introduction

The SpiderSubnet resource represents a set of IP addresses. When application administrators need to allocate fixed IP addresses for their applications, they usually have to rely on platform administrators to provide the available IPs and routing information. However, this collaboration between different operational teams can lead to complex workflows for creating each application.
With the Spiderpool's SpiderSubnet, this process is greatly simplified. SpiderSubnet can automatically allocate IP addresses from the subnet to SpiderIPPool, while also allowing applications to have fixed IP addresses. This automation significantly reduces operational costs and streamlines the workflow.

## SpiderSubnet features

When the Subnet feature is enabled, each instance of IPPool belongs to the same subnet as the Subnet instance. The IP addresses in the IPPool instance must be a subset of those in the Subnet instance, and there should be no overlapping IP addresses among different IPPool instances. By default, the routing configuration of IPPool instances inherits the settings from the corresponding Subnet instance.

To allocate fixed IP addresses for applications and decouple the roles of application administrators and their network counterparts, the following two practices can be adopted:

- Manually create IPPool: application administrators manually create IPPool instances, ensuring that the range of available IP addresses are defined in the corresponding Subnet instance. This allows them to have control over which specific IP addresses are used.

- Automatically create IPPool: application administrators can specify the name of the Subnet instance in the Pod annotation. Spiderpool automatically creates an IPPool instance with fixed IP addresses coming from the Subnet instance. The IP addresses in the instance are then allocated to Pods. Spiderpool also monitors application scaling and deletion events, automatically adjusting the IP pool size or removing IPs as needed.

SpiderSubnet also supports several controllers, including ReplicaSet, Deployment, StatefulSet, DaemonSet, Job, CronJob, and k8s extended operator. If you need to use a third-party controller, you can refer to the doc [Spiderpool supports operator](./operator.md).

This feature does not support the bare Pod.

> Notice: Before v0.7.0 version, you have to create a SpiderSubnet resource before you create a SpiderIPPool resource with SpiderSubnet feature enabled. Since v0.7.0 version, you can create an orphan SpiderIPPool without a SpiderSubnet resource.

## Prerequisites

1. A ready Kubernetes cluster.

2. [Helm](https://helm.sh/docs/intro/install/) has already been installed.

## Steps

### Install Spiderpool

Refer to [Installation](./readme.md) to install Spiderpool. And make sure that the helm installs the option `--ipam.spiderSubnet.enable=true --ipam.spiderSubnet.autoPool.enable=true`. The `ipam.spiderSubnet.autoPool.enable` provide the `Automatically create IPPool` ability.

### Install CNI

To simplify the creation of JSON-formatted Multus CNI configuration, Spiderpool introduces the SpiderMultusConfig CR, which automates the management of Multus NetworkAttachmentDefinition CRs. Here is an example of creating a Macvlan SpiderMultusConfig:

- master: the interface `ens192` is used as the spec for master.

```bash
MACVLAN_MASTER_INTERFACE0="ens192"
MACVLAN_MULTUS_NAME0="macvlan-$MACVLAN_MASTER_INTERFACE0"
MACVLAN_MASTER_INTERFACE1="ens224"
MACVLAN_MULTUS_NAME1="macvlan-$MACVLAN_MASTER_INTERFACE1"

cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: ${MACVLAN_MULTUS_NAME0}
  namespace: kube-system
spec:
  cniType: macvlan
  enableCoordinator: true
  macvlan:
    master:
    - ${MACVLAN_MASTER_INTERFACE0}
---
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: ${MACVLAN_MULTUS_NAME1}
  namespace: kube-system
spec:
  cniType: macvlan
  enableCoordinator: true
  macvlan:
    master:
    - ${MACVLAN_MASTER_INTERFACE1}
EOF
```

With the provided configuration, we create the following two Macvlan SpiderMultusConfigs that will automatically generate a Multus NetworkAttachmentDefinition CR corresponding to the host's `ens192` and `ens224` network interfaces.

```bash
~# kubectl get spidermultusconfigs.spiderpool.spidernet.io -n kube-system
NAME             AGE
macvlan-ens192   26m
macvlan-ens224   26m

~# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system
NAME             AGE
macvlan-ens192   27m
macvlan-ens224   27m
```

### Create Subnets

```bash
~# cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderSubnet
metadata:
  name: subnet-6
spec:
  subnet: 10.6.0.0/16
  gateway: 10.6.0.1
  ips:
    - 10.6.168.101-10.6.168.110
  routes:
    - dst: 10.7.0.0/16
      gw: 10.6.0.1
---
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderSubnet
metadata:
  name: subnet-7
spec:
  subnet: 10.7.0.0/16
  gateway: 10.7.0.1
  ips:
    - 10.7.168.101-10.7.168.110
  routes:
    - dst: 10.6.0.0/16
      gw: 10.7.0.1
EOF
```

Apply the above YAML configuration to create two SpiderSubnet instances and configure gateway and routing information for each of them.

```bash
~# kubectl get spidersubnet
NAME       VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT
subnet-6   4         10.6.0.0/16   0                    10
subnet-7   4         10.7.0.0/16   0                    10

~# kubectl get spidersubnet subnet-6  -o jsonpath='{.spec}' | jq
{
  "gateway": "10.6.0.1",
  "ipVersion": 4,
  "ips": [
    "10.6.168.101-10.6.168.110"
  ],
  "routes": [
    {
      "dst": "10.7.0.0/16",
      "gw": "10.6.0.1"
    }
  ],
  "subnet": "10.6.0.0/16",
  "vlan": 0
}

~# kubectl get spidersubnet subnet-7  -o jsonpath='{.spec}' | jq
{
  "gateway": "10.7.0.1",
  "ipVersion": 4,
  "ips": [
    "10.7.168.101-10.7.168.110"
  ],
  "routes": [
    {
      "dst": "10.6.0.0/16",
      "gw": "10.7.0.1"
    }
  ],
  "subnet": "10.7.0.0/16",
  "vlan": 0
}
```

### Automatically fix IPs for single NIC

The following YAML example creates two replicas of a Deployment application:

- `ipam.spidernet.io/subnet`: specifies the Spiderpool subnet. Spiderpool automatically selects IP addresses from this subnet to create a fixed IP pool associated with the application, ensuring fixed IP assignment. (Notice: this feature don't support wildcard.)

- `ipam.spidernet.io/ippool-ip-number`: specifies the number of IP addresses in the IP pool. This annotation can be written in two ways: specifying a fixed quantity using a numeric value, such as `ipam.spidernet.io/ippool-ip-number：1`, or specifying a relative quantity using a plus and a number, such as `ipam.spidernet.io/ippool-ip-number：+1`. The latter means that the IP pool will dynamically maintain an additional IP address based on the number of replicas, ensuring temporary IPs are available during elastic scaling.

- `ipam.spidernet.io/ippool-reclaim`: indicate whether the automatically created fixed IP pool should be reclaimed upon application deletion.

- `v1.multus-cni.io/default-network`: create a default network interface for the application.

```shell
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app-1
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test-app-1
  template:
    metadata:
      annotations:
        ipam.spidernet.io/subnet: |-
            {      
              "ipv4": ["subnet-6"]
            }
        ipam.spidernet.io/ippool-ip-number: '+1'
        v1.multus-cni.io/default-network: kube-system/macvlan-ens192
        ipam.spidernet.io/ippool-reclaim: "false"
      labels:
        app: test-app-1
    spec:
      containers:
      - name: test-app-1
        image: nginx
        imagePullPolicy: IfNotPresent
        ports:
        - name: http
          containerPort: 80
          protocol: TCP
EOF
```

When creating the application, Spiderpool selects random IP addresses from the specified subnet to create a fixed IP pool that is bound to the Pod's network interface. The automatic pool automatically inherits the gateway and routing of the subnet.

```bash
~# kubectl get spiderippool
NAME                          VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
auto4-test-app-1-eth0-a5bd3   4         10.6.0.0/16   2                    3                false

~# kubectl get po -l app=test-app-1 -o wide
NAME                         READY   STATUS    RESTARTS   AGE   IP             NODE                NOMINATED NODE   READINESS GATES
test-app-1-74cbbf654-2ndzl   1/1     Running   0          46s   10.6.168.101   controller-node-1   <none>           <none>
test-app-1-74cbbf654-4f2w2   1/1     Running   0          46s   10.6.168.103   worker-node-1       <none>           <none>

~# kubectl get spiderippool auto4-test-app-1-eth0-a5bd3 -ojsonpath={.spec} | jq
{
  "default": false,
  "disable": false,
  "gateway": "10.6.0.1",
  "ipVersion": 4,
  "ips": [
    "10.6.168.101-10.6.168.103"
  ],
  "podAffinity": {
    "matchLabels": {
      "ipam.spidernet.io/app-api-group": "apps",
      "ipam.spidernet.io/app-api-version": "v1",
      "ipam.spidernet.io/app-kind": "Deployment",
      "ipam.spidernet.io/app-name": "test-app-1",
      "ipam.spidernet.io/app-namespace": "default"
    }
  },
  "routes": [
    {
      "dst": "10.7.0.0/16",
      "gw": "10.6.0.1"
    }
  ],
  "subnet": "10.6.0.0/16",
  "vlan": 0
}
```

To achieve the desired fixed IP pool, Spiderpool adds built-in labels and PodAffinity to bind the pool to the specific application. With the annotation of `ipam.spidernet.io/ippool-reclaim: false`, IP addresses are reclaimed upon application deletion, but the automatic pool itself remains intact. If you want the pool to be available for other applications, you need to manually remove these built-in labels and PodAffinity.

```bash
Additional Labels:
  ipam.spidernet.io/owner-application-gv
  ipam.spidernet.io/owner-application-kind
  ipam.spidernet.io/owner-application-namespace
  ipam.spidernet.io/owner-application-name
  ipam.spidernet.io/owner-application-uid

Additional PodAffinity:
  ipam.spidernet.io/app-api-group
  ipam.spidernet.io/app-api-version
  ipam.spidernet.io/app-kind
  ipam.spidernet.io/app-namespace
  ipam.spidernet.io/app-name
```

After multiple tests and Pod restarts, the Pod's IP remains fixed within the IP pool range:

```bash
~# kubectl delete po -l app=test-app-1

~# kubectl get po -l app=test-app-1 -o wide
NAME                         READY   STATUS    RESTARTS   AGE   IP             NODE                NOMINATED NODE   READINESS GATES
test-app-1-74cbbf654-7v54p   1/1     Running   0          7s    10.6.168.101   worker-node-1       <none>           <none>
test-app-1-74cbbf654-qzxp7   1/1     Running   0          7s    10.6.168.102   controller-node-1   <none>           <none>
```

### Fixed IP Pool Name

Fixed IP pools are automatically created based on applications, so they need unique and easily queryable names. Currently, the naming convention follows this pattern: `auto{ipVersion}-{appName}-{NicName}-{Max5RandomCharacter}`

- ipVersion: Indicates whether it's an IPv4 or IPv6 pool, value is either 4 or 6
- appName: Represents the application name
- NicName: Represents the network interface name assigned to the POD
- Max5RandomCharacter: Represents a 5-character random string generated from the application's UUID to distinguish fixed IP pools of different applications

For example: If you create a deployment named nginx with network interface eth0, its fixed IP pool name would be `auto4-nginx-eth0-9a2b3`

### Dynamically scale fixed IP pools

When creating the application, the annotation `ipam.spidernet.io/ippool-ip-number`: '+1' is specified to allocate one extra fixed IP compared to the number of replicas. This configuration prevents any issues during rolling updates, ensuring that new Pods have available IPs while the old Pods are not deleted yet.

Let's consider a scaling scenario where the replica count increases from 2 to 3. In this case, the two fixed IP pools associated with the application will automatically scale from 3 IPs to 4 IPs, maintaining one redundant IP address as expected:

```bash
~# kubectl scale deploy test-app-1 --replicas 3
deployment.apps/test-app-1 scaled

~# kubectl get po -l app=test-app-1 -o wide
NAME                         READY   STATUS    RESTARTS   AGE   IP             NODE                NOMINATED NODE   READINESS GATES
test-app-1-74cbbf654-7v54p   1/1     Running   0          54s   10.6.168.101   worker-node-1       <none>           <none>
test-app-1-74cbbf654-9w8gd   1/1     Running   0          19s   10.6.168.103   worker-node-1       <none>           <none>
test-app-1-74cbbf654-qzxp7   1/1     Running   0          54s   10.6.168.102   controller-node-1   <none>           <none>

~# kubectl get spiderippool
NAME                          VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
auto4-test-app-1-eth0-a5bd3   4         10.6.0.0/16   3                    4                false
```

With the information mentioned, scaling the application in Spiderpool is as simple as adjusting the replica count for the application.

### Automatically reclaim IP pools

During application creation, the annotation `ipam.spidernet.io/ippool-reclaim` is specified. Its default value of `true` indicates that when the application is deleted, the corresponding automatic pool is also removed. However, `false` in this case means that upon application deletion, the assigned IPs within the automatically created fixed IP pool will be reclaimed, while retaining the pool itself.

For scenarios requiring pool retention, when the application is recreated with the same name as a deployment or statefulset, it can continue to use the previously created IP pool, maintaining IP consistency.

```bash
~# kubectl delete deploy test-app-1
deployment.apps "test-app-1" deleted

~# kubectl get spiderippool
NAME                          VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
auto4-test-app-1-eth0-a5bd3   4         10.6.0.0/16   0                    4                false
```

With the provided application YAML, creating an application with the same name again will automatically reuse the existing IP pools. Instead of creating new IP pools, the previously created ones will be utilized. This ensures consistency in the replica count and the IP allocation within the pool.

```bash
~# kubectl get spiderippool
NAME                          VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
auto4-test-app-1-eth0-a5bd3   4         10.6.0.0/16   2                    3                false
```

```bash
~# kubectl get po -l app=test-app-1 -o wide
NAME                         READY   STATUS    RESTARTS   AGE   IP             NODE                NOMINATED NODE   READINESS GATES
test-app-1-74cbbf654-2ndzl   1/1     Running   0          46s   10.6.168.101   controller-node-1   <none>           <none>
test-app-1-74cbbf654-4f2w2   1/1     Running   0          46s   10.6.168.103   worker-node-1       <none>           <none>
test-app-1-74cbbf654-qzxp7   1/1     Running   0          7s    10.6.168.102   controller-node-1   <none>           <none>
```

### Automatically fix IPs for multiple NICs

To assign fixed IPs to the multiple NICs of Pods, follow the instructions in this section. In the example YAML below, a Deployment with two replicas is created, each having multiple network interfaces. The annotations therein include:

- `ipam.spidernet.io/subnets`: specify the subnet for Spiderpool. Spiderpool will randomly select IPs from this subnet to create fixed IP pools associated with the application, ensuring persistent IP assignment. In this example, this annotation creates two fixed IP pools belonging to two different underlay subnets for the Pods.

- `v1.multus-cni.io/default-network`: create a default network interface for the application.

- `k8s.v1.cni.cncf.io/networks`: create an additional network interface for the application.

```bash
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app-2
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test-app-2
  template:
    metadata:
      annotations:
        ipam.spidernet.io/subnets: |-
         [
            {      
              "interface": "eth0",
              "ipv4": ["subnet-6"]
            },{      
              "interface": "net1",
              "ipv4": ["subnet-7"]
            }
         ]
        v1.multus-cni.io/default-network: kube-system/macvlan-ens192
        k8s.v1.cni.cncf.io/networks: kube-system/macvlan-ens224
      labels:
        app: test-app-2
    spec:
      containers:
      - name: test-app-2
        image: nginx
        imagePullPolicy: IfNotPresent
        ports:
        - name: http
          containerPort: 80
          protocol: TCP
EOF
```

When creating the application, Spiderpool randomly selects IPs from the specified two Underlay subnets to create fixed IP pools. These pools are then associated with the two network interfaces of the application's Pods. Each network interface's fixed pool automatically inherits the gateway, routing, and other properties of its respective subnet.

```bash
~# kubectl get spiderippool
NAME                          VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
auto4-test-app-2-eth0-44037   4         10.6.0.0/16   2                    3                false
auto4-test-app-2-net1-44037   4         10.7.0.0/16   2                    3                false

~# kubectl get po -l app=test-app-2 -o wide
NAME                          READY   STATUS    RESTARTS   AGE     IP             NODE                NOMINATED NODE   READINESS GATES
test-app-2-f5d6b8d6c-8hxvw    1/1     Running   0          6m22s   10.6.168.101   controller-node-1   <none>           <none>
test-app-2-f5d6b8d6c-rvx55    1/1     Running   0          6m22s   10.6.168.105   worker-node-1       <none>           <none>

~# kubectl get spiderippool auto4-test-app-2-eth0-44037 -ojsonpath={.spec} | jq
{
  "default": false,
  "disable": false,
  "gateway": "10.6.0.1",
  "ipVersion": 4,
  "ips": [
    "10.6.168.101",
    "10.6.168.105-10.6.168.106"
  ],
  "podAffinity": {
    "matchLabels": {
      "ipam.spidernet.io/app-api-group": "apps",
      "ipam.spidernet.io/app-api-version": "v1",
      "ipam.spidernet.io/app-kind": "Deployment",
      "ipam.spidernet.io/app-name": "test-app-2",
      "ipam.spidernet.io/app-namespace": "default"
    }
  },
  "routes": [
    {
      "dst": "10.7.0.0/16",
      "gw": "10.6.0.1"
    }
  ],
  "subnet": "10.6.0.0/16",
  "vlan": 0
}

~# kubectl get spiderippool auto4-test-app-2-net1-44037 -ojsonpath={.spec} | jq
{
  "default": false,
  "disable": false,
  "gateway": "10.7.0.1",
  "ipVersion": 4,
  "ips": [
    "10.7.168.101-10.7.168.103"
  ],
  "podAffinity": {
    "matchLabels": {
      "ipam.spidernet.io/app-api-group": "apps",
      "ipam.spidernet.io/app-api-version": "v1",
      "ipam.spidernet.io/app-kind": "Deployment",
      "ipam.spidernet.io/app-name": "test-app-2",
      "ipam.spidernet.io/app-namespace": "default"
    }
  },
  "routes": [
    {
      "dst": "10.6.0.0/16",
      "gw": "10.7.0.1"
    }
  ],
  "subnet": "10.7.0.0/16",
  "vlan": 0
}
```

SpiderSubnet also supports dynamic IP scaling for multiple network interfaces and automatic reclamation of IP pools.

### Manually create IPPool instances inheriting the subnet's properties

Below is an example of creating an IPPool instance that inherits the properties of `subnet-6` with a subnet ID of `10.6.0.0/16`. The available IP range of this IPPool instance must be a subset of `subnet-6.spec.ips`.

```bash
~# cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: ippool-test
spec:
  ips:
  - 10.6.168.108-10.6.168.110
  subnet: 10.6.0.0/16
EOF
```

Using the provided YAML, you can manually create an IPPool instance that will inherit the attributes of the subnet having the specified subnet ID, such as gateway, routing, and other properties.

```bash
~# kubectl get spiderippool
NAME          VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
ippool-test   4         10.6.0.0/16   0                    3                false

~# kubectl get spidersubnet
NAME       VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT
subnet-6   4         10.6.0.0/16   3                    10
subnet-7   4         10.7.0.0/16   0                    10

~# kubectl get spiderippool ippool-test  -o jsonpath='{.spec}' | jq
{
  "default": false,
  "disable": false,
  "gateway": "10.6.0.1",
  "ipVersion": 4,
  "ips": [
    "10.6.168.108-10.6.168.110"
  ],
  "routes": [
    {
      "dst": "10.7.0.0/16",
      "gw": "10.6.0.1"
    }
  ],
  "subnet": "10.6.0.0/16",
  "vlan": 0
}
```

## Conclusion

SpiderSubnet helps to separate the roles of infrastructure administrators and their application counterparts by enabling automatic creation and dynamic scaling of fixed IP pools for applications that require static IPs.
