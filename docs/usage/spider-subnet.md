# SpiderSubnet

The Spiderpool owns a CRD SpiderSubnet, which can help applications (such as Deployment, ReplicaSet, StatefulSet, Job, CronJob, DaemonSet) to create a corresponding SpiderIPPool.

Here are some annotations that you should write down on the application template pod annotation:

| Annotation                         | Description                                                                                               | Example                                                             |
|------------------------------------|-----------------------------------------------------------------------------------------------------------|---------------------------------------------------------------------|
| ipam.spidernet.io/subnet           | Choose one SpiderSubnet V4 and V6 CR to use                                                               | {"ipv4": ["subnet-demo-v4"], "ipv6": ["subnet-demo-v6"]}            |
| ipam.spidernet.io/subnets          | Choose multiple SpiderSubnet V4 and V6 CR to use (the current version only supports to use the first one) | [{"interface":"eth0", "ipv4":["v4-subnet1"],"ipv6":["v6-subnet1"]}] |
| ipam.spidernet.io/ippool-ip-number | The IP numbers of the corresponding SpiderIPPool (fixed and flexible mode)                                | +2                                                                  |
| ipam.spidernet.io/ippool-reclaim   | Specify the corresponding SpiderIPPool to delete or not once the application was deleted (default true)   | true                                                                |

## Notice

1. The annotation `ipam.spidernet.io/subnets` has higher priority over `ipam.spidernet.io/subnet`.
   If you specify both of them two, it only uses `ipam.spidernet.io/subnets` mode.

2. For annotation `ipam.spidernet.io/ippool-ip-number`, you can use '2' for fixed IP number or '+2' for flexible mode.
   The value '+2' means the SpiderSubnet auto-created IPPool will add 2 more IPs based on your application replicas.
   If you choose to use flexible mode, the auto-created IPPool IPs will expand or shrink dynamically by your application replicas.

3. The current version only supports to use one SpiderSubnet V4/V6 CR, you shouldn't specify 2 or more SpiderSubnet V4 CRs and the spiderpool-controller
will choose the first one to use.

## Get Started

### Enable SpiderSubnet feature

Firstly, please ensure you have installed the Spiderpool and configure the CNI file, refer [install](./install.md) for details.

Check configmap `spiderpool-conf` property `enableSpiderSubnet` whether is already set to `true` or not.

```shell
kubectl -n kube-system get configmap spiderpool-conf -o yaml
```

If you want to set it `true`, just execute `helm upgrade spiderpool spiderpool/spiderpool --set feature.enableSpiderSubnet=true -n kube-system`.

### Set cluster Subnet DefaultFlexibleIPNumber

In the upper operation you can also find property `clusterSubnetDefaultFlexibleIPNumber` in configmap `spiderpool-conf`, this is the cluster default flexible IP number.

For example, if you set it `1` and it will be the same with `ipam.spidernet.io/ippool-ip-number: +1`. Furthermore, you could do not specify the annotation
because it will use the `clusterSubnetDefaultFlexibleIPNumber` by default.

If you want to change it, just execute `helm upgrade spiderpool spiderpool/spiderpool --set clusterDefaultPool.subnetFlexibleIPNumber=2 -n kube-system`

### Create a SpiderSubnet

Install a SpiderSubnet example:

```shell
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/spider-subnet/subnet-demo.yaml
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/spider-subnet/deploy-use-subnet.yaml
```

### Check the related CR data

Here's the SpiderSubnet, SpiderIPPool, and Pod information.

