// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ethtool

import (
	"strconv"
	"strings"

	"github.com/safchain/ethtool"
	"go.opentelemetry.io/otel/attribute"

	"github.com/spidernet-io/spiderpool/pkg/rdmametrics/oteltype"
)

func Stats(netIfName string) ([]oteltype.Metrics, error) {
	tool, err := ethtool.NewEthtool()
	if err != nil {
		return nil, err
	}
	defer tool.Close()
	stats, err := tool.Stats(netIfName)
	if err != nil {
		return nil, err
	}

	res := make([]oteltype.Metrics, 0)
	for k, v := range stats {
		if strings.Contains(k, "vport") {
			res = append(res, oteltype.Metrics{Name: k, Value: int64(v)})
			continue
		}
		if expectPriorityMetrics(k) {
			name, priority, ok := extractNameWithPriority(k)
			if ok {
				res = append(res, oteltype.Metrics{
					Name:  name,
					Value: int64(v),
					Labels: []attribute.KeyValue{{
						Key: "priority", Value: attribute.IntValue(priority),
					}},
				})
			}
		}
	}

	speed, err := tool.CmdGetMapped(netIfName)
	if err != nil {
		return nil, err
	}

	// speed unknown = 4294967295
	if val, ok := speed["speed"]; ok && val != 4294967295 {
		res = append(res, oteltype.Metrics{
			Name:  "vport_speed_mbps",
			Value: int64(val),
		})
	}
	return res, nil
}

func expectPriorityMetrics(s string) bool {
	if len(s) == 14 {
		if (strings.HasPrefix(s, "rx_prio") || strings.HasPrefix(s, "tx_prio")) && strings.HasSuffix(s, "_pause") {
			return true
		}
	}
	if len(s) == 17 {
		if (strings.HasPrefix(s, "rx_prio") || strings.HasPrefix(s, "tx_prio")) && strings.HasSuffix(s, "_discards") {
			return true
		}
	}
	return false
}

func extractNameWithPriority(str string) (name string, priority int, ok bool) {
	parts := strings.Split(str, "_")
	if len(parts) != 3 {
		return "", 0, false
	}

	name = parts[0] + "_" + parts[2]

	rawPriority := parts[1]
	if len(rawPriority) < 5 {
		return "", 0, false
	}

	var err error

	if strings.HasPrefix(rawPriority, "prio") {
		priority, err = strconv.Atoi(strings.TrimPrefix(rawPriority, "prio"))
		if err != nil {
			return "", 0, false
		}
	} else {
		return "", 0, false
	}

	return name, priority, true
}
