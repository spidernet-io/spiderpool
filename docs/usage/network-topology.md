# Application across network zones

**English** | [**简体中文**](./network-topology-zh_CN.md)

## Introduce

The rising popularity of private cloud data centers has made underlay networks  essential components of data center network architecture by offering efficient network transmission and improved network topology management capabilities. Underlay is widely used in the following scenarios due to its low latency, reliability, and security:

- Latency-sensitive applications: applications in specific industries, such as financial trading and real-time video transmission, are highly sensitive to network latency. Underlay networks directly control physical and link-layer connections to reduce data transmission time, providing an ideal solution for these applications.

- Firewall security control and management: firewalls are often used to manage north-south traffic,  namely communication between internal and external networks, by checking, filtering, and restricting communication traffic. IP address management (IPAM) solutions of underlay networks that allocate fixed egress IP addresses for applications can provide better communication management and control between the cluster and external networks, further enhancing overall network security.

In the cluster of the Underlay network, when its nodes are distributed in different regions or data centers, and some node regions can only use specific subnets, it will bring challenges to IP address management (IPAM). This article will introduce a method that can realize A complete underlay network solution for IP allocation across network regions.

![network-topology](../images/across-networks-1.png)

## Project Functions

Spiderpool provides the function of node topology, which can help solve the problem of IP allocation across network regions. Its implementation principle is as follows.

- A cluster, but the nodes of the cluster are distributed in different regions or data centers, some nodes' regions can only use the subnet 10.6.1.0/24, and some nodes' regions can only use the subnet 172.16.2.0/24.

In the appeal scenario, when an application deploys copies across subnets, IPAM is required to assign IP addresses that match the subnet to different Pods under the same application on different nodes. Spiderpool's CR: `SpiderIPPool` The nodeName field is provided to realize the affinity between the IP pool and the node, so that when the Pod is scheduled to a certain node, it can obtain the IP address from the Underlay subnet where the node is located, and realize the node topology function.

## Implementation Requirements

