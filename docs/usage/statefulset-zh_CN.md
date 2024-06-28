# StatefulSet

**简体中文** | [**English**](./statefulset.md)

## 介绍

*由于 StatefulSet 多用在有状态的服务中，因此对网络稳定的标识信息有了更高的要求。Spiderpool 能保证 StatefulSet 的 Pod 在重启、重建场景下，持续获取到相同的 IP 地址。*

## StatefulSet 固定地址

StatefulSet 会在以下一些场景中会出现固定地址的使用：

1. StatefulSet 对应的 Pod 出现的故障重建的情况

2. StatefulSet 在副本数量不变的情况下，删除 Pod 使其重启的情况

此外，StatefulSet 和 Deployment 控制器，对于 IP 地址固定的需求是不一样的：

- 对于 StatefulSet，Pod 副本重启前后，其 Pod 名保持不变，但是 Pod UUID 发生了变化，其是有状态的，应用管理员希望该 Pod 重启前后，仍能分配到相同的 IP 地址。

- 对于 Deployment，Pod 副本重启前后，其 Pod 名字和 Pod UUID 都发生了变化，所以是无状态的，因此并不要新老交替的 Pod 使用相同的 IP 地址，我们可能只希望 Deployment 中所有副本所使用的 IP 是固定在某个 IP 范围内即可。

开源社区的众多 CNI 方案并不能很好的支持 StatefulSet 的固定 IP 的需求。而 Spiderpool 提供的 StatefulSet 方案，能够保证 StatefulSet Pod 在重启、重建场景下，持续获取到相同的 IP 地址。

> - 该功能默认开启。若开启，无任何限制，StatefulSet 可通过有限 IP 地址集合的 IP 池来固化 IP 的范围，但是，无论 StatefulSet 是否使用固定的 IP 池，它的 Pod 都可以持续分配到相同 IP。若关闭，StatefulSet 应用将被当做无状态对待，使用 Helm 安装 Spiderpool 时，可以通过 `--set ipam.enableStatefulSet=false` 关闭。
>
> - 在 StatefulSet 副本经由`缩容`到`扩容`的变化过程中，Spiderpool 并不保证新扩容 Pod 能够获取到之前缩容 Pod 的 IP 地址。
>
> - 在 v0.9.4 及之前的的版本，当 StatefulSet 准备就绪并且其 Pod 正在运行时，即使修改 StatefulSet 注解指定了另一个 IP 池，并重启 Pod，Pod IP 地址也不会生效到新的 IP 池范围内，而是继续使用旧的固定 IP。当大于 0.9.4 版本之后更换 IP 池重启 Pod 会完成 IP 地址切换。

## 实施要求

1. 一套 Kubernetes 集群。

