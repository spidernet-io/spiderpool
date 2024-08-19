# spiderpool-agent

This page describes CLI options and ENV of spiderpool-agent.

## spiderpool-agent daemon

Run the spiderpool agent daemon.

### Options

```
    --config-dir string         config file path (default /tmp/spiderpool/config-map)
    --ipam-config-dir string    config file for ipam plugin 
```

### ENV

| env                                             | default | description                                                                                     |
|-------------------------------------------------|---------|-------------------------------------------------------------------------------------------------|
| SPIDERPOOL_LOG_LEVEL                            | info    | Log level, optional values are "debug", "info", "warn", "error", "fatal", "panic".              |
| SPIDERPOOL_ENABLED_METRIC                       | false   | Enable/disable metrics.                                                                         |
| SPIDERPOOL_HEALTH_PORT                          | 5710    | Metric HTTP server port.                                                                        |
| SPIDERPOOL_METRIC_HTTP_PORT                     | 5711    | Spiderpool-agent backend HTTP server port.                                                      |
| SPIDERPOOL_GOPS_LISTEN_PORT                     | 5712    | Port that gops is listening on. Disabled if empty.                                              |
| SPIDERPOOL_UPDATE_CR_MAX_RETRIES                | 3       | Max retries to update k8s resources.                                                            |
| SPIDERPOOL_WORKLOADENDPOINT_MAX_HISTORY_RECORDS | 100     | Max historical IP allocation information allowed for a single Pod recorded in WorkloadEndpoint. |
| SPIDERPOOL_IPPOOL_MAX_ALLOCATED_IPS             | 5000    | Max number of IP that a single IP pool can provide.                                             |
| SPIDERPOOL_ENABLED_RELEASE_CONFLICT_IPS         | true    | Enable/disable release conflict IPs.                                                            |

## spiderpool-agent helps set sysctl configs for each node

To optimize the kernel network configuration of a node, spiderpool-agent will by default configure the following kernel parameters:

| sysctl config | value | description |
| -------------| ------| ------------|
| net.ipv4.neigh.default.gc_thresh3 | 28160 | This is the hard maximum number of entries to keep in the ARP cache. The garbage collector will always run if there are more than this number of entries in the cache. for ipv4  |
| net.ipv6.neigh.default.gc_thresh3 | 28160 | This is the hard maximum number of entries to keep in the ARP cache. The garbage collector will always run if there are more than this number of entries in the cache. for ipv6. Note: this is only avaliable in some low kernel version.|
| net.ipv4.conf.all.arp_notify | 1 |  Generate gratuitous arp requests when device is brought up or hardware address changes.|
| net.ipv4.conf.all.forwarding | 1 | enable ipv4 forwarding |
| net.ipv6.conf.all.forwarding | 1 | enable ipv6 forwarding |
| net.ipv4.conf.all.rp_filter   | 0 | no source validation for the each incoming packet |

Note: Some kernel parameters can only be set in certain kernel versions, so we will ignore the "kernel parameter does not exist" error when configure the kernel parameters. Example: `net.ipv6.neigh.default.gc_thresh3`.

Users can edit the `spiderpoolAgent.securityContext` field of values.yaml in the chart before installing spiderpool to update the kernel parameters that need additional configuration, or manually edit spiderpool-agent daemonSet after installing Spiderpool, and then restart spiderpool-agent pods:

Users can disable this feature by following command when installing Spiderpool:

```
helm install spiderpool -n kube-system --set global.tuneSysctlConfig=false
```

Or configure the spiderpool-conf configMap, set tuneSysctlConfig to false and restart the spiderpool-agent pods.

## spiderpool-agent shutdown

Notify of stopping the spiderpool-agent daemon.

## spiderpool-agent metric

Get local metrics.

### Options

```
    --port string         http server port of local metric (default to 5711)
```
