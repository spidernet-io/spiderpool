# SpiderSubnet

## Description

The spiderpool owns a CRD SpiderSubnet, it can help applications (such as: Deployment, ReplicaSet, StatefulSet, Job, CronJob, DaemonSet.) to create a corresponding SpiderIPPool.

Here are some annotations that you should write down on the application template pod annotation:

| annotation                                      | description                                                                                                          |
|-------------------------------------------------|----------------------------------------------------------------------------------------------------------------------|
| spiderpool.spidernet.io/spider-subnet-v4        | specify which SpiderSubnet V4 CR to use                                                                              |
| spiderpool.spidernet.io/spider-subnet-v6        | specify which SpiderSubnet V6 CR to use                                                                              |
| spiderpool.spidernet.io/assign-ip-number        | the fixed IP numbers of the corresponding SpiderIPPool (this is the SpiderIPPool IP numbers)                         |
| spiderpool.spidernet.io/flexible-ip-number      | the extra IP numbers to allocate (the SpiderIPPool IP number = "flexible-ip-number" + "application replicas number") |
| spiderpool.spidernet.io/reclaim-ippool          | specify the corresponding SpiderIPPool to delete or not once the application was deleted (default true)              |

### Notice

1. The annotation `spiderpool.spidernet.io/flexible-ip-number` has higher priority over `spiderpool.spidernet.io/assign-ip-number`.
If you specify both of them two, it only uses `spiderpool.spidernet.io/flexible-ip-number` mode.

2. It only supports single network interfaces to Pods in Kubernetes.

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

1. Here's the SpiderSubnet, SpiderIPPool, Pod information

    ```
    $ kubectl get ss
    NAME                VERSION   SUBNET          FREE-IP-COUNT   TOTAL-IP-COUNT
    subnet-demo-v4      4         172.16.0.0/16   100             100
    ------------------------------------------------------------------------------------------
    $ kubectl get sp
    NAME                                                       VERSION   SUBNET          ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DISABLE
    auto-deployment-default-demo-deploy-subnet-v4-1664524013   4         172.16.0.0/16   1                    2                false
    ------------------------------------------------------------------------------------------
    $ kubectl get sp auto-deployment-default-demo-deploy-subnet-v4-1664524013 -o yaml 
    apiVersion: spiderpool.spidernet.io/v1
    kind: SpiderIPPool
    metadata:
      creationTimestamp: "2022-09-30T07:46:53Z"
      finalizers:
      - spiderpool.spidernet.io
      generation: 1
      labels:
        ippool-version: v4
        owner-application: deployment-default-demo-deploy-subnet
        owner-application-uid: c452047f-3e28-46f4-90e7-18e543b8521d
        owner-spider-subnet: subnet-demo-v4
      name: auto-deployment-default-demo-deploy-subnet-v4-1664524013
      resourceVersion: "200002"
      uid: 91695195-8547-418f-81c4-f8c3f0521b7b
    spec:
      disable: false
      ipVersion: 4
      ips:
      - 172.16.41.1-172.16.41.2
      podAffinity:
        matchLabels:
          app: demo-deploy-subnet
      subnet: 172.16.0.0/16
      vlan: 0
    status:
      allocatedIPCount: 1
      allocatedIPs:
        172.16.41.2:
          containerID: 477752c45776e3d3177fdb1c0c869c9ee9f64afeb3131a355ddcd95533b63996
          interface: eth0
          namespace: default
          node: spider-worker
          ownerControllerName: demo-deploy-subnet-5594f9966b
          ownerControllerType: ReplicaSet
          pod: demo-deploy-subnet-5594f9966b-nc6hc
      totalIPCount: 2
    ------------------------------------------------------------------------------------------
    $ kubectl get po -o wide 
    NAME                                  READY   STATUS    RESTARTS   AGE   IP            NODE            NOMINATED NODE   READINESS GATES
    demo-deploy-subnet-5594f9966b-nc6hc   1/1     Running   0          77s   172.16.41.2   spider-worker   <none>           <none>
    ```

