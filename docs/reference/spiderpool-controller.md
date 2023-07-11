# spiderpool-controller

This page describes CLI options and ENV of spiderpool-controller.

## spiderpool-controller daemon

Run the spiderpool controller daemon.

### Options

```
    --config-dir string         config file path (default /tmp/spiderpool/config-map)
```

### ENV

```
    SPIDERPOOL_LOG_LEVEL                        log level (DEBUG|INFO|ERROR)
    SPIDERPOOL_ENABLED_METRIC                   enable metrics (true|false)
    SPIDERPOOL_METRIC_HTTP_PORT                 metric port (default to 5721)
    SPIDERPOOL_GC_IP_ENABLED                    enable GC ip in ippool, prior to other GC environment (true|false, default to true)
    SPIDERPOOL_GC_TERMINATING_POD_IP_ENABLED    enable GC ip of terminating pod whose graceful-time times out (true|false, default to true)
    SPIDERPOOL_GC_ADDITIONAL_GRACE_DELAY        delay to GC ip after graceful-time times out (second, default to 0)
    SPIDERPOOL_HEALTH_PORT                      http port  (default to 5710)
    SPIDERPOOL_GC_DEFAULT_INTERVAL_DURATION     all intervals of GC (second, default to 600)
```

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
