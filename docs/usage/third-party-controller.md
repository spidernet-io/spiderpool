# Spiderpool supports third-party controllers

## Description

This page will use [OpenKruise](https://openkruise.io/zh/docs/) to demonstrate how Spiderpool supports third-party controllers. *As for how to install OpenKruise with [Helm](https://helm.sh/), please refer to [OpenKruise](https://openkruise.io/zh/docs/)* 

### Notice

1. In the current version, we do not support `extended StatefulSet` to keep fixed IP address because we can't get the `extended StatefulSet` object yaml
to fetch its replicas for scaling the IPPool IPs counts.
For kubernetes StatefulSet, we use the StatefulSet object replicas property and stable and serial pod name to choose whether to keep its IP address for next schedule.

2. If the third party controller controllers `StatefulSet`, it surely runs normally and keeps the `StatefulSet` characteristic.

## Set up Spiderpool

If you have not deployed Spiderpool yet, see [installation](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/install.md) for instructions on how to deploy and configure Spiderpool.

## Create an IPPool

Next, let's create a custom IPPool.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/basic/custom-ipv4-ippool.yaml
```

```yaml
apiVersion: spiderpool.spidernet.io/v1
kind: SpiderIPPool
metadata:
  name: custom-ipv4-ippool
spec:
  ipVersion: 4
  subnet: 172.18.41.0/24
  ips:
    - 172.18.41.40-172.18.41.50
```

## Run 

Finally, create an OpenKruise CloneSet that has 3 replicas.

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

## Third-party controller with SpiderSubnet feature

We also support third-party controllers to use SpiderSubnet feature, it is used as the normal kubernetes application controllers. Refer [SpiderSubnet](./spider-subnet.md) for details.

### Notice

1. You must specify a fixed IP number for auto-created IPPool if you want to use SpiderSubnet feature.
Here's an example `ipam.spidernet.io/ippool-ip-number: "5"`

2. We don't support reclaim IPPool for third-party controller currently.
So, setting annotation `ipam.spidernet.io/ippool-reclaim: "true"` does not take effect.
And you need to delete the corresponding auto-created IPPool by yourself once you clean up the third-party controller application.

### Run

We assume you have already enabled SpiderSubnet feature and created cluster default subnet.
The following two yaml will lead to the same effect.

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
          {"ipv4": ["default-v4-subnet"], "ipv6": ["default-v6-subnet"]}
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

As expected, Spiderpool will create auto-created IPPool from `default-v4-subnet` and `default-v6-subnet` objects.
And Pods of OpenKruise CloneSet `custom-kruise-cloneset` will be assigned with IP addresses from the created IPPools.

```text
$ kubectl get sp | grep kruise

NAME                                                                           VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DISABLE
auto-unknown-default-custom-kruise-cloneset-v4-eth0-1f5d4dcf3251               4         172.18.0.0/16             3                    5                false
auto-unknown-default-custom-kruise-cloneset-v6-eth0-1f5d4dcf3251               6         fc00:f853:ccd:e793::/64   3                    5                false
------------------------------------------------------------------------------------------
$ kubectl get po -l app=custom-kruise-cloneset -o wide

NAME                           READY   STATUS    RESTARTS   AGE     IP              NODE            NOMINATED NODE   READINESS GATES
custom-kruise-cloneset-98q8r   1/1     Running   0          3m27s   172.18.40.8     spider-worker   <none>           2/2
custom-kruise-cloneset-gql4j   1/1     Running   0          3m27s   172.18.40.9     spider-worker   <none>           2/2
custom-kruise-cloneset-kzt5q   1/1     Running   0          3m27s   172.18.40.6     spider-worker   <none>           2/2
```
