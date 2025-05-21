package nri

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/containerd/nri/pkg/api"
	"github.com/containernetworking/cni/libcni"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	cni100 "github.com/containernetworking/cni/pkg/types/100"
	"github.com/vishvananda/netns"
)

type CNIConfig struct {
	DeviceID string `json:"deviceID"`
	PicBusID string `json:"picBusID"`
	ConfList cnitypes.NetConfList
	RawBytes []byte
}

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

func cniAdd(ctx context.Context, rawnetconflist []byte, rc libcni.RuntimeConf) (cnitypes.Result, error) {
	cniConfig := libcni.NewCNIConfigWithCacheDir([]string{defaultCniBinPath}, defaultCniResultCacheDir, nil)

	confList, err := libcni.ConfListFromBytes(rawnetconflist)
	if err != nil {
		return nil, fmt.Errorf("error converting the raw bytes into a conflist: %v", err)
	}

	result, err := cniConfig.AddNetworkList(ctx, confList, &rc)
	if err != nil {
		return nil, fmt.Errorf("error adding network list: %v", err)
	}

	return result, nil
}

func buildSecondaryCniConfig(netName, config string) (*libcni.NetworkConfigList, error) {
	if netName == "" || config == "" {
		return nil, fmt.Errorf("netName or config is empty")
	}

	configBytes, err := getRawConfigBytes(netName, config)
	if err != nil {
		return nil, fmt.Errorf("error appending device ID to raw CNI config bytes: %v", err)
	}

	confList, err := libcni.ConfListFromBytes(configBytes)
	if err != nil {
		return nil, fmt.Errorf("error converting the raw bytes into a config: %v", err)
	}

	return confList, nil
}

func getRawConfigBytes(netName, cniConfig string) ([]byte, error) {
	if netName == "" || cniConfig == "" {
		return nil, fmt.Errorf("netName or cniConfig is empty")
	}

	var rawConfig map[string]interface{}
	var err error

	err = json.Unmarshal([]byte(cniConfig), &rawConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw CNI config: %v", err)
	}

	// Inject network name if missing from Config for the thick plugin case
	if n, ok := rawConfig["name"]; !ok || n == "" {
		rawConfig["name"] = netName
	}

	return json.Marshal(rawConfig)
}

func buildRuntimeConfig(pod *api.PodSandbox, deviceInfo DeviceInfo, idx int, podNetNs netns.NsHandle) libcni.RuntimeConf {
	capabilityArgs := map[string]interface{}{}
	capabilityArgs["deviceID"] = deviceInfo.PciAddress

	return libcni.RuntimeConf{
		ContainerID: pod.Uid,
		NetNS:       podNetNs.String(),
		IfName:      fmt.Sprintf("net%d", idx),
		Args: [][2]string{
			{"IgnoreUnknown", "true"},
			{"K8S_POD_NAMESPACE", pod.Namespace},
			{"K8S_POD_NAME", pod.Name},
			{"K8S_POD_INFRA_CONTAINER_ID", pod.Id},
			{"K8S_POD_UID", pod.Uid},
		},
		CapabilityArgs: capabilityArgs,
	}
}
