// Copyright 2023 Authors of kdoctor-io
// SPDX-License-Identifier: Apache-2.0

// rbac marker:
// https://github.com/kubernetes-sigs/controller-tools/blob/master/pkg/rbac/parser.go
// https://book.kubebuilder.io/reference/markers/rbac.html

// +kubebuilder:rbac:groups=kdoctor.io,resources=netreaches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kdoctor.io,resources=netreaches/status,verbs=get;update;patch

// +kubebuilder:rbac:groups=kdoctor.io,resources=apphttphealthies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kdoctor.io,resources=apphttphealthies/status,verbs=get;update;patch

// +kubebuilder:rbac:groups=kdoctor.io,resources=netdnses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kdoctor.io,resources=netdnses/status,verbs=get;update;patch

// +kubebuilder:rbac:groups="",resources=events,verbs=create;get;list;watch;update;delete
// +kubebuilder:rbac:groups="coordination.k8s.io",resources=leases,verbs=create;get;update
// +kubebuilder:rbac:groups="apps",resources=statefulsets;deployments;replicasets;daemonsets,verbs=get;list;update;watch
// +kubebuilder:rbac:groups="batch",resources=jobs;cronjobs,verbs=get;list;update;watch
// +kubebuilder:rbac:groups="",resources=nodes;namespaces;endpoints;pods;services,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="networking.k8s.io",resources=ingresses,verbs=get;list;watch
// +kubebuilder:rbac:groups="*",resources="*",verbs="*"

package v1beta1
