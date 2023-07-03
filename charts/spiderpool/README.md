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

```shell
helm repo add spiderpool https://spidernet-io.github.io/spiderpool

IPV4_SUBNET_YOU_EXPECT="172.18.40.0/24"
IPV4_IPRANGES_YOU_EXPECT="172.18.40.40-172.20.40.200"

helm install spiderpool spiderpool/spiderpool --wait --namespace kube-system \
  --set spiderpoolController.tls.method=auto \
  --set ipam.enableIPv4=true --set ipam.enableIPv6=false \
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

| Name                            | Description                                         | Value                                |
| ------------------------------- | --------------------------------------------------- | ------------------------------------ |
| `global.imageRegistryOverride`  | Global Docker image registry                        | `""`                                 |
| `global.nameOverride`           | instance name                                       | `""`                                 |
| `global.clusterDnsDomain`       | cluster dns domain                                  | `cluster.local`                      |
| `global.commonAnnotations`      | Annotations to add to all deployed objects          | `{}`                                 |
| `global.commonLabels`           | Labels to add to all deployed objects               | `{}`                                 |
| `global.ipamBinHostPath`        | the host path of the IPAM plugin directory.         | `/opt/cni/bin`                       |
| `global.ipamUNIXSocketHostPath` | the host path of unix domain socket for ipam plugin | `/var/run/spidernet/spiderpool.sock` |
| `global.configName`             | the configmap name                                  | `spiderpool-conf`                    |


### ipam parameters

| Name                                   | Description                                                                                      | Value   |
| -------------------------------------- | ------------------------------------------------------------------------------------------------ | ------- |
| `ipam.enableIPv4`                      | enable ipv4                                                                                      | `true`  |
| `ipam.enableIPv6`                      | enable ipv6                                                                                      | `true`  |
| `ipam.enableStatefulSet`               | the network mode                                                                                 | `true`  |
| `ipam.enableSpiderSubnet`              | SpiderSubnet feature gate.                                                                       | `false` |
| `ipam.subnetDefaultFlexibleIPNumber`   | the default flexible IP number of SpiderSubnet feature auto-created IPPools                      | `1`     |
| `ipam.gc.enabled`                      | enable retrieve IP in spiderippool CR                                                            | `true`  |
| `ipam.gc.gcAll.intervalInSecond`       | the gc all interval duration                                                                     | `600`   |
| `ipam.gc.GcDeletingTimeOutPod.enabled` | enable retrieve IP for the pod who times out of deleting graceful period                         | `true`  |
| `ipam.gc.GcDeletingTimeOutPod.delay`   | the gc delay seconds after the pod times out of deleting graceful period                         | `0`     |
| `grafanaDashboard.install`             | install grafanaDashboard for spiderpool. This requires the grafana operator CRDs to be available | `false` |
| `grafanaDashboard.namespace`           | the grafanaDashboard namespace. Default to the namespace of helm instance                        | `""`    |
| `grafanaDashboard.annotations`         | the additional annotations of spiderpool grafanaDashboard                                        | `{}`    |
| `grafanaDashboard.labels`              | the additional label of spiderpool grafanaDashboard                                              | `{}`    |


### coordinator parameters

| Name                           | Description                                                               | Value      |
| ------------------------------ | ------------------------------------------------------------------------- | ---------- |
| `coordinator.enabled`          | enable SpiderCoordinator                                                  | `true`     |
| `coordinator.name`             | the name of the default SpiderCoordinator CR                              | `default`  |
| `coordinator.tuneMode`         | optional network mode, ["underlay", "overlay", "disabled"]                | `underlay` |
| `coordinator.podCIDRType`      | Pod CIDR type that should be collected, [ "cluster", "calico", "cilium" ] | `cluster`  |
| `coordinator.detectGateway`    | detect the reachability of the gateway                                    | `true`     |
| `coordinator.detectIPConflict` | detect IP address conflicts                                               | `true`     |
| `coordinator.tunePodRoutes`    | tune Pod routes                                                           | `true`     |


### multus parameters

| Name                                          | Description                                                                                                                                                                                                           | Value                             |
| --------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------- |
| `multus.enableMultusConfig`                   | enable SpiderMultusConfig                                                                                                                                                                                             | `true`                            |
| `multus.multusCNI.install`                    | enable install multus-CNI                                                                                                                                                                                             | `true`                            |
| `multus.multusCNI.name`                       | the name of spiderpool multus                                                                                                                                                                                         | `spiderpool-multus`               |
| `multus.multusCNI.image.registry`             | the multus-CNI image registry                                                                                                                                                                                         | `ghcr.io`                         |
| `multus.multusCNI.image.repository`           | the multus-CNI image repository                                                                                                                                                                                       | `k8snetworkplumbingwg/multus-cni` |
| `multus.multusCNI.image.pullPolicy`           | the multus-CNI image pullPolicy                                                                                                                                                                                       | `IfNotPresent`                    |
| `multus.multusCNI.image.digest`               | the multus-CNI image digest                                                                                                                                                                                           | `""`                              |
| `multus.multusCNI.image.tag`                  | the multus-CNI image tag                                                                                                                                                                                              | `v3.9.3`                          |
| `multus.multusCNI.image.imagePullSecrets`     | the multus-CNI image imagePullSecrets                                                                                                                                                                                 | `[]`                              |
| `multus.multusCNI.defaultCniCRName`           | please create the related NetworkAttachmentDefinition for default CNI after installation, if this value is empty, multus will automatically get default CNI according to the existed CNI conf file in /etc/cni/net.d/ | `""`                              |
| `multus.multusCNI.resources.limits.cpu`       | the cpu limit of multus-CNI daemonset pod                                                                                                                                                                             | `100m`                            |
| `multus.multusCNI.resources.limits.memory`    | the memory limit of multus-CNI daemonset pod                                                                                                                                                                          | `50Mi`                            |
| `multus.multusCNI.resources.requests.cpu`     | the cpu requests of multus-CNI daemonset pod                                                                                                                                                                          | `100m`                            |
| `multus.multusCNI.resources.requests.memory`  | the memory requests of multus-CNI daemonset pod                                                                                                                                                                       | `50Mi`                            |
| `multus.multusCNI.podAnnotations`             | the additional annotations of multus-CNI daemonset pod                                                                                                                                                                | `{}`                              |
| `multus.multusCNI.podLabels`                  | the additional label of multus-CNI daemonset pod                                                                                                                                                                      | `{}`                              |
| `multus.multusCNI.securityContext.privileged` | the securityContext privileged of multus-CNI daemonset pod                                                                                                                                                            | `true`                            |
| `multus.multusCNI.extraEnv`                   | the additional environment variables of multus-CNI daemonset pod container                                                                                                                                            | `[]`                              |
| `multus.multusCNI.extraVolumes`               | the additional volumes of multus-CNI daemonset pod container                                                                                                                                                          | `[]`                              |
| `multus.multusCNI.extraVolumeMounts`          | the additional hostPath mounts of multus-CNI daemonset pod container                                                                                                                                                  | `[]`                              |
| `multus.multusCNI.log.logLevel`               | the multus-CNI daemonset pod log level                                                                                                                                                                                | `debug`                           |
| `multus.multusCNI.log.logFile`                | the multus-CNI daemonset pod log file                                                                                                                                                                                 | `/var/log/multus.log`             |


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

| Name                                                                                 | Description                                                                                      | Value                                      |
| ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------ | ------------------------------------------ |
| `spiderpoolAgent.name`                                                               | the spiderpoolAgent name                                                                         | `spiderpool-agent`                         |
| `spiderpoolAgent.binName`                                                            | the binName name of spiderpoolAgent                                                              | `/usr/bin/spiderpool-agent`                |
| `spiderpoolAgent.image.registry`                                                     | the image registry of spiderpoolAgent                                                            | `ghcr.io`                                  |
| `spiderpoolAgent.image.repository`                                                   | the image repository of spiderpoolAgent                                                          | `spidernet-io/spiderpool/spiderpool-agent` |
| `spiderpoolAgent.image.pullPolicy`                                                   | the image pullPolicy of spiderpoolAgent                                                          | `IfNotPresent`                             |
| `spiderpoolAgent.image.digest`                                                       | the image digest of spiderpoolAgent, which takes preference over tag                             | `""`                                       |
| `spiderpoolAgent.image.tag`                                                          | the image tag of spiderpoolAgent, overrides the image tag whose default is the chart appVersion. | `""`                                       |
| `spiderpoolAgent.image.imagePullSecrets`                                             | the image imagePullSecrets of spiderpoolAgent                                                    | `[]`                                       |
| `spiderpoolAgent.serviceAccount.create`                                              | create the service account for the spiderpoolAgent                                               | `true`                                     |
| `spiderpoolAgent.serviceAccount.annotations`                                         | the annotations of spiderpoolAgent service account                                               | `{}`                                       |
| `spiderpoolAgent.service.annotations`                                                | the annotations for spiderpoolAgent service                                                      | `{}`                                       |
| `spiderpoolAgent.service.type`                                                       | the type for spiderpoolAgent service                                                             | `ClusterIP`                                |
| `spiderpoolAgent.priorityClassName`                                                  | the priority Class Name for spiderpoolAgent                                                      | `system-node-critical`                     |
| `spiderpoolAgent.affinity`                                                           | the affinity of spiderpoolAgent                                                                  | `{}`                                       |
| `spiderpoolAgent.extraArgs`                                                          | the additional arguments of spiderpoolAgent container                                            | `[]`                                       |
| `spiderpoolAgent.extraEnv`                                                           | the additional environment variables of spiderpoolAgent container                                | `[]`                                       |
| `spiderpoolAgent.extraVolumes`                                                       | the additional volumes of spiderpoolAgent container                                              | `[]`                                       |
| `spiderpoolAgent.extraVolumeMounts`                                                  | the additional hostPath mounts of spiderpoolAgent container                                      | `[]`                                       |
| `spiderpoolAgent.podAnnotations`                                                     | the additional annotations of spiderpoolAgent pod                                                | `{}`                                       |
| `spiderpoolAgent.podLabels`                                                          | the additional label of spiderpoolAgent pod                                                      | `{}`                                       |
| `spiderpoolAgent.resources.limits.cpu`                                               | the cpu limit of spiderpoolAgent pod                                                             | `1000m`                                    |
| `spiderpoolAgent.resources.limits.memory`                                            | the memory limit of spiderpoolAgent pod                                                          | `1024Mi`                                   |
| `spiderpoolAgent.resources.requests.cpu`                                             | the cpu requests of spiderpoolAgent pod                                                          | `100m`                                     |
| `spiderpoolAgent.resources.requests.memory`                                          | the memory requests of spiderpoolAgent pod                                                       | `128Mi`                                    |
| `spiderpoolAgent.securityContext`                                                    | the security Context of spiderpoolAgent pod                                                      | `{}`                                       |
| `spiderpoolAgent.httpPort`                                                           | the http Port for spiderpoolAgent, for health checking                                           | `5710`                                     |
| `spiderpoolAgent.healthChecking.startupProbe.failureThreshold`                       | the failure threshold of startup probe for spiderpoolAgent health checking                       | `60`                                       |
| `spiderpoolAgent.healthChecking.startupProbe.periodSeconds`                          | the period seconds of startup probe for spiderpoolAgent health checking                          | `2`                                        |
| `spiderpoolAgent.healthChecking.livenessProbe.failureThreshold`                      | the failure threshold of startup probe for spiderpoolAgent health checking                       | `6`                                        |
| `spiderpoolAgent.healthChecking.livenessProbe.periodSeconds`                         | the period seconds of startup probe for spiderpoolAgent health checking                          | `10`                                       |
| `spiderpoolAgent.healthChecking.readinessProbe.failureThreshold`                     | the failure threshold of startup probe for spiderpoolAgent health checking                       | `3`                                        |
| `spiderpoolAgent.healthChecking.readinessProbe.periodSeconds`                        | the period seconds of startup probe for spiderpoolAgent health checking                          | `10`                                       |
| `spiderpoolAgent.prometheus.enabled`                                                 | enable spiderpool agent to collect metrics                                                       | `false`                                    |
| `spiderpoolAgent.prometheus.enabledDebugMetric`                                      | enable spiderpool agent to collect debug level metrics                                           | `false`                                    |
| `spiderpoolAgent.prometheus.port`                                                    | the metrics port of spiderpool agent                                                             | `5711`                                     |
| `spiderpoolAgent.prometheus.serviceMonitor.install`                                  | install serviceMonitor for spiderpool agent. This requires the prometheus CRDs to be available   | `false`                                    |
| `spiderpoolAgent.prometheus.serviceMonitor.namespace`                                | the serviceMonitor namespace. Default to the namespace of helm instance                          | `""`                                       |
| `spiderpoolAgent.prometheus.serviceMonitor.annotations`                              | the additional annotations of spiderpoolAgent serviceMonitor                                     | `{}`                                       |
| `spiderpoolAgent.prometheus.serviceMonitor.labels`                                   | the additional label of spiderpoolAgent serviceMonitor                                           | `{}`                                       |
| `spiderpoolAgent.prometheus.serviceMonitor.interval`                                 | represents the interval of spiderpoolAgent serviceMonitor's scraping action                      | `10s`                                      |
| `spiderpoolAgent.prometheus.prometheusRule.install`                                  | install prometheusRule for spiderpool agent. This requires the prometheus CRDs to be available   | `false`                                    |
| `spiderpoolAgent.prometheus.prometheusRule.namespace`                                | the prometheusRule namespace. Default to the namespace of helm instance                          | `""`                                       |
| `spiderpoolAgent.prometheus.prometheusRule.annotations`                              | the additional annotations of spiderpoolAgent prometheusRule                                     | `{}`                                       |
| `spiderpoolAgent.prometheus.prometheusRule.labels`                                   | the additional label of spiderpoolAgent prometheusRule                                           | `{}`                                       |
| `spiderpoolAgent.prometheus.prometheusRule.enableWarningIPAMAllocationFailure`       | the additional rule of spiderpoolAgent prometheusRule                                            | `true`                                     |
| `spiderpoolAgent.prometheus.prometheusRule.enableWarningIPAMAllocationOverTime`      | the additional rule of spiderpoolAgent prometheusRule                                            | `true`                                     |
| `spiderpoolAgent.prometheus.prometheusRule.enableWarningIPAMHighAllocationDurations` | the additional rule of spiderpoolAgent prometheusRule                                            | `true`                                     |
| `spiderpoolAgent.prometheus.prometheusRule.enableWarningIPAMReleaseFailure`          | the additional rule of spiderpoolAgent prometheusRule                                            | `true`                                     |
| `spiderpoolAgent.prometheus.prometheusRule.enableWarningIPAMReleaseOverTime`         | the additional rule of spiderpoolAgent prometheusRule                                            | `true`                                     |
| `spiderpoolAgent.debug.logLevel`                                                     | the log level of spiderpool agent [debug, info, warn, error, fatal, panic]                       | `info`                                     |
| `spiderpoolAgent.debug.gopsPort`                                                     | the gops port of spiderpool agent                                                                | `5712`                                     |


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

| Name                                        | Description                                                                                                                 | Value                                           |
| ------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------- |
| `spiderpoolInit.name`                       | the init job for installing default spiderippool                                                                            | `spiderpool-init`                               |
| `spiderpoolInit.binName`                    | the binName name of spiderpoolInit                                                                                          | `/usr/bin/spiderpool-init`                      |
| `spiderpoolInit.hostnetwork`                | enable hostnetwork mode of spiderpoolInit pod. Notice, if no CNI available before spiderpool installation, must enable this | `true`                                          |
| `spiderpoolInit.image.registry`             | the image registry of spiderpoolInit                                                                                        | `ghcr.io`                                       |
| `spiderpoolInit.image.repository`           | the image repository of spiderpoolInit                                                                                      | `spidernet-io/spiderpool/spiderpool-controller` |
| `spiderpoolInit.image.pullPolicy`           | the image pullPolicy of spiderpoolInit                                                                                      | `IfNotPresent`                                  |
| `spiderpoolInit.image.digest`               | the image digest of spiderpoolInit, which takes preference over tag                                                         | `""`                                            |
| `spiderpoolInit.image.tag`                  | the image tag of spiderpoolInit, overrides the image tag whose default is the chart appVersion.                             | `""`                                            |
| `spiderpoolInit.image.imagePullSecrets`     | the image imagePullSecrets of spiderpoolInit                                                                                | `[]`                                            |
| `spiderpoolInit.priorityClassName`          | the priority Class Name for spiderpoolInit                                                                                  | `system-node-critical`                          |
| `spiderpoolInit.affinity`                   | the affinity of spiderpoolInit                                                                                              | `{}`                                            |
| `spiderpoolInit.extraArgs`                  | the additional arguments of spiderpoolInit container                                                                        | `[]`                                            |
| `spiderpoolInit.resources.limits.cpu`       | the cpu limit of spiderpoolInit pod                                                                                         | `200m`                                          |
| `spiderpoolInit.resources.limits.memory`    | the memory limit of spiderpoolInit pod                                                                                      | `256Mi`                                         |
| `spiderpoolInit.resources.requests.cpu`     | the cpu requests of spiderpoolInit pod                                                                                      | `100m`                                          |
| `spiderpoolInit.resources.requests.memory`  | the memory requests of spiderpoolInit pod                                                                                   | `128Mi`                                         |
| `spiderpoolInit.extraEnv`                   | the additional environment variables of spiderpoolInit container                                                            | `[]`                                            |
| `spiderpoolInit.securityContext`            | the security Context of spiderpoolInit pod                                                                                  | `{}`                                            |
| `spiderpoolInit.podAnnotations`             | the additional annotations of spiderpoolInit pod                                                                            | `{}`                                            |
| `spiderpoolInit.podLabels`                  | the additional label of spiderpoolInit pod                                                                                  | `{}`                                            |
| `spiderpoolInit.serviceAccount.annotations` | the annotations of spiderpoolInit service account                                                                           | `{}`                                            |


