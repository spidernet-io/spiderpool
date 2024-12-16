# SpiderSubnet

**简体中文** | [**English**](./spider-subnet.md)

## 介绍

SpiderSubnet 资源代表 IP 地址的集合，当需要为应用分配固定的 IP 地址时，应用管理员需要平台管理员告知可用的 IP 地址和路由属性等，但双方分属两个不同的运营部门，这使得每一个应用创建的工作流程繁琐，借助于 Spiderpool 的 SpiderSubnet 功能，它能自动从中子网分配 IP 给 SpiderIPPool，并且还能为应用固定 IP 地址，极大的减少了运维的成本。

## SpiderSubnet 功能

启用 Subnet 功能时，每一个 IPPool 实例都归属于子网号相同的 Subnet 实例，IPPool 实例中的 IP 地址必须是 Subnet 实例中 IP 地址的子集，IPPool 实例之间不出现重叠 IP 地址，创建 IPPool 实例时的各种路由属性，默认继承 Subnet 实例中的设置。

在为应用分配固定的 IP 地址时，带来了如下两种实践手段，从而完成应用管理员和网络管理员的职责解耦：

- 手动创建 IPPool : 应用管理员手动创建 IPPool 实例时，可基于对应的 Subnet 实例中的 IP 地址约束，来获知可使用哪些 IP 地址。

- 自动创建 IPPool : 应用管理员可在 Pod annotation 中注明使用的 Subnet 实例名，在应用创建时，Spiderpool 会自动根据 Subnet 实例中的可用 IP 地址来创建固定 IP 的 IPPool 实例，从中分配 IP 地址给 Pod。并且 Spiderpool 能够自动监控应用的扩缩容和删除事件，自动完成 IPPool 中的 IP 地址扩缩容和删除。

SpiderSubnet 功能还支持众多的控制器，如：ReplicaSet、Deployment、Statefulset、Daemonset、Job、Cronjob，第三方控制器等。对于第三方控制器，您可以参考[示例](./operator.md)。

该功能并不支持自主式 Pod。

> 注意：在 v0.7.0 版本之前，在启动 SpiderSubnet 功能下你必须得先创建一个 SpiderSubnet 资源才可以创建 SpiderIPPool 资源。在v0.7.0版本开始，支持创建一个独立的 SpiderIPPool 资源而不依赖于 SpiderSubnet 资源。

## 实施要求

1. 一套 Kubernetes 集群。

