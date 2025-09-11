// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package rdmametrics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/vishvananda/netlink"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/exec"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/spidernet-io/spiderpool/pkg/lock"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/podownercache"
	"github.com/spidernet-io/spiderpool/pkg/rdmametrics/ethtool"
	"github.com/spidernet-io/spiderpool/pkg/rdmametrics/oteltype"
)

var netnsPathList = []string{"/var/run/netns", "/var/run/docker/netns"}

var (
	readDir                = os.ReadDir
	rdmaSystemGetNetnsMode = netlink.RdmaSystemGetNetnsMode

	rdmaMetricsPrefix = "rdma_"

	knownMetricsKeyDescription = map[string]string{
		"rx_write_requests":               "The number of received WRITE requests for the associated QPs.",
		"rx_read_requests":                "The number of received read requests",
		"rx_atomic_requests":              "The number of received atomic requests",
		"rx_dct_connect":                  "The number of received DCT connect requests",
		"out_of_buffer":                   "The number of out of buffer errors",
		"out_of_sequence":                 "The number of out-of-order arrivals",
		"duplicate_request":               "The number of duplicate requests",
		"rnr_nak_retry_err":               "The number of received RNR NAK packets did not exceed the QP retry limit",
		"packet_seq_err":                  "The number of packet sequence errors",
		"implied_nak_seq_err":             "The number of implied NAK sequence errors",
		"local_ack_timeout_err":           "The number of times QP's ack timer expired for RC, XRC, DCT QPs at the sender side",
		"resp_local_length_error":         "The number of times responder detected local length errors",
		"resp_cqe_error":                  "The number of response CQE errors",
		"req_cqe_error":                   "The number of times requester detected CQEs completed with errors",
		"req_remote_invalid_request":      "The number of times requester detected remote invalid request errors",
		"req_remote_access_errors":        "The number of request remote access errors",
		"resp_remote_access_errors":       "The number of response remote access errors",
		"resp_cqe_flush_error":            "The number of response CQE flush errors",
		"req_cqe_flush_error":             "The number of request CQE flush errors",
		"roce_adp_retrans":                "The number of RoCE adaptive retransmissions",
		"roce_adp_retrans_to":             "The number of RoCE adaptive retransmission timeouts",
		"roce_slow_restart":               "The number of RoCE slow restart",
		"roce_slow_restart_cnps":          "The number of times RoCE slow restart generated CNP packets",
		"roce_slow_restart_trans":         "The number of times RoCE slow restart changed state to slow restart",
		"rp_cnp_ignored":                  "The number of CNP packets received and ignored by the Reaction Point HCA",
		"rp_cnp_handled":                  "The number of CNP packets handled by the Reaction Point HCA to throttle the transmission rate",
		"np_ecn_marked_roce_packets":      "The number of RoCEv2 packets received by the notification point which were marked for experiencing the congestion (ECN bits where '11' on the ingress RoCE traffic)",
		"np_cnp_sent":                     "The number of CNP packets sent by the Notification Point when it noticed congestion experienced in the RoCEv2 IP header (ECN bits)",
		"rx_icrc_encapsulated":            "The number of RoCE packets with ICRC errors",
		"rx_vport_rdma_unicast_packets":   "The number of unicast RDMA packets received on the virtual port.",
		"tx_vport_rdma_unicast_packets":   "The number of unicast RDMA packets transmitted from the virtual port.",
		"rx_vport_rdma_multicast_packets": "The number of multicast RDMA packets received on the virtual port.",
		"tx_vport_rdma_multicast_packets": "The number of multicast RDMA packets transmitted from the virtual port.",
		"rx_vport_rdma_unicast_bytes":     "The number of bytes received in unicast RDMA packets on the virtual port.",
		"tx_vport_rdma_unicast_bytes":     "The number of bytes transmitted in unicast RDMA packets from the virtual port.",
		"rx_vport_rdma_multicast_bytes":   "The number of bytes received in multicast RDMA packets on the virtual port.",
		"tx_vport_rdma_multicast_bytes":   "The number of bytes transmitted in multicast RDMA packets from the virtual port.",
		"vport_speed_mbps":                "The speed of the virtual port expressed in megabits per second (Mbps).",
		"rx_discards":                     "The number of packets discarded by the device.",
		"tx_discards":                     "The number of packets discarded by the device.",
		"rx_pause":                        "The number of packets dropped by the device.",
		"tx_pause":                        "The number of packets dropped by the device.",
		"device_tos":                      "RDMA device traffic class (TOS) value."}
)

