# Bandwidth manager for IPVlan CNI

**English** ｜ [**简体中文**](./ipvlan_bandwidth-zh_CN.md)

This article will show how to implement the bandwidth management capabilities of IPVlan CNI with the help of the project [cilium-chaining](https://github.com/spidernet-io/cilium-chaining).

## Background

Kubernetes supports setting the ingress/egress bandwidth of a Pod by injecting Annotations into the Pod, refer to [Bandwidth Limiting](https://kubernetes.io/zh-cn/docs/concepts/extend-kubernetes/compute-storage-net/network-plugins/#support-traffic-shaping)

When we use IPVlan as CNI, it does not have the ability to manage the ingress/egress traffic bandwidth of the Pod itself. The open source project Cilium supports cni-chaining to work with IPVlan, based on eBPF technology, it can help IPVlan realize accelerated access to services, bandwidth capacity management and other functions. However, Cilium has removed support for IPVlan Dataplane in the latest release.

[cilium-chaining](https://github.com/spidernet-io/cilium-chaining) project is built on cilium v1.12.7 and supports IPVlan Dataplane, we can use it to support IPVlan Dataplane by cilium v1.12.7. Dataplane, we can use it to help IPVlan realize the network bandwidth management capability of Pod.

## Pre-conditions

* Helm and Kubectl binary tools.
* Requires node kernel to be greater than 4.19.

## Installing Spiderpool

Installation of Spiderpool can be found in the documentation: [Install Spiderpool](./install/underlay/get-started-macvlan.md)

## Install the cilium-chaining project

Install the cilium-chaining project with the following command.

```shell
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/cilium-chaining/main/manifests/cilium-chaining.yaml
```

Check the status of the installation: 

```shell
~# kubectl get po -n kube-system | grep cilium-chain
cilium-chaining-gl76b                        1/1     Running     0              137m
cilium-chaining-nzvrg                        1/1     Running     0              137m
```

## Creating a CNI config and IPPool

Refer to the following command to create a CNI configuration file:

```shell
# Create Multus Network-attachement-definition CR
IPVLAN_MASTER_INTERFACE=ens192
IPPOOL_NAME=ens192-v4
cat << EOF | kubectl apply -f - - apiVersion: k8s-v4
apiVersion: k8s.cni.cncf.io/v1
Type: NetworkAttachmentDefinition
Metadata:
  Name: ipvlan
  Namespace: kube-system
spec:
  config: ' { "cniVersion": "0.4.0", "name": "terway-chainer", "plugins": [ { "type":
    "ipvlan", "mode": "l2", "master": "${IPVLAN_MASTER_INTERFACE}", "ipam": { "type": "spiderpool", "default_ipv4_ippool":
    ["${ippool_name}"]}} , { "type": "cilium-cni" }, { "type": "coordinator" }
    ] }'
EOF
```

> Configure cilium-cni in cni-chain mode with ipvlan cni

Create a CNI configuration by referring to the following command.

```shell
# Create the IP pool
cat << EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata.
  name: ${IPPOOL_NAME}
spec: ${IPPOOL_NAME}
  default: false
  disable: false
  gateway: 172.51.0.1
  ipVersion: 4
  ips: 172.51.0.100-172.51.0.108
  - 172.51.0.100-172.51.0.108
  subnet: 172.51.0.230/16
```

> Note that ens192 needs to exist on the host, and the network segment on which the IP pool is configured needs to be the same as the physical network on which ens192 resides.

## Create an application to test bandwidth limitation

Create a test application using the CNI configuration and IP pool created above to verify that the Pod's bandwidth is limited.

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

A few annotations to introduce.

* `v1.multus-cni.io/default-network: ipvlan`: Specifies that the default CNI for the Pod is the previously created ipvlan.
* `kubernetes.io/ingress-bandwidth: 100m`: Sets the ingress bandwidth of the Pod to 100M.
* `kubernetes.io/ingress-bandwidth: 100m`: Sets the Pod's egress bandwidth to 100M.

```shell
~# kubectl get po -o wide
NAME                     READY   STATUS        RESTARTS   AGE    IP               NODE          NOMINATED NODE   READINESS GATES
test-58d785fb4c-b9cld    1/1     Running       0          175m   172.51.0.102     10-20-1-230   <none>           <none>
test-58d785fb4c-kwh4h    1/1     Running       0          175m   172.51.0.100     10-20-1-220   <none>           <none>
```

When the Pod is created, go to the Pod's network namespace and test its network bandwidth using the `iperf3` utility.

```shell
root@10-20-1-230:~# crictl ps | grep test
0e3e211f83723       8f2213208a7f5       39 seconds ago      Running             nginx                      0                   3f668220e8349       test-58d785fb4c-b9cld
root@10-20-1-230:~# crictl inspect 0e3e211f83723 | grep pid
    "pid": 976027,
            "pid": 1
            "type": "pid"
root@10-20-1-230:~# nsenter -t 976027 -n
root@10-20-1-230:~#
root@10-20-1-230:~# iperf3 -c 172.51.0.100
Connecting to host 172.51.0.100, port 5201
[  5] local 172.51.0.102 port 50504 connected to 172.51.0.100 port 5201
[ ID] Interval           Transfer     Bitrate         Retr  Cwnd
[  5]   0.00-1.00   sec  37.1 MBytes   311 Mbits/sec    0   35.4 KBytes
[  5]   1.00-2.00   sec  11.2 MBytes  94.4 Mbits/sec    0    103 KBytes
[  5]   2.00-3.00   sec  11.2 MBytes  94.4 Mbits/sec    0   7.07 KBytes
[  5]   3.00-4.00   sec  11.2 MBytes  94.4 Mbits/sec    0   29.7 KBytes
[  5]   4.00-5.00   sec  11.2 MBytes  94.4 Mbits/sec    0   33.9 KBytes
[  5]   5.00-6.00   sec  12.5 MBytes   105 Mbits/sec    0   29.7 KBytes
[  5]   6.00-7.00   sec  10.0 MBytes  83.9 Mbits/sec    0   62.2 KBytes
[  5]   7.00-8.00   sec  12.5 MBytes   105 Mbits/sec    0   22.6 KBytes
[  5]   8.00-9.00   sec  10.0 MBytes  83.9 Mbits/sec    0   69.3 KBytes
[  5]   9.00-10.00  sec  10.0 MBytes  83.9 Mbits/sec    0   52.3 KBytes
- - - - - - - - - - - - - - - - - - - - - - - - -
[ ID] Interval           Transfer     Bitrate         Retr
[  5]   0.00-10.00  sec   137 MBytes   115 Mbits/sec    0             sender
[  5]   0.00-10.00  sec   134 MBytes   113 Mbits/sec                  receiver

iperf Done.
```

You can see that the result is 115 Mbits/sec, indicating that the Pod's bandwidth has been limited to the size we defined in the annotations.