2. 已安装 [Helm](https://helm.sh/docs/intro/install/)。

## 步骤

### 安装 Spiderpool

可参考 [安装](./readme-zh_CN.md) 安装 Spiderpool. 其中，务必确保 helm 安装选项 `ipam.enableStatefulSet=true`

### 安装 CNI 配置

Spiderpool 为简化书写 JSON 格式的 Multus CNI 配置，它提供了 SpiderMultusConfig CR 来自动管理 Multus NetworkAttachmentDefinition CR。如下是创建 Macvlan SpiderMultusConfig 配置的示例：

- master：在此示例用接口 `ens192` 作为 master 的参数。

```bash
MACVLAN_MASTER_INTERFACE="ens192"
MACVLAN_MULTUS_NAME="macvlan-$MACVLAN_MASTER_INTERFACE"

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
EOF
```

在本文示例中，使用如上配置，创建如下的 Macvlan SpiderMultusConfig，将基于它自动生成的 Multus NetworkAttachmentDefinition CR。

```bash
~# kubectl get spidermultusconfigs.spiderpool.spidernet.io -n kube-system
NAME             AGE
macvlan-ens192   26m

~# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system
NAME             AGE
macvlan-ens192   27m
```

### 创建 IPPool

```bash
~# cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: test-ippool
spec:
  subnet: 10.6.0.0/16
  ips:
    - 10.6.168.101-10.6.168.110
EOF
```

### 创建 StatefulSet 应用

以下的示例 Yaml 中，会创建 2 个副本的 StatefulSet 应用，其中：

- `ipam.spidernet.io/ippool`：用于指定 Spiderpool 的 IP 池，Spiderpool 会自动在该池中选择一些 IP 与应用形成绑定，实现 StatefulSet 应用的 IP 固定效果。

- `v1.multus-cni.io/default-network`：为应用创建一张默认网卡。

```bash
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test-sts
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test-sts
  template:
    metadata:
      annotations:
        ipam.spidernet.io/ippool: |-
            {      
              "ipv4": ["test-ippool"]
            }
        v1.multus-cni.io/default-network: kube-system/macvlan-ens192
      labels:
        app: test-sts
    spec:
      containers:
        - name: test-sts
          image: nginx
          imagePullPolicy: IfNotPresent
EOF
```

最终，在 StatefulSet 应用被创建时，Spiderpool 会从指定 IPPool 中随机选择一些 IP 来与应用形成绑定关系。

```bash
~# kubectl get spiderippool
NAME          VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
test-ippool   4         10.6.0.0/16   2                    10               false

~# kubectl get po -l app=test-sts -o wide
NAME         READY   STATUS    RESTARTS   AGE     IP             NODE    NOMINATED NODE   READINESS GATES
test-sts-0   1/1     Running   0          3m13s   10.6.168.105   node2   <none>           <none>
test-sts-1   1/1     Running   0          3m12s   10.6.168.102   node1   <none>           <none>
```

重启 StatefulSet Pod，观察到每个 Pod 的 IP 均不会变化，符合预期。

```bash
~# kubectl get pod | grep "test-sts" | awk '{print $1}' | xargs kubectl delete pod
pod "test-sts-0" deleted
pod "test-sts-1" deleted

~# kubectl get po -l app=test-sts -o wide
NAME         READY   STATUS    RESTARTS   AGE   IP             NODE    NOMINATED NODE   READINESS GATES
test-sts-0   1/1     Running   0          18s   10.6.168.105   node2   <none>           <none>
test-sts-1   1/1     Running   0          17s   10.6.168.102   node1   <none>           <none>
```

扩容、缩容 StatefulSet Pod ，观察每个 Pod 的 IP 变化，符合预期。

```bash
~# kubectl scale deploy test-sts --replicas 3
statefulset.apps/test-sts scaled

~# kubectl get po -l app=test-sts -o wide
NAME         READY   STATUS    RESTARTS   AGE     IP             NODE    NOMINATED NODE   READINESS GATES
test-sts-0   1/1     Running   0          4m58s   10.6.168.105   node2   <none>           <none>
test-sts-1   1/1     Running   0          4m57s   10.6.168.102   node1   <none>           <none>
test-sts-2   1/1     Running   0          4s      10.6.168.109   node2   <none>           <none>

~# kubectl get pod | grep "test-sts" | awk '{print $1}' | xargs kubectl delete pod
pod "test-sts-0" deleted
pod "test-sts-1" deleted
pod "test-sts-2" deleted

~# kubectl get po -l app=test-sts -o wide
NAME         READY   STATUS    RESTARTS   AGE   IP             NODE    NOMINATED NODE   READINESS GATES
test-sts-0   1/1     Running   0          6s    10.6.168.105   node2   <none>           <none>
test-sts-1   1/1     Running   0          4s    10.6.168.102   node1   <none>           <none>
test-sts-2   1/1     Running   0          3s    10.6.168.109   node2   <none>           <none>

~# kubectl scale sts test-sts --replicas 2
statefulset.apps/test-sts scaled

~# kubectl get po -l app=test-sts -o wide
NAME         READY   STATUS    RESTARTS   AGE   IP             NODE    NOMINATED NODE   READINESS GATES
test-sts-0   1/1     Running   0          88s   10.6.168.105   node2   <none>           <none>
test-sts-1   1/1     Running   0          86s   10.6.168.102   node1   <none>           <none>
```

## 总结

Spiderpool 能保证 Statefulset Pod 在重启、重建场景下，持续获取到相同的 IP 地址。能很好的满足 Statefulset 类型控制器的固定 IP 需求。
