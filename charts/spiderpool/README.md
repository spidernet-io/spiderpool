# spiderpool

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


### feature parameters

| Name                                      | Description                                                              | Value    |
| ----------------------------------------- | ------------------------------------------------------------------------ | -------- |
| `feature.enableIPv4`                      | enable ipv4                                                              | `true`   |
| `feature.enableIPv6`                      | enable ipv6                                                              | `true`   |
| `feature.networkMode`                     | the network mode                                                         | `legacy` |
| `feature.enableStatefulSet`               | the network mode                                                         | `true`   |
| `feature.gc.enabled`                      | enable retrieve IP in spiderippool CR                                    | `true`   |
| `feature.gc.GcDeletingTimeOutPod.enabled` | enable retrieve IP for the pod who times out of deleting graceful period | `true`   |
| `feature.gc.GcDeletingTimeOutPod.delay`   | the gc delay seconds after the pod times out of deleting graceful period | `0`      |
| `feature.gc.GcEvictedPodPod.enabled`      | enable retrieve IP for the pod who is evicted                            | `true`   |
| `feature.gc.GcEvictedPodPod.delay`        | the gc delay seconds after the pod is evicted                            | `0`      |


### clusterDefaultPool parameters

| Name                                   | Description                                  | Value               |
| -------------------------------------- | -------------------------------------------- | ------------------- |
| `clusterDefaultPool.installIPv4IPPool` | install ipv4 spiderpool instance             | `false`             |
| `clusterDefaultPool.installIPv6IPPool` | install ipv6 spiderpool instance             | `false`             |
| `clusterDefaultPool.ipv4IPPoolName`    | the name of ipv4 spiderpool instance         | `default-v4-ippool` |
| `clusterDefaultPool.ipv6IPPoolName`    | the name of ipv6 spiderpool instance         | `default-v6-ippool` |
| `clusterDefaultPool.ipv4Subnet`        | the subnet of ipv4 spiderpool instance       | `""`                |
| `clusterDefaultPool.ipv6Subnet`        | the subnet of ipv6 spiderpool instance       | `""`                |
| `clusterDefaultPool.ipv4IPRanges`      | the available IP of ipv4 spiderpool instance | `[]`                |
| `clusterDefaultPool.ipv6IPRanges`      | the available IP of ipv6 spiderpool instance | `[]`                |


### spiderpoolAgent parameters

