// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=ippools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=ippools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=workloadendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=workloadendpoints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=reservedips,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=nodes;namespaces;pods,verbs=get;list;watch;update

package v1
