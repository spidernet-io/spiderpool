package dra

import (
	"net"

	"github.com/Mellanox/rdmamap"
	"github.com/spidernet-io/spiderpool/pkg/networking/networking"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
	resourceapi "k8s.io/api/resource/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"
)

type DeviceState struct {
	logger *zap.Logger
}

func (d *DeviceState) Init(logger *zap.Logger) (*DeviceState, error) {
	return &DeviceState{logger: logger}, nil
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
		isVirtual, err := networking.IsVirtualNetDevice(link.Attrs().Name)
		if err != nil {
			d.logger.Debug("Failed to check if netdev is virtual device", zap.String("netdev", link.Attrs().Name), zap.Error(err))
			continue
		}
		if isVirtual {
			d.logger.Sugar().Debugf("netdev %s is virtual device, skip add to resource slices", link.Attrs().Name)
			continue
		}

		isVf := networking.IsSriovVfForNetDev(link.Attrs().Name)
		if isVf {
			d.logger.Sugar().Debugf("netdev %s is sriov vf, skip to add to resource slices", link.Attrs().Name)
			continue
		}
		devices = append(devices, d.getNetDevice(link))
	}
	return devices
}

func (d *DeviceState) getNetDevice(link netlink.Link) resourceapi.Device {
	device := resourceapi.Device{
		Name: link.Attrs().Name,
		Basic: &resourceapi.BasicDevice{
			Attributes: make(map[resourceapi.QualifiedName]resourceapi.DeviceAttribute),
			Capacity:   make(map[resourceapi.QualifiedName]resourceapi.DeviceCapacity),
		},
	}

	// make sure the ifname is an valid dns1123 label, if not normalize it
	if len(validation.IsDNS1123Label(link.Attrs().Name)) > 0 {
		device.Name = NormalizedDNS1123Label(link.Attrs().Name)
		d.logger.Sugar().Debugf("iface %s is invalid DNS1123 label, normalized to %s", link.Attrs().Name, device.Name)
	}

	d.addBasicAttributesForNetDev(link, device.Basic)
	d.addGPUAffinityAttributesForNetDev(link.Attrs().Name, device.Basic)
	// pci attributes
	d.addPCIAttributesForNetDev(link.Attrs().Name, device.Basic)
	// bandwidth attributes
	d.addBandwidthAttributesForNetDev(link.Attrs().Name, device.Basic)
	d.addSpiderMultusConfigAttributesForNetDev(link.Attrs().Name, device.Basic)
	return device
}

func (d *DeviceState) addPCIAttributesForNetDev(iface string, device *resourceapi.BasicDevice) {
	// get vendor id, device id and pci address from sysfs
	deviceId, err := networking.GetPciDeviceIdForNetDev(iface)
	if err != nil {
		d.logger.Error("Failed to get PCI deviceId for netdev", zap.String("iface", iface), zap.Error(err))
	}
	device.Attributes["device"] = resourceapi.DeviceAttribute{StringValue: ptr.To(deviceId)}

	vendor, err := networking.GetPciVendorForNetDev(iface)
	if err != nil {
		d.logger.Error("Failed to get PCI vendor for netdev", zap.String("iface", iface), zap.Error(err))
	}
	device.Attributes["vendor"] = resourceapi.DeviceAttribute{StringValue: ptr.To(vendor)}

	// get pci address from sysfs
	pciAddress, err := networking.GetPciAddessForNetDev(iface)
	if err != nil {
		d.logger.Error("Failed to get PCI address for netdev", zap.String("iface", iface), zap.Error(err))
	}
	device.Attributes["pciAddress"] = resourceapi.DeviceAttribute{StringValue: ptr.To(pciAddress)}

	// sriov-related attributes
	// first check if the netdev is sriov pf or sriov vf
	isSriovPf, err := networking.IsSriovPfForNetDev(iface)
	if err != nil {
		d.logger.Sugar().Debugf("Failed to check if netdev %s is sriov pf", iface, zap.Error(err))
	} else if isSriovPf {
		// get sriov vf totalcount
		totalVfs, err := networking.GetSriovTotalVfsForNetDev(iface)
		if err != nil {
			d.logger.Error("Failed to get sriov vf count for netdev", zap.String("iface", iface), zap.Error(err))
		}

		device.Capacity = map[resourceapi.QualifiedName]resourceapi.DeviceCapacity{
			"totalVfs": {
				Value: *resource.NewQuantity(int64(totalVfs), resource.DecimalSI),
			},
		}

		// deviceVfList, err := networking.GetVFList(pciAddress)
		// if err != nil {
		// 	d.logger.Error("Failed to get sriov vf list for netdev", zap.String("iface", iface), zap.Error(err))
		// }
		// NOTE: spec.devices[5].basic.attributes[vfPciAddresses].string: Too long: may not be more than 64 bytes"
		// device.Attributes["allVfPciAddresses"] = resourceapi.DeviceAttribute{StringValue: ptr.To(strings.Join(deviceVfList, ","))}

		// get available vf pci addresses
		availableVfPciAddresses, err := networking.GetSriovAvailableVfPciAddressesForNetDev(pciAddress)
		if err != nil {
			d.logger.Error("Failed to get available sriov vf pci addresses for netdev", zap.String("iface", iface), zap.Error(err))
		} else {
			// // get available vf count
			device.Attributes["availableVfCount"] = resourceapi.DeviceAttribute{IntValue: ptr.To(int64(len(availableVfPciAddresses)))}
		}
		// the value Must not be longer than 64 characters
		// device.Attributes["availableVfPciAddresses"] = resourceapi.DeviceAttribute{StringValue: ptr.To(strings.Join(availableVfPciAddresses, ","))}
	}
}