```text
$ kubectl get ss
NAME                VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT
subnet-demo-v4      4         172.16.0.0/16             3                    200
subnet-demo-v6      6         fc00:f853:ccd:e790::/64   3                    200

$ kubectl get sp
NAME                                                                           VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DISABLE
auto-deployment-default-demo-deploy-subnet-v4-eth0-6b26cd19032e                4         172.16.0.0/16             1                    3                false
auto-deployment-default-demo-deploy-subnet-v6-eth0-6b26cd19032e                6         fc00:f853:ccd:e790::/64   1                    3                false

$ kubectl get sp auto-deployment-default-demo-deploy-subnet-v4-eth0-6b26cd19032e -o yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  creationTimestamp: "2022-12-26T06:16:04Z"
  finalizers:
  - spiderpool.spidernet.io
  generation: 2
  labels:
    ipam.spidernet.io/interface: eth0
    ipam.spidernet.io/ippool-reclaim: "true"
    ipam.spidernet.io/ippool-version: IPv4
    ipam.spidernet.io/owner-application: Deployment_default_demo-deploy-subnet
    ipam.spidernet.io/owner-application-uid: 9608de8b-fe9b-4a20-9d7c-6b26cd19032e
    ipam.spidernet.io/owner-spider-subnet: subnet-demo-v4
  name: auto-deployment-default-demo-deploy-subnet-v4-eth0-6b26cd19032e
  ownerReferences:
  - apiVersion: spiderpool.spidernet.io/v2beta1
    blockOwnerDeletion: true
    controller: true
    kind: SpiderSubnet
    name: subnet-demo-v4
    uid: ac4ab322-ddde-4a6f-9290-bc795b60ca3f
  resourceVersion: "1690"
  uid: e09ae97c-9693-4d78-b49f-412573e1da95
spec:
  disable: false
  ipVersion: 4
  ips:
  - 172.16.41.1-172.16.41.3
  podAffinity:
    matchLabels:
      app: demo-deploy-subnet
  subnet: 172.16.0.0/16
  vlan: 0
status:
  allocatedIPCount: 1
  allocatedIPs:
    172.16.41.2:
      containerID: 9e5ccb7900f32c6dc76ed3fbd309724ca6eb0235d2a18b2e850c7c91fa28f91d
      interface: eth0
      namespace: default
      node: spider-worker
      ownerControllerName: demo-deploy-subnet
      ownerControllerType: Deployment
      pod: demo-deploy-subnet-b454f5b69-vsc8d
  autoDesiredIPCount: 3
  totalIPCount: 3

$ kubectl get po -o wide 
NAME                                 READY   STATUS    RESTARTS   AGE    IP            NODE            NOMINATED NODE   READINESS GATES
demo-deploy-subnet-b454f5b69-vsc8d   1/1     Running   0          67s    172.16.41.2   spider-worker   <none>           <none>
```

Try to scale the deployment replicas and check the SpiderIPPool.

