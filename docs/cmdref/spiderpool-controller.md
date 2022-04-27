# spiderpool-controller

CLI

## spiderpool-controller daemon

### Synopsis

run spiderpool controller daemon

### Options

```
    --config-dir string         config file path (default /tmp/spiderpool/config-map)
```

### ENV

```
    SPIDERPOOL_LOG_LEVEL                        log level (DEBUG|INFO|ERROR)
    SPIDERPOOL_ENABLED_PPROF                    enable pprof (true|false)
    SPIDERPOOL_ENABLED_METRIC                   enable metrics (true|false)
    SPIDERPOOL_METRIC_HTTP_PORT                 metric port (default to 5721)
    SPIDERPOOL_GC_IPPOOL_ENABLED                enable GC ip in ippool, prior to other GC environment (true|false, default to true)
    SPIDERPOOL_GC_TERMINATING_POD_IP_ENABLED    enable GC ip of terminating pod whose graceful-time times out (true|false, default to true)
    SPIDERPOOL_GC_TERMINATING_POD_IP_DELAY      delay to GC ip after graceful-time times out (second, default to 0)
    SPIDERPOOL_GC_EVICTED_POD_IP_ENABLED        enable GC ip of evicted pod (true|false, default to true)
    SPIDERPOOL_GC_EVICTED_POD_IP_DELAY          delay to GC ip of evicted pod (second, default to 0)
    SPIDERPOOL_HTTP_PORT                        http port  (default to 5710)
```

## spiderpool-controller shutdown

### Synopsis

notify to stop spiderpool-controller daemon

## spiderpool-controller metric

get local metric

### Options

```
    --port string         http server port of local metric (default to 5721)
```

## spiderpool-controller status

show status:
(1) whether local is controller leader
(2)...

### Options

```
    --port string         http server port of local metric (default to 5720)
```