| Name                                                             | Description                                                                                      | Value                                      |
| ---------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ | ------------------------------------------ |
| `spiderpoolAgent.name`                                           | the spiderpoolAgent name                                                                         | `spiderpool-agent`                         |
| `spiderpoolAgent.binName`                                        | the binName name of spiderpoolAgent                                                              | `spiderpool-agent`                         |
| `spiderpoolAgent.image.registry`                                 | the image registry of spiderpoolAgent                                                            | `ghcr.io`                                  |
| `spiderpoolAgent.image.repository`                               | the image repository of spiderpoolAgent                                                          | `spidernet-io/spiderpool/spiderpool-agent` |
| `spiderpoolAgent.image.pullPolicy`                               | the image pullPolicy of spiderpoolAgent                                                          | `IfNotPresent`                             |
| `spiderpoolAgent.image.digest`                                   | the image digest of spiderpoolAgent, which takes preference over tag                             | `""`                                       |
| `spiderpoolAgent.image.tag`                                      | the image tag of spiderpoolAgent, overrides the image tag whose default is the chart appVersion. | `""`                                       |
| `spiderpoolAgent.image.imagePullSecrets`                         | the image imagePullSecrets of spiderpoolAgent                                                    | `[]`                                       |
| `spiderpoolAgent.serviceAccount.create`                          | create the service account for the spiderpoolAgent                                               | `true`                                     |
| `spiderpoolAgent.serviceAccount.annotations`                     | the annotations of spiderpoolAgent service account                                               | `{}`                                       |
| `spiderpoolAgent.service.annotations`                            | the annotations for spiderpoolAgent service                                                      | `{}`                                       |
| `spiderpoolAgent.service.type`                                   | the type for spiderpoolAgent service                                                             | `ClusterIP`                                |
| `spiderpoolAgent.priorityClassName`                              | the priority Class Name for spiderpoolAgent                                                      | `system-node-critical`                     |
| `spiderpoolAgent.affinity`                                       | the affinity of spiderpoolAgent                                                                  | `{}`                                       |
| `spiderpoolAgent.extraArgs`                                      | the additional arguments of spiderpoolAgent container                                            | `[]`                                       |
| `spiderpoolAgent.extraEnv`                                       | the additional environment variables of spiderpoolAgent container                                | `[]`                                       |
| `spiderpoolAgent.extraVolumes`                                   | the additional volumes of spiderpoolAgent container                                              | `[]`                                       |
| `spiderpoolAgent.extraVolumeMounts`                              | the additional hostPath mounts of spiderpoolAgent container                                      | `[]`                                       |
| `spiderpoolAgent.podAnnotations`                                 | the additional annotations of spiderpoolAgent pod                                                | `{}`                                       |
| `spiderpoolAgent.podLabels`                                      | the additional label of spiderpoolAgent pod                                                      | `{}`                                       |
| `spiderpoolAgent.resources.limits.cpu`                           | the cpu limit of spiderpoolAgent pod                                                             | `1000m`                                    |
| `spiderpoolAgent.resources.limits.memory`                        | the memory limit of spiderpoolAgent pod                                                          | `1024Mi`                                   |
| `spiderpoolAgent.resources.requests.cpu`                         | the cpu requests of spiderpoolAgent pod                                                          | `100m`                                     |
| `spiderpoolAgent.resources.requests.memory`                      | the memory requests of spiderpoolAgent pod                                                       | `128Mi`                                    |
| `spiderpoolAgent.securityContext`                                | the security Context of spiderpoolAgent pod                                                      | `{}`                                       |
| `spiderpoolAgent.healthChecking.port`                            | the http Port for spiderpoolAgent health checking                                                | `5710`                                     |
| `spiderpoolAgent.healthChecking.startupProbe.failureThreshold`   | the failure threshold of startup probe for spiderpoolAgent health checking                       | `60`                                       |
| `spiderpoolAgent.healthChecking.startupProbe.periodSeconds`      | the period seconds of startup probe for spiderpoolAgent health checking                          | `2`                                        |
| `spiderpoolAgent.healthChecking.livenessProbe.failureThreshold`  | the failure threshold of startup probe for spiderpoolAgent health checking                       | `6`                                        |
| `spiderpoolAgent.healthChecking.livenessProbe.periodSeconds`     | the period seconds of startup probe for spiderpoolAgent health checking                          | `10`                                       |
| `spiderpoolAgent.healthChecking.readinessProbe.failureThreshold` | the failure threshold of startup probe for spiderpoolAgent health checking                       | `3`                                        |
| `spiderpoolAgent.healthChecking.readinessProbe.periodSeconds`    | the period seconds of startup probe for spiderpoolAgent health checking                          | `10`                                       |
| `spiderpoolAgent.prometheus.enabled`                             | enable spiderpool agent to collect metrics                                                       | `false`                                    |
| `spiderpoolAgent.prometheus.port`                                | the metrics port of spiderpool agent                                                             | `5711`                                     |
| `spiderpoolAgent.prometheus.serviceMonitor.install`              | install serviceMonitor for spiderpool agent. This requires the prometheus CRDs to be available   | `false`                                    |
| `spiderpoolAgent.prometheus.prometheusRule.install`              | install prometheusRule for spiderpool agent. This requires the prometheus CRDs to be available   | `false`                                    |
| `spiderpoolAgent.debug.logLevel`                                 | the log level of spiderpool agent [debug, info, warn, error, fatal, panic]                       | `info`                                     |
| `spiderpoolAgent.debug.gopsPort`                                 | the gops port of spiderpool agent                                                                | `5712`                                     |


