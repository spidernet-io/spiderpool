# Reserved IP

**English** ｜ [**简体中文**](./reserved-ip-zh_CN.md)

## Introduction

*Spiderpool reserve some IP addresses for the whole Kubernetes cluster through the ReservedIP CR, ensuring that these addresses are not allocated by IPAM.*

## Features of Reserved IP

To avoid IP conflicts when it is known that an IP address is being used externally to the cluster, it can be a time-consuming and labor-intensive task to remove that IP address from existing IPPool instances. Furthermore, network administrators want to ensure that this IP address is not allocated from any current or future IPPool resources. To address these concerns, the ReservedIP CR allows for the specification of IP addresses that should not be utilized by the cluster. Even if an IPPool instance includes those IP addresses, the IPAM plugin will refrain from assigning them to Pods.

The IP addresses specified in the ReservedIP CR serve two purposes:

- Clearly identify those IP addresses already in use by hosts outside the cluster.

- Explicitly prevent the utilization of those IP addresses for network communication, such as subnet IPs or broadcast IPs.

## Prerequisites

1. A ready Kubernetes kubernetes.

2. [Helm](https://helm.sh/docs/intro/install/) has been already installed.

## Steps

### Install Spiderpool

Refer to [Installation](./readme.md) to install Spiderpool.

### Install CNI

To simplify the creation of JSON-formatted Multus CNI configurations, Spiderpool introduces the SpiderMultusConfig CR, which automates the management of Multus NetworkAttachmentDefinition CRs. Here is an example of creating a Macvlan SpiderMultusConfig:

- master：the interface `ens192` is used as the spec for master.

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

With the provided configuration, we create a Macvlan SpiderMultusConfig that will automatically generate the corresponding Multus NetworkAttachmentDefinition CR.

```bash
~# kubectl get spidermultusconfigs.spiderpool.spidernet.io -n kube-system
NAME             AGE
macvlan-ens192   26m

~# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system
NAME             AGE
macvlan-ens192   27m
```

### Create reserved IPs

To create reserved IPs, use the following YAML to specify `spec.ips` as `10.6.168.131-10.6.168.132`:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderReservedIP
metadata:
  name: test-reservedip
spec:
  ips:
  - 10.6.168.131-10.6.168.132
EOF
```

### Create an IP pool

Create an IP pool with `spec.ips` set to `10.6.168.131-10.6.168.133`, containing a total of 3 IP addresses. However, given the previously created reserved IPs, only 1 IP address is available in this IP pool.

```bash
cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: test-ippool
spec:
  subnet: 10.6.0.0/16
  ips:
  - 10.6.168.131-10.6.168.133
EOF
```

To allocate IP addresses from this IP pool, use the following YAML to create a Deployment with 2 replicas:

- `ipam.spidernet.io/ippool`: specify the IP pool for assigning IP addresses to the application

```shell
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      annotations:
        ipam.spidernet.io/ippool: |-
            {      
              "ipv4": ["test-ippool"]
            }
        v1.multus-cni.io/default-network: kube-system/macvlan-ens192
      labels:
        app: test-app
    spec:
      containers:
      - name: test-app
        image: nginx
        imagePullPolicy: IfNotPresent
        ports:
        - name: http
          containerPort: 80
          protocol: TCP
EOF
```

Because both IP addresses in the IP pool are reserved by the ReservedIP CR, only one IP address is available in the pool. This means that only one Pod of the application can run successfully, while the other Pod fails to create due to the "all IPs have been exhausted" error.

```bash
~# kubectl get po -owide
NAME                       READY   STATUS              RESTARTS   AGE   IP             NODE    NOMINATED NODE   READINESS GATES
test-app-67dd9f645-dv8xz   1/1     Running             0          17s   10.6.168.133   node2   <none>           <none>
test-app-67dd9f645-lpjgs   0/1     ContainerCreating   0          17s   <none>         node1   <none>           <none>
```

If a Pod of the application already has been assigned a reserved IP, adding that IP address to the ReservedIP CR will result in the replica failing to run after restarting. Use the following command to add the Pod's allocated IP address to the ReservedIP CR, and then restart the Pod. As expected, the Pod will fail to start due to the "all IPs have been exhausted" error.

```bash
~# kubectl patch spiderreservedip test-reservedip --patch '{"spec":{"ips":["10.6.168.131-10.6.168.133"]}}' --type=merge

～# kubectl delete po test-app-67dd9f645-dv8xz 
pod "test-app-67dd9f645-dv8xz" deleted

~# kubectl get po -owide
NAME                       READY   STATUS              RESTARTS   AGE     IP       NODE    NOMINATED NODE   READINESS GATES
test-app-67dd9f645-fvx4m   0/1     ContainerCreating   0          9s      <none>   node2   <none>           <none>
test-app-67dd9f645-lpjgs   0/1     ContainerCreating   0          2m18s   <none>   node1   <none>           <none>
```

Once the reserved IP is removed, the Pod can obtain an IP address and run successfully.

```bash
~# kubectl delete sr test-reservedip
spiderreservedip.spiderpool.spidernet.io "test-reservedip" deleted

~# kubectl get po -owide
NAME                       READY   STATUS    RESTARTS   AGE     IP             NODE    NOMINATED NODE   READINESS GATES
test-app-67dd9f645-fvx4m   1/1     Running   0          4m23s   10.6.168.133   node2   <none>           <none>
test-app-67dd9f645-lpjgs   1/1     Running   0          6m14s   10.6.168.131   node1   <none>           <none>
```

## Conclusion

SpiderReservedIP simplifies network planning for infrastructure administrators.
