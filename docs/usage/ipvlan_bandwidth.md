# IPVlan Bandwidth Management and Network Policies

[**简体中文**](./ipvlan_bandwidth-zh_CN.md) | **English**

This document demonstrates how to implement bandwidth management and network policy capabilities for IPVlan CNI using the [cilium-chaining](https://github.com/spidernet-io/cilium-chaining) project.

## Background

- Kubernetes officially supports setting Pod ingress/egress bandwidth through Pod Annotations, as documented in [Bandwidth Limits](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/network-plugins/#support-traffic-shaping). However, when using IPVlan as the CNI, it lacks the native capability to manage Pod ingress/egress traffic bandwidth.

- Kubernetes supports managing network traffic between Pods through NetworkPolicy CRDs. Network policies are typically implemented by CNI projects like Cilium and Calico, but this functionality is missing for Underlay CNIs.

The open-source project Cilium supports working with IPVlan in a CNI-chaining mode, utilizing eBPF technology to provide IPVlan with features like accelerated Service access and bandwidth management. However, Cilium has since removed support for the IPVlan Dataplane. The [cilium-chaining](https://github.com/spidernet-io/cilium-chaining) project, based on cilium v1.12.7, continues to support the IPVlan Dataplane, enabling us to implement Pod network bandwidth management and network policy capabilities for IPVlan.

## Prerequisites

- Helm and Kubectl binary tools
- Node kernel version must be greater than 4.19
- The cluster can have Calico installed as the default network, but the Cilium component should not be installed

## Install Spiderpool

Refer to the documentation for installing Spiderpool: [Install Spiderpool](./install/underlay/get-started-macvlan.md)

## Install Cilium-chaining Project

Use the following command to install the cilium-chaining project:

```shell
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/cilium-chaining/main/manifests/cilium-chaining.yaml
```

Check installation status:

```shell
~# kubectl get po -n kube-system | grep cilium-chain
cilium-chaining-gl76b                        1/1     Running     0              137m
cilium-chaining-nzvrg                        1/1     Running     0              137m
```

## Create CNI Configuration and IP Pool

Refer to the following command to create the CNI configuration:

```shell
IPVLAN_MASTER_INTERFACE=ens192
IPPOOL_NAME=ens192-v4
cat << EOF | kubectl apply -f - 
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: ipvlan-conf
  namespace: kube-system
  annotations:
    cni.spidernet.io/cniconfig-name: "spidernet-chainer"
spec:
  cniType: ipvlan
  ipvlan:
    master:
    - ens192
    ippools:
      ipv4:
      - ${IPPOOL_NAME}
  chainCNIJsonData: 
  - |
    {
        "type": "cilium-cni"
    }
EOF
```

> Configure cilium-cni to work in cni-chain mode with ipvlan cni
> You must specify `cni.spidernet.io/cniconfig-name` as `spidernet-chainer`, otherwise cilium-chaining cannot provide network policy and bandwidth management capabilities to Pods through CNI Chain mode

Check the generated Network-Attachment-Definition CR:

```shell
kubectl get network-attachment-definitions.k8s.cni.cncf.io -n spiderpool ipvlan-conf -o yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  annotations:
    cni.spidernet.io/cniconfig-name: "spidernet-chainer"
  creationTimestamp: "2025-11-12T02:26:04Z"
  generation: 2
  name: ipvlan-conf
  namespace: spiderpool
  ownerReferences:
  - apiVersion: spiderpool.spidernet.io/v2beta1
    blockOwnerDeletion: true
    controller: true
    kind: SpiderMultusConfig
    name: ipvlan-conf
    uid: 42f8cd3c-0e1b-49f0-be11-0e41a6d2dc2a
  resourceVersion: "39190606"
  uid: 053079b3-0036-43a2-a8f4-423879076e38
spec:
  config: '{"cniVersion":"0.3.1","name":"spidernet-chainer","plugins":[{"type":"ipvlan","master":"ens192","ipam":{"type":"spiderpool","default_ipv4_ippool":["ens192-v4"]}},{"type":"cilium-cni"},{"txQueueLen":0,"tunePodRoutes":true,"mode":"auto","type":"coordinator"}]}'
```

Create an IP pool:

```shell
cat << EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: ${IPPOOL_NAME}
spec:
  default: false
  disable: false
  gateway: 172.51.0.1
  ipVersion: 4
  ips:
  - 172.51.0.100-172.51.0.108
  subnet: 172.51.0.230/16
```

> Note: ens192 must exist on the host, and the IP pool subnet must match the physical network where ens192 is located

## Create Application to Test Bandwidth Limitation

Use the CNI configuration and IP pool created above to create a test application and verify if the Pod's bandwidth is limited:

```shell
cat << EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      annotations:
        v1.multus-cni.io/default-network: kube-system/ipvlan
        kubernetes.io/ingress-bandwidth: 100M
        kubernetes.io/egress-bandwidth: 100M
      labels:
        app: test
    spec:
      containers:
      - env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        - name: POD_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
        image: nginx
        imagePullPolicy: IfNotPresent
        name: nginx
        ports:
        - containerPort: 80
          name: http
          protocol: TCP
        resources: {}
```

Annotations explanation:

- `v1.multus-cni.io/default-network: ipvlan`: Specifies the default CNI for the Pod as the previously created ipvlan
- `kubernetes.io/ingress-bandwidth: 100M`: Sets the Pod's ingress bandwidth to 100M
- `kubernetes.io/egress-bandwidth: 100M`: Sets the Pod's egress bandwidth to 100M

```shell
~# kubectl get po -o wide
NAME                     READY   STATUS        RESTARTS   AGE    IP               NODE          NOMINATED NODE   READINESS GATES
test-58d785fb4c-b9cld    1/1     Running       0          175m   172.51.0.102     10-20-1-230   <none>           <none>
test-58d785fb4c-kwh4h    1/1     Running       0          175m   172.51.0.100     10-20-1-220   <none>           <none>
```

After the Pods are created, enter the network namespace of each Pod and use the `iperf3` tool to test the network bandwidth:

```shell
~# crictl ps | grep test
b2b60a6e14e21       8f2213208a7f5       10 seconds ago      Running             nginx                     0                   f46848c1a713a       test-58d785fb4c-kwh4h
~# crictl inspect b2b60a6e14e21 | grep pid
    "pid": 284529,
            "pid": 1
            "type": "pid"
~# nsenter -t 284529 -n
~# iperf3 -s
```

On another node, use another Pod as a client to access:

```bash
iperf3 -c <server-pod-ip>
```

You should observe that the bandwidth is limited to approximately 100Mbps, confirming that the bandwidth limitation is working as expected.

## Network Policy Testing

Create a network policy to prohibit all outbound traffic from the Pod:

```shell
cat << EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: test-ipvlan
  namespace: default
spec:
  ingress:
  - {}
  podSelector:
    matchLabels:
      app: test
  policyTypes:
  - Ingress
  - Egress
EOF
```

Test if the Pod's outbound traffic is restricted:

```
root@test-58d785fb4c-b9cld :/# ping 172.51.0.100 -c 1
PING 172.51.0.100 (172.51.0.100) 56(84) bytes of data.
^C
--- 172.51.0.100 ping statistics ---
0 packets transmitted, 0 received
```
