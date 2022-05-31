# annotation

## Pod Annotation

Pod could specify spiderpool annotation for special request

* "ipam.spidernet.io/ippool", specify which ippool is used for the assigning IP.

        "ipam.spidernet.io/ippool": { "interface": "eth0",
                                      "ipv4pool": "v4pool1",
                                      "ipv6pool": "v6pool1,v6pool2"
                                    }

  * interface: optional, when integrate with [multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni), it could specify which ippool is used to the interface.
  
  * ipv4pool: specify which ippool is used to assign ipv4 ip. It could set with multiple ippool by comma-seperated string. When enableIpv4 in configmap "spiderpool-conf" is set to true, this filed is required.
  
  * ipv6pool: specify which ippool is used to assign ipv6 ip. It could set with multiple ippool by comma-seperated string. When enableIpv6 in configmap "spiderpool-conf" is set to true, this filed is required.

* "ipam.spidernet.io/ippools", this one is similar to "ipam.spidernet.io/ippool", but could be used to multiple interface case. BTW, the "ipam.spidernet.io/ippools" has precedence over "ipam.spidernet.io/ippool".

        "ipam.spidernet.io/ippools": [{ "interface": "eth0", "ipv4pool": "v4pool1", "ipv6pool": "v6pool1", "defaultRoute": true },
                                      { "interface": "eth1", "ipv4pool": "v4pool2", "ipv6pool": "v6pool2", "defaultRoute": false}]

  * defaultRoute: required, if set to be true, the IPAM plugin will return the default gateway route recorded in the ippool.

    limit:

  * For different interface, it is forbid to use ippools in a same subnet.

* "ipam.spidernet.io/routes", administrator could use this to take effect additional route.

        "ipam.spidernet.io/routes":[{"interface": "eth0", "dst": "10.0.0.0/16", "gw": "192.168.1.1"}]

* "ipam.spidernet.io/assigned-INTERFACE", this is the IP assigned result for an interface.

        "ipam.spidernet.io/assigned-eth0": { "interface": "eth0", "ipv4pool": "v4pool1", "ipv6pool": "v6pool1", 
                                             "ipv4": "172.16.0.100/16", "ipv6": "fd00::100/64", "vlan": 100 }

## Namespace Annotation

the namespace resource could set following annotation to specify default ippool, to override the default ippool of the cluster recorded in configmap "spiderpool-conf".

* "ipam.spidernet.io/defaultv4ippool"

        "ipam.spidernet.io/defaultv4ippool": [ "ipv4pool1","ipv4pool2"]

    notice: if multiple ippool are listed, it will try to assign IP from the later ippool when the former one is not allocatable.

* "ipam.spidernet.io/defaultv6ippool"

        "ipam.spidernet.io/defaultv6ippool": [ "ipv6pool1","ipv6pool2"]

    notice: if multiple ippool are listed, it will try to assign IP from the later ippool when the former one is not allocatable.
