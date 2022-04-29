// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Spiderpool

package framework

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

const SpiderLabelSelector = "app.kubernetes.io/name: spiderpool"

type Option func(f *CLusterConfig)

type Framework struct {
	BaseName        string
	SystemNameSpace string
	KubeClientSet   kubernetes.Interface
	KubeConfig      *rest.Config
	CLusterConfig   *CLusterConfig
}

// CLusterConfig the install information about cluster
// TODO: CLusterConfig  more cluster information should be included
type CLusterConfig struct {
	IpFamily string
	Multus   bool
	Spider   bool
}

// NewFramework init Framework struct
func NewFramework(baseName, kubeconfig string, clusterOption ...Option) *Framework {
	if kubeconfig == "" {
		klog.Fatal("kubeconfig must be specify")
	}

	f := &Framework{
		BaseName:      baseName,
		CLusterConfig: &CLusterConfig{},
	}

	for _, option := range clusterOption {
		option(f.CLusterConfig)
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		klog.Fatal(err)
	}
	f.KubeConfig = cfg

	cfg.QPS = 1000
	cfg.Burst = 2000
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatal(err)
	}

	f.KubeClientSet = kubeClient

	return f
}

// WithIpFamily mutates the inner state to set the
// IpFamily attribute
func WithIpFamily(ipFamily string) Option {
	return func(f *CLusterConfig) {
		f.IpFamily = ipFamily
	}
}

// WithMultus mutates the inner state to set the
// Multus attribute
func WithMultus(install bool) Option {
	return func(f *CLusterConfig) {
		f.Multus = install
	}
}

// WithSpider mutates the inner state to set the
// Spider attribute
func WithSpider(install bool) Option {
	return func(f *CLusterConfig) {
		f.Multus = install
	}
}
