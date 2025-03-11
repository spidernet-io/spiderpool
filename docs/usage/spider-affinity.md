# SpiderIPPool Affinity

**English** ｜ [**简体中文**](./spider-affinity-zh_CN.md)

## Introduction

SpiderIPPool is a representation of a collection of IP addresses. It allows storing different IP addresses from the same subnet in separate IPPool instances, ensuring that there is no overlap between address sets. This design provides flexibility in managing IP resources within the underlay network, especially when faced with limited availability. SpiderIPPool offers the ability to assign different SpiderIPPool instances to various applications and tenants through affinity rules, allowing for both shared subnet usage and micro-isolation.

## Quick Start

In [SpiderIPPool CRD](./../reference/crd-spiderippool.md), we defined lots of properties to use with affinities:

- `spec.podAffinity` controls whether the pool can be used by the Pod.
- `spec.namespaceName` and `spec.namespaceAffinity` verify if they match the Namespace of the Pod. If there is no match, the pool cannot be used. (`namespaceName` takes precedence over `namespaceAffinity`).
- `spec.nodeName` and `spec.nodeAffinity` verify if they match the node where the Pod is located. If there is no match, the pool cannot be used. (`nodeName` takes precedence over `nodeAffinity`).
- `multusName` determines whether the current network card which using the pool matches the CNI configuration used by the multus net-attach-def resource. If there is no match, the pool cannot be used.

These fields not only serve as **filters** but also have a **sorting effect**. The more matching fields there are, the higher priority the IP pool has for usage.

## Application Affinity

Firewalls are commonly used in clusters to manage communication between internal and external networks (north-south communication). To enforce secure access control, firewalls inspect and filter communication traffic while restricting outbound communication. In order to align with firewall policies and enable north-south communication within the underlay network, certain Deployments require all Pods to be assigned IP addresses within a specific range.

Existing community solutions rely on annotations to handle IP address allocation for such cases. However, this approach has limitations:

- Manual modification of annotations becomes necessary as the application scales, leading to potential errors.

- IP management through annotations is far apart from the IPPool CR mechanism, resulting in a lack of visibility into available IP addresses.

- Conflicting IP addresses can easily be assigned to different applications, causing deployment failures.

Spiderpool addresses these challenges by leveraging the flexibility of IPPools, where IP address collections can be adjusted. By combining this with the `podAffinity` setting in the SpiderIPPool CR, Spiderpool enables the binding of specific applications or groups of applications to particular IPPools. This ensures a unified approach to IP management, decouples application scaling from IP address scaling, and provides a fixed IP usage range for each application.

### Create IPPool with Application Affinity

SpiderIPPool provides the `podAffinity` field. When an application is created and attempts to allocate an IP address from the SpiderIPPool, it can successfully obtain an IP if the Pods' `selector.matchLabels` match the specified podAffinity. Otherwise, IP allocation from that SpiderIPPool will be denied.

Based on the above, using the following Yaml, create the following SpiderIPPool with application affinity, which will provide the IP address for the `app: test-app-3` Pod's eligible `selector.matchLabel`.

```bash
~# cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: test-pod-ippool
spec:
  subnet: 10.6.0.0/16
  ips:
    - 10.6.168.151-10.6.168.160
  podAffinity:
    matchLabels:
      app: test-app-3
EOF
```

Creating Applications with Specific matchLabels. In the example YAML provided, a set of Deployment applications is created.

```bash
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app-3
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app-3
  template:
    metadata:
      annotations:
        ipam.spidernet.io/ippool: |-
            {      
              "ipv4": ["test-pod-ippool"]
            }
        v1.multus-cni.io/default-network: kube-system/macvlan-ens192
      labels:
        app: test-app-3
    spec:
      containers:
      - name: test-app-3
        image: nginx
        imagePullPolicy: IfNotPresent
EOF
```

- `ipam.spidernet.io/ippool`: specify the IP pool with application affinity.
- `v1.multus-cni.io/default-network`: create a default network interface for the application.
- `matchLabels`: set the label for the application.

After creating the application, the Pods with `matchLabels` that match the IPPool's application affinity successfully obtain IP addresses from that SpiderIPPool. The assigned IP addresses remain within the IP pool.

```bash
～# kubectl get spiderippool
NAME                VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
test-pod-ippool     4         10.6.0.0/16   1                    10               false

~# kubectl get po -l app=test-app-3 -owide
NAME                          READY   STATUS    RESTARTS   AGE   IP             NODE    NOMINATED NODE   READINESS GATES
test-app-3-6994b9d5bb-qpf5p   1/1     Running   0          52s   10.6.168.154   node2   <none>           <none>
```

