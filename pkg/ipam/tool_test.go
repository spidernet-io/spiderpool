// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"encoding/json"
	"reflect"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

var _ = Describe("Testing ipam tool", Label("unitest", "ipamtool_test"), func() {
	var Mockdetails []spiderpoolv1.IPAllocationDetail
	var MockipConfigs []*models.IPConfig
	var MockIpv4, MockIPv4Gateway, MockIPv4Pool string
	var MockIpv6, MockIPv6Gateway, MockIPv6Pool string

	BeforeEach(func() {
		// init IPAllocationDetail and IPConfig
		Mockdetails = []spiderpoolv1.IPAllocationDetail{
			{
				NIC:         "eth0",
				IPv4:        new(string),
				IPv4Pool:    new(string),
				Vlan:        new(spiderpoolv1.Vlan),
				IPv4Gateway: new(string),
				IPv6:        new(string),
				IPv6Pool:    new(string),
				IPv6Gateway: new(string),
			},
		}
		*Mockdetails[0].IPv4 = "127.0.0.6/24"
		*Mockdetails[0].IPv4Pool = "pool1"
		*Mockdetails[0].IPv4Gateway = "127.0.0.1"
		*Mockdetails[0].IPv6 = "2001:0db8:3c4d:0015:0000:0000:1a2f:1a2b/24"
		*Mockdetails[0].IPv6Pool = "pool1"
		*Mockdetails[0].IPv6Gateway = "127.4.0.1"

		MockipConfigs = []*models.IPConfig{
			{
				Address: new(string),
				Gateway: "127.0.0.1",
				IPPool:  "pool1",
				Nic:     new(string),
				Version: new(int64),
				Vlan:    int64(6),
			},
			{
				Address: new(string),
				Gateway: "127.4.0.1",
				IPPool:  "pool1",
				Nic:     new(string),
				Version: new(int64),
				Vlan:    int64(6),
			},
		}

		*MockipConfigs[0].Address = "127.0.0.6/24"
		*MockipConfigs[0].Nic = "eth0"
		*MockipConfigs[0].Version = constant.IPv4

		*MockipConfigs[1].Address = "2001:0db8:3c4d:0015:0000:0000:1a2f:1a2b/24"
		*MockipConfigs[1].Nic = "eth0"
		*MockipConfigs[1].Version = constant.IPv6

	})

	Context("Test conversion function", func() {

		It("Testing convertToIPConfigs", func() {
			var MockIp, MockIPGateway string
			// get IPConfig using convertToIPConfigs
			ipConfigs := convertToIPConfigs(Mockdetails)

			// Check for whether successfully convert
			for num, ipconf := range ipConfigs {
				if *ipconf.Version == constant.IPv4 {
					MockIp = *Mockdetails[num/2].IPv4
					MockIPGateway = *Mockdetails[num/2].IPv4Gateway
				} else {
					MockIp = *Mockdetails[num/2].IPv6
					MockIPGateway = *Mockdetails[num/2].IPv6Gateway
				}
				MockVlan := int64(*Mockdetails[num/2].Vlan)

				//check ipconf.Address
				Expect(reflect.DeepEqual(*ipconf.Address, MockIp)).To(Equal(true))

				//check ipconf.Gateway
				Expect(reflect.DeepEqual(ipconf.Gateway, MockIPGateway)).To(Equal(true))

				//check ipconf.Nic
				Expect(reflect.DeepEqual(*ipconf.Nic, Mockdetails[num/2].NIC)).To(Equal(true))

				//check ipconf.Vlan
				Expect(reflect.DeepEqual(ipconf.Vlan, MockVlan)).To(Equal(true))
			}

		})

		It("Testing convertToIPDetails", func() {
			// get IPConfig using IPAllocationDetail
			IPalloDetail := convertToIPDetails(MockipConfigs)

			// Check for whether successful convert
			for num, ipdetail := range IPalloDetail {
				MockIpv4 = *ipdetail.IPv4
				MockIPv4Gateway = *ipdetail.IPv4Gateway
				MockIPv4Pool = *ipdetail.IPv4Pool

				MockIpv6 = *ipdetail.IPv6
				MockIPv6Gateway = *ipdetail.IPv6Gateway
				MockIPv6Pool = *ipdetail.IPv6Pool

				MockVlan := int64(*ipdetail.Vlan)
				MockNIC := ipdetail.NIC

				// check ipConfigs.Address in ipv4,ipv6
				Expect(reflect.DeepEqual(*MockipConfigs[num*2].Address, MockIpv4)).To(Equal(true))
				Expect(reflect.DeepEqual(*MockipConfigs[num*2+1].Address, MockIpv6)).To(Equal(true))

				// check ipConfigs.Gateway
				Expect(reflect.DeepEqual(MockipConfigs[num*2].Gateway, MockIPv4Gateway)).To(Equal(true))
				Expect(reflect.DeepEqual(MockipConfigs[num*2+1].Gateway, MockIPv6Gateway)).To(Equal(true))

				// check ipConfigs.Nic
				Expect(reflect.DeepEqual(*MockipConfigs[num*2].Nic, MockNIC)).To(Equal(true))
				Expect(reflect.DeepEqual(*MockipConfigs[num*2+1].Nic, MockNIC)).To(Equal(true))

				// check ipConfigs.Vlan
				Expect(reflect.DeepEqual(MockipConfigs[num*2].Vlan, MockVlan)).To(Equal(true))
				Expect(reflect.DeepEqual(MockipConfigs[num*2+1].Vlan, MockVlan)).To(Equal(true))

				// check ipConfigs.IPPool
				Expect(reflect.DeepEqual(MockipConfigs[num*2].IPPool, MockIPv4Pool)).To(Equal(true))
				Expect(reflect.DeepEqual(MockipConfigs[num*2+1].IPPool, MockIPv6Pool)).To(Equal(true))
			}
		})
	})

	Context("Test pool attribute generation function", func() {

		It("Testing groupIPDetails", func() {
			containerID := "container" + tools.RandomName()
			groupDetail := groupIPDetails(containerID, Mockdetails)

			// Check for whether successfully name containder id and ip
			var ipPool string
			for _, ipdetail := range Mockdetails {
				ipPool = *ipdetail.IPv4Pool
				MockIpv4 = strings.Split(*ipdetail.IPv4, "/")[0]
				MockIpv6 = strings.Split(*ipdetail.IPv6, "/")[0]
				ipWithcid := groupDetail[ipPool]

				// check ipdetail.IPv4
				Expect(reflect.DeepEqual(MockIpv4, ipWithcid[0].IP)).To(Equal(true))

				// check ipdetail.IPv6
				Expect(reflect.DeepEqual(MockIpv6, ipWithcid[1].IP)).To(Equal(true))

				// check ipdetail.containerID
				Expect(reflect.DeepEqual(containerID, ipWithcid[0].ContainerID)).To(Equal(true))

				// check ipdetail.containerID
				Expect(reflect.DeepEqual(containerID, ipWithcid[1].ContainerID)).To(Equal(true))
			}
		})

		It("Testing genIPAssignmentAnnotation", func() {
			ipWithAnno, err := genIPAssignmentAnnotation(MockipConfigs)
			Expect(err).ShouldNot(HaveOccurred())

			var assignNic string
			var mockIp, mockIPPool string
			for _, ipConfig := range MockipConfigs {
				assignNic = constant.AnnotationPre + "/assigned-" + *ipConfig.Nic
				anno := new(types.AnnoPodAssignedEthxValue)
				err := json.Unmarshal([]byte(ipWithAnno[assignNic]), anno)
				Expect(err).ShouldNot(HaveOccurred())

				// Judge IP version
				if *ipConfig.Version == constant.IPv4 {
					mockIp = anno.IPv4
					mockIPPool = anno.IPv4Pool
				} else {
					mockIp = anno.IPv6
					mockIPPool = anno.IPv6Pool
				}

				// check ipConfig.Vlan
				Expect(ipConfig.Vlan).Should(Equal(int64(anno.Vlan)))

				// check ipConfig.eth
				Expect(*ipConfig.Nic).Should(Equal(anno.NIC))

				// check ipConfig.IPPool
				Expect(ipConfig.IPPool).Should(Equal(mockIPPool))

				// check ipConfig.Address
				Expect(*ipConfig.Address).Should(Equal(mockIp))
			}
		})
	})

})
