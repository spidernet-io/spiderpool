// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"fmt"
	"reflect"
	"strings"
)

func valueToStringGenerated(v interface{}) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("*%v", pv)
}

func (in *PodSubnetAnnoConfig) String() string {
	if in == nil {
		return "nil"
	}

	s := strings.Join([]string{`&PodSubnetAnnoConfig{`,
		`MultipleSubnets` + fmt.Sprintf("%v", in.MultipleSubnets),
		`SingleSubnet:` + strings.Replace(strings.Replace(in.SingleSubnet.String(), "AnnoSubnetItem", "", 1), `&`, ``, 1) + `,`,
		`FlexibleIPNum:` + valueToStringGenerated(in.FlexibleIPNum) + `,`,
		`AssignIPNumber:` + fmt.Sprintf("%v", in.AssignIPNum) + `,`,
		`ReclaimIPPool:` + fmt.Sprintf("%v", in.ReclaimIPPool),
		`}`,
	}, "")
	return s
}

func (in *AnnoSubnetItem) String() string {
	if in == nil {
		return "nil"
	}

	s := strings.Join([]string{`&AnnoSubnetItem{`,
		`Interface:` + fmt.Sprintf("%v", in.Interface) + `,`,
		`IPv4:` + fmt.Sprintf("%v", in.IPv4) + `,`,
		`IPv6:` + fmt.Sprintf("%v", in.IPv6),
		`}`,
	}, "")
	return s
}
