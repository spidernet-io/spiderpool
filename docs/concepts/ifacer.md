# Meta plugin: Ifacer Configuration

The ifacer plugin helps users create VLAN sub-interfaces or bond devices based on the provided CNI configuration file. Here are some examples to show how to configure it:

## Create a vlan sub-interface base on master interface

The CR of multus's net-atta-def is configured like this:

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: ifacer-vlan120-underlay
  namespace: kube-system
spec:
  config: |-
    {
        "cniVersion": "0.3.1",
        "name": "macvlan-vlan120",
        "plugins": [
            {
                "type": "ifacer",
                "interfaces": ["ens160"],
                "vlanID": 120
            },
            {
                "type": "macvlan",
                "master": "ens160.120",
                "mode": "bridge",
                "ipam": {
                    "type": "spiderpool"
                }
            },{
                "type": "coordinator",
                "tune_mode": "underlay"
            }
        ]
    }
```

Fields description:

- plugins[0].type(string, required): ifacer, the name of plugin.
- plugins[0].interfaces([]string,required): ifacer create VLAN sub-interfaces based on this master interface. Note: When creating a VLAN sub-interface, the elements of the array must have only one master interface. And the interface exists on the host.
- plugins[0].vlanID: The VLAN tag of the VLAN sub-interface. Note: the value must be in range: [0,4094]. and "0" indicates that no VLAN sub-interfaces will be created.

Note: 

- The name of the created vlan sub-interface is spliced from the master interface and vlanId. The format is: "<master>.<vlanID>".
- If a VLAN sub-interface with the same name already exists on the node, ifacer checks if the interface is in the UP state. if not, sets to UP and exits.


## Create a bond device

The CR of multus's net-atta-def is configured like this:

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: ifacer-bond0-underlay
  namespace: kube-system
spec:
  config: |-
    {
        "cniVersion": "0.3.1",
        "name": "macvlan-bond0",
        "plugins": [
            {
                "type": "ifacer",
                "interfaces": ["ens160","ens192"],
                "vlanID": 120,
                "bond": {
                   "name": "bond0",
                   "mode": 0,
                   "options": "primary=ens160"
                 }
            },
            {
                "type": "macvlan",
                "master": "ens160.120",
                "mode": "bridge",
                "ipam": {
                    "type": "spiderpool"
                }
            },{
                "type": "coordinator",
                "tune_mode": "underlay"
            }
        ]
    }
```

Fields description:

- plugins[0].type(string, required): ifacer, the name of plugin.
- plugins[0].interfaces([]string,required): ifacer create VLAN sub-interfaces based on this master interface. Note: When creating a VLAN sub-interface, the elements of the array must have only one master interface. And the interface exists on the host.
- plugins[0].vlanID(int,optional): The VLAN tag of the VLAN sub-interface created based on the bond device. Note: the value must be in range: [0,4094]. and "0" indicates that no VLAN sub-interfaces will be created.
- plugins[0].bond.name(string,optional): the name of bond device, If not specified, the default is sp_bond0.
- plugins[0].bond.mode(string,optional): bond mode, the value must be in range: [0,6].
- plugins[0].bond.options(string,optional), bond options for the bonding driver are supplied as parameters to the bonding module at load time, or are specified via sysfs. Multiple parameters separated by ";"，input-formatted: "primary=ens160;arp_interval=1". More details see <https://www.kernel.org/doc/Documentation/networking/bonding.txt>.

Note:

- If a bond device with the same name already exists on the node, ifacer checks if the interface is bond type and in the UP state. if not bond type, return error; if not in UP state,sets to UP and exits.
- If a VLAN sub-interface created base on the bond device with the same name already exists on the node, ifacer checks if the interface is in the UP state. if not, sets to UP and exits.
