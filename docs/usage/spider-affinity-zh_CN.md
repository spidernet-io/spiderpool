# SpiderIPPool Affinity

**简体中文** | [**English**](./spider-affinity.md)

## 介绍

SpiderIPPool 资源代表 IP 地址的集合，一个 Subnet 中的不同 IP 地址，可分别存储到不同的 IPPool 实例中（Spiderpool 会校验 IPPool 之间的地址集合不重叠）。因此，依据需求，SpiderIPPool 中的 IP 集合可大可小。能很好的应对 underlay 网络的 IP 地址资源有限情况，且这种设计特点，能够通过各种亲和性规则让不同的应用、租户来绑定不同的 SpiderIPPool，也能分享相同的 SpiderIPPool，既能够让所有应用共享使用同一个子网，又能够实现 "微隔离"。

## 快速入门

在 [SpiderIPPool CRD](./../reference/crd-spiderippool.md) 里，我们有定义很多的字段来搭配亲和性使用，如:

- `spec.podAffinity` 字段可控制该池是否可被 Pod 使用。
- `spec.namespaceName` 和 `spec.namespaceAffinity` 字段会校验是否与 Pod 的Namespace相匹配，若不匹配则不可使用。(`namespaceName` 优先级高于 `namespaceAffinity`)
- `spec.nodeName` 和 `spec.nodeAffinity` 字段会校验是否与 Pod 所在的节点相匹配，若不匹配则不可使用。(`nodeName` 优先级高于 `nodeAffinity`)
- `multusName` 字段会判断当前网卡是否与 multus 的 net-attach-def 资源使用的 CNI 配置相匹配，若不匹配则不可使用。

这些字段不仅起到**过滤**的作用，同时也会起到一个**排序**的效果，若匹配的字段越多，越优先使用该 IP 池。

```yaml
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
  nodeName:
    - master
    - worker1
  namespaceName:
    - kube-system
    - default
  multusName:
    - kube-system/macvlan-vlan0
```

## 应用亲和性

在集群中，防火墙通常用于管理南北向通信，即集群内部和外部网络之间的通信。为了实现安全管控，防火墙需要对通信流量进行检查和过滤，并对出口通信进行限制。由于防火墙安全管控，一组 Deployment 它的所有 Pod 期望能够在一个固定的 IP 地址范围内轮滚分配 IP 地址，以配合防火墙的放行策略，从而实现 Underlay 网络下的南北通信。

在社区现有方案中，是通过在 Deployment 上写关于 IP 地址的注解来实现。但这种方式存在一些缺点，如：

- 随着应用的扩容，需要人为手动的修改应用的 annotaiton ，重新规划 IP 地址。

- annotaiton 方式的 IP 管理，脱钩于它们自身的 IPPool CR 机制，形成管理上的空白，无法获知哪些 IP 可用。

- 不同应用间极其容易分配了冲突的 IP 地址，从而导致应用创建失败。

对此，Spiderpool 借助于 IPPool 的 IP 集合可大可小的特点，并结合设置 IPPool 的 `podAffinity`，可实现同一组或者多组应用的亲和绑定，既保证了 IP 管理方式的统一，又解耦 "应用扩容" 和 "IP 地址扩容"，也固定了应用的 IP 使用范围。

### 创建应用亲和性的 IPPool

SpiderIPPool 提供了 `podAffinity` 字段，当应用创建时，尝试从 SpiderIPPool 分配 IP 时，若 Pod 的 `selector.matchLabels` 符合该 podAffinity 设置，则能从该 SpiderIPPool 中成功分配出 IP，否则无法从该 SpiderIPPool 中分配出IP。

依据如上所述，使用如下的 YAML，创建如下具备应用亲和的 SpiderIPPool，它将为 `app: test-app-3` Pod 符合条件的 `selector.matchLabel` 提供 IP 地址。

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

创建指定 matchLabels 的应用。以下的示例 YAML 中， 会创建一组 Deployment 应用：

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

- `ipam.spidernet.io/ippool`：Spiderpool 用于指定设置了应用亲和的 IP 池。
- `v1.multus-cni.io/default-network`：为应用创建一张默认网卡。
- `matchLabels`: 设置应用的 Label。

最终，创建应用后，Pod 的 `matchLabels` 符合该 IPPool 的应用亲和设置，成功从该 IPPool 中获得 IP 地址分配。并且应用的 IP 固定在该 IP 池内。

```bash
～# kubectl get spiderippool
NAME                VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
test-pod-ippool     4         10.6.0.0/16   1                    10               false

~# kubectl get po -l app=test-app-3 -owide
NAME                          READY   STATUS    RESTARTS   AGE   IP             NODE    NOMINATED NODE   READINESS GATES
test-app-3-6994b9d5bb-qpf5p   1/1     Running   0          52s   10.6.168.154   node2   <none>           <none>
```

