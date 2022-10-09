# SpiderIPPool node affinity

*Spiderpool supports for configuring the affinity between IP pools and Nodes. It means only Pods running on a specific Node can use the IP pools that have configured the affinity to the Node. Node affinity should be regarded as a **filtering mechanism** rather than a [pool selection rule](TODO).*

## Set up Spiderpool

If you have not set up Spiderpool yet, follow [Quick Installation](./install.md) for instructions on how to install and simply configure Spiderpool.

## Get started

Since the cluster in this example has only two Nodes (1 master and 1 worker), it is required to remove the relevant taints on the master Node through `kubectl taint`, so that ordinary Pods can also be scheduled to it. If your cluster has two or more worker Nodes, please ignore the step above.

Then, create two SpiderIPPools with three IP addresses, one of which will provide IP addresses for all Pods running on the master Node in the next.

```bash
kubectl apply -f https://github.com/spidernet-io/spiderpool/tree/main/docs/example/ippool-affinity-node/master-ipv4-ippool.yaml
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
  - 172.18.41.40-172.18.41.42
  nodeAffinity:
    matchExpressions:
    - {key: node-role.kubernetes.io/master, operator: Exists}
```

The other provides IP addresses for the Pods on the worker Node.

```bash
kubectl apply -f https://github.com/spidernet-io/spiderpool/tree/main/docs/example/ippool-affinity-node/worker-ipv4-ippool.yaml
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
  - 172.18.42.40-172.18.42.42
  nodeAffinity:
    matchExpressions:
    - {key: node-role.kubernetes.io/master, operator: DoesNotExist}
```

Here, we use `node-role.kubernetes.io/master` Node annotation to distinguish two different Nodes (actually, they can be different node regions). If there is no `node-role.kubernetes.io/master` annotation on your Nodes, you can change it to another one or add some annotations you want.

Now, run a Deployment with 3 replicas (let all Pods be scheduled to the same Node) and let their Pods select the above two SpiderIPPools in the form of [alternative IP pools](./ippool-multi.md).

```bash
kubectl apply -f https://github.com/spidernet-io/spiderpool/tree/main/docs/example/ippool-affinity-node/node-affinity-deploy.yaml
```

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: node-affinity-deploy
spec:
  replicas: 3
  selector:
    matchLabels:
      app: node-affinity-deploy
  template:
    metadata:
      annotations:   
        ipam.spidernet.io/ippool: |-
          {
            "interface": "eth0",
            "ipv4pools": ["master-ipv4-ippool", "worker-ipv4-ippool"]
          }
      labels:
        app: node-affinity-deploy
    spec:
      containers:
      - name: node-affinity-deploy
        image: busybox
        imagePullPolicy: IfNotPresent
        command: ["/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"]
```

Finally, you will find that Pods on different Nodes will use different SpiderIPPools.

```bash
kubectl get se
NAME                                    INTERFACE   IPV4POOL             IPV4               IPV6POOL   IPV6   NODE                   CREATETION TIME
node-affinity-deploy-85f8b6997b-dlnrt   eth0        worker-ipv4-ippool   172.18.42.41/24                      spider-worker          35s
node-affinity-deploy-85f8b6997b-j6k6f   eth0        worker-ipv4-ippool   172.18.42.42/24                      spider-worker          35s
node-affinity-deploy-85f8b6997b-pk4jb   eth0        master-ipv4-ippool   172.18.41.41/24                      spider-control-plane   35s
```

## Clean up

Clean the relevant resources so that you can run this tutorial again.

```bash
kubectl delete -f https://github.com/spidernet-io/spiderpool/tree/main/docs/example/ippool-affinity-node --ignore-not-found=true
```