```text
$ kubectl patch deploy demo-deploy-subnet --patch '{"spec": {"replicas": 2}}'
deployment.apps/demo-deploy-subnet patched
------------------------------------------------------------------------------------------
$ kubectl get ss
NAME                VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT
subnet-demo-v4      4         172.16.0.0/16             4                    200
subnet-demo-v6      6         fc00:f853:ccd:e790::/64   4                    200
------------------------------------------------------------------------------------------
$ kubectl get sp
NAME                                                                           VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DISABLE
auto-deployment-default-demo-deploy-subnet-v4-eth0-6b26cd19032e                4         172.16.0.0/16             2                    4                false
auto-deployment-default-demo-deploy-subnet-v6-eth0-6b26cd19032e                6         fc00:f853:ccd:e790::/64   2                    4                false
------------------------------------------------------------------------------------------
$ kubectl get sp auto-deployment-default-demo-deploy-subnet-v4-eth0-6b26cd19032e -o yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  creationTimestamp: "2022-12-26T06:16:04Z"
  finalizers:
  - spiderpool.spidernet.io
  generation: 3
  labels:
    ipam.spidernet.io/interface: eth0
    ipam.spidernet.io/ippool-reclaim: "true"
    ipam.spidernet.io/ippool-version: IPv4
    ipam.spidernet.io/owner-application: Deployment_default_demo-deploy-subnet
    ipam.spidernet.io/owner-application-uid: 9608de8b-fe9b-4a20-9d7c-6b26cd19032e
    ipam.spidernet.io/owner-spider-subnet: subnet-demo-v4
  name: auto-deployment-default-demo-deploy-subnet-v4-eth0-6b26cd19032e
  ownerReferences:
  - apiVersion: spiderpool.spidernet.io/v2beta1
    blockOwnerDeletion: true
    controller: true
    kind: SpiderSubnet
    name: subnet-demo-v4
    uid: ac4ab322-ddde-4a6f-9290-bc795b60ca3f
  resourceVersion: "2009"
  uid: e09ae97c-9693-4d78-b49f-412573e1da95
spec:
  disable: false
  ipVersion: 4
  ips:
  - 172.16.41.1-172.16.41.3
  - 172.16.41.200
  podAffinity:
    matchLabels:
      app: demo-deploy-subnet
  subnet: 172.16.0.0/16
  vlan: 0
status:
  allocatedIPCount: 2
  allocatedIPs:
    172.16.41.2:
      containerID: 9e5ccb7900f32c6dc76ed3fbd309724ca6eb0235d2a18b2e850c7c91fa28f91d
      interface: eth0
      namespace: default
      node: spider-worker
      ownerControllerName: demo-deploy-subnet
      ownerControllerType: Deployment
      pod: demo-deploy-subnet-b454f5b69-vsc8d
    172.16.41.3:
      containerID: 3eea370feb7557749098f002a72e95377a369e9af53cc1af6dc9f21cbebec82c
      interface: eth0
      namespace: default
      node: spider-control-plane
      ownerControllerName: demo-deploy-subnet
      ownerControllerType: Deployment
      pod: demo-deploy-subnet-b454f5b69-687w4
  autoDesiredIPCount: 4
  totalIPCount: 4
------------------------------------------------------------------------------------------
$ kubectl get po -o wide
NAME                                 READY   STATUS    RESTARTS   AGE     IP            NODE                   NOMINATED NODE   READINESS GATES
demo-deploy-subnet-b454f5b69-687w4   1/1     Running   0          2m35s   172.16.41.3   spider-control-plane   <none>           <none>
demo-deploy-subnet-b454f5b69-vsc8d   1/1     Running   0          4m38s   172.16.41.2   spider-worker          <none>           <none>
```

As you can see, the SpiderSubnet object `subnet-demo-v4` allocates another IP to SpiderIPPool `auto-deployment-default-demo-deploy-subnet-v4-eth0-6b26cd19032e`
and SpiderSubnet object `subnet-demo-v6` allocates another IP to SpiderIPPool `auto-deployment-default-demo-deploy-subnet-v6-eth0-6b26cd19032e`.

### Cluster Default SpiderSubnet

In order to simplify SpiderSubnet usage, we add ClusterDefaultSubnet support.
Once we enable SpiderSubnet feature and have Cluster default subnet, we can create the application directly without any other annotations.

#### Check ClusterDefaultSubnet

Firstly, let's check the configmap `clusterDefaultIPv4Subnet` and `clusterDefaultIPv6Subnet` properties. If there are no values, we can set it by ourselves.

```shell
kubectl -n kube-system get configmap spiderpool-conf -o yaml
```

#### Create Application

Just create the deployment without any other subnet annotations.

```shell
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/spider-subnet/cluster-default-subnet-deployment.yaml
```

#### Notice

1. it will create the auto-create IPPool with the configmap `clusterSubnetDefaultFlexibleIPNumber` property.
2. The ClusterDefaultSubnet function only supports single interface

### SpiderSubnet with multiple interfaces

Make sure the interface name and use `ipam.spidernet.io/subnets` annotation just like this:

```text
      annotations:
        k8s.v1.cni.cncf.io/networks: kube-system/macvlan-cni2
        ipam.spidernet.io/subnets: |-
          [{"interface": "eth0", "ipv4": ["subnet-demo-v4-1"], "ipv6": ["subnet-demo-v6-1"]}, 
           {"interface": "net2", "ipv4": ["subnet-demo-v4-2"], "ipv6": ["subnet-demo-v6-2"]}]
```

Install the example:

```shell
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/spider-subnet/multiple-interfaces.yaml
```
