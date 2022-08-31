# SpiderIPPool node affinity

*Spiderpool supports multiple ways to select ippool. Pod will select a specific ippool to allocate IP according to the corresponding rules that with different priorities. Meanwhile, ippool can use selector to filter its user.*

## Setup Spiderpool

If you have not set up Spiderpool yet, follow the guide [Quick Installation](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/install.md) for instructions on how to install and simply configure Spiderpool.

## Get started

After understanding what selectors' "filtering mechanism" is, let's take a look at how selectors work with ippool selection rules. There are two ippools:

```yaml
apiVersion: spiderpool.spidernet.io/v1
kind: SpiderIPPool
metadata:
  name: master-v4-ippool
spec:
  ipVersion: IPv4
  subnet: 172.18.0.0/16
  ips:
  - 172.18.50.41-172.18.50.50
  nodeSelector:
    matchExpressions:
    - {key: node-role.kubernetes.io/master, operator: Exists}
```

Obviously, ippool `master-v4-ippool` only works with the control plane nodes of Kubernetes. The Pod which scheduled to the master nodes can be correctly allocated IP from this ippool.

```yaml
apiVersion: spiderpool.spidernet.io/v1
kind: SpiderIPPool
metadata:
  name: worker-v4-ippool
spec:
  ipVersion: IPv4
  subnet: 172.18.0.0/16
  ips:
  - 172.18.50.51-172.18.50.60
  nodeSelector:
    matchExpressions:
    - {key: node-role.kubernetes.io/master, operator: DoesNotExist}
```

And ippool `worker-v4-ippool` is the opposite.

Then, we run the following Deployment `example`, which has 5 replicas. We expect that the Pods controlled by `example` can be evenly scheduled to master nodes and worker nodes. Of course, maybe you should remove some related Taints.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: example
spec:
  replicas: 5
  selector:
    matchLabels:
      app: example
  template:
    metadata:
      annotations:   
        ipam.spidernet.io/ippool: |-
          {
            "interface": "eth0",
            "ipv4pools": ["master-v4-ippool", "worker-v4-ippool"]
          }
      labels:
        app: example
    spec:
      containers:
      - image: alpine
        imagePullPolicy: IfNotPresent
        name: example
        command: ["/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"]
```

Finally, we will find that Pods at different nodes use different ippools.

```bash
$ kubectl get swe -n default
NAME                       INTERFACE   IPV4POOL           IPV4              IPV6POOL   IPV6   NODE            CREATETION TIME
example-554cc84db6-kr8j5   eth0        master-v4-ippool   172.18.50.47/16                     control-plane   3s
example-554cc84db6-lkdbz   eth0        worker-v4-ippool   172.18.50.51/16                     worker          4s
example-554cc84db6-qbmwv   eth0        worker-v4-ippool   172.18.50.58/16                     worker          3s
example-554cc84db6-r6qpt   eth0        worker-v4-ippool   172.18.50.55/16                     worker          4s
example-554cc84db6-rjstk   eth0        master-v4-ippool   172.18.50.43/16                     control-plane   4s
```

## Clean up

Let's clean the relevant resources so that we can run this tutorial again.

```bash
kubectl delete ns test-ns1
kubectl delete -f https://github.com/spidernet-io/spiderpool/tree/main/docs/example/ippool-affinity-node --ignore-not-found=true
```
