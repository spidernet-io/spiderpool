# Egress Policy

**English** | [**简体中文**](./egress-zh_CN.md)

## Introduction

Spiderpool is an Underlay networking solution for Kubernetes, but the egress IP address is not fixed in a Kubernetes cluster when a Pod accesses an external service. In an Overlay network, the egress IP address is the address of the node on which the Pod resides, whereas in an Underlay network, the Pod communicates directly with the outside world using its own IP address. Therefore, when a Pod undergoes new scheduling, the IP address of the Pod when communicating with the outside world will change regardless of the network mode. This instability creates IP address management challenges for system maintainers. Especially when the cluster scale increases and network troubleshooting is required, it is difficult to control the egress traffic based on the Pod's original egress IP outside the cluster. Spiderpool can be used with the component [EgressGateway](https://github.com/spidernet-io/egressgateway) to solve the problem of Pod egress traffic management in Underlay network.

## Features of EgressGateway

EgressGateway is an open source Egress gateway designed to solve the problem of exporting Egress IP addresses in different CNI network modes (Spiderpool, Calico, Flannel, Weave). By flexibly configuring and managing egress policies, Egress IP is set for tenant-level or cluster-level workloads, so that when a Pod accesses an external network, the system will uniformly use this set Egress IP as the egress address, thus providing a stable egress traffic management solution. However, all EgressGateway rules are effective on the host's network namespace. To make the EgressGateway policy effective, the traffic of Pods accessing the outside of the cluster has to go through the host's network namespace. Therefore, you can configure the subnet routes forwarded from the host via the `spec.hijackCIDR` field of `spidercoordinators` in Spiderpool, and then configure the subnet routes forwarded from the host via [coordinator](../concepts/coordinator.md) to forward matching traffic from the veth pair to the host. This enables egress traffic management on an underlay network by allowing access to external traffic to be matched by EgressGateway rules.

Some of the features and benefits of Spiderpool with EgressGateway are as follows:

* Solve IPv4 IPv6 dual-stack connectivity, ensuring seamless communication across different protocol stacks.
* Solve the high availability of Egress Nodes, ensuring network connectivity remains unaffected by single-point failures.
* Support finer-grained policy control, allowing flexible filtering of Pods' Egress policies, including Destination CIDR.
* Support application-level control, allowing EgressGateway to filter Egress applications (Pods) for precise management of specific application outbound traffic.
* Support multiple egress gateways instance, capable of handling communication between multiple network partitions or clusters.
* Support namespaced egress IP.
* Support automatic detection of cluster traffic for egress gateways policies.
* Support namespace default egress instances.
* Can be used in low kernel version, making EgressGateway suitable for various Kubernetes deployment environments.

## Prerequisites

1. A ready-to-use Kubernetes.

