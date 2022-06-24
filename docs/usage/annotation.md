# annotation

>Spiderpool provides annotations for configuring custom ippools and routes.

## Pod Annotation

Pod could specify Spiderpool annotations for special request.

### ipam.spidernet.io/ippool

Specify which ippool is used for the assigning IP.

```yaml
ipam.spidernet.io/ippool: |-
  {
    "interface": "eth0",
    "ipv4pools": ["v4pool1"],
    "ipv6pools": ["v6pool1", "v6pool2"]
  }
```

- `interface` (string, optional): When integrate with [multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni), it could specify which ippool is used to the interface. The interface information in the CNI request will be used as the default value when this field is not specified.
- `ipv4pools` (array, optional): Specify which ippool is used to assign IPv4 IP. When `enableIPv4` in Configmap `spiderpool-conf` is set to true,  this filed must be specified.
- `ipv6pools` (array, optional): Specify which ippool is used to assign IPv6 IP. When `enableIPv6` in Configmap `spiderpool-conf` is set to true, this filed must be specified.

### ipam.spidernet.io/ippools

It is similar to `ipam.spidernet.io/ippool`, but could be used to multiple interface case. BTW, `ipam.spidernet.io/ippools` has precedence over `ipam.spidernet.io/ippool`.

```yaml
ipam.spidernet.io/ippools: |-
  [{
      "interface": "eth0",
      "ipv4pools": ["v4pool1"],
      "ipv6pools": ["v6pool1"],
      "defaultRoute": true,
   },{
      "interface": "eth1",
      "ipv4pools": ["v4pool2"],
      "ipv6pools": ["v6pool2"],
      "defaultRoute": false
  }]
```

- `interface` (string, required): Because of CNI request that only carry information of one interface, the interface field must be specified in the case of multiple interface case.
- `ipv4pools` (array, optional): Specify which ippool is used to assign IPv4 IP. When `enableIPv4` in Configmap `spiderpool-conf` is set to true, this filed must be specified.
- `ipv6pools` (array, optional): Specify which ippool is used to assign IPv6 IP. When `enableIPv6` in Configmap `spiderpool-conf` is set to true, this filed must be specified.
- `defaultRoute` (bool, optional): If set to be true, the IPAM plugin will return the default gateway route recorded in the ippool.

For different interface, it is forbid to use ippools in a same subnet.

### ipam.spidernet.io/routes

Users could use this to take effect additional routes.

```yaml
ipam.spidernet.io/routes: |-
  [{
      "interface": "eth0",
      "dst": "10.0.0.0/16",
      "gw": "192.168.1.1",
  }]
```

- `interface` (string, required): The name of the interface over which the destination is reachable.
- `dst` (string, required): Network destination of the route.
- `gw` (string, required): The forwarding or next hop IP address.

### ipam.spidernet.io/assigned-{INTERFACE}

It is the IP assigned result of the interface. It is only used by Spiderpool, not reserved for users.

```yaml
ipam.spidernet.io/assigned-eth0: |-
  {
    "interface": "eth0",
    "ipv4pool": "v4pool1",
    "ipv6pool": "v6pool1",
    "ipv4": "172.16.0.100/16",
    "ipv6": "fd00::100/64",
    "vlan": 100,
  }
```

## Namespace Annotation

Namespace could set following annotations to specify default ippools, to override the default ippools of the cluster recorded in Configmap `spiderpool-conf`.

### ipam.spidernet.io/defaultv4ippool

```yaml
ipam.spidernet.io/defaultv4ippool: ["v4pool1","v4pool2"]
```

If multiple ippools are listed, it will try to assign IP from the later ippool when the former one is not allocatable.

### ipam.spidernet.io/defaultv6ippool

```yaml
ipam.spidernet.io/defaultv6ippool: ["v6pool1","v6pool2"]
```

If multiple ippools are listed, it will try to assign IP from the later ippool when the former one is not allocatable.
