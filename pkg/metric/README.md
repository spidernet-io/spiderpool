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

Spiderpool agent exports some metrics related with IPAM allocation and release. Currently, those include:


| Name                                         | description                                                                                                                 |
|----------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------|
| ipam_allocation_total_counts                 | Count of the number of the IPAM allocation requests that Spiderpool Agent received , prometheus type: counter               |
| ipam_allocation_failure_counts               | Number of Spiderpool Agent IPAM allocation failures, prometheus type: counter                                               |
| ipam_allocation_rollback_failure_counts      | Number of Spiderpool Agent IPAM allocation rollback failure, prometheus type: counter                                       |
| ipam_allocation_err_internal_counts          | Number of Spiderpool Agent IPAM allocation internal error, prometheus type: counter                                         |
| ipam_allocation_err_no_available_pool_counts | Number of Spiderpool Agent IPAM allocation no available IPPool error, prometheus type: counter                              |
| ipam_allocation_err_retries_exhausted_counts | Number of Spiderpool Agent IPAM allocation retries exhausted error, prometheus type: counter                                |
| ipam_allocation_err_ip_used_out_counts       | Number of Spiderpool Agent IPAM allocation IP addresses used out error, prometheus type: counter                            |
| ipam_allocation_average_duration_seconds     | The average duration of all Spiderpool Agent allocation processes, prometheus type: gauge                                   |
| ipam_allocation_max_duration_seconds         | The maximum duration of Spiderpool Agent allocation process (per-process), prometheus type: gauge                           |
| ipam_allocation_min_duration_seconds         | The minimum duration of Spiderpool Agent allocation process (per-process), prometheus type: gauge                           |
| ipam_allocation_latest_duration_seconds      | The latest duration of Spiderpool Agent allocation process (per-process), prometheus type: gauge                            |
| ipam_allocation_duration_seconds             | Per Spiderpool Agent allocation process, it can be showed IPAM allocation duration distribution, prometheus type: histogram |
| ipam_release_total_counts                    | Count of the number of Spiderpool Agent received the IPAM release requests, prometheus type: counter                        |
| ipam_release_failure_counts                  | Number of Spiderpool Agent IPAM release failure, prometheus type: counter                                                   |
| ipam_release_err_internal_counts             | Number of Spiderpool Agent IPAM releasing internal error, prometheus type: counter                                          |
| ipam_release_err_retries_exhausted_counts    | Number of Spiderpool Agent IPAM releasing retries exhausted error, prometheus type: counter                                 |
| ipam_release_average_duration_seconds        | The average duration of all Spiderpool Agent release processes, prometheus type: gauge                                      |
| ipam_release_max_duration_seconds            | The maximum duration of Spiderpool Agent release process (per-process), prometheus type: gauge                              |
| ipam_release_min_duration_seconds            | The minimum duration of Spiderpool Agent release process (per-process), prometheus type: gauge                              |
| ipam_release_latest_duration_seconds         | The latest duration of Spiderpool Agent release process (per-process), prometheus type: gauge                               |
| ipam_release_duration_seconds                | Per Spiderpool Agent release process, it can be showed IPAM release duration distribution, prometheus type: histogram       |

### Spiderpool Controller

Spiderpool controller exports some metrics related with SpiderIPPool IP garbage collection. Currently, those include:

| Name                 | description                                                                              |
|----------------------|------------------------------------------------------------------------------------------|
| ip_gc_total_counts   | Number of Spiderpool Controller IP garbage collection, prometheus type: counter          |
| ip_gc_failure_counts | Number of Spiderpool Controller IP garbage collection failures, prometheus type: counter |
| subnet_ippool_counts | Number of SpiderSubnet corresponding IPPools number, prometheus type: gauge              |
