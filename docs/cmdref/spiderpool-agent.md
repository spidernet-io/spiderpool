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

```
    SPIDERPOOL_LOG_LEVEL                log level (DEBUG|INFO|ERROR)
    SPIDERPOOL_ENABLED_METRIC           enable metrics (true|false)
    SPIDERPOOL_METRIC_HTTP_PORT         metric port (default to 5711)
    SPIDERPOOL_HEALTH_PORT              http port  (default to 5710)
```

## spiderpool-agent shutdown

Notify of stopping the spiderpool-agent daemon.

## spiderpool-agent metric

Get local metrics.

### Options

```
    --port string         http server port of local metric (default to 5711)
```
