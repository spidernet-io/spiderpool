# Annotations

Spiderpool provides annotations for configuring custom ippools and routes.

## Pod annotations

For a pod, you can specify Spiderpool annotations for a special request.

### ipam.spidernet.io/ippool

Specify the ippools used to allocate IP.

```yaml
ipam.spidernet.io/ippool: |-
  {
    "interface": "eth0",
    "ipv4pools": ["v4-ippool1"],
    "ipv6pools": ["v6-ippool1", "v6-ippool2"]
  }
```

- `interface` (string, optional): When integrated with [multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni), it could specify which ippool is used to the interface. The interface information in the CNI request will be used as the default value when this field is not specified.
- `ipv4pools` (array, optional): Specify which ippool is used to allocate the IPv4 address. When `enableIPv4` in the `spiderpool-conf` ConfigMap is set to true, this field is required.
- `ipv6pools` (array, optional): Specify which ippool is used to allocate the IPv6 address. When `enableIPv6` in the `spiderpool-conf` ConfigMap is set to true, this field is required.

### ipam.spidernet.io/ippools

It is similar to `ipam.spidernet.io/ippool` but could be used in the case with multiple interfaces. Note that `ipam.spidernet.io/ippools` has precedence over `ipam.spidernet.io/ippool`.

```yaml
ipam.spidernet.io/ippools: |-
  [{
      "interface": "eth0",
      "ipv4pools": ["v4-ippool1"],
      "ipv6pools": ["v6-ippool1"],
      "defaultRoute": true
   },{
      "interface": "eth1",
      "ipv4pools": ["v4-ippool2"],
      "ipv6pools": ["v6-ippool2"],
      "defaultRoute": false
  }]
```

- `interface` (string, required): Since the CNI request only carries the information of one interface, the `interface` field shall be specified to distinguish in the case of multiple interfaces.
- `ipv4pools` (array, optional): Specify which ippool is used to allocate the IPv4 address. When `enableIPv4` in the `spiderpool-conf` ConfigMap is set to true, this field is required.
- `ipv6pools` (array, optional): Specify which ippool is used to allocate the IPv6 address. When `enableIPv6` in the `spiderpool-conf` ConfigMap is set to true, this field is required.
- `defaultRoute` (bool, optional): If set to true, the IPAM plugin will return the default gateway route recorded in the ippool.

For different interfaces, it is not recommended to use ippools of the same subnet.

### ipam.spidernet.io/routes

You can use the following code to enable additional routes take effect.

```yaml
ipam.spidernet.io/routes: |-
  [{
      "interface": "eth0",
      "dst": "10.0.0.0/16",
      "gw": "192.168.1.1"
  }]
```

- `interface` (string, required): The name of the interface over which the destination is reachable.
- `dst` (string, required): Network destination of the route.
- `gw` (string, required): The forwarding or next hop IP address.

### ipam.spidernet.io/assigned-{INTERFACE}

It is the IP allocation result of the interface. It is only used by Spiderpool, not reserved for users.

```yaml
ipam.spidernet.io/assigned-eth0: |-
  {
    "interface": "eth0",
    "ipv4pool": "v4-ippool1",
    "ipv6pool": "v6-ippool1",
    "ipv4": "172.16.0.100/16",
    "ipv6": "fd00::100/64",
    "vlan": 100
  }
```

## Namespace annotations

Namespace could set following annotations to specify default ippools. They are valid for all Pods under the Namespace.

### ipam.spidernet.io/defaultv4ippool

```yaml
ipam.spidernet.io/defaultv4ippool: '["ns-v4-ippool1","ns-v4-ippool2"]'
```

If multiple ippools are listed, it will try to allocate IP from the later ippool when the former one is not allocatable.

### ipam.spidernet.io/defaultv6ippool

```yaml
ipam.spidernet.io/defaultv6ippool: '["ns-v6-ippool1","ns-v6-ippool2"]'
```

For other procedure, similar to [Pod Annotations](#pod-annotations) described above.
