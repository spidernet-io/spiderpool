# StatefulSet

## Description

The spiderpool supports IP assignment for StatefulSet.

When the number of statefulset replicas is not scaled up or down, all Pods could hold same IP address even when the Pod is restarting or rebuilding.

* Pod restarts

    Once a Pod restarts (its pause container restarts), Spiderpool will keep use the previous IP,
    and change the IPPool CR property ContainerID with the new pause container ID.

    In the meanwhile, spiderendpoint will still keep the previous IP but refresh the ContainerID property.

* Pod is deleted and re-created

    After deleting a StatefulSet Pod, kubernetes will re-create a Pod with the same name.

    In this case, Spiderpool will also keep the previous IP and update the ContainerID.

### Notice

* Currently, it's not allowed to change StatefulSet annotation for using another pool when a StatefulSet is ready and its Pods are running.

* When the statefulset is scaled down and then scaled up, the scaled-up Pod is not guaranteed to get the previous IP.

* The [IP-GC](./gc.md) feature (reclaim IP for the Pod of graceful-period timeout) does work for StatefulSet Pod.

## Get Started

### Enable StatefulSet support

Firstly, please ensure you have installed the spiderpool and configure the CNI file. Refer to [install](./install/install.md) for details.

Check whether the property `enableStatefulSet` of the configmap `spiderpool-conf` is already set to `true` or not.

```shell
kubectl -n kube-system get configmap spiderpool-conf -o yaml
```

If you want to set it `true`, run `helm upgrade spiderpool spiderpool/spiderpool --set ipam.enableStatefulSet=true -n kube-system`.

### Create a StatefulSet

This is an example to install a StatefulSet.

```shell
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/statefulset/statefulset-demo.yaml
```

### Validate the Spiderpool related CR data

Here's the created Pod, spiderippool, and spiderendpoint CR information:

```text
$ kubectl get po -o wide
NAME                       READY   STATUS    RESTARTS   AGE   IP              NODE            NOMINATED NODE   READINESS GATES
demo-sts-0                 1/1     Running   0          8s    172.22.40.181   spider-worker   <none>           <none>
---------------------------------------------------------------------------------------------------------------------
$ kubectl get sp default-v4-ippool -o yaml

...
"172.22.40.181":{"interface":"eth0","pod":"default/demo-sts-0","podUid":"fa50a7c2-99e7-4e97-a3f1-8d6503067b54"}
...

---------------------------------------------------------------------------------------------------------------------
$ kubectl get se demo-sts-0 -o yaml

...
status:
  current:
    ips:
    - interface: eth0
      ipv4: 172.22.40.181/16
      ipv4Pool: default-v4-ippool
      ipv6: fc00:f853:ccd:e793:f::d/64
      ipv6Pool: default-v6-ippool
      vlan: 0
    node: spider-worker
    uid: fa50a7c2-99e7-4e97-a3f1-8d6503067b54
  ownerControllerName: demo-sts
  ownerControllerType: StatefulSet
...
```

Try to delete Pod `demo-sts-0` and check whether the rebuilding Pod keeps the previous IP or not.

```text
$ kubectl delete po demo-sts-0
pod "demo-sts-0" deleted
---------------------------------------------------------------------------------------------------------------------
$ kubectl get po -o wide
NAME                       READY   STATUS    RESTARTS   AGE   IP              NODE            NOMINATED NODE   READINESS GATES
demo-sts-0                 1/1     Running   0          12s   172.22.40.181   spider-worker   <none>           <none>
---------------------------------------------------------------------------------------------------------------------
$ kubectl get sp default-v4-ippool -o yaml

...
"172.22.40.181":{"interface":"eth0","pod":"default/demo-sts-0","podUid":"425d6552-63bb-4b4c-aab2-b2db95de0ab1"}
...

---------------------------------------------------------------------------------------------------------------------
$ kubectl get se demo-sts-0 -o yaml

...
status:
  current:
    ips:
    - interface: eth0
      ipv4: 172.22.40.181/16
      ipv4Pool: default-v4-ippool
      ipv6: fc00:f853:ccd:e793:f::d/64
      ipv6Pool: default-v6-ippool
      vlan: 0
    node: spider-worker
    uid: 425d6552-63bb-4b4c-aab2-b2db95de0ab1
  ownerControllerName: demo-sts
  ownerControllerType: StatefulSet
...
```

And you can see, the re-created Pod still holds the previous IP, spiderippool, and spiderendpoint updated containerID property.

### clean up

Delete the StatefulSet object: `demo-sts`.

```text
$ kubectl delete sts demo-sts
statefulset.apps "demo-sts" deleted

---------------------------------------------------------------------------------------------------------------------

$ kubectl get sp default-v4-ippool -o yaml | grep demo-sts-0

---------------------------------------------------------------------------------------------------------------------

$ kubectl get se demo-sts-0 -o yaml
Error from server (NotFound): spiderendpoints.spiderpool.spidernet.io "demo-sts-0" not found
```

The related data is cleaned up now.
