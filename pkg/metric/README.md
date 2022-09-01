# Metrics

Spiderpool can be configured to serve [Opentelemetry](https://opentelemetry.io/) metrics.
Spiderpool metrics provide the insight of Spiderpool Agent and Spiderpool Controller.

## Get Started

### Enable Metric support

Firstly, please ensure you have installed the spiderpool and configured the CNI file, refer [install](./install.md) for details

Check the environment variable `SPIDERPOOL_ENABLED_METRIC` of the daemonset `spiderpool-agent` for whether it is already set to `true` or not.

Check the environment variable `SPIDERPOOL_ENABLED_METRIC` of deployment `spiderpool-controller` for whether it is already set to `true` or not.

```shell
kubectl -n kube-system get daemonset spiderpool-agent -o yaml
------
kubectl -n kube-system get deployment spiderpool-controller -o yaml
```

You can set one or both of them to `true`.
For example, let's enable spiderpool agent metrics by running `helm upgrade --set spiderpoolAgent.prometheus.enabled=true`.

## Metric reference

### Spiderpool Agent

Spiderpool agent exports some metrics related with IPAM allocation and deallocation. Currently, those include:


| Name                                          | description                                                                                                                     |
|-----------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------|
| ipam_allocation_total_counts                  | Count of the number of the IPAM allocation requests that Spiderpool Agent received , prometheus type: counter                   |
| ipam_allocation_failure_counts                | Number of Spiderpool Agent IPAM allocation failures, prometheus type: counter                                                   |
| ipam_allocation_average_duration_seconds      | The average duration of all Spiderpool Agent allocation processes, prometheus type: gauge                                       |
| ipam_allocation_max_duration_seconds          | The maximum duration of Spiderpool Agent allocation process (per-process), prometheus type: gauge                               |
| ipam_allocation_min_duration_seconds          | The minimum duration of Spiderpool Agent allocation process (per-process), prometheus type: gauge                               |
| ipam_allocation_latest_duration_seconds       | The latest duration of Spiderpool Agent allocation process (per-process), prometheus type: gauge                                |
| ipam_allocation_duration_seconds              | Per Spiderpool Agent allocation process, it can be showed IPAM allocation duration distribution, prometheus type: histogram     |
| ipam_deallocation_total_counts                | Count of the number of Spiderpool Agent received the IPAM deallocation requests, prometheus type: counter                       |
| ipam_deallocation_failure_counts              | Number of Spiderpool Agent IPAM deallocation failure, prometheus type: counter                                                  |
| ipam_deallocation_average_duration_seconds    | The average duration of all Spiderpool Agent deallocation processes, prometheus type: gauge                                     |
| ipam_deallocation_max_duration_seconds        | The maximum duration of Spiderpool Agent deallocation process (per-process), prometheus type: gauge                             |
| ipam_deallocation_min_duration_seconds        | The minimum duration of Spiderpool Agent deallocation process (per-process), prometheus type: gauge                             |
| ipam_deallocation_latest_duration_seconds     | The latest duration of Spiderpool Agent deallocation process (per-process), prometheus type: gauge                              |
| ipam_deallocation_duration_seconds            | Per Spiderpool Agent deallocation process, it can be showed IPAM deallocation duration distribution, prometheus type: histogram |

### Spiderpool Controller

Spiderpool controller exports some metrics related with SpiderIPPool IP garbage collection. Currently, those include:

| Name                      | description                                                                              |
|---------------------------|------------------------------------------------------------------------------------------|
| ip_gc_total_counts        | Number of Spiderpool Controller IP garbage collection, prometheus type: counter          |
| ip_gc_failure_counts      | Number of Spiderpool Controller IP garbage collection failures, prometheus type: counter |