2. 已安装 [Helm](https://helm.sh/docs/intro/install/)。

## 步骤

### 安装 Spiderpool

可参考 [安装教程](./readme-zh_CN.md) 来安装 Spiderpool. 请务必确保 helm 安装选项 `--ipam.spiderSubnet.enable=true --ipam.spiderSubnet.autoPool.enable=true`. 其中，`ipam.spiderSubnet.autoPool.enable` 提供 `自动创建 IPPool` 的能力。

### 安装 CNI 配置

Spiderpool 为简化书写 JSON 格式的 Multus CNI 配置，它提供了 SpiderMultusConfig CR 来自动管理 Multus NetworkAttachmentDefinition CR。如下是创建 Macvlan SpiderMultusConfig 配置的示例：

- master：在此示例用接口 `ens192` 和 `ens224` 作为 master 的参数。

```bash
MACVLAN_MASTER_INTERFACE0="ens192"
MACVLAN_MULTUS_NAME0="macvlan-$MACVLAN_MASTER_INTERFACE0"
MACVLAN_MASTER_INTERFACE1="ens224"
MACVLAN_MULTUS_NAME1="macvlan-$MACVLAN_MASTER_INTERFACE1"

cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: ${MACVLAN_MULTUS_NAME0}
  namespace: kube-system
spec:
  cniType: macvlan
  enableCoordinator: true
  macvlan:
    master:
    - ${MACVLAN_MASTER_INTERFACE0}
---
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: ${MACVLAN_MULTUS_NAME1}
  namespace: kube-system
spec:
  cniType: macvlan
  enableCoordinator: true
  macvlan:
    master:
    - ${MACVLAN_MASTER_INTERFACE1}
EOF
```

在本文示例中，使用如上配置，创建如下的两个 Macvlan SpiderMultusConfig，将基于它们自动生成的 Multus NetworkAttachmentDefinition CR，它对应了宿主机的 `ens192` 与 `ens224` 网卡。

```bash
~# kubectl get spidermultusconfigs.spiderpool.spidernet.io -n kube-system
NAME             AGE
macvlan-ens192   26m
macvlan-ens224   26m

~# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system
NAME             AGE
macvlan-ens192   27m
macvlan-ens224   27m
```

### 创建 Subnet

```bash
~# cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderSubnet
metadata:
  name: subnet-6
spec:
  subnet: 10.6.0.0/16
  gateway: 10.6.0.1
  ips:
    - 10.6.168.101-10.6.168.110
  routes:
    - dst: 10.7.0.0/16
      gw: 10.6.0.1
---
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderSubnet
metadata:
  name: subnet-7
spec:
  subnet: 10.7.0.0/16
  gateway: 10.7.0.1
  ips:
    - 10.7.168.101-10.7.168.110
  routes:
    - dst: 10.6.0.0/16
      gw: 10.7.0.1
EOF
```

使用如上的 Yaml，创建 2 个 SpiderSubnet，并分别为其配置网关与路由信息。

```bash
~# kubectl get spidersubnet
NAME       VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT
subnet-6   4         10.6.0.0/16   0                    10
subnet-7   4         10.7.0.0/16   0                    10

~# kubectl get spidersubnet subnet-6  -o jsonpath='{.spec}' | jq
{
  "gateway": "10.6.0.1",
  "ipVersion": 4,
  "ips": [
    "10.6.168.101-10.6.168.110"
  ],
  "routes": [
    {
      "dst": "10.7.0.0/16",
      "gw": "10.6.0.1"
    }
  ],
  "subnet": "10.6.0.0/16",
  "vlan": 0
}

~# kubectl get spidersubnet subnet-7  -o jsonpath='{.spec}' | jq
{
  "gateway": "10.7.0.1",
  "ipVersion": 4,
  "ips": [
    "10.7.168.101-10.7.168.110"
  ],
  "routes": [
    {
      "dst": "10.6.0.0/16",
      "gw": "10.7.0.1"
    }
  ],
  "subnet": "10.7.0.0/16",
  "vlan": 0
}
```

### 自动固定单网卡 IP

以下的示例 Yaml 中， 会创建 2 个副本的 Deployment 应用 ，其中：

- `ipam.spidernet.io/subnet`：用于指定 Spiderpool 的子网，Spiderpool 会自动在该子网中随机选择一些 IP 来创建固定 IP 池，与本应用绑定，实现 IP 固定的效果。在本示例中该注解会为 Pod 创建 1 个对应子网的固定 IP 池。(注意：不支持通配符的形式。)

- `ipam.spidernet.io/ippool-ip-number`：用于指定创建 IP 池 中 的 IP 数量。该 annotation 的写法支持两种方式：一种是数字的方式指定 IP 池的固定数量，例如 `ipam.spidernet.io/ippool-ip-number：1`；另一种方式是使用加号和数字指定 IP 池的相对数量，例如`ipam.spidernet.io/ippool-ip-number：+1`，即表示 IP 池中的数量会自动实时保持在应用的副本数的基础上多 1 个 IP，以解决应用在弹性扩缩容的时有临时的 IP 可用。

- `ipam.spidernet.io/ippool-reclaim`： 其表示自动创建的固定 IP 池是否随着应用的删除而被回收。

- `v1.multus-cni.io/default-network`：为应用创建一张默认网卡。

```bash
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app-1
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test-app-1
  template:
    metadata:
      annotations:
        ipam.spidernet.io/subnet: |-
            {      
              "ipv4": ["subnet-6"]
            }
        ipam.spidernet.io/ippool-ip-number: '+1'
        v1.multus-cni.io/default-network: kube-system/macvlan-ens192
        ipam.spidernet.io/ippool-reclaim: "false"
      labels:
        app: test-app-1
    spec:
      containers:
      - name: test-app-1
        image: nginx
        imagePullPolicy: IfNotPresent
        ports:
        - name: http
          containerPort: 80
          protocol: TCP
EOF
```

最终, 在应用被创建时，Spiderpool 会从指定子网中随机选择一些 IP 来创建出固定 IP 池，与 Pod 的网卡形成绑定，同时自动池会自动继承子网的网关、路由属性。

```bash
~# kubectl get spiderippool
NAME                          VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
auto4-test-app-1-eth0-a5bd3   4         10.6.0.0/16   2                    3                false

~# kubectl get po -l app=test-app-1 -o wide
NAME                         READY   STATUS    RESTARTS   AGE   IP             NODE                NOMINATED NODE   READINESS GATES
test-app-1-74cbbf654-2ndzl   1/1     Running   0          46s   10.6.168.101   controller-node-1   <none>           <none>
test-app-1-74cbbf654-4f2w2   1/1     Running   0          46s   10.6.168.103   worker-node-1       <none>           <none>

~# kubectl get spiderippool auto4-test-app-1-eth0-a5bd3 -ojsonpath={.spec} | jq
{
  "default": false,
  "disable": false,
  "gateway": "10.6.0.1",
  "ipVersion": 4,
  "ips": [
    "10.6.168.101-10.6.168.103"
  ],
  "podAffinity": {
    "matchLabels": {
      "ipam.spidernet.io/app-api-group": "apps",
      "ipam.spidernet.io/app-api-version": "v1",
      "ipam.spidernet.io/app-kind": "Deployment",
      "ipam.spidernet.io/app-name": "test-app-1",
      "ipam.spidernet.io/app-namespace": "default"
    }
  },
  "routes": [
    {
      "dst": "10.7.0.0/16",
      "gw": "10.6.0.1"
    }
  ],
  "subnet": "10.6.0.0/16",
  "vlan": 0
}
```

为实现固定 IP 池效果，Spiderpool 会给自动池补充如下的一些特殊的内建 Label 和 PodAffinity，它们用于指向应用，与应用形成绑定关系，明确该池只服务于指定的应用。当 `ipam.spidernet.io/ippool-reclaim: false` 时，删除应用后，IP会被回收，但自动池不会被回收。如果期望该池能被其他应用所使用，需要手动摘除这些内建 Label 和 PodAffinity。

```bash
Additional Labels:
  ipam.spidernet.io/owner-application-gv
  ipam.spidernet.io/owner-application-kind
  ipam.spidernet.io/owner-application-namespace
  ipam.spidernet.io/owner-application-name
  ipam.spidernet.io/owner-application-uid

Additional PodAffinity:
  ipam.spidernet.io/app-api-group
  ipam.spidernet.io/app-api-version
  ipam.spidernet.io/app-kind
  ipam.spidernet.io/app-namespace
  ipam.spidernet.io/app-name
```

经过多次测试，不断重启 Pod，其 Pod 的 IP 都被固定在 IP 池范围内:

```bash
~# kubectl delete po -l app=test-app-1

~# kubectl get po -l app=test-app-1 -o wide
NAME                         READY   STATUS    RESTARTS   AGE   IP             NODE                NOMINATED NODE   READINESS GATES
test-app-1-74cbbf654-7v54p   1/1     Running   0          7s    10.6.168.101   worker-node-1       <none>           <none>
test-app-1-74cbbf654-qzxp7   1/1     Running   0          7s    10.6.168.102   controller-node-1   <none>           <none>
```

### 固定 IP 池的名字

固定 IP 池是根据应用自动创建的，因此需要一个唯一且可查询的名字。目前命名规则遵循以下格式：`auto{ipVersion}-{appName}-{NicName}-{Max5RandomCharacter}`

- ipVersion：表示 IPv4 或者 IPv6 的池，值为 4 或者 6
- appName：表示应用的名字
- NicName：表示分配给 POD 的网卡名字
- Max5RandomCharacter：表示从应用 UUID 中生成的 5 位随机字符串，用于区分不同应用的固定 IP 池

例如, 果你创建一个名为 nginx 的 deployment，其网卡名为 eth0，那么它的固定 IP 池名字为 `auto4-nginx-eth0-9a2b3`

### 动态扩缩固定 IP 池

创建应用时指定了注解 `ipam.spidernet.io/ippool-ip-number`: '+1'，其表示应用分配到的固定 IP 数量比应用的副本数多 1 个，在应用滚动更新时，能够避免旧 Pod 未删除，新 Pod 没有可用 IP 的问题。

以下演示了扩容场景，将应用的副本数从 2 扩容到 3，应用对应的两个固定 IP 池会自动从 3 个 IP 扩容到 4 个 IP，一直保持一个冗余 IP，符合预期。

```bash
~# kubectl scale deploy test-app-1 --replicas 3
deployment.apps/test-app-1 scaled

~# kubectl get po -l app=test-app-1 -o wide
NAME                         READY   STATUS    RESTARTS   AGE   IP             NODE                NOMINATED NODE   READINESS GATES
test-app-1-74cbbf654-7v54p   1/1     Running   0          54s   10.6.168.101   worker-node-1       <none>           <none>
test-app-1-74cbbf654-9w8gd   1/1     Running   0          19s   10.6.168.103   worker-node-1       <none>           <none>
test-app-1-74cbbf654-qzxp7   1/1     Running   0          54s   10.6.168.102   controller-node-1   <none>           <none>

~# kubectl get spiderippool
NAME                          VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
auto4-test-app-1-eth0-a5bd3   4         10.6.0.0/16   3                    4                false
```

通过上述，Spiderpool 对于应用扩缩容的场景，只需要修改应用的副本数即可。

### 自动回收 IP 池

创建应用时指定了注解 `ipam.spidernet.io/ippool-reclaim`，该注解默认值为 `true`，为 true 时，随着应用的删除，将自动删除对应的自动池。在本文中设置为 `false`，其表示删除应用时，自动创建的固定 IP 池会回收其中被分配的 IP ，但池不会被回收。

对于需要保留池的应用场景，可以是当应用再次以同名的 deployment 或者 statefulset 来创建应用时，能够继续使用原有创建的 IP 池，以保持 IP 不变。

```bash
~# kubectl delete deploy test-app-1
deployment.apps "test-app-1" deleted

~# kubectl get spiderippool
NAME                          VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
auto4-test-app-1-eth0-a5bd3   4         10.6.0.0/16   0                    4                false
```

使用上述所示的应用 Yaml，再次创建同名应用，可以观察到不会再次创建新的 IP 池，将自动复用旧 IP 池，并且其副本数和 IP 池的 IP 分配情况与实际相同。

```bash
~# kubectl get spiderippool
NAME                          VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
auto4-test-app-1-eth0-a5bd3   4         10.6.0.0/16   2                    3                false
```

### 自动固定多网卡 IP

如果您希望为 Pod 实现多网卡 IP 的固定，参考本章节。在以下的示例 Yaml 中， 会创建 2 个副本的 Deployment，每个副本拥有多张网卡，其中：

- `ipam.spidernet.io/subnets`：用于指定 Spiderpool 的子网，Spiderpool 会自动在该子网中随机选择一些 IP 来创建固定 IP 池，与本应用绑定，实现 IP 固定的效果。在本示例中该注解会为 Pod 创建 2 个属于不同 Underlay 子网的固定 IP 池。

- `v1.multus-cni.io/default-network`：为应用创建一张默认网卡。

- `k8s.v1.cni.cncf.io/networks`：为应用创建另一张网卡。

```bash
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app-2
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test-app-2
  template:
    metadata:
      annotations:
        ipam.spidernet.io/subnets: |-
         [
            {      
              "interface": "eth0",
              "ipv4": ["subnet-6"]
            },{      
              "interface": "net1",
              "ipv4": ["subnet-7"]
            }
         ]
        v1.multus-cni.io/default-network: kube-system/macvlan-ens192
        k8s.v1.cni.cncf.io/networks: kube-system/macvlan-ens224
      labels:
        app: test-app-2
    spec:
      containers:
      - name: test-app-2
        image: nginx
        imagePullPolicy: IfNotPresent
        ports:
        - name: http
          containerPort: 80
          protocol: TCP
EOF
```

最终, 在应用创建时，Spiderpool 会从指定 2 个 Underlay 子网中随机选择一些 IP 来创建出对应的固定 IP 池，并与应用 Pod 的两张网卡分别形成绑定。每张网卡对应的固定池都将会自动继承其所归属子网的网关、路由等属性。

```bash
~# kubectl get spiderippool
NAME                          VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
auto4-test-app-2-eth0-44037   4         10.6.0.0/16   2                    3                false
auto4-test-app-2-net1-44037   4         10.7.0.0/16   2                    3                false

~# kubectl get po -l app=test-app-2 -o wide
NAME                          READY   STATUS    RESTARTS   AGE     IP             NODE                NOMINATED NODE   READINESS GATES
test-app-2-f5d6b8d6c-8hxvw    1/1     Running   0          6m22s   10.6.168.101   controller-node-1   <none>           <none>
test-app-2-f5d6b8d6c-rvx55    1/1     Running   0          6m22s   10.6.168.105   worker-node-1       <none>           <none>

~# kubectl get spiderippool auto4-test-app-2-eth0-44037 -ojsonpath={.spec} | jq
{
  "default": false,
  "disable": false,
  "gateway": "10.6.0.1",
  "ipVersion": 4,
  "ips": [
    "10.6.168.101",
    "10.6.168.105-10.6.168.106"
  ],
  "podAffinity": {
    "matchLabels": {
      "ipam.spidernet.io/app-api-group": "apps",
      "ipam.spidernet.io/app-api-version": "v1",
      "ipam.spidernet.io/app-kind": "Deployment",
      "ipam.spidernet.io/app-name": "test-app-2",
      "ipam.spidernet.io/app-namespace": "default"
    }
  },
  "routes": [
    {
      "dst": "10.7.0.0/16",
      "gw": "10.6.0.1"
    }
  ],
  "subnet": "10.6.0.0/16",
  "vlan": 0
}

~# kubectl get spiderippool auto4-test-app-2-net1-44037 -ojsonpath={.spec} | jq
{
  "default": false,
  "disable": false,
  "gateway": "10.7.0.1",
  "ipVersion": 4,
  "ips": [
    "10.7.168.101-10.7.168.103"
  ],
  "podAffinity": {
    "matchLabels": {
      "ipam.spidernet.io/app-api-group": "apps",
      "ipam.spidernet.io/app-api-version": "v1",
      "ipam.spidernet.io/app-kind": "Deployment",
      "ipam.spidernet.io/app-name": "test-app-2",
      "ipam.spidernet.io/app-namespace": "default"
    }
  },
  "routes": [
    {
      "dst": "10.6.0.0/16",
      "gw": "10.7.0.1"
    }
  ],
  "subnet": "10.7.0.0/16",
  "vlan": 0
}
```

SpiderSubnet 也支持多网卡的动态 IP 扩缩容、自动回收 IP 池等功能。

### 手动创建 IPPool 实例继承子网属性

如下是一个归属于子网 `subnet-6`，子网号为：`10.6.0.0/16` 的 IPPool 实例示例。该 IPPool 实例的可用 IP 范围必须是子网 `subnet-6.spec.ips` 的子集。

```bash
~# cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: ippool-test
spec:
  ips:
  - 10.6.168.108-10.6.168.110
  subnet: 10.6.0.0/16
EOF
```

使用上述 Yaml，手动创建 IPPool 实例，可以看到它归属于子网号相同的子网，同时继承了对应子网的网关、路由等属性。

```bash
~# kubectl get spiderippool
NAME          VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
ippool-test   4         10.6.0.0/16   0                    3                false

~# kubectl get spidersubnet
NAME       VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT
subnet-6   4         10.6.0.0/16   3                    10
subnet-7   4         10.7.0.0/16   0                    10

~# kubectl get spiderippool ippool-test  -o jsonpath='{.spec}' | jq
{
  "default": false,
  "disable": false,
  "gateway": "10.6.0.1",
  "ipVersion": 4,
  "ips": [
    "10.6.168.108-10.6.168.110"
  ],
  "routes": [
    {
      "dst": "10.7.0.0/16",
      "gw": "10.6.0.1"
    }
  ],
  "subnet": "10.6.0.0/16",
  "vlan": 0
}
```

## 总结

SpiderSubnet 功能可以帮助将基础设施管理员和应用程序管理员的责任分开，支持自动创建和动态扩展固定 IPPool 到每个需要静态 IP 的应用程序。
