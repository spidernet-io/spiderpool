// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

// rbac marker:
// https://github.com/kubernetes-sigs/controller-tools/blob/master/pkg/rbac/parser.go
// https://book.kubebuilder.io/reference/markers/rbac.html

// +kubebuilder:rbac:groups=spiderdoctor.spidernet.io,resources=nethttps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spiderdoctor.spidernet.io,resources=nethttps/status,verbs=get;update;patch

// +kubebuilder:rbac:groups=spiderdoctor.spidernet.io,resources=netdnss,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spiderdoctor.spidernet.io,resources=netdnss/status,verbs=get;update;patch

// +kubebuilder:rbac:groups="",resources=events,verbs=create;get;list;watch;update;delete
// +kubebuilder:rbac:groups="coordination.k8s.io",resources=leases,verbs=create;get;update
// +kubebuilder:rbac:groups="apps",resources=statefulsets;deployments;replicasets;daemonsets,verbs=get;list;update;watch
// +kubebuilder:rbac:groups="batch",resources=jobs;cronjobs,verbs=get;list;update;watch
// +kubebuilder:rbac:groups="",resources=nodes;namespaces;endpoints;pods;services,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="networking.k8s.io",resources=ingresses,verbs=get;list;watch

package v1
