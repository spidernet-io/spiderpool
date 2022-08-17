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
                "log_file_path":"/var/log/spidernet/spiderpool.log",
                "log_file_max_size":"100M",
                "log_file_max_age":"30d",
                "log_file_max_count":7,
                "log_level":"INFO"
            }
        }
    ]
}
```

- `log_file_path` (string, optional): Path to log file  of IPAM plugin, default to `"/var/log/spidernet/spiderpool.log"`.
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
  ipamUnixSocketPath: /var/run/spidernet/spiderpool.sock
  networkMode: legacy
  enableIPv4: true
  enableIPv6: true
  clusterDefaultIPv4IPPool: [default-v4-ippool]
  clusterDefaultIPv6IPPool: [default-v6-ippool]
```

- `ipamUnixSocketPath` (string): Spiderpool agent listens to this UNIX socket file and handle IPAM requests from IPAM plugin.
- `networkMode`:
  - `legacy`: Applicable to the traditional physical machine network.
- `enableIPv4` (bool):
  - `true`: Enable IPv4 IP allocation capability of Spiderpool.
  - `false`: Disable IPv4 IP allocation capability of Spiderpool.
- `enableIPv6` (bool):
  - `true`: Enable IPv6 IP allocation capability of Spiderpool.
  - `false`: Disable IPv6 IP allocation capability of Spiderpool.
- `clusterDefaultIPv4IPPool` (array): Global default IPv4 ippools. It takes effect throughout the cluster.
- `clusterDefaultIPv6IPPool` (array): Global default IPv6 ippools. It takes effect throughout the cluster.

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
