package dra

import (
	"github.com/vishvananda/netlink"
	resourceapi "k8s.io/api/resource/v1beta1"
	"k8s.io/utils/ptr"
)

type DeviceState struct {
}

// GetNetDevices get all net devices from the node, the attributes of every devices
// should be included but not limited to:
// isRdma, isSriov, gpuAffinity, ipaddress, macaddress, bandwidth
// type(ib/eth),vendor,device, pciAddress, etc.
func (d *DeviceState) GetNetDevices() []resourceapi.Device {
	links, err := netlink.LinkList()
	if err != nil {
		return nil
	}
	var devices []resourceapi.Device
	for _, link := range links {
		if link.Type() == "device" {
			devices = append(devices, resourceapi.Device{
				Name: link.Attrs().Name,
				Basic: &resourceapi.BasicDevice{
					Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
						"isRdma": resourceapi.DeviceAttribute{
							BoolValue: ptr.To(false),
						},
					},
				},
			})
		}
	}
	return devices
}
