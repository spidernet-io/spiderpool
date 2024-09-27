# Upgrading Spiderpool Versions

This document describes breaking changes, as well as how to fix them, that have occurred at given releases.
Please consult the segments from your current release until now before upgrading your spiderpool.

## Upgrade to 0.3.6 from (<=0.3.5)

### Description

1. There's a design flaw for SpiderSubnet feature in auto-created IPPool label.
   The previous label `ipam.spidernet.io/owner-application` corresponding value uses '-' as separative sign.
   For example, we have deployment `ns398-174835790/deploy398-82311862` and the corresponding label value is `Deployment-ns398-174835790-deploy398-82311862`.
   It's very hard to unpack it to trace back what the application namespace and name is.  
   Now, we use '_' rather than '-' as slash for SpiderSubnet feature label `ipam.spidernet.io/owner-application`, and the upper case
   will be like `Deployment_ns398-174835790_deploy398-82311862`.  
   Reference PR: [#1162](https://github.com/spidernet-io/spiderpool/pull/1162)
2. In order to support multiple interfaces with SpiderSubnet feature, we also add one more label for auto-created IPPool.
   The key is `ipam.spidernet.io/interface`, and the value is the corresponding interface name.

### Operation steps

1. Find all auto-created IPPools, their name format is `auto-${appKind}-${appNS}-${appName}-v${ipVersion}-${uid}` such as `auto-deployment-default-demo-deploy-subnet-v4-69d041b98b41`.

2. Replace their label, just like this:

    ```shell
    kubectl patch sp ${auto-pool} --type merge --patch '{"metadata": {"labels": {"ipam.spidernet.io/owner-application": ${AppLabelValue}}}}'
    ```

3. Add one more label

    ```shell
    kubectl patch sp ${auto-pool} --type merge --patch '{"metadata": {"labels": {"ipam.spidernet.io/interface": "eth0"}}}}'
    ```

4. Update your Spiderpool components version and restart them all.

## Upgrade to 0.4.0 from (<0.4.0)

### Description

Due to the architecture adjustment, the SpiderEndpoint.Status.OwnerControllerType property is changed from `None` to `Pod`.

### Operation steps

1. Find all SpiderEndpoint objects that their Status OwnerControllerType is `None`

2. Replace the subresource SpiderEndpoint.Status.OwnerControllerType property from `None` to `Pod`

## Upgrade

This upgrade guide is intended for Spiderpool running on Kubernetes. If you have questions, feel free to ping us on the [Slack channel](https://app.slack.com/client/T08PSQ7BQ/C05JPU3M48P).

NOTE: Read the full upgrade guide to understand all the necessary steps before performing them.

### Upgrade steps

The following steps will describe how to upgrade all of the components from one stable release to a later stable release.

1. Setup the Helm repository and update:

    ```bash
    ~# helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    ~# helm repo update spiderpool
    ...Successfully got an update from the "spiderpool" chart repository
    Update Complete. ⎈Happy Helming!⎈
    ```

2. If the spiderpool-init pod exists it needs to be removed

    Spiderpool-init Pod will help us initialize some environment information. It is very useful to delete it before upgrading.

    ```bash
    ~# kubectl get po -n kube-system spiderpool-init
    NAME              READY   STATUS      RESTARTS   AGE
    spiderpool-init   0/1     Completed   0          49m
    ~# kubectl delete po -n kube-system spiderpool-init
    pod "spiderpool-init" deleted
    ```

3. Update version via helm

    (Optional) Spiderpool chart version has changed, you can get the latest stable version with the following command, unzip the chart package and apply all crds.

    ```bash
    ~# helm search repo spiderpool --versions
    ...
    ~# helm fetch spiderpool/spiderpool --version <upgraded-version>
    ...
    ~# tar -xvf spiderpool-<upgraded-version>.tgz && cd spiderpool/crds
    ~# ls | grep '\.yaml$' | xargs -I {} kubectl apply -f {}
    ```

    To upgrade spiderpool using Helm, you can change `<upgraded-version>` to any stable version.

    ```bash
    helm upgrade --install spiderpool spiderpool/spiderpool --wait --debug --version <upgraded-version> \
        -n kube-system \
    ```

    Running the previous command will overwrite the existing cluster's ConfigMap, so it is important to retain any existing options, either by setting them on the command line or storing them in a YAML file, similar to:

    ```bash
    ~# kubectl get cm -n kube-system spiderpool-conf -oyaml > my-config.yaml
    ```

    The `--reuse-values` flag may only be safely used if the Spiderpool chart version remains unchanged, for example when helm upgrade is used to apply configuration changes without upgrading Spiderpool. Instead, if you want to reuse the values from your existing installation, save the old values in a values file, check the file for any renamed or deprecated values, and then pass it to the helm upgrade command as described above. You can retrieve and save the values from an existing installation with the following command:

    ```bash
    helm get values spiderpool --namespace=kube-system -o yaml > old-values.yaml
    ```

    Helm Optional:

    > Spider v0.7.0 brings some changes. Can be adapted through `--set`
    >
    > 1. In version 0.7.0, spiderpool integrates multus. If you have installed multus, pass `--set multus.multusCNI.install=false` to disable the installation when upgrading.
    > 2. In version 0.7.0, the ipam.enableIPv6 value is changed to true by default. Determine the status of this value in your cluster configuration. If it is false, please change it through `--set ipam.enableIPv6=false`.
    > 3. In version 0.7.0, the default value of ipam.enableSpiderSubnet was changed to false. Determine the status of this value in your cluster configuration. If it is true, change it with `--set ipam.enableSpiderSubnet=true`.

4. verify

    You can verify that the upgraded version matches by running the following command.

    ```bash
    ~# helm list -A
    NAME        NAMESPACE   REVISION    UPDATED                                 STATUS      CHART                           APP VERSION
    spiderpool  kube-system 3           2023-08-31 23:29:28.884386053 +0800 CST deployed    spiderpool-<upgraded-version>   <upgraded-version>
    
    ~# kubectl get po -n kube-system | grep spiderpool
    spiderpool-agent-22z7f                 1/1     Running     0                3m
    spiderpool-agent-cjps5                 1/1     Running     0                3m
    spiderpool-agent-mwpsp                 1/1     Running     0                3m
    spiderpool-controller-647888ff-g5q4n   1/1     Running     0                3m
    spiderpool-init                        0/1     Completed   0                3m

    ~# kubectl get po -n kube-system spiderpool-controller-647888ff-g5q4n -oyaml |grep image

    ~# kubectl get po -n kube-system spiderpool-agent-cjps5 -oyaml |grep image

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

### Upgrading from a version below 0.9.4 (Includes 0.9.4) to a higher version

In versions below 0.9.4, when statefulSet is rapidly scaling up or down, Spiderpool GC may mistakenly reclaim IP addresses in IPPool, causing the same IP to be assigned to multiple Pods in the K8S cluster, resulting in IP address conflicts. This issue has been fixed, see [Fix](https://github.com/spidernet-io/spiderpool/pull/3778), but after the upgrade, the conflicting IP addresses cannot be automatically corrected by Spiderpool. You need to manually restart the Pod with the conflicting IP to assist in resolving the issue. In the new version, there will no longer be an issue with IP conflicts caused by incorrect GC IPs.

### Upgrading from a version below 0.9.5 (Excludes 0.9.5) to a higher version

In versions lower than 0.9.5, the spiderSubnet field in Spiderpool Charts values.yaml changed from `ipam.spidersubnet` to `ipam.spiderSubnet`, so you cannot safely use the `--reuse-values` flag to upgrade from versions < 0.9.5 to 0.9.5 and above. Please modify the values.yaml file or use the `--set ipam.spiderSubnet.enable=true` flag to override the value in the values.yaml file.

### More notes on version upgrades

*TODO.*

## FAQ

Due to your high availability requirements for Spiderpool, you may set multiple replicas of the spiderpool-controller Pod through `--set spiderpoolController.replicas=5` during installation. The Pod of spiderpool-controller will occupy some port addresses of the node by default. The default port Please refer to [System Configuration](./system-requirements-zh_CN.md) for occupancy. If your number of replicas is exactly the same as the number of nodes, then the Pod will fail to start because the node has no available ports during the upgrade. You can refer to the following Modifications can be made in two ways.

1. When executing the upgrade command, you can change the port by appending the helm parameter `--set spiderpoolController.httpPort`, and you can change the port through [helm Values.yaml](https://github.com/spidernet-io/spiderpool/blob/main/charts/spiderpool/values.yaml) and [System Configuration](./system-requirements-zh_CN.md) to check the ports that need to be modified.

2. The type of spiderpool-controller is `Deployment`. You can reduce the number of replicas and restore the number of replicas after the Pod starts normally.
