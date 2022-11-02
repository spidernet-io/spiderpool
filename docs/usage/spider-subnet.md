# SpiderSubnet

## Description

The spiderpool owns a CRD SpiderSubnet, it can help applications (such as: Deployment, ReplicaSet, StatefulSet, Job, CronJob, DaemonSet.) to create a corresponding SpiderIPPool.

Here are some annotations that you should write down on the application template pod annotation:

| annotation                         | description                                                                                               | example                                                                                       |
|------------------------------------|-----------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------|
| ipam.spidernet.io/subnet           | choose one SpiderSubnet V4 and V6 CR to use                                                               | {"ipv4": ["subnet-demo-v4","good"], "ipv6": ["subnet-demo-v6"]}                               |
| ipam.spidernet.io/subnets          | choose multiple SpiderSubnet V4 and V6 CR to use (the current version only supports to use the first one) | [{"interface":"eth0", "ipv4":["v4-subnet1","v4-subnet2"],"ipv6":["v6-subnet1","v6-subnet2"]}] |
| ipam.spidernet.io/ippool-ip-number | the IP numbers of the corresponding SpiderIPPool (fixed and flexible mode)                                | +2                                                                                            |
| ipam.spidernet.io/ippool-reclaim   | specify the corresponding SpiderIPPool to delete or not once the application was deleted (default true)   | true                                                                                          |

### Notice

1. The annotation `ipam.spidernet.io/subnets` has higher priority over `ipam.spidernet.io/subnet`.
If you specify both of them two, it only uses `ipam.spidernet.io/subnets` mode.

2. For annotation `ipam.spidernet.io/ippool-ip-number`, you can use '2' for fixed IP number or '+2' for flexible mode.
The value '+2' means the SpiderSubnet auto-created IPPool will add 2 more IPs based on your application replicas.
If you choose to use flexible mode, the auto-created IPPool IPs will expand or shrink dynamically by your application replicas.

3. The current version only supports to use one SpiderSubnet V4/V6 CR, you shouldn't specify 2 or more SpiderSubnet V4 CRs and the spiderpool-controller
will choose the first one to use.

4. It only supports single network interfaces to Pods in Kubernetes.

## Get Started

### Enable SpiderSubnet feature

Firstly, please ensure you have installed the spiderpool and configure the CNI file, refer [install](./install.md) for details.

Check configmap `spiderpool-conf` property `enableSpiderSubnet` whether is already set to `true` or not.

```shell
kubectl -n kube-system get configmap spiderpool-conf -o yaml
```

If you want to set it `true`, just execute `helm upgrade spiderpool spiderpool/spiderpool --set feature.enableSpiderSubnet=true -n kube-system`.

### Create a SpiderSubnet

install a SpiderSubnet example

```shell
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/spider-subnet/subnet-demo.yaml
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/spider-subnet/deploy-use-subnet.yaml
```

### Check the related CR data

Here's the SpiderSubnet, SpiderIPPool, Pod information

