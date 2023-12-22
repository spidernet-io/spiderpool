// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package spidercoordinator_suite_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	e2e "github.com/spidernet-io/e2eframework/framework"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

const (
	CLUSTER_POD_SUBNET_V4        = "10.233.64.0/18"
	CLUSTER_POD_SUBNET_V6        = "fd00:10:233:64::/64"
	CALICO_CLUSTER_POD_SUBNET_V4 = "10.243.64.0/18"
	CALICO_CLUSTER_POD_SUBNET_V6 = "fd00:10:243::/112"
	CILIUM_CLUSTER_POD_SUBNET_V4 = "10.244.64.0/18"
	CILIUM_CLUSTER_POD_SUBNET_V6 = "fd00:10:244::/112"
)

func TestSpiderCoordinator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SpiderCoordinator Suite")
}

var frame *e2e.Framework
var v4PodCIDRString, v6PodCIDRString string

var _ = BeforeSuite(func() {
	defer GinkgoRecover()
	var e error
	frame, e = e2e.NewFramework(GinkgoT(), []func(*runtime.Scheme) error{spiderpoolv2beta1.AddToScheme})
	Expect(e).NotTo(HaveOccurred())

	if !common.CheckRunOverlayCNI() && !common.CheckCalicoFeatureOn() && !common.CheckCiliumFeatureOn() {
		if frame.Info.IpV4Enabled {
			v4PodCIDRString = CLUSTER_POD_SUBNET_V4
		}
		if frame.Info.IpV6Enabled {
			v6PodCIDRString = CLUSTER_POD_SUBNET_V6
		}
		GinkgoWriter.Println("This environment is in underlay mode.")
	}

	if common.CheckRunOverlayCNI() && common.CheckCalicoFeatureOn() && !common.CheckCiliumFeatureOn() {
		if frame.Info.IpV4Enabled {
			v4PodCIDRString = CALICO_CLUSTER_POD_SUBNET_V4
		}
		if frame.Info.IpV6Enabled {
			v6PodCIDRString = CALICO_CLUSTER_POD_SUBNET_V6
		}
		GinkgoWriter.Println("The environment is calico mode.")
	}

	if common.CheckRunOverlayCNI() && common.CheckCiliumFeatureOn() && !common.CheckCalicoFeatureOn() {
		if frame.Info.IpV4Enabled {
			v4PodCIDRString = CILIUM_CLUSTER_POD_SUBNET_V4
		}
		if frame.Info.IpV6Enabled {
			v6PodCIDRString = CILIUM_CLUSTER_POD_SUBNET_V6
		}
		GinkgoWriter.Println("The environment is cilium mode.")
	}
})

func GetSpiderCoordinator(name string) (*spiderpoolv2beta1.SpiderCoordinator, error) {
	var spc spiderpoolv2beta1.SpiderCoordinator
	err := frame.GetResource(types.NamespacedName{
		Name: name,
	}, &spc)
	if nil != err {
		return nil, err
	}

	return &spc, nil
}

func PatchSpiderCoordinator(desired, original *spiderpoolv2beta1.SpiderCoordinator, opts ...client.PatchOption) error {

	mergePatch := client.MergeFrom(original)
	d, err := mergePatch.Data(desired)
	GinkgoWriter.Printf("the patch is: %v. \n", string(d))
	if err != nil {
		return fmt.Errorf("failed to generate patch, err is %v", err)
	}

	return frame.PatchResource(desired, mergePatch, opts...)
}
