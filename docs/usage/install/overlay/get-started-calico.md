# Calico Quick Start

**English** | [**简体中文**](./get-started-calico-zh_cn.md)

This page showcases the utilization of `Spiderpool`, a comprehensive Underlay network solution, in a cluster where [Calico](https://github.com/projectcalico/calico) serves as the default CNI. `Spiderpool` leverages Multus to attach an additional NIC created with `Macvlan` to Pods and coordinates routes among multiple NICs using `coordinator`. This setup enables Pod's east-west traffic to be forwarded through the Calico-created NIC (eth0). The advantages offered by Spiderpool's solution are:

- Solve Macvlan's problem for accessing ClusterIP when Pods have both Calico and Macvlan NICs attached.
- Facilitate the forwarding of external access to NodePort through Calico's data path, eliminating the need for external routing. Whereas, external routing is typically required for forwarding when Macvlan is used as the CNI.
- Coordinate subnet routing for Pods with multiple Calico and Macvlan NICs, guaranteeing consistent traffic forwarding path for Pods and uninterrupted network connectivity.

> `NAD` is an abbreviation for Multus **N**etwork-**A**ttachment-**D**efinition CR.

## Prerequisites

- A ready Kubernetes cluster.
- Calico has been already installed as the default CNI for your cluster. If it is not installed, please refer to [the official documentation](https://docs.tigera.io/calico/latest/getting-started/kubernetes/) or follow the commands below for installation:

   ```shell
   ~# kubectl apply -f https://github.com/projectcalico/calico/blob/master/manifests/calico.yaml
   ~# kubectl wait --for=condition=ready -l k8s-app=calico-node  pod -n kube-system 
   ```

  
- Helm binary

## Install Spiderpool

Follow the command below to install Spiderpool:

```shell
~# helm repo add spiderpool https://spidernet-io.github.io/spiderpool
~# helm repo update spiderpool
~# helm install spiderpool spiderpool/spiderpool --namespace kube-system  --set coordinator.mode=overlay --wait 
```

> If Macvlan CNI is not installed in your cluster, you can install it on each node by using the Helm parameter `--set plugins.installCNI=true`.
>
> Specify the name of the NetworkAttachmentDefinition instance for the default CNI used by Multus via `multus.multusCNI.defaultCniCRName`. If the `multus.multusCNI.defaultCniCRName` option is provided, an empty NetworkAttachmentDefinition instance will be automatically generated upon installation. Otherwise, Multus will attempt to create a NetworkAttachmentDefinition instance based on the first CNI configuration found in the /etc/cni/net.d directory. If no suitable configuration is found, a NetworkAttachmentDefinition instance named `default` will be created to complete the installation of Multus.

Check the status of Spiderpool after the installation is complete:

```shell
~# kubectl get po -n kube-system | grep spiderpool
spiderpool-agent-htcnc                                      1/1     Running     0                 1m
spiderpool-agent-pjqr9                                      1/1     Running     0                 1m
spiderpool-controller-7b7f8dd9cc-xdj95                      1/1     Running     0                 1m
spiderpool-init                                             0/1     Completed   0                 1m
```

Please check if `Spidercoordinator.status.phase` is `Synced`, and if the overlayPodCIDR is consistent with the pod subnet configured by Cilium in the cluster:

```shell
~# kubectl get configmaps -n kube-system cilium-config -o yaml | grep cluster-pool
  cluster-pool-ipv4-cidr: 10.244.64.0/18
  cluster-pool-ipv4-mask-size: "24"
  ipam: cluster-pool

~# kubectl  get spidercoordinators.spiderpool.spidernet.io default -o yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderCoordinator
metadata:
  finalizers:
  - spiderpool.spidernet.io
  name: default
spec:
  detectGateway: false
  detectIPConflict: false
  hijackCIDR:
  - 169.254.0.0/16
  hostRPFilter: 0
  hostRuleTable: 500
  mode: auto
  podCIDRType: calico
  podDefaultRouteNIC: ""
  podMACPrefix: ""
  tunePodRoutes: true
status:
  overlayPodCIDR:
  - 10.244.64.0/18
  phase: Synced
  serviceCIDR:
  - 10.233.0.0/18
```

> 1.If the phase is not synced, the pod will be prevented from being created.
> 
> 2.If the overlayPodCIDR does not meet expectations, it may cause pod communication issue.

### Create SpiderIPPool

The subnet for the interface `ens192` on the cluster nodes here is `10.6.0.0/16`. Create a SpiderIPPool using this subnet:

```shell
cat << EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: 10-6-v4
spec:
  disable: false
  gateway: 10.6.0.1
  ipVersion: 4
  ips:
  - 10.6.212.100-10.6.212.200
  subnet: 10.6.0.0/16
EOF
```

> The subnet should be consistent with the subnet of `ens192` on the nodes, and ensure that the IP addresses do not conflict with any existing ones.

### Create SpiderMultusConfig

The Multus NAD instance is created using Spidermultusconfig:

```shell
cat << EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: macvlan-ens192
spec:
  cniType: macvlan
  macvlan:
    master:
    - ens192
    ippools:
      ipv4:
      - 10-6-v4
    vlanID: 0
EOF
```

> Set `spec.macvlan.master` to `ens192` which must be present on the host. The subnet specified in `spec.macvlan.spiderpoolConfigPools.IPv4IPPool` should match that of `ens192`。

Check if the Multus NAD has been created successfully:

```shell
~# kubectl  get network-attachment-definitions.k8s.cni.cncf.io  macvlan-ens192 -o yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"spiderpool.spidernet.io/v2beta1","kind":"SpiderMultusConfig","metadata":{"annotations":{},"name":"macvlan-ens192","namespace":"default"},"spec":{"cniType":"macvlan","coordinator":{"podCIDRType":"cluster","tuneMode":"overlay"},"enableCoordinator":true,"macvlan":{"master":["ens192"],"spiderpoolConfigPools":{"IPv4IPPool":["10-6-v4"]},"vlanID":0}}}
  creationTimestamp: "2023-06-30T07:12:21Z"
  generation: 1
  name: macvlan-ens192
  namespace: default
  ownerReferences:
  - apiVersion: spiderpool.spidernet.io/v2beta1
    blockOwnerDeletion: true
    controller: true
    kind: SpiderMultusConfig
    name: macvlan-ens192
    uid: 3f902f46-d9d4-4c62-a7c3-98d4a9aa26e4
  resourceVersion: "24713635"
  uid: 712d1e58-ab57-49a7-9189-0fffc64aa9c3
spec:
  config: '{"cniVersion":"0.3.1","name":"macvlan-ens192","plugins":[{"type":"macvlan","ipam":{"type":"spiderpool","default_ipv4_ippool":["10-6-v4"]},"master":"ens192","mode":"bridge"},{"type":"coordinattor","ipam":{},"dns":{},"detectGateway":false,"tunePodRoutes":true,"mode":"overlay","hostRuleTable":500,"detectIPConflict":false}]}'
```

### Create an application

Run the following command to create the demo application nginx:

```shell
~# cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      annotations:
        k8s.v1.cni.cncf.io/networks: macvlan-ens192
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx
        imagePullPolicy: IfNotPresent
        ports:
        - name: http
          containerPort: 80
          protocol: TCP
EOF
```

- `k8s.v1.cni.cncf.io/networks`: specifies that Multus uses `macvlan-ens192` to attach an additional interface to the Pod.

Check the Pod's IP allocation after it is ready:

```shell
~#  kubectl get po -l app=nginx -o wide
NAME                     READY   STATUS    RESTARTS   AGE   IP                NODE        NOMINATED NODE   READINESS GATES
nginx-4653bc4f24-aswpm   1/1     Running   0          2m    10.233.105.167    controller  <none>           <none>
nginx-4653bc4f24-rswak   1/1     Running   0          2m    10.233.73.210     worker01    <none>           <none>
```

```shell
~# kubectl get se
NAME                     INTERFACE   IPV4POOL            IPV4               IPV6POOL   IPV6   NODE
nginx-4653bc4f24-rswak   net1        10-6-v4             10.6.212.145/16                      worker01
nginx-4653bc4f24-aswpm   net1        10-6-v4             10.6.212.148/16                      controller
```

Enter the Pod and use the command `ip` to view information such as IP addresses and routes within the Pod:

```shell
[root@controller1 ~]# kubectl exec it nginx-4653bc4f24-rswak sh
# ip a
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host
       valid_lft forever preferred_lft forever
2: tunl0@NONE: <NOARP> mtu 1480 qdisc noop state DOWN group default qlen 1000
    link/ipip 0.0.0.0 brd 0.0.0.0
4: eth0@if3: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1430 qdisc noqueue state UP group default
    link/ether a2:99:9d:04:01:80 brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 10.233.73.210/32 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fd85:ee78:d8a6:8607::1:eb84/128 scope global
       valid_lft forever preferred_lft forever
    inet6 fe80::a099:9dff:fe04:180/64 scope link
       valid_lft forever preferred_lft forever
5: net1@if2: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether 2a:1e:a1:db:2a:9a brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 10.6.212.145/16 brd 10.6.255.255 scope global net1
       valid_lft forever preferred_lft forever
    inet6 fd00:10:6::2e5/64 scope global
       valid_lft forever preferred_lft forever
    inet6 fe80::281e:a1ff:fedb:2a9a/64 scope link
       valid_lft forever preferred_lft forever
/# ip rule
0: from all lookup local
32760: from 10.233.73.210 lookup 100
32762: from all to 169.254.1.1 lookup 100
32763: from all to 10.233.64.0/18 lookup 100
32764: from all to 10.233.0.0/18 lookup 100
32765: from all to 10.6.212.132 lookup 100
32766: from all lookup main
32767: from all lookup default
/# ip route
default via 10.6.0.1 dev net1
10.6.0.0/16 dev net1 scope link  src 10.6.212.145
/ # ip route show table 100
default via 169.254.1.1 dev eth0
10.6.212.132 dev eth0 scope link
10.233.0.0/18 via 10.6.212.132 dev eth0 
10.233.64.0/18 via 10.6.212.132 dev eth0
169.254.1.1 dev eth0 scope link
```

Explanation of the above:

> The Pod is allocated two interfaces: eth0 (Calico) and net1 (Macvlan), having IPv4 addresses of 10.233.73.210 and 10.6.212.145, respectively.
> 10.233.0.0/18 and 10.233.64.0/18 represent the cluster's CIDR. When the Pod accesses this subnet, traffic will be forwarded through eth0. Each route table will include this route.
> 10.6.212.132 is the IP address of the node where the Pod has been scheduled. This route ensures that when the Pod accesses the host, it will be forwarded through eth0.
> This series of routing rules guarantees that the Pod will forward traffic through eth0 when accessing targets within the cluster and through net1 for external targets.
> By default, the Pod's default route is reserved in net1. To reserve it in eth0, add the following annotation to the Pod's metadata: "ipam.spidernet.io/default-route-nic: eth0".

To test the basic network connectivity of the Pod, we will use the example of accessing the CoreDNS Pod and Service:

```shell
~# kubectl  get all -n kube-system -l k8s-app=kube-dns -o wide
NAME                           READY   STATUS    RESTARTS      AGE   IP               NODE          NOMINATED NODE   READINESS GATES
pod/coredns-57fbf68cf6-2z65h   1/1     Running   1 (91d ago)   91d   10.233.105.131   worker1       <none>           <none>
pod/coredns-57fbf68cf6-kvcwl   1/1     Running   3 (91d ago)   91d   10.233.73.195    controller    <none>           <none>

NAME              TYPE        CLUSTER-IP   EXTERNAL-IP   PORT(S)                  AGE   SELECTOR
service/coredns   ClusterIP   10.233.0.3   <none>        53/UDP,53/TCP,9153/TCP   91d   k8s-app=kube-dns

~# Access the CoreDNS Pod across nodes
~# kubectl  exec nginx-4653bc4f24-rswak -- ping 10.233.73.195 -c 2
PING 10.233.73.195 (10.233.73.195): 56 data bytes
64 bytes from 10.233.73.195: seq=0 ttl=62 time=2.348 ms
64 bytes from 10.233.73.195: seq=1 ttl=62 time=0.586 ms

--- 10.233.73.195 ping statistics ---
2 packets transmitted, 2 packets received, 0% packet loss
round-trip min/avg/max = 0.586/1.467/2.348 ms

~# Access the CoreDNS Service
~# kubectl exec  nginx-4653bc4f24-rswak -- curl 10.233.0.3:53 -I
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
  0     0    0     0    0     0      0      0 --:--:--  0:00:02 --:--:--     0
curl: (52) Empty reply from server
```

Test the Pod's connectivity for north-south traffic, specifically accessing targets in another subnet (10.7.212.101):

```shell
[root@controller1 cyclinder]# kubectl exec nginx-4653bc4f24-rswak -- ping 10.7.212.101 -c 2
PING 10.7.212.101 (10.7.212.101): 56 data bytes
64 bytes from 10.7.212.101: seq=0 ttl=61 time=4.349 ms
64 bytes from 10.7.212.101: seq=1 ttl=61 time=0.877 ms

--- 10.7.212.101 ping statistics ---
2 packets transmitted, 2 packets received, 0% packet loss
round-trip min/avg/max = 0.877/2.613/4.349 ms
```