1. Installed [Helm](https://helm.sh/docs/intro/install/).

## Steps

### Clusters spanning network regions

Prepare a set of clusters that span the network area. For example, node 1 uses `10.6.0.0/16` and node 2 uses `10.7.0.0/16` subnet. The following is the cluster information and network topology used:

```bash
~# kubectl get nodes -owide
NAME                STATUS   ROLES           AGE  VERSION   INTERNAL-IP   EXTERNAL-IP
controller-node-1   Ready    control-plane   1h   v1.25.3   10.6.168.71   <none>
worker-node-1       Ready    <none>          1h   v1.25.3   10.7.168.73   <none>        

~# kubectl get nodes --show-labels
NAME                STATUS   ROLES                  AGE  VERSION   LABELS
controller-node-1   Ready    control-plane,master   1h   v1.25.3   node-subnet=subnet-6, ...
worker-node-1       Ready    <none>                 1h   v1.25.3   node-subnet=subnet-7, ...
```

![network-topology](../images/across-networks-2.png)

### Install Spiderpool

Install Spiderpool via helm.

```bash
helm repo add spiderpool https://spidernet-io.github.io/spiderpool

helm repo update spiderpool

helm install spiderpool spiderpool/spiderpool --namespace kube-system --set multus.multusCNI.defaultCniCRName="macvlan-conf"
```

> If you are using a cloud server from a Chinese mainland cloud provider, you can enhance image pulling speed by specifying the parameter `--set global.imageRegistryOverride=ghcr.m.daocloud.io`.
>
> If Macvlan CNI is not installed in your cluster, you can install it on each node by using the Helm parameter `--set plugins.installCNI=true`.
>
> Specify the name of the NetworkAttachmentDefinition instance for the default CNI used by Multus via `multus.multusCNI.defaultCniCRName`. If the `multus.multusCNI.defaultCniCRName` option is provided, an empty NetworkAttachmentDefinition instance will be automatically generated upon installation. Otherwise, Multus will attempt to create a NetworkAttachmentDefinition instance based on the first CNI configuration found in the /etc/cni/net.d directory. If no suitable configuration is found, a NetworkAttachmentDefinition instance named `default` will be created to complete the installation of Multus.

Verify the installation：

```bash
~# kubectll get po -n kube-sysem | grep spiderpool
NAME                                     READY   STATUS      RESTARTS   AGE                                
spiderpool-agent-7hhkz                   1/1     Running     0          13m
spiderpool-agent-kxf27                   1/1     Running     0          13m
spiderpool-controller-76798dbb68-xnktr   1/1     Running     0          13m
spiderpool-init                          0/1     Completed   0          13m
```

### Install CNI configuration

To simplify writing Multus CNI configuration in JSON format, Spiderpool provides SpiderMultusConfig CR to automatically manage Multus NetworkAttachmentDefinition CR. Here is an example of creating an IPvlan SpiderMultusConfig configuration:

- master: In this example the interface `eth0` is used as the parameter for master, this parameter should match the interface name on the nodes where the cluster spans network zones.

```shell
MACVLAN_MASTER_INTERFACE="eth0"
MACVLAN_MULTUS_NAME="macvlan-conf"

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
```

In the example of this article, use the above configuration to create the following two Macvlan SpiderMultusConfig, which will be automatically generated based on the Multus NetworkAttachmentDefinition CR, which corresponds to the `eth0` network card of the host.

```bash
~# ~# kubectl get spidermultusconfigs.spiderpool.spidernet.io -n kube-system
NAME           AGE
macvlan-conf   10m

~# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system

NAME           AGE
macvlan-conf   10m
```

### Create IPPools

CRD of Spiderpool: SpiderIPPool provides `nodeName` field. When nodeName is not empty, when Pod starts on a node and tries to allocate IP from SpiderIPPool, if the node where Pod is located matches the nodeName setting, it can be retrieved from SpiderIPPool The IP is allocated successfully, otherwise the IP cannot be allocated from the SpiderIPPool. When nodeName is empty, Spiderpool does not enforce any allocation limit on Pods.

As above, using the following Yaml, create 2 SpiderIPPools that will provide IP addresses to Pods on different nodes.

```bash
~# cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: test-ippool-6
spec:
  subnet: 10.6.0.0/16
  ips:
    - 10.6.168.60-10.6.168.69
  gateway: 10.6.0.1
  nodeName:
  - controller-node-1
---
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: test-ippool-7
spec:
  subnet: 10.7.0.0/16
  ips:
    - 10.7.168.60-10.7.168.69
  gateway: 10.7.0.1
  nodeName:
  - worker-node-1
EOF
```

## Create the application

The following sample yaml creates a daemonSet application where:

- ipam.spidernet.io/ippool: It is used to specify the IP pool of Spiderpool. Multiple IP pools can be set as alternative pools. Spiderpool will try to allocate IP addresses in sequence according to the order of elements in the "IP pool array". When assigning IP in the network area scenario, if the node to which the application copy is scheduled meets the IPPool.spec.nodeAffinity annotation of the first IP pool, the Pod will obtain the IP allocation from the pool. Select the IP pool in the selected pool and continue to allocate IPs for Pods until all candidate pools fail to be screened. You can learn more about usage with alternative pools.

- v1.multus-cni.io/default-network: Used to specify the NetworkAttachmentDefinition configuration of Multus, which will create a default network card for the application.

```bash
~# cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: test-app
spec:
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      annotations:   
        ipam.spidernet.io/ippool: |-
          {
            "ipv4": ["test-ippool-6", "test-ippool-7"]
          }
        v1.multus-cni.io/default-network: kube-system/macvlan-conf
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

After creating an application, it can be observed that each Pod's IP address is assigned from an IP pool belonging to the same subnet as its node.

```bash
~# kubectl get po -l app=test-app -o wide
NAME             READY   STATUS    RESTARTS   AGE   IP            NODE                NOMINATED NODE   READINESS GATES
test-app-j9ftl   1/1     Running   0          45s   10.6.168.65   controller-node-1   <none>           <none>
test-app-nkq5h   1/1     Running   0          45s   10.7.168.61   worker-node-1       <none>           <none>

~# kubectl get spiderippool
NAME            VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
test-ippool-6   4         10.6.0.0/16   1                    10               false     false
test-ippool-7   4         10.7.0.0/16   1                    10               false     false
```

Communication between Pods across network zones:

```bash
~# kubectl exec -ti test-app-j9ftl -- ping 10.7.168.61 -c 2

PING 10.7.168.61 (10.7.168.61) 56(84) bytes of data.
64 bytes from 10.7.168.61: icmp_seq=1 ttl=63 time=1.06 ms
64 bytes from 10.7.168.61: icmp_seq=2 ttl=63 time=0.515 ms

--- 10.7.168.61 ping statistics ---
2 packets transmitted, 2 received, 0% packet loss, time 1002ms
rtt min/avg/max/mdev = 0.515/0.789/1.063/0.274 ms
```

## Summarize

Pods in different network areas can communicate normally, and Spiderpool can well meet the IP allocation requirements based on cross-network areas.
