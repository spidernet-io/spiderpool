# StatefulSet

**English** ｜ [**简体中文**](./statefulset-zh_CN.md)

## Introduction

*Due to StatefulSet being commonly used for stateful services, there is a higher demand for stable network identifiers. Spiderpool ensures that StatefulSet Pods consistently retain the same IP address, even in scenarios such as restarts or rebuilds.*

## StatefulSet features

StatefulSet utilizes fixed addresses in the following scenarios:

- When a StatefulSet Pod fails and needs to be reconstructed.

- Once a Pod is deleted and needs to be restarted and the replicas of the StatefulSet remains unchanged.

The requirements for fixed IP address differ between StatefulSet and Deployment:

- For StatefulSet, the Pod's name remains the same throughout Pod restarts despite its changed UUID. As the Pod is stateful, application administrators hope that each Pod continues to be assigned the same IP address after restarts.

- For Deployment, both the Pod name and its UUID change after restarts. Deployment Pods are stateless, so there is no need to maintain the same IP address between Pod restarts. Instead, administrators often prefer IP addresses to be allocated within a specified range for all replicas in Deployment.

Many open-source CNI solutions provide limited support for fixing IP addresses for StatefulSet. However, Spiderpool's StatefulSet solution guarantees consistent allocation of the same IP address to the Pods during restarts and rebuilds.

> - This feature is enabled by default. When it is enabled, StatefulSet Pods can be assigned fixed IP addresses from a specified IP pool range. Whether or not using a fixed IP pool, StatefulSet Pods will consistently receive the same IP address. StatefulSet applications will be treated as stateless if the feature is disabled. You can disable it during the installation of Spiderpool using Helm via the flag `--set ipam.enableStatefulSet=false`.
>
> - During the transition from scaling down to scaling up StatefulSet replicas, Spiderpool does not guarantee that new Pods will inherit the IP addresses previously used by the scaled-down Pods.
>
> - In version 0.9.4 and prior versions, when a StatefulSet is ready and its Pod is running, even if you modify the StatefulSet annotation to specify a different IP pool and restart the Pod, the Pod's IP address will not switch to the new IP pool range but will continue to use the old fixed IP. Starting from version 0.9.4 and above, changing the IP pool and restarting the Pod will complete the IP address switch.

## Prerequisites

1. A ready Kubernetes cluster.

