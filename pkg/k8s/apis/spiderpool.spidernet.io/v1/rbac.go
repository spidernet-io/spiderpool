// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=spidersubnets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=spidersubnets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=spiderippools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=spiderippools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=spiderendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=spiderendpoints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=spiderreservedips,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=nodes;namespaces;endpoints;pods;secrets,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;get;list;watch;update;delete
// +kubebuilder:rbac:groups="coordination.k8s.io",resources=leases,verbs=create;get;update
// +kubebuilder:rbac:groups="apps",resources=statefulsets;deployments;replicasets;daemonsets,verbs=get;list;update;watch
// +kubebuilder:rbac:groups="batch",resources=jobs;cronjobs,verbs=get;list;update;watch
// +kubebuilder:rbac:groups="admissionregistration.k8s.io",resources=mutatingwebhookconfigurations;validatingwebhookconfigurations,verbs=get;list;update;watch;patch

package v1
