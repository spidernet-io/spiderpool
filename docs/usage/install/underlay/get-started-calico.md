# Calico Quick Start

**English** | [**简体中文**](./get-started-calico-zh_CN.md)

Spiderpool is able to provide static IPs to Deployments, StatefulSets, and other types of applications in underlay networks. In this page, we'll introduce how to build a complete Underlay network environment in Calico + BGP mode, and integrate it with Spiderpool to enable fixed IP addresses for applications. This solution meets the following requirements:

* Assign static IP addresses to applications

* Scale IP pools dynamically based on replica counts

* Enable external clients outside the cluster to access applications without their IPs

## Prerequisites

* An available **_Kubernetes_** cluster with a recommended version higher than 1.22, where **_Calico_** is installed as the default CNI.

    Make sure that Calico is not configured to use IPIP or VXLAN tunneling as we'll demonstrate how to use Calico for underlay networks.

    Confirm that Calico has enabled BGP configuration in full-mesh mode.

* SpiderPool's subnet feature needs to be enabled.

* Helm and Calicoctl

## Install Spiderpool

```shell
helm repo add spiderpool https://spidernet-io.github.io/spiderpool
helm install spiderpool spiderpool/spiderpool --namespace kube-system --set ipam.enableSpiderSubnet=true --set multus.multusCNI.install=false
```

> Spiderpool is IPv4-Only by default. If you want to enable IPv6, refer to [Spiderpool IPv6](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/ipv6.md).
>
> `ipam.enableSpiderSubnet=true`: SpiderPool's subnet feature needs to be enabled.
> 
> If you are mainland user who is not available to access ghcr.io，You can specify the parameter `-set global.imageRegistryOverride=ghcr.m.daocloud.io` to avoid image pulling failures for Spiderpool.

Create a subnet for a Pod (SpiderSubnet):

```shell
cat << EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderSubnet
metadata:
  name: nginx-subnet-v4
  labels:  
    ipam.spidernet.io/subnet-cidr: 10-244-0-0-16
spec:
  ips:
  - 10.244.100.0-10.244.200.1
  subnet: 10.244.0.0/16
EOF
```

Verify the installation：

```shell
[root@master ~]# kubectl get po -n kube-system  | grep spiderpool
spiderpool-agent-27fr2                     1/1     Running     0          2m
spiderpool-agent-8vwxj                     1/1     Running     0          2m
spiderpool-controller-bc8d67b5f-xwsql      1/1     Running     0          2m

[root@master ~]# kubectl get ss
NAME              VERSION   SUBNET          ALLOCATED-IP-COUNT   TOTAL-IP-COUNT
nginx-subnet-v4   4         10.244.0.0/16   0                    25602
```

## Configure Calico BGP [optional]

In this example, we want Calico to work in underlay mode and announce the Spiderpool subnet (`10.244.0.0/16`) to the BGP router via the BGP protocol, ensuring that clients outside the cluster can directly access the real IP addresses of the Pods through BGP router.

> If you don't need external clients to access pod IPs directly, skip this step.

The network topology is as follows:

![calico-bgp](../../../images/calico-bgp.svg)

