// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=spidersubnets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=spidersubnets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=spiderippools,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=spiderippools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=spiderendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=spiderreservedips,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=spidercoordinators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=spidercoordinators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="coordination.k8s.io",resources=leases,verbs=create;get;update
// +kubebuilder:rbac:groups="apps",resources=statefulsets;deployments;replicasets;daemonsets,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="batch",resources=jobs;cronjobs,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="",resources=nodes;namespaces;endpoints;pods;configmaps,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="*",resources="*",verbs=get;list;watch

package v2beta1
