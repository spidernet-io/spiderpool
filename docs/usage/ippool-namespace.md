# Namespace default IPPool

*Spiderpool provides default IP pools at Namespace level. Pods running under this Namespace and without [more priority pool selection rules](TODO) will allocate IP addresses from them.*

## Setup Spiderpool

If you have not set up Spiderpool yet, follow the guide [Quick Installation](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/install.md) for instructions on how to install and simply configure Spiderpool.

## Get started

First, let's create a new Namespace and the SpiderIPPool that will be bound to it.

```bash
kubectl create ns test-ns1
kubectl apply -f https://github.com/spidernet-io/spiderpool/tree/main/docs/example/ippool-namespace/ns1-default-ipv4-ippool.yaml
```

Obviously, we expect that all Pods created in Namespace `test-ns1` will be allocated IP addresses from SpiderIPPool `ns1-default-ipv4-ippool`.

```bash
kubectl get sp -l case=ns
NAME                      VERSION   SUBNET           ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DISABLE
ns1-default-ipv4-ippool   4         172.18.41.0/24   0                    4                false
```

Next, use `ipam.spidernet.io/defaultv4ippool` Namespace annotation to specify the pool selection rules and bind Namespace and SpiderIPPool one by one.

```bash
kubectl patch ns test-ns1 --patch-file https://github.com/spidernet-io/spiderpool/tree/main/docs/example/ippool-namespace/ns1-ippool-selection-patch.yaml
```

```yaml
metadata:
  annotations:
    ipam.spidernet.io/defaultv4ippool: '["ns1-default-ipv4-ippool"]'
```

Now, run a Deployment with 3 replicas under the Namespace `test-ns1`.

```bash
kubectl apply -f https://github.com/spidernet-io/spiderpool/tree/main/docs/example/ippool-namespace/ns1-default-ippool-deploy.yaml
```

Finally, as we initially expected,  all Pods in a specific Namespace are allocated IP addresses from the specified SpiderIPPool.

```bash
kubectl get se -n test-ns1
NAME                                         INTERFACE   IPV4POOL                  IPV4              IPV6POOL   IPV6   NODE            CREATETION TIME
ns1-default-ippool-deploy-7cd5449c88-9xncm   eth0        ns1-default-ipv4-ippool   172.18.41.41/24                     spider-worker   57s
ns1-default-ippool-deploy-7cd5449c88-dpfjs   eth0        ns1-default-ipv4-ippool   172.18.41.43/24                     spider-worker   57s
ns1-default-ippool-deploy-7cd5449c88-vjtdd   eth0        ns1-default-ipv4-ippool   172.18.41.42/24                     spider-worker   58s
```

Of course, the `ipam.spidernet.io/defaultv4ippool` Namespace annotation also supports the ability of [alternative IP pools](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/ippool-multi.md). We can specify multiple default IP pools for a certain Namespace at the same time. And, a certain SpiderIPPool can also be specified as the default IP pool for multiple Namespaces.

If you want to bind a SpiderIPPool to a specific Namespace in an **exclusive** way, it means that no Namespace other than this (or a group of Namespaces) has permission to use this SpiderIPPool, please refer to [SpiderIPPool namespace affinity](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/ippool-affinity-namespace.md).

## Clean up

Let's clean the relevant resources so that we can run this tutorial again.

```bash
kubectl delete ns test-ns1
kubectl delete -f https://github.com/spidernet-io/spiderpool/tree/main/docs/example/ippool-namespace --ignore-not-found=true
```
