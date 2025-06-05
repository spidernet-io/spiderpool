package dra

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"

	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	crdclientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	"github.com/spidernet-io/spiderpool/pkg/networking/networking"
	"github.com/spidernet-io/spiderpool/pkg/utils"

	"go.uber.org/zap"

	"github.com/Mellanox/rdmamap"
	"github.com/vishvananda/netlink"
	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

type DeviceState struct {
	namespace    string
	logger       *zap.Logger
	spiderClient crdclientset.Interface
	client       client.Client
}

func (d *DeviceState) Init(logger *zap.Logger, client client.Client) (*DeviceState, error) {
	d.namespace = utils.GetAgentNamespace()
	d.logger = logger
	d.client = client
	return d, nil
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
		// skip virtual device but not vlan type
		if isVirtual && (link.Type() != "vlan") {
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
		Name:                     link.Attrs().Name,
		Attributes:               make(map[resourceapi.QualifiedName]resourceapi.DeviceAttribute),
		Capacity:                 make(map[resourceapi.QualifiedName]resourceapi.DeviceCapacity),
		AllowMultipleAllocations: ptr.To(true),
	}

	// make sure the ifname is an valid dns1123 label, if not normalize it
	if len(validation.IsDNS1123Label(link.Attrs().Name)) > 0 {
		device.Name = NormalizedDNS1123Label(link.Attrs().Name)
		d.logger.Sugar().Debugf("iface %s is invalid DNS1123 label, normalized to %s", link.Attrs().Name, device.Name)
	}

	d.addBasicAttributesForNetDev(link, device.Attributes)
	d.addRDMATopoAttributes(link.Attrs().Name, device.Attributes)
	// pci attributes
	d.addPCIAttributesForNetDev(link.Attrs().Name, device.Attributes)
	// bandwidth attributes
	d.addBandwidthAttributesForNetDev(link.Attrs().Name, device.Attributes)
	d.addSpiderMultusConfigAttributesForNetDev(link.Attrs().Name, device.Attributes)
	return device
}

func (d *DeviceState) addPCIAttributesForNetDev(iface string, device map[resourceapi.QualifiedName]resourceapi.DeviceAttribute) {
	// get vendor id, device id and pci address from sysfs
	// deviceId, err := networking.GetPciDeviceIdForNetDev(iface)
	// if err != nil {
	// 	d.logger.Error("Failed to get PCI deviceId for netdev", zap.String("iface", iface), zap.Error(err))
	// }
	// device.Attributes["device"] = resourceapi.DeviceAttribute{StringValue: ptr.To(deviceId)}

	// vendor, err := networking.GetPciVendorForNetDev(iface)
	// if err != nil {
	// 	d.logger.Error("Failed to get PCI vendor for netdev", zap.String("iface", iface), zap.Error(err))
	// }
	// device.Attributes["vendor"] = resourceapi.DeviceAttribute{StringValue: ptr.To(vendor)}

	// get pci address from sysfs
	pciAddress, err := networking.GetPciAddessForNetDev(iface)
	if err != nil {
		d.logger.Error("Failed to get PCI address for netdev", zap.String("iface", iface), zap.Error(err))
	}
	device["pciAddress"] = resourceapi.DeviceAttribute{StringValue: ptr.To(pciAddress)}

	// sriov-related attributes
	// first check if the netdev is sriov pf or sriov vf
	isSriovPf, err := networking.IsSriovPfForNetDev(iface)
	if err != nil {
		d.logger.Sugar().Debugf("Failed to check if netdev %s is sriov pf", iface, zap.Error(err))
	}

	// get sriov vf totalcount
	totalVfs, err := networking.GetSriovTotalVfsForNetDev(iface)
	if err != nil {
		d.logger.Error("Failed to get sriov vf count for netdev", zap.String("iface", iface), zap.Error(err))
	}

	device["totalVfs"] = resourceapi.DeviceAttribute{
		IntValue: ptr.To(int64(totalVfs)),
	}

	if isSriovPf {
		// get available vf pci addresses
		availableVfPciAddresses, err := networking.GetSriovAvailableVfPciAddressesForNetDev(iface)
		if err != nil {
			d.logger.Error("Failed to get available sriov vf pci addresses for netdev", zap.String("iface", iface), zap.Error(err))
		}
		// get available vf count
		device["availableVfs"] = resourceapi.DeviceAttribute{IntValue: ptr.To(int64(len(availableVfPciAddresses)))}
	}

	// device.Attributes["vfPciAddressPrefix"] = resourceapi.DeviceAttribute{StringValue: ptr.To(GetPciAddressPrefix(pciAddress))}
	// deviceVfList, err := networking.GetVFList(pciAddress)
	// if err != nil {
	// 	d.logger.Error("Failed to get sriov vf list for netdev", zap.String("iface", iface), zap.Error(err))
	// }
	// // NOTE: spec.devices[5].basic.attributes[vfPciAddresses].string: Too long: may not be more than 64 bytes"
	// device.Attributes["allVfPciAddressSuffix"] = resourceapi.DeviceAttribute{StringValue: ptr.To(strings.Join(deviceVfList, ","))}

	// // the value Must not be longer than 64 characters
	// device.Attributes["availableVfPciAddressSuffix"] = resourceapi.DeviceAttribute{StringValue: ptr.To(strings.Join(availableVfPciAddresses, ","))}
}

