// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=spiderippools,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=spidersubnets;spiderendpoints;spiderreservedips;spidermultusconfigs;spiderclaimparameters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=spidercoordinators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spiderpool.spidernet.io,resources=spidersubnets/status;spiderippools/status;spidercoordinators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="coordination.k8s.io",resources=leases,verbs=create;get;update
// +kubebuilder:rbac:groups="apps",resources=statefulsets;deployments;replicasets;daemonsets,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="resource.k8s.io",resources=resourceclaims;resourceclaims/status;podschedulingcontexts/status;resourceclaimtemplates;resourceclasses;podschedulingcontexts,verbs=get;list;patch;watch;update
// +kubebuilder:rbac:groups="networking.k8s.io",resources=servicecidrs,verbs=get;list;watch
// +kubebuilder:rbac:groups="batch",resources=jobs;cronjobs,verbs=get;list;watch;update;delete
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces;endpoints;pods;pods/status;configmaps,verbs=get;list;watch;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines;virtualmachineinstances,verbs=get;list
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=mutatingwebhookconfigurations;validatingwebhookconfigurations,verbs=get;list;watch;delete;update
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kruise.io,resources=clonesets;statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=crd.projectcalico.org,resources=ippools,verbs=get;list;watch
// +kubebuilder:rbac:groups=cilium.io,resources=ciliumpodippools,verbs=get;list;watch

package v2beta1