创建另一个应用，并指定一个不符合 IPPool 应用亲和的 `matchLabels`，Spiderpool 将会拒绝为其分配 IP 地址。

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

- `matchLabels`: 设置应用的 Label 为 `test-unmatch-labels`，不匹配 IPPool 亲和性。

当 Pod 的 matchLabels 不符合该 IPPool 的应用亲和时，获得 IP 地址分配失败，符合预期。

```bash
kubectl get po -l app=test-unmatch-labels -owide
NAME                                  READY   STATUS              RESTARTS   AGE   IP       NODE    NOMINATED NODE   READINESS GATES
test-unmatch-labels-699755574-9ncp7   0/1     ContainerCreating   0          16s   <none>   node1   <none>           <none>
```

### 应用共享的 IPPool

1. 创建应用共享的 IPPool

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

2. 创建两个 deployment，其 Pod 设置注释 “ipam.spidernet.io/ippool” 以显式指定池选择规则。它将成功获得IP地址

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

    确认最终状态

    ```bash
    kubectl get po -l app=static -o wide
    NAME                                             READY   STATUS    RESTARTS   AGE   IP             NODE              
    shared-static-ippool-deploy-1-8588c887cb-gcbjb   1/1     Running   0          62s   172.18.41.45   spider-control-plane 
    shared-static-ippool-deploy-1-8588c887cb-wfdvt   1/1     Running   0          62s   172.18.41.46   spider-control-plane 
    shared-static-ippool-deploy-2-797c8df6cf-6vllv   1/1     Running   0          62s   172.18.41.44   spider-worker 
    shared-static-ippool-deploy-2-797c8df6cf-ftk2d   1/1     Running   0          62s   172.18.41.47   spider-worker
    ```

## 节点亲和性

不同的 node 上，可用的 IP 范围也许并不相同，例如：

- 同一数据中心内，集群接入的 node 分属不同 subnet 。

- 单个集群中，node 跨越了不同的数据中心。

在以上场景中，当同一个应用的不同副本被调度到了不同的 node 上，需要分配不同 subnet 下的 underlay IP 地址。在当前社区现有方案，它们并不能满足这样的需求。

对此，Spiderpool 提供一种节点亲和的方式，能很好的解决上述问题。Spiderpool 的 SpiderIPPool CR 中，提供了 `nodeAffinity` 与 `nodeName` 字段，用于设置 node label selector，从而实现 IPPool 和 node 之间亲和性，当 Pod 被调度到某个 node 上后，IPAM 插件能够从亲和的 IPPool 中进行 IP 地址分配。

### 创建节点亲和的 IPPool

SpiderIPPool 提供了 `nodeAffinity` 字段，当 Pod 在某个节点上启动，尝试从 SpiderIPPool 分配 IP 时，若 Pod 所在节点符合该 nodeAffinity 设置，则能从该 SpiderIPPool 中成功分配出 IP，否则无法从该 SpiderIPPool 中分配出IP。

依据如上所述，使用如下的 YAML，创建如下具备节点亲和的 SpiderIPPool，它将为在运行该节点上的 Pod 提供 IP 地址。

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

SpiderIPPool 提供了另一种节点亲和性方式供选择：`nodeName`，当 `nodeName` 不为空时，Pod 在某个节点上启动，并尝试从 SpiderIPPool 分配 IP 地址, 若 Pod 所在节点符合该 `nodeName`，则能从该 SpiderIPPool 中成功分配出 IP，若 Pod 所在节点不符合 `nodeName`，则无法从该 SpiderIPPool 中分配出 IP。当 nodeName 为空时，Spiderpool 对 Pod 不实施任何分配限制，参考如下：

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

创建应用。以下的示例 YAML 中，会创建 1 组 DaemonSet 应用，其中：

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

- `ipam.spidernet.io/ippool`：Spiderpool 用于指定设置了节点亲和的 IP 池。
- `v1.multus-cni.io/default-network`：用于指定应用所使用的 IP 池。

创建应用后，可以发现，只有当 Pod 所在节点符合该 IPPool 的节点亲和设置，才能从该 IPPool 中获得 IP 地址分配。并且应用的 IP 固定在该 IP 池内。

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

## 租户亲和性

管理员往往会在集群划分多租户，能更好地隔离、管理和协作，同时也能提供更高的安全性、资源利用率和灵活性等。需要不同功能的应用部署在不同租户下，对此，期望实现一个 IPPool 能同一个或者多个 namespace 下的应用实现亲和，而拒绝不相干租户的应用创建，能帮助管理员减少运维负担。

