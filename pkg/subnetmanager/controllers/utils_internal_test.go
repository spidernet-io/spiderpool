// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"encoding/json"
	"fmt"
	"github.com/agiledragon/gomonkey/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/types"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"strings"
)

var _ = Describe("Utils", func() {
	It("SubnetPoolName", Label("unitest", "SubnetPoolName"), func() {
		//controllerKind, controllerNS, controllerName string, ipVersion types.IPVersion, ifName string, controllerUID apitypes.UID
		controllerKind := "test-kind"
		controllerNS := "test-ns"
		controllerName := "test-name"
		ipVersion := types.IPVersion(4)
		ifName := "test-nic"
		controllerUID := apitypes.UID("a-b-c-d")
		lastOne := "d"
		expectRes := fmt.Sprintf("auto-%s-%s-%s-v%d-%s-%s",
			strings.ToLower(controllerKind), strings.ToLower(controllerNS), strings.ToLower(controllerName), ipVersion, ifName, strings.ToLower(lastOne))

		res := SubnetPoolName(controllerKind, controllerNS, controllerName, ipVersion, ifName, controllerUID)
		Expect(res).To(Equal(expectRes), "failed UT")
	})

	It("AppLabelValue", Label("unitest", "AppLabelValue"), func() {
		appKind := "test-kind"
		appNS := "test-ns"
		appName := "test-name"
		expectRes := fmt.Sprintf("%s_%s_%s", appKind, appNS, appName)
		res := AppLabelValue(appKind, appNS, appName)
		Expect(res).To(Equal(expectRes))
	})

	It("ParseAppLabelValue", Label("unitest", "ParseAppLabelValue"), func() {
		str := "a_b_c"
		//appKind, appNS, appName string, isFound bool
		appKind := "a"
		appNS := "b"
		appName := "c"

		value, ns, name, found := ParseAppLabelValue(str)
		Expect(appKind).To(Equal(value))
		Expect(appNS).To(Equal(ns))
		Expect(found).To(BeTrue())
		Expect(appName).To(Equal(name))
	})

	It("GetAppReplicas", Label("unitest", "GetAppReplicas"), func() {
		Expect(GetAppReplicas(nil)).To(Equal(0))
		Expect(GetAppReplicas(pointer.Int32(4))).To(Equal(int(4)))
	})

	Context("GenSubnetFreeIPs", Label("unitest", "GenSubnetFreeIPs"), func() {
		var subnet spiderpoolv1.SpiderSubnet
		var e = fmt.Errorf("bad")

		BeforeEach(func() {
			subnet = spiderpoolv1.SpiderSubnet{
				Spec: spiderpoolv1.SubnetSpec{
					IPVersion: pointer.Int64(4),
					IPs: []string{
						"10.0.0.10-10.0.0.100",
						"10.0.1.10-10.0.1.101",
					},
					ExcludeIPs: []string{
						"10.0.1.101",
					},
				},
				Status: spiderpoolv1.SubnetStatus{
					ControlledIPPools: spiderpoolv1.PoolIPPreAllocations{
						"test-pool": spiderpoolv1.PoolIPPreAllocation{
							IPs: []string{
								"10.0.0.10-10.0.0.100",
							},
						},
					},
				},
			}
		})

		It("failed to ParseIPRanges", func() {
			patch := gomonkey.ApplyFuncReturn(spiderpoolip.ParseIPRange, nil, e)
			defer patch.Reset()

			_, err := GenSubnetFreeIPs(&subnet)
			Expect(err).To(MatchError(e))
		})

		It("failed to AssembleTotalIPs", func() {
			patch := gomonkey.ApplyFuncReturn(spiderpoolip.AssembleTotalIPs, nil, e)
			defer patch.Reset()
			_, err := GenSubnetFreeIPs(&subnet)
			Expect(err).To(MatchError(e))
		})

		It("succeeded to GenSubnetFreeIPs", func() {
			_, err := GenSubnetFreeIPs(&subnet)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("GetSubnetAnnoConfig", Label("unitest", "GetSubnetAnnoConfig"), func() {
		var podAnno map[string]string
		var e = fmt.Errorf("bad")
		var log = logutils.Logger.Named("test")
		//ipam.spidernet.io/subnets: '[{"interface":"eth0","ipv4":["default-v4-subnet"],"ipv6":["default-v6-subnet"]}]'
		var patches []*gomonkey.Patches

		AfterEach(func() {
			for _, patch := range patches {
				if patch != nil {
					patch.Reset()
				}
			}
		})

		It("failed to Unmarshal subnets", func() {
			podAnno = map[string]string{constant.AnnoSpiderSubnets: "test-val"}
			patch := gomonkey.ApplyFuncReturn(json.Unmarshal, e)
			defer patch.Reset()

			_, err := GetSubnetAnnoConfig(podAnno, log)
			Expect(err).To(HaveOccurred())
		})

		It("failed to Unmarshal subnet", func() {
			podAnno = map[string]string{constant.AnnoSpiderSubnet: "test-val"}
			patch := gomonkey.ApplyFuncReturn(json.Unmarshal, e)
			defer patch.Reset()

			_, err := GetSubnetAnnoConfig(podAnno, log)
			Expect(err).To(HaveOccurred())
		})

		It("no spiderSubnet annotations", func() {
			podAnno = map[string]string{"noSubnetKey": "test-val"}

			config, err := GetSubnetAnnoConfig(podAnno, log)
			Expect(err).To(BeNil())
			Expect(config).To(BeNil())
		})

		It("failed to GetPoolIPNumber", func() {
			podAnno = map[string]string{
				constant.AnnoSpiderSubnet:             "test-val",
				constant.AnnoSpiderSubnetPoolIPNumber: "+1",
			}

			patch := gomonkey.ApplyFuncReturn(json.Unmarshal, nil)
			patches = append(patches, patch)
			patch = gomonkey.ApplyFuncReturn(GetPoolIPNumber, false, 0, e)
			patches = append(patches, patch)

			config, err := GetSubnetAnnoConfig(podAnno, log)
			Expect(err).To(MatchError(e))
			Expect(config).To(BeNil())
		})

		It("negative number", func() {
			podAnno = map[string]string{
				constant.AnnoSpiderSubnet:             "test-val",
				constant.AnnoSpiderSubnetPoolIPNumber: "+1",
			}

			patch := gomonkey.ApplyFuncReturn(json.Unmarshal, nil)
			patches = append(patches, patch)
			patch = gomonkey.ApplyFuncReturn(GetPoolIPNumber, false, -1, nil)
			patches = append(patches, patch)

			config, err := GetSubnetAnnoConfig(podAnno, log)
			Expect(err).To(HaveOccurred())
			Expect(config).To(BeNil())
		})

		It("negative number", func() {
			podAnno = map[string]string{
				constant.AnnoSpiderSubnet:             "test-val",
				constant.AnnoSpiderSubnetPoolIPNumber: "+1",
			}

			patch := gomonkey.ApplyFuncReturn(json.Unmarshal, nil)
			patches = append(patches, patch)
			patch = gomonkey.ApplyFuncReturn(GetPoolIPNumber, false, -1, nil)
			patches = append(patches, patch)

			config, err := GetSubnetAnnoConfig(podAnno, log)
			Expect(err).To(HaveOccurred())
			Expect(config).To(BeNil())
		})

		It("failed to ShouldReclaimIPPool", func() {
			podAnno = map[string]string{
				constant.AnnoSpiderSubnet: "test-val",
			}

			patch := gomonkey.ApplyFuncReturn(json.Unmarshal, nil)
			patches = append(patches, patch)
			patch = gomonkey.ApplyFuncReturn(ShouldReclaimIPPool, nil, e)
			patches = append(patches, patch)

			config, err := GetSubnetAnnoConfig(podAnno, log)
			Expect(err).To(HaveOccurred())
			Expect(config).To(BeNil())
		})

		It("reclaimPool is true", func() {
			podAnno = map[string]string{
				constant.AnnoSpiderSubnet:             "test-val",
				constant.AnnoSpiderSubnetPoolIPNumber: "+1",
			}

			patch := gomonkey.ApplyFuncReturn(GetPoolIPNumber, true, +1, nil)
			patches = append(patches, patch)
			patch = gomonkey.ApplyFuncReturn(json.Unmarshal, nil)
			patches = append(patches, patch)
			patch = gomonkey.ApplyFuncReturn(ShouldReclaimIPPool, false, nil)
			patches = append(patches, patch)
			patch = gomonkey.ApplyFuncReturn(mutateAndValidateSubnetAnno, nil)
			patches = append(patches, patch)

			config, err := GetSubnetAnnoConfig(podAnno, log)
			Expect(err).NotTo(HaveOccurred())
			Expect(config).NotTo(BeNil())
			Expect(*config.FlexibleIPNum).To(Equal(int(1)))
			Expect(config.ReclaimIPPool).To(BeFalse())
		})

		It("isFlexible is true", func() {
			podAnno = map[string]string{
				constant.AnnoSpiderSubnet: "test-val",
			}

			patch := gomonkey.ApplyFuncReturn(json.Unmarshal, nil)
			patches = append(patches, patch)
			patch = gomonkey.ApplyFuncReturn(ShouldReclaimIPPool, true, nil)
			patches = append(patches, patch)
			patch = gomonkey.ApplyFuncReturn(mutateAndValidateSubnetAnno, nil)
			patches = append(patches, patch)

			config, err := GetSubnetAnnoConfig(podAnno, log)
			Expect(err).NotTo(HaveOccurred())
			Expect(config).NotTo(BeNil())
			Expect(config.ReclaimIPPool).To(BeTrue())
		})

		It("isFlexible is false", func() {
			podAnno = map[string]string{
				constant.AnnoSpiderSubnet:             "test-val",
				constant.AnnoSpiderSubnetPoolIPNumber: "+1",
			}
			patch := gomonkey.ApplyFuncReturn(GetPoolIPNumber, false, +1, nil)
			patches = append(patches, patch)
			patch = gomonkey.ApplyFuncReturn(json.Unmarshal, nil)
			patches = append(patches, patch)
			patch = gomonkey.ApplyFuncReturn(ShouldReclaimIPPool, true, nil)
			patches = append(patches, patch)
			patch = gomonkey.ApplyFuncReturn(mutateAndValidateSubnetAnno, nil)
			patches = append(patches, patch)

			config, err := GetSubnetAnnoConfig(podAnno, log)
			Expect(err).NotTo(HaveOccurred())
			Expect(config).NotTo(BeNil())
			Expect(config.AssignIPNum).To(Equal(int(1)))
		})

		It("failed to mutateAndValidateSubnetAnno", func() {
			podAnno = map[string]string{
				constant.AnnoSpiderSubnet:             "test-val",
				constant.AnnoSpiderSubnetPoolIPNumber: "+1",
			}
			patch := gomonkey.ApplyFuncReturn(GetPoolIPNumber, false, +1, nil)
			patches = append(patches, patch)
			patch = gomonkey.ApplyFuncReturn(json.Unmarshal, nil)
			patches = append(patches, patch)
			patch = gomonkey.ApplyFuncReturn(ShouldReclaimIPPool, true, nil)
			patches = append(patches, patch)

			config, err := GetSubnetAnnoConfig(podAnno, log)
			Expect(err).To(HaveOccurred())
			Expect(config).To(BeNil())
		})
	})

	Context("mutateAndValidateSubnetAnno", Label("unitest", "mutateAndValidateSubnetAnno"), func() {
		var subnetConfig types.PodSubnetAnnoConfig

		It("MultipleSubnets empty IPv4 subnet", func() {
			subnetConfig = types.PodSubnetAnnoConfig{
				MultipleSubnets: []types.AnnoSubnetItem{
					{IPv4: []string{""}},
				},
			}
			Expect(mutateAndValidateSubnetAnno(&subnetConfig)).To(HaveOccurred())
		})

		It("MultipleSubnets empty IPv6 subnet", func() {
			subnetConfig = types.PodSubnetAnnoConfig{
				MultipleSubnets: []types.AnnoSubnetItem{
					{IPv6: []string{""}},
				},
			}
			Expect(mutateAndValidateSubnetAnno(&subnetConfig)).To(HaveOccurred())
		})

		It("MultipleSubnets both IPV4 and IPv6 empty subnet", func() {
			subnetConfig = types.PodSubnetAnnoConfig{
				MultipleSubnets: []types.AnnoSubnetItem{
					{IPv4: []string{}},
					{IPv6: []string{}},
				},
			}
			Expect(mutateAndValidateSubnetAnno(&subnetConfig)).To(HaveOccurred())
		})

		It("MultipleSubnets containsDuplicate v4subnet", func() {
			subnetConfig = types.PodSubnetAnnoConfig{
				MultipleSubnets: []types.AnnoSubnetItem{
					{IPv4: []string{"10.0.0.0/16"}},
					{IPv4: []string{"10.0.0.0/16"}},
				},
			}
			Expect(mutateAndValidateSubnetAnno(&subnetConfig)).To(HaveOccurred())
		})

		It("MultipleSubnets containsDuplicate interface", func() {
			subnetConfig = types.PodSubnetAnnoConfig{
				MultipleSubnets: []types.AnnoSubnetItem{
					{
						IPv6:      []string{"fd00:10::/120"},
						Interface: "a",
					},
					{
						IPv6:      []string{"fd00:20::/120"},
						Interface: "a",
					},
				},
			}
			Expect(mutateAndValidateSubnetAnno(&subnetConfig)).To(HaveOccurred())
		})

		It("SingleSubnet empty IPv4 subnet", func() {
			subnetConfig = types.PodSubnetAnnoConfig{
				SingleSubnet: &types.AnnoSubnetItem{
					IPv4: []string{""},
				},
			}
		})
		It("SingleSubnet empty IPv6 subnet", func() {
			subnetConfig = types.PodSubnetAnnoConfig{
				SingleSubnet: &types.AnnoSubnetItem{
					IPv6: []string{""},
				},
			}
		})

		It("SingleSubnet both IPV4 and IPv6 empty subnet", func() {
			subnetConfig = types.PodSubnetAnnoConfig{
				SingleSubnet: &types.AnnoSubnetItem{
					IPv4: []string{},
					IPv6: []string{},
				},
			}
			Expect(mutateAndValidateSubnetAnno(&subnetConfig)).To(HaveOccurred())
		})

		It("specify 'eth0' as the default single interface if it's none", func() {
			subnetConfig = types.PodSubnetAnnoConfig{
				SingleSubnet: &types.AnnoSubnetItem{
					IPv4:      []string{"10.0.0.0/16"},
					Interface: "",
				},
			}
			Expect(mutateAndValidateSubnetAnno(&subnetConfig)).NotTo(HaveOccurred())
		})

		It("no subnets specified", func() {
			subnetConfig = types.PodSubnetAnnoConfig{
				MultipleSubnets: []types.AnnoSubnetItem{},
			}
			Expect(mutateAndValidateSubnetAnno(&subnetConfig)).To(HaveOccurred())
		})

		It("succeeded to mutateAndValidateSubnetAnno", func() {
			subnetConfig = types.PodSubnetAnnoConfig{
				MultipleSubnets: []types.AnnoSubnetItem{
					{
						IPv4:      []string{"10.0.0.0/16"},
						Interface: "a",
					},
				},
			}
			Expect(mutateAndValidateSubnetAnno(&subnetConfig)).NotTo(HaveOccurred())
		})
	})

	Context("GetPoolIPNumber", Label("unitest", "GetPoolIPNumber"), func() {
		var str string
		It("invalid input", func() {
			str = "+++123"
			ok, num, err := GetPoolIPNumber(str)
			Expect(err).To(HaveOccurred())
			Expect(ok).To(BeFalse())
			Expect(num).To(Equal(int(-1)))

			str = "+bad"
			ok, num, err = GetPoolIPNumber(str)
			Expect(err).To(HaveOccurred())
			Expect(ok).To(BeFalse())
			Expect(num).To(Equal(int(-1)))
		})

		It("succeeded get ip number", func() {
			str = "+123"
			ok, num, err := GetPoolIPNumber(str)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(num).To(Equal(int(123)))

			str = "321"
			ok, num, err = GetPoolIPNumber(str)
			GinkgoWriter.Printf("ok:%t,num:%d,err:%w\n", ok, num, err)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
			Expect(num).To(Equal(int(321)))
		})
	})

	Context("CalculateJobPodNum", Label("unitest", "CalculateJobPodNum"), func() {
		var jobSpecParallelism, jobSpecCompletions *int32

		It("jobSpecParallelism not nil", func() {
			jobSpecParallelism = pointer.Int32(0)
			jobSpecCompletions = nil
			Expect(CalculateJobPodNum(jobSpecParallelism, jobSpecCompletions)).To(Equal(int(1)))

			jobSpecParallelism = pointer.Int32(2)
			Expect(CalculateJobPodNum(jobSpecParallelism, jobSpecCompletions)).To(Equal(int(2)))
		})

		It("jobSpecCompletions not nil", func() {
			jobSpecParallelism = nil
			jobSpecCompletions = pointer.Int32(0)
			Expect(CalculateJobPodNum(jobSpecParallelism, jobSpecCompletions)).To(Equal(int(1)))

			jobSpecCompletions = pointer.Int32(2)
			Expect(CalculateJobPodNum(jobSpecParallelism, jobSpecCompletions)).To(Equal(int(2)))
		})

		It("both not nil", func() {
			jobSpecParallelism = pointer.Int32(3)
			jobSpecCompletions = pointer.Int32(0)
			Expect(CalculateJobPodNum(jobSpecParallelism, jobSpecCompletions)).To(Equal(int(1)))

			jobSpecCompletions = pointer.Int32(2)
			Expect(CalculateJobPodNum(jobSpecParallelism, jobSpecCompletions)).To(Equal(int(2)))
		})

		It("both nil", func() {
			jobSpecParallelism = nil
			jobSpecCompletions = nil
			Expect(CalculateJobPodNum(jobSpecParallelism, jobSpecCompletions)).To(Equal(int(1)))
		})
	})

	Context("IsDefaultIPPoolMode", Label("unitest", "IsDefaultIPPoolMode"), func() {
		var subnetConfig *types.PodSubnetAnnoConfig
		It("nil subnetConfig", func() {
			subnetConfig = nil
			Expect(IsDefaultIPPoolMode(subnetConfig)).To(BeTrue())
		})

		It("SpiderSubnet with multiple interfaces", func() {
			subnetConfig = &types.PodSubnetAnnoConfig{
				MultipleSubnets: []types.AnnoSubnetItem{
					{Interface: "a"},
				},
			}
			Expect(IsDefaultIPPoolMode(subnetConfig)).To(BeFalse())
		})

		It("SpiderSubnet with single interface", func() {
			subnetConfig = &types.PodSubnetAnnoConfig{
				SingleSubnet: &types.AnnoSubnetItem{
					Interface: "a",
				},
			}
			Expect(IsDefaultIPPoolMode(subnetConfig)).To(BeFalse())
		})

		It("not IsDefaultIPPoolMode", func() {
			subnetConfig = &types.PodSubnetAnnoConfig{}
			Expect(IsDefaultIPPoolMode(subnetConfig)).To(BeFalse())
		})
	})

	Context("ShouldReclaimIPPool", Label("unitest", "ShouldReclaimIPPool"), func() {
		var anno map[string]string
		It("failed to ParseBool", func() {
			anno = map[string]string{constant.AnnoSpiderSubnetReclaimIPPool: "bad-value"}
			ok, err := ShouldReclaimIPPool(anno)
			Expect(err).To(HaveOccurred())
			Expect(ok).To(BeFalse())
		})

		It("succeeded to ParseBool", func() {
			anno = map[string]string{constant.AnnoSpiderSubnetReclaimIPPool: "false"}
			ok, err := ShouldReclaimIPPool(anno)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())

			anno = map[string]string{constant.AnnoSpiderSubnetReclaimIPPool: "true"}
			ok, err = ShouldReclaimIPPool(anno)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("no specified reclaim-IPPool", func() {
			anno = map[string]string{"bad-key": "bad-value"}
			ok, err := ShouldReclaimIPPool(anno)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})
	})

})
