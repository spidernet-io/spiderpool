// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package rdmametrics

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/vishvananda/netlink"
	"regexp"
	"strings"
	"unicode"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/exec"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/lock"
)

var cli client.Client

var (
	rdmaMetricsPrefix = "rdma_"

	knownMetricsKeyDescription = map[string]string{
		"rx_write_requests":          "The number of received WRITE requests for the associated QPs.",
		"rx_read_requests":           "The number of received read requests",
		"rx_atomic_requests":         "The number of received atomic requests",
		"rx_dct_connect":             "The number of received DCT connect requests",
		"out_of_buffer":              "The number of out of buffer errors",
		"out_of_sequence":            "The number of out-of-order arrivals",
		"duplicate_request":          "The number of duplicate requests",
		"rnr_nak_retry_err":          "The number of received RNR NAK packets did not exceed the QP retry limit",
		"packet_seq_err":             "The number of packet sequence errors",
		"implied_nak_seq_err":        "The number of implied NAK sequence errors",
		"local_ack_timeout_err":      "The number of times QP's ack timer expired for RC, XRC, DCT QPs at the sender side",
		"resp_local_length_error":    "The number of times responder detected local length errors",
		"resp_cqe_error":             "The number of response CQE errors",
		"req_cqe_error":              "The number of times requester detected CQEs completed with errors",
		"req_remote_invalid_request": "The number of times requester detected remote invalid request errors",
		"req_remote_access_errors":   "The number of request remote access errors",
		"resp_remote_access_errors":  "The number of response remote access errors",
		"resp_cqe_flush_error":       "The number of response CQE flush errors",
		"req_cqe_flush_error":        "The number of request CQE flush errors",
		"roce_adp_retrans":           "The number of RoCE adaptive retransmissions",
		"roce_adp_retrans_to":        "The number of RoCE adaptive retransmission timeouts",
		"roce_slow_restart":          "The number of RoCE slow restart",
		"roce_slow_restart_cnps":     "The number of times RoCE slow restart generated CNP packets",
		"roce_slow_restart_trans":    "The number of times RoCE slow restart changed state to slow restart",
		"rp_cnp_ignored":             "The number of CNP packets received and ignored by the Reaction Point HCA",
		"rp_cnp_handled":             "The number of CNP packets handled by the Reaction Point HCA to throttle the transmission rate",
		"np_ecn_marked_roce_packets": "The number of RoCEv2 packets received by the notification point which were marked for experiencing the congestion (ECN bits where '11' on the ingress RoCE traffic)",
		"np_cnp_sent":                "The number of CNP packets sent by the Notification Point when it noticed congestion experienced in the RoCEv2 IP header (ECN bits)",
		"rx_icrc_encapsulated":       "The number of RoCE packets with ICRC errors",

		// Other
	}

	// skip to export these fields
	// key and reason
	skipRDMAStatsField = map[string]string{
		"port":   "The field is attribute is used to identify the nic port",
		"ifname": "The field is attribute is used to identify the nic name",
	}

	observableMap = map[string]metric.Int64ObservableCounter{}
)

func Register(ctx context.Context, meter metric.Meter, client client.Client) error {
	cli = client
	var err error
	list := make([]metric.Observable, 0)

	for key, description := range knownMetricsKeyDescription {
		key = rdmaMetricsPrefix + key
		d := metric.WithDescription(description)
		val, err := meter.Int64ObservableCounter(rdmaMetricsPrefix+key, d)
		if err != nil {
			return err
		}
		observableMap[key] = val
		list = append(list, val)
	}
	e := &exporter{meter: meter, ch: make(chan struct{}, 10)}
	registration, err := meter.RegisterCallback(e.Callback, list...)
	if err != nil {
		return err
	}
	e.registration = registration
	go e.daemon(ctx)
	return nil
}

type exporter struct {
	meter                metric.Meter
	lock                 lock.Mutex
	registration         metric.Registration
	unRegistrationMetric map[string]struct{}
	ch                   chan struct{}
}

