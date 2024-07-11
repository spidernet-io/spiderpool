# Uninstall Guide

**English** | [**简体中文**](./uninstall-zh_CN.md)

This uninstall guide is intended for Spiderpool running on Kubernetes. If you have questions, feel free to ping us on [Spiderpool Community](../../README.md#community).

## Warning

- Read the full uninstall guide to understand all the necessary steps before performing them.

## Uninstall Spiderpool

Understand the running application and understand the impact that uninstalling Spiderpool may have on other related components (such as middleware). Please make sure you fully understand the risks before starting the uninstallation steps.

1. Query the Spiderpool installed in the cluster through `helm ls`

     ```bash
     helm ls -A | grep -i spiderpool
     ```

2. Uninstall Spiderpool via `helm uninstall`

     ```bash
     helm uninstall <spiderpool-name> --namespace <spiderpool-namespace>
     ```

     Replace `<spiderpool-name>` with the name of the Spiderpool you want to uninstall and `<spiderpool-namespace>` with the namespace of the Spiderpool.

### Above v1.0.0

The function of automatically cleaning Spiderpool resources was introduced after v1.0.0. It is enabled through the `spiderpoolController.cleanup.enabled` configuration item. The value defaults to `true`. You can verify whether the number of resources related to Spiderpool is automatically cleared as follows.

```bash
kubectl get spidersubnets.spiderpool.spidernet.io -o name | wc -l
kubectl get spiderips.spiderpool.spidernet.io -o name | wc -l
kubectl get spiderippools.spiderpool.spidernet.io -o name | wc -l
kubectl get spiderreservedips.spiderpool.spidernet.io -o name | wc -l
kubectl get spiderendpoints.spiderpool.spidernet.io -o name | wc -l
kubectl get spidercoordinators.spiderpool.spidernet.io -o name | wc -l
```

### Below v1.0.0

In versions lower than v1.0.0, Some CR resources having [finalizers](https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/) prevents complete cleanup via `helm uninstall`. You can download the cleaning script below to perform the necessary cleanup and avoid any unexpected errors during future deployments of Spiderpool.

```bash
wget https://raw.githubusercontent.com/spidernet-io/spiderpool/main/tools/scripts/cleanCRD.sh
chmod +x cleanCRD.sh && ./cleanCRD.sh
```

## FAQ

Spiderpool was not deleted using the `helm uninstall` method, but through `kubectl delete <spiderpool deployed namespace>`, which resulted in resource uninstallation residues, thus affecting the new installation. You need to clean it up manually. You can get the following cleanup script to complete the cleanup to ensure that there will be no unexpected errors when you deploy Spiderpool next time.

```bash
wget https://raw.githubusercontent.com/spidernet-io/spiderpool/main/tools/scripts/cleanCRD.sh
chmod +x cleanCRD.sh && ./cleanCRD.sh
```
