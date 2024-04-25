# 升级指南

[**English**](./upgrade.md) | **简体中文**

本升级指南适用于在 Kubernetes 上运行的 Spiderpool。如果您有任何疑问，请随时通过 [Spiderpool Community](../../README-zh_CN.md#_6) 联系我们。

## 注意事项

- 在执行升级之前，请阅读完整的升级指南以了解所有必要的步骤。

- 在 Kubernetes 进行 Spiderpool 升级时，Kubernetes 首先将终止已有 Pod，然后拉取新的镜像版本，最后使用新的镜像启动 Pod。为了减少停机时间并防止升级期间发生 `ErrImagePull` 错误，可以参考如下命令，提前拉取对应版本的镜像。

    ```bash
    # 以 docker 为例，请修改 [upgraded-version] 为你升级的版本。
    docker pull ghcr.io/spidernet-io/spiderpool/spiderpool-agent:[upgraded-version]
    docker pull ghcr.io/spidernet-io/spiderpool/spiderpool-controller:[upgraded-version]

    # 如果您是中国大陆用户，可以使用镜像源 ghcr.m.daocloud.io
    docker pull ghcr.m.daocloud.io/spidernet-io/spiderpool/spiderpool-agent:[upgraded-version]
    docker pull ghcr.m.daocloud.io/spidernet-io/spiderpool/spiderpool-controller:[upgraded-version]
    ```

## 步骤

建议每次都升级到 Spiderpool 的最新且被维护的补丁版本。通过 [Stable Releases](../../README-zh_CN.md#_2) 了解受到支持的最新补丁版本。

### 使用 Helm 升级 Spiderpool

1. 确保您已安装 [Helm](https://helm.sh/docs/intro/install/) 。

2. 设置 Helm 存储库并更新

    ```bash
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    helm repo update spiderpool
    ```

3. 删除 spiderpool-init Pod

    `spiderpool-init` Pod 会帮助初始化环境信息，每次运行完毕后其处于 `complete` 状态。在 `helm upgrade` 时，由于 `spiderpool-init` 本质是一个 Pod ，当你要升级到的版本发生了资源变更，将会 Patch 失败，介于可能并不清楚具体的变更，建议升级前手动删除一下 spiderpool-init Pod，避免出现 helm upgrade 失败的情况。

    ```bash
    Error: UPGRADE FAILED: cannot patch "spiderpool-init" with kind Pod: Pod "spiderpool-init" is invalid: spec: Forbidden: pod updates may not change fields other than `spec.containers[*].image`,`spec.initContainers[*].image`,`spec.activeDeadlineSeconds`,`spec.tolerations` (only additions to existing tolerations),`spec.terminationGracePeriodSeconds` (allow it to be set to 1 if it was previously negative)
    ```

4. helm upgrade 升级

    ```bash
    # -n 指定你 Spiderpool 所在命名空间，并修改 [upgraded-version] 为你要升级到的版本。
    helm upgrade spiderpool spiderpool/spiderpool -n kube-system --version [upgraded-version]
    ```

### 配置升级

您可以通过 `--set` 在升级时去更新 Spiderpool 配置，可用的 values 参数，请查看 [values](https://github.com/spidernet-io/spiderpool/tree/main/charts/spiderpool/README.md) 说明文档。 以下示例展示了如何开启 Spiderpool 的 [SpiderSubnet 功能](../spider-subnet-zh_CN.md)

```bash
helm upgrade spiderpool spiderpool/spiderpool -n kube-system --version [upgraded-version] --set ipam.spidersubnet.enable=true
```

同时您也可以使用 `--reuse-values` 重用上一个 release 的值并合并来自命令行的任何覆盖。但仅当 Spiderpool chart 版本保持不变时，才可以安全地使用 `--reuse-values` 标志，例如，当使用 helm upgrade 来更改 Spiderpool 配置而不升级 Spiderpool 组件。 `--reuse-values` 使用，参考如下示例：

```bash
helm upgrade spiderpool spiderpool/spiderpool -n kube-system --version [upgraded-version] --set ipam.spidersubnet.enable=true --reuse-values
```

相反，如果 Spiderpool chart 版本发生了变化，您想重用现有安装中的值，请将旧值保存在值文件中，检查该文件中是否有任何重命名或弃用的值，然后将其传递给 helm upgrade 命令，您可以使用以下命令检索并保存现有安装中的值：

```bash
helm get values spiderpool --namespace=kube-system -o yaml > old-values.yaml
helm upgrade spiderpool spiderpool/spiderpool -n kube-system --version [upgraded-version] -f old-values.yaml
```

### 升级回滚

有时由于升级过程中遗漏了某个步骤或出现问题，可能需要回滚升级。 要回滚请参考运行如下命令：

```bash
helm history spiderpool --namespace=kube-system
helm rollback spiderpool [REVISION] --namespace=kube-system
```

## 版本具体说明

下列的升级注意事项，将随着新版本的发布滚动更新，它们将存在优先级关系（从旧到新），您的当前版本满足任何一项，在进行升级时，需要依次检查从该项到最新的每一个注意事项。

### 低于 0.3.6（不包含 0.3.6）升级到更高版本的注意事项

在低于 0.3.6 的版本中，使用了 `-` 作为 [SpiderSubnet](../spider-subnet-zh_CN.md) 自动池名称的分隔符。最终很难将其解压以追溯自动池所对应的应用程序的命名空间和名称，在这些版本中 SpiderSubnet 功能是存在设计缺陷的，在最新的补丁版本中，对此做了修改与优化，并且在 0.3.6 往后的版本中支持了 SpiderSubnet 功能的多个网络接口。如上所述，新版本中新建的自动池名称已发生了改变，例如，应用 `kube-system/test-app` 对应的 IPv4 自动池为 `auto4-test-app-eth0-40371`。 同时自动池中被标记了如下的一些 label。

```bash
metadata:
  labels:
    ipam.spidernet.io/interface: eth0
    ipam.spidernet.io/ip-version: IPv4
    ipam.spidernet.io/ippool-cidr: 172-100-0-0-16
    ipam.spidernet.io/ippool-reclaim: "true"
    ipam.spidernet.io/owner-application-gv: apps_v1
    ipam.spidernet.io/owner-application-kind: DaemonSet
    ipam.spidernet.io/owner-application-name: test-app
    ipam.spidernet.io/owner-application-namespace: kube-system
    ipam.spidernet.io/owner-application-uid: 2f78ccdd-398e-49e6-a85b-40371db6fdbd
    ipam.spidernet.io/owner-spider-subnet: vlan100-v4
spec:
  podAffinity:
    matchLabels:
      ipam.spidernet.io/app-api-group: apps
      ipam.spidernet.io/app-api-version: v1
      ipam.spidernet.io/app-kind: DaemonSet
      ipam.spidernet.io/app-name: test-app
      ipam.spidernet.io/app-namespace: kube-system
```

低于 0.3.6 升级最新补丁版本属于不兼容升级，如果启用了 SpiderSubnet 功能，为使存量自动池可用，需要为存量的自动池增加如上所述的一系列标签，操作如下：

```bash
kubectl patch sp ${auto-pool} --type merge --patch '{"metadata": {"labels": {"ipam.spidernet.io/owner-application-name": "test-app"}}}'
kubectl patch sp ${auto-pool} --type merge --patch '{"metadata": {"labels": {"ipam.spidernet.io/owner-application-namespace": "kube-system"}}}'
...
```

同时 SpiderSubnet 支持了多网络接口，需要为自动池增加对应的网络接口 `label`，如下：

```bash
kubectl patch sp ${auto-pool} --type merge --patch '{"metadata": {"labels": {"ipam.spidernet.io/interface": "eth0"}}}}'
```

### 低于 0.4.0（不包含 0.4.0）升级到更高版本的注意事项

由于架构调整，`SpiderEndpoint.Status.OwnerControllerType` 属性从 `None` 更改为 `Pod`。 故查找所有 `Status.OwnerControllerType` 为 `None` 的 SpiderEndpoint 对象，将 `SpiderEndpoint.Status.OwnerControllerType` 属性从 `None` 替换为 `Pod`。

### 低于 0.5.0（包含 0.5.0）升级到更高版本的注意事项

在高于 0.5.0 的版本中，新增了 [SpiderMultusConfig](../spider-multus-config-zh_CN.md) 和 [Coordinator](../../concepts/coordinator-zh_CN.md) 功能。但由于 helm upgrade 升级时，无法自动去安装对应的 CRD：`spidercoordinators.spiderpool.spidernet.io` 和 `spidermultusconfigs.spiderpool.spidernet.io`。故在升级前，您可以通过以下命令获取最新的稳定版本，并解压 chart 包并应用所有 CRD。

```bash
~# helm search repo spiderpool --versions
# 请替换 [upgraded-version] 为要升级到的版本。
~# helm fetch spiderpool/spiderpool --version [upgraded-version]
~# tar -xvf spiderpool-[upgraded-version].tgz && cd spiderpool/crds
~# ls | grep '\.yaml$' | xargs -I {} kubectl apply -f {}
```

### 低于 0.7.3（包含 0.7.3）升级到更高版本的注意事项

在 0.7.3 以下版本中，Spiderpool 会启用一组 DaemonSet: `spiderpool-multus` 来管理 Multus 相关配置。在更高版本中，弃用了该 DaemonSet，并且将 Multus 的配置移到了 `spiderpool-agent` 中纳管，同时新增了`卸载时自动清理 Muluts 配置`的功能，它默认是启用的。在升级时通过 `--set multus.multusCNI.uninstall=false` 禁用它，避免在升级阶段 CNI 配置文件、CRD 等被删除，从而导致 Pod 创建失败。

### 低于 0.9.0 (不包含 0.9.0) 升级到最高版本的注意事项

由于在 0.9.0 的版本中，我们给 [SpiderCoordinator CRD](./../../reference/crd-spidercoordinator.md) 补充了 `txQueueLen` 字段，但由于执行升级时 Helm 不支持升级或删除 CRD，因此在升级前需要你手动更新一下 CRD。(建议越过 0.9.0 直接升级至 0.9.1 版本)

### 更多版本升级的注意事项

*TODO.*

## FAQ

由于您对 Spiderpool 高可用的要求，您在安装时可能会通过 `--set spiderpoolController.replicas=5` 设置 spiderpool-controller Pod 多副本，spiderpool-controller 的 Pod 会默认占用节点的一些端口地址，默认端口占用参考[系统配置](./system-requirements-zh_CN.md)，如果您的副本数与节点数刚好就相同，那么在升级时 Pod 将会因为节点无可用端口而启动失败，您可以参考如下两种方式进行修改。

1. 执行升级命令时，您可以通过附加 helm 参数 `--set spiderpoolController.httpPort` 对端口更改，可以通过 [helm Values.yaml](https://github.com/spidernet-io/spiderpool/blob/main/charts/spiderpool/values.yaml) 和[系统配置](./system-requirements-zh_CN.md) 查看需要修改的端口。

2. spiderpool-controller 的控制器类型是 `Deployment`, 您可以通过缩减副本数，让 Pod 正常启动后，再恢复副本数。