type GetObservable func(string) (metric.Int64ObservableCounter, bool)

type EthtoolImpl struct {
	Stats func(netIfName string) ([]oteltype.Metrics, error)
}

type NetlinkImpl struct {
	RdmaLinkList func() ([]*netlink.RdmaLink, error)
	LinkList     func() ([]netlink.Link, error)
}

type RDMADevice struct {
	NetDevName   string
	NodeGuid     string
	SysImageGuid string
	IsRoot       bool
}

type NetnsItem struct {
	ID string
	Fd string
}

func Register(ctx context.Context, meter metric.Meter, cache podownercache.CacheInterface) error {
	log := logutils.Logger.Named("rdma-metrics-exporter")
	nodeName, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}
	attributeNodeName := attribute.String("node_name", nodeName)
	e := &exporter{
		observableMap: make(map[string]metric.Int64ObservableCounter),
		nodeName:      attributeNodeName,
		meter:         meter,
		netns: func(netns NetnsItem, toRun func() error) error {
			if netns.ID != "" {
				netns, err := ns.GetNS(netns.Fd)
				if err != nil {
					return err
				}
				return netns.Do(func(netNS ns.NetNS) error {
					return toRun()
				})
			}
			return toRun()
		},
		exec:                  exec.New(),
		log:                   log,
		ch:                    make(chan struct{}, 10),
		waitToRegisterMetrics: make(map[string]struct{}),
		ethtool:               EthtoolImpl{Stats: ethtool.Stats},
		netlinkImpl: NetlinkImpl{
			RdmaLinkList: netlink.RdmaLinkList,
			LinkList:     netlink.LinkList,
		},
		cache: cache,
	}
	err = e.registerMetrics(meter)
	if err != nil {
		return err
	}
	log.Info("rdma metrics registered")
	go e.daemon(ctx)
	return nil
}

type exporter struct {
	nodeName              attribute.KeyValue
	meter                 metric.Meter
	lock                  lock.Mutex
	log                   *zap.Logger
	ch                    chan struct{}
	netns                 func(netns NetnsItem, toRun func() error) error
	netlinkImpl           NetlinkImpl
	ethtool               EthtoolImpl
	exec                  exec.Interface
	registration          metric.Registration
	waitToRegisterMetrics map[string]struct{}
	observableMap         map[string]metric.Int64ObservableCounter
	cache                 podownercache.CacheInterface
}

func (e *exporter) registerMetrics(meter metric.Meter) error {
	list := make([]metric.Observable, 0)
	// register known metrics
	for key, description := range knownMetricsKeyDescription {
		keyWithPrefix := rdmaMetricsPrefix + key
		d := metric.WithDescription(description)
		val, err := meter.Int64ObservableCounter(keyWithPrefix, d)
		if err != nil {
			return err
		}
		e.observableMap[keyWithPrefix] = val
		list = append(list, val)
	}
	// register discovered metrics
	for key := range e.waitToRegisterMetrics {
		keyWithPrefix := rdmaMetricsPrefix + key
		val, err := e.meter.Int64ObservableCounter(keyWithPrefix)
		if err != nil {
			return err
		}
		e.observableMap[keyWithPrefix] = val
		list = append(list, val)
	}
	e.waitToRegisterMetrics = make(map[string]struct{})

	registration, err := meter.RegisterCallback(e.Callback, list...)
	if err != nil {
		return err
	}
	e.registration = registration
	return nil
}