2. [Helm](https://helm.sh/docs/intro/install/) has been already installed.

## Steps

### Install Spiderpool

Refer to [Installation](./readme.md) to install Spiderpool and create SpiderMultusConfig CR and IPPool CR.

After installing Spiderpool, Add the service addresses outside the cluster to the 'hijackCIDR' field in the 'default' object of spiderpool.spidercoordinators. This ensures that when Pods access these external services, the traffic is routed through the host where the Pod is located, allowing the EgressGateway rules to match.

```bash
# For running Pods, you need to restart them for these routing rules to take effect within the Pods.
~# kubectl patch spidercoordinators default  --type='merge' -p '{"spec": {"hijackCIDR": ["10.6.168.63/32"]}}'
```

### Install EgressGateway

Install EgressGateway via helm:

```shell
helm repo add egressgateway https://spidernet-io.github.io/egressgateway/
helm repo update egressgateway
helm install egressgateway egressgateway/egressgateway -n kube-system --set feature.tunnelIpv4Subnet="192.200.0.1/16" --set feature.enableGatewayReplyRoute=true  --wait --debug
```

> If IPv6 is required, enable it with the option `-set feature.enableIPv6=true` and set `feature.tunnelIpv6Subnet`, it is worth noting that when configuring IPv4 or IPv6 segments via `feature.tunnelIpv4Subnet` and `feature. tunnelIpv6Subnet`, it is worth noting that when configuring IPv4 or IPv6 segments via `feature.tunnelIpv4Subnet` and `feature.tunnelIpv6Subnet`, you need to make sure that the segments don't conflict with any other addresses in the cluster.
> `feature.enableGatewayReplyRoute` is true to enable return routing rules on gateway nodes, which must be enabled when pairing with Spiderpool to support underlay CNI.
>
> If you are a mainland user who is not available to access ghcr.io, you can specify the parameter `-set global.imageRegistryOverride=ghcr.m.daocloud.io` to avoid image pulling failures for EgressGateway.

Verify your EgressGateway installation:

```bash
~# kubectl get pod -n kube-system | grep egressgateway
egressgateway-agent-4s8lt                   1/1     Running     0     29m
egressgateway-agent-thzth                   1/1     Running     0     29m
egressgateway-controller-77698899df-tln7j   1/1     Running     0     29m
```

For more installation details, refer to [EgressGateway Installation](https://github.com/spidernet-io/egressgateway/blob/main/docs/usage/Install.en.md).

### Create an instance of EgressGateway

An EgressGateway defines a set of nodes that act as an egress gateway for the cluster, through which egress traffic within the cluster will be forwarded out of the cluster. Therefore, an EgressGateway instance needs to be pre-defined. The following example Yaml creates an EgressGateway instance.

* `spec.ippools.ipv4`: defines a set of egress IP addresses, which need to be adjusted according to the actual situation of the specific environment. The CIDR of `spec.ippools.ipv4` should be the same as the subnet of the egress NIC on the gateway node (usually the NIC of the default route), or else the egress access may not work.

* `spec.nodeSelector`: the node affinity method provided by EgressGateway, when `selector.matchLabels` matches with a node, the node will be used as the egress gateway for the cluster, when `selector.matchLabels` does not match with a node, the When `selector.matchLabels` does not match with a node, the EgressGateway skips that node and it will not be used as an egress gateway for the cluster, which supports selecting multiple nodes for high availability.

```bash
cat <<EOF | kubectl apply -f -
apiVersion: egressgateway.spidernet.io/v1beta1
kind: EgressGateway
metadata:
  name: default
spec:
  ippools:
    ipv4:
    - "10.6.168.201-10.6.168.205"
  nodeSelector:
    selector:
      matchLabels:
        egressgateway: "true"
EOF
```

Label the node with the Label specified in `nodeSelector.selector.matchLabels` above so that the node can be selected by the EgressGateway to act as an egress gateway.

```bash
~# kubectl get node
NAME                STATUS   ROLES           AGE     VERSION
controller-node-1   Ready    control-plane   5d17h   v1.26.7
worker-node-1       Ready    <none>          5d17h   v1.26.7

~# kubectl label node worker-node-1 egressgateway="true"
```

When the creation is complete, check the EgressGateway status.

* `spec.ippools.ipv4DefaultEIP` represents the default VIP of the EgressGateway for the group, which is an IP address that will be randomly selected from `spec.ippools.ipv4`, and its function is: when creating an EgressPolicy object for an application, if no VIP address is specified, the is used if no VIP address is specified when creating an EgressPolicy object for the application.

* `status.nodeList` represents the status of the nodes identified as matching the `spec.nodeSelector` and the corresponding EgressTunnel object for that node.

```shell
~# kubectl get EgressGateway default -o yaml
...
spec:
  ippools:
    ipv4DefaultEIP: 10.6.168.201
...
status:
  nodeList:
  - name: worker-node-1
    status: Ready
```

### Create Applications and Egress Policies

Create an application that will be used to test Pod access for external cluster purposes and label it to be associated with the EgressPolicy, as shown in the following example Yaml.

* `v1.multus-cni.io/default-network`: used to specify the subnet used by the application, the Multus CR corresponding to this value needs to be created in advance by referring to the [installation](./readme.md) document to create it in advance.

* `ipam.spidernet.io/ippool`: Specify which SpiderIPPool resources are used by the Pod, the corresponding SpiderIPPool CR should be created in advance by referring to the [Installation](./readme.md).

```bash
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: test-app
  name: test-app
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
      annotations:
        ipam.spidernet.io/ippool: |-
            {      
              "ipv4": ["v4-pool"],
            }
        v1.multus-cni.io/default-network: kube-system/macvlan-conf
    spec:
      containers:
      - image: nginx
        imagePullPolicy: IfNotPresent
        name: test-app
        ports:
        - name: http
          containerPort: 80
          protocol: TCP
EOF
```

The EgressPolicy instance is used to define which Pods' egress traffic is to be forwarded through the EgressGateway node, as well as other configuration details. The following is an example of creating an EgressPolicy CR object for an application.

* `spec.egressGatewayName` is used to specify which set of EgressGateways to use.

* `spec.appliedTo.podSelector` is used to specify on which Pods within the cluster this policy takes effect.

* `namespace` is used to specify the tenant where the EgressPolicy object resides. Because EgressPolicy is tenant-level, it must be created under the same namespace as the associated application, so that when the matching Pod accesses any address outside the cluster, it can be forwarded by the EgressGateway Node.

```bash
cat <<EOF | kubectl apply -f -
apiVersion: egressgateway.spidernet.io/v1beta1
kind: EgressPolicy
metadata:
 name: test
 namespace: default
spec:
 egressGatewayName: default
 appliedTo:
  podSelector:
   matchLabels:
    app: "test-app"
EOF
```

When creation is complete, check the status of the EgressPolicy.

* `status.eip` shows the egress IP address used by the group when applying out of the cluster.

* `status.node` shows which EgressGateway node is responsible for forwarding traffic out of the EgressPolicy.

```bash
~# kubectl get EgressPolicy -A
NAMESPACE   NAME   GATEWAY   IPV4           IPV6   EGRESSNODE
default     test   default   10.6.168.201          worker-node-1
 
~# kubectl get EgressPolicy test -o yaml
apiVersion: egressgateway.spidernet.io/v1beta1
kind: EgressPolicy
metadata:
  name: test
  namespace: default
spec:
  appliedTo:
    podSelector:
      matchLabels:
        app: test-app
  egressIP:
    allocatorPolicy: default
    useNodeIP: false
status:
  eip:
    ipv4: 10.6.168.201
  node: worker-node-1
```

After creating the EgressPolicy object, an EgressEndpointSlices object containing a collection of IP addresses of all the applications will be generated according to the application selected by the EgressPolicy, so that you can check whether the IP addresses in the EgressEndpointSlices object are normal or not when the application cannot be accessed by export.

```bash
~# kubectl get egressendpointslices -A
NAMESPACE   NAME         AGE
default     test-4vbqf   41s

~# kubectl get egressendpointslices test-kvlp6 -o yaml
apiVersion: egressgateway.spidernet.io/v1beta1
endpoints:
- ipv4:
  - 10.6.168.208
  node: worker-node-1
  ns: default
  pod: test-app-f44846544-8dnzp
kind: EgressEndpointSlice
metadata:
  name: test-4vbqf
  namespace: default
```

### Test Results

Deploy the application `nettools` outside the cluster to emulate a service outside the cluster, and `nettools` will return the source IP address of the requester in the http reply.

```bash
~# docker run -d --net=host ghcr.io/spidernet-io/egressgateway-nettools:latest /usr/bin/nettools-server -protocol web -webPort 8080
```

To verify the effect of egress traffic in a test app within the cluster: test-app, you can see that the source IP returned by `nettools` complies with `EgressPolicy.status.eip` when accessing an external service in the Pod corresponding to this app.

```bash
~# kubectl get pod -owide
NAME                       READY   STATUS    RESTARTS      AGE     IP              NODE                NOMINATED NODE   READINESS GATES
test-app-f44846544-8dnzp   1/1     Running   0             4m27s   10.6.168.208    worker-node-1       <none>           <none>

~# kubectl exec -it test-app-f44846544-8dnzp bash

~# curl 10.6.168.63:8080 # IP address of the node outside the cluster + webPort
Remote IP: 10.6.168.201
```
