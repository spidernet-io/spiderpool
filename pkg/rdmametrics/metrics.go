// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package rdmametrics

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
)

const netnsPath = "/var/run/netns"

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
	}

	// skip to export these fields
	// key and reason
	skipRDMAStatsField = map[string]string{
		"port":             "The field is attribute is used to identify the nic port",
		"ifname":           "The field is attribute is used to identify the nic netnsPath",
		"net_dev_name":     "The field is attribute is used to identify the nic net_dev_name",
		"node_guid":        "The field is attribute is used to identify the nic node_guid",
		"sys_image_guid":   "The field is attribute is used to identify the nic sys_image_guid",
		"rdma_parent_name": "The field is attribute is used to identify the nic rdma_parent_name",
		"is_root":          "The field is attribute is used to identify the nic is_root",
	}
)

type GetObservable func(string) (metric.Int64ObservableCounter, bool)

type EthtoolImpl struct {
	Stats func(netIfName string) (map[string]uint64, error)
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

func Register(ctx context.Context, meter metric.Meter, cache podownercache.CacheInterface) error {
	log := logutils.Logger.Named("rdma-metrics-exporter")
	nodeName, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}
	attributeNodeName := attribute.String("node_name", nodeName)
	e := &exporter{
		observableMap: make(map[string]metric.Int64ObservableCounter),
		nodeName:      &attributeNodeName,
		meter:         meter,
		netns: func(netnsID string, toRun func() error) error {
			if netnsID != "" {
				netns, err := ns.GetNS(filepath.Join(netnsPath, netnsID))
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
	nodeName              *attribute.KeyValue
	meter                 metric.Meter
	lock                  lock.Mutex
	log                   *zap.Logger
	ch                    chan struct{}
	netns                 func(netnsID string, toRun func() error) error
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
	list = append(list, "")

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
	for _, netNsID := range list {
		if err := e.processNetNS(netNsID, nodeGuidNetDeviceNameMap, observer, getObservable); err != nil {
			e.log.Error("failed to process net ns", zap.String("net_ns_id", netNsID), zap.Error(err))
			continue
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

func (e *exporter) processNetNS(netNsID string,
	nodeGuidNetDeviceNameMap map[string]string,
	observer metric.Observer, getObservable GetObservable) error {
	podPrimaryIP, statsList, err := getRDMAStats(netNsID, e.netns, nodeGuidNetDeviceNameMap, e.netlinkImpl, e.exec, e.ethtool)
	if err != nil {
		e.log.Error("failed to get RDMA stats", zap.String("net_ns_id", netNsID), zap.Error(err))
		return err
	}
	if len(statsList) == 0 {
		return nil
	}

	attributes := []*attribute.KeyValue{
		e.nodeName,
	}

	if pod := e.cache.GetPodByIP(podPrimaryIP); pod != nil {
		namespace := attribute.String("pod_namespace", pod.Namespace)
		name := attribute.String("pod_name", pod.Name)
		ownerAPIVersion := attribute.String("owner_api_version", pod.OwnerInfo.APIVersion)
		ownerKind := attribute.String("owner_kind", pod.OwnerInfo.Kind)
		ownerNamespace := attribute.String("owner_namespace", pod.OwnerInfo.Namespace)
		ownerName := attribute.String("owner_name", pod.OwnerInfo.Name)
		attributes = append(attributes, &namespace, &name, &ownerAPIVersion, &ownerKind, &ownerNamespace, &ownerName)
	}
	for _, stats := range statsList {
		processStats(stats, observer, getObservable, attributes...)
	}
	return nil
}

func getPodAttributes(item types.NamespacedName) (*attribute.KeyValue, *attribute.KeyValue) {
	var attributeNamespace, attributeName *attribute.KeyValue
	if item.Namespace != "" {
		t := attribute.String("pod_namespace", item.Namespace)
		attributeNamespace = &t
	}
	if item.Name != "" {
		t := attribute.String("pod_name", item.Name)
		attributeName = &t
	}
	return attributeNamespace, attributeName
}

func processStats(stats map[string]interface{}, observer metric.Observer,
	getObservable GetObservable, attributes ...*attribute.KeyValue) {
	nicExtAttributes := getIdentifyAttributes(stats)
	attributes = append(attributes, nicExtAttributes...)

	for key, val := range stats {
		if _, skip := skipRDMAStatsField[key]; skip {
			continue
		}
		if observable, ok := getObservable(key); ok {
			if value, ok := val.(float64); ok {
				observe(observer, observable, int(value), attributes...)
				continue
			}
			if value, ok := val.(uint64); ok {
				observe(observer, observable, int(value), attributes...)
			}
		}
	}
}

func getIdentifyAttributes(stats map[string]interface{}) []*attribute.KeyValue {
	res := make([]*attribute.KeyValue, 0)
	if val, ok := stats["port"].(float64); ok {
		res = append(res, &attribute.KeyValue{Key: "port", Value: attribute.IntValue(int(val))})
	}
	if val, ok := stats["ifname"].(string); ok {
		res = append(res, &attribute.KeyValue{Key: "ifname", Value: attribute.StringValue(val)})
	}
	if val, ok := stats["net_dev_name"].(string); ok {
		res = append(res, &attribute.KeyValue{Key: "net_dev_name", Value: attribute.StringValue(val)})
	}
	if val, ok := stats["node_guid"].(string); ok {
		res = append(res, &attribute.KeyValue{Key: "node_guid", Value: attribute.StringValue(val)})
	}
	if val, ok := stats["sys_image_guid"].(string); ok {
		res = append(res, &attribute.KeyValue{Key: "sys_image_guid", Value: attribute.StringValue(val)})
	}
	if val, ok := stats["rdma_parent_name"].(string); ok {
		res = append(res, &attribute.KeyValue{Key: "rdma_parent_name", Value: attribute.StringValue(val)})
	}
	if val, ok := stats["is_root"].(bool); ok {
		res = append(res, &attribute.KeyValue{Key: "is_root", Value: attribute.BoolValue(val)})
	}
	return res
}

func observe(observer metric.Observer, counter metric.Int64ObservableCounter, value int, attributes ...*attribute.KeyValue) {
	list := make([]attribute.KeyValue, 0, len(attributes))
	for _, item := range attributes {
		if item != nil {
			list = append(list, *item)
		}
	}
	observer.ObserveInt64(counter, int64(value), metric.WithAttributes(list...))
}

func listNodeNetNS() ([]string, error) {
	mode, err := rdmaSystemGetNetnsMode()
	if err != nil {
		return nil, err
	}
	if mode != "exclusive" {
		return []string{}, nil
	}

	dirEntries, err := readDir(netnsPath)
	if err != nil {
		return nil, err
	}

	netnsList := make([]string, 0, len(dirEntries))
	for _, entry := range dirEntries {
		netnsList = append(netnsList, entry.Name())
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

func getRDMAStats(nsID string,
	netnsDo func(nsID string, toRun func() error) error,
	nodeGuidNetNameMap map[string]string,
	nl NetlinkImpl,
	e exec.Interface, ethtool EthtoolImpl) (string, []map[string]interface{}, error) {

	var srcIP string
	var rdmaStats []map[string]interface{}

	err := netnsDo(nsID, func() error {
		cmd := e.Command("rdma", "statistic", "-j")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("error executing 'rdma statistic -j' command: %w", err)
		}

		if err := json.Unmarshal(output, &rdmaStats); err != nil {
			return fmt.Errorf("failed to unmarshal rdma stats: %w", err)
		}

		netDevMap, err := getIfnameNetDevMap(nl)
		if err != nil {
			return fmt.Errorf("failed to get ifname net dev map: %w", err)
		}

		for i, item := range rdmaStats {
			newMap := make(map[string]interface{})
			for k, v := range item {
				newMap[camelToSnake(k)] = v
			}

			// append more metrics
			var ifname string
			if val, ok := newMap["ifname"].(string); ok {
				ifname = val
			}
			if ifname != "" {
				if devName, ok := netDevMap[ifname]; ok {
					newMap["net_dev_name"] = devName.NetDevName
					newMap["node_guid"] = devName.NodeGuid
					newMap["sys_image_guid"] = devName.SysImageGuid
					newMap["is_root"] = devName.IsRoot
					stats, err := ethtool.Stats(devName.NetDevName)
					if err == nil {
						for k, v := range stats {
							if strings.Contains(k, "vport") {
								newMap[k] = v
							}
						}
					}
					if rdmaParentName, ok := nodeGuidNetNameMap[devName.SysImageGuid]; ok {
						newMap["rdma_parent_name"] = rdmaParentName
					}
				}
			}
			rdmaStats[i] = newMap
		}

		// host netns, don't need get default ip to mapping pod metadata to metrics
		if nsID == "" {
			return nil
		}
		srcIP, err = getDefaultIP(e)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", nil, err
	}

	return srcIP, rdmaStats, nil
}

// map of node guid to rdma name
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
	re := regexp.MustCompile(`\bsrc\s+(\S+)`)

	// Check for IPv4 default route
	cmd := e.Command("ip", "route", "get", "1.0.0.0")
	output, err := cmd.CombinedOutput()
	if err == nil {
		match := re.FindStringSubmatch(string(output))
		if len(match) > 0 {
			return match[1], nil
		}
	}

	// Check for IPv6 default route
	cmd = e.Command("ip", "-6", "route", "get", "2001:4860:4860::8888")
	output, err = cmd.CombinedOutput()
	if err == nil {
		match := re.FindStringSubmatch(string(output))
		if len(match) > 0 {
			return match[1], nil
		}
	}

	return "", fmt.Errorf("failed to find default IP address: %w", err)
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