func (e *exporter) reRegisterMetrics() error {
	e.lock.Lock()
	defer e.lock.Unlock()
	err := e.registration.Unregister()
	if err != nil {
		return fmt.Errorf("failed to unregister metric: %w", err)
	}
	err = e.registerMetrics(e.meter)
	if err != nil {
		return err
	}
	return nil
}

func (e *exporter) daemon(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			close(e.ch)
			return
		case _, ok := <-e.ch:
			if !ok {
				return
			}
			err := e.reRegisterMetrics()
			if err != nil {
				e.log.Error("failed to re-register metrics", zap.Error(err))
			}
		}
	}
}

func (e *exporter) Callback(ctx context.Context, observer metric.Observer) error {
	list, err := listNodeNetNS()
	if err != nil {
		e.log.Error("failed to list node net ns", zap.Error(err))
		return fmt.Errorf("failed to list node net ns: %w", err)
	}
	list = append(list, NetnsItem{})

	unRegistrationMetric := make([]string, 0)
	getObservable := func(key string) (metric.Int64ObservableCounter, bool) {
		if val, ok := e.observableMap[rdmaMetricsPrefix+key]; ok {
			return val, ok
		}
		unRegistrationMetric = append(unRegistrationMetric, key)
		return nil, false
	}
	nodeGuidNetDeviceNameMap, err := getNodeGuidNetDeviceNameMap(e.netlinkImpl)
	if err != nil {
		e.log.Error("failed to get node guid net device name map", zap.Error(err))
		return fmt.Errorf("failed to get node guid net device name map: %w", err)
	}
	for _, netns := range list {
		if err := e.processNetNS(netns, nodeGuidNetDeviceNameMap, observer, getObservable); err != nil {
			e.log.Error("failed to process net ns", zap.String("net_ns_id", netns.ID), zap.Error(err))
			continue
		}
	}

	deviceTrafficClassList, err := GetDeviceTrafficClass(e.netlinkImpl)
	if err != nil {
		e.log.Error("failed to get device traffic class", zap.Error(err))
	} else {
		for _, item := range deviceTrafficClassList {
			if observable, ok := getObservable("device_tos"); ok {
				observer.ObserveInt64(observable, int64(item.TrafficClass), metric.WithAttributes(
					e.nodeName,
					attribute.String("ifname", item.IfName),
					attribute.String("net_dev_name", item.NetDevName),
				))
			}
		}
	}

	if len(unRegistrationMetric) > 0 {
		e.updateUnregisteredMetrics(unRegistrationMetric)
	}
	return nil
}

func (e *exporter) updateUnregisteredMetrics(unRegistrationMetric []string) {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.waitToRegisterMetrics == nil {
		e.waitToRegisterMetrics = make(map[string]struct{})
	}
	for _, key := range unRegistrationMetric {
		e.waitToRegisterMetrics[key] = struct{}{}
	}
	select {
	case e.ch <- struct{}{}:
	default:
		e.log.Warn("channel is closed or full, cannot send data")
	}
}