### spiderpoolController parameters

| Name                                                                  | Description                                                                                                                       | Value                                           |
| --------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------- |
| `spiderpoolController.name`                                           | the spiderpoolController name                                                                                                     | `spiderpool-controller`                         |
| `spiderpoolController.replicas`                                       | the replicas number of spiderpoolController pod                                                                                   | `1`                                             |
| `spiderpoolController.binName`                                        | the binName name of spiderpoolController                                                                                          | `spiderpool-controller`                         |
| `spiderpoolController.hostnetwork`                                    | enable hostnetwork mode of spiderpoolController pod. Notice, if no CNI available before spiderpool installation, must enable this | `true`                                          |
| `spiderpoolController.image.registry`                                 | the image registry of spiderpoolController                                                                                        | `ghcr.io`                                       |
| `spiderpoolController.image.repository`                               | the image repository of spiderpoolController                                                                                      | `spidernet-io/spiderpool/spiderpool-controller` |
| `spiderpoolController.image.pullPolicy`                               | the image pullPolicy of spiderpoolController                                                                                      | `IfNotPresent`                                  |
| `spiderpoolController.image.digest`                                   | the image digest of spiderpoolController, which takes preference over tag                                                         | `""`                                            |
| `spiderpoolController.image.tag`                                      | the image tag of spiderpoolController, overrides the image tag whose default is the chart appVersion.                             | `""`                                            |
| `spiderpoolController.image.imagePullSecrets`                         | the image imagePullSecrets of spiderpoolController                                                                                | `[]`                                            |
| `spiderpoolController.serviceAccount.create`                          | create the service account for the spiderpoolController                                                                           | `true`                                          |
| `spiderpoolController.serviceAccount.annotations`                     | the annotations of spiderpoolController service account                                                                           | `{}`                                            |
| `spiderpoolController.service.annotations`                            | the annotations for spiderpoolController service                                                                                  | `{}`                                            |
| `spiderpoolController.service.type`                                   | the type for spiderpoolController service                                                                                         | `ClusterIP`                                     |
| `spiderpoolController.priorityClassName`                              | the priority Class Name for spiderpoolController                                                                                  | `system-node-critical`                          |
| `spiderpoolController.affinity`                                       | the affinity of spiderpoolController                                                                                              | `{}`                                            |
| `spiderpoolController.extraArgs`                                      | the additional arguments of spiderpoolController container                                                                        | `[]`                                            |
| `spiderpoolController.extraEnv`                                       | the additional environment variables of spiderpoolController container                                                            | `[]`                                            |
| `spiderpoolController.extraVolumes`                                   | the additional volumes of spiderpoolController container                                                                          | `[]`                                            |
| `spiderpoolController.extraVolumeMounts`                              | the additional hostPath mounts of spiderpoolController container                                                                  | `[]`                                            |
| `spiderpoolController.podAnnotations`                                 | the additional annotations of spiderpoolController pod                                                                            | `{}`                                            |
| `spiderpoolController.podLabels`                                      | the additional label of spiderpoolController pod                                                                                  | `{}`                                            |
| `spiderpoolController.securityContext`                                | the security Context of spiderpoolController pod                                                                                  | `{}`                                            |
| `spiderpoolController.resources.limits.cpu`                           | the cpu limit of spiderpoolController pod                                                                                         | `500m`                                          |
| `spiderpoolController.resources.limits.memory`                        | the memory limit of spiderpoolController pod                                                                                      | `1024Mi`                                        |
| `spiderpoolController.resources.requests.cpu`                         | the cpu requests of spiderpoolController pod                                                                                      | `100m`                                          |
| `spiderpoolController.resources.requests.memory`                      | the memory requests of spiderpoolController pod                                                                                   | `128Mi`                                         |
| `spiderpoolController.podDisruptionBudget.enabled`                    | enable podDisruptionBudget for spiderpoolController pod, ref: https://kubernetes.io/docs/concepts/workloads/pods/disruptions/     | `false`                                         |
| `spiderpoolController.podDisruptionBudget.minAvailable`               | minimum number/percentage of pods that should remain scheduled.                                                                   | `1`                                             |
| `spiderpoolController.healthChecking.port`                            | the http Port for spiderpoolController health checking                                                                            | `5720`                                          |
| `spiderpoolController.healthChecking.startupProbe.failureThreshold`   | the failure threshold of startup probe for spiderpoolController health checking                                                   | `30`                                            |
| `spiderpoolController.healthChecking.startupProbe.periodSeconds`      | the period seconds of startup probe for spiderpoolController health checking                                                      | `2`                                             |
| `spiderpoolController.healthChecking.livenessProbe.failureThreshold`  | the failure threshold of startup probe for spiderpoolController health checking                                                   | `6`                                             |
| `spiderpoolController.healthChecking.livenessProbe.periodSeconds`     | the period seconds of startup probe for spiderpoolController health checking                                                      | `10`                                            |
| `spiderpoolController.healthChecking.readinessProbe.failureThreshold` | the failure threshold of startup probe for spiderpoolController health checking                                                   | `3`                                             |
| `spiderpoolController.healthChecking.readinessProbe.periodSeconds`    | the period seconds of startup probe for spiderpoolController health checking                                                      | `10`                                            |
| `spiderpoolController.webhookPort`                                    | the http port for spiderpoolController webhook                                                                                    | `5722`                                          |
| `spiderpoolController.cliPort`                                        | the http port for spiderpoolController CLI                                                                                        | `5723`                                          |
| `spiderpoolController.prometheus.enabled`                             | enable spiderpool Controller to collect metrics                                                                                   | `false`                                         |
| `spiderpoolController.prometheus.port`                                | the metrics port of spiderpool Controller                                                                                         | `5721`                                          |
| `spiderpoolController.prometheus.serviceMonitor.install`              | install serviceMonitor for spiderpool Controller. This requires the prometheus CRDs to be available                               | `false`                                         |
| `spiderpoolController.prometheus.prometheusRule.install`              | install prometheusRule for spiderpool Controller. This requires the prometheus CRDs to be available                               | `false`                                         |
| `spiderpoolController.debug.logLevel`                                 | the log level of spiderpool Controller [debug, info, warn, error, fatal, panic]                                                   | `info`                                          |
| `spiderpoolController.debug.gopsPort`                                 | the gops port of spiderpool Controller                                                                                            | `5724`                                          |
| `spiderpoolController.tls.method`                                     | the method for generating TLS certificates. [ provided , certmanager ]                                                            | `provided`                                      |
| `spiderpoolController.tls.secretName`                                 | the secret name for storing TLS certificates                                                                                      | `spiderpool-controller-server-certs`            |
| `spiderpoolController.tls.certmanager.certValidityDuration`           | generated certificates validity duration in days for 'certmanager' method                                                         | `365`                                           |
| `spiderpoolController.tls.certmanager.issuerName`                     | issuer name of cert manager 'certmanager'. If not specified, a CA issuer will be created.                                         | `""`                                            |
| `spiderpoolController.tls.certmanager.extraDnsNames`                  | extra DNS names added to certificate when it's auto generated                                                                     | `[]`                                            |
| `spiderpoolController.tls.certmanager.extraIPAddresses`               | extra IP addresses added to certificate when it's auto generated                                                                  | `[]`                                            |
| `spiderpoolController.tls.provided.tlsCert`                           | encoded tls certificate for provided method                                                                                       | `""`                                            |
| `spiderpoolController.tls.provided.tlsKey`                            | encoded tls key for provided method                                                                                               | `""`                                            |
| `spiderpoolController.tls.provided.tlsCa`                             | encoded tls CA for provided method                                                                                                | `""`                                            |


