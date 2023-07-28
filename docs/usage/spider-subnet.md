# SpiderSubnet

The Spiderpool owns a CRD SpiderSubnet, which can help applications (such as Deployment, ReplicaSet, StatefulSet, Job, CronJob, DaemonSet) to create a corresponding SpiderIPPool.

Here are some annotations that you should write down on the application template Pod annotation:

| Annotation                         | Description                                                                                                            | Example                                                                     |
|------------------------------------|------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------|
| ipam.spidernet.io/subnet           | Choose one SpiderSubnet V4 and V6 CR to use                                                                            | {"interface":"eth0", "ipv4":["subnet-demo-v4"], "ipv6":["subnet-demo-v6"]} |
| ipam.spidernet.io/subnets          | Choose multiple SpiderSubnet V4 and V6 CR to use (the current version only supports to use the first one)              | [{"interface":"eth0", "ipv4":["v4-subnet1"], "ipv6":["v6-subnet1"]}]        |
| ipam.spidernet.io/ippool-ip-number | The IP numbers of the corresponding SpiderIPPool (fixed and flexible mode, optional and default '+1')                  | +2                                                                          |
| ipam.spidernet.io/ippool-reclaim   | Specify the corresponding SpiderIPPool to delete or not once the application was deleted (optional and default 'true') | true                                                                        |

## Notice

1. The annotation `ipam.spidernet.io/subnets` has higher priority over `ipam.spidernet.io/subnet`. If you specify both of them two, it only uses `ipam.spidernet.io/subnets` mode.

2. In annotation `ipam.spidernet.io/subnet` mode, it will use default interface name `eth0` if you do not set `interface` property.

3. For annotation `ipam.spidernet.io/ippool-ip-number`, you can use '2' for fixed IP number or '+2' for flexible mode. The value '+2' means the SpiderSubnet auto-created IPPool will add 2 more IPs based on your application replicas. If you choose to use flexible mode, the auto-created IPPool IPs will expand or shrink dynamically by your application replicas. This is an optional annotation. If left unset, it will use the `clusterSubnetDefaultFlexibleIPNumber` property from the `spiderpool-conf` ConfigMap as the flexible IP number in flexible mode. Refer to [config](../reference/configmap.md) for details.

4. The current version only supports using one SpiderSubnet V4/V6 CR for one Interface. You shouldn't specify two or more SpiderSubnet V4 CRs. The system will choose the first one to use.

5. It's invalid to modify the Auto-created IPPool `Spec.IPs` by users.

6. The auto-created IPPool will only serve your specified application, and the system will bind a special Spiderpool podAffinity to it. If you want to use `ipam.spidernet.io/ippool` or `ipam.spidernet.io/ippools` annotations to specify the 'reserved' auto-created IPPool, you should edit the IPPool to remove its special Spiderpool podAffinity and annotations `ipam.spidernet.io/owner-application*`.

## Get Started

### Enable SpiderSubnet feature

Firstly, please ensure you have installed the Spiderpool and configure the CNI file, refer to [install](./install.md) for details.

Check configmap `spiderpool-conf` property `enableSpiderSubnet` whether is already set to `true` or not.

```shell
kubectl -n kube-system get configmap spiderpool-conf -o yaml
```

If you want to set it `true`, just execute `helm upgrade spiderpool spiderpool/spiderpool --set ipam.enableSpiderSubnet=true -n kube-system`.

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

$ kubectl get sp -o wide
NAME                                  VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE   APP-NAMESPACE
auto4-demo-deploy-subnet-eth0-337bc   4         172.16.0.0/16             1                    3                false     false     default
auto6-demo-deploy-subnet-eth0-337bc   6         fc00:f853:ccd:e790::/64   1                    3                false     false     default
------------------------------------------------------------------------------------------
$ kubectl get sp auto4-demo-deploy-subnet-eth0-337bc -o yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  annotations:
    ipam.spidernet.io/ippool-ip-number: "+2"
  creationTimestamp: "2023-05-09T09:25:24Z"
  finalizers:
  - spiderpool.spidernet.io
  generation: 1
  labels:
    ipam.spidernet.io/interface: eth0
    ipam.spidernet.io/ip-version: IPv4
    ipam.spidernet.io/ippool-cidr: 172-16-0-0-16
    ipam.spidernet.io/ippool-reclaim: "true"
    ipam.spidernet.io/owner-application-gv: apps_v1
    ipam.spidernet.io/owner-application-kind: Deployment
    ipam.spidernet.io/owner-application-name: demo-deploy-subnet
    ipam.spidernet.io/owner-application-namespace: default
    ipam.spidernet.io/owner-application-uid: bea129aa-acf5-40cb-8669-337bc1e95a41
    ipam.spidernet.io/owner-spider-subnet: subnet-demo-v4
  name: auto4-demo-deploy-subnet-eth0-337bc
  ownerReferences:
  - apiVersion: spiderpool.spidernet.io/v2beta1
    blockOwnerDeletion: true
    controller: true
    kind: SpiderSubnet
    name: subnet-demo-v4
    uid: 3f5882a0-12e6-40f5-a103-431de700195f
  resourceVersion: "1381"
  uid: 4ebc5725-5d3d-448a-889f-ce9e3943b2aa
