// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package applicationinformers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agiledragon/gomonkey/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
)

var _ = Describe("Utils", func() {
	Context("test AutoPoolName", func() {
		controllerName := "test-name"
		ipVersion := constant.IPv4
		ifName := "test-nic"

		It("normal controller name and uid", Label("unittest", "AutoPoolName"), func() {
			controllerUID := apitypes.UID("a-b-c-d")
			lastOne := "d"
			expectRes := fmt.Sprintf("auto%d-%s-%s-%s",
				ipVersion, strings.ToLower(controllerName), strings.ToLower(ifName), strings.ToLower(lastOne))

			result := AutoPoolName(controllerName, ipVersion, ifName, controllerUID)
			Expect(result).To(Equal(expectRes))
		})

		It("controller uid out of length", Label("unittest", "AutoPoolName"), func() {
			controllerUID := apitypes.UID("a-b-c-defghi")
			lastOne := "defgh"
			expectRes := fmt.Sprintf("auto%d-%s-%s-%s",
				ipVersion, strings.ToLower(controllerName), strings.ToLower(ifName), strings.ToLower(lastOne))

			result := AutoPoolName(controllerName, ipVersion, ifName, controllerUID)
			Expect(result).To(Equal(expectRes))
		})
	})

	Context("test ApplicationLabelGV", func() {
		It("no API Group with coresv1", func() {
			apiVersion := corev1.SchemeGroupVersion.String()
			Expect(ApplicationLabelGV(apiVersion)).To(Equal(corev1.SchemeGroupVersion.Version))
		})

		It("test appsv1", func() {
			apiVersion := appsv1.SchemeGroupVersion.String()
			expected := fmt.Sprintf("%s_%s", appsv1.SchemeGroupVersion.Group, appsv1.SchemeGroupVersion.Version)
			Expect(ApplicationLabelGV(apiVersion)).To(Equal(expected))
		})
	})

	Context("test ParseApplicationGVStr", func() {
		It("coresv1", func() {
			gvStr := corev1.SchemeGroupVersion.Version
			apiVersion, isMatch := ParseApplicationGVStr(gvStr)
			Expect(isMatch).To(BeTrue())
			Expect(apiVersion).To(Equal(corev1.SchemeGroupVersion.String()))
		})

		It("appsv1", func() {
			gvStr := fmt.Sprintf("%s_%s", appsv1.SchemeGroupVersion.Group, appsv1.SchemeGroupVersion.Version)
			apiVersion, isMatch := ParseApplicationGVStr(gvStr)
			Expect(isMatch).To(BeTrue())
			Expect(apiVersion).To(Equal(appsv1.SchemeGroupVersion.String()))
		})

		It("mismatch ParseApplicationGVStr", func() {
			gvStr := fmt.Sprintf("%s_%s_%s", "a", "b", "c")
			_, isMatch := ParseApplicationGVStr(gvStr)
			Expect(isMatch).To(BeFalse())
		})
	})

	Context("test ApplicationNamespacedName", func() {
		appKind := "test-kind"
		appNS := "test-ns"
		appName := "test-name"

		var appNamespacedName types.AppNamespacedName
		BeforeEach(func() {
			appNamespacedName.Kind = appKind
			appNamespacedName.Namespace = appNS
			appNamespacedName.Name = appName
		})

		It("no API Group", Label("unittest", "AppLabelValue"), func() {
			// the corev1 only has API Version, its API Group is empty
			apiVersion := corev1.SchemeGroupVersion.String()
			expectResult := fmt.Sprintf("%s_%s_%s_%s", corev1.SchemeGroupVersion.Version, appKind, appNS, appName)

			appNamespacedName.APIVersion = apiVersion
			result := ApplicationNamespacedName(appNamespacedName)

			Expect(result).To(Equal(expectResult))
		})

		It("test appsv1", Label("unittest", "AppLabelValue"), func() {
			apiVersion := appsv1.SchemeGroupVersion.String()
			expectResult := fmt.Sprintf("%s_%s_%s_%s_%s", appsv1.SchemeGroupVersion.Group, appsv1.SchemeGroupVersion.Version, appKind, appNS, appName)

			appNamespacedName.APIVersion = apiVersion
			result := ApplicationNamespacedName(appNamespacedName)

			Expect(result).To(Equal(expectResult))
		})
	})

	Context("ParseApplicationNamespacedName", Label("unittest", "ParseApplicationNamespacedName"), func() {
		appKind := "test-kind"
		appNS := "test-ns"
		appName := "test-name"

		It("match appsv1 appNamespacedName", func() {
			appNamespacedNameKey := fmt.Sprintf("%s_%s_%s_%s_%s", appsv1.SchemeGroupVersion.Group, appsv1.SchemeGroupVersion.Version, appKind, appNS, appName)

			appNamespacedName, isMatch := ParseApplicationNamespacedName(appNamespacedNameKey)
			Expect(isMatch).To(BeTrue())
			Expect(appNamespacedName.APIVersion).Should(Equal(appsv1.SchemeGroupVersion.String()))
			Expect(appNamespacedName.Kind).Should(Equal(appKind))
			Expect(appNamespacedName.Namespace).Should(Equal(appNS))
			Expect(appNamespacedName.Name).Should(Equal(appName))
		})

		It("match corev1 appNamespacedName", func() {
			appNamespacedNameKey := fmt.Sprintf("%s_%s_%s_%s", corev1.SchemeGroupVersion.Version, appKind, appNS, appName)

			appNamespacedName, isMatch := ParseApplicationNamespacedName(appNamespacedNameKey)
			Expect(isMatch).To(BeTrue())
			Expect(appNamespacedName.APIVersion).Should(Equal(corev1.SchemeGroupVersion.String()))
			Expect(appNamespacedName.Kind).Should(Equal(appKind))
			Expect(appNamespacedName.Namespace).Should(Equal(appNS))
			Expect(appNamespacedName.Name).Should(Equal(appName))
		})

		It("not match", func() {
			appNamespacedNameKey := "wrong-input"
			_, isMatch := ParseApplicationNamespacedName(appNamespacedNameKey)
			Expect(isMatch).To(BeFalse())
		})
	})

	It("GetAppReplicas", Label("unittest", "GetAppReplicas"), func() {
		Expect(GetAppReplicas(nil)).To(Equal(0))
		Expect(GetAppReplicas(ptr.To(int32(4)))).To(Equal(4))
	})

	Context("GenSubnetFreeIPs", Label("unittest", "GenSubnetFreeIPs"), func() {
		var subnet spiderpoolv2beta1.SpiderSubnet

		BeforeEach(func() {
			subnet = spiderpoolv2beta1.SpiderSubnet{
				Spec: spiderpoolv2beta1.SubnetSpec{
					IPVersion: ptr.To(int64(4)),
					IPs: []string{
						"10.0.0.10-10.0.0.100",
						"10.0.1.10-10.0.1.101",
					},
					ExcludeIPs: []string{
						"10.0.1.101",
					},
				},
			}

			controlledIPPools := spiderpoolv2beta1.PoolIPPreAllocations{
				"test-pool": spiderpoolv2beta1.PoolIPPreAllocation{
					IPs: []string{
						"10.0.0.10-10.0.0.100",
					},
				},
			}
			pools, err := convert.MarshalSubnetAllocatedIPPools(controlledIPPools)
			Expect(err).NotTo(HaveOccurred())
			subnet.Status.ControlledIPPools = pools
			subnet.Status.TotalIPCount = ptr.To(int64(182))
			subnet.Status.AllocatedIPCount = ptr.To(int64(91))
		})

		It("failed to unmarshal SpiderSubnet status", func() {
			patch := gomonkey.ApplyFuncReturn(json.Unmarshal, constant.ErrUnknown)
			defer patch.Reset()

			_, err := GenSubnetFreeIPs(&subnet)
			Expect(err).To(HaveOccurred())
		})

		It("failed to ParseIPRanges", func() {
			patch := gomonkey.ApplyFuncReturn(spiderpoolip.ParseIPRange, nil, constant.ErrUnknown)
			defer patch.Reset()

			_, err := GenSubnetFreeIPs(&subnet)
			Expect(err).To(HaveOccurred())
		})

		It("failed to AssembleTotalIPs", func() {
			patch := gomonkey.ApplyFuncReturn(spiderpoolip.AssembleTotalIPs, nil, constant.ErrUnknown)
			defer patch.Reset()
			_, err := GenSubnetFreeIPs(&subnet)
			Expect(err).To(HaveOccurred())
		})

		It("succeeded to GenSubnetFreeIPs", func() {
			freeIPs, err := GenSubnetFreeIPs(&subnet)
			Expect(err).NotTo(HaveOccurred())
			Expect(freeIPs).Should(HaveLen(91))
		})
	})

	Context("GetSubnetAnnoConfig", Label("unittest", "GetSubnetAnnoConfig"), func() {
		var log = logutils.Logger.Named("test")
		defaultSubnetsAnno := `[{"interface":"eth0","ipv4":["default-v4-subnet"],"ipv6":["default-v6-subnet"]}]`

		It("failed to Unmarshal subnets", func() {
			podAnno := map[string]string{constant.AnnoSpiderSubnets: "wrong-inputs"}
			_, err := GetSubnetAnnoConfig(podAnno, log)
			Expect(err).To(HaveOccurred())
		})

		It("failed to Unmarshal subnet", func() {
			podAnno := map[string]string{constant.AnnoSpiderSubnet: "wrong-input"}
			_, err := GetSubnetAnnoConfig(podAnno, log)
			Expect(err).To(HaveOccurred())
		})

		It("no spiderSubnet annotations", func() {
			var podAnno map[string]string
			config, err := GetSubnetAnnoConfig(podAnno, log)
			Expect(err).To(BeNil())
			Expect(config).To(BeNil())
		})

		It("failed to GetPoolIPNumber", func() {
			podAnno := map[string]string{
				constant.AnnoSpiderSubnets:            defaultSubnetsAnno,
				constant.AnnoSpiderSubnetPoolIPNumber: "a",
			}

			_, err := GetSubnetAnnoConfig(podAnno, log)
			Expect(err).To(HaveOccurred())
		})

		It("negative number", func() {
			podAnno := map[string]string{
				constant.AnnoSpiderSubnets:            defaultSubnetsAnno,
				constant.AnnoSpiderSubnetPoolIPNumber: "-1",
			}

			_, err := GetSubnetAnnoConfig(podAnno, log)
			Expect(err).To(HaveOccurred())
		})

		It("fixed IP number with '5'", func() {
			podAnno := map[string]string{
				constant.AnnoSpiderSubnets:            defaultSubnetsAnno,
				constant.AnnoSpiderSubnetPoolIPNumber: "5",
			}

			config, err := GetSubnetAnnoConfig(podAnno, log)
			Expect(err).NotTo(HaveOccurred())
			Expect(config).NotTo(BeNil())
			Expect(config.AssignIPNum).To(Equal(5))
		})

		It("flexible IP number with '+0'", func() {
			podAnno := map[string]string{
				constant.AnnoSpiderSubnets: defaultSubnetsAnno,
			}

			config, err := GetSubnetAnnoConfig(podAnno, log)
			Expect(err).NotTo(HaveOccurred())
			Expect(config).NotTo(BeNil())
			Expect(config.FlexibleIPNum).NotTo(BeNil())
			Expect(*config.FlexibleIPNum).To(Equal(0))
		})

		It("failed to check whether should ReclaimIPPool", func() {
			podAnno := map[string]string{
				constant.AnnoSpiderSubnets:             defaultSubnetsAnno,
				constant.AnnoSpiderSubnetReclaimIPPool: "wrong-input",
			}

			_, err := GetSubnetAnnoConfig(podAnno, log)
			Expect(err).To(HaveOccurred())
		})

		It("set ReclaimIPPool with false", func() {
			podAnno := map[string]string{
				constant.AnnoSpiderSubnets:             defaultSubnetsAnno,
				constant.AnnoSpiderSubnetReclaimIPPool: "false",
			}

			config, err := GetSubnetAnnoConfig(podAnno, log)
			Expect(err).NotTo(HaveOccurred())
			Expect(config).NotTo(BeNil())
			Expect(config.ReclaimIPPool).To(BeFalse())
		})

		It("default to reclaimPool", func() {
			podAnno := map[string]string{
				constant.AnnoSpiderSubnets:            defaultSubnetsAnno,
				constant.AnnoSpiderSubnetPoolIPNumber: "+1",
			}

			config, err := GetSubnetAnnoConfig(podAnno, log)
			Expect(err).NotTo(HaveOccurred())
			Expect(config).NotTo(BeNil())
			Expect(*config.FlexibleIPNum).To(Equal(1))
			Expect(config.ReclaimIPPool).To(BeTrue())
		})

		It("failed to mutateAndValidateSubnetAnno", func() {
			podAnno := map[string]string{
				constant.AnnoSpiderSubnets:            defaultSubnetsAnno,
				constant.AnnoSpiderSubnetPoolIPNumber: "+1",
			}
			patch := gomonkey.ApplyFuncReturn(mutateAndValidateSubnetAnno, constant.ErrUnknown)
			defer patch.Reset()

			_, err := GetSubnetAnnoConfig(podAnno, log)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("mutateAndValidateSubnetAnno", Label("unittest", "mutateAndValidateSubnetAnno"), func() {
		var subnetConfig types.PodSubnetAnnoConfig

		It("MultipleSubnets empty IPv4 subnet", func() {
			subnetConfig = types.PodSubnetAnnoConfig{
				MultipleSubnets: []types.AnnoSubnetItem{
					{Interface: "eth0", IPv4: []string{""}},
				},
			}
			Expect(mutateAndValidateSubnetAnno(&subnetConfig)).To(HaveOccurred())
		})

		It("MultipleSubnets empty IPv6 subnet", func() {
			subnetConfig = types.PodSubnetAnnoConfig{
				MultipleSubnets: []types.AnnoSubnetItem{
					{Interface: "eth0", IPv6: []string{""}},
				},
			}
			Expect(mutateAndValidateSubnetAnno(&subnetConfig)).To(HaveOccurred())
		})

		It("MultipleSubnets both IPV4 and IPv6 empty subnet", func() {
			subnetConfig = types.PodSubnetAnnoConfig{
				MultipleSubnets: []types.AnnoSubnetItem{
					{Interface: "eth0", IPv4: []string{}, IPv6: []string{}},
				},
			}
			Expect(mutateAndValidateSubnetAnno(&subnetConfig)).To(HaveOccurred())
		})

		It("MultipleSubnets containsDuplicate v4subnet", func() {
			subnetConfig = types.PodSubnetAnnoConfig{
				MultipleSubnets: []types.AnnoSubnetItem{
					{Interface: "eth0", IPv4: []string{"subnet1"}},
					{Interface: "eth1", IPv4: []string{"subnet1"}},
				},
			}
			Expect(mutateAndValidateSubnetAnno(&subnetConfig)).To(HaveOccurred())
		})

		It("MultipleSubnets containsDuplicate interface", func() {
			subnetConfig = types.PodSubnetAnnoConfig{
				MultipleSubnets: []types.AnnoSubnetItem{
					{
						IPv6:      []string{"subnet-v6-1"},
						Interface: "eth0",
					},
					{
						IPv6:      []string{"subnet-v6-2"},
						Interface: "eth0",
					},
				},
			}
			Expect(mutateAndValidateSubnetAnno(&subnetConfig)).To(HaveOccurred())
		})

		It("SingleSubnet empty IPv4 subnet", func() {
			subnetConfig = types.PodSubnetAnnoConfig{
				SingleSubnet: &types.AnnoSubnetItem{
					Interface: "eth0", IPv4: []string{""},
				},
			}
			Expect(mutateAndValidateSubnetAnno(&subnetConfig)).To(HaveOccurred())
		})

		It("SingleSubnet empty IPv6 subnet", func() {
			subnetConfig = types.PodSubnetAnnoConfig{
				SingleSubnet: &types.AnnoSubnetItem{
					Interface: "eth0", IPv6: []string{""},
				},
			}
			Expect(mutateAndValidateSubnetAnno(&subnetConfig)).To(HaveOccurred())
		})

		It("SingleSubnet both IPV4 and IPv6 empty subnet", func() {
			subnetConfig = types.PodSubnetAnnoConfig{
				SingleSubnet: &types.AnnoSubnetItem{
					Interface: "eth0", IPv4: []string{}, IPv6: []string{},
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

	Context("CalculateJobPodNum", Label("unittest", "CalculateJobPodNum"), func() {
		var jobSpecParallelism, jobSpecCompletions *int32

		It("jobSpecParallelism not nil", func() {
			jobSpecParallelism = ptr.To(int32(0))
			jobSpecCompletions = nil
			Expect(CalculateJobPodNum(jobSpecParallelism, jobSpecCompletions)).To(Equal(int(1)))

			jobSpecParallelism = ptr.To(int32(2))
			Expect(CalculateJobPodNum(jobSpecParallelism, jobSpecCompletions)).To(Equal(int(2)))
		})

		It("jobSpecCompletions not nil", func() {
			jobSpecParallelism = nil
			jobSpecCompletions = ptr.To(int32(0))
			Expect(CalculateJobPodNum(jobSpecParallelism, jobSpecCompletions)).To(Equal(int(1)))

			jobSpecCompletions = ptr.To(int32(2))
			Expect(CalculateJobPodNum(jobSpecParallelism, jobSpecCompletions)).To(Equal(int(2)))
		})

		It("both not nil", func() {
			jobSpecParallelism = ptr.To(int32(3))
			jobSpecCompletions = ptr.To(int32(0))
			Expect(CalculateJobPodNum(jobSpecParallelism, jobSpecCompletions)).To(Equal(int(1)))

			jobSpecCompletions = ptr.To(int32(2))
			Expect(CalculateJobPodNum(jobSpecParallelism, jobSpecCompletions)).To(Equal(int(2)))
		})

		It("both nil", func() {
			jobSpecParallelism = nil
			jobSpecCompletions = nil
			Expect(CalculateJobPodNum(jobSpecParallelism, jobSpecCompletions)).To(Equal(int(1)))
		})
	})

	Context("IsDefaultIPPoolMode", Label("unittest", "IsDefaultIPPoolMode"), func() {
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

	Context("GetPoolIPNumber", Label("unittest", "GetPoolIPNumber"), func() {
		It("fixed IP number", func() {
			isFlexible, ipNum, err := GetPoolIPNumber("5")
			Expect(err).NotTo(HaveOccurred())
			Expect(isFlexible).To(BeFalse())
			Expect(ipNum).To(Equal(5))
		})

		It("flexible IP number", func() {
			isFlexible, ipNum, err := GetPoolIPNumber("+5")
			Expect(err).NotTo(HaveOccurred())
			Expect(isFlexible).To(BeTrue())
			Expect(ipNum).To(Equal(5))
		})

		It("wrong input", func() {
			_, _, err := GetPoolIPNumber("++5")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("GenerateGVR", Labels{"unittest", "GenerateGVR"}, func() {
		It("appsv1-deployment", func() {
			appNamespacedName := types.AppNamespacedName{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       constant.KindDeployment,
				Namespace:  "test-ns",
				Name:       "test-name",
			}
			gvr, err := GenerateGVR(appNamespacedName)
			Expect(err).NotTo(HaveOccurred())
			Expect(gvr.Group).To(Equal(appsv1.SchemeGroupVersion.Group))
			Expect(gvr.Version).To(Equal(appsv1.SchemeGroupVersion.Version))
			Expect(gvr.Resource).To(Equal("deployments"))
		})

		It("corev1-pod", func() {
			appNamespacedName := types.AppNamespacedName{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       constant.KindPod,
				Namespace:  "test-ns",
				Name:       "test-name",
			}
			gvr, err := GenerateGVR(appNamespacedName)
			Expect(err).NotTo(HaveOccurred())
			Expect(gvr.Group).To(BeEmpty())
			Expect(gvr.Version).To(Equal(corev1.SchemeGroupVersion.Version))
			Expect(gvr.Resource).To(Equal("pods"))
		})

		It("wrong APIVersion input", func() {
			appNamespacedName := types.AppNamespacedName{
				APIVersion: "/v1/",
				Kind:       constant.KindPod,
				Namespace:  "test-ns",
				Name:       "test-name",
			}
			_, err := GenerateGVR(appNamespacedName)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("IsThirdController", Label("unittest", "IsThirdController"), func() {
		var appNamespacedName types.AppNamespacedName
		BeforeEach(func() {
			appNamespacedName.Namespace = "test-ns"
			appNamespacedName.Name = "test-name"
		})

		It("appsv1-deployment", func() {
			appNamespacedName.APIVersion = appsv1.SchemeGroupVersion.String()
			appNamespacedName.Kind = constant.KindDeployment
			isThirdController := IsThirdController(appNamespacedName)
			Expect(isThirdController).To(BeFalse())
		})

		It("openkruise-clonset", func() {
			appNamespacedName.APIVersion = kruisev1.SchemeGroupVersion.String()
			appNamespacedName.Kind = "CloneSet"
			isThirdController := IsThirdController(appNamespacedName)
			Expect(isThirdController).To(BeTrue())
		})

		It("appsv1-ClonsSet", func() {
			appNamespacedName.APIVersion = appsv1.SchemeGroupVersion.String()
			appNamespacedName.Kind = "ClonsSet"
			isThirdController := IsThirdController(appNamespacedName)
			Expect(isThirdController).To(BeTrue())
		})
	})

	Context("IsReclaimAutoPoolLabelValue", func() {
		It("reclaim auto pool", func() {
			labelValue := IsReclaimAutoPoolLabelValue(true)
			Expect(labelValue).To(Equal(constant.True))
		})

		It("don't reclaim auto pool", func() {
			labelValue := IsReclaimAutoPoolLabelValue(false)
			Expect(labelValue).To(Equal(constant.False))
		})
	})

	Context("AutoPoolIPVersionLabelValue", func() {
		It("Label IPv4", func() {
			labelValue := AutoPoolIPVersionLabelValue(constant.IPv4)
			Expect(labelValue).To(Equal(constant.LabelValueIPVersionV4))
		})

		It("Label IPv6", func() {
			labelValue := AutoPoolIPVersionLabelValue(constant.IPv6)
			Expect(labelValue).To(Equal(constant.LabelValueIPVersionV6))
		})
	})
})
