# Namespace default IPPool

*Spiderpool provides default IP pools at Namespace level. Pod running under a Namespace and not configured with a higher priority [pool selection rule](TODO) will be assigned with IP addresses from the Namespace default IP pools.*

## Set up Spiderpool

If you have not deployed Spiderpool yet, follow the guide [installation](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/install.md) for instructions on how to deploy and easily configure Spiderpool.

## Get started

First, create a new Namespace and an IPPool to be bound to it.

```bash
kubectl create ns test-ns1
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-namespace/ns1-default-ipv4-ippool.yaml
```

Next, We expect that all Pods created in Namespace `test-ns1` will be assigned with IP addresses from the IPPool `ns1-default-ipv4-ippool`.

```bash
kubectl get sp -l case=ns
NAME                      VERSION   SUBNET           ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DISABLE
ns1-default-ipv4-ippool   4         172.18.41.0/24   0                    4                false
```

Use Namespace annotation `ipam.spidernet.io/defaultv4ippool` to specify the pool selection rules and bind Namespace and IPPool one by one.

```bash
kubectl patch ns test-ns1 --patch-file https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-namespace/ns1-ippool-selection-patch.yaml
```

```yaml
metadata:
  annotations:
    ipam.spidernet.io/default-ipv4-ippool: '["ns1-default-ipv4-ippool"]'
```

Create a Deployment with 3 replicas under the Namespace `test-ns1`.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-namespace/ns1-default-ippool-deploy.yaml
```

Finally, all Pods in the specific Namespace are assigned with IP addresses from the specified IPPool.

```bash
kubectl get se -n test-ns1
NAME                                         INTERFACE   IPV4POOL                  IPV4              IPV6POOL   IPV6   NODE            CREATETION TIME
ns1-default-ippool-deploy-7cd5449c88-9xncm   eth0        ns1-default-ipv4-ippool   172.18.41.41/24                     spider-worker   57s
ns1-default-ippool-deploy-7cd5449c88-dpfjs   eth0        ns1-default-ipv4-ippool   172.18.41.43/24                     spider-worker   57s
ns1-default-ippool-deploy-7cd5449c88-vjtdd   eth0        ns1-default-ipv4-ippool   172.18.41.42/24                     spider-worker   58s
```

Of course, the Namespace annotation `ipam.spidernet.io/defaultv4ippool` also supports the syntax of [alternative IP pools](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/ippool-multi.md). You can specify multiple default IP pools for a certain Namespace at the same time. In addition, one IPPool can be specified as the default IP pool for different Namespaces.

>If you want to bind an IPPool to a specific Namespace in an **exclusive** way, it means that no Namespace other than this (or a group of Namespaces) has permission to use this IPPool, please refer to [SpiderIPPool namespace affinity](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/ippool-affinity-namespace.md).

## Clean up

Clean the relevant resources so that you can run this tutorial again.

```bash
kubectl delete ns test-ns1
kubectl delete -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-namespace/ns1-default-ipv4-ippool.yaml --ignore-not-found=true
```
