// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package string

import (
	"fmt"
	"reflect"
	"strings"

	v1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
)

func ValueToStringGenerated(v interface{}) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("*%v", pv)
}

func ParseNsAndName(s string) (ns, name string) {
	s = strings.TrimSpace(s)
	r := strings.Split(s, "/")
	if len(r) == 1 {
		return v1.NamespaceDefault, r[0]
	}
	return r[0], r[1]
}
