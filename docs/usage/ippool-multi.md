# Multiple IPPool

*Spiderpool can specify multiple alternative IP pools for one IP allocation.*

## Set up Spiderpool

If you have not deployed Spiderpool yet, follow the guide [installation](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/install.md) for instructions on how to deploy and easily configure Spiderpool.

## Get Started

First, create two IPPools each containing 2 IP addresses.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-multi/test-ipv4-ippools.yaml
```

Create a Pod and allocate an IP address to it from these IPPools.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-multi/dummy-pod.yaml
```

You will find that you still have 3 available IP addresses, one in IPPool `default-ipv4-ippool` and two in IPPool `backup-ipv4-ippool`.

```bash
kubectl get sp -l case=backup
NAME                  VERSION   SUBNET           ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DISABLE
backup-ipv4-ippool    4         172.18.42.0/24   0                    2                false
default-ipv4-ippool   4         172.18.41.0/24   1                    2                false
```

Then, create a Deployment with 2 replicas and allocate IP addresses to its Pods from the two IPPools above.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-multi/multi-ippool-deploy.yaml
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
            "ipv4": ["default-ipv4-ippool", "backup-ipv4-ippool"]
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

Spiderpool will successively try to allocate IP addresses **in the order of** the elements in the "IP pool array" until the first allocation succeeds or all fail. Of course, you can specify the [pool selection rules](TODO) (that defines alternative IP pools) in many ways, the Pod annotation `ipam.spidernet.io/ippool` is used here to select IP pools.

Finally, when addresses in IPPool `default-ipv4-ippool` are used up, the IPPool `backup-ipv4-ippool` takes over.

```bash
kubectl get se
NAME                                   INTERFACE   IPV4POOL              IPV4              IPV6POOL   IPV6   NODE            CREATETION TIME
dummy                                  eth0        default-ipv4-ippool   172.18.41.41/24                     spider-worker   1m20s
multi-ippool-deploy-669bf7cf79-4x88m   eth0        default-ipv4-ippool   172.18.41.40/24                     spider-worker   2m31s
multi-ippool-deploy-669bf7cf79-k7zkk   eth0        backup-ipv4-ippool    172.18.42.41/24                     spider-worker   2m31s
```

## Clean up

Clean the relevant resources so that you can run this tutorial again.

```bash
kubectl delete \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-multi/test-ipv4-ippools.yaml \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-multi/dummy-pod.yaml \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-multi/multi-ippool-deploy.yaml \
--ignore-not-found=true
```
