# SpiderSubnet

The Spiderpool owns a CRD SpiderSubnet, which can help applications (such as Deployment, ReplicaSet, StatefulSet, Job, CronJob, DaemonSet) to create a corresponding SpiderIPPool.

Here are some annotations that you should write down on the application template pod annotation:

| Annotation                         | Description                                                                                               | Example                                                                     |
|------------------------------------|-----------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------|
| ipam.spidernet.io/subnet           | Choose one SpiderSubnet V4 and V6 CR to use                                                               | {{"interface":"eth0", "ipv4":["subnet-demo-v4"], "ipv6":["subnet-demo-v6"]} |
| ipam.spidernet.io/subnets          | Choose multiple SpiderSubnet V4 and V6 CR to use (the current version only supports to use the first one) | [{"interface":"eth0", "ipv4":["v4-subnet1"], "ipv6":["v6-subnet1"]}]        |
| ipam.spidernet.io/ippool-ip-number | The IP numbers of the corresponding SpiderIPPool (fixed and flexible mode, default '+0')                  | +2                                                                          |
| ipam.spidernet.io/ippool-reclaim   | Specify the corresponding SpiderIPPool to delete or not once the application was deleted (default true)   | true                                                                        |

## Notice

1. The annotation `ipam.spidernet.io/subnets` has higher priority over `ipam.spidernet.io/subnet`.
   If you specify both of them two, it only uses `ipam.spidernet.io/subnets` mode.

2. In annotation `ipam.spidernet.io/subnet` mode, it will use default interface name `eth0` if you do not set `interface` property.

3. For annotation `ipam.spidernet.io/ippool-ip-number`, you can use '2' for fixed IP number or '+2' for flexible mode.
   The value '+2' means the SpiderSubnet auto-created IPPool will add 2 more IPs based on your application replicas.
   If you choose to use flexible mode, the auto-created IPPool IPs will expand or shrink dynamically by your application replicas.

4. The current version only supports to use one SpiderSubnet V4/V6 CR, you shouldn't specify 2 or more SpiderSubnet V4 CRs and the spiderpool-controller
will choose the first one to use.

## Get Started

### Enable SpiderSubnet feature

Firstly, please ensure you have installed the Spiderpool and configure the CNI file, refer [install](./install.md) for details.

Check configmap `spiderpool-conf` property `enableSpiderSubnet` whether is already set to `true` or not.

```shell
kubectl -n kube-system get configmap spiderpool-conf -o yaml
```

If you want to set it `true`, just execute `helm upgrade spiderpool spiderpool/spiderpool --set feature.enableSpiderSubnet=true -n kube-system`.

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
NAME                                           VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
auto-demo-deploy-subnet-v4-eth0-6b707f3114b8   4         172.16.0.0/16             1                    3                false     false
auto-demo-deploy-subnet-v6-eth0-6b707f3114b8   6         fc00:f853:ccd:e790::/64   1                    3                false     false
------------------------------------------------------------------------------------------
$ kubectl get sp auto-demo-deploy-subnet-v4-eth0-6b707f3114b8 -o yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  annotations:
    ipam.spidernet.io/application: apps/v1:Deployment:default:demo-deploy-subnet
  creationTimestamp: "2023-03-27T02:21:54Z"
  finalizers:
  - spiderpool.spidernet.io
  generation: 1
  labels:
    ipam.spidernet.io/ippool-cidr: 172-16-0-0-16
    ipam.spidernet.io/ippool-reclaim: "true"
    ipam.spidernet.io/owner-application-uid: 30f7c69c-3c8e-4dbc-b22b-6b707f3114b8
    ipam.spidernet.io/owner-spider-subnet: subnet-demo-v4
  name: auto-demo-deploy-subnet-v4-eth0-6b707f3114b8
  ownerReferences:
  - apiVersion: spiderpool.spidernet.io/v2beta1
    blockOwnerDeletion: true
    controller: true
    kind: SpiderSubnet
    name: subnet-demo-v4
    uid: 4e42b6a5-f8b4-481a-8895-9f18efcc4557
  resourceVersion: "2135"
  uid: 19e2ee81-6718-4416-9cd2-9f3cf5acc5b6
spec:
  default: false
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
  allocatedIPs: '{"172.16.41.1":{"interface":"eth0","pod":"default/demo-deploy-subnet-778786474b-rt7vd","podUid":"dcd1684d-6a5f-4a83-a36d-8e5a56f2c5e2"}}'
  totalIPCount: 3

$ kubectl get po -o wide 
NAME                                  READY   STATUS    RESTARTS   AGE     IP              NODE            NOMINATED NODE   READINESS GATES
demo-deploy-subnet-778786474b-rt7vd   1/1     Running   0          109s    172.16.41.1     spider-worker   <none>           <none>
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
NAME                                           VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
auto-demo-deploy-subnet-v4-eth0-6b707f3114b8   4         172.16.0.0/16             2                    4                false     false
auto-demo-deploy-subnet-v6-eth0-6b707f3114b8   6         fc00:f853:ccd:e790::/64   2                    4                false     false
------------------------------------------------------------------------------------------
$ kubectl get sp auto-demo-deploy-subnet-v4-eth0-6b707f3114b8 -o yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  annotations:
    ipam.spidernet.io/application: apps/v1:Deployment:default:demo-deploy-subnet
  creationTimestamp: "2023-03-27T02:21:54Z"
  finalizers:
  - spiderpool.spidernet.io
  generation: 2
  labels:
    ipam.spidernet.io/ippool-cidr: 172-16-0-0-16
    ipam.spidernet.io/ippool-reclaim: "true"
    ipam.spidernet.io/owner-application-uid: 30f7c69c-3c8e-4dbc-b22b-6b707f3114b8
    ipam.spidernet.io/owner-spider-subnet: subnet-demo-v4
  name: auto-demo-deploy-subnet-v4-eth0-6b707f3114b8
  ownerReferences:
  - apiVersion: spiderpool.spidernet.io/v2beta1
    blockOwnerDeletion: true
    controller: true
    kind: SpiderSubnet
    name: subnet-demo-v4
    uid: 4e42b6a5-f8b4-481a-8895-9f18efcc4557
  resourceVersion: "2632"
  uid: 19e2ee81-6718-4416-9cd2-9f3cf5acc5b6
spec:
  default: false
  disable: false
  ipVersion: 4
  ips:
  - 172.16.41.1-172.16.41.4
  podAffinity:
    matchLabels:
      app: demo-deploy-subnet
  subnet: 172.16.0.0/16
  vlan: 0
status:
  allocatedIPCount: 2
  allocatedIPs: '{"172.16.41.1":{"interface":"eth0","pod":"default/demo-deploy-subnet-778786474b-rt7vd","podUid":"dcd1684d-6a5f-4a83-a36d-8e5a56f2c5e2"},"172.16.41.4":{"interface":"eth0","pod":"default/demo-deploy-subnet-778786474b-82vr8","podUid":"477fd1a3-c416-44e7-9023-064f4aa784a1"}}'
  totalIPCount: 4
------------------------------------------------------------------------------------------
$ kubectl get po -o wide
NAME                                  READY   STATUS    RESTARTS   AGE     IP              NODE                   NOMINATED NODE   READINESS GATES
demo-deploy-subnet-778786474b-82vr8   1/1     Running   0          76s     172.16.41.4     spider-control-plane   <none>           <none>
demo-deploy-subnet-778786474b-rt7vd   1/1     Running   0          3m57s   172.16.41.1     spider-worker          <none>           <none>
```

As you can see, the SpiderSubnet object `subnet-demo-v4` allocates another IP to SpiderIPPool `auto-demo-deploy-subnet-v4-eth0-6b707f3114b8`
and SpiderSubnet object `subnet-demo-v6` allocates another IP to SpiderIPPool `auto-demo-deploy-subnet-v6-eth0-6b707f3114b8`.

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