当前社区中并没有解决上述场景的有效方案，Spiderpool 通过设置 SpiderIPPool CR 中的 `namespaceAffinity` 或 `namespaceName` 字段，实现同一个或者多个租户的亲和性，从而使得满足条件的应用才能够从 IPPool 中分配到 IP 地址。

### 创建租户亲和的 IPPool

创建租户

```bash
~# kubectl create ns test-ns1
namespace/test-ns1 created
~# kubectl create ns test-ns2
namespace/test-ns2 created
```

使用如下的 YAML，创建租户亲和的 IPPool。

SpiderIPPool 提供了 `namespaceAffinity` 字段，当应用创建时，尝试从 SpiderIPPool 分配 IP 时，若 Pod 所在租户符合该 namespaceAffinity 设置，则能从该 SpiderIPPool 中成功分配出 IP，否则无法从该 SpiderIPPool 中分配出IP。

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

SpiderIPPool 提供了另一种租户亲和性方式供选择：`namespaceName`，当 `namespaceName` 不为空时，Pod 被创建时，并尝试从 SpiderIPPool 分配 IP 地址, 若 Pod 所在租户符合该 `namespaceName`，则能从该 SpiderIPPool 中成功分配出 IP，若 Pod 所在租户不符合 `namespaceName`，则无法从该 SpiderIPPool 中分配出 IP。当 `namespaceName` 为空时，Spiderpool 对 Pod 不实施任何分配限制，参考如下：

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

创建指定租户的应用。以下的示例 YAML 中，会创建一组在租户 `test-ns1` 下的 Deployment 应用：

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

- `ipam.spidernet.io/ippool`：Spiderpool 用于指定设置了租户亲和的 IP 池。
- `v1.multus-cni.io/default-network`：为应用创建一张默认网卡。
- `namespace`：指定应用所在租户。

最终, 创建应用后，在租户内的应用 Pod 成功从所亲和的 IPPool 中分配到了 IP 地址。

```bash
~# kubectl get spiderippool
NAME              VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
test-ns1-ippool   4         10.6.0.0/16   1                    10               false

~# kubectl get  po -l app=test-app-2 -A  -o wide
NAMESPACE   NAME                      READY   STATUS    RESTARTS   AGE   IP             NODE    NOMINATED NODE   READINESS GATES
test-ns1    test-app-2-975d9f-6bww2   1/1     Running   0          44s   10.6.168.111   node2   <none>           <none>
```

创建一个不在上述 `test-ns1` 租户内的应用，Spiderpool 将会拒绝为其分配 IP 地址，自动拒绝不相干租户的应用使用该 IPPool。

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

当 Pod 所属租户不符合该 IPPool 的租户亲和，获得 IP 地址分配失败，符合预期。

```bash
~# kubectl get po -l app=test-other-ns -A -o wide
NAMESPACE     NAME                              READY   STATUS              RESTARTS   AGE   IP       NODE    NOMINATED NODE   READINESS GATES
test-ns2    test-other-ns-56cc9b7d95-hx4b5   0/1     ContainerCreating   0          6m3s   <none>   node2   <none>           <none>
```

## 网卡配置亲和性

当为应用创建多网卡时候，我们可以为**集群级别缺省池**指定 multus 的 net-attach-def 实例亲和性。该方法相比于通过注解 `ipam.spidernet.io/ippools` 显式指定网卡与 IPPool 资源的绑定关系更为简单。

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

- `spec.default` 字段设置为 `true`, 以此减少为应用打上 `ipam.spidernet.io/ippool` 或 `ipam.spidernet.io/ippools` 注解，让体验更为简单。

- `spec.multusName` 字段配置该 IPPool 对应的 multus 网卡配置。(若您未指定对应 multus 的 net-attach-def 实例的 namespace，我们会默认将其视为属于 spiderpool 安装时的命名空间)

创建多网卡的应用。我们只需以下的示例 YAML 中， 会创建有两张网卡的 Deployment 应用 ，其中：

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

- `v1.multus-cni.io/default-network`：为创建的应用选择默认网卡配置信息。(若不指定该注解而直接使用 multus 集群默认网卡配置信息，请在 helm 安装 spiderpool 时通过参数指定默认网卡配置信息 `--set multus.multusCNI.defaultCniCRName=default/macvlan-vlan0-eth0`)

- `k8s.v1.cni.cncf.io/networks`：为创建的应用选择额外网卡的配置信息。

## 总结

SpiderIPPool 中的 IP 集合可大可小。能很好的应对 Underlay 网络的 IP 地址资源有限情况，且这种设计特点，
能够通过各种亲和性规则让不同的应用、租户来绑定不同的 SpiderIPPool，也能分享相同的 SpiderIPPool，
既能够让所有应用共享使用同一个 Subnet，又能够实现 "微隔离"。