func (e *exporter) daemon(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-e.ch:
			if !ok {
				return
			}
			e.lock.Lock()
			err := e.registration.Unregister()
			if err != nil {
				fmt.Printf("find new rdma metrics, need reregister collectorer, failed to unregister: %v\n", err)
				return
			}

			list := make([]metric.Observable, 0)
			for _, v := range observableMap {
				list = append(list, v)
			}
			for key := range e.unRegistrationMetric {
				if _, ok := observableMap[key]; ok {
					continue
				}
				val, err := e.meter.Int64ObservableCounter(key)
				if err != nil {
					fmt.Printf("find new rdma metrics, need reregister collectorer, failed to register callback: %v\n", err)
					return
				}
				observableMap[key] = val
				list = append(list, val)
			}
			newRegistration, err := e.meter.RegisterCallback(e.Callback, list...)
			if err != nil {
				fmt.Printf("find new rdma metrics, need reregister collectorer, failed to register callback: %v\n", err)
				return
			}
			e.registration = newRegistration
			e.unRegistrationMetric = make(map[string]struct{})
			e.lock.Unlock()
		}
	}
}

func (e *exporter) Callback(ctx context.Context, observer metric.Observer) error {
	unRegistrationMetric := make([]string, 0)

	list, err := listNodeNetNS()
	if err != nil {
		return fmt.Errorf("failed to list node net ns: %w", err)
	}
	// add empty string for host net ns
	list = append(list, "")

	ipMapPod, err := getIPToPodMap(ctx, cli)
	if err != nil {
		return fmt.Errorf("failed to get ip map pod: %w", err)
	}

	for _, netNsID := range list {
		podPrimaryIP, statsList, err := getRDMAStats(netNsID)
		if err != nil {
			continue
		}
		if len(statsList) == 0 {
			continue
		}

		// for pod, add pod info to attributes
		var attributeNamespace, attributeName *attribute.KeyValue
		item, ok := ipMapPod[podPrimaryIP]
		if ok {
			if item.Namespace != "" {
				t := attribute.String("pod_namespace", item.Namespace)
				attributeNamespace = &t
			}
			if item.Name != "" {
				t := attribute.String("pod_name", item.Name)
				attributeName = &t
			}
		}

		for _, stats := range statsList {
			var identifyPort int
			var identifyName string

			if val, ok := stats["port"].(int); ok {
				identifyPort = val
			}
			if val, ok := stats["ifname"].(string); ok {
				identifyName = val
			}

			port := attribute.Int("port", identifyPort)
			ifName := attribute.String("ifname", identifyName)

			for key, val := range stats {
				if _, skip := skipRDMAStatsField[key]; skip {
					continue
				}
				key = rdmaMetricsPrefix + key
				observable, ok := observableMap[key]
				if !ok {
					// 1. extension: if the value is of type int, automatically register it with
					//    OpenTelemetry as a counter type.
					// 2. if you find this metric valuable, please add it to the knownMetricsKeyDescription
					//    variable for better description.
					// 3. if rdma metrics need different types of metrics, they will be explicitly declared using
					//    knownMetricsKeyDescription.
					if _, ok := val.(float64); ok {
						unRegistrationMetric = append(unRegistrationMetric, key)
					}
				}
				// type assertion to float64
				// JSON, uses Javascript syntax and definitions. JavaScript only supports 64-bit
				// floating point numbers. Ref# javascript.info/number
				floatValue, ok := val.(float64)
				if !ok {
					continue
				}
				observe(observer, observable, int(floatValue), attributeNamespace, attributeName, &port, &ifName)
			}
		}
	}
	if len(unRegistrationMetric) > 0 {
		e.lock.Lock()
		defer e.lock.Unlock()
		if e.unRegistrationMetric == nil {
			e.unRegistrationMetric = make(map[string]struct{})
		}
		for _, key := range unRegistrationMetric {
			e.unRegistrationMetric[key] = struct{}{}
		}
		e.ch <- struct {
		}{}
	}
	return nil
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
	mode, err := netlink.RdmaSystemGetNetnsMode()
	if err != nil {
		return nil, err
	}
	if mode != "exclusive" {
		return make([]string, 0), err
	}
	e := exec.New()
	cmdPath, err := e.LookPath("ip")
	if err != nil {
		return nil, fmt.Errorf("error finding 'ip' command: %w", err)
	}
	cmd := e.Command(cmdPath, "netns", "list")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error executing 'ip' command: %w", err)
	}
	list := getNetNSName(string(output))
	return list, nil
}

