// Copyright (c) 2023-2024 Tigera, Inc. All rights reserved.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	KindExternalNetwork     = "ExternalNetwork"
	KindExternalNetworkList = "ExternalNetworkList"
)

// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExternalNetworkList is a list of ExternalNetwork resources.
type ExternalNetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []ExternalNetwork `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ExternalNetwork struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec ExternalNetworkSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// ExternalNetworkSpec contains the specification for a external network resource.
type ExternalNetworkSpec struct {
	// The index of a linux kernel routing table that should be used for the routes associated with the external network.
	// The value should be unique for each external network.
	// The value should not be in the range of `RouteTableRanges` field in FelixConfiguration.
	// The kernel routing table index should not be used by other processes on the node.
	RouteTableIndex *uint32 `json:"routeTableIndex" validate:"required"`
}
