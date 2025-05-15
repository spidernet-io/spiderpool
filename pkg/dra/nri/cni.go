package nri

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/containernetworking/cni/libcni"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	cni100 "github.com/containernetworking/cni/pkg/types/100"
	netutils "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/utils"
)

type NetworkStatus struct {
	Name       string      `json:"name"`
	Interface  string      `json:"interface"`
	IPs        []string    `json:"ips"`
	Mac        string      `json:"mac"`
	Gateway    []string    `json:"gateway"`
	DeviceInfo *DeviceInfo `json:"deviceInfo"`
}

type DeviceInfo struct {
	Iface           string
	PciAddress      string   `json:"pciAddress,omitempty"`
	RdmaDevice      string   `json:"rdma-device,omitempty"`
	RdmaCharDevices []string `json:"rdma-char-device,omitempty"`
}

func (ns *NetworkStatus) parseNetworkStatus(result *cni100.Result) {
	if result == nil {
		return
	}

	indexToIPs := make(map[int]*cni100.IPConfig, len(result.IPs))
	for _, ips := range result.IPs {
		indexToIPs[*ips.Interface] = ips
	}

	for index, i := range result.Interfaces {
		ips := indexToIPs[index]
		ns.IPs = append(ns.IPs, ips.Address.IP.String())
		ns.Mac = i.Mac
		ns.Gateway = append(ns.Gateway, ips.Gateway.String())
	}
	return
}

func cniAdd(ctx context.Context, rawnetconflist []byte) (cnitypes.Result, error) {
	cniConfig := libcni.NewCNIConfigWithCacheDir([]string{defaultCniBinPath}, defaultCniResultCacheDir, nil)

	confList, err := libcni.ConfListFromBytes(rawnetconflist)
	if err != nil {
		return nil, fmt.Errorf("error converting the raw bytes into a conflist: %v", err)
	}

	result, err := cniConfig.AddNetworkList(ctx, confList, nil)
	if err != nil {
		return nil, fmt.Errorf("error adding network list: %v", err)
	}

	return result, nil
}

func buildSecondaryCniConfig(vfDeviceId, config string) (*libcni.NetworkConfigList, error) {
	if vfDeviceId == "" || config == "" {
		return nil, fmt.Errorf("vfDeviceId or config is empty")
	}

	configBytes, err := netutils.GetCNIConfigFromSpec(config, "")
	if err != nil {
		return nil, fmt.Errorf("error getting CNI config from spec: %v", err)
	}

	configBytes, err = appendDeviceIDToRawCniConfigBytes(configBytes, vfDeviceId)
	if err != nil {
		return nil, fmt.Errorf("error appending device ID to raw CNI config bytes: %v", err)
	}

	confList, err := libcni.ConfListFromBytes(configBytes)
	if err != nil {
		return nil, fmt.Errorf("error converting the raw bytes into a config: %v", err)
	}

	return confList, nil
}

func appendDeviceIDToRawCniConfigBytes(rawCniConfigBytes []byte, vfDeviceId string) ([]byte, error) {
	if rawCniConfigBytes == nil || vfDeviceId == "" {
		return nil, fmt.Errorf("rawCniConfigBytes or vfDeviceId is empty")
	}

	var rawConfig map[string]interface{}
	var err error

	err = json.Unmarshal(rawCniConfigBytes, &rawConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw CNI config: %v", err)
	}

	// Inject VF device ID
	rawConfig["deviceID"] = vfDeviceId
	rawConfig["picBusID"] = vfDeviceId

	return json.Marshal(rawConfig)
}
