# Egress Policy

[**English**](./egress.md) | **简体中文**

## 介绍

Spiderpool 是一个 Kubernetes 的 Underlay 网络解决方案，但在 Kubernetes 集群中，Pod 访问外部服务时，其出口 IP 地址不是固定的。在 Overlay 网络中，出口 IP 地址为 Pod 所在节点的地址，而在 Underlay 网络中，Pod 直接使用自身的 IP 地址与外部通信。因此，当 Pod 发生新的调度时，无论哪种网络模式，Pod 与外部通信时的 IP 地址都会发生变化。这种不稳定性给系统维护人员带来了 IP 地址管理的挑战。特别是在集群规模扩大以及需要进行网络故障诊断时，在集群外部，基于 Pod 原本的出口 IP 来管控出口流量很难实现。而 Spiderpool 可以搭配组件 [EgressGateway](https://github.com/spidernet-io/egressgateway) 完美解决 Underlay 网络下 Pod 出口流量管理的问题。

## 项目功能

EgressGateway 是一个开源的 Egress 网关，旨在解决在不同 CNI 网络模式下（Spiderpool、Calico、Flannel、Weave）出口 Egress IP 地址的问题。通过灵活配置和管理出口策略，为租户级或集群级工作负载设置 Egress IP，使得 Pod 访问外部网络时，系统会统一使用这个设置的 Egress IP 作为出口地址，从而提供了稳定的出口流量管理解决方案。但 EgressGateway 所有的规则都是生效在主机网络命名空间上的，要使 EgressGateway 策略生效，则 Pod 访问集群外部的流量，要经过主机的网络命名空间。因此可以通过 Spiderpool 的 `spidercoordinators` 的 `spec.hijackCIDR` 字段配置从主机转发的子网路由，再通过 [coordinator](../concepts/coordinator-zh_CN.md) 将匹配的流量从 veth pair 转发到主机上。使得所访问的外部流量从而被 EgressGateway 规则匹配，借此实现 underlay 网络下出口流量管理。

Spiderpool 搭配 EgressGateway 具备如下的一些功能：

- 解决 IPv4/IPv6 双栈连接问题，确保网络通信在不同协议栈下的无缝连接。
- 解决 Egress 节点的高可用性问题，确保网络连通性不受单点故障的干扰。
- 允许更精细的策略控制，可以通过 EgressGateway 灵活地过滤 Pods 的 Egress 策略，包括 Destination CIDR。
- 允许过滤 Egress 应用（Pod），能够更精确地管理特定应用的出口流量。
- 支持多个出口网关实例，能够处理多个网络分区或集群之间的通信。
- 支持租户级别的 Egress IP。
- 支持自动检测集群流量的 Egress 网关策略。
- 支持命名空间默认 Egress 实例。
- 可用于较低内核版本，适用于各种 Kubernetes 部署环境。

## 实施要求

1. 一套 Kubernetes 集群。

2. 已安装 [Helm](https://helm.sh/docs/intro/install/)。

## 步骤

### 安装 Spiderpool

可参考 [安装](./readme-zh_CN.md) 安装 Spiderpool，并创建 SpiderMultusConfig CR 与 IPPool CR.

在安装完 Spiderpool 后，将集群外的服务地址添加到 `spiderpool.spidercoordinators` 的 'default' 对象的 'hijackCIDR' 中，使 Pod 访问这些外部服务时，流量先经过 Pod 所在的主机，从而被 EgressGateway 规则匹配。

```shell
# "10.6.168.63/32" 为外部服务地址。对于已经运行的 Pod，需要重启 Pod，这些路由规则才会在 Pod 中生效。
~# kubectl patch spidercoordinators default  --type='merge' -p '{"spec": {"hijackCIDR": ["10.6.168.63/32"]}}'
```

### 安装 EgressGateway

通过 helm 安装 EgressGateway

```shell
helm repo add egressgateway https://spidernet-io.github.io/egressgateway/
helm repo update egressgateway
helm install egressgateway egressgateway/egressgateway -n kube-system --set feature.tunnelIpv4Subnet="192.200.0.1/16" --set feature.enableGatewayReplyRoute=true  --wait --debug
```

> 如果需要使用 IPv6 ，可使用选项 `--set feature.enableIPv6=true` 开启，并设置 `feature.tunnelIpv6Subnet`, 值得注意的是在通过 `feature.tunnelIpv4Subnet` 与 `feature.tunnelIpv6Subnet` 配置 IPv4 或 IPv6 网段时，需要保证网段和集群内的其他地址不冲突。
> `feature.enableGatewayReplyRoute` 为 true 时，将开启网关节点上的返回路由规则，在与 Spiderpool 搭配支持 underlay CNI 时，必须开启该选项。
>
> 如果您是中国用户，还可以指定参数 `--set global.imageRegistryOverride=ghcr.m.daocloud.io` 避免 EgressGateway 的镜像拉取失败。

验证 EgressGateway 安装

```bash
~# kubectl get pod -n kube-system | grep egressgateway
egressgateway-agent-4s8lt                   1/1     Running     0     29m
egressgateway-agent-thzth                   1/1     Running     0     29m
egressgateway-controller-77698899df-tln7j   1/1     Running     0     29m
```

更多安装细节，参考 [EgressGateway 安装](https://github.com/spidernet-io/egressgateway/blob/main/docs/usage/Install.zh.md)

### 创建 EgressGateway 实例

EgressGateway 定义了一组节点作为集群的出口网关，集群内的 egress 流量将会通过这组节点转发而出集群。因此，需要预先定义 EgressGateway 实例，以下的示例 Yaml 会创建一个 EgressGateway 实例，其中：

- `spec.ippools.ipv4`：定义了一组 egress 的出口 IP 地址，需要根据具体环境的实际情况调整。并且 `spec.ippools.ipv4` 的 CIDR 应该与网关节点上的出口网卡（一般情况下是默认路由的网卡）的子网相同，否则，可能导致 egress 访问不通。

- `spec.nodeSelector`：EgressGateway 提供的节点亲和性方式，当 `selector.matchLabels` 与节点匹配时，该节点将会被作为集群的出口网关，当 `selector.matchLabels` 与节点不匹配时，EgressGateway 会略过该节点，它将不会被作为集群的出口网关，它支持选择多个节点来实现高可用。

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

给节点打上上述中 `nodeSelector.selector.matchLabels` 所指定的 Label，使节点能被 EgressGateway 选中，作为出口网关。

```bash
~# kubectl get node
NAME                STATUS   ROLES           AGE     VERSION
controller-node-1   Ready    control-plane   5d17h   v1.26.7
worker-node-1       Ready    <none>          5d17h   v1.26.7

~# kubectl label node worker-node-1 egressgateway="true"
```

创建完成后，查看 EgressGateway 状态。其中：

- `spec.ippools.ipv4DefaultEIP` 代表该组 EgressGateway 的默认 VIP，它是会从 `spec.ippools.ipv4` 中随机选择的一个 IP 地址，它的作用是：当为应用创建 EgressPolicy 对象时，如果未指定 VIP 地址，则使用该默认 VIP

- `status.nodeList` 代表识别到了符合 `spec.nodeSelector` 的节点及该节点对应的 EgressTunnel 对象的状态。

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

### 创建应用和出口策略

创建一个应用，它将用于测试 Pod 访问集群外部用途，并给它打上 label 以便与 EgressPolicy 关联，如下是示例 Yaml，其中：

- `v1.multus-cni.io/default-network`：用于指定应用所使用的子网，该值对应的 Multus CR 需参考[安装](./readme-zh_CN.md)文档提前创建。

- `ipam.spidernet.io/ippool`：指定 Pod 使用哪些的 SpiderIPPool 资源, 该值对应的 SpiderIPPool CR 需参考[安装](./readme-zh_CN.md)文档提前创建。

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
              "ipv4": ["v4-pool"]
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

EgressPolicy 实例用于定义哪些 Pod 的出口流量要经过 EgressGateway 节点转发，以及其它的配置细节。如下是为应用创建 EgressPolicy CR 对象的示例，其中。

- `spec.egressGatewayName` 用于指定了使用哪一组 EgressGateway 。

- `spec.appliedTo.podSelector` 用于指定本策略在集群内的哪些 Pod 上生效。

- `namespace` 用于指定 EgressPolicy 对象所在租户，因为 EgressPolicy 是租户级别的，所以它务必创建在上述关联应用的相同 namespace 下，这样匹配的 Pod 访问任意集群外部的地址时，才能被 EgressGateway Node 转发。

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

创建完成后，查看 EgressPolicy 的状态。其中：

- `status.eip` 展示了该组应用出集群时使用的出口 IP 地址。

- `status.node` 展示了哪一个 EgressGateway 的节点负责该 EgressPolicy 出口流量的转发。

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

创建 EgressPolicy 对象后，会根据 EgressPolicy 选择的应用生成一个包含所有应用的 IP 地址集合的 EgressEndpointSlices 对象，当应用无法出口访问时，可以查看 EgressEndpointSlices 对象中的 IP 地址是否正常。

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

### 测试

在集群外部署应用 `nettools`，用于模拟一个集群外部的服务，而 `nettools` 会在 http 回复中返回请求者的源 IP 地址。

```bash
~# docker run -d --net=host ghcr.io/spidernet-io/egressgateway-nettools:latest /usr/bin/nettools-server -protocol web -webPort 8080
```

在集群内的测试应用：test-app 中，验证出口流量的效果，我们可以看到在该应用对应 Pod 中访问外部服务时，`nettools` 返回的源 IP 符合了 `EgressPolicy.status.eip` 的效果。

```bash
~# kubectl get pod -owide
NAME                       READY   STATUS    RESTARTS      AGE     IP              NODE                NOMINATED NODE   READINESS GATES
test-app-f44846544-8dnzp   1/1     Running   0             4m27s   10.6.168.208    worker-node-1       <none>           <none>

~# kubectl exec -it test-app-f44846544-8dnzp bash

~# curl 10.6.1.92:8080 # 集群外节点的 IP 地址 + webPort
Remote IP: 10.6.168.201
```
