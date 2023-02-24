// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package string

import (
	"fmt"
	"reflect"
)

func ValueToStringGenerated(v interface{}) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("*%v", pv)
}