```text
NAME                VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT
subnet-demo-v4      4         172.16.0.0/16             3                    200
subnet-demo-v6      6         fc00:f853:ccd:e790::/64   3                    200
------------------------------------------------------------------------------------------
$ kubectl get sp
NAME                                                       VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DISABLE
auto-deployment-default-demo-deploy-subnet-v4-1667358680   4         172.16.0.0/16             1                    3                false
auto-deployment-default-demo-deploy-subnet-v6-1667358681   6         fc00:f853:ccd:e790::/64   1                    3                false
------------------------------------------------------------------------------------------
$ kubectl get sp auto-deployment-default-demo-deploy-subnet-v4-1667358680 -o yaml
apiVersion: spiderpool.spidernet.io/v1
kind: SpiderIPPool
metadata:
  creationTimestamp: "2022-11-02T03:11:20Z"
  finalizers:
  - spiderpool.spidernet.io
  generation: 2
  labels:
    ipam.spidernet.io/ippool-reclaim: "true"
    ipam.spidernet.io/ippool-version: IPv4
    ipam.spidernet.io/owner-application: deployment-default-demo-deploy-subnet
    ipam.spidernet.io/owner-application-uid: 94ceb817-8250-460e-b1af-69d041b98b41
    ipam.spidernet.io/owner-spider-subnet: subnet-demo-v4
  name: auto-deployment-default-demo-deploy-subnet-v4-1667358680
  ownerReferences:
  - apiVersion: spiderpool.spidernet.io/v1
    blockOwnerDeletion: true
    controller: true
    kind: SpiderSubnet
    name: subnet-demo-v4
    uid: 9ec4d464-2dd7-49d3-b394-d3e2eea5e907
  resourceVersion: "117905"
  uid: bd1c883d-e560-4215-aa60-58ea91d58922
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
      containerID: 3b944b1eb35e27030ef78139b3dcd4aa1c671cc2b9ecc56b7751ac51a78377c2
      interface: eth0
      namespace: default
      node: spider-worker
      ownerControllerName: demo-deploy-subnet-774d69487d
      ownerControllerType: ReplicaSet
      pod: demo-deploy-subnet-774d69487d-z6f57
  autoDesiredIPCount: 3
  totalIPCount: 3
------------------------------------------------------------------------------------------
$ kubectl get po -o wide 
NAME                                  READY   STATUS    RESTARTS   AGE   IP             NODE            NOMINATED NODE   READINESS GATES
demo-deploy-subnet-774d69487d-z6f57   1/1     Running   0          71s   172.16.41.2    spider-worker   <none>           <none>
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
NAME                                                       VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DISABLE
auto-deployment-default-demo-deploy-subnet-v4-1667358680   4         172.16.0.0/16             2                    4                false
auto-deployment-default-demo-deploy-subnet-v6-1667358681   6         fc00:f853:ccd:e790::/64   2                    4                false
------------------------------------------------------------------------------------------
$ kubectl get sp auto-deployment-default-demo-deploy-subnet-v4-1667358680 -o yaml
apiVersion: spiderpool.spidernet.io/v1
kind: SpiderIPPool
metadata:
  creationTimestamp: "2022-11-02T03:11:20Z"
  finalizers:
  - spiderpool.spidernet.io
  generation: 3
  labels:
    ipam.spidernet.io/ippool-reclaim: "true"
    ipam.spidernet.io/ippool-version: IPv4
    ipam.spidernet.io/owner-application: deployment-default-demo-deploy-subnet
    ipam.spidernet.io/owner-application-uid: 94ceb817-8250-460e-b1af-69d041b98b41
    ipam.spidernet.io/owner-spider-subnet: subnet-demo-v4
  name: auto-deployment-default-demo-deploy-subnet-v4-1667358680
  ownerReferences:
  - apiVersion: spiderpool.spidernet.io/v1
    blockOwnerDeletion: true
    controller: true
    kind: SpiderSubnet
    name: subnet-demo-v4
    uid: 9ec4d464-2dd7-49d3-b394-d3e2eea5e907
  resourceVersion: "118455"
  uid: bd1c883d-e560-4215-aa60-58ea91d58922
spec:
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
  allocatedIPs:
    172.16.41.2:
      containerID: 3b944b1eb35e27030ef78139b3dcd4aa1c671cc2b9ecc56b7751ac51a78377c2
      interface: eth0
      namespace: default
      node: spider-worker
      ownerControllerName: demo-deploy-subnet-774d69487d
      ownerControllerType: ReplicaSet
      pod: demo-deploy-subnet-774d69487d-z6f57
    172.16.41.4:
      containerID: 70b4ec8d730c72c3f892677112d4eb7bbcf863a641e196682e94e50b8975aa24
      interface: eth0
      namespace: default
      node: spider-control-plane
      ownerControllerName: demo-deploy-subnet-774d69487d
      ownerControllerType: ReplicaSet
      pod: demo-deploy-subnet-774d69487d-zqjr4
  autoDesiredIPCount: 4
  totalIPCount: 4
------------------------------------------------------------------------------------------
$ kubectl get po -o wide
NAME                                  READY   STATUS    RESTARTS   AGE     IP             NODE                   NOMINATED NODE   READINESS GATES
demo-deploy-subnet-774d69487d-z6f57   1/1     Running   0          9m18s   172.16.41.2    spider-worker          <none>           <none>
demo-deploy-subnet-774d69487d-zqjr4   1/1     Running   0          4m30s   172.16.41.4    spider-control-plane   <none>           <none>
```

As you can see, the SpiderSubnet object `subnet-demo-v4` allocates another IP to SpiderIPPool `auto-deployment-default-demo-deploy-subnet-v4-1667358680`
and SpiderSubnet object `subnet-demo-v4` allocates another IP to SpiderIPPool `auto-deployment-default-demo-deploy-subnet-v6-1667358681`.
