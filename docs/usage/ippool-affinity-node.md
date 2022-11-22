# IPPool node affinity

*Spiderpool supports affinity between IP pools and Nodes. It means only Pods running on this Node can use the IP pools that have an affinity to this Node.*

>*Node affinity should be regarded as a **filtering mechanism** rather than a [pool selection rule](TODO).*

## Set up Spiderpool

If you have not deployed Spiderpool yet, follow the guide [installation](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/install.md) for instructions on how to deploy and easily configure Spiderpool.

## Get started

Since the cluster in this example has only two Nodes (1 master and 1 worker), it is required to remove the relevant taints on the master Node through `kubectl taint`, so that ordinary Pods can also be scheduled to it. If your cluster has two or more worker Nodes, please ignore the step above.

Create two IPPools with 1 IP address each, one of which will provide IP addresses for all Pods running on the master Node.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-affinity-node/master-ipv4-ippool.yaml
```

```yaml
apiVersion: spiderpool.spidernet.io/v1
kind: SpiderIPPool
metadata:
  name: master-ipv4-ippool
spec:
  ipVersion: 4
  subnet: 172.18.41.0/24
  ips:
  - 172.18.41.40
  nodeAffinity:
    matchExpressions:
    - {key: node-role.kubernetes.io/master, operator: Exists}
```

The other provides IP addresses for the Pods on the worker Node.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-affinity-node/worker-ipv4-ippool.yaml
```

```yaml
apiVersion: spiderpool.spidernet.io/v1
kind: SpiderIPPool
metadata:
  name: worker-ipv4-ippool
spec:
  ipVersion: 4
  subnet: 172.18.42.0/24
  ips:
  - 172.18.42.40
  nodeAffinity:
    matchExpressions:
    - {key: node-role.kubernetes.io/master, operator: DoesNotExist}
```

Here, the value of the Node annotation `node-role.kubernetes.io/master` distinguishes two Nodes with different roles (or different node regions). If there is no annotation `node-role.kubernetes.io/master` on your Nodes, you can change it to another one or add some annotations you want.

Then, create a Deployment with 2 replicas, and set `podAntiAffinity` to ensure that the two Pods which select the above IPPools according to the syntax of [alternative IP pools](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/ippool-multi.md) can be scheduled to different Nodes.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-affinity-node/node-affinity-deploy.yaml
```

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: node-affinity-deploy
spec:
  replicas: 2
  selector:
    matchLabels:
      app: node-affinity-deploy
  template:
    metadata:
      annotations:   
        ipam.spidernet.io/ippool: |-
          {
            "ipv4": ["master-ipv4-ippool", "worker-ipv4-ippool"]
          }
      labels:
        app: node-affinity-deploy
    spec:
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchLabels:
                app: node-affinity-deploy
            topologyKey: kubernetes.io/hostname
      containers:
      - name: node-affinity-deploy
        image: busybox
        imagePullPolicy: IfNotPresent
        command: ["/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"]
```

Finally, you will find that Pods on different Nodes will use different IPPools.

```bash
kubectl get se
NAME                                    INTERFACE   IPV4POOL             IPV4              IPV6POOL   IPV6   NODE                   CREATETION TIME
node-affinity-deploy-66c9874465-rvdkm   eth0        master-ipv4-ippool   172.18.41.40/24                     spider-control-plane   35s
node-affinity-deploy-66c9874465-wb8ds   eth0        worker-ipv4-ippool   172.18.42.40/24                     spider-worker          35s
```

## Clean up

Clean the relevant resources so that you can run this tutorial again.

```bash
kubectl delete \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-affinity-node/master-ipv4-ippool.yaml \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-affinity-node/worker-ipv4-ippool.yaml \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-affinity-node/node-affinity-deploy.yaml \
--ignore-not-found=true
```
