# Multiple IPPool

*Spiderpool can specify multiple alternative IP pools for an IPAM allocation.*

## Setup Spiderpool

If you have not set up Spiderpool yet, follow the guide [Quick Installation](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/install.md) for instructions on how to install and simply configure Spiderpool.

## Get Started

First, we create two SpiderIPPools, and they both contain 2 IP addresses.

```bash
kubectl apply -f https://github.com/spidernet-io/spiderpool/tree/main/docs/example/ippool-multi/test-ipv4-ippools.yaml
```

Then, run a Pod and allocate one of these IP addresses.

```bash
kubectl apply -f https://github.com/spidernet-io/spiderpool/tree/main/docs/example/ippool-multi/dummy-pod.yaml
```

As you can see, we still have 3 available IP addresses, one in SpiderIPPool `default-ipv4-ippool` and two in SpiderIPPool `backup-ipv4-ippool`.

```bash
kubectl get sp -l case=backup
NAME                  VERSION   SUBNET           ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DISABLE
backup-ipv4-ippool    4         172.18.42.0/24   0                    2                false
default-ipv4-ippool   4         172.18.41.0/24   1                    2                false
```

Now, run a Deployment with 2 replicas and let its Pods allocate IP addresses from the two SpiderIPPool above.

```bash
kubectl apply -f https://github.com/spidernet-io/spiderpool/tree/main/docs/example/ippool-multi/multi-ippool-deploy.yaml
```

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: multi-ippool-deploy
spec:
  replicas: 2
  selector:
    matchLabels:
      app: multi-ippool-deploy
  template:
    metadata:
      annotations:
        ipam.spidernet.io/ippool: |-
          {
            "interface": "eth0",
            "ipv4pools": ["default-ipv4-ippool", "backup-ipv4-ippool"]
          }
      labels:
        app: multi-ippool-deploy
    spec:
      containers:
      - name: multi-ippool-deploy
        image: busybox
        imagePullPolicy: IfNotPresent
        command: ["/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"]
```

Spiderpool will successively try to allocate IP addresses **in the order of** the elements in the "IP pool array" until the first allocation succeeds or all fail. Of course, you can specify the pool selection rules (that defines alternative IP pools) in [many ways](TODO). Here, we use `ipam.spidernet.io/ippool` Pod annotation to select IP pools.

Finally, we'll see that when IP addresses in SpiderIPPool `default-ipv4-ippool` use out, SpiderIPPool `backup-ipv4-ippool` takes over.

```bash
kubectl get se
NAME                                   INTERFACE   IPV4POOL              IPV4              IPV6POOL   IPV6   NODE            CREATETION TIME
dummy                                  eth0        default-ipv4-ippool   172.18.41.41/24                     spider-worker   1m20s
multi-ippool-deploy-669bf7cf79-4x88m   eth0        default-ipv4-ippool   172.18.41.40/24                     spider-worker   2m31s
multi-ippool-deploy-669bf7cf79-k7zkk   eth0        backup-ipv4-ippool    172.18.42.41/24                     spider-worker   2m31s
```

## Clean up

Let's clean the relevant resources so that we can run this tutorial again.

```bash
kubectl delete -f https://github.com/spidernet-io/spiderpool/tree/main/docs/example/ippool-multi --ignore-not-found=true
```
