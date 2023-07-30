# IPAM Plugin Configuration

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
