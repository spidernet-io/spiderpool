# Reserved IP

*Spiderpool reserve some IP addresses for the whole cluster, which will not be used by any IPAM allocation. Usually, these IP addresses are external IP addresses or cannot be used for network communication (e.g. broadcast address).*

## SpiderIPPool excludeIPs

First of all, you may have observed that there is an `excludeIPs` field in SpiderIPPool. To some extent, it is also a mechanism for reserving IP, but its main work is not so. Honestly, `excludeIPs` field is more of a **syntax sugar**, so that users can define their SpiderIPPool more flexibly.

For example, now you want to create an SpiderIPPool, which contains two IP ranges: `172.18.41.40-172.18.41.44` and `172.18.41.46-172.18.41.50`. Without using `excludeIPs`, you should define the `ips` as follows:

```yaml
apiVersion: spiderpool.spidernet.io/v1
kind: SpiderIPPool
metadata:
  name: not-use-excludeips
spec:
  ipVersion: 4
  subnet: 172.18.41.0/24
  ips:
  - 172.18.41.40-172.18.41.44
  - 172.18.41.46-172.18.41.50
```

But in fact, you can more concisely describe this semantics through `excludeIPs`:

```yaml
apiVersion: spiderpool.spidernet.io/v1
kind: SpiderIPPool
metadata:
  name: use-excludeips
spec:
  ipVersion: 4
  subnet: 172.18.41.0/24
  ips:
  - 172.18.41.40-172.18.41.50
  excludeIPs:
  - 172.18.41.45
```

`excludeIPs` also supports the format of IP range and you can define multiple `excludeIPs` records to segment a subnet in more detail.

`excludeIPs` will make sure that any Pod that allocates IP addresses from this SpiderIPPool will not use these excluded IP addresses. However, it should be noted that this mechanism only has an effect on the **SpiderIPPool itself** with `excludeIPs` defined.

## Use SpiderReservedIP

Unlike `excluedIPs` field in SpiderIPPool, SpiderReservedIP is actually used to define the global reserved IP address rules of a cluster. It ensures that no Pod in a cluster will use these IP addresses defined by itself, whether or not some SpiderIPPools have inadvertently defined the same IP addresses for Pods use. More details refer to [definition of SpiderReservedIP](https://github.com/spidernet-io/spiderpool/blob/main/docs/concepts/reservedip.md).

### Setup Spiderpool

If you have not set up Spiderpool yet, follow the guide [Quick Installation](./install.md) for instructions on how to install and simply configure Spiderpool.

### Get Started

To understand how it works, let's do such a test. First, you should create an SpiderReservedIP which reserves 9 IP addresses from `172.18.42.41` to `172.18.42.49`.

```bash
kubectl apply -f https://github.com/spidernet-io/spiderpool/tree/main/docs/example/reserved-ip/test-ipv4-reservedip.yaml
```

```yaml
apiVersion: spiderpool.spidernet.io/v1
kind: SpiderReservedIP
metadata:
  name: test-ipv4-reservedip
spec:
  ipVersion: 4
  ips:
  - 172.18.42.41-172.18.42.49
```

At the same time, create an SpiderIPPool with 10 IP addresses from `172.18.42.41` to `172.18.42.50`. Yes, you deliberately make it hold one more IP address than the SpiderReservedIP above.

```bash
kubectl apply -f https://github.com/spidernet-io/spiderpool/tree/main/docs/example/reserved-ip/test-ipv4-ippool.yaml
```

```yaml
apiVersion: spiderpool.spidernet.io/v1
kind: SpiderIPPool
metadata:
  name: test-ipv4-ippool
spec:
  ipVersion: 4
  subnet: 172.18.42.0/24
  ips:
  - 172.18.42.41-172.18.42.50
```

Then, run a Deployment with 3 replicas and let its Pods allocate IP addresses from the SpiderIPPool above.

```bash
kubectl apply -f https://github.com/spidernet-io/spiderpool/tree/main/docs/example/reserved-ip/reservedip-deploy.yaml
```

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: reservedip-deploy
spec:
  replicas: 3
  selector:
    matchLabels:
      app: reservedip-deploy
  template:
    metadata:
      annotations:
        ipam.spidernet.io/ippool: |-
          {
            "interface": "eth0",
            "ipv4pools": ["test-ipv4-ippool"]
          }
      labels:
        app: reservedip-deploy
    spec:
      containers:
      - name: reservedip-deploy
        image: busybox
        imagePullPolicy: IfNotPresent
        command: ["/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"]
```

After a while, only one of these Pods using IP `172.18.42.50` can run successfully because "all IP used out".

```bash
kubectl get po -l app=reservedip-deploy -o wide
NAME                                 READY   STATUS              RESTARTS   AGE   IP             NODE            
reservedip-deploy-6cf9858886-cm7bp   0/1     ContainerCreating   0          35s   <none>         spider-worker
reservedip-deploy-6cf9858886-lb7cr   0/1     ContainerCreating   0          35s   <none>         spider-worker
reservedip-deploy-6cf9858886-pkcfl   1/1     Running             0          35s   172.18.42.50   spider-worker
```

But when we remove this SpiderReservedIP, everything will return to normal.

```bash
kubectl delete -f https://github.com/spidernet-io/spiderpool/tree/main/docs/example/reserved-ip/test-ipv4-reservedip.yaml
```

Another interesting question is that what happens if an IP address to be reserved has been allocated before SpiderReservedIP is created? Of course, we dare not stop this running Pod and recycle its IP addresses, but SpiderReservedIP will still ensure that no Pod can continue to use this IP address when this Pod releases these IP addresses. Therefore, SpiderReservedIPs should be confirmed as early as possible before network planning, rather than being supplemented at the end of all work.

### Clean up

Clean the relevant resources so that you can run this tutorial again.

```bash
kubectl delete -f https://github.com/spidernet-io/spiderpool/tree/main/docs/example/reserved-ip --ignore-not-found=true
```

## A Trap

So, can you use SpiderIPPool's field `excludeIPs` to achieve the same effect as SpiderReservedIP? The answer is **NO**! Look at such a case, now you want to reserve an IP `172.18.43.31` for an external application of the cluster, which may be a Redis node. To achieve this, you created such an SpiderIPPool:

```yaml
apiVersion: spiderpool.spidernet.io/v1
kind: SpiderIPPool
metadata:
  name: already-in-use
spec:
  ipVersion: 4
  subnet: 172.18.43.0/24
  ips:
  - 172.18.43.1-172.18.43.31
  excludeIPs:
  - 172.18.43.31
```

I believe that if there is only one SpiderIPPool under the subnet `172.18.43.0/24` network segment in cluster, there will be no problem and it can even work perfectly. Unfortunately, your friends may not know about it, and then he/she created such an SpiderIPPool (Different SpiderIPPools allow to define the same `subnet`. More details refer to [validation of SpiderIPPool](TODO)):

```yaml
apiVersion: spiderpool.spidernet.io/v1
kind: SpiderIPPool
metadata:
  name: created-by-someone-else
spec:
  ipVersion: 4
  subnet: 172.18.43.0/24
  ips:
  - 172.18.43.31-172.18.43.50
```

After a period of time, a Pod may be allocated with IP `172.18.43.31` from the SpiderIPPool `created-by-someone-else`, and then it holds the same IP address as your Redis node. After that, your Redis may not work as well.

So, if you really want to reserve an IP address instead of excluding an IP address, SpiderReservedIP makes life better.
