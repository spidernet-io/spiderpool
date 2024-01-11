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


## spiderpool-agent shutdown

Notify of stopping the spiderpool-agent daemon.

## spiderpool-agent metric

Get local metrics.

### Options

```
    --port string         http server port of local metric (default to 5711)
```