2. Try to scale the deployment replicas and check the SpiderIPPool.

    ```
    $ kubectl patch deploy demo-deploy-subnet --patch '{"spec": {"replicas": 2}}'
    deployment.apps/demo-deploy-subnet patched
    ------------------------------------------------------------------------------------------
    $ kubectl get ss subnet-demo-v4 -o yaml                
    apiVersion: spiderpool.spidernet.io/v1
    kind: SpiderSubnet
    metadata:
      annotations:
        kubectl.kubernetes.io/last-applied-configuration: |
          {"apiVersion":"spiderpool.spidernet.io/v1","kind":"SpiderSubnet","metadata":{"annotations":{},"name":"subnet-demo-v4"},"spec":{"ipVersion":4,"ips":["172.16.41.1-172.16.41.100"],"subnet":"172.16.0.0/16"}}
      creationTimestamp: "2022-09-30T07:46:26Z"
      generation: 1
      name: subnet-demo-v4
      resourceVersion: "201295"
      uid: 4890e576-d057-4065-b107-1dba294c491f
    spec:
      ipVersion: 4
      ips:
      - 172.16.41.1-172.16.41.100
      subnet: 172.16.0.0/16
      vlan: 0
    status:
      freeIPCount: 97
      freeIPs:
      - 172.16.41.4-172.16.41.100
      totalIPCount: 100
    ------------------------------------------------------------------------------------------
    $ kubectl get sp auto-deployment-default-demo-deploy-subnet-v4-1664524013 -o yaml                                                        
    apiVersion: spiderpool.spidernet.io/v1
    kind: SpiderIPPool
    metadata:
      creationTimestamp: "2022-09-30T07:46:53Z"
      finalizers:
      - spiderpool.spidernet.io
      generation: 2
      labels:
        ippool-version: v4
        owner-application: deployment-default-demo-deploy-subnet
        owner-application-uid: c452047f-3e28-46f4-90e7-18e543b8521d
        owner-spider-subnet: subnet-demo-v4
      name: auto-deployment-default-demo-deploy-subnet-v4-1664524013
      resourceVersion: "201311"
      uid: 91695195-8547-418f-81c4-f8c3f0521b7b
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
      allocatedIPCount: 2
      allocatedIPs:
        172.16.41.1:
          containerID: b3228598020e38c4774d62f85bb35ab857434904903027dbeb0d1b8edaea4d63
          interface: eth0
          namespace: default
          node: spider-control-plane
          ownerControllerName: demo-deploy-subnet-5594f9966b
          ownerControllerType: ReplicaSet
          pod: demo-deploy-subnet-5594f9966b-nwqmj
        172.16.41.2:
          containerID: 477752c45776e3d3177fdb1c0c869c9ee9f64afeb3131a355ddcd95533b63996
          interface: eth0
          namespace: default
          node: spider-worker
          ownerControllerName: demo-deploy-subnet-5594f9966b
          ownerControllerType: ReplicaSet
          pod: demo-deploy-subnet-5594f9966b-nc6hc
      totalIPCount: 3
    ------------------------------------------------------------------------------------------
    $ kubectl get po -o wide                                                         
    NAME                                  READY   STATUS    RESTARTS   AGE     IP            NODE                   NOMINATED NODE   READINESS GATES
    demo-deploy-subnet-5594f9966b-nc6hc   1/1     Running   0          14m     172.16.41.2   spider-worker          <none>           <none>
    demo-deploy-subnet-5594f9966b-nwqmj   1/1     Running   0          2m16s   172.16.41.1   spider-control-plane   <none>           <none>
    ```

And you can see, the SpiderSubnet object `subnet-demo-v4` allocate another IP to SpiderIPPool `auto-deployment-default-demo-deploy-subnet-v4-1664524013`.
