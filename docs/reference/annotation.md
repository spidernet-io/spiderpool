# Annotations

Spiderpool provides annotations for configuring custom IPPools and routes.

## Pod annotations

After enabling the feature [SpiderSubnet](../usage/spider-subnet.md) (default enabled after v0.4.0), the annotations related to Subnet will take effect. They always have higher priority than IPPool related annotations.

### ipam.spidernet.io/subnet

Specify the Subnets used to generate IPPools and allocate IP addresses.

```yaml
ipam.spidernet.io/subnet: |-
  {
    "ipv4": ["demo-v4-subnet1"],
    "ipv6": ["demo-v6-subnet1"]
  }
```

- `ipv4` (array, optional): Specify which Subnet is used to generate IPPool and allocate the IPv4 address. When `enableIPv4` in the ConfigMap `spiderpool-conf` is set to true, this field is required.
- `ipv6` (array, optional): Specify which Subnet is used to generate IPPool and allocate the IPv6 address. When `enableIPv6` in the ConfigMap `spiderpool-conf` is set to true, this field is required.

### ipam.spidernet.io/subnets

```yaml
ipam.spidernet.io/subnets: |-
  [{
      "interface": "eth0",
      "ipv4": ["demo-v4-subnet1"],
      "ipv6": ["demo-v6-subnet1"]
   },{
      "interface": "net1",
      "ipv4": ["demo-v4-subnet2"],
      "ipv6": ["demo-v6-subnet2"]
  }]
```

- `interface` (string, required): Since the CNI request only carries the information of one interface, the field `interface` shall be specified to distinguish in the case of multiple interfaces.
- `ipv4` (array, optional): Specify which Subnet is used to generate IPPool and allocate the IPv4 address. When `enableIPv4` in the ConfigMap `spiderpool-conf` is set to true, this field is required.
- `ipv6` (array, optional): Specify which Subnet is used to generate IPPool and allocate the IPv6 address. When `enableIPv6` in the ConfigMap `spiderpool-conf` is set to true, this field is required.

### ipam.spidernet.io/ippool-ip-number

This annotation is used with [SpiderSubnet](../usage/spider-subnet.md) feature enabled.
It specifies the IP numbers of the corresponding SpiderIPPool (fixed and flexible mode, optional and default '+1').

```yaml
ipam.spidernet.io/ippool-ip-number: +1
```

### ipam.spidernet.io/ippool-reclaim

This annotation is used with [SpiderSubnet](../usage/spider-subnet.md) feature enabled.
It specifies the corresponding SpiderIPPool to delete or not once the application was deleted (optional and default 'true').

```yaml
ipam.spidernet.io/ippool-reclaim: true
```

### ipam.spidernet.io/ippool

Specify the IPPools used to allocate IP addresses.

```yaml
ipam.spidernet.io/ippool: |-
  {
    "ipv4": ["demo-v4-ippool1"],
    "ipv6": ["demo-v6-ippool1", "demo-v6-ippool2"]
  }
```

- `ipv4` (array, optional): Specify which IPPool is used to allocate the IPv4 address. When `enableIPv4` in the ConfigMap `spiderpool-conf` is set to true, this field is required.
- `ipv6` (array, optional): Specify which IPPool is used to allocate the IPv6 address. When `enableIPv6` in the ConfigMap `spiderpool-conf` is set to true, this field is required.

### ipam.spidernet.io/ippools

It is similar to `ipam.spidernet.io/ippool` but could be used in the case with multiple interfaces. Note that `ipam.spidernet.io/ippools` has precedence over `ipam.spidernet.io/ippool`.

```yaml
ipam.spidernet.io/ippools: |-
  [{
      "interface": "eth0",
      "ipv4": ["demo-v4-ippool1"],
      "ipv6": ["demo-v6-ippool1"],
      "cleangateway": true
   },{
      "interface": "net1",
      "ipv4": ["demo-v4-ippool2"],
      "ipv6": ["demo-v6-ippool2"],
      "cleangateway": false
  }]
```

- `interface` (string, required): Since the CNI request only carries the information of one interface, the field `interface` shall be specified to distinguish in the case of multiple interfaces.
- `ipv4` (array, optional): Specify which IPPool is used to allocate the IPv4 address. When `enableIPv4` in the ConfigMap `spiderpool-conf` is set to true, this field is required.
- `ipv6` (array, optional): Specify which IPPool is used to allocate the IPv6 address. When `enableIPv6` in the ConfigMap `spiderpool-conf` is set to true, this field is required.
- `cleangateway` (bool, optional): If set to true, no gateway routing will take effect on this network card, regardless of whether a gateway IP is set in the IPPool of this network card.

### ipam.spidernet.io/routes

You can use the following code to enable additional routes take effect.

```yaml
ipam.spidernet.io/routes: |-
  [{
      "dst": "10.0.0.0/16",
      "gw": "192.168.1.1"
  },{
      "dst": "172.10.40.0/24",
      "gw": "172.18.40.1"
  }]
```

- `dst` (string, required): Network destination of the route.
- `gw` (string, required): The forwarding or next hop IP address.

## Namespace annotations

A Namespace can set the following annotations to specify default IPPools which are effective for all Pods under the Namespace.

### ipam.spidernet.io/default-ipv4-ippool

```yaml
ipam.spidernet.io/default-ipv4-ippool: '["ns-v4-ippool1","ns-v4-ippool2"]'
```

### ipam.spidernet.io/default-ipv6-ippool

```yaml
ipam.spidernet.io/default-ipv6-ippool: '["ns-v6-ippool1","ns-v6-ippool2"]'
```
