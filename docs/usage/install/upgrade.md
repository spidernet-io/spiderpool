# Upgrade Guide

**English** | [**简体中文**](./upgrade-zh_CN.md)

This upgrade guide is intended for Spiderpool running on Kubernetes. If you have questions, feel free to ping us on [Spiderpool Community](../../README.md#stable-releases).

## Warning

- Read the full upgrade guide to understand all the necessary steps before performing them.

- When rolling out an upgrade with Kubernetes, Kubernetes will first terminate the pod followed by pulling the new image version and then finally spin up the new image. In order to reduce the downtime of the agent and to prevent ErrImagePull errors during upgrade. You can refer to the following command to pull the corresponding version of the image in advance.

    ```bash
    # Taking docker as an example, please modify [upgraded-version] to your upgraded version.
    docker pull ghcr.io/spidernet-io/spiderpool/spiderpool-agent:[upgraded-version]
    docker pull ghcr.io/spidernet-io/spiderpool/spiderpool-controller:[upgraded-version]

    # If you are mainland user who is not available to access ghcr.io, you can use the mirror source ghcr.m.daocloud.io
    docker pull ghcr.m.daocloud.io/spidernet-io/spiderpool/spiderpool-agent:[upgraded-version]
    docker pull ghcr.m.daocloud.io/spidernet-io/spiderpool/spiderpool-controller:[upgraded-version]
    ```

## Steps

It is recommended to always upgrade to the latest and maintained patch version of Spiderpool. Check [Stable Releases](../../README.md#community) to learn about the latest supported patch versions.

### Upgrading Spiderpool via Helm

1. Make sure you have [Helm](https://helm.sh/docs/intro/install/) installed.

2. Setup Helm repository and update

    ```bash
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    helm repo update spiderpool
    ```

3. Remove spiderpool-init Pod

    `spiderpool-init` Pod will help initialize environment information, and it will be in `complete` state after each run. During `helm upgrade`, since `spiderpool-init` is essentially a Pod, patching some resources will fail. So delete it via `kubectl delete spiderpool-init` before upgrading.

    ```bash
    Error: UPGRADE FAILED: cannot patch "spiderpool-init" with kind Pod: Pod "spiderpool-init" is invalid: spec: Forbidden: pod updates may not change fields other than `spec.containers[*].image`,`spec.initContainers[*].image`,`spec.activeDeadlineSeconds`,`spec.tolerations` (only additions to existing tolerations),`spec.terminationGracePeriodSeconds` (allow it to be set to 1 if it was previously negative)
    ```

4. Upgrade via `helm upgrade`

    ```bash
    # -n specifies the namespace where your Spiderpool is located, and modify [upgraded-version] to the version you want to upgrade to.
    helm upgrade spiderpool spiderpool/spiderpool -n kube-system --version [upgraded-version]
    ```

### Configuration upgrade

You can use `--set` to update the Spiderpool configuration when upgrading. For available values parameters, please see the [values](https://github.com/spidernet-io/spiderpool/tree/main/charts/spiderpool/README.md) documentation. The following example shows how to enable Spiderpool's [SpiderSubnet function](../spider-subnet.md)

```bash
helm upgrade spiderpool spiderpool/spiderpool -n kube-system --version [upgraded-version] --set ipam.spidersubnet.enable=true
```

You can also use `--reuse-values` to reuse the values from the previous release and merge any overrides from the command line. However, it is only safe to use the `--reuse-values` flag if the Spiderpool chart version remains unchanged, e.g. when using helm upgrade to change the Spiderpool configuration without upgrading the Spiderpool components. For `--reuse-values` usage, see the following example:

```bash
helm upgrade spiderpool spiderpool/spiderpool -n kube-system --version [upgraded-version] --set ipam.spidersubnet.enable=true --reuse-values
```

Conversely, if the Spiderpool chart version has changed and you want to reuse the values from the existing installation, save the old values in a values file, check that file for any renamed or deprecated values, and pass it to helm upgrade command, you can retrieve and save values from existing installations using.

```bash
helm get values spiderpool --namespace=kube-system -o yaml > old-values.yaml
helm upgrade spiderpool spiderpool/spiderpool -n kube-system --version [upgraded-version] -f old-values.yaml
```

### Rolling Back

Occasionally, it may be necessary to undo the rollout because a step was missed or something went wrong during upgrade. To undo the rollout run:

```bash
helm history spiderpool --namespace=kube-system
helm rollback spiderpool [REVISION] --namespace=kube-system
```

## Version Specific Notes

The following upgrade notes will be updated on a rolling basis with the release of new versions. They will have a priority relationship (from old to new). If your current version meets any one of them, when upgrading, you need to check in order from that item to Latest on every note.

### Upgrading from a version below 0.3.6 (Excludes 0.3.6) to a higher version

In versions lower than 0.3.6, `-` is used as a separator for [SpiderSubnet](../spider-subnet.md) delimiter for autopool names. It was ultimately difficult to extract it to trace the namespace and name of the application to which the autopool corresponded. The SpiderSubnet functionality in these releases was flawed by design, and has been modified and optimised in the latest patch releases, as well as supporting multiple network interfaces for the SpiderSubnet functionality in releases from 0.3.6 onwards. As mentioned above, the names of the new auto pools created in the new release have been changed, e.g., the IPv4 auto pool corresponding to application `kube-system/test-app` is `auto4-test-app-eth0-40371`. At the same time, the auto pool is marked with some labels as follows.

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

Upgrading below 0.3.6 to the latest patch version is an incompatible upgrade. If the SpiderSubnet feature is enabled, you will need to add a series of tags as described above to the stock auto pool in order to make it available to the stock auto pool, as follows:

```bash
kubectl patch sp ${auto-pool} --type merge --patch '{"metadata": {"labels": {"ipam.spidernet.io/owner-application-name": "test-app"}}}'
kubectl patch sp ${auto-pool} --type merge --patch '{"metadata": {"labels": {"ipam.spidernet.io/owner-application-namespace": "kube-system"}}}'
...
```

SpiderSubnet supports multiple network interfaces, you need to add the corresponding network interface `label` for the auto pool as follows:

```bash
kubectl patch sp ${auto-pool} --type merge --patch '{"metadata": {"labels": {"ipam.spidernet.io/interface": "eth0"}}}}'
```

### Upgrading from a version below 0.4.0 (Excludes 0.4.0) to a higher version

Due to architecture adjustment, `SpiderEndpoint.Status.OwnerControllerType` property is changed from `None` to `Pod`. Therefore, find all SpiderEndpoint objects with `Status.OwnerControllerType` of `None` and replace the `SpiderEndpoint.Status.OwnerControllerType` property from `None` to `Pod`.

### Upgrading from a version below 0.5.0 (Includes 0.5.0) to a higher version

In versions higher than 0.5.0, the [SpiderMultusConfig](../spider-multus-config.md) and [Coordinator](../../concepts/coordinator.md) functions are added. However, due to helm upgrade, the corresponding CRDs cannot be automatically installed: `spidercoordinators.spiderpool.spidernet.io` and `spidermultusconfigs.spiderpool.spidernet.io`. Therefore, before upgrading, you can obtain the latest stable version through the following commands, decompress the chart package and apply all CRDs.

```bash
~# helm search repo spiderpool --versions
# Please replace [upgraded-version] with the version you want to upgrade to.
~# helm fetch spiderpool/spiderpool --version [upgraded-version]
~# tar -xvf spiderpool-[upgraded-version].tgz && cd spiderpool/crds
~# ls | grep '\.yaml$' | xargs -I {} kubectl apply -f {}
```

### Upgrading from a version below 0.7.3 (Includes 0.7.3) to a higher version

In versions below 0.7.3, Spiderpool will enable a set of DaemonSet: `spiderpool-multus` to manage Multus related configurations. In later versions, the DaemonSet was deprecated, and the Muluts configuration was moved to `spiderpool-agent` for management. At the same time, the function of `automatically cleaning up the Muluts configuration during uninstallation` was added, which is enabled by default. Disable it by `--set multus.multusCNI.uninstall=false` when upgrading to avoid CNI configuration files, CRDs, etc. being deleted during the upgrade phase, causing Pod creation to fail.

### Upgrading from a version below 0.9.0 (Excludes 0.9.0) to a higher version

Due to the addition of the `txQueueLen` field to the [SpiderCoordinator CRD](./../../reference/crd-spidercoordinator.md) in version 0.9.0, you need to manually update the CRD before upgrading as Helm does not support upgrading or deleting CRDs during the upgrade process.(We suggest skipping version 0.9.0 and upgrading directly to version 0.9.1)

### More notes on version upgrades

*TODO.*

## FAQ

Due to your high availability requirements for Spiderpool, you may set multiple replicas of the spiderpool-controller Pod through `--set spiderpoolController.replicas=5` during installation. The Pod of spiderpool-controller will occupy some port addresses of the node by default. The default port Please refer to [System Configuration](./system-requirements-zh_CN.md) for occupancy. If your number of replicas is exactly the same as the number of nodes, then the Pod will fail to start because the node has no available ports during the upgrade. You can refer to the following Modifications can be made in two ways.

1. When executing the upgrade command, you can change the port by appending the helm parameter `--set spiderpoolController.httpPort`, and you can change the port through [helm Values.yaml](https://github.com/spidernet-io/spiderpool/blob/main/charts/spiderpool/values.yaml) and [System Configuration](./system-requirements-zh_CN.md) to check the ports that need to be modified.

2. The type of spiderpool-controller is `Deployment`. You can reduce the number of replicas and restore the number of replicas after the Pod starts normally.