func (d *DeviceState) addBasicAttributesForNetDev(link netlink.Link, device *resourceapi.BasicDevice) {
	linkAttrs := link.Attrs()
	device.Attributes["name"] = resourceapi.DeviceAttribute{StringValue: ptr.To(linkAttrs.Name)}
	device.Attributes["mtu"] = resourceapi.DeviceAttribute{IntValue: ptr.To(int64(linkAttrs.MTU))}
	device.Attributes["state"] = resourceapi.DeviceAttribute{StringValue: ptr.To(linkAttrs.OperState.String())}
	device.Attributes["mac"] = resourceapi.DeviceAttribute{StringValue: ptr.To(linkAttrs.HardwareAddr.String())}
	isRDMA := rdmamap.IsRDmaDeviceForNetdevice(linkAttrs.Name)
	device.Attributes["rdma"] = resourceapi.DeviceAttribute{BoolValue: &isRDMA}

	d.addIPAddressAttributesForNetDev(link, device)
}

func (d *DeviceState) addIPAddressAttributesForNetDev(link netlink.Link, device *resourceapi.BasicDevice) {
	addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		d.logger.Sugar().Errorf("Failed to get addresses for netdev %s", link.Attrs().Name, zap.Error(err))
		device.Attributes["ipv4CIDR"] = resourceapi.DeviceAttribute{StringValue: ptr.To("")}
		device.Attributes["ipv6CIDR"] = resourceapi.DeviceAttribute{StringValue: ptr.To("")}
		return
	}

	for _, addr := range addrs {
		if addr.IP.IsMulticast() || addr.IP.IsLinkLocalUnicast() {
			continue
		}

		// addr.IPNet.String() => 10.6.1.1/24, not 10.6.1.0/24
		ipNetString := addr.IPNet.String()
		_, ipnet, err := net.ParseCIDR(ipNetString)
		if err != nil {
			d.logger.Sugar().Errorf("Failed to parse CIDR for netdev %s", link.Attrs().Name, zap.Error(err))
			continue
		}

		if ipnet.IP.To4() != nil {
			device.Attributes["ipv4CIDR"] = resourceapi.DeviceAttribute{StringValue: ptr.To(ipnet.String())}
		}
		if ipnet.IP.To4() == nil {
			device.Attributes["ipv6CIDR"] = resourceapi.DeviceAttribute{StringValue: ptr.To(ipnet.String())}
		}
	}
}

func (d *DeviceState) addBandwidthAttributesForNetDev(iface string, device *resourceapi.BasicDevice) {
	bandwidth, err := networking.GetNetdevBandwidth(iface)
	if err != nil {
		d.logger.Sugar().Debugf("Failed to get bandwidth for netdev %s: %v", iface, err)
		// Set default values if we can't get the real bandwidth
		device.Attributes["bandwidthMbps"] = resourceapi.DeviceAttribute{IntValue: ptr.To(int64(0))}
		return
	}

	// Calculate bandwidth based on speed and duplex mode
	device.Attributes["bandwidthMbps"] = resourceapi.DeviceAttribute{IntValue: ptr.To(int64(bandwidth))}
}

func (d *DeviceState) addGPUAffinityAttributesForNetDev(iface string, device *resourceapi.BasicDevice) {
	// TODO(@cyclinder): gpu topo attributes
	device.Attributes["PIXAffinityGpus"] = resourceapi.DeviceAttribute{StringValue: ptr.To("")}
	device.Attributes["PHBAffinityGpus"] = resourceapi.DeviceAttribute{StringValue: ptr.To("")}
	device.Attributes["SYSAffinityGpus"] = resourceapi.DeviceAttribute{StringValue: ptr.To("")}
	device.Attributes["NODEAffinityGpus"] = resourceapi.DeviceAttribute{StringValue: ptr.To("")}
}

func (d *DeviceState) addSpiderMultusConfigAttributesForNetDev(iface string, device *resourceapi.BasicDevice) {
	// TODO(@cyclinder): spider multus config attributes
	device.Attributes["multusConfigs"] = resourceapi.DeviceAttribute{StringValue: ptr.To("")}
}