However, when creating another application with different `matchLabels` that do not meet the IPPool's application affinity, Spiderpool will reject IP address allocation.

```bash
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-unmatch-labels
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-unmatch-labels
  template:
    metadata:
      annotations:
        ipam.spidernet.io/ippool: |-
            {      
              "ipv4": ["test-pod-ippool"]
            }
        v1.multus-cni.io/default-network: kube-system/macvlan-ens192
      labels:
        app: test-unmatch-labels
    spec:
      containers:
      - name: test-unmatch-labels
        image: nginx
        imagePullPolicy: IfNotPresent
EOF
```

- `matchLabels`: set the label of the application to `test-unmatch-labels`, which does not match IPPool affinity.

Getting an IP address assignment fails as expected when the Pod's matchLabels do not match the application affinity for that IPPool.

```bash
kubectl get po -l app=test-unmatch-labels -owide
NAME                                  READY   STATUS              RESTARTS   AGE   IP       NODE    NOMINATED NODE   READINESS GATES
test-unmatch-labels-699755574-9ncp7   0/1     ContainerCreating   0          16s   <none>   node1   <none>           <none>
```

### Shared IPPool

1. Create an IPPool

    ```bash
    kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-affinity-pod/shared-static-ipv4-ippool.yaml
    ```

    ```yaml
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderIPPool
    metadata:
      name: shared-static-ipv4-ippool
    spec:
      subnet: 172.18.41.0/24
      ips:
        - 172.18.41.44-172.18.41.47
    ```

2. Create two Deployment  whose Pods are setting the Pod annotation `ipam.spidernet.io/ippool` to explicitly specify the pool selection rule. It will succeed to get IP address.

    ```bash
    kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/ippool-affinity-pod/shared-static-ippool-deploy.yaml
    ```

    ```yaml
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: shared-static-ippool-deploy-1
    spec:
      replicas: 2
      selector:
        matchLabels:
          app: static
      template:
        metadata:
          annotations:
            ipam.spidernet.io/ippool: |-
              {
                "ipv4": ["shared-static-ipv4-ippool"]
              }
          labels:
            app: static
        spec:
          containers:
            - name: shared-static-ippool-deploy-1
              image: busybox
              imagePullPolicy: IfNotPresent
              command: ["/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"]
    ---
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: shared-static-ippool-deploy-2
    spec:
      replicas: 2
      selector:
        matchLabels:
          app: static
      template:
        metadata:
          annotations:
            ipam.spidernet.io/ippool: |-
              {
                "ipv4": ["shared-static-ipv4-ippool"]
              }
          labels:
            app: static
        spec:
          containers:
            - name: shared-static-ippool-deploy-2
              image: busybox
              imagePullPolicy: IfNotPresent
              command: ["/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"]
    ```

    The Pods are running.

    ```bash
    kubectl get po -l app=static -o wide
    NAME                                             READY   STATUS    RESTARTS   AGE   IP             NODE              
    shared-static-ippool-deploy-1-8588c887cb-gcbjb   1/1     Running   0          62s   172.18.41.45   spider-control-plane 
    shared-static-ippool-deploy-1-8588c887cb-wfdvt   1/1     Running   0          62s   172.18.41.46   spider-control-plane 
    shared-static-ippool-deploy-2-797c8df6cf-6vllv   1/1     Running   0          62s   172.18.41.44   spider-worker 
    shared-static-ippool-deploy-2-797c8df6cf-ftk2d   1/1     Running   0          62s   172.18.41.47   spider-worker
    ```

## Node Affinity

Nodes in a cluster might have access to different IP ranges. Some scenarios include:

- Nodes in the same data center belonging to different subnets.

- Nodes spanning multiple data centers within a single cluster.

In such cases, replicas of an application are scheduled on different nodes require IP addresses from different subnets. Current community solutions are limited to satisfy this needs.
To address this problem, Spiderpool support node affinity solution. By setting the `nodeAffinity` and `nodeName` fields in the SpiderIPPool CR, administrators can define a node label selector. This enables the IPAM plugin to allocate IP addresses from the specified IPPool when Pods are scheduled on nodes that match the affinity rules.

### IPPool with Node Affinity

SpiderIPPool offers the `nodeAffinity` field. When a Pod is scheduled on a node and attempts to allocate an IP address from the SpiderIPPool, it can successfully obtain an IP if the node satisfies the specified nodeAffinity condition. Otherwise, it will be unable to allocate an IP address from that SpiderIPPool.

To create a SpiderIPPool with node affinity, use the following YAML configuration. This SpiderIPPool will provide IP addresses for Pods running on the designated node.