func getNetNSName(output string) []string {
	list := make([]string, 0)
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 1 {
			list = append(list, parts[0])
		}
	}
	return list
}

func getIPToPodMap(ctx context.Context, cli client.Client) (map[string]types.NamespacedName, error) {
	list := new(corev1.PodList)
	err := cli.List(ctx, list)
	if err != nil {
		return nil, err
	}

	res := map[string]types.NamespacedName{}
	for _, item := range list.Items {
		if item.DeletionTimestamp != nil {
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

func getHostRDMAStats() ([]map[string]interface{}, error) {
	e := exec.New()
	cmdPath, err := e.LookPath("rdma")
	if err != nil {
		return nil, fmt.Errorf("error finding 'rdma' command: %w", err)
	}
	cmd := e.Command(cmdPath, "statistic", "-j")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error executing 'rdma' command: %w", err)
	}
	rdmaStats := make([]map[string]interface{}, 0)
	err = json.Unmarshal(output, &rdmaStats)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal rdma stats: %w", err)
	}
	for i, item := range rdmaStats {
		newMap := make(map[string]interface{})
		for k, v := range item {
			snake := camelToSnake(k)
			newMap[snake] = v
		}
		rdmaStats[i] = newMap
	}
	return nil, nil
}

func getRDMAStats(nsID string) (string, []map[string]interface{}, error) {
	if nsID == "" {
		stats, err := getHostRDMAStats()
		return "", stats, err
	}

	rdmaStats := make([]map[string]interface{}, 0)

	e := exec.New()
	cmdPath, err := e.LookPath("ip")
	if err != nil {
		return "", nil, fmt.Errorf("error finding 'ip' command: %w", err)
	}
	cmd := e.Command(cmdPath, "netns", "exec", nsID, "rdma", "statistic", "-j")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", nil, fmt.Errorf("error executing 'ip exec [NSID] rdma statistic -j' command: %w", err)
	}
	err = json.Unmarshal(output, &rdmaStats)
	if err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal rdma stats: %w", err)
	}
	if len(rdmaStats) == 0 {
		return "", nil, nil
	}

	for i, item := range rdmaStats {
		newMap := make(map[string]interface{})
		for k, v := range item {
			newMap[camelToSnake(k)] = v
		}
		rdmaStats[i] = newMap
	}

	// find the default IP address
	// command: ip netns exec [NS_ID] ip route get 1.0.0.0
	cmd = e.Command(cmdPath, "netns", "exec", nsID, "ip", "r", "g", "1.0.0.0")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return "", nil, fmt.Errorf("error executing 'ip netns exec [NSID] ip r g 1.0.0.0' command: %w", err)
	}
	re := regexp.MustCompile(`\bsrc\s+(\S+)`)
	match := re.FindStringSubmatch(string(output))
	if len(match) == 0 {
		// try parse ipv6
		// TODO (lou-lan) only ipv6
		return "", nil, fmt.Errorf("failed to find src ip")
	}
	srcIP := match[1]
	return srcIP, rdmaStats, err
}

func camelToSnake(camel string) string {
	var result []rune

	for i, r := range camel {
		if unicode.IsUpper(r) {
			// If it's not the first character, add an underscore
			if i > 0 {
				result = append(result, '_')
			}
			// Convert to lowercase
			r = unicode.ToLower(r)
		}
		result = append(result, r)
	}

	return string(result)
}
