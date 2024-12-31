// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package kubevirt_test

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	e2e "github.com/spidernet-io/e2eframework/framework"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubevirtv1 "kubevirt.io/api/core/v1"
	k8yaml "sigs.k8s.io/yaml"

	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta2"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

const (
	TEST_VM_TEMPLATE_PATH = "./testvm.yaml"
	randomLength          = 6
)

var (
	vmTemplate = new(kubevirtv1.VirtualMachine)
	frame      *e2e.Framework
)

func TestKubevirt(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Kubevirt Suite")
}

var _ = BeforeSuite(func() {
	defer GinkgoRecover()

	if common.CheckRunOverlayCNI() {
		Skip("overlay CNI is installed , ignore this suite")
	}

	var err error
	frame, err = e2e.NewFramework(GinkgoT(), []func(*runtime.Scheme) error{spiderpoolv2beta1.AddToScheme, kubevirtv1.AddToScheme})
	Expect(err).NotTo(HaveOccurred())

	// make sure we have macvlan net-attach-def resource
	_, err = frame.GetMultusInstance(common.MacvlanUnderlayVlan0, common.MultusNs)
	if nil != err {
		if errors.IsNotFound(err) {
			Skip(fmt.Sprintf("no kubevirt multus CR '%s/%s' installed, ignore this suite", common.MultusNs, common.MacvlanUnderlayVlan0))
		}
		Fail(err.Error())
	}

	// make sure we have ovs net-attach-def resource
	_, err = frame.GetMultusInstance(common.OvsVlan30, common.MultusNs)
	if nil != err {
		if errors.IsNotFound(err) {
			Skip(fmt.Sprintf("no kubevirt multus CR '%s/%s' installed, ignore this suite", common.MultusNs, common.OvsVlan30))
		}
		Fail(err.Error())
	}
	_, err = frame.GetMultusInstance(common.OvsVlan40, common.MultusNs)
	if nil != err {
		if errors.IsNotFound(err) {
			Skip(fmt.Sprintf("no kubevirt multus CR '%s/%s' installed, ignore this suite", common.MultusNs, common.OvsVlan40))
		}
		Fail(err.Error())
	}

	if frame.Info.IpV4Enabled {
		_, err := getSpiderIPPoolByName(common.SpiderPoolIPv4SubnetVlan30)
		if nil != err {
			if errors.IsNotFound(err) {
				Skip(fmt.Sprintf("no kubevirt IPv4 IPPool resource '%s' installed, ignore this suite", common.SpiderPoolIPv4SubnetVlan30))
			}
			Fail(err.Error())
		}
		_, err = getSpiderIPPoolByName(common.SpiderPoolIPv4SubnetVlan40)
		if nil != err {
			if errors.IsNotFound(err) {
				Skip(fmt.Sprintf("no kubevirt IPv4 IPPool resource '%s' installed, ignore this suite", common.SpiderPoolIPv4SubnetVlan40))
			}
			Fail(err.Error())
		}
	}
	if frame.Info.IpV6Enabled {
		_, err := getSpiderIPPoolByName(common.SpiderPoolIPv6SubnetVlan30)
		if nil != err {
			if errors.IsNotFound(err) {
				Skip(fmt.Sprintf("no kubevirt IPv6 IPPool resource '%s' installed, ignore this suite", common.SpiderPoolIPv6SubnetVlan30))
			}
			Fail(err.Error())
		}
		_, err = getSpiderIPPoolByName(common.SpiderPoolIPv6SubnetVlan40)
		if nil != err {
			if errors.IsNotFound(err) {
				Skip(fmt.Sprintf("no kubevirt IPv6 IPPool resource '%s' installed, ignore this suite", common.SpiderPoolIPv6SubnetVlan40))
			}
			Fail(err.Error())
		}
	}

	readTestVMTemplate()
})

func readTestVMTemplate() {
	bytes, err := os.ReadFile(TEST_VM_TEMPLATE_PATH)
	Expect(err).NotTo(HaveOccurred())

	err = k8yaml.Unmarshal(bytes, vmTemplate)
	Expect(err).NotTo(HaveOccurred())
}

func getSpiderIPPoolByName(name string) (*spiderpoolv2beta1.SpiderIPPool, error) {
	var pool spiderpoolv2beta1.SpiderIPPool
	err := frame.GetResource(types.NamespacedName{
		Name: name,
	}, &pool)
	if nil != err {
		return nil, err
	}

	return &pool, nil
}
