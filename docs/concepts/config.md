# Configuration

> Instructions for global configuration and environment arguments of Spiderpool.

## IPAM Plugin Configuration

Here is an example of IPAM configuration.

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
                "log_file_max_size":"100",
                "log_file_max_age":"30",
                "log_file_max_count":7,
                "log_level":"INFO",
                "default_ipv4_ippool": ["default-ipv4-pool1","default-ipv4-pool2"],
                "default_ipv6_ippool": ["default-ipv6-pool1","default-ipv6-pool2"]
            }
        }
    ]
}
```

- `log_file_path` (string, optional): Path to log file of IPAM plugin, default to `"/var/log/spidernet/spiderpool.log"`.
- `log_file_max_size` (string, optional): Max size of each rotated file, default to `"100"`(unit MByte).
- `log_file_max_age` (string, optional): Max age of each rotated file, default to `"30"`(unit Day).
- `log_file_max_count` (string, optional): Max number of rotated file, default to `"7"`.
- `log_level` (string, optional): Log level, default to `"INFO"`. It could be `"INFO"`, `"DEBUG"`, `"WARN"`, `"ERROR"`.
- `default_ipv4_ippool` (string array, optional): Default IPAM IPv4 Pool to use.
- `default_ipv6_ippool` (string array, optional): Default IPAM IPv6 Pool to use.

## Configmap Configuration

Configmap "spiderpool-conf" is the global configuration of Spiderpool.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: spiderpool-conf
  namespace: kube-system
data:
  conf.yml: |
    ipamUnixSocketPath: /var/run/spidernet/spiderpool.sock
    networkMode: legacy
    enableIPv4: true
    enableIPv6: true
    enableStatefulSet: true
    enableSpiderSubnet: true
    clusterDefaultIPv4IPPool: [default-v4-ippool]
    clusterDefaultIPv6IPPool: [default-v6-ippool]
    clusterDefaultIPv4Subnet: [default-v4-subnet]
    clusterDefaultIPv6Subnet: [default-v6-subnet]
    clusterSubnetDefaultFlexibleIPNumber: 1
```

- `ipamUnixSocketPath` (string): Spiderpool agent listens to this UNIX socket file and handles IPAM requests from IPAM plugin.
- `networkMode`:
  - `legacy`: Applicable to the traditional physical machine network.
- `enableIPv4` (bool):
  - `true`: Enable IPv4 IP allocation capability of Spiderpool.
  - `false`: Disable IPv4 IP allocation capability of Spiderpool.
- `enableIPv6` (bool):
  - `true`: Enable IPv6 IP allocation capability of Spiderpool.
  - `false`: Disable IPv6 IP allocation capability of Spiderpool.
- `enableStatefulSet` (bool):
  - `true`: Enable StatefulSet capability of Spiderpool.
  - `false`: Disable StatefulSet capability of Spiderpool.
- `enableSpiderSubnet` (bool):
  - `true`: Enable SpiderSubnet capability of Spiderpool.
  - `false`: Disable SpiderSubnet capability of Spiderpool.
- `clusterDefaultIPv4IPPool` (array): Global default IPv4 ippools. It takes effect across the cluster.
- `clusterDefaultIPv6IPPool` (array): Global default IPv6 ippools. It takes effect across the cluster.
- `clusterDefaultIPv4Subnet` (array): Global default IPv4 subnets. It takes effect across the cluster.
- `clusterDefaultIPv6Subnet` (array): Global default IPv6 subnets. It takes effect across the cluster.
- `clusterSubnetDefaultFlexibleIPNumber` (int): Global SpiderSubnet default flexible IP number. It takes effect across the cluster.

## Spiderpool-agent env

| env                                             | default | description                                                  |
| ----------------------------------------------- | ------- | ------------------------------------------------------------ |
| SPIDERPOOL_LOG_LEVEL                            | info    | Log level, optional values are "debug", "info", "warn", "error", "fatal", "panic". |
| SPIDERPOOL_ENABLED_METRIC                       | false   | Enable/disable metrics.                                   |
| SPIDERPOOL_HEALTH_PORT                          | 5710    | Metric HTTP server port.                                     |
| SPIDERPOOL_METRIC_HTTP_PORT                     | 5711    | Spiderpool-agent backend HTTP server port.                   |
| SPIDERPOOL_GOPS_LISTEN_PORT                     | 5712    | Port that gops is listening on. Disabled if empty.    |
| SPIDERPOOL_UPDATE_CR_MAX_RETRIES                 | 3       | Max retries to update k8s resources.                         |
| SPIDERPOOL_WORKLOADENDPOINT_MAX_HISTORY_RECORDS | 100     | Max historical IP allocation information allowed for a single Pod recorded in WorkloadEndpoint. |
| SPIDERPOOL_IPPOOL_MAX_ALLOCATED_IPS             | 5000    | Max number of IP that a single IP pool can provide.          |

## Spiderpool-controller env

| env                         | default | description                                                  |
| --------------------------- | ------- | ------------------------------------------------------------ |
| SPIDERPOOL_LOG_LEVEL        | info    | Log level, optional values are "debug", "info", "warn", "error", "fatal", "panic". |
| SPIDERPOOL_ENABLED_METRIC   | false   | Enable/disable metrics.                                   |
| SPIDERPOOL_HEALTH_PORT      | 5720    | Spiderpool-controller backend HTTP server port.              |
| SPIDERPOOL_METRIC_HTTP_PORT | 5721    | Metric HTTP server port.                                     |
| SPIDERPOOL_WEBHOOK_PORT     | 5722    | Webhook HTTP server port.                                    |
| SPIDERPOOL_CLI_PORT         | 5723    | Spiderpool-CLI HTTP server port.                             |
| SPIDERPOOL_GOPS_LISTEN_PORT | 5724    | Port that gops is listening on. Disabled if empty.    |
