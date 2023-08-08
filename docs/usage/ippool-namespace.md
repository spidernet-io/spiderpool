# Namespace default IPPool

*Spiderpool provides default IP pools at Namespace level. A Pod not configured with a [pool selection rule](TODO) of higher priority will be assigned with IP addresses from the default IP pools of its Namespace.*

## Set up Spiderpool

If you have not deployed Spiderpool yet, follow the guide [installation](./install/underlay/get-started-kind.md) for instructions on how to deploy and easily configure Spiderpool.

## Get started

1. Create a Namespace named as `test-ns1`.

    ```bash
    kubectl create ns test-ns1
    ```

2. Create an IPPool to be bound with Namespace `test-ns1`.

    ```bash
    kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-namespace/ns1-default-ipv4-ippool.yaml
    ```

3. Check the status of this IPPool with the following command.

    ```bash
    kubectl get sp -l case=ns
    NAME                      VERSION   SUBNET           ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DISABLE
    ns1-default-ipv4-ippool   4         172.18.41.0/24   0                    4                false
    ```

4. Specify pool selection rules for Namespace `test-ns1` with the following command and annotation.

    ```bash
    kubectl patch ns test-ns1 --patch-file https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-namespace/ns1-ippool-selection-patch.yaml
    ```

    ```yaml
    metadata:
      annotations:
        ipam.spidernet.io/default-ipv4-ippool: '["ns1-default-ipv4-ippool"]'
    ```

5. Create a Deployment with 3 replicas in the Namespace `test-ns1`.

    ```bash
    kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-namespace/ns1-default-ippool-deploy.yaml
    ```

Now, all Pods in the Namespace should have been assigned with an IP address from the specified IPPool. Verify it with the following command:

```bash
kubectl get se -n test-ns1
NAME                                         INTERFACE   IPV4POOL                  IPV4              IPV6POOL   IPV6   NODE            CREATETION TIME
ns1-default-ippool-deploy-7cd5449c88-9xncm   eth0        ns1-default-ipv4-ippool   172.18.41.41/24                     spider-worker   57s
ns1-default-ippool-deploy-7cd5449c88-dpfjs   eth0        ns1-default-ipv4-ippool   172.18.41.43/24                     spider-worker   57s
ns1-default-ippool-deploy-7cd5449c88-vjtdd   eth0        ns1-default-ipv4-ippool   172.18.41.42/24                     spider-worker   58s
```

The Namespace annotation `ipam.spidernet.io/defaultv4ippool` also supports the syntax of [alternative IP pools](ippool-multi.md), which means **you can specify multiple default IP pools for a Namespace**. In addition, one IPPool can be specified as the default IP pool for different Namespaces.

> If you want to bind an IPPool to a specific Namespace in an **exclusive** way, it means that no Namespace other than this (or a group of Namespaces) has permission to use this IPPool, please refer to [SpiderIPPool namespace affinity](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/ippool-affinity-namespace.md).

## Clean up

Clean relevant resources so that you can run this tutorial again.

```bash
kubectl delete ns test-ns1
kubectl delete -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-namespace/ns1-default-ipv4-ippool.yaml --ignore-not-found=true
```
