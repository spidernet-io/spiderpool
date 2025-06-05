// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package nri

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/containerd/nri/pkg/api"
	"github.com/containernetworking/cni/libcni"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	cni100 "github.com/containernetworking/cni/pkg/types/100"
)

type CNIConfig struct {
	DeviceID string `json:"deviceID"`
	PicBusID string `json:"picBusID"`
	ConfList cnitypes.NetConfList
	RawBytes []byte
}

type NetworkStatus struct {
	Name       string      `json:"name"`
	Device     string      `json:"device,omitempty"`
	Interface  string      `json:"interface"`
	IPs        []string    `json:"ips"`
	Mac        string      `json:"mac"`
	Gateway    []string    `json:"gateway"`
	DeviceInfo *DeviceInfo `json:"deviceInfo"`
}

type DeviceInfo struct {
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
		if ips == nil {
			continue
		}
		ns.Interface = i.Name
		ns.IPs = append(ns.IPs, ips.Address.String())
		ns.Mac = i.Mac
		if ips.Gateway != nil {
			ns.Gateway = append(ns.Gateway, ips.Gateway.String())
		}
	}
}

func cniAdd(ctx context.Context, confList *libcni.NetworkConfigList, rc libcni.RuntimeConf) (cnitypes.Result, error) {
	cniConfig := libcni.NewCNIConfigWithCacheDir([]string{defaultCniBinPath}, defaultCniResultCacheDir, nil)

	result, err := cniConfig.AddNetworkList(ctx, confList, &rc)
	if err != nil {
		return nil, fmt.Errorf("error adding network list: %v", err)
	}

	return result, nil
}

func cniDel(ctx context.Context, confList *libcni.NetworkConfigList, rc libcni.RuntimeConf) error {
	cniConfig := libcni.NewCNIConfigWithCacheDir([]string{defaultCniBinPath}, defaultCniResultCacheDir, nil)

	if err := cniConfig.DelNetworkList(ctx, confList, &rc); err != nil {
		return fmt.Errorf("error deleting network list: %v", err)
	}

	return nil
}

func buildSecondaryCniConfig(netName, config string, vfDeviceId string) (*libcni.NetworkConfigList, error) {
	if vfDeviceId == "" || netName == "" || config == "" {
		return nil, fmt.Errorf("vfDeviceId or netName or config is empty")
	}

	data, err := appendDeviceIDInCNIConfig(netName, vfDeviceId, config)
	if err != nil {
		return nil, fmt.Errorf("error appending device ID to raw CNI config bytes: %v", err)
	}

	confList, err := libcni.ConfListFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("error converting the raw bytes into a config: %v", err)
	}

	return confList, nil
}

func appendDeviceIDInCNIConfig(netName, vfDeviceId, cniConfig string) ([]byte, error) {
	if netName == "" || vfDeviceId == "" || cniConfig == "" {
		return nil, fmt.Errorf("netName or vfDeviceId or cniConfig is empty")
	}

	var rawConfig map[string]interface{}
	var err error

	err = json.Unmarshal([]byte(cniConfig), &rawConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw CNI config: %v", err)
	}

	// Inject device ID
	pList, ok := rawConfig["plugins"]
	if !ok {
		return nil, fmt.Errorf("failed to get plugin list")
	}

	pMap, ok := pList.([]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to typecast plugin list")
	}

	for idx, plugin := range pMap {
		currentPlugin, ok := plugin.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("failed to typecast plugin #%d", idx)
		}
		// Inject deviceID
		currentPlugin["deviceID"] = vfDeviceId
		currentPlugin["pciBusID"] = vfDeviceId
	}

	// Inject network name if missing from Config for the thick plugin case
	if n, ok := rawConfig["name"]; !ok || n == "" {
		rawConfig["name"] = netName
	}

	data, err := json.Marshal(rawConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal raw CNI config: %v", err)
	}

	return data, nil
}

func buildRuntimeConfig(pod *api.PodSandbox, deviceInfo DeviceInfo, nicName string, podNetNs string) libcni.RuntimeConf {
	capabilityArgs := map[string]any{}
	capabilityArgs["deviceID"] = deviceInfo.PciAddress

	return libcni.RuntimeConf{
		ContainerID: pod.Uid,
		NetNS:       podNetNs,
		IfName:      nicName,
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
