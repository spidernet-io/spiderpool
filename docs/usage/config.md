# Configuration

## IPAM Plugin Configuration

the following is an example for IPAM configuration

```yaml
{
    "cniVersion": "0.3.1",
    "name": "macvlan-pod-network",
    "plugins": [
        {
            "name": "macvlan-pod-network",
            "type": "macvlan",
            "master": "ens256",
            "mode": "bridge",
            "mtu": 1500,
            "ipam": {
                "type": "spiderpool",
                "log_file_path": "/var/run/spidernet/spiderpool.log",
                "log_file_max_size": "100M",
                "log_file_max_age": "30d",
                "log_file_max_count": 7,
                "log_level": "INFO"
            }
        }
      ]
}
```

* log_file_path

    optional, log file path of the IPAM plugin, default to "/var/run/spidernet/spiderpool.log"

* log_file_max_size

    optional, max file size for each rotated file, default to "100M"

* log_file_max_age

    optional, max file age for each rotated file, default to "30d"

* log_file_max_count

    optional, max number of rotated file, default to "7"

* log_level

    optional, log level, default to "INFO". It could be "INFO", "DEBUG", "WARN", "ERROR"

## Configmap Configuration

The configmap "spiderpool-conf" is the global configuration of spiderpool.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: spiderpool-conf
  namespace: kube-system
data:
  ipamUnixSocketPath: "/var/run/spidernet/spiderpool.sock"
  enableIpv4: true
  enableIpv6: true
  clusterDefaultIpv4Ippool: []
  clusterDefaultIpv6Ippool: []
  networkMode: "legacy"
```

* ipamUnixSocketPath
    the spiderpool agent pod will listen on this unix socket file, and handle IPAM request from the IPAM plugin

* enableIpv4

  * true: the spiderpool will assign ipv4 IP, if fail to assign an ipv4 IP, the IPAM plugin will fail for pod creating
  
  * false: the spiderpool will ignore assigning ipv4 IP

* enableIpv6

  * true: the spiderpool will assign ipv6 IP, if fail to assign an ipv6 IP, the IPAM plugin will fail for pod creating

  * false: the spiderpool will ignore assigning ipv6 IP

* clusterDefaultIpv4Ippool

    the global default ippool of ipv4, it could set to multiple ippool for backup case. Notice, the IP version of these ippool must be IPv4.

* clusterDefaultIpv6Ippool

    the global default ippool of ipv6, it could set to multiple ippool for backup case. Notice, the IP version of these ippool must be IPv6.

* networkMode

    network mode of spiderpool, currently, it only support:

  * "legacy"

## spiderpool controller environment

| environment                              | description            | value                                       |
|------------------------------------------|------------------------|---------------------------------------------|
| SPIDERPOOL_LOG_LEVEL                     | log level              | "INFO", "DEBUG", "ERROR", default to "INFO" |
| SPIDERPOOL_ENABLED_PPROF                 | enable pprof for debug | 5721                                        |                             |
| SPIDERPOOL_ENABLED_METRIC                | enable metrics         | "true" or "false". default to "false"       |
| SPIDERPOOL_METRIC_HTTP_PORT              | metrics port           | 5721                                        |
| SPIDERPOOL_HTTP_PORT                     |                        |                                             |
| SPIDERPOOL_GC_IPPOOL_ENABLED             |                        |                                             |
| SPIDERPOOL_GC_TERMINATING_POD_IP_ENABLED |                        |                                             |
| SPIDERPOOL_GC_TERMINATING_POD_IP_DELAY   |                        |                                             |
| SPIDERPOOL_GC_EVICTED_POD_IP_ENABLED     |                        |                                             |
| SPIDERPOOL_GC_EVICTED_POD_IP_DELAY       |                        |                                             |

## spiderpool agent environment

| environment                 | description            | value                                       |
|-----------------------------|------------------------|---------------------------------------------|
| SPIDERPOOL_LOG_LEVEL        | log level              | "INFO", "DEBUG", "ERROR", default to "INFO" |
| SPIDERPOOL_ENABLED_PPROF    | enable pprof for debug | 5721                                        |                             |
| SPIDERPOOL_ENABLED_METRIC   | enable metrics         | "true" or "false". default to "false"       |
| SPIDERPOOL_METRIC_HTTP_PORT | metrics port           | 5721                                        |
| SPIDERPOOL_HTTP_PORT        |                        |                                             |