spec:
  default: false
  disable: false
  ipVersion: 4
  ips:
  - 172.16.41.1-172.16.41.3
  podAffinity:
    matchLabels:
      ipam.spidernet.io/app-api-group: apps
      ipam.spidernet.io/app-api-version: v1
      ipam.spidernet.io/app-kind: Deployment
      ipam.spidernet.io/app-name: demo-deploy-subnet
      ipam.spidernet.io/app-namespace: default
  subnet: 172.16.0.0/16
  vlan: 0
status:
  allocatedIPCount: 1
  allocatedIPs: '{"172.16.41.1":{"interface":"eth0","pod":"default/demo-deploy-subnet-778786474b-kls7s","podUid":"cc310a87-8c5a-4bff-9a84-0c4975bd5921"}}'
  totalIPCount: 3

$ kubectl get po -o wide 
NAME                                  READY   STATUS    RESTARTS   AGE     IP             NODE            NOMINATED NODE   READINESS GATES
demo-deploy-subnet-778786474b-kls7s   1/1     Running   0          3m10s   172.16.41.1    spider-worker   <none>           <none>
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
$ kubectl get sp -o wide
NAME                                  VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE   APP-NAMESPACE
auto4-demo-deploy-subnet-eth0-337bc   4         172.16.0.0/16             2                    4                false     false     default
auto6-demo-deploy-subnet-eth0-337bc   6         fc00:f853:ccd:e790::/64   2                    4                false     false     default
------------------------------------------------------------------------------------------
$ kubectl get sp auto4-demo-deploy-subnet-eth0-337bc -o yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  annotations:
    ipam.spidernet.io/ippool-ip-number: "+2"
  creationTimestamp: "2023-05-09T09:25:24Z"
  finalizers:
  - spiderpool.spidernet.io
  generation: 2
  labels:
    ipam.spidernet.io/interface: eth0
    ipam.spidernet.io/ip-version: IPv4
    ipam.spidernet.io/ippool-cidr: 172-16-0-0-16
    ipam.spidernet.io/ippool-reclaim: "true"
    ipam.spidernet.io/owner-application-gv: apps_v1
    ipam.spidernet.io/owner-application-kind: Deployment
    ipam.spidernet.io/owner-application-name: demo-deploy-subnet
    ipam.spidernet.io/owner-application-namespace: default
    ipam.spidernet.io/owner-application-uid: bea129aa-acf5-40cb-8669-337bc1e95a41
    ipam.spidernet.io/owner-spider-subnet: subnet-demo-v4
  name: auto4-demo-deploy-subnet-eth0-337bc
  ownerReferences:
  - apiVersion: spiderpool.spidernet.io/v2beta1
    blockOwnerDeletion: true
    controller: true
    kind: SpiderSubnet
    name: subnet-demo-v4
    uid: 3f5882a0-12e6-40f5-a103-431de700195f
  resourceVersion: "2004"
  uid: 4ebc5725-5d3d-448a-889f-ce9e3943b2aa
spec:
  default: false
  disable: false
  ipVersion: 4
  ips:
  - 172.16.41.1-172.16.41.4
  podAffinity:
    matchLabels:
      ipam.spidernet.io/app-api-group: apps
      ipam.spidernet.io/app-api-version: v1
      ipam.spidernet.io/app-kind: Deployment
      ipam.spidernet.io/app-name: demo-deploy-subnet
      ipam.spidernet.io/app-namespace: default
  subnet: 172.16.0.0/16
  vlan: 0
status:
  allocatedIPCount: 2
  allocatedIPs: '{"172.16.41.1":{"interface":"eth0","pod":"default/demo-deploy-subnet-778786474b-kls7s","podUid":"cc310a87-8c5a-4bff-9a84-0c4975bd5921"},"172.16.41.2":{"interface":"eth0","pod":"default/demo-deploy-subnet-778786474b-2kf24","podUid":"0a239f85-be47-4b98-bf29-c8bf02b6b59e"}}'
  totalIPCount: 4
------------------------------------------------------------------------------------------
$ kubectl get po -o wide
NAME                                  READY   STATUS    RESTARTS   AGE     IP             NODE                   NOMINATED NODE   READINESS GATES
demo-deploy-subnet-778786474b-2kf24   1/1     Running   0          63s     172.16.41.2    spider-control-plane   <none>           <none>
demo-deploy-subnet-778786474b-kls7s   1/1     Running   0          4m39s   172.16.41.1    spider-worker          <none>           <none>
```

As you can see, the SpiderSubnet object `subnet-demo-v4` allocates another IP to SpiderIPPool `auto4-demo-deploy-subnet-eth0-337bc`
and SpiderSubnet object `subnet-demo-v6` allocates another IP to SpiderIPPool `auto6-demo-deploy-subnet-eth0-337bc`.

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
