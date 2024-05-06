# 卸载指南

[**English**](./uninstall.md) | **简体中文**

本卸载指南适用于在 Kubernetes 上运行的 Spiderpool。如果您有任何疑问，请随时通过 [Spiderpool Community](../../README-zh_CN.md#_6) 联系我们。

## 注意事项

- 在执行卸载之前，请阅读完整的卸载指南以了解所有必要的步骤。

## 卸载

了解正在运行应用，理解卸载 Spiderpool 可能对其他相关组件（如中间件） 产生的影响，请确保完全了解风险后，才开始执行卸载步骤。

1. 通过 `helm ls` 查询集群所安装的 Spiderpool

    ```bash
    helm ls -A | grep -i spiderpool
    ```

2. 通过 `helm  uninstall` 卸载 Spiderpool

    ```bash
    helm uninstall <spiderpool-name> --namespace <spiderpool-namespace>
    ```

    将 `<spiderpool-name>` 替换为要卸载的 Spiderpool 的名称，将 `<spiderpool-namespace>` 替换为 Spiderpool 所在的命名空间。

### v0.10.0 以上版本

在 v0.10.0 之后引入了自动清理 Spiderpool 资源的功能，它通过 `spiderpoolController.cleanup.enabled` 配置项来启用，该值默认为 `true`，您可以通过如下方式验证与 Spiderpool 相关的资源数量是否自动被清理。

```bash
kubectl get spidersubnets.spiderpool.spidernet.io -o name | wc -l 
kubectl get spiderips.spiderpool.spidernet.io -o name | wc -l
kubectl get spiderippools.spiderpool.spidernet.io -o name | wc -l
kubectl get spiderreservedips.spiderpool.spidernet.io -o name | wc -l
kubectl get spiderendpoints.spiderpool.spidernet.io -o name | wc -l
kubectl get spidercoordinators.spiderpool.spidernet.io -o name | wc -l
```

### v0.10.0 以下版本

在低于 v0.10.0 的版本中，由于 Spiderpool 的某些 CR 资源中存在 [finalizers](https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/) ，导致 `helm uninstall` 命令无法清理干净，您需要手动清理。可获取如下清理脚本来完成清理，以确保下次部署 Spiderpool 时不会出现意外错误。

```bash
wget https://raw.githubusercontent.com/spidernet-io/spiderpool/main/tools/scripts/cleanCRD.sh
chmod +x cleanCRD.sh && ./cleanCRD.sh
```