2. [Helm](https://helm.sh/docs/intro/install/) has already been installed.

## Steps

### Install Spiderpool

Refer to [Installation](./readme.md) to install Spiderpool. And make sure that the helm installs the option `ipam.enableStatefulSet=true`.

### Install CNI

To simplify the creation of JSON-formatted Multus CNI configuration, Spiderpool introduces the SpiderMultusConfig CR, which automates the management of Multus NetworkAttachmentDefinition CRs. Here is an example of creating a Macvlan SpiderMultusConfig:

- master: the interface `ens192` is used as the spec for master.

```bash
MACVLAN_MASTER_INTERFACE="ens192"
MACVLAN_MULTUS_NAME="macvlan-$MACVLAN_MASTER_INTERFACE"

cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: ${MACVLAN_MULTUS_NAME}
  namespace: kube-system
spec:
  cniType: macvlan
  enableCoordinator: true
  macvlan:
    master:
    - ${MACVLAN_MASTER_INTERFACE}
EOF
```

With the provided configuration, we create the following Macvlan SpiderMultusConfig that will automatically generate a Multus NetworkAttachmentDefinition CR.

```bash
~# kubectl get spidermultusconfigs.spiderpool.spidernet.io -n kube-system
NAME             AGE
macvlan-ens192   26m

~# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system
NAME             AGE
macvlan-ens192   27m
```

### Create an IP pool

```bash
~# cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: test-ippool
spec:
  subnet: 10.6.0.0/16
  ips:
    - 10.6.168.101-10.6.168.110
EOF
```

### Create StatefulSet applications

The following YAML example creates a StatefulSet application with 2 replicas:

- `ipam.spidernet.io/ippool`: specify the IP pool for Spiderpool. Spiderpool automatically selects some IP addresses from this pool and bind them to the application, ensuring that the IP addresses remain fixed for the StatefulSet application.

- `v1.multus-cni.io/default-network`: create a default network interface for the application.

```bash
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test-sts
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test-sts
  template:
    metadata:
      annotations:
        ipam.spidernet.io/ippool: |-
            {      
              "ipv4": ["test-ippool"]
            }
        v1.multus-cni.io/default-network: kube-system/macvlan-ens192
      labels:
        app: test-sts
    spec:
      containers:
        - name: test-sts
          image: nginx
          imagePullPolicy: IfNotPresent
EOF
```

When the StatefulSet application is created, Spiderpool will select a random set of IP addresses from the specified IP pool and bind them to the application.

```bash
~# kubectl get spiderippool
NAME          VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
test-ippool   4         10.6.0.0/16   2                    10               false

~# kubectl get po -l app=test-sts -o wide
NAME         READY   STATUS    RESTARTS   AGE     IP             NODE    NOMINATED NODE   READINESS GATES
test-sts-0   1/1     Running   0          3m13s   10.6.168.105   node2   <none>           <none>
test-sts-1   1/1     Running   0          3m12s   10.6.168.102   node1   <none>           <none>
```

Upon restarting StatefulSet Pods, it is observed that each Pod retains its assigned IP address.

```bash
~# kubectl get pod | grep "test-sts" | awk '{print $1}' | xargs kubectl delete pod
pod "test-sts-0" deleted
pod "test-sts-1" deleted

~# kubectl get po -l app=test-sts -o wide
NAME         READY   STATUS    RESTARTS   AGE   IP             NODE    NOMINATED NODE   READINESS GATES
test-sts-0   1/1     Running   0          18s   10.6.168.105   node2   <none>           <none>
test-sts-1   1/1     Running   0          17s   10.6.168.102   node1   <none>           <none>
```

Upon scaling up or down the StatefulSet Pods, the IP addresses of each Pod change as expected.

```bash
~# kubectl scale deploy test-sts --replicas 3
statefulset.apps/test-sts scaled

~# kubectl get po -l app=test-sts -o wide
NAME         READY   STATUS    RESTARTS   AGE     IP             NODE    NOMINATED NODE   READINESS GATES
test-sts-0   1/1     Running   0          4m58s   10.6.168.105   node2   <none>           <none>
test-sts-1   1/1     Running   0          4m57s   10.6.168.102   node1   <none>           <none>
test-sts-2   1/1     Running   0          4s      10.6.168.109   node2   <none>           <none>

~# kubectl get pod | grep "test-sts" | awk '{print $1}' | xargs kubectl delete pod
pod "test-sts-0" deleted
pod "test-sts-1" deleted
pod "test-sts-2" deleted

~# kubectl get po -l app=test-sts -o wide
NAME         READY   STATUS    RESTARTS   AGE   IP             NODE    NOMINATED NODE   READINESS GATES
test-sts-0   1/1     Running   0          6s    10.6.168.105   node2   <none>           <none>
test-sts-1   1/1     Running   0          4s    10.6.168.102   node1   <none>           <none>
test-sts-2   1/1     Running   0          3s    10.6.168.109   node2   <none>           <none>

~# kubectl scale sts test-sts --replicas 2
statefulset.apps/test-sts scaled

~# kubectl get po -l app=test-sts -o wide
NAME         READY   STATUS    RESTARTS   AGE   IP             NODE    NOMINATED NODE   READINESS GATES
test-sts-0   1/1     Running   0          88s   10.6.168.105   node2   <none>           <none>
test-sts-1   1/1     Running   0          86s   10.6.168.102   node1   <none>           <none>
```

## Conclusion

Spiderpool ensures that StatefulSet Pods maintain a consistent IP address even during scenarios like restarts or rebuilds, satisfying the requirement for fixed IP addresses in StatefulSet.
