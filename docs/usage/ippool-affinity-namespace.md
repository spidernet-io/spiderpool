# Namespace affinity of IPPool

*Spiderpool supports affinity between IP pools and Namespaces. It means only Pods running under these Namespaces can use the IP pools that have an affinity to these Namespaces.*

>*Namespace affinity should be regarded as a **filtering mechanism** rather than a [pool selection rule](TODO).*

## Set up Spiderpool

If you have not deployed Spiderpool yet, follow the guide [installation](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/install.md) for instructions on how to deploy and easily configure Spiderpool.

## Get started

First, create a new Namespace `test-ns`.

```bash
kubectl create namespace test-ns
```

Create an IPPool that will be bound to it.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-affinity-namespace/test-ns-ipv4-ippool.yaml
```

```yaml
apiVersion: spiderpool.spidernet.io/v1
kind: SpiderIPPool
metadata:
  name: test-ns-ipv4-ippool
spec:
  ipVersion: 4
  subnet: 172.18.41.0/24
  ips:
  - 172.18.41.40-172.18.41.41
  namespaceAffinity:
    matchLabels:
      kubernetes.io/metadata.name: test-ns
```

>For convenience, this example uses a native Namespace label `kubernetes.io/metadata.name` as the matching condition of IPPool affinity. You can replace them with desired labels to match the corresponding Namespaces.

Next, create two Deployments under `test-ns` and `default` Namespaces respectively, and configure the Pods therein to get IP addresses from the IPPool above.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-affinity-namespace/different-ns-deploys.yaml
```

You will find that the Deployment under Namespace `test-ns` is running.

```bash
kubectl get deploy -n test-ns
NAME             READY   UP-TO-DATE   AVAILABLE   AGE
test-ns-deploy   1/1     1            1           35s
```

And its Pod has been assigned with an IP address from that IPPool.

```bash
kubectl get se -n test-ns
NAME                             INTERFACE   IPV4POOL              IPV4              IPV6POOL   IPV6   NODE            CREATETION TIME
test-ns-deploy-74c6784f9-dlkmx   eth0        test-ns-ipv4-ippool   172.18.41.41/24                     spider-worker   46s
```

However, the Deployment under Namespace `default` cannot work properly. You can troubleshoot with the Events of its Pod:

```bash
kubectl describe po default-ns-deploy-5587c7bd47-xbmj2 -n default
...
Events:
  Type     Reason                  Age   From               Message
  ----     ------                  ----  ----               -------
  Normal   Scheduled               18s   default-scheduler  Successfully assigned default/default-ns-deploy-5587c7bd47-xbmj2 to spider-worker
  Warning  FailedCreatePodSandBox  17s   kubelet            Failed to create pod sandbox: rpc error: code = Unknown desc = failed to setup network for sandbox "97f18ae3ee315f58347f8936f819dd20b29c2d0a3d457fc6f0022282bf513e91": [default/default-ns-deploy-5587c7bd47-xbmj2:macvlan-cni-default]: error adding container to network "macvlan-cni-default": spiderpool IP allocation error: [POST /ipam/ip][500] postIpamIpFailure  failed to allocate IP addresses in standard mode: no IPPool available, all IPv4 IPPools [test-ns-ipv4-ippool] of eth0 filtered out: unmatched Namespace affinity of IPPool test-ns-ipv4-ippool
```

Obviously, this Pod has no permission to get IP addresses from IPPool `test-ns-ipv4-ippool`.

>You can specify a [default IP pool for a Namespace](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/ippool-namespace.md) and set the corresponding `namespaceAffinity` for the IPPool to achieve the effect of "a Namespace static IP pool".

## Clean up

Clean the relevant resources so that you can run this tutorial again.

```bash
kubectl delete ns test-ns
kubectl delete \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-affinity-namespace/test-ns-ipv4-ippool.yaml \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-affinity-namespace/different-ns-deploys.yaml \
--ignore-not-found=true
```
