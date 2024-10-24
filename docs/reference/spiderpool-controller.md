# spiderpool-controller

This page describes CLI options and ENV of spiderpool-controller.

## spiderpool-controller daemon

Run the spiderpool controller daemon.

### Options

```
    --config-dir string         config file path (default /tmp/spiderpool/config-map)
```

### ENV

| env                                                               | default | description                                                                                      |
|-------------------------------------------------------------------|---------|--------------------------------------------------------------------------------------------------|
| SPIDERPOOL_LOG_LEVEL                                              | info    | Log level, optional values are "debug", "info", "warn", "error", "fatal", "panic".               |
| SPIDERPOOL_ENABLED_METRIC                                         | false   | Enable/disable metrics.                                                                          |
| SPIDERPOOL_ENABLED_DEBUG_METRIC                                   | false   | Enable spiderpool agent to collect debug level metrics.                                          |
| SPIDERPOOL_METRIC_HTTP_PORT                                       | false   | The metrics port of spiderpool agent.                                                            |
| SPIDERPOOL_GOPS_LISTEN_PORT                                       | 5724    | The gops port of spiderpool Controller.                                                          |
| SPIDERPOOL_WEBHOOK_PORT                                           | 5722    | Webhook HTTP server port.                                                                        |
| SPIDERPOOL_HEALTH_PORT                                            | 5720    | The http Port for spiderpoolController, for health checking and http service.                    |
| SPIDERPOOL_GC_IP_ENABLED                                          | true    | Enable/disable IP GC.                                                                            |
| SPIDERPOOL_GC_STATELESS_TERMINATING_POD_ON_READY_NODE_ENABLED     | true    | Enable/disable IP GC for stateless Terminating pod when the pod corresponding node is ready.     |
| SPIDERPOOL_GC_STATELESS_TERMINATING_POD_ON_NOT_READY_NODE_ENABLED | true    | Enable/disable IP GC for stateless Terminating pod when the pod corresponding node is not ready. |
| SPIDERPOOL_GC_ADDITIONAL_GRACE_DELAY                              | true    | The gc delay seconds after the pod times out of deleting graceful period.                        |
| SPIDERPOOL_GC_DEFAULT_INTERVAL_DURATION                           | true    | The gc all interval duration.                                                                    |
| SPIDERPOOL_MULTUS_CONFIG_ENABLED                                  | true    | Enable/disable SpiderMultusConfig.                                                               |
| SPIDERPOOL_CNI_CONFIG_DIR                                         | /etc/cni/net.d    | The host path of the cni config directory.                                                       |
| SPIDERPOOL_CILIUM_CONFIGMAP_NAMESPACE_NAME                        | kube-system/cilium-config.    | The cilium's configMap, default is kube-system/cilium-config.                                    |
| SPIDERPOOL_COORDINATOR_DEFAULT_NAME                               | default | the name of default spidercoordinator CR |
| SPIDERPOOL_CONTROLLER_DEPLOYMENT_NAME                                          | spiderpool-controller | The deployment name of spiderpool-controller.                                                    | 

## spiderpool-controller shutdown

Notify of stopping spiderpool-controller daemon.

## spiderpool-controller metric

Get local metrics.

### Options

```
    --port string         http server port of local metric (default to 5721)
```

## spiderpool-controller status

Show status:

1. Whether local is controller leader
2. ...

### Options

```
    --port string         http server port of local metric (default to 5720)
```
