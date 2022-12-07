# Spiderpool supports third-party controllers

## Description

This page will use [OpenKruise](https://openkruise.io/zh/docs/) to demonstrate how Spiderpool supports third-party controllers. *As for how to install OpenKruise with [Helm](https://helm.sh/), please refer to [OpenKruise](https://openkruise.io/zh/docs/)* 

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
