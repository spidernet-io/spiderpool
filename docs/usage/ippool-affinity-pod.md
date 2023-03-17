# Pod affinity of IPPool

*Spiderpool supports affinity between IP pools and Pods. This feature helps you to better implement the capability of **static IP** for workloads (Deployment, StatefulSet, etc.).*

>*Pod affinity should be regarded as a **filtering mechanism** rather than a [pool selection rule](TODO).*

## Set up Spiderpool

If you have not deployed Spiderpool yet, follow the guide [installation](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/install.md) for instructions on how to deploy and easily configure Spiderpool.

## Get started

First, create an IPPool configured with `podAffinity`.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-affinity-pod/static-ipv4-ippool.yaml
```

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: static-ipv4-ippool
spec:
  ipVersion: 4
  subnet: 172.18.41.0/24
  ips:
  - 172.18.41.40-172.18.41.43
  podAffinity:
    matchLabels:
      app: static
```

>This means that only the Pods with the label `app: static` can use this IPPool.

Then, create a Deployment that select its Pods by using the same label and set the Pod annotation `ipam.spidernet.io/ippool` to explicitly specify the pool selection rule.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-affinity-pod/static-ippool-deploy.yaml
```

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: static-ippool-deploy
spec:
  replicas: 2
  selector:
    matchLabels:
      app: static
  template:
    metadata:
      annotations:
        ipam.spidernet.io/ippool: |-
          {
            "ipv4": ["static-ipv4-ippool"]
          }
      labels:
        app: static
    spec:
      containers:
      - name: static-ippool-deploy
        image: busybox
        imagePullPolicy: IfNotPresent
        command: ["/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"]
```

The Pods are running.

```bash
kubectl get po -l app=static -o wide
NAME                                    READY   STATUS    RESTARTS   AGE   IP             NODE
static-ippool-deploy-7f478cc7d7-l7wm5   1/1     Running   0          20s   172.18.41.42   spider-control-plane
static-ippool-deploy-7f478cc7d7-vphw9   1/1     Running   0          20s   172.18.41.40   spider-worker
```

If necessary, you can try to create this Deployment which will also use the same IPPool `static-ipv4-ippool` to allocate IP addresses, but its Pods do not have the label `app: static`.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-affinity-pod/wrong-static-ippool-deploy.yaml
```

As a result, these Pods cannot run successfully because they do not have permission to use the IPPool.

```bash
kubectl describe po wrong-static-ippool-deploy-6c496cfb7d-wptq5
...
Events:
  Type     Reason                  Age   From               Message
  ----     ------                  ----  ----               -------
  Normal   Scheduled               35s   default-scheduler  Successfully assigned default/wrong-static-ippool-deploy-6c496cfb7d-wptq5 to spider-worker
  Warning  FailedCreatePodSandBox  34s   kubelet            Failed to create pod sandbox: rpc error: code = Unknown desc = failed to setup network for sandbox "a6f717aede91a356b552ad38c66112a26e5f7a4f7d23b7067870f33f05d350bc": [default/wrong-static-ippool-deploy-6c496cfb7d-wptq5:macvlan-cni-default]: error adding container to network "macvlan-cni-default": spiderpool IP allocation error: [POST /ipam/ip][500] postIpamIpFailure  failed to allocate IP addresses in standard mode: no IPPool available, all IPv4 IPPools [static-ipv4-ippool] of eth0 filtered out: unmatched Pod affinity of IPPool static-ipv4-ippool
```

## Summary

The steps to enable static IP addresses (i.e, static IPPool) for a Deployment are as follows:

1. Create an  IPPool you expect and configure its `podAffinity`.

2. Prepare a Deployment whose Pods have the label (or labels) matching the affinity.

   >Usually, you should select the value of the field `matchLabels` as this label (or labels).

3. Specify pool selection rules through the annotation `ipam.spidernet.io/ippool` or `ipam.spidernet.io/ippools` (multi-NIC) in Pod template. You can learn more about their differences [here](TODO).

4. Apply the manifest of this Deployment.

An interesting question is, with the scale up of a Deployment, what happens when the IP addresses in the IPPool are insufficient? The result is that some Pods will not work because they cannot be assigned with some IP addresses and you have to **manually** expand the IPPool to solve this problem.

However, if you enable the feature *SpiderSubnet* of Spiderpool, the IPPool will not only be automatically created with the creation of the Deployment (or other workloads), but also automatically scale up/down with the change of `replicas`. More details about [SpiderSubnet](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/spider-subnet.md).

Finally, as for how StatefulSet enables static IP addresses, you can get some help [here](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/statefulset.md).

## Clean up

Clean the relevant resources so that you can run this tutorial again.

```bash
kubectl delete \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-affinity-pod/static-ippool-deploy.yaml \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-affinity-pod/wrong-static-ippool-deploy.yaml \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-affinity-pod/static-ipv4-ippool.yaml \
--ignore-not-found=true
```
