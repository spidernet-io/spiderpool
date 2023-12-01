# Kind Quick Start

[**English**](./get-started-kind.md) | **简体中文**

Kind 是一个使用 Docker 容器节点运行本地 Kubernetes 集群的工具。Spiderpool 提供了安装 Kind 集群的脚本，您可以使用它来部署符合您需求的集群，进行 Spiderpool 的测试与体验。

## 先决条件

* 已安装 [Go](https://go.dev/)

* 克隆 Spiderpool 代码仓库到本地主机上，并进入 Spiderpool 工程的根目录。

    ```bash
    git clone https://github.com/spidernet-io/spiderpool.git && cd spiderpool
    ```

* 通过以下方式获取 Spiderpool 的最新镜像 tag

    ```bash
    ~# SPIDERPOOL_LATEST_IMAGE_TAG=$(curl -s https://api.github.com/repos/spidernet-io/spiderpool/releases | jq -r '.[].tag_name' | head -n 1)
    ```

* 执行 `make dev-doctor`，检查本地主机上的开发工具是否满足部署 Kind 集群与 Spiderpool 的条件，如果缺少组件会为您自动安装。

## Spiderpool 脚本支持的多种安装模式

如果您在中国大陆，安装时可以额外指定参数 `-e E2E_CHINA_IMAGE_REGISTRY=true` ，以帮助您更快的拉取镜像。

### 安装 Spiderpool 在 Underlay CNI（Macvlan） 集群
  
  ```bash
  ~# make e2e_init_underlay -e E2E_SPIDERPOOL_TAG=$SPIDERPOOL_LATEST_IMAGE_TAG
  ```

### 安装 Spiderpool 在 Calico Overlay CNI 集群

  ```bash
  ~# make e2e_init_overlay_calico -e E2E_SPIDERPOOL_TAG=$SPIDERPOOL_LATEST_IMAGE_TAG
  ```

### 在启用了 kube-proxy 的 Cilium 集群中安装 Spiderpool

* 确认操作系统 Kernel 版本号是是否 >= 4.9.17，内核过低时将会导致安装失败，推荐 Kernel 5.10+ 。

  ```bash
  ~# make e2e_init_overlay_cilium -e E2E_SPIDERPOOL_TAG=$SPIDERPOOL_LATEST_IMAGE_TAG
  ```

### 在启用了 ebpf 的 Cilium 集群中安装 Spiderpool

* 确认操作系统 Kernel 版本号是是否 >= 4.9.17，内核过低时将会导致安装失败，推荐 Kernel 5.10+ 。

  ```bash
  ~# make e2e_init_cilium_with_ebpf -e E2E_SPIDERPOOL_TAG=$SPIDERPOOL_LATEST_IMAGE_TAG
  ```

## 验证安装

在 Spiderpool 工程的根目录下执行如下命令，为 kubectl 配置 Kind 集群的 KUBECONFIG。

```bash
~# export KUBECONFIG=$(pwd)/test/.cluster/spider/.kube/config
```

您可以看到类似如下的内容输出：

```bash
~# kubectl get nodes
NAME                   STATUS   ROLES           AGE     VERSION
spider-control-plane   Ready    control-plane   2m29s   v1.26.2
spider-worker          Ready    <none>          2m58s   v1.26.2

~# kubectll get po -n kube-sysem | grep spiderpool
NAME                                           READY   STATUS      RESTARTS   AGE                                
spiderpool-agent-4dr97                         1/1     Running     0          3m
spiderpool-agent-4fkm4                         1/1     Running     0          3m
spiderpool-controller-7864477fc7-c5dk4         1/1     Running     0          3m
spiderpool-controller-7864477fc7-wpgjn         1/1     Running     0          3m
spiderpool-init                                0/1     Completed   0          3m

~# kubectl get spiderippool
NAME                VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
default-v4-ippool   4         172.18.0.0/16             5                    253              true      
default-v6-ippool   6         fc00:f853:ccd:e793::/64   5                    253              true      
vlan100-v4          4         172.100.0.0/16            0                    2559             false
vlan100-v6          6         fd00:172:100::/64         0                    65009            false
vlan100-v4          4         172.200.0.0/16            0                    2559             false
vlan200-v6          6         fd00:172:200::/64         0                    65009            false
```

Spiderpool 提供的快速安装 Kind 集群脚本会自动为您创建一个应用，以验证您的 Kind 集群是否能够正常工作，以下是应用的运行状态：

```bash
~# kubectl get po -l app=test-pod -o wide
NAME                       READY   STATUS    RESTARTS   AGE     IP             NODE            NOMINATED NODE   READINESS GATES
test-pod-856f9689d-876nm   1/1     Running   0          5m34s   172.18.40.63   spider-worker   <none>           <none>
```

您也可以手动创建应用验证 Kind 集群是否能够正常工作，以下命令会创建 1 个副本 Deployment：

```bash
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
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

```bash
~# kubectl get po -l app=test-app -o wide
NAME                        READY   STATUS    RESTARTS   AGE     IP              NODE                   NOMINATED NODE   READINESS GATES
test-app-84d5699474-dbtl5   1/1     Running   0          6m23s   172.18.40.112   spider-control-plane   <none>           <none>
```

通过测试，Kind 集群一切正常，您可以基于它测试与体验 Spiderpool 的更多功能。

## 卸载

* 卸载 Kind 集群

    执行 `make clean` 卸载 Kind 集群。

* 删除测试镜像

    ```bash
    ~# docker rmi -f $(docker images | grep spiderpool | awk '{print $3}')
    ~# docker rmi -f $(docker images | grep multus | awk '{print $3}')
    ```
