# spiderpool

## Introduction

The Spiderpool is an IP Address Management (IPAM) CNI plugin that assigns IP addresses for kubernetes clusters.

Any Container Network Interface (CNI) plugin supporting third-party IPAM plugins can use the Spiderpool.

## Why Spiderpool

Most overlay CNIs, like
[Cilium](https://github.com/cilium/cilium)
and [Calico](https://github.com/projectcalico/calico),
have a good implementation of IPAM, so the Spiderpool is not intentionally designed for these cases, but maybe integrated with them.

The Spiderpool is intentionally designed to use with underlay network, where administrators can accurately manage each IP.

Currently, in the community, the IPAM plugins such as [whereabout](https://github.com/k8snetworkplumbingwg/whereabouts), [kube-ipam](https://github.com/cloudnativer/kube-ipam),
[static](https://github.com/containernetworking/plugins/tree/main/plugins/ipam/static),
[dhcp](https://github.com/containernetworking/plugins/tree/main/plugins/ipam/dhcp), and [host-local](https://github.com/containernetworking/plugins/tree/main/plugins/ipam/host-local),
few of them could help solve complex underlay-network issues, so we decide to develop the Spiderpool.

BTW, there are also some CNI plugins that could work on the underlay mode, such as [kube-ovn](https://github.com/kubeovn/kube-ovn) and [coil](https://github.com/cybozu-go/coil).
But the Spiderpool provides lots of different features, you could see [Features](#features) for details.

## Features

The Spiderpool provides a large number of different features as follows.

* Based on CRD storage, all operation could be done with kubernetes API-server.

* Support for assigning IP addresses with three options: IPv4-only, IPv6-only, and dual-stack.

* Support for working on the clusters with three options: IPv4-only, IPv6-only, and dual-stack.

* Support for creating multiple ippools.
  Different namespaces and applications could monopolize or share an ippool.

* An application could specify multiple backup ippool resources, in case that IP addresses in an ippool are out of use. Therefore, you neither need to scale up the IP resources in a fixed ippool, nor need to modify the application yaml to change a ippool.

* Support to bind range of IP address only to an applications. No need to hard code an IP list in deployment yaml, which is not easy to modify. With Spiderpool, you only need to set the selector field of ippool and scale up or down the IP resource of an ippool dynamically.

* Support Statefulset pod who will be always assigned same IP addresses.

* Different pods in a single controller could get IP addresses from
  different subnets for an application deployed in different subnets or zones.

* Administrator could safely edit ippool resources, the Spiderpool will help validate the modification and prevent from data race.

* Collect resources in real time, especially for solving IP leakage or slow collection, which may make new pod fail to assign IP addresses.

* Support ranges of CNI plugin who supports third-party IPAM plugins. Especially, the Spiderpool could help much for CNI like [spiderflat](https://github.com/spidernet-io/spiderflat),
  [macvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan),
  [vlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/vlan),
  [ipvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/ipvlan),
  [sriov CNI](https://github.com/k8snetworkplumbingwg/sriov-cni),
  [ovs CNI](https://github.com/k8snetworkplumbingwg/ovs-cni).

* Especially support for [Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni) case to assign IP for multiple interfaces.

* Have a good performance for assigning and collecting IP.

* Support to reserve IP who will not be assigned to any pod.

* Included metrics for looking into IP usage and issues.

* By CidrManager, it could automatically create new ippool for application who needs fixed IP address, and retrieve the ippool when application is deleted. That could reduce the administrator workload.

* Support for both AMD64 and ARM64.

## Install

### Quick Start

```shell
helm install spiderpool spiderpool/spiderpool --wait --namespace kube-system 
```

> NOTICE:
>
> (1). By default, SpiderPool automatically installs Multus, and if your cluster already has Multus installed, you can use "--set multus.multusCNI.install=false" disable installing Multus.
>
> (2). By default, spiderpool creates a corresponding Spidermultusconfig instance for the cluster default CNI (the first CNI configuration file under the /etc/cni/net.d path). If no CNI files are found, SpiderPool creates a Spidermultusconfig instance named default, and you need to manually update the CNI configuration of this instance after installation.
>
> (3). You can manually specify the default CNI of the cluster through "--set multus.multusCNI.defaultCniCRName=<defaultCNIName>". you need to manually create this instance after installation.

### Init default IPPool

```shell
helm repo add spiderpool https://spidernet-io.github.io/spiderpool

IPV4_SUBNET_YOU_EXPECT="172.18.40.0/24"
IPV4_IPRANGES_YOU_EXPECT="172.18.40.40-172.20.40.200"

helm install spiderpool spiderpool/spiderpool --wait --namespace kube-system \
  --set clusterDefaultPool.installIPv4IPPool=true  \
  --set clusterDefaultPool.ipv4Subnet=${IPV4_SUBNET_YOU_EXPECT} \
  --set clusterDefaultPool.ipv4IPRanges={${IPV4_IPRANGES_YOU_EXPECT}}
```

> NOTICE:
>
> (1) if default ippool is installed by helm, please add '--wait' parament in the helm command. Because, the spiderpool will install
> webhook for checking spiderippool CRs, if the spiderpool controller pod is not running, the default ippool will fail to apply and the helm install command fails
> Or else, you could create default ippool after helm installation.
>
> (2) spiderpool-controller pod is running as hostnetwork mode, and it needs take host port,
> it is set with podAntiAffinity to make sure that a node will only run a spiderpool-controller pod.
> so, if you set the replicas number of spiderpool-controller to be bigger than 2, make sure there is enough nodes

## Parameters

### Global parameters

| Name                            | Description                                                                 | Value                                |
| ------------------------------- | --------------------------------------------------------------------------- | ------------------------------------ |
| `global.imageRegistryOverride`  | Global image registry for all images, which is used for offline environment | `""`                                 |
| `global.nameOverride`           | instance name                                                               | `""`                                 |
| `global.clusterDnsDomain`       | cluster dns domain                                                          | `cluster.local`                      |
| `global.commonAnnotations`      | Annotations to add to all deployed objects                                  | `{}`                                 |
| `global.commonLabels`           | Labels to add to all deployed objects                                       | `{}`                                 |
| `global.cniBinHostPath`         | the host path of the IPAM plugin directory.                                 | `/opt/cni/bin`                       |
| `global.cniConfHostPath`        | the host path of the cni config directory                                   | `/etc/cni/net.d`                     |
| `global.ipamUNIXSocketHostPath` | the host path of unix domain socket for ipam plugin                         | `/var/run/spidernet/spiderpool.sock` |
| `global.configName`             | the configmap name                                                          | `spiderpool-conf`                    |
| `global.ciliumConfigMap`        | the cilium's configMap, default is kube-system/cilium-config                | `kube-system/cilium-config`          |

### ipam parameters

| Name                                   | Description                                                                 | Value  |
| -------------------------------------- | --------------------------------------------------------------------------- | ------ |
| `ipam.enableIPv4`                      | enable ipv4                                                                 | `true` |
| `ipam.enableIPv6`                      | enable ipv6                                                                 | `true` |
| `ipam.enableStatefulSet`               | the network mode                                                            | `true` |
| `ipam.enableKubevirtStaticIP`          | the feature to keep kubevirt vm pod static IP                               | `true` |
| `ipam.enableSpiderSubnet`              | SpiderSubnet feature gate.                                                  | `true` |
| `ipam.subnetDefaultFlexibleIPNumber`   | the default flexible IP number of SpiderSubnet feature auto-created IPPools | `1`    |
| `ipam.gc.enabled`                      | enable retrieve IP in spiderippool CR                                       | `true` |
| `ipam.gc.gcAll.intervalInSecond`       | the gc all interval duration                                                | `600`  |
| `ipam.gc.GcDeletingTimeOutPod.enabled` | enable retrieve IP for the pod who times out of deleting graceful period    | `true` |
| `ipam.gc.GcDeletingTimeOutPod.delay`   | the gc delay seconds after the pod times out of deleting graceful period    | `0`    |

### grafanaDashboard parameters

| Name                           | Description                                                                                      | Value   |
| ------------------------------ | ------------------------------------------------------------------------------------------------ | ------- |
| `grafanaDashboard.install`     | install grafanaDashboard for spiderpool. This requires the grafana operator CRDs to be available | `false` |
| `grafanaDashboard.namespace`   | the grafanaDashboard namespace. Default to the namespace of helm instance                        | `""`    |
| `grafanaDashboard.annotations` | the additional annotations of spiderpool grafanaDashboard                                        | `{}`    |
| `grafanaDashboard.labels`      | the additional label of spiderpool grafanaDashboard                                              | `{}`    |

### coordinator parameters

| Name                           | Description                                                                                                                              | Value                |
| ------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------- | -------------------- |
| `coordinator.enabled`          | enable SpiderCoordinator                                                                                                                 | `true`               |
| `coordinator.name`             | the name of the default SpiderCoordinator CR                                                                                             | `default`            |
| `coordinator.mode`             | optional network mode, ["auto","underlay", "overlay", "disabled"]                                                                        | `auto`               |
| `coordinator.podCIDRType`      | Pod CIDR type that should be collected, [ "auto", "cluster", "calico", "cilium", "none" ]                                                | `auto`               |
| `coordinator.detectGateway`    | detect the reachability of the gateway                                                                                                   | `false`              |
| `coordinator.detectIPConflict` | detect IP address conflicts                                                                                                              | `false`              |
| `coordinator.tunePodRoutes`    | tune Pod routes                                                                                                                          | `true`               |
| `coordinator.hijackCIDR`       | Additional subnets that need to be hijacked to the host forward, the default link-local range "169.254.0.0/16" is used for NodeLocal DNS | `["169.254.0.0/16"]` |
| `coordinator.vethLinkAddress`  | configure an link-local address for veth0 device. empty means disable. default is empty. Format is like 169.254.100.1                    | `""`                 |

### rdma parameters

| Name                                                              | Description                                             | Value                                  |
| ----------------------------------------------------------------- | ------------------------------------------------------- | -------------------------------------- |
| `rdma.rdmaSharedDevicePlugin.install`                             | install rdma shared device plugin for macvlan cni       | `false`                                |
| `rdma.rdmaSharedDevicePlugin.name`                                | the name of rdma shared device plugin                   | `spiderpool-rdma-shared-device-plugin` |
| `rdma.rdmaSharedDevicePlugin.image.registry`                      | the image registry of rdma shared device plugin         | `ghcr.io`                              |
| `rdma.rdmaSharedDevicePlugin.image.repository`                    | the image repository of rdma shared device plugin       | `mellanox/k8s-rdma-shared-dev-plugin`  |
| `rdma.rdmaSharedDevicePlugin.image.pullPolicy`                    | the image pullPolicy of rdma shared device plugin       | `IfNotPresent`                         |
| `rdma.rdmaSharedDevicePlugin.image.digest`                        | the image digest of rdma shared device plugin           | `""`                                   |
| `rdma.rdmaSharedDevicePlugin.image.tag`                           | the image tag of rdma shared device plugin              | `latest`                               |
| `rdma.rdmaSharedDevicePlugin.image.imagePullSecrets`              | the image imagePullSecrets of rdma shared device plugin | `[]`                                   |
| `rdma.rdmaSharedDevicePlugin.podAnnotations`                      | the additional annotations                              | `{}`                                   |
| `rdma.rdmaSharedDevicePlugin.podLabels`                           | the additional label                                    | `{}`                                   |
| `rdma.rdmaSharedDevicePlugin.resources.limits.cpu`                | the cpu limit                                           | `300m`                                 |
| `rdma.rdmaSharedDevicePlugin.resources.limits.memory`             | the memory limit                                        | `300Mi`                                |
| `rdma.rdmaSharedDevicePlugin.resources.requests.cpu`              | the cpu requests                                        | `100m`                                 |
| `rdma.rdmaSharedDevicePlugin.resources.requests.memory`           | the memory requests                                     | `50Mi`                                 |
| `rdma.rdmaSharedDevicePlugin.deviceConfig.periodicUpdateInterval` | periodic Update Interval                                | `300`                                  |
| `rdma.rdmaSharedDevicePlugin.deviceConfig.resourcePrefix`         | resource prefix                                         | `spidernet.io`                         |
| `rdma.rdmaSharedDevicePlugin.deviceConfig.resourceName`           | resource Name                                           | `hca_shared_devices`                   |
| `rdma.rdmaSharedDevicePlugin.deviceConfig.rdmaHcaMax`             | rdma Hca Max                                            | `500`                                  |
| `rdma.rdmaSharedDevicePlugin.deviceConfig.vendors`                | rdma device vendors, default to mellanox device         | `15b3`                                 |
| `rdma.rdmaSharedDevicePlugin.deviceConfig.deviceIDs`              | rdma device IDs, default to mellanox device             | `1017`                                 |

### multus parameters

| Name                                          | Description                                                                                                                                                                                                                                                                                                                                                                                      | Value                             |
| --------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | --------------------------------- |
| `multus.enableMultusConfig`                   | enable SpiderMultusConfig                                                                                                                                                                                                                                                                                                                                                                        | `true`                            |
| `multus.multusCNI.install`                    | enable install multus-CNI                                                                                                                                                                                                                                                                                                                                                                        | `true`                            |
| `multus.multusCNI.uninstall`                  | enable remove multus-CNI configuration and binary files on multus-ds pod shutdown. Enable this if you uninstall multus from your cluster. Disable this in the multus upgrade phase to prevent CNI configuration file from being removed, which may cause pods start failure                                                                                                                      | `false`                           |
| `multus.multusCNI.name`                       | the name of spiderpool multus                                                                                                                                                                                                                                                                                                                                                                    | `spiderpool-multus`               |
| `multus.multusCNI.image.registry`             | the multus-CNI image registry                                                                                                                                                                                                                                                                                                                                                                    | `ghcr.io`                         |
| `multus.multusCNI.image.repository`           | the multus-CNI image repository                                                                                                                                                                                                                                                                                                                                                                  | `k8snetworkplumbingwg/multus-cni` |
| `multus.multusCNI.image.pullPolicy`           | the multus-CNI image pullPolicy                                                                                                                                                                                                                                                                                                                                                                  | `IfNotPresent`                    |
| `multus.multusCNI.image.digest`               | the multus-CNI image digest                                                                                                                                                                                                                                                                                                                                                                      | `""`                              |
| `multus.multusCNI.image.tag`                  | the multus-CNI image tag                                                                                                                                                                                                                                                                                                                                                                         | `v3.9.3`                          |
| `multus.multusCNI.image.imagePullSecrets`     | the multus-CNI image imagePullSecrets                                                                                                                                                                                                                                                                                                                                                            | `[]`                              |
| `multus.multusCNI.defaultCniCRName`           | if this value is empty, multus will automatically get default CNI according to the existed CNI conf file in /etc/cni/net.d/, if no cni files found in /etc/cni/net.d, A Spidermultusconfig CR named default will be created, please update the related SpiderMultusConfig for default CNI after installation. The namespace of defaultCniCRName follows with the release namespace of spdierpool | `""`                              |
| `multus.multusCNI.securityContext.privileged` | the securityContext privileged of multus-CNI daemonset pod                                                                                                                                                                                                                                                                                                                                       | `true`                            |
| `multus.multusCNI.extraEnv`                   | the additional environment variables of multus-CNI daemonset pod container                                                                                                                                                                                                                                                                                                                       | `[]`                              |
| `multus.multusCNI.extraVolumes`               | the additional volumes of multus-CNI daemonset pod container                                                                                                                                                                                                                                                                                                                                     | `[]`                              |
| `multus.multusCNI.extraVolumeMounts`          | the additional hostPath mounts of multus-CNI daemonset pod container                                                                                                                                                                                                                                                                                                                             | `[]`                              |
| `multus.multusCNI.log.logLevel`               | the multus-CNI daemonset pod log level                                                                                                                                                                                                                                                                                                                                                           | `debug`                           |
| `multus.multusCNI.log.logFile`                | the multus-CNI daemonset pod log file                                                                                                                                                                                                                                                                                                                                                            | `/var/log/multus.log`             |

### plugins parameters

| Name                             | Description                                                | Value                                        |
| -------------------------------- | ---------------------------------------------------------- | -------------------------------------------- |
| `plugins.installCNI`             | install all cni plugins to each node                       | `false`                                      |
| `plugins.installRdmaCNI`         | install rdma cni used to isolate rdma device for sriov cni | `false`                                      |
| `plugins.installOvsCNI`          | install ovs cni to each node                               | `false`                                      |
| `plugins.image.registry`         | the image registry of plugins                              | `ghcr.io`                                    |
| `plugins.image.repository`       | the image repository of plugins                            | `spidernet-io/spiderpool/spiderpool-plugins` |
| `plugins.image.pullPolicy`       | the image pullPolicy of plugins                            | `IfNotPresent`                               |
| `plugins.image.digest`           | the image digest of plugins                                | `""`                                         |
| `plugins.image.tag`              | the image tag of plugins                                   | `v0.8.0`                                     |
| `plugins.image.imagePullSecrets` | the image imagePullSecrets of plugins                      | `[]`                                         |

### clusterDefaultPool parameters

| Name                                   | Description                                                                  | Value               |
| -------------------------------------- | ---------------------------------------------------------------------------- | ------------------- |
| `clusterDefaultPool.installIPv4IPPool` | install ipv4 spiderpool instance. It is required to set ipam.enableIPv4=true | `false`             |
| `clusterDefaultPool.installIPv6IPPool` | install ipv6 spiderpool instance. It is required to set ipam.enableIPv6=true | `false`             |
| `clusterDefaultPool.ipv4IPPoolName`    | the name of ipv4 spiderpool instance                                         | `default-v4-ippool` |
| `clusterDefaultPool.ipv6IPPoolName`    | the name of ipv6 spiderpool instance                                         | `default-v6-ippool` |
| `clusterDefaultPool.ipv4SubnetName`    | the name of ipv4 spidersubnet instance                                       | `default-v4-subnet` |
| `clusterDefaultPool.ipv6SubnetName`    | the name of ipv6 spidersubnet instance                                       | `default-v6-subnet` |
| `clusterDefaultPool.ipv4Subnet`        | the subnet of ipv4 spiderpool instance                                       | `""`                |
| `clusterDefaultPool.ipv6Subnet`        | the subnet of ipv6 spiderpool instance                                       | `""`                |
| `clusterDefaultPool.ipv4IPRanges`      | the available IP of ipv4 spiderpool instance                                 | `[]`                |
| `clusterDefaultPool.ipv6IPRanges`      | the available IP of ipv6 spiderpool instance                                 | `[]`                |
| `clusterDefaultPool.ipv4Gateway`       | the gateway of ipv4 subnet                                                   | `""`                |
| `clusterDefaultPool.ipv6Gateway`       | the gateway of ipv6 subnet                                                   | `""`                |

### spiderpoolAgent parameters

| Name                                                                                 | Description                                                                                                                                                                      | Value                                      |
| ------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------ |
| `spiderpoolAgent.name`                                                               | the spiderpoolAgent name                                                                                                                                                         | `spiderpool-agent`                         |
| `spiderpoolAgent.binName`                                                            | the binName name of spiderpoolAgent                                                                                                                                              | `/usr/bin/spiderpool-agent`                |
| `spiderpoolAgent.image.registry`                                                     | the image registry of spiderpoolAgent                                                                                                                                            | `ghcr.io`                                  |
| `spiderpoolAgent.image.repository`                                                   | the image repository of spiderpoolAgent                                                                                                                                          | `spidernet-io/spiderpool/spiderpool-agent` |
| `spiderpoolAgent.image.pullPolicy`                                                   | the image pullPolicy of spiderpoolAgent                                                                                                                                          | `IfNotPresent`                             |
| `spiderpoolAgent.image.digest`                                                       | the image digest of spiderpoolAgent, which takes preference over tag                                                                                                             | `""`                                       |
| `spiderpoolAgent.image.tag`                                                          | the image tag of spiderpoolAgent, overrides the image tag whose default is the chart appVersion.                                                                                 | `""`                                       |
| `spiderpoolAgent.image.imagePullSecrets`                                             | the image imagePullSecrets of spiderpoolAgent                                                                                                                                    | `[]`                                       |
| `spiderpoolAgent.serviceAccount.create`                                              | create the service account for the spiderpoolAgent                                                                                                                               | `true`                                     |
| `spiderpoolAgent.serviceAccount.annotations`                                         | the annotations of spiderpoolAgent service account                                                                                                                               | `{}`                                       |
| `spiderpoolAgent.service.annotations`                                                | the annotations for spiderpoolAgent service                                                                                                                                      | `{}`                                       |
| `spiderpoolAgent.service.type`                                                       | the type for spiderpoolAgent service                                                                                                                                             | `ClusterIP`                                |
| `spiderpoolAgent.priorityClassName`                                                  | the priority Class Name for spiderpoolAgent                                                                                                                                      | `system-node-critical`                     |
| `spiderpoolAgent.affinity`                                                           | the affinity of spiderpoolAgent                                                                                                                                                  | `{}`                                       |
| `spiderpoolAgent.extraArgs`                                                          | the additional arguments of spiderpoolAgent container                                                                                                                            | `[]`                                       |
| `spiderpoolAgent.extraEnv`                                                           | the additional environment variables of spiderpoolAgent container                                                                                                                | `[]`                                       |
| `spiderpoolAgent.extraVolumes`                                                       | the additional volumes of spiderpoolAgent container                                                                                                                              | `[]`                                       |
| `spiderpoolAgent.extraVolumeMounts`                                                  | the additional hostPath mounts of spiderpoolAgent container                                                                                                                      | `[]`                                       |
| `spiderpoolAgent.podAnnotations`                                                     | the additional annotations of spiderpoolAgent pod                                                                                                                                | `{}`                                       |
| `spiderpoolAgent.podLabels`                                                          | the additional label of spiderpoolAgent pod                                                                                                                                      | `{}`                                       |
| `spiderpoolAgent.resources.limits.cpu`                                               | the cpu limit of spiderpoolAgent pod                                                                                                                                             | `1000m`                                    |
| `spiderpoolAgent.resources.limits.memory`                                            | the memory limit of spiderpoolAgent pod                                                                                                                                          | `1024Mi`                                   |
| `spiderpoolAgent.resources.requests.cpu`                                             | the cpu requests of spiderpoolAgent pod                                                                                                                                          | `100m`                                     |
| `spiderpoolAgent.resources.requests.memory`                                          | the memory requests of spiderpoolAgent pod                                                                                                                                       | `128Mi`                                    |
| `spiderpoolAgent.tuneSysctlConfig`                                                   | enable to set required sysctl on each node to run spiderpool. refer to [Spiderpool-agent](https://spidernet-io.github.io/spiderpool/dev/reference/spiderpool-agent/) for details | `true`                                     |
| `spiderpoolAgent.securityContext`                                                    | the security Context of spiderpoolAgent pod                                                                                                                                      | `{}`                                       |
| `spiderpoolAgent.httpPort`                                                           | the http Port for spiderpoolAgent, for health checking                                                                                                                           | `5710`                                     |
| `spiderpoolAgent.healthChecking.startupProbe.failureThreshold`                       | the failure threshold of startup probe for spiderpoolAgent health checking                                                                                                       | `60`                                       |
| `spiderpoolAgent.healthChecking.startupProbe.periodSeconds`                          | the period seconds of startup probe for spiderpoolAgent health checking                                                                                                          | `2`                                        |
| `spiderpoolAgent.healthChecking.livenessProbe.failureThreshold`                      | the failure threshold of startup probe for spiderpoolAgent health checking                                                                                                       | `6`                                        |
| `spiderpoolAgent.healthChecking.livenessProbe.periodSeconds`                         | the period seconds of startup probe for spiderpoolAgent health checking                                                                                                          | `10`                                       |
| `spiderpoolAgent.healthChecking.readinessProbe.failureThreshold`                     | the failure threshold of startup probe for spiderpoolAgent health checking                                                                                                       | `3`                                        |
| `spiderpoolAgent.healthChecking.readinessProbe.periodSeconds`                        | the period seconds of startup probe for spiderpoolAgent health checking                                                                                                          | `10`                                       |
| `spiderpoolAgent.prometheus.enabled`                                                 | enable spiderpool agent to collect metrics                                                                                                                                       | `false`                                    |
| `spiderpoolAgent.prometheus.enabledDebugMetric`                                      | enable spiderpool agent to collect debug level metrics                                                                                                                           | `false`                                    |
| `spiderpoolAgent.prometheus.port`                                                    | the metrics port of spiderpool agent                                                                                                                                             | `5711`                                     |
| `spiderpoolAgent.prometheus.serviceMonitor.install`                                  | install serviceMonitor for spiderpool agent. This requires the prometheus CRDs to be available                                                                                   | `false`                                    |
| `spiderpoolAgent.prometheus.serviceMonitor.namespace`                                | the serviceMonitor namespace. Default to the namespace of helm instance                                                                                                          | `""`                                       |
| `spiderpoolAgent.prometheus.serviceMonitor.annotations`                              | the additional annotations of spiderpoolAgent serviceMonitor                                                                                                                     | `{}`                                       |
| `spiderpoolAgent.prometheus.serviceMonitor.labels`                                   | the additional label of spiderpoolAgent serviceMonitor                                                                                                                           | `{}`                                       |
| `spiderpoolAgent.prometheus.serviceMonitor.interval`                                 | represents the interval of spiderpoolAgent serviceMonitor's scraping action                                                                                                      | `10s`                                      |
| `spiderpoolAgent.prometheus.prometheusRule.install`                                  | install prometheusRule for spiderpool agent. This requires the prometheus CRDs to be available                                                                                   | `false`                                    |
| `spiderpoolAgent.prometheus.prometheusRule.namespace`                                | the prometheusRule namespace. Default to the namespace of helm instance                                                                                                          | `""`                                       |
| `spiderpoolAgent.prometheus.prometheusRule.annotations`                              | the additional annotations of spiderpoolAgent prometheusRule                                                                                                                     | `{}`                                       |
| `spiderpoolAgent.prometheus.prometheusRule.labels`                                   | the additional label of spiderpoolAgent prometheusRule                                                                                                                           | `{}`                                       |
| `spiderpoolAgent.prometheus.prometheusRule.enableWarningIPAMAllocationFailure`       | the additional rule of spiderpoolAgent prometheusRule                                                                                                                            | `true`                                     |
| `spiderpoolAgent.prometheus.prometheusRule.enableWarningIPAMAllocationOverTime`      | the additional rule of spiderpoolAgent prometheusRule                                                                                                                            | `true`                                     |
| `spiderpoolAgent.prometheus.prometheusRule.enableWarningIPAMHighAllocationDurations` | the additional rule of spiderpoolAgent prometheusRule                                                                                                                            | `true`                                     |
| `spiderpoolAgent.prometheus.prometheusRule.enableWarningIPAMReleaseFailure`          | the additional rule of spiderpoolAgent prometheusRule                                                                                                                            | `true`                                     |
| `spiderpoolAgent.prometheus.prometheusRule.enableWarningIPAMReleaseOverTime`         | the additional rule of spiderpoolAgent prometheusRule                                                                                                                            | `true`                                     |
| `spiderpoolAgent.debug.logLevel`                                                     | the log level of spiderpool agent [debug, info, warn, error, fatal, panic]                                                                                                       | `info`                                     |
| `spiderpoolAgent.debug.gopsPort`                                                     | the gops port of spiderpool agent                                                                                                                                                | `5712`                                     |

### spiderpoolController parameters

| Name                                                                            | Description                                                                                                                       | Value                                           |
| ------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------- |
| `spiderpoolController.name`                                                     | the spiderpoolController name                                                                                                     | `spiderpool-controller`                         |
| `spiderpoolController.replicas`                                                 | the replicas number of spiderpoolController pod                                                                                   | `1`                                             |
| `spiderpoolController.binName`                                                  | the binName name of spiderpoolController                                                                                          | `/usr/bin/spiderpool-controller`                |
| `spiderpoolController.hostnetwork`                                              | enable hostnetwork mode of spiderpoolController pod. Notice, if no CNI available before spiderpool installation, must enable this | `true`                                          |
| `spiderpoolController.image.registry`                                           | the image registry of spiderpoolController                                                                                        | `ghcr.io`                                       |
| `spiderpoolController.image.repository`                                         | the image repository of spiderpoolController                                                                                      | `spidernet-io/spiderpool/spiderpool-controller` |
| `spiderpoolController.image.pullPolicy`                                         | the image pullPolicy of spiderpoolController                                                                                      | `IfNotPresent`                                  |
| `spiderpoolController.image.digest`                                             | the image digest of spiderpoolController, which takes preference over tag                                                         | `""`                                            |
| `spiderpoolController.image.tag`                                                | the image tag of spiderpoolController, overrides the image tag whose default is the chart appVersion.                             | `""`                                            |
| `spiderpoolController.image.imagePullSecrets`                                   | the image imagePullSecrets of spiderpoolController                                                                                | `[]`                                            |
| `spiderpoolController.serviceAccount.create`                                    | create the service account for the spiderpoolController                                                                           | `true`                                          |
| `spiderpoolController.serviceAccount.annotations`                               | the annotations of spiderpoolController service account                                                                           | `{}`                                            |
| `spiderpoolController.service.annotations`                                      | the annotations for spiderpoolController service                                                                                  | `{}`                                            |
| `spiderpoolController.service.type`                                             | the type for spiderpoolController service                                                                                         | `ClusterIP`                                     |
| `spiderpoolController.priorityClassName`                                        | the priority Class Name for spiderpoolController                                                                                  | `system-node-critical`                          |
| `spiderpoolController.affinity`                                                 | the affinity of spiderpoolController                                                                                              | `{}`                                            |
| `spiderpoolController.extraArgs`                                                | the additional arguments of spiderpoolController container                                                                        | `[]`                                            |
| `spiderpoolController.extraEnv`                                                 | the additional environment variables of spiderpoolController container                                                            | `[]`                                            |
| `spiderpoolController.extraVolumes`                                             | the additional volumes of spiderpoolController container                                                                          | `[]`                                            |
| `spiderpoolController.extraVolumeMounts`                                        | the additional hostPath mounts of spiderpoolController container                                                                  | `[]`                                            |
| `spiderpoolController.podAnnotations`                                           | the additional annotations of spiderpoolController pod                                                                            | `{}`                                            |
| `spiderpoolController.podLabels`                                                | the additional label of spiderpoolController pod                                                                                  | `{}`                                            |
| `spiderpoolController.securityContext`                                          | the security Context of spiderpoolController pod                                                                                  | `{}`                                            |
| `spiderpoolController.resources.limits.cpu`                                     | the cpu limit of spiderpoolController pod                                                                                         | `500m`                                          |
| `spiderpoolController.resources.limits.memory`                                  | the memory limit of spiderpoolController pod                                                                                      | `1024Mi`                                        |
| `spiderpoolController.resources.requests.cpu`                                   | the cpu requests of spiderpoolController pod                                                                                      | `100m`                                          |
| `spiderpoolController.resources.requests.memory`                                | the memory requests of spiderpoolController pod                                                                                   | `128Mi`                                         |
| `spiderpoolController.podDisruptionBudget.enabled`                              | enable podDisruptionBudget for spiderpoolController pod                                                                           | `false`                                         |
| `spiderpoolController.podDisruptionBudget.minAvailable`                         | minimum number/percentage of pods that should remain scheduled.                                                                   | `1`                                             |
| `spiderpoolController.httpPort`                                                 | the http Port for spiderpoolController, for health checking and http service                                                      | `5720`                                          |
| `spiderpoolController.healthChecking.startupProbe.failureThreshold`             | the failure threshold of startup probe for spiderpoolController health checking                                                   | `30`                                            |
| `spiderpoolController.healthChecking.startupProbe.periodSeconds`                | the period seconds of startup probe for spiderpoolController health checking                                                      | `2`                                             |
| `spiderpoolController.healthChecking.livenessProbe.failureThreshold`            | the failure threshold of startup probe for spiderpoolController health checking                                                   | `6`                                             |
| `spiderpoolController.healthChecking.livenessProbe.periodSeconds`               | the period seconds of startup probe for spiderpoolController health checking                                                      | `10`                                            |
| `spiderpoolController.healthChecking.readinessProbe.failureThreshold`           | the failure threshold of startup probe for spiderpoolController health checking                                                   | `3`                                             |
| `spiderpoolController.healthChecking.readinessProbe.periodSeconds`              | the period seconds of startup probe for spiderpoolController health checking                                                      | `10`                                            |
| `spiderpoolController.webhookPort`                                              | the http port for spiderpoolController webhook                                                                                    | `5722`                                          |
| `spiderpoolController.prometheus.enabled`                                       | enable spiderpool Controller to collect metrics                                                                                   | `false`                                         |
| `spiderpoolController.prometheus.enabledDebugMetric`                            | enable spiderpool Controller to collect debug level metrics                                                                       | `false`                                         |
| `spiderpoolController.prometheus.port`                                          | the metrics port of spiderpool Controller                                                                                         | `5721`                                          |
| `spiderpoolController.prometheus.serviceMonitor.install`                        | install serviceMonitor for spiderpool agent. This requires the prometheus CRDs to be available                                    | `false`                                         |
| `spiderpoolController.prometheus.serviceMonitor.namespace`                      | the serviceMonitor namespace. Default to the namespace of helm instance                                                           | `""`                                            |
| `spiderpoolController.prometheus.serviceMonitor.annotations`                    | the additional annotations of spiderpoolController serviceMonitor                                                                 | `{}`                                            |
| `spiderpoolController.prometheus.serviceMonitor.labels`                         | the additional label of spiderpoolController serviceMonitor                                                                       | `{}`                                            |
| `spiderpoolController.prometheus.serviceMonitor.interval`                       | represents the interval of spiderpoolController serviceMonitor's scraping action                                                  | `10s`                                           |
| `spiderpoolController.prometheus.prometheusRule.install`                        | install prometheusRule for spiderpool agent. This requires the prometheus CRDs to be available                                    | `false`                                         |
| `spiderpoolController.prometheus.prometheusRule.namespace`                      | the prometheusRule namespace. Default to the namespace of helm instance                                                           | `""`                                            |
| `spiderpoolController.prometheus.prometheusRule.annotations`                    | the additional annotations of spiderpoolController prometheusRule                                                                 | `{}`                                            |
| `spiderpoolController.prometheus.prometheusRule.labels`                         | the additional label of spiderpoolController prometheusRule                                                                       | `{}`                                            |
| `spiderpoolController.prometheus.prometheusRule.enableWarningIPGCFailureCounts` | the additional rule of spiderpoolController prometheusRule                                                                        | `true`                                          |
| `spiderpoolController.debug.logLevel`                                           | the log level of spiderpool Controller [debug, info, warn, error, fatal, panic]                                                   | `info`                                          |
| `spiderpoolController.debug.gopsPort`                                           | the gops port of spiderpool Controller                                                                                            | `5724`                                          |
| `spiderpoolController.tls.method`                                               | the method for generating TLS certificates. [ provided , certmanager , auto]                                                      | `auto`                                          |
| `spiderpoolController.tls.secretName`                                           | the secret name for storing TLS certificates                                                                                      | `spiderpool-controller-server-certs`            |
| `spiderpoolController.tls.certmanager.certValidityDuration`                     | generated certificates validity duration in days for 'certmanager' method                                                         | `365`                                           |
| `spiderpoolController.tls.certmanager.issuerName`                               | issuer name of cert manager 'certmanager'. If not specified, a CA issuer will be created.                                         | `""`                                            |
| `spiderpoolController.tls.certmanager.extraDnsNames`                            | extra DNS names added to certificate when it's auto generated                                                                     | `[]`                                            |
| `spiderpoolController.tls.certmanager.extraIPAddresses`                         | extra IP addresses added to certificate when it's auto generated                                                                  | `[]`                                            |
| `spiderpoolController.tls.provided.tlsCert`                                     | encoded tls certificate for provided method                                                                                       | `""`                                            |
| `spiderpoolController.tls.provided.tlsKey`                                      | encoded tls key for provided method                                                                                               | `""`                                            |
| `spiderpoolController.tls.provided.tlsCa`                                       | encoded tls CA for provided method                                                                                                | `""`                                            |
| `spiderpoolController.tls.auto.caExpiration`                                    | ca expiration for auto method                                                                                                     | `73000`                                         |
| `spiderpoolController.tls.auto.certExpiration`                                  | server cert expiration for auto method                                                                                            | `73000`                                         |
| `spiderpoolController.tls.auto.extraIpAddresses`                                | extra IP addresses of server certificate for auto method                                                                          | `[]`                                            |
| `spiderpoolController.tls.auto.extraDnsNames`                                   | extra DNS names of server cert for auto method                                                                                    | `[]`                                            |

### spiderpoolInit parameters

| Name                                        | Description                                                                                     | Value                                           |
| ------------------------------------------- | ----------------------------------------------------------------------------------------------- | ----------------------------------------------- |
| `spiderpoolInit.name`                       | the init job for installing default spiderippool                                                | `spiderpool-init`                               |
| `spiderpoolInit.binName`                    | the binName name of spiderpoolInit                                                              | `/usr/bin/spiderpool-init`                      |
| `spiderpoolInit.image.registry`             | the image registry of spiderpoolInit                                                            | `ghcr.io`                                       |
| `spiderpoolInit.image.repository`           | the image repository of spiderpoolInit                                                          | `spidernet-io/spiderpool/spiderpool-controller` |
| `spiderpoolInit.image.pullPolicy`           | the image pullPolicy of spiderpoolInit                                                          | `IfNotPresent`                                  |
| `spiderpoolInit.image.digest`               | the image digest of spiderpoolInit, which takes preference over tag                             | `""`                                            |
| `spiderpoolInit.image.tag`                  | the image tag of spiderpoolInit, overrides the image tag whose default is the chart appVersion. | `""`                                            |
| `spiderpoolInit.image.imagePullSecrets`     | the image imagePullSecrets of spiderpoolInit                                                    | `[]`                                            |
| `spiderpoolInit.extraArgs`                  | the additional arguments of spiderpoolInit container                                            | `[]`                                            |
| `spiderpoolInit.extraEnv`                   | the additional environment variables of spiderpoolInit container                                | `[]`                                            |
| `spiderpoolInit.securityContext`            | the security Context of spiderpoolInit pod                                                      | `{}`                                            |
| `spiderpoolInit.serviceAccount.annotations` | the annotations of spiderpoolInit service account                                               | `{}`                                            |

### sriov network operator parameters

| Name                                       | Description                                                                                           | Value                                                       |
| ------------------------------------------ | ----------------------------------------------------------------------------------------------------- | ----------------------------------------------------------- |
| `sriov.install`                            | install sriov network operator                                                                        | `false`                                                     |
| `sriov.name`                               | the name of sriov network operator                                                                    | `spiderpool-sriov-operator`                                 |
| `sriov.affinity`                           | the affinity                                                                                          | `{}`                                                        |
| `sriov.hostnetwork`                        | enable hostnetwork mode. Notice, if no CNI available before spiderpool installation, must enable this | `true`                                                      |
| `sriov.replicas`                           | the replicas number                                                                                   | `1`                                                         |
| `sriov.resourcePrefix`                     | the resource prefix                                                                                   | `spidernet.io`                                              |
| `sriov.priorityClassName`                  | the priority Class Name                                                                               | `system-node-critical`                                      |
| `sriov.enableAdmissionController`          | enable Admission Controller                                                                           | `false`                                                     |
| `sriov.resources.limits.cpu`               | the cpu limit                                                                                         | `300m`                                                      |
| `sriov.resources.limits.memory`            | the memory limit                                                                                      | `300Mi`                                                     |
| `sriov.resources.requests.cpu`             | the cpu requests                                                                                      | `100m`                                                      |
| `sriov.resources.requests.memory`          | the memory requests                                                                                   | `128Mi`                                                     |
| `sriov.image.registry`                     | registry for all images                                                                               | `ghcr.io`                                                   |
| `sriov.image.pullPolicy`                   | the image pullPolicy for all images                                                                   | `IfNotPresent`                                              |
| `sriov.image.imagePullSecrets`             | the image imagePullSecrets for all images                                                             | `[]`                                                        |
| `sriov.image.operator.repository`          | the image repository                                                                                  | `k8snetworkplumbingwg/sriov-network-operator`               |
| `sriov.image.operator.tag`                 | the image tag                                                                                         | `v1.2.0`                                                    |
| `sriov.image.sriovConfigDaemon.repository` | the image repository                                                                                  | `k8snetworkplumbingwg/sriov-network-operator-config-daemon` |
| `sriov.image.sriovConfigDaemon.tag`        | the image tag                                                                                         | `v1.2.0`                                                    |
| `sriov.image.sriovCni.repository`          | the image repository                                                                                  | `k8snetworkplumbingwg/sriov-cni`                            |
| `sriov.image.sriovCni.tag`                 | the image tag                                                                                         | `v2.7.0`                                                    |
| `sriov.image.ibSriovCni.repository`        | the image repository                                                                                  | `k8snetworkplumbingwg/ib-sriov-cni`                         |
| `sriov.image.ibSriovCni.tag`               | the image tag                                                                                         | `v1.0.2`                                                    |
| `sriov.image.sriovDevicePlugin.repository` | the image repository                                                                                  | `k8snetworkplumbingwg/sriov-network-device-plugin`          |
| `sriov.image.sriovDevicePlugin.tag`        | the image tag                                                                                         | `v3.5.1`                                                    |
| `sriov.image.resourcesInjector.repository` | the image repository                                                                                  | `k8snetworkplumbingwg/network-resources-injector`           |
| `sriov.image.resourcesInjector.tag`        | the image tag                                                                                         | `v1.5`                                                      |
| `sriov.image.webhook.repository`           | the image repository                                                                                  | `k8snetworkplumbingwg/sriov-network-operator-webhook`       |
| `sriov.image.webhook.tag`                  | the image tag                                                                                         | `v1.2.0`                                                    |
