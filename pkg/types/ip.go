// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package types

type IPVersion = int64

type Vlan = int64

type IPAndCID struct {
	IP          string
	ContainerID string
	Node        string
}