```bash
~# cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: test-node1-ippool
spec:
  subnet: 10.6.0.0/16
  ips:
    - 10.6.168.101-10.6.168.110
  nodeAffinity:
    matchExpressions:
    - {key: kubernetes.io/hostname, operator: In, values: [node1]}
EOF
```

SpiderIPPool provides an additional option for node affinity: `nodeName`. When `nodeName` is specified, a Pod is scheduled on a specific node and attempts to allocate an IP address from the SpiderIPPool. If the node matches the specified `nodeName`, the IP address can be successfully allocated from that SpiderIPPool. If not, it will be unable to allocate an IP address from that SpiderIPPool. When nodeName is left empty, Spiderpool does not impose any allocation restrictions on Pods. For example:

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: test-node1-ippool
spec:
  subnet: 10.6.0.0/16
  ips:
    - 10.6.168.101-10.6.168.110
  nodeName:
  - node1
```

Create a set of DaemonSet applications using the following example YAML:

```bash
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: test-app-1
  labels:
    app: test-app-1
spec:
  selector:
    matchLabels:
      app: test-app-1
  template:
    metadata:
      labels:
        app: test-app-1
      annotations:
        ipam.spidernet.io/ippool: |-
            {      
              "ipv4": ["test-node1-ippool"]
            }
        v1.multus-cni.io/default-network: kube-system/macvlan-ens192
    spec:
      containers:
      - name: test-app
        image: nginx
        imagePullPolicy: IfNotPresent
EOF
```

- `ipam.spidernet.io/ippool`: specify the IP pool with node affinity.

- `v1.multus-cni.io/default-network`:  Identifies the IP pool used by the application.

After creating an application, it can be observed that IP addresses are only allocated from the corresponding IPPool if the Pod's node matches the IPPool's node affinity. The IP address of the application remains within the assigned IPPool.

```bash
～# kubectl get spiderippool
NAME                VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
test-node1-ippool   4         10.6.0.0/16   1                    10               false

~# kubectl get po -l app=test-app-1 -owide
NAME               READY   STATUS              RESTARTS   AGE    IP             NODE     NOMINATED NODE   READINESS GATES
test-app-1-2cmnz   0/1     ContainerCreating   0          115s   <none>         node2    <none>           <none>
test-app-1-br5gw   0/1     ContainerCreating   0          115s   <none>         master   <none>           <none>
test-app-1-dvhrx   1/1     Running             0          115s   10.6.168.108   node1    <none>           <none>
```

## IPPool with Namespace Affinity

Cluster administrators often partition their clusters into multiple namespaces to improve isolation, management, collaboration, security, and resource utilization. When deploying applications under different namespaces, it becomes necessary to assign specific IPPools to each namespace, preventing applications from unrelated namespaces from using them.

Spiderpool addresses this requirement by introducing the `namespaceAffinity` or `namespaceName` fields in the SpiderIPPool CR. This allows administrators to define affinity rules between IPPools and one or more namespaces, ensuring that only applications meeting the specified conditions can be allocated IP addresses from the respective IPPools.

### Create IPPool with Namespace Affinity

```bash
~# kubectl create ns test-ns1
namespace/test-ns1 created
~# kubectl create ns test-ns2
namespace/test-ns2 created
```

To create an IPPool with namaspace affinity, use the following YAML:

SpiderIPPool provides the `namespaceAffinity` field. When an application is created and attempts to allocate an IP address from the SpiderIPPool, it will only succeed if the Pod's namespace matches the specified namespaceAffinity. Otherwise, IP allocation from that SpiderIPPool will be denied.

```bash
~# cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: test-ns1-ippool
spec:
  subnet: 10.6.0.0/16
  ips:
    - 10.6.168.111-10.6.168.120
  namespaceAffinity:
    matchLabels:
      kubernetes.io/metadata.name: test-ns1
EOF
```

SpiderIPPool also offers another option for namespace affinity: `namespaceName`. When `namespaceName` is not empty, a Pod is created and attempts to allocate an IP address from the SpiderIPPool. If the namespace of the Pod matches the specified `namespaceName`, it can successfully obtain an IP from that SpiderIPPool. However, if the namespace does not match the `namespaceName`, it will be unable to allocate an IP address from that SpiderIPPool. When `namespaceName` is empty, Spiderpool does not impose any restrictions on IP allocation for Pods. For example:

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: test-ns1-ippool
spec:
  subnet: 10.6.0.0/16
  ips:
    - 10.6.168.111-10.6.168.120
  namespaceName: 
    - test-ns1
```

Create Applications in a Specified Namespace. In the provided YAML example, a set of Deployment applications is created under the `test-ns1` namespace.

