# spiderpool-agent

CLI

## spiderpool-agent daemon

### Synopsis

run spiderpool agent daemon

### Options

```
    --config-dir string         config file path (default /tmp/spiderpool/config-map)
    --ipam-config-dir string    config file for ipam plugin 
```

### ENV

```
    SPIDERPOOL_LOG_LEVEL                log level (DEBUG|INFO|ERROR)
    SPIDERPOOL_ENABLED_PPROF            enable pprof (true|false)
    SPIDERPOOL_ENABLED_METRIC           enable metrics (true|false)
    SPIDERPOOL_METRIC_HTTP_PORT         metric port (default to 5711)
    SPIDERPOOL_HTTP_PORT                http port  (default to 5710)
```

## spiderpool-agent shutdown

### Synopsis

notify to stop spiderpool-agent daemon

## spiderpool-agent metric

get local metric

### Options

```
    --port string         http server port of local metric (default to 5711)
```
