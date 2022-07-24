package ipam

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"reflect"
	"strings"
)

var _ = Describe("Testing ipam tool", func() {
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
			{
				NIC:         "eth1",
				IPv4:        new(string),
				IPv4Pool:    new(string),
				Vlan:        new(spiderpoolv1.Vlan),
				IPv4Gateway: new(string),
				IPv6:        new(string),
				IPv6Pool:    new(string),
				IPv6Gateway: new(string),
			},
		}
		*Mockdetails[0].IPv4 = "10.1.0.5/24"
		*Mockdetails[0].IPv4Pool = "pool1"
		*Mockdetails[0].IPv4Gateway = "10.1.0.1"
		*Mockdetails[0].IPv6 = "2001:0db8:3c4d:0015:0000:0000:1a2f:1a2b/24"
		*Mockdetails[0].IPv6Pool = "pool1"
		*Mockdetails[0].IPv6Gateway = "1.2.3.1"

		*Mockdetails[1].IPv4 = "10.1.0.6/24"
		*Mockdetails[1].IPv4Pool = "pool2"
		*Mockdetails[1].IPv4Gateway = "10.1.0.2"
		*Mockdetails[1].IPv6 = "3001:0db8:3c4d:0025:0000:0000:1a2f:1a2b/24"
		*Mockdetails[1].IPv6Pool = "pool2"
		*Mockdetails[1].IPv6Gateway = "1.2.3.2"

		MockipConfigs = []*models.IPConfig{
			{
				Address: new(string),
				Gateway: "10.1.0.1",
				IPPool:  "pool1",
				Nic:     new(string),
				Version: new(int64),
				Vlan:    int64(6),
			},
			{
				Address: new(string),
				Gateway: "1.2.3.1",
				IPPool:  "pool1",
				Nic:     new(string),
				Version: new(int64),
				Vlan:    int64(6),
			},
			{
				Address: new(string),
				Gateway: "10.1.0.2",
				IPPool:  "pool2",
				Nic:     new(string),
				Version: new(int64),
				Vlan:    int64(6),
			},
			{
				Address: new(string),
				Gateway: "1.2.3.2",
				IPPool:  "pool2",
				Nic:     new(string),
				Version: new(int64),
				Vlan:    int64(6),
			},
		}
		*MockipConfigs[0].Address = "10.1.0.5/24"
		*MockipConfigs[0].Nic = "eth0"
		*MockipConfigs[0].Version = 4
		*MockipConfigs[1].Address = "2001:0db8:3c4d:0015:0000:0000:1a2f:1a2b/24"
		*MockipConfigs[1].Nic = "eth0"
		*MockipConfigs[1].Version = 6

		*MockipConfigs[2].Address = "10.1.0.6/24"
		*MockipConfigs[2].Nic = "eth1"
		*MockipConfigs[2].Version = 4
		*MockipConfigs[3].Address = "3001:0db8:3c4d:0025:0000:0000:1a2f:1a2b/24"
		*MockipConfigs[3].Nic = "eth1"
		*MockipConfigs[3].Version = 6

	})

	Context("Test conversion function", func() {

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

	Context("Text generate function", func() {

		It("Testing groupIPDetails", func() {
			containerID := "containier" + tools.RandomName()
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

	})

})
