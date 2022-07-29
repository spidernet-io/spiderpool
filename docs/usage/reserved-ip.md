# Reserved IP

>Spiderpool reserve some IP addresses for the whole cluster, which will not be used by any IPAM allocation. Usually, these IP addresses are some external IP addresses.

## IPPool excludeIPs

First of all, you may have observed that there is an `excludeIPs` field in IPPool CRD. To some extent, it is also a mechanism for reserving IP addresses, but its main work is not so. Honestly, `excludeIPs` field is more of a **syntax sugar**, so that users can define their IPPool CRD more flexibly.

For example, now we want to create an IPPool, which contains two IP ranges: `172.18.40.40-172.18.40.44` and `172.18.40.46-172.18.40.50`. Without using `excludeIPs`, we may need the following IPPool manifest:

```yaml
apiVersion: spiderpool.spidernet.io/v1
kind: IPPool
metadata:
  name: not-use-excludeIPs
spec:
  ipVersion: 4
  subnet: 172.18.40.0/24
  ips:
  - 172.18.40.40-172.18.40.44
  - 172.18.40.46-172.18.40.50
```

But in fact, we can more concisely describe this semantics through `excludeIPs`:

```yaml
apiVersion: spiderpool.spidernet.io/v1
kind: IPPool
metadata:
  name: use-excludeIPs
spec:
  ipVersion: 4
  subnet: 172.18.40.0/24
  ips:
  - 172.18.40.40-172.18.40.50
  excludeIPs:
  - 172.18.40.45
```

Of course, `ExcludeIPs` also supports the format of IP range and we can define multiple `excludeIPs` records to segment a subnet in more detail:

```yaml
apiVersion: spiderpool.spidernet.io/v1
kind: IPPool
metadata:
  name: IP-ranges-in-excludeIPs
spec:
  ipVersion: 4
  subnet: 172.18.40.0/24
  ips:
  - 172.18.40.40-172.18.40.50
  excludeIPs:
  - 172.18.40.45
  - 172.18.40.47-172.18.40.49
```

`excludeIPs` will make sure that any Pod that allocates IP from this IPPool will not use these excluded IP addresses. However, it should be noted that this mechanism only has an effect on the **IPPool itself** with `excludeIPs` defined.

## ReservedIP

Unlike `excluedIPs` field in IPPool CRD, ReservedIP CRD is actually used to define the global reserved IP rules of a cluster. ReservedIP ensures that no Pod in a cluster will use these IP addresses defined by it, whether or not some IPPools have inadvertently defined the same IP addresses for Pods use.

For example, we have such an IPPool with 10 IP addresses from `172.18.50.40` to `172.18.50.50`:

```yaml
apiVersion: spiderpool.spidernet.io/v1
kind: IPPool
metadata:
  name: test-IPv4-IP-pool
spec:
  ipVersion: 4
  subnet: 172.18.50.0/24
  ips:
  - 172.18.50.40-172.18.50.50
```

Unfortunately, a ReservedIP has been **pre created** in cluster, which reserves 100 IP addresses from `172.18.50.1` to `172.18.50.100`:

```yaml
apiVersion: spiderpool.spidernet.io/v1
kind: ReservedIP
metadata:
  name: reserved
spec:
  ipVersion: 4
  ips:
  - 172.18.50.1-172.18.50.100
```

Now, if we create a Deployment and let its Pods allocate IP addresses from IPPool `test-IPv4-IP-pool`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: example
spec:
  replicas: 3
  selector:
    matchLabels:
      app: example
  template:
    metadata:
      annotations:   
        ipam.spidernet.io/ippool: |-
          {
            "interface": "eth0",
            "ipv4pools": ["test-IPv4-IP-pool"],
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

None of these Pods can run successfully because "all IP addresses are used out". But when we remove this ReservedIP `reserved`, everything will return to normal.

```bash
$ kubectl get po -l app=example
NAMESPACE     NAME                                          READY   STATUS              RESTARTS   AGE
default       example-6c5cdc6fb6-hvzrp                      0/1     ContainerCreating   0          35s
default       example-6c5cdc6fb6-zj2zk                      0/1     ContainerCreating   0          35s
default       example-6c5cdc6fb6-k2fkm                      0/1     ContainerCreating   0          35s
```

Another interesting question is that what happens if an IP address to be reserved has been allocated before ReservedIP is created? Of course, we dare not stop this running Pod and recycle IP, but ReservedIP will still ensure that no Pod can continue to use this IP address when this Pod releases its IP. Therefore, ReservedIPs should be confirmed as early as possible before network planning, rather than being supplemented at the end of all work.

## A Trap

So, can we use IPPool's field `excludeIPs` to achieve the same effect as ReservedIP CRD? The answer is **NO**! Look at such a case, now we want to reserve an IP `172.18.60.31` for an external application of the cluster, which may be a Redis node. To achieve this, we created such an IPPool:

```yaml
apiVersion: spiderpool.spidernet.io/v1
kind: IPPool
metadata:
  name: IPv4-IP-pool-already-in-use
spec:
  ipVersion: 4
  subnet: 172.18.60.0/24
  ips:
  - 172.18.60.1-172.18.60.31
  excludeIPs:
  - 172.18.60.31
```

I believe that if there is only one IPPool under the subnet `172.18.60.0/24` network segment in cluster, there will be no problem and it can even work perfectly. Unfortunately, your friends may not know about it, and then he/she created such an IPPool (Different IPPools allow to define the same subnet, more details of [validating of IPPool CRD](TODO)):

```yaml
apiVersion: spiderpool.spidernet.io/v1
kind: IPPool
metadata:
  name: IPv4-IP-pool-created-by-someone
spec:
  ipVersion: 4
  subnet: 172.18.60.0/24
  ips:
  - 172.18.60.31-172.18.60.50
```

After a period of time, a Pod may be allocated IP `172.18.60.31` from IPPool `IPv4-IP-pool-created-by-someone`, and then it holds the same IP address as your Redis node. After that, your Redis may not work as well.

So, if you really want to reserve an IP address instead of excluding an IP address, ReservedIP CRD makes life better :)
