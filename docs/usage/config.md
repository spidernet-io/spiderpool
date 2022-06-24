# Configuration

>Instructions for global configuration and environment args of Spiderpool.

## IPAM Plugin Configuration

There is an example of IPAM configuration.

```json
{
    "cniVersion":"0.3.1",
    "name":"macvlan-pod-network",
    "plugins":[
        {
            "name":"macvlan-pod-network",
            "type":"macvlan",
            "master":"ens256",
            "mode":"bridge",
            "mtu":1500,
            "ipam":{
                "type":"spiderpool",
                "log_file_path":"/var/run/spidernet/spiderpool.log",
                "log_file_max_size":"100M",
                "log_file_max_age":"30d",
                "log_file_max_count":7,
                "log_level":"INFO"
            }
        }
    ]
}
```

- `log_file_path` (string, optional): Path to log file  of IPAM plugin, default to `"/var/run/spidernet/spiderpool.log"`.
- `log_file_max_size` (string, optional): Max size of each rotated file, default to `"100M"`.
- `log_file_max_age` (string, optional): Max age of each rotated file, default to `"30d"`.
- `log_file_max_count` (string, optional): Max number of rotated file, default to `"7"`.
- `log_level` (string, optional): Log level, default to `"INFO"`. It could be `"INFO"`, `"DEBUG"`, `"WARN"`, `"ERROR"`.

## Configmap Configuration

Configmap "spiderpool-conf" is the global configuration of Spiderpool.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: spiderpool-conf
  namespace: kube-system
data:
  ipamUnixSocketPath: "/var/run/spidernet/spiderpool.sock"
  networkMode: legacy
  enableIPv4: true
  enableIPv6: true
  clusterDefaultIPv4IPPool: ["v4pool1"]
  clusterDefaultIPv6IPPool: ["v6pool1"]
```

- `ipamUnixSocketPath` (string): Spiderpool agent will listen on this unix socket file, and handle IPAM request from the IPAM plugin.
- `networkMode`:
  - `legacy`: Applicable to the traditional physical machine network.
- `enableIPv4` (bool):
  - `true`: Spiderpool will assign ipv4 IP, if fail to assign an IPv4 IP, the IPAM plugin will fail when pod creating.
  - `false`: Spiderpool will ignore assigning IPv4 IP.
- `enableIPv6` (bool):
  - `true`: Spiderpool will assign IPv6 IP, if fail to assign an IPv6 IP, the IPAM plugin will fail when pod creating.
  - `false`: Spiderpool will ignore assigning IPv6 IP.
- `clusterDefaultIPv4IPPool` (array): Global default ippools of IPv4, it could set to multiple ippools in backup case. Notice, the IP version of these ippools must be IPv4.
- `clusterDefaultIPv6IPPool` (array): Global default ippools of ipv6, it could set to multiple ippools in backup case. Notice, the IP version of these ippools must be IPv6.

## Spiderpool-agent env

| env                                             | default | description                                                  |
| ----------------------------------------------- | ------- | ------------------------------------------------------------ |
| SPIDERPOOL_LOG_LEVEL                            | info    | Log level, optional values are "debug", "info", "warn", "error", "fatal", "panic". |
| SPIDERPOOL_ENABLED_METRIC                       | false   | Whether to enable metrics.                                   |
| SPIDERPOOL_HEALTH_PORT                          | 5710    | Metric HTTP server port.                                     |
| SPIDERPOOL_METRIC_HTTP_PORT                     | 5711    | Spiderpool-agent backend HTTP server port.                   |
| SPIDERPOOL_GOPS_LISTEN_PORT                     | 5712    | Port that gops is listen on , set to empty to disable it.    |
| SPIDERPOOL_UPDATE_CR_MAX_RETRYS                 | 3       | Max retries to update k8s resources.                         |
| SPIDERPOOL_WORKLOADENDPOINT_MAX_HISTORY_RECORDS | 100     | Max historical IP allocation information allowed for a single Pod recorded in WorkloadEndpoint. |
| SPIDERPOOL_IPPOOL_MAX_ALLOCATED_IPS             | 5000    | Max number of IP that a single IP pool can provide.          |

## Spiderpool-controller env

| env                         | default | description                                                  |
| --------------------------- | ------- | ------------------------------------------------------------ |
| SPIDERPOOL_LOG_LEVEL        | info    | Log level, optional values are "debug", "info", "warn", "error", "fatal", "panic". |
| SPIDERPOOL_ENABLED_METRIC   | false   | Whether to enable metrics.                                   |
| SPIDERPOOL_HEALTH_PORT      | 5720    | Spiderpool-controller backend HTTP server port.              |
| SPIDERPOOL_METRIC_HTTP_PORT | 5721    | Metric HTTP server port.                                     |
| SPIDERPOOL_WEBHOOK_PORT     | 5722    | Webhook HTTP server port.                                    |
| SPIDERPOOL_CLI_PORT         | 5723    | Spiderpool-CLI HTTP server port.                             |
| SPIDERPOOL_GOPS_LISTEN_PORT | 5724    | Port that gops is listen on , set to empty to disable it.    |
