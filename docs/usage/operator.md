# IPAM for operator

**English** | [**简体中文**](./operator-zh_CN.md)

## Description

Operator is popularly used to implement customized controller. Spiderpool supports to assign IP to Pods created not by kubernetes-native controller. There are two ways to do this:

1. Manual ippool

    The administrator could create ippool object and assign IP to Pods.

2. Automatical ippool

    Spiderpool support to automatically manage ippool for application, it could create, delete, scale up and down a dedicated spiderippool object with static IP address just for one application.

    This feature uses informer technology to watch application, parses its replicas number and manage spiderippool object, it works well with kubernetes-native controller like Deployment, ReplicaSet, StatefulSet, Job, CronJob, DaemonSet.

    This feature also support none kubernetes-native controller, but Spiderpool could not parse the object yaml of none kubernetes-native controller, has some limitations:

    * does not support automatically scale up and down the IP

    * does not support automatically delete the ippool

    In the future, spiderpool may support all operation of automatical ippool.

Another issue about none kubernetes-native controller is stateful or stateless. Because Spiderpool has no idea whether application created by none kubernetes-native controller is stateful or not.
So Spiderpool treats them as `stateless` Pod like `Deployment`, this means Pods created by none kubernetes-native controller is able to fix the IP range like `Deployment`, but not able to bind each Pod to a specific IP address like `Statefulset`.

## Get Started

It will use [OpenKruise](https://openkruise.io/zh/docs/) to demonstrate how Spiderpool supports operator.

### Set up Spiderpool

See [installation](./install/get-started-kind.md) for more details.

### Set up OpenKruise

Please refer to [OpenKruise](https://openkruise.io/docs/installation/)

### Create Pod by `Manual ippool` way

1. Create a custom IPPool.

    ```bash
    kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/basic/custom-ipv4-ippool.yaml
    ```

    ```yaml
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderIPPool
    metadata:
      name: custom-ipv4-ippool
    spec:
      subnet: 172.18.41.0/24
      ips:
        - 172.18.41.40-172.18.41.50
    ```

2. Create an OpenKruise CloneSet that has 3 replicas, and sepecify the ippool by annotations `ipam.spidernet.io/ippool`

    ```yaml
    apiVersion: apps.kruise.io/v1alpha1
    kind: CloneSet
    metadata:
      name: custom-kruise-cloneset
    spec:
      replicas: 3
      selector:
        matchLabels:
          app: custom-kruise-cloneset
      template:
        metadata:
          annotations:
            ipam.spidernet.io/ippool: |-
              {
                "ipv4": ["custom-ipv4-ippool"]
              }
          labels:
            app: custom-kruise-cloneset
        spec:
          containers:
          - name: custom-kruise-cloneset
            image: busybox
            imagePullPolicy: IfNotPresent
            command: ["/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"]
    ```

    As expected, Pods of OpenKruise CloneSet `custom-kruise-cloneset` will be assigned with IP addresses from IPPool `custom-ipv4-ippool`.

    ```bash
    kubectl get po -l app=custom-kruise-cloneset -o wide
    NAME                           READY   STATUS    RESTARTS   AGE   IP             NODE            NOMINATED NODE   READINESS GATES
    custom-kruise-cloneset-8m9ls   1/1     Running   0          96s   172.18.41.44   spider-worker   <none>           2/2
    custom-kruise-cloneset-c4z9f   1/1     Running   0          96s   172.18.41.50   spider-worker   <none>           2/2
    custom-kruise-cloneset-w9kfm   1/1     Running   0          96s   172.18.41.46   spider-worker   <none>           2/2
    ```

## Create Pod by `Automatical ippool` way

1. Create an OpenKruise CloneSet that has 3 replicas, and specify the subnet by annotations `ipam.spidernet.io/subnet`

    ```yaml
    apiVersion: apps.kruise.io/v1alpha1
    kind: CloneSet
    metadata:
      name: custom-kruise-cloneset
    spec:
      replicas: 3
      selector:
        matchLabels:
          app: custom-kruise-cloneset
      template:
        metadata:
          annotations:
            ipam.spidernet.io/subnet: |- 
              {"ipv4": ["subnet-demo-v4"], "ipv6": ["subnet-demo-v6"]}
            ipam.spidernet.io/ippool-ip-number: "5"
          labels:
            app: custom-kruise-cloneset
        spec:
          containers:
          - name: custom-kruise-cloneset
            image: busybox
            imagePullPolicy: IfNotPresent
            command: ["/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"]
    ```

    > NOTICE:
    >
    > 1. You must specify a fixed IP number for auto-created IPPool like `ipam.spidernet.io/ippool-ip-number: "5"`.
      Because Spiderpool has no idea about the replica number, so it does not support annotation like `ipam.spidernet.io/ippool-ip-number: "+5"`.

2. Check status

    As expected, Spiderpool will create auto-created IPPool from `subnet-demo-v4` and `subnet-demo-v6` objects.
    And Pods of OpenKruise CloneSet `custom-kruise-cloneset` will be assigned with IP addresses from the created IPPools.

    ```text
    $ kubectl get sp | grep kruise
    NAME                                      VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE   APP-NAMESPACE
    auto4-custom-kruise-cloneset-eth0-028d6   4         172.16.0.0/16             3                    5                false     false     default
    auto6-custom-kruise-cloneset-eth0-028d6   6         fc00:f853:ccd:e790::/64   3                    5                false     false     default
    ------------------------------------------------------------------------------------------
    $ kubectl get po -l app=custom-kruise-cloneset -o wide
    NAME                           READY   STATUS    RESTARTS   AGE   IP            NODE            NOMINATED NODE   READINESS GATES
    custom-kruise-cloneset-f52dn   1/1     Running   0          61s   172.16.41.4   spider-worker   <none>           2/2
    custom-kruise-cloneset-mq67v   1/1     Running   0          61s   172.16.41.5   spider-worker   <none>           2/2
    custom-kruise-cloneset-nprpf   1/1     Running   0          61s   172.16.41.1   spider-worker   <none>           2/2
    ```
