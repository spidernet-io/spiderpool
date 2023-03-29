# Pod affinity of IPPool

IPpool supports affinity setting for Pods. when setting `sepc.podAffinity` in SpiderIPPool, only the selected pods 
could get IP address from it, and another pods could not even specifying the annotation.

* when setting the `sepc.podAffinity`, it helps to better implement the capability of **static IP** for workloads like Deployment, StatefulSet etc.

* when no `sepc.podAffinity`, all applications could share the IP address in the ippool

## Get started

the example shows how `sepc.podAffinity` works.

### Set up Spiderpool

follow the guide [installation](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/install.md) to install Spiderpool.

### shared ippool

TODO

### occupied ippool

1. create an IPPool configured with `podAffinity`. the `spec.podAffinity` means only application labeled with `app: static` can get IP from this IPPool.

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

2. create a Deployment whose Pods are labeled with `app: static`, and set the Pod annotation `ipam.spidernet.io/ippool` to explicitly specify the pool selection rule. It will succeed to get IP address.
   
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

3. create a Deployment using the same IPPool `static-ipv4-ippool` to allocate IP addresses, but its Pods do not have the label `app: static`, it will fail to get IP address.
    
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

### Clean up

Clean the relevant resources so that you can run this tutorial again.

    ```bash
    kubectl delete \
    -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-affinity-pod/static-ippool-deploy.yaml \
    -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-affinity-pod/wrong-static-ippool-deploy.yaml \
    -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-affinity-pod/static-ipv4-ippool.yaml \
    --ignore-not-found=true
    ```
