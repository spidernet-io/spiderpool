# Reserved IP

*Spiderpool reserve some IP addresses for the whole Kubernetes cluster, which will not be used by any IPAM allocation results. Typically, these IP addresses are external IP addresses or cannot be used for network communication (e.g. broadcast address).*

## IPPool excludeIPs

You may have observed that there is a field `excludeIPs` in SpiderIPPool CRD. To some extent, it is also a mechanism for reserving IP addresses, but its main function is not like this. Field `excludeIPs` is more of a **syntax sugar**, so that users can more flexibly define the IP address ranges of the IPPool.

For example, create an IPPool without using `excludeIPs`, which contains two IP ranges: `172.18.41.40-172.18.41.44` and `172.18.41.46-172.18.41.50`, you should define the `ips` as follows:

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: not-use-excludeips
spec:
  subnet: 172.18.41.0/24
  ips:
  - 172.18.41.40-172.18.41.44
  - 172.18.41.46-172.18.41.50
```

But in fact, this semantics can be more succinctly described through `excludeIPs`:

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: use-excludeips
spec:
  subnet: 172.18.41.0/24
  ips:
  - 172.18.41.40-172.18.41.50
  excludeIPs:
  - 172.18.41.45
```

Field `excludeIPs` will make sure that any Pod that allocates IP addresses from this IPPool will not use these excluded IP addresses. However, it should be noted that this mechanism only has an effect on the **IPPool itself** with `excludeIPs` defined.

## Use SpiderReservedIP

Unlike configuring field `excluedIPs` in SpiderIPPool CR, creating a SpiderReservedIP CR is really a way to define the global reserved IP address rules of a Kubernetes cluster. The IP addresses defined in ReservedIP cannot be used by any Pod in the cluster, regardless of whether some IPPools have inadvertently defined them. More details refer to [definition of SpiderReservedIP](https://github.com/spidernet-io/spiderpool/blob/main/docs/concepts/spiderreservedip.md).

### Set up Spiderpool

If you have not deployed Spiderpool yet, follow the guide [installation](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/install.md) for instructions on how to deploy and easily configure Spiderpool.

### Get Started

To understand how it works, let's do such a test. First, create an ReservedIP which reserves 9 IP addresses from `172.18.42.41` to `172.18.42.49`.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/reserved-ip/test-ipv4-reservedip.yaml
```

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderReservedIP
metadata:
  name: test-ipv4-reservedip
spec:
  ips:
  - 172.18.42.41-172.18.42.49
```

At the same time, create an IPPool with 10 IP addresses from `172.18.42.41` to `172.18.42.50`. Yes, we deliberately make it hold one more IP address than the ReservedIP above.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/reserved-ip/test-ipv4-ippool.yaml
```

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: test-ipv4-ippool
spec:
  subnet: 172.18.42.0/24
  ips:
  - 172.18.42.41-172.18.42.50
```

Then, create a Deployment with 3 replicas and allocate IP addresses to its Pods from the IPPool above.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/reserved-ip/reservedip-deploy.yaml
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
            "ipv4": ["test-ipv4-ippool"]
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

But when you delete this ReservedIP, everything will return to normal.

```bash
kubectl delete -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/reserved-ip/test-ipv4-reservedip.yaml
```

Another interesting question is that what happens if an IP address to be reserved has been allocated before ReservedIP is created? Of course, we dare not stop this running Pod and recycle its IP addresses, but ReservedIP will still ensure that when the Pod is terminated, no other Pods can continue to use the reserved IP address.

> Therefore, ReservedIPs should be confirmed as early as possible before network planning, rather than being supplemented at the end of all work.

### Clean up

Clean the relevant resources so that you can run this tutorial again.

```bash
kubectl delete \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/reserved-ip/test-ipv4-reservedip.yaml \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/reserved-ip/test-ipv4-ippool.yaml \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/reserved-ip/reservedip-deploy.yaml \
--ignore-not-found=true
```

## A Trap

So, can you use IPPool's field `excludeIPs` to achieve the same effect as ReservedIP? The answer is **NO**! Look at such a case, now you want to reserve an IP `172.18.43.31` for an external application of the Kubernetes cluster, which may be a Redis node. To achieve this, you created such an IPPool:

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: already-in-use
spec:
  subnet: 172.18.43.0/24
  ips:
  - 172.18.43.1-172.18.43.31
  excludeIPs:
  - 172.18.43.31
```

I believe that if there is only one IPPool under the subnet `172.18.43.0/24` network segment in cluster, there will be no problem and it can even work perfectly. Unfortunately, your friends may not know about it, and then he/she created such an IPPool:

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: created-by-someone-else
spec:
  subnet: 172.18.43.0/24
  ips:
  - 172.18.43.31-172.18.43.50
```

> Different IPPools allow to define the same field `subnet`, more details refer to [validation of IPPool](TODO).

After a period of time, a Pod may be allocated with IP `172.18.43.31` from the IPPool `created-by-someone-else`, and then it holds the same IP address as your Redis node. After that, the Redis may not work as well.

So, if you really want to reserve an IP address instead of excluding an IP address, SpiderReservedIP makes life better.
