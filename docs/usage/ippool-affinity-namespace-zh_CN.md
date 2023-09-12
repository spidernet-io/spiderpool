# Namespace affinity of IPPool

**简体中文** | [**English**](./ippool-affinity-namespace.md)

## 介绍

Spiderpool 支持租户亲和性，

## 租户亲和功能

在生产集群中，可能具备不同的多个租户，应用管理员希望 IPPool 能与一个或者多个租户下的应用实现亲和，并且当不属于这些租户的应用使用这些 IP 池时，将会被拒绝分配 IP 地址。

Spiderpool 提供了两种方式实现租户的亲和性功能：

1. 通过设置 IPPool 的 `spec.namespaceAffinity` 字段，实现同一个或者多个租户的亲和性，从而使得满足条件的应用才能够从 IP 池中分配到 IP 地址。

2. 通过设置 IPPool 的 `spec.namespaceName` 字段，实现同一个或者多个租户的亲和性，从而使得满足条件的应用才能够从 IP 池中分配到 IP 地址。


## 实施要求

1. 一套 Kubernetes 集群。

2. 已安装 [Helm](https://helm.sh/docs/intro/install/)。

## 步骤

### 安装 Spiderpool

- 通过 helm 安装 Spiderpool。

```bash
helm repo add spiderpool https://spidernet-io.github.io/spiderpool
helm repo update spiderpool
helm install spiderpool spiderpool/spiderpool --namespace kube-system --set multus.multusCNI.defaultCniCRName="macvlan-ens192" 
```

> 如果您所在地区是中国大陆，可以指定参数 `--set global.imageRegistryOverride=ghcr.m.daocloud.io` ，以帮助您更快的拉取镜像。
>
> 通过 `multus.multusCNI.defaultCniCRName` 指定集群的 Multus clusterNetwork，clusterNetwork 是 Multus 插件的一个特定字段，用于指定 Pod 的默认网络接口。

- 检查安装完成

```bash
~# kubectl get po -n kube-system | grep spiderpool
NAME                                     READY   STATUS      RESTARTS   AGE                                
spiderpool-agent-7hhkz                   1/1     Running     0          13m
spiderpool-agent-kxf27                   1/1     Running     0          13m
spiderpool-controller-76798dbb68-xnktr   1/1     Running     0          13m
spiderpool-init                          0/1     Completed   0          13m
spiderpool-multus-7vkm2                  1/1     Running     0          13m
spiderpool-multus-rwzjn                  1/1     Running     0          13m
```

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

### 创建租户

```bash
~# kubectl create ns test-ns1
namespace/test-ns1 created
~# kubectl create ns test-ns2
namespace/test-ns2 created
```

### 创建 IPPools

使用如下的 Yaml，创建 2 个 IPPool。

- SpiderIPPool 提供了 `namespaceAffinity` 字段，当 Pod 属于某个租户时，尝试从 SpiderIPPool 分配 IP 时，若 Pod 所在租户符合该 namespaceAffinity 设置，则能从该 SpiderIPPool 中成功分配出 IP，否则无法从该 SpiderIPPool 中分配出IP。

- SpiderIPPool 提供了另一种亲和性方式：`namespaceName`，当 `namespaceName` 不为空时，Pod 被创建时，并尝试从 SpiderIPPool 分配 IP 地址, 若 Pod 所在租户符合该 `namespaceName`，则能从该 SpiderIPPool 中成功分配出 IP，若 Pod 所在租户不符合 `namespaceName`，则无法从该 SpiderIPPool 中分配出 IP。当 `namespaceName` 为空时，Spiderpool 对 Pod 不实施任何分配限制。

```bash
~# cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: test-ns1-ippool
spec:
  subnet: 10.6.0.0/16
  ips:
    - 10.6.168.101-10.6.168.110
  namespaceAffinity:
    matchLabels:
      kubernetes.io/metadata.name: test-ns1
---
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: test-ns2-ippool
spec:
  subnet: 10.6.0.0/16
  ips:
    - 10.6.168.111-10.6.168.120
  namespaceName: 
    - test-ns2
EOF
```

### 创建不同租户下的应用

以下的示例 Yaml 中， 会创建 2 个副本的 Deployment 应用 ，其中：

- `ipam.spidernet.io/ippool`：用于指定 Spiderpool 的 IP 池，Spiderpool 会自动在该池中选择一些 IP 与应用形成绑定，实现应用的 IP 固定效果。

- `v1.multus-cni.io/default-network`：为应用创建一张默认网卡。

- `namespace`: 指定应用所在租户。

```bash
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: test-ns1
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      annotations:
        ipam.spidernet.io/ippool: |-
            {      
              "ipv4": ["test-ns1-ippool"]
            }
        v1.multus-cni.io/default-network: kube-system/macvlan-ens192
      labels:
        app: test-app
    spec:
      containers:
      - name: test-app
        image: nginx
        imagePullPolicy: IfNotPresent
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: test-ns2
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      annotations:
        ipam.spidernet.io/ippool: |-
            {      
              "ipv4": ["test-ns2-ippool"]
            }
        v1.multus-cni.io/default-network: kube-system/macvlan-ens192
      labels:
        app: test-app
    spec:
      containers:
      - name: test-app
        image: nginx
        imagePullPolicy: IfNotPresent
EOF
```

Spiderpool 通过 namespaceAffinity 和 namespaceName 实现了 IPPool 与租户之间的亲和性。最终, 在应用创建时，在租户内的应用成功从所亲和的 IPPool 中分配到了 IP 地址。

```bash
~# kubectl get spiderippool
NAME              VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
test-ns1-ippool   4         10.6.0.0/16   1                    10               false
test-ns2-ippool   4         10.6.0.0/16   1                    10               false

~# kubectl get  po -l app=test-app -A  -o wide
NAMESPACE   NAME                        READY   STATUS    RESTARTS   AGE   IP             NODE    NOMINATED NODE   READINESS GATES
test-ns1    test-app-6d8fb6b797-b252q   1/1     Running   0          95s   10.6.168.101   node2   <none>           <none>
test-ns2    test-app-6c5794c554-l498g   1/1     Running   0          95s   10.6.168.114   node2   <none>           <none>
```

创建一个不在上述 `test-ns1`和 `test-ns2` 租户内的应用，Spiderpool 将会拒绝为其分配 IP 地址。

```bash
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app-other
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app-other
  template:
    metadata:
      annotations:
        ipam.spidernet.io/ippool: |-
            {      
              "ipv4": ["test-ns1-ippool","test-ns2-ippool"]
            }
        v1.multus-cni.io/default-network: kube-system/macvlan-ens192
      labels:
        app: test-app-other
    spec:
      containers:
      - name: test-app-other
        image: nginx
        imagePullPolicy: IfNotPresent
EOF
```

不在亲和的租户内，Pod 获得 IP 地址分配失败，符合预期。

```bash
~# kubectl get po -l app=test-app-other -A -o wide
NAMESPACE     NAME                              READY   STATUS              RESTARTS   AGE   IP       NODE    NOMINATED NODE   READINESS GATES
kube-system   test-app-other-6cbdbbddf7-mv8fc   0/1     ContainerCreating   0          31s   <none>   node1   <none>           <none>

~# kubectl describe po -n kube-system   test-app-other-6cbdbbddf7-mv8fc
Warning  FailedCreatePodSandBox  2s    kubelet            Failed to create pod sandbox: rpc error: code = Unknown desc = failed to setup network for sandbox "a96803f44feaf149c1ffa9907d37e9205d4f045472e0bd842ba1b138fdb635b9": plugin type="multus" name="multus-cni-network" failed (add): [kube-system/test-app-other-6cbdbbddf7-mv8fc/e7c98a23-a52e-48c3-bb43-99b5a974e787:macvlan-ens192]: error adding container to network "macvlan-ens192": spiderpool IP allocation error: [POST /ipam/ip][500] postIpamIpFailure  failed to allocate IP addresses in standard mode: no IPPool available, all IPv4 IPPools [test-ns1-ippool test-ns2-ippool] of eth0 filtered out: [unmatched Namespace affinity of IPPool test-ns1-ippool, unmatched Namespace name of IPPool test-ns2-ippool]
```

## 总结

Spiderpool 的 `namespaceAffinity` 与 `namespaceName` 功能实现同一个或者多个租户下的应用实现亲和，且拒绝为不相干租户的应用分配 IP 地址。