func (e *exporter) processNetNS(netns NetnsItem,
	nodeGuidNetDeviceNameMap map[string]string,
	observer metric.Observer, getObservable GetObservable) error {

	list := make([]oteltype.Metrics, 0)

	err := e.netns(netns, func() error {
		var rdmaStats []map[string]interface{}
		cmd := e.exec.Command("rdma", "statistic", "-j")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("error executing 'rdma statistic -j' command: %w", err)
		}

		if err := json.Unmarshal(output, &rdmaStats); err != nil {
			return fmt.Errorf("failed to unmarshal rdma stats: %w", err)
		}

		netDevMap, err := getIfnameNetDevMap(e.netlinkImpl)
		if err != nil {
			return fmt.Errorf("failed to get ifname net dev map: %w", err)
		}

		for _, item := range rdmaStats {
			ifName, ok := item["ifname"].(string)
			if !ok {
				continue
			}
			deviceInfo, ok := netDevMap[ifName]
			if !ok {
				continue
			}
			commonLabels := []attribute.KeyValue{
				e.nodeName,
				attribute.String("ifname", ifName),
				attribute.String("net_dev_name", deviceInfo.NetDevName),
				attribute.String("node_guid", deviceInfo.NodeGuid),
				attribute.String("sys_image_guid", deviceInfo.SysImageGuid),
				attribute.Bool("is_root", deviceInfo.IsRoot),
			}
			if rdmaParentName, ok := nodeGuidNetDeviceNameMap[deviceInfo.SysImageGuid]; ok {
				commonLabels = append(commonLabels, attribute.String("rdma_parent_name", rdmaParentName))
			}

			// host netns, don't need get default ip to mapping pod metadata to metrics
			if netns.ID != "" {
				srcIP, err := getDefaultIP(e.exec)
				if err != nil {
					return err
				}
				if pod := e.cache.GetPodByIP(srcIP); pod != nil {
					commonLabels = append(commonLabels, attribute.String("pod_namespace", pod.Namespace))
					commonLabels = append(commonLabels, attribute.String("pod_name", pod.Name))
					commonLabels = append(commonLabels, attribute.String("owner_api_version", pod.OwnerInfo.APIVersion))
					commonLabels = append(commonLabels, attribute.String("owner_kind", pod.OwnerInfo.Kind))
					commonLabels = append(commonLabels, attribute.String("owner_namespace", pod.OwnerInfo.Namespace))
					commonLabels = append(commonLabels, attribute.String("owner_name", pod.OwnerInfo.Name))
				}
			}

			for key, val := range item {
				if key == "ifname" || key == "port" {
					continue
				}
				if tmp, ok := val.(float64); ok {
					list = append(list, oteltype.Metrics{
						Name:   camelToSnake(key),
						Value:  int64(tmp),
						Labels: commonLabels,
					})
				}
			}
			stats, err := e.ethtool.Stats(deviceInfo.NetDevName)
			if err == nil {
				for _, stat := range stats {
					stat.Labels = append(stat.Labels, commonLabels...)
					list = append(list, stat)
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	for _, v := range list {
		if observable, ok := getObservable(v.Name); ok {
			observer.ObserveInt64(observable, v.Value, metric.WithAttributes(v.Labels...))
		}
	}
	return nil
}

func listNodeNetNS() ([]NetnsItem, error) {
	mode, err := rdmaSystemGetNetnsMode()
	if err != nil {
		return nil, err
	}
	if mode != "exclusive" {
		return []NetnsItem{}, nil
	}

	netnsList := make([]NetnsItem, 0)

	for _, path := range netnsPathList {
		dirEntries, err := readDir(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}

		for _, entry := range dirEntries {
			// skip default netns, default netns is a host netns
			if entry.Name() == "default" {
				continue
			}
			fdFullPath := filepath.Join(path, entry.Name())
			netnsList = append(netnsList, NetnsItem{
				ID: entry.Name(),
				Fd: fdFullPath,
			})
		}
	}

	return netnsList, nil
}

func getIPToPodMap(ctx context.Context, cli client.Client) (map[string]types.NamespacedName, error) {
	list := new(corev1.PodList)
	err := cli.List(ctx, list)
	if err != nil {
		return nil, err
	}

	res := map[string]types.NamespacedName{}
	for _, item := range list.Items {
		if item.Spec.HostNetwork {
			continue
		}
		if len(item.Status.PodIPs) == 0 {
			continue
		}
		for _, ip := range item.Status.PodIPs {
			if ip.IP == "" {
				continue
			}
			res[ip.IP] = types.NamespacedName{Namespace: item.Namespace, Name: item.Name}
		}
	}
	return res, err
}

// getNodeGuidNetDeviceNameMap get map of node guid to rdma name
func getNodeGuidNetDeviceNameMap(nl NetlinkImpl) (map[string]string, error) {
	list, err := getIfnameNetDevMap(nl)
	if err != nil {
		return nil, err
	}
	res := make(map[string]string)
	for _, item := range list {
		if item.IsRoot {
			res[item.NodeGuid] = item.NetDevName
		}
	}
	return res, nil
}

func getIfnameNetDevMap(nl NetlinkImpl) (map[string]RDMADevice, error) {
	netList, err := nl.LinkList()
	if err != nil {
		return nil, err
	}

	netDevHardwareAddrMap := make(map[string]string)
	for _, item := range netList {
		if item.Attrs().Name == "" {
			continue
		}
		addr := item.Attrs().HardwareAddr.String()
		if addr == "" {
			continue
		}
		if item.Type() == "ipoib" {
			if len(addr) == 59 {
				// for example:
				// ib device hardware addr 00:00:01:af:fe:80:00:00:00:00:00:00:03:a7:83:7a:20:bf:ed:2f
				// node guid = addr[36:] =                                     03:a7:83:7a:20:bf:ed:2f
				addr = addr[36:]
				netDevHardwareAddrMap[addr] = item.Attrs().Name
			}
			continue
		}
		netDevHardwareAddrMap[addr] = item.Attrs().Name
	}

	res := make(map[string]RDMADevice)
	rdmaList, err := nl.RdmaLinkList()
	if err != nil {
		return nil, err
	}

	for _, v := range rdmaList {
		guid := reverseMACAddress(v.Attrs.NodeGuid)
		if devName, ok := netDevHardwareAddrMap[guid]; ok {
			res[v.Attrs.Name] = RDMADevice{
				NetDevName:   devName,
				NodeGuid:     v.Attrs.NodeGuid,
				SysImageGuid: v.Attrs.SysImageGuid,
				IsRoot:       v.Attrs.NodeGuid == v.Attrs.SysImageGuid,
			}
			continue
		}
		if len(guid) == 23 {
			// 3a:b0:33:ff:fe:1a:0d:70
			// 3a:b0:33:      1a:0d:70
			roceHardwareAddr := guid[:9] + guid[15:]
			if devName, ok := netDevHardwareAddrMap[roceHardwareAddr]; ok {
				res[v.Attrs.Name] = RDMADevice{
					NetDevName:   devName,
					NodeGuid:     v.Attrs.NodeGuid,
					SysImageGuid: v.Attrs.SysImageGuid,
					IsRoot:       v.Attrs.NodeGuid == v.Attrs.SysImageGuid,
				}
			}
		}
	}
	return res, nil
}

// getDefaultIP returns the default IP address of the host/pod
func getDefaultIP(e exec.Interface) (string, error) {
	// Check for IPv4 default route
	cmd := e.Command("ip", "route", "get", "1.0.0.0")
	output, err := cmd.CombinedOutput()
	if err == nil {
		if res, ok := extractSrcIPStringIndex(string(output)); ok {
			return res, nil
		}
	}

	// Check for IPv6 default route
	cmd = e.Command("ip", "-6", "route", "get", "2001:4860:4860::8888")
	output, err = cmd.CombinedOutput()
	if err == nil {
		if res, ok := extractSrcIPStringIndex(string(output)); ok {
			return res, nil
		}
	}

	return "", fmt.Errorf("failed to find default IP address: %w", err)
}

func extractSrcIPStringIndex(raw string) (string, bool) {
	const marker = " src "
	startIndex := strings.Index(raw, marker)

	if startIndex == -1 {
		return "", false
	}

	ipStartIndex := startIndex + len(marker)
	if ipStartIndex >= len(raw) {
		return "", false
	}

	endIndex := strings.Index(raw[ipStartIndex:], " ")

	if endIndex == -1 {
		return raw[ipStartIndex:], true
	}
	return raw[ipStartIndex : ipStartIndex+endIndex], true
}

// camelToSnake converts a camelCase string to snake_case
func camelToSnake(camel string) string {
	var result []rune
	for i, r := range camel {
		if unicode.IsUpper(r) {
			if i > 0 && (i+1 < len(camel) && unicode.IsLower(rune(camel[i+1]))) {
				result = append(result, '_')
			}
			r = unicode.ToLower(r)
		}
		result = append(result, r)
	}
	return string(result)
}

func reverseMACAddress(mac string) string {
	parts := strings.Split(mac, ":")
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, ":")
}