1. Configure a host outside the cluster as BGP Router

    We will use an Ubuntu server as BGP Router. FRR needs to be installed beforehand:

    ```shell
    root@router:~# apt install -y frr
    ```

    FRR enable BGP:

    ```shell
    root@router:~# sed -i 's/bgpd=no/bgpd=yes/' /etc/frr/daemons
    root@router:~# systemctl restart frr
    ```

    Configure FRR:

    ```shell
    root@router:~# vtysh
    router# config
    router(config)# router bgp 23000 
    router(config)# bgp router-id 172.16.13.1 
    router(config)# neighbor 172.16.13.11 remote-as 64512 
    router(config)# neighbor 172.16.13.21 remote-as 64512  
    router(config)# no bgp ebgp-requires-policy 
    ```

    > Configuration descriptions:
    >
    > * The AS on the router side is `23000`, and the AS on the cluster node side is `64512`. The BGP neighbor relationship between the router and the node is ebgp, while the relationship between the nodes is ibgp.
    > * `ebgp-requires-policy` needs to be disabled, otherwise the BGP session cannot be established.
    > * 172.16.13.11/21 is the IP address of the cluster node.
    >
    > For more information, refer to [frrouting](https://docs.frrouting.org/en/latest/bgp.html).

2. Configure BGP neighbor for Calico

    `calico_backend: bird` needs to be configured to establish a BGP session:

    ```shell
    [root@master1 ~]# kubectl get cm -n kube-system calico-config -o yaml
    apiVersion: v1
    data:
      calico_backend: bird
      cluster_type: kubespray,bgp
    kind: ConfigMap
    metadata:
      annotations:
        kubectl.kubernetes.io/last-applied-configuration: |
          {"apiVersion":"v1","data":{"calico_backend":"bird","cluster_type":"kubespray,bgp"},"kind":"ConfigMap","metadata":{"annotations":{},"name":"calico-config","namespace":"kube-system"}}
    creationTimestamp: "2023-02-26T15:16:35Z"
    name: calico-config
    namespace: kube-system
    resourceVersion: "2056"
    uid: 001bbd09-9e6f-42c6-9339-39f71f81d363
    ```

    In this example, the default route for the node is on BGP router. As a result, nodes simply need to synchronize their local routes with BGP Router without synchronizing them with each other. Consequently, _Calico BGP Full-Mesh_ needs to be disabled:

    ```shell
    [root@master1 ~]# calicoctl patch bgpconfiguration default -p '{"spec": {"nodeToNodeMeshEnabled": false}}'
    ```

    Create BGPPeer:

    ```shell
    [root@master1 ~]# cat << EOF | calicoctl apply -f -
    apiVersion: projectcalico.org/v3
    kind: BGPPeer
    metadata:
      name: my-global-peer
    spec:
      peerIP: 172.16.13.1
      asNumber: 23000
    EOF
    ```

    > peerIP is the IP address of BGP Router
    >
    > asNumber is the AS number of BGP Router

    Check if the BGP session is established:

    ```shell
    [root@master1 ~]# calicoctl node status
    Calico process is running.
     
    IPv4 BGP status
    +--------------+-----------+-------+------------+-------------+
    | PEER ADDRESS | PEER TYPE | STATE |   SINCE    |    INFO     |
    +--------------+-----------+-------+------------+-------------+
    | 172.16.13.1  | global    | up    | 2023-03-15 | Established |
    +--------------+-----------+-------+------------+-------------+
     
    IPv6 BGP status
    No IPv6 peers found.
    ```

    For more information on Calico BGP configuration, refer to [Calico BGP](https://docs.tigera.io/calico/3.25/networking/configuring/bgp).

## Create a Calico IP pool in the same subnet

Create a Calico IP pool with the same CIDR as the Spiderpool subnet, otherwise Calico won't advertise the route of the Spiderpool subnet:

```shell
cat << EOF | calicoctl apply -f -
apiVersion: projectcalico.org/v3
kind: IPPool
metadata:
  name: spiderpool-subnet
spec:
  blockSize: 26
  cidr: 10.244.0.0/16
  ipipMode: Never
  natOutgoing: false
  nodeSelector: all()
  vxlanMode: Never
EOF
```

> The CIDR needs to correspond to the subnet of Spiderpool: `10.244.0.0/16`
>
> Set ipipMode and vxlanMode to: Never

## Switch Calico's `IPAM` to Spiderpool

Change the Calico CNI configuration file  `/etc/cni/net.d/10-calico.conflist` on each node to switch the ipam field to Spiderpool:

```json
"ipam": {
    "type": "spiderpool"
},
```

## Create applications

Take the Nginx application as an example:

```shell
cat <<EOF | kubectl create -f -
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
        ipam.spidernet.io/subnet: '{"ipv4":["nginx-subnet-v4"]}'
        ipam.spidernet.io/ippool-ip-number: '+3'
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

> `ipam.spidernet.io/subnet`: Assign static IPs from "nginx-subnet-v4" SpiderSubnet
>
> `ipam.spidernet.io/ippool-ip-number`: '+3' means the number of static IPs allocated to the application is three more than the number of its replicas, guaranteeing available temporary IPs during application rolling updates.

When a Pod is created, Spiderpool automatically creates an IP pool named `auto-nginx-v4-eth0-452e737e5e12` from `subnet: nginx-subnet-v4` specified in the annotations and binds it to the Pod. The IP range is `10.244.100.90-10.244.100.95`, and the number of IPs in the pool is `5`:

```shell
[root@master1 ~]# kubectl get sp
NAME                              VERSION   SUBNET          ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
auto-nginx-v4-eth0-452e737e5e12   4         10.244.0.0/16   2                    5                false     false

[root@master ~]# kubectl get sp auto-nginx-v4-eth0-452e737e5e12 -o jsonpath='{.spec.ips}' 
["10.244.100.90-10.244.100.95"]
```

When the replicas restart, its IP is fixed within the IP pool range of `auto-nginx-v4-eth0-452e737e5e12`:

```shell
[root@master1 ~]# kubectl get po -o wide
NAME                     READY   STATUS        RESTARTS   AGE     IP              NODE      NOMINATED NODE   READINESS GATES
nginx-644659db67-szgcg   1/1     Running       0          23s     10.244.100.90    worker5   <none>           <none>
nginx-644659db67-98rcg   1/1     Running       0          23s     10.244.100.92    master1   <none>           <none>
```

When the replica count increases to `3`, the IP address of the new replica is still assigned from the automatic pool: `auto-nginx-v4-eth0-452e737e5e12(10.244.100.90-10.244.100.95)`

```shell
[root@master1 ~]# kubectl scale deploy nginx --replicas 3  # scale pods
deployment.apps/nginx scaled

[root@master1 ~]# kubectl get po -o wide
NAME                     READY   STATUS        RESTARTS   AGE     IP              NODE      NOMINATED NODE   READINESS GATES
nginx-644659db67-szgcg   1/1     Running       0          1m     10.244.100.90    worker5   <none>           <none>
nginx-644659db67-98rcg   1/1     Running       0          1m     10.244.100.92    master1   <none>           <none>
nginx-644659db67-brqdg   1/1     Running       0          10s    10.244.100.94    master1   <none>           <none>
```

Check if both `ALLOCATED-IP-COUNT` and `TOTAL-IP-COUNT` of the automatic pool `auto-nginx-v4-eth0-452e737e5e12` increase by 1:

```shell
[root@master1 ~]# kubectl get sp
NAME                              VERSION   SUBNET          ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
auto-nginx-v4-eth0-452e737e5e12   4         10.244.0.0/16   3                    6                false     false
```

## Conclusion

The test result shows that clients outside the cluster can access Nginx Pods directly via their IP addresses. Nginx Pods can also communicate across cluster nodes, including Calico subnets. In Calico BGP mode, Spiderpool can be integrated with Calico to satisfy the fixed IP requirements for Deployments and other types of applications.
