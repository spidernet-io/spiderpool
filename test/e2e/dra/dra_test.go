// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package dra_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/api/resource/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Describe("dra", Label("dra"), func() {

	Context("DRA Smoke test ", func() {
		var v4PoolName, v6PoolName, namespace, depName, multusNadName, spiderClaimName string

		BeforeEach(func() {
			// generate some test data
			namespace = "ns-" + common.GenerateString(10, true)
			depName = "dep-name-" + common.GenerateString(10, true)
			multusNadName = "test-multus-" + common.GenerateString(10, true)
			spiderClaimName = "spc-" + common.GenerateString(10, true)

			// create namespace and ippool
			err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				var v4PoolObj, v6PoolObj *spiderpoolv2beta1.SpiderIPPool
				if frame.Info.IpV4Enabled {
					v4PoolName, v4PoolObj = common.GenerateExampleIpv4poolObject(1)
					gateway := strings.Split(v4PoolObj.Spec.Subnet, "0/")[0] + "1"
					v4PoolObj.Spec.Gateway = &gateway
					err = common.CreateIppool(frame, v4PoolObj)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v4 IPPool %v: %v \n", v4PoolName, err)
						return err
					}
				}
				if frame.Info.IpV6Enabled {
					v6PoolName, v6PoolObj = common.GenerateExampleIpv6poolObject(1)
					gateway := strings.Split(v6PoolObj.Spec.Subnet, "/")[0] + "1"
					v6PoolObj.Spec.Gateway = &gateway
					err = common.CreateIppool(frame, v6PoolObj)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v6 IPPool %v: %v \n", v6PoolName, err)
						return err
					}
				}
				return nil
			}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())

			// Define multus cni NetworkAttachmentDefinition and create
			nad := &spiderpoolv2beta1.SpiderMultusConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      multusNadName,
					Namespace: namespace,
				},
				Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
					CniType: ptr.To(constant.MacvlanCNI),
					MacvlanConfig: &spiderpoolv2beta1.SpiderMacvlanCniConfig{
						Master: []string{common.NIC1},
						VlanID: ptr.To(int32(100)),
					},
					CoordinatorConfig: &spiderpoolv2beta1.CoordinatorSpec{
						PodDefaultRouteNIC: &common.NIC2,
					},
				},
			}
			Expect(frame.CreateSpiderMultusInstance(nad)).NotTo(HaveOccurred())

			Expect(common.CreateSpiderClaimParameter(frame, &spiderpoolv2beta1.SpiderClaimParameter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      spiderClaimName,
					Namespace: namespace,
					// kind k8s v1.29.0 -> use containerd v1.7.1 -> use cdi version(v0.5.4)
					// v0.5.4 don't support CDISpec version 0.6.0, so update the cdi version
					// by the annotation
					Annotations: map[string]string{
						constant.AnnoDraCdiVersion: "0.5.0",
					},
				},
				Spec: spiderpoolv2beta1.ClaimParameterSpec{
					RdmaAcc: true,
				},
			})).NotTo(HaveOccurred())

			DeferCleanup(func() {
				GinkgoWriter.Printf("delete spiderMultusConfig %v/%v. \n", namespace, multusNadName)
				Expect(frame.DeleteSpiderMultusInstance(namespace, multusNadName)).NotTo(HaveOccurred())

				GinkgoWriter.Printf("delete namespace %v. \n", namespace)
				Expect(frame.DeleteNamespace(namespace)).NotTo(HaveOccurred())

				if frame.Info.IpV4Enabled {
					GinkgoWriter.Printf("delete v4 ippool %v. \n", v4PoolName)
					Expect(common.DeleteIPPoolByName(frame, v4PoolName)).NotTo(HaveOccurred())
				}
				if frame.Info.IpV6Enabled {
					GinkgoWriter.Printf("delete v6 ippool %v. \n", v6PoolName)
					Expect(common.DeleteIPPoolByName(frame, v6PoolName)).NotTo(HaveOccurred())
				}

				Expect(
					common.DeleteSpiderClaimParameter(frame, spiderClaimName, namespace),
				).NotTo(HaveOccurred())
			})
		})

		It("Creating a Pod to verify DRA if works", Label("Q00001"), func() {
			// create resourceclaimtemplate
			Expect(
				common.CreateResourceClaimTemplate(frame, &v1alpha2.ResourceClaimTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name:      spiderClaimName,
						Namespace: namespace,
					},
					Spec: v1alpha2.ResourceClaimTemplateSpec{
						Spec: v1alpha2.ResourceClaimSpec{
							ResourceClassName: constant.DRADriverName,
							ParametersRef: &v1alpha2.ResourceClaimParametersReference{
								APIGroup: constant.SpiderpoolAPIGroup,
								Kind:     constant.KindSpiderClaimParameter,
								Name:     spiderClaimName,
							},
						},
					},
				})).NotTo(HaveOccurred())

			podIppoolsAnno := types.AnnoPodIPPoolsValue{
				types.AnnoIPPoolItem{
					NIC: common.NIC1,
				},
				types.AnnoIPPoolItem{
					NIC: common.NIC2,
				},
			}
			if frame.Info.IpV4Enabled {
				podIppoolsAnno[0].IPv4Pools = []string{common.SpiderPoolIPv4PoolDefault}
				podIppoolsAnno[1].IPv4Pools = []string{v4PoolName}
			}
			if frame.Info.IpV6Enabled {
				podIppoolsAnno[0].IPv6Pools = []string{common.SpiderPoolIPv6PoolDefault}
				podIppoolsAnno[1].IPv6Pools = []string{v6PoolName}
			}
			podAnnoMarshal, err := json.Marshal(podIppoolsAnno)
			Expect(err).NotTo(HaveOccurred())
			var annotations = make(map[string]string)
			annotations[common.MultusNetworks] = fmt.Sprintf("%s/%s", namespace, multusNadName)
			annotations[constant.AnnoPodIPPools] = string(podAnnoMarshal)
			deployObject := common.GenerateDraDeploymentYaml(depName, spiderClaimName, namespace, int32(1))
			deployObject.Spec.Template.Annotations = annotations
			Expect(frame.CreateDeployment(deployObject)).NotTo(HaveOccurred())

			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			depObject, err := frame.WaitDeploymentReady(depName, namespace, ctx)
			Expect(err).NotTo(HaveOccurred(), "waiting for deploy ready failed:  %v ", err)
			podList, err := frame.GetPodListByLabel(depObject.Spec.Template.Labels)
			Expect(err).NotTo(HaveOccurred(), "failed to get podList: %v ", err)

			cm, err := frame.GetConfigmap(common.SpiderPoolConfigmapName, common.SpiderPoolConfigmapNameSpace)
			Expect(err).NotTo(HaveOccurred(), "failed to get spiderpool-conf configMap: %v")

			list := strings.Split(cm.Data["conf.yml"], "\n")
			Expect(list).NotTo(BeEmpty())

			GinkgoWriter.Printf("Got list: %v\n", list)
			var draLibraryPath string
			for _, l := range list {
				if strings.Contains(l, "libraryPath") {
					GinkgoWriter.Printf("Got : %v\n", l)
					res := strings.Split(l, " ")
					Expect(len(res)).To(Equal(4))
					draLibraryPath = res[3]
					break
				}
			}

			Expect(draLibraryPath).NotTo(BeEmpty())
			GinkgoWriter.Printf("Got draLibraryPath: %v\n", draLibraryPath)
			var executeCommandResult []byte
			soBaseName := path.Base(draLibraryPath)
			for _, pod := range podList.Items {
				// check so if exist
				checkSoCommand := "ls " + draLibraryPath
				_, err := frame.ExecCommandInPod(pod.Name, pod.Namespace, checkSoCommand, ctx)
				Expect(err).NotTo(HaveOccurred(), "failed to check the dra so if ok to mount: %v", err)

				checkEnvComand := "printenv LD_PRELOAD"
				executeCommandResult, err = frame.ExecCommandInPod(pod.Name, pod.Namespace, checkEnvComand, ctx)
				Expect(err).NotTo(HaveOccurred(), "failed to check the dra so if ok to mount: %v", err)

				executeCommandResult = bytes.TrimSuffix(executeCommandResult, []byte("\n"))
				Expect(string(executeCommandResult)).To(Equal(soBaseName), "unexpected result: %s", executeCommandResult)
			}
		})
	})
})
