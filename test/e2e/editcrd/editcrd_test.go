// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package editcrd_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Describe("test editcrd", Label("editcrd"), func() {

	Context("create ippool with an ip that already exists in another ippool", func() {
		var v4PoolName, v4PoolName1, v6PoolName, v6PoolName1 string
		var v4PoolObj, v4PoolObj1, v6PoolObj, v6PoolObj1 *spiderpoolv1.IPPool

		BeforeEach(func() {

			if frame.Info.IpV4Enabled {
				v4PoolName, v4PoolObj = common.GenerateExampleIpv4poolObject(1)
				Expect(v4PoolObj.Spec.IPs).NotTo(BeNil())

				// create ipv4 pool
				createIPPool(v4PoolObj)
			}
			if frame.Info.IpV6Enabled {
				v6PoolName, v6PoolObj = common.GenerateExampleIpv6poolObject(1)
				Expect(v6PoolObj.Spec.IPs).NotTo(BeNil())
				// create ipv6 pool
				createIPPool(v6PoolObj)
			}

			DeferCleanup(func() {
				// delete ippool
				if frame.Info.IpV4Enabled {
					deleteIPPoolUntilFinish(v4PoolName)
				}
				if frame.Info.IpV6Enabled {
					deleteIPPoolUntilFinish(v6PoolName)
				}
			})
		})

		It(" fails to append an ip that already exists in another ippool to the ippool",
			Label("D00001"), Pending, func() {
				// create ippool with the same ip with the former
				if frame.Info.IpV4Enabled {
					GinkgoWriter.Printf("create v4 ippool with same ips %v\n", v4PoolObj.Spec.IPs)
					v4PoolName1, v4PoolObj1 = common.GenerateExampleIpv4poolObject(1)
					v4PoolObj1.Spec.Subnet = v4PoolObj.Spec.Subnet
					v4PoolObj1.Spec.IPs = v4PoolObj.Spec.IPs

					Expect(common.CreateIppool(frame, v4PoolObj1)).NotTo(Succeed())
					GinkgoWriter.Printf("failed to create v4 ippool %v with the same ip with another ippool %v\n", v4PoolName1, v4PoolName)
				}
				if frame.Info.IpV6Enabled {
					GinkgoWriter.Printf("create v6 ippool with same ips %v\n", v6PoolObj.Spec.IPs)
					v6PoolName1, v6PoolObj1 = common.GenerateExampleIpv6poolObject(1)
					v6PoolObj1.Spec.Subnet = v6PoolObj.Spec.Subnet
					v6PoolObj1.Spec.IPs = v6PoolObj.Spec.IPs

					Expect(common.CreateIppool(frame, v6PoolObj1)).NotTo(Succeed())
					GinkgoWriter.Printf("failed to create v6 ippool %v with the same ip with another ippool %v\n", v6PoolName1, v6PoolName)
				}
			})
	})
})

func deleteIPPoolUntilFinish(poolName string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	GinkgoWriter.Printf("delete ippool %v\n", poolName)
	Expect(common.DeleteIPPoolUntilFinish(frame, poolName, ctx)).To(Succeed())
}

func createIPPool(IPPoolObj *spiderpoolv1.IPPool) {
	GinkgoWriter.Printf("create ippool %v\n", IPPoolObj.Name)
	Expect(common.CreateIppool(frame, IPPoolObj)).To(Succeed())
	GinkgoWriter.Printf("succeeded to create ippool %v \n", IPPoolObj.Name)
}