func (d *DeviceState) addBasicAttributesForNetDev(link netlink.Link, device map[resourceapi.QualifiedName]resourceapi.DeviceAttribute) {
	linkAttrs := link.Attrs()
	device["linkType"] = resourceapi.DeviceAttribute{StringValue: ptr.To(link.Type())}
	if link.Type() == "device" {
		device["linkType"] = resourceapi.DeviceAttribute{StringValue: ptr.To("ethernet")}
	}
	device["name"] = resourceapi.DeviceAttribute{StringValue: ptr.To(linkAttrs.Name)}
	device["mtu"] = resourceapi.DeviceAttribute{IntValue: ptr.To(int64(linkAttrs.MTU))}
	device["state"] = resourceapi.DeviceAttribute{StringValue: ptr.To(linkAttrs.OperState.String())}
	device["mac"] = resourceapi.DeviceAttribute{StringValue: ptr.To(linkAttrs.HardwareAddr.String())}
	isRDMA := rdmamap.IsRDmaDeviceForNetdevice(linkAttrs.Name)
	if isRDMA {
		d.addGPUAffinityAttributesForNetDev(link.Attrs().Name, device)
	}
	device["rdma"] = resourceapi.DeviceAttribute{BoolValue: &isRDMA}
	d.addIPAddressAttributesForNetDev(link, device)
}

func (d *DeviceState) addIPAddressAttributesForNetDev(link netlink.Link, device map[resourceapi.QualifiedName]resourceapi.DeviceAttribute) {
	addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		d.logger.Sugar().Errorf("Failed to get addresses for netdev %s", link.Attrs().Name, zap.Error(err))
		device["ipv4CIDR"] = resourceapi.DeviceAttribute{StringValue: ptr.To("")}
		device["ipv6CIDR"] = resourceapi.DeviceAttribute{StringValue: ptr.To("")}
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
			device["ipv4CIDR"] = resourceapi.DeviceAttribute{StringValue: ptr.To(ipnet.String())}
		}
		if ipnet.IP.To4() == nil {
			device["ipv6CIDR"] = resourceapi.DeviceAttribute{StringValue: ptr.To(ipnet.String())}
		}
	}
}

func (d *DeviceState) addBandwidthAttributesForNetDev(iface string, device map[resourceapi.QualifiedName]resourceapi.DeviceAttribute) {
	bandwidth, err := networking.GetNetdevBandwidth(iface)
	if err != nil {
		d.logger.Sugar().Debugf("Failed to get bandwidth for netdev %s: %v", iface, err)
	}

	device["bandwidthGbps"] = resourceapi.DeviceAttribute{
		StringValue: ptr.To(fmt.Sprintf("%d", bandwidth/1000)),
	}
}

func (d *DeviceState) addRDMATopoAttributes(iface string, device map[resourceapi.QualifiedName]resourceapi.DeviceAttribute) {
	device["topoZone"] = resourceapi.DeviceAttribute{StringValue: ptr.To("")}
}

func (d *DeviceState) addGPUAffinityAttributesForNetDev(iface string, device map[resourceapi.QualifiedName]resourceapi.DeviceAttribute) {
	gdrGpus, err := networking.GetGdrGpusForNetDevice(iface)
	if err != nil {
		d.logger.Sugar().Errorf("Failed to get GDR GPUs for netdev %s: %v", iface, err)
	}
	device["gdrAffinityGpus"] = resourceapi.DeviceAttribute{StringValue: ptr.To(strings.Join(gdrGpus, ","))}
	//device.Attributes["PHBAffinityGpus"] = resourceapi.DeviceAttribute{StringValue: ptr.To("")}
	// device.Attributes["SYSAffinityGpus"] = resourceapi.DeviceAttribute{StringValue: ptr.To("")}
	// device.Attributes["NODEAffinityGpus"] = resourceapi.DeviceAttribute{StringValue: ptr.To("")}
}

func (d *DeviceState) addSpiderMultusConfigAttributesForNetDev(iface string, device map[resourceapi.QualifiedName]resourceapi.DeviceAttribute) {
	// TODO(@cyclinder): spider multus config attributes
	var cniConfigs []string

	// Use client interface instead of direct API calls to reduce API pressure
	var configList spiderpoolv2beta1.SpiderMultusConfigList
	if err := d.client.List(context.Background(), &configList, &client.ListOptions{Namespace: d.namespace}); err != nil {
		d.logger.Sugar().Errorf("Failed to list spider multus configs: %v", err)
		device["cniConfigs"] = resourceapi.DeviceAttribute{StringValue: ptr.To("")}
		return
	}

	// Match spider multus config name with netdev name
	// e.g. enp11s0f0np0-macvlan0, enp11s0f1np1-sriov1
	pattern := regexp.MustCompile(fmt.Sprintf(".*%s.*", regexp.QuoteMeta(iface)))
	for _, config := range configList.Items {
		if pattern.MatchString(config.Name) {
			cniConfigs = append(cniConfigs, config.Name)
		}
	}
	device["cniConfigs"] = resourceapi.DeviceAttribute{StringValue: ptr.To(strings.Join(cniConfigs, ","))}
}
