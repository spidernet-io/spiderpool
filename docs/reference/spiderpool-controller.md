# spiderpool-controller

This page describes CLI options and ENV of spiderpool-controller.

## spiderpool-controller daemon

Run the spiderpool controller daemon.

### Options

```
    --config-dir string         config file path (default /tmp/spiderpool/config-map)
```

### ENV

| env                                      | default | description                                                                        |
|------------------------------------------|---------|------------------------------------------------------------------------------------|
| SPIDERPOOL_LOG_LEVEL                     | info    | Log level, optional values are "debug", "info", "warn", "error", "fatal", "panic". |
| SPIDERPOOL_ENABLED_METRIC                | false   | Enable/disable metrics.                                                            |
| SPIDERPOOL_HEALTH_PORT                   | 5720    | Spiderpool-controller backend HTTP server port.                                    |
| SPIDERPOOL_METRIC_HTTP_PORT              | 5721    | Metric HTTP server port.                                                           |
| SPIDERPOOL_WEBHOOK_PORT                  | 5722    | Webhook HTTP server port.                                                          |
| SPIDERPOOL_CLI_PORT                      | 5723    | Spiderpool-CLI HTTP server port.                                                   |
| SPIDERPOOL_GOPS_LISTEN_PORT              | 5724    | Port that gops is listening on. Disabled if empty.                                 |
| SPIDERPOOL_GC_IP_ENABLED                 | true    | Enable/disable IP GC.                                                              |
| SPIDERPOOL_GC_TERMINATING_POD_IP_ENABLED | true    | Enable/disable IP GC for Terminating pod.                                          |


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
