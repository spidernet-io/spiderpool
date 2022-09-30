# StatefulSet

## Description

The spiderpool supports IP assignment for StatefulSet.

when the replica number of statefulset is not scaled up or down, all pods could hold same IP address even pod restarting or rebuilding happens.

* Pod restarts

    Once a pod restarts (its pause container restarts), Spiderpool will keep use the previous IP,
    and change the IPPool CR property ContainerID with the new pause container ID.

    In the meanwhile, spiderendpoint will still keep the previous IP but refresh the ContainerID property.

* Pod deleted and re-creates

    After deleting a StatefulSet pod, kubernetes will re-create a pod with the same name.

    In this case, Spiderpool will also keep the previous IP and update the ContainerID.

### Notice

* Currently, it's not allowed to change StatefulSet annotation for using another pool when a StatefulSet is ready and its pods are running.

* When the statefulset is scaled down and then scaled up, the scaled-up pod is not guaranteed to get the IP of scaled-down pod event they have the same name

* The [RIPOGT feature](./ippool-gc.md) ( reclaim IP for the pod of graceful-period timeout ) does work for statefulset pod.

## Get Started

### Enable StatefulSet support

Firstly, please ensure you have installed the spiderpool and configure the CNI file, refer [install](./install.md) for details

Check configmap `spiderpool-conf` property `enableStatefulSet` whether is already set to `true` or not.

```shell
kubectl -n kube-system get configmap spiderpool-conf -o yaml
```

If you want to set it `true`, just execute `helm upgrade --set feature.enableStatefulSet=true`

### Create a StatefulSet

install a StatefulSet example

```shell
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/statefulset/statefulset-demo.yaml
```

### Validate the Spiderpool related CR data

1. Here's the created Pod, spiderippool, spiderendpoint CR information:

    ```text
    $ kubectl get po -o wide
    NAME                        READY   STATUS    RESTARTS      AGE   IP              NODE            NOMINATED NODE   READINESS GATES
    demo-sts-0                  1/1     Running   0             82s   172.18.40.47    spider-worker   <none>           <none>

    ---------------------------------------------------------------------------------------------------------------------

    $ kubectl get sp default-v4-ippool -o yaml
    ...
    172.18.40.47:
    containerID: cffbddab79dc8eea447315da9b84db402d515b657d2b6943a87b47cdfa876359
    interface: eth0
    namespace: default
    node: spider-worker
    ownerControllerType: StatefulSet
    pod: demo-sts-0
    ...

    ---------------------------------------------------------------------------------------------------------------------

    $ kubectl get se demo-sts-0 -o yaml
    ...
    status:
    current:
    containerID: cffbddab79dc8eea447315da9b84db402d515b657d2b6943a87b47cdfa876359
    creationTime: "2022-07-29T08:54:12Z"
    ips:
    - interface: eth0
    ipv4: 172.18.40.47/16
    ipv4Gateway: ""
    ipv4Pool: default-v4-ippool
    ipv6: fc00:f853:ccd:e793:f::63/64
    ipv6Gateway: ""
    ipv6Pool: default-v6-ippool
    vlan: 0
    node: spider-worker
    ...
    ```

2. Try to delete pod `demo-sts-0` and check whether the rebuilding pod keeps the previous IP or not.

    ```text
    $ kubectl delete po demo-sts-0
    pod "demo-sts-0" deleted

    ---------------------------------------------------------------------------------------------------------------------

    $ kubectl get po -o wide
    NAME                        READY   STATUS    RESTARTS      AGE   IP              NODE            NOMINATED NODE   READINESS GATES
    demo-sts-0                  1/1     Running   0             20s   172.18.40.47    spider-worker   <none>           <none>

    ---------------------------------------------------------------------------------------------------------------------

    $ kubectl get sp default-v4-ippool -o yaml
    ...
    172.18.40.47:
    containerID: 5c7a1c9cf494c02090848bd3f8131817d02ee9f3046cd33a5ec4b74b897d6789
    interface: eth0
    namespace: default
    node: spider-worker
    ownerControllerType: StatefulSet
    pod: demo-sts-0
    ...

    ---------------------------------------------------------------------------------------------------------------------

    $ kubectl get se demo-sts-0 -o yaml
    ...
    status:
    current:
    containerID: 5c7a1c9cf494c02090848bd3f8131817d02ee9f3046cd33a5ec4b74b897d6789
    creationTime: "2022-07-29T08:54:12Z"
    ips:
    - interface: eth0
    ipv4: 172.18.40.47/16
    ipv4Gateway: ""
    ipv4Pool: default-v4-ippool
    ipv6: fc00:f853:ccd:e793:f::63/64
    ipv6Gateway: ""
    ipv6Pool: default-v6-ippool
    vlan: 0
    node: spider-worker
    history:
      - containerID: 5c7a1c9cf494c02090848bd3f8131817d02ee9f3046cd33a5ec4b74b897d6789
        creationTime: "2022-07-29T08:54:12Z"
        ips:
        - interface: eth0
          ipv4: 172.18.40.47/16
          ipv4Gateway: ""
          ipv4Pool: default-v4-ippool
          ipv6: fc00:f853:ccd:e793:f::63/64
          ipv6Gateway: ""
          ipv6Pool: default-v6-ippool
          vlan: 0
          node: spider-worker
      - containerID: cffbddab79dc8eea447315da9b84db402d515b657d2b6943a87b47cdfa876359
        creationTime: "2022-07-29T08:54:12Z"
        ips:
        - interface: eth0
          ipv4: 172.18.40.47/16
          ipv4Gateway: ""
          ipv4Pool: default-v4-ippool
          ipv6: fc00:f853:ccd:e793:f::63/64
          ipv6Gateway: ""
          ipv6Pool: default-v6-ippool
          vlan: 0
          node: spider-worker
          ownerControllerType: StatefulSet
          ...
    ```

    And you can see, the re-create Pod still hold the previous IP, and spiderippool, spiderendpoint updated containerID property.

### clean up

delete StatefulSet object `demo-sts`.

```text
$ kubectl delete sts demo-sts
statefulset.apps "demo-sts" deleted

---------------------------------------------------------------------------------------------------------------------

$ kubectl get sp default-v4-ippool -o yaml | grep demo-sts-0

---------------------------------------------------------------------------------------------------------------------

$ kubectl get se demo-sts-0 -o yaml
Error from server (NotFound): spiderendpoints.spiderpool.spidernet.io "demo-sts-0" not found
```

   The related data was cleaned up.