```bash
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app-2
  namespace: test-ns1
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app-2
  template:
    metadata:
      annotations:
        ipam.spidernet.io/ippool: |-
            {      
              "ipv4": ["test-ns1-ippool"]
            }
        v1.multus-cni.io/default-network: kube-system/macvlan-ens192
      labels:
        app: test-app-2
    spec:
      containers:
      - name: test-app-2
        image: nginx
        imagePullPolicy: IfNotPresent
EOF
```

- `ipam.spidernet.io/ippool`：specify the IP pool with tenant affinity.

- `v1.multus-cni.io/default-network`: create a default network interface for the application.

- `namespace`: the namespace where the application resides.

After creating the application, the Pods within the designated namespace successfully allocate IP addresses from the associated IPPool with namespace affinity.

```bash
~# kubectl get spiderippool
NAME              VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
test-ns1-ippool   4         10.6.0.0/16   1                    10               false

~# kubectl get  po -l app=test-app-2 -A  -o wide
NAMESPACE   NAME                      READY   STATUS    RESTARTS   AGE   IP             NODE    NOMINATED NODE   READINESS GATES
test-ns1    test-app-2-975d9f-6bww2   1/1     Running   0          44s   10.6.168.111   node2   <none>           <none>
```

However, if an application is created outside the `test-ns1` namespace, Spiderpool will reject IP address allocation, preventing unrelated namespace from using that IPPool.

```bash
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-other-ns
  namespace: test-ns2
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-other-ns
  template:
    metadata:
      annotations:
        ipam.spidernet.io/ippool: |-
            {      
              "ipv4": ["test-ns1-ippool"]
            }
        v1.multus-cni.io/default-network: kube-system/macvlan-ens192
      labels:
        app: test-other-ns
    spec:
      containers:
      - name: test-other-ns
        image: nginx
        imagePullPolicy: IfNotPresent
EOF
```

Getting an IP address assignment fails as expected when the Pod belongs to a namesapce that does not match the affinity of that IPPool.

```bash
~# kubectl get po -l app=test-other-ns -A -o wide
NAMESPACE     NAME                              READY   STATUS              RESTARTS   AGE   IP       NODE    NOMINATED NODE   READINESS GATES
test-ns2    test-other-ns-56cc9b7d95-hx4b5   0/1     ContainerCreating   0          6m3s   <none>   node2   <none>           <none>
```

## Multus affinity

When creating multiple network interfaces for an application, we can specify the affinity of multus net-attach-def instance for the **cluster-level default pool**. This way is simpler compared to explicitly specifying the binding relationship between network interfaces and IPPool resources through the `ipam.spidernet.io/ippools` annotation.

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: test-ippool-eth0
spec:
  default: true
  subnet: 10.6.0.0/16
  ips:
    - 10.6.168.151-10.6.168.160
  multusName:
    - default/macvlan-vlan0-eth0
---
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
   name: test-ippool-eth1
spec:
   default: true
   subnet: 10.7.0.0/16
   ips:
      - 10.7.168.151-10.7.168.160
   multusName:
      - kube-system/macvlan-vlan0-eth1
```

- Set the `spec.default` field to `true` to simplify the experience by reducing the need to annotate the application with `ipam.spidernet.io/ippool` or `ipam.spidernet.io/ippools`.

- Configure the `spec.multusName` field to specify the multus net-attach-def instance. (If you do not specify the namespace of the corresponding multus net-attach-def instance, we will default to the namespace where Spiderpool is installed.)

Create an application with multiple network interfaces, you can use the following example YAML:

```bash
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      annotations:
        v1.multus-cni.io/default-network: default/macvlan-vlan0-eth0
        k8s.v1.cni.cncf.io/networks: kube-system/macvlan-vlan0-eth1
      labels:
        app: test-app
    spec:
      containers:
      - name: test-app
        image: nginx
        imagePullPolicy: IfNotPresent
EOF
```

- `v1.multus-cni.io/default-network`: Choose the default network configuration for the created application. (If you don't specify this annotation and directly use the clusterNetwork configuration of the multus, please specify the default network configuration during the installation of Spiderpool via Helm using the parameter `--set multus.multusCNI.defaultCniCRName=default/macvlan-vlan0-eth0`).

- `k8s.v1.cni.cncf.io/networks`: Selects the additional network configuration for the created application.

## Summary

The set of IPs in the SpiderIPPool can be large or small. It can effectively address the limited IP address
resources in the underlay network, and this feature allows different applications and tenants to bind to
different SpiderIPPools through various affinity rules. It also enables sharing the same SpiderIPPool,
allowing all applications to share the same subnet while achieving "micro-segmentation."
