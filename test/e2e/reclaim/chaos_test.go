// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package reclaim_test

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"golang.org/x/net/context"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta2"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubectl/pkg/util/podutils"
)

var _ = Describe("Chaos Testing of GC", Label("reclaim"), func() {

	Context("GC correctly handles ip addresses", func() {
		var (
			gcNamespace                    string = "sts-ns" + tools.RandomName()
			replicasNum                    int32  = 5
			v4PoolName, v6PoolName         string
			v4PoolObj, v6PoolObj           *spiderpool.SpiderIPPool
			v4PoolNameList, v6PoolNameList []string
			err                            error
		)

		BeforeEach(func() {

			err = frame.CreateNamespaceUntilDefaultServiceAccountReady(gcNamespace, common.ServiceAccountReadyTimeout)
			Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", gcNamespace)

			Eventually(func() error {
				if frame.Info.IpV4Enabled {
					v4PoolName, v4PoolObj = common.GenerateExampleIpv4poolObject(5)
					err = common.CreateIppool(frame, v4PoolObj)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v4 IPPool %v: %v \n", v4PoolName, err)
						return err
					}
					v4PoolNameList = append(v4PoolNameList, v4PoolName)
				}
				if frame.Info.IpV6Enabled {
					v6PoolName, v6PoolObj = common.GenerateExampleIpv6poolObject(5)
					err = common.CreateIppool(frame, v6PoolObj)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v6 IPPool %v: %v \n", v6PoolName, err)
						return err
					}
					v6PoolNameList = append(v6PoolNameList, v6PoolName)
				}
				return err
			}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())

			DeferCleanup(func() {
				defer GinkgoRecover()
				if CurrentSpecReport().Failed() {
					GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
					return
				}

				Expect(frame.DeleteNamespace(gcNamespace)).NotTo(HaveOccurred(), "failed to delete namespace %v", gcNamespace)
				if frame.Info.IpV4Enabled {
					Expect(common.DeleteIPPoolByName(frame, v4PoolName)).NotTo(HaveOccurred())
				}
				if frame.Info.IpV6Enabled {
					Expect(common.DeleteIPPoolByName(frame, v6PoolName)).NotTo(HaveOccurred())
				}
			})
		})

		It("The IPPool is used by 2 statefulSets and scaling up/down the replicas, gc works normally and there is no IP conflict in statefulset.", Label("G00011"), func() {
			var (
				stsNameOne string = "sts-1-" + tools.RandomName()
				stsNameTwo string = "sts-2-" + tools.RandomName()
			)

			// 1. Using the default pool, create a statefulset application with 5 replicas and check if spiderpool has assigned it an IP address.
			var annotations = make(map[string]string)
			podIppoolAnnoStr := common.GeneratePodIPPoolAnnotations(frame, common.NIC1, v4PoolNameList, v6PoolNameList)
			annotations[constant.AnnoPodIPPool] = podIppoolAnnoStr
			annotations[common.MultusDefaultNetwork] = fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanUnderlayVlan0)
			stsOneYaml := common.GenerateExampleStatefulSetYaml(stsNameOne, gcNamespace, replicasNum)
			stsOneYaml.Spec.Template.Annotations = annotations
			GinkgoWriter.Printf("Try to create first StatefulSet %v/%v \n", gcNamespace, stsNameOne)
			Expect(frame.CreateStatefulSet(stsOneYaml)).To(Succeed(), "failed to create first StatefulSet %v/%v \n", gcNamespace, stsNameOne)

			var podList *corev1.PodList
			Eventually(func() bool {
				podList, err = frame.GetPodListByLabel(stsOneYaml.Spec.Template.Labels)
				if nil != err || len(podList.Items) == 0 || len(podList.Items) != int(*stsOneYaml.Spec.Replicas) {
					return false
				}
				return frame.CheckPodListRunning(podList)
			}, common.PodStartTimeout, common.ForcedWaitingTime).Should(BeTrue())
			ok, _, _, err := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			// 2. Reduce the number of replicas of the first statefulset 5 to 0.
			// Since statefulset deletes Pods one by one, it is expected that when there are 3 replicas left,
			// additional statefulset applications will be created.
			stsOneObj, err := frame.GetStatefulSet(stsNameOne, gcNamespace)
			Expect(err).NotTo(HaveOccurred())
			desiredStsOneObj := stsOneObj.DeepCopy()
			desiredStsOneObj.Spec.Replicas = ptr.To(int32(0))
			Expect(common.PatchStatefulSet(frame, desiredStsOneObj, stsOneObj)).NotTo(HaveOccurred())
			GinkgoWriter.Printf("Reduce the number of application replicas of the first statefulset %v/%v from 5 to 0 \n", gcNamespace, stsNameOne)

			Eventually(func() bool {
				podList, err = frame.GetPodListByLabel(desiredStsOneObj.Spec.Template.Labels)
				if nil != err || len(podList.Items) > 3 {
					return false
				}

				GinkgoWriter.Printf("When the first statefulset replica is scaled down to 3, current replicas: %v , perform other operations. \n", len(podList.Items))
				return true
			}, common.PodStartTimeout, common.ForcedWaitingTime).Should(BeTrue())

			// 3. Create another statefulset application and use the same IPPool.
			stsTwoYaml := common.GenerateExampleStatefulSetYaml(stsNameTwo, gcNamespace, replicasNum)
			stsTwoYaml.Spec.Template.Annotations = annotations
			GinkgoWriter.Printf("Try to create second StatefulSet %v/%v \n", gcNamespace, stsNameTwo)
			Expect(frame.CreateStatefulSet(stsTwoYaml)).To(Succeed(), "failed to create second StatefulSet %v/%v \n", gcNamespace, stsNameTwo)

			// 4. Restore the number of replicas of the first statefulset application from 0 to 5.
			stsOneObj, err = frame.GetStatefulSet(stsNameOne, gcNamespace)
			Expect(err).NotTo(HaveOccurred())
			desiredStsOneObj = stsOneObj.DeepCopy()
			desiredStsOneObj.Spec.Replicas = ptr.To(int32(5))
			Expect(common.PatchStatefulSet(frame, desiredStsOneObj, stsOneObj)).NotTo(HaveOccurred())
			GinkgoWriter.Printf("Restore the number of application replicas of the first statefulset %v/%v from 0 to 5 \n", gcNamespace, stsNameOne)

			// 5. It is expected that there are only 5 Pods in 2 statefulsets that can run normally and their IP addresses have no conflicts.
			timeout := 1 * time.Minute
			startTime := time.Now()
			var runningPodList corev1.PodList
			for {
				// The time of GC error collection is uncertain, so we set a time of 1 minutes.
				// In CI, GC All is triggered every two minutes.
				// It is expected that GC All and tracePod_worker processes will be triggered multiple times within 1 minutes to check whether the IP address is incorrectly collected by GC,
				// resulting in an IP address conflict.
				if time.Since(startTime) >= timeout {
					fmt.Println("3 minutes have passed, IP conflict check passed, exiting.")
					break
				}
				GinkgoWriter.Printf("Start checking for IP conflicts, time: %v \n", time.Since(startTime))
				runningPodList = corev1.PodList{}
				podList, err = frame.GetPodList(client.InNamespace(gcNamespace))
				Expect(err).NotTo(HaveOccurred(), "failed to get podList, error: %v \n", err)

				isIPConflict := make(map[corev1.PodIP]string)
				for _, pod := range podList.Items {
					if !podutils.IsPodReady(&pod) {
						continue
					}

					runningPodList.Items = append(runningPodList.Items, pod)
					// Check if there is any conflict in the recorded IPv4 or IPv6 address
					for _, ip := range pod.Status.PodIPs {
						if _, ok := isIPConflict[ip]; ok {
							errorString := fmt.Sprintf("The IP address:%v of Pod %v conflicts with the IP address: %v of Pod %v \n", ip, isIPConflict[ip], pod.Status.PodIPs, pod.Name)
							Fail(errorString)
						} else {
							isIPConflict[ip] = pod.Name
						}
					}
				}
				time.Sleep(10 * time.Second)
			}

			// Check that the number of Pods running correctly is the expected 5 (because the pool is limited to 5 IPs)
			GinkgoWriter.Printf("Start checking whether the number of Pods is equal to 5, Pod replicas: %v \n", len(runningPodList.Items))
			Expect(len(runningPodList.Items)).To(Equal(5), "expected 5 Pods in running state, found %d running Pods \n", len(runningPodList.Items))

			// Check that all Pods are assigned IPs in the expected IPPool.
			GinkgoWriter.Printf("Start checking that the Pod IPs are recorded in the desired v4 pool %v and v6 pool %v \n", v4PoolNameList, v6PoolNameList)
			ok, _, _, err = common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, &runningPodList)
			Expect(ok).To(BeTrue())
			Expect(err).NotTo(HaveOccurred(), "error: %v \n", err)
		})
	})

	Context("Chaos Testing of GC", func() {
		var (
			chaosNamespace                 string = "gc-chaos-ns-" + tools.RandomName()
			replicasNum                    int32  = 3
			testUser                       int    = 6
			ipNumInIPPool                  int    = testUser * int(replicasNum)
			v4PoolName, v6PoolName         string
			v4PoolObj, v6PoolObj           *spiderpool.SpiderIPPool
			v4PoolNameList, v6PoolNameList []string
			gcDefaultIntervalDuration      string
			err                            error
		)
		const SPIDERPOOL_GC_DEFAULT_INTERVAL_DURATION = "SPIDERPOOL_GC_DEFAULT_INTERVAL_DURATION"

		BeforeEach(func() {
			gcDefaultIntervalDuration, err = common.GetSpiderControllerEnvValue(frame, SPIDERPOOL_GC_DEFAULT_INTERVAL_DURATION)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("SPIDERPOOL_GC_DEFAULT_INTERVAL_DURATION: %s \n", gcDefaultIntervalDuration)

			err = frame.CreateNamespaceUntilDefaultServiceAccountReady(chaosNamespace, common.ServiceAccountReadyTimeout)
			Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", chaosNamespace)
			GinkgoWriter.Printf("succeed to create namespace %s \n", chaosNamespace)

			Eventually(func() error {
				if frame.Info.IpV4Enabled {
					v4PoolName, v4PoolObj = common.GenerateExampleIpv4poolObject(ipNumInIPPool)
					err = common.CreateIppool(frame, v4PoolObj)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v4 IPPool %v: %v \n", v4PoolName, err)
						return err
					}
					v4PoolNameList = append(v4PoolNameList, v4PoolName)
					GinkgoWriter.Printf("succeed to create v4 IPPool %s \n", v4PoolName)
				}
				if frame.Info.IpV6Enabled {
					v6PoolName, v6PoolObj = common.GenerateExampleIpv6poolObject(ipNumInIPPool)
					err = common.CreateIppool(frame, v6PoolObj)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v6 IPPool %v: %v \n", v6PoolName, err)
						return err
					}
					v6PoolNameList = append(v6PoolNameList, v6PoolName)
					GinkgoWriter.Printf("succeed to create v6 IPPool %s \n", v6PoolName)
				}
				return err
			}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())

			DeferCleanup(func() {
				if CurrentSpecReport().Failed() {
					GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
					return
				}

				// Finally, we need to recheck the running status of the spiderpoolController. The expected status is: running.
				ctx, cancel := context.WithTimeout(context.Background(), common.PodReStartTimeout)
				defer cancel()
				err = frame.WaitPodListRunning(map[string]string{"app.kubernetes.io/component": constant.SpiderpoolController}, len(frame.Info.KindNodeList), ctx)
				Expect(err).NotTo(HaveOccurred(), "The restarted spiderpool-controller did not recover correctly.")

				// clean all
				Expect(frame.DeleteNamespace(chaosNamespace)).NotTo(HaveOccurred(), "failed to delete namespace %v", chaosNamespace)
				GinkgoWriter.Printf("succeed to delete namespace %s \n", chaosNamespace)

				if frame.Info.IpV4Enabled {
					Expect(common.DeleteIPPoolByName(frame, v4PoolName)).NotTo(HaveOccurred())
					GinkgoWriter.Printf("succeed to delete v4 IPPool %s \n", v4PoolName)
				}
				if frame.Info.IpV6Enabled {
					Expect(common.DeleteIPPoolByName(frame, v6PoolName)).NotTo(HaveOccurred())
					GinkgoWriter.Printf("succeed to delete v6 IPPool %s \n", v6PoolName)
				}
			})
		})

		It("Multiple resource types compete for a single IPPool. In scenarios of creation, scaling up/down, and deletion, GC all can correctly handle IP addresses.", Serial, Label("G00012"), func() {
			var (
				stsNameOne       string = "gc-chaos-sts-1-" + tools.RandomName()
				stsNameTwo       string = "gc-chaos-sts-2-" + tools.RandomName()
				deployNameOne    string = "gc-chaos-deploy-1-" + tools.RandomName()
				deployNameTwo    string = "gc-chaos-deploy-2-" + tools.RandomName()
				kruiseStsNameOne string = "gc-chaos-kruise-sts-1-" + tools.RandomName()
				kruiseStsNameTwo string = "gc-chaos-kruise-sts-2-" + tools.RandomName()
			)

			// 1. Use the spiderpool annotation to specify the same IPPool for all users, allowing them to compete for IP resources.
			var annotations = map[string]string{
				constant.AnnoPodIPPool:      common.GeneratePodIPPoolAnnotations(frame, common.NIC1, v4PoolNameList, v6PoolNameList),
				common.MultusDefaultNetwork: fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanUnderlayVlan0),
			}
			GinkgoWriter.Printf("succeed to generate annotations %s \n", annotations)

			// 2. Create 6 groups of applications, including 2 groups of statefulset, 2 groups of deployment, and 2 groups of kurise statefulset
			// Users of the same type form competition, and users of different types form competition.
			// Define the creation time of the resource, which will occur at a random point within 30 seconds.
			createPodSet := func(createFunc func() error, createName string, duration time.Duration) error {
				time.Sleep(time.Duration(rand.Intn(int(duration.Seconds()))) * time.Second)

				startTime := time.Now()
				err := createFunc()
				if err != nil {
					return fmt.Errorf("failed to create %s/%s, at time %v, error %v", chaosNamespace, createName, startTime, err)
				}

				GinkgoWriter.Printf("Succeeded in creating %s/%s at time %v \n", chaosNamespace, createName, startTime)
				return nil
			}

			createFuncs := map[string]func() error{
				stsNameOne: func() error {
					stsOneYaml := common.GenerateExampleStatefulSetYaml(stsNameOne, chaosNamespace, replicasNum)
					stsOneYaml.Spec.Template.Annotations = annotations
					stsOneYaml.Spec.Template.Labels = map[string]string{
						"app":       stsOneYaml.Name,
						"namespace": chaosNamespace,
					}
					return frame.CreateStatefulSet(stsOneYaml)
				},
				stsNameTwo: func() error {
					stsTwoYaml := common.GenerateExampleStatefulSetYaml(stsNameTwo, chaosNamespace, replicasNum)
					stsTwoYaml.Spec.Template.Annotations = annotations
					stsTwoYaml.Spec.Template.Labels = map[string]string{
						"app":       stsTwoYaml.Name,
						"namespace": chaosNamespace,
					}
					return frame.CreateStatefulSet(stsTwoYaml)
				},
				deployNameOne: func() error {
					deployOneYaml := common.GenerateExampleDeploymentYaml(deployNameOne, chaosNamespace, replicasNum)
					deployOneYaml.Spec.Template.Annotations = annotations
					deployOneYaml.Spec.Template.Labels = map[string]string{
						"app":       deployOneYaml.Name,
						"namespace": chaosNamespace,
					}
					return frame.CreateDeployment(deployOneYaml)
				},
				deployNameTwo: func() error {
					deployTwoYaml := common.GenerateExampleDeploymentYaml(deployNameTwo, chaosNamespace, replicasNum)
					deployTwoYaml.Spec.Template.Annotations = annotations
					deployTwoYaml.Spec.Template.Labels = map[string]string{
						"app":       deployTwoYaml.Name,
						"namespace": chaosNamespace,
					}
					return frame.CreateDeployment(deployTwoYaml)
				},
				kruiseStsNameOne: func() error {
					kruiseStsOneYaml := common.GenerateExampleKruiseStatefulSetYaml(kruiseStsNameOne, chaosNamespace, replicasNum)
					kruiseStsOneYaml.Spec.Template.Annotations = annotations
					kruiseStsOneYaml.Spec.Template.Labels = map[string]string{
						"app":       kruiseStsOneYaml.Name,
						"namespace": chaosNamespace,
					}
					return common.CreateKruiseStatefulSet(frame, kruiseStsOneYaml)
				},
				kruiseStsNameTwo: func() error {
					kruiseStsTwoYaml := common.GenerateExampleKruiseStatefulSetYaml(kruiseStsNameTwo, chaosNamespace, replicasNum)
					kruiseStsTwoYaml.Spec.Template.Annotations = annotations
					kruiseStsTwoYaml.Spec.Template.Labels = map[string]string{
						"app":       kruiseStsTwoYaml.Name,
						"namespace": chaosNamespace,
					}
					return common.CreateKruiseStatefulSet(frame, kruiseStsTwoYaml)
				},
			}

			var createWg sync.WaitGroup
			createWg.Add(len(createFuncs))
			for name, createFunc := range createFuncs {
				go func(name string, createFunc func() error) {
					defer GinkgoRecover()
					defer createWg.Done()
					err := createPodSet(createFunc, name, 30*time.Second)
					Expect(err).NotTo(HaveOccurred())
				}(name, createFunc)
			}
			createWg.Wait()

			// waitForPodsAndCheckPoolSanity is a function that performs a series of checks after waiting for a
			// specified gc all interval. It first sleeps for a duration calculated from `gcDefaultIntervalDuration` plus
			// an additional 20 seconds. After the sleep period, it ensures that all Pods within the specified
			// namespace are running as expected by using a timeout context.
			// it also verifies the sanity of the associated IP pools
			gcDefaultIntervalDuration, err := strconv.Atoi(gcDefaultIntervalDuration)
			Expect(err).NotTo(HaveOccurred())
			waitForPodsAndCheckPoolSanity := func() {
				time.Sleep(time.Duration(gcDefaultIntervalDuration+20) * time.Second)
				ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
				defer cancel()
				Expect(frame.WaitPodListRunning(map[string]string{"namespace": chaosNamespace}, ipNumInIPPool, ctx)).NotTo(HaveOccurred(),
					"failed to check pod status in namespace %s, error %v", chaosNamespace, err)
				GinkgoWriter.Printf("all Pods in the namespace %s are running normally. \n", chaosNamespace)
				podList, err := frame.GetPodListByLabel(map[string]string{"namespace": chaosNamespace})
				Expect(err).NotTo(HaveOccurred())
				Expect(common.ValidatePodIPConflict(podList)).NotTo(HaveOccurred())
				if frame.Info.IpV4Enabled {
					Expect(common.CheckIppoolSanity(frame, v4PoolName)).NotTo(HaveOccurred(), "error %v", err)
					GinkgoWriter.Printf("successfully checked sanity of spiderpool %v \n", v4PoolName)
				}
				if frame.Info.IpV6Enabled {
					Expect(common.CheckIppoolSanity(frame, v6PoolName)).NotTo(HaveOccurred(), "error %v", err)
					GinkgoWriter.Printf("successfully checked sanity of spiderpool %v \n", v6PoolName)
				}
			}

			// 3. Verify that all Pods are running normally, their IP addresses do not conflict, and the Pod UID match those in the IPPool and endpoint.
			GinkgoWriter.Println("create: starting to check the pod running status and the sanity of the IPs in the IPPool.")
			waitForPodsAndCheckPoolSanity()

			// 4. Scale up or down all applications randomly, with a random number of replicas.
			randNumbers := common.GenerateRandomNumbers(testUser*int(replicasNum), testUser)
			GinkgoWriter.Printf("Randomly scale the number of replicas: %v \n", randNumbers)

			// randomly scale up and down the replicas of the six Pod groups
			// The default replica number for all resources is replicasNum: 3
			scalePodSet := func(scaleFunc func(scale int32) error, scaleName string, scale int32, duration time.Duration) error {
				time.Sleep(time.Duration(rand.Intn(int(duration.Seconds()))) * time.Second)

				startTime := time.Now()
				err := scaleFunc(scale)
				if err != nil {
					return fmt.Errorf("failed to scale %s/%s replicas to %d at time %v, error %v", chaosNamespace, scaleName, scale, startTime, err)
				}

				GinkgoWriter.Printf("Succeeded in scaling %s/%s to %d replicas at time %v\n", chaosNamespace, scaleName, scale, startTime)
				return nil
			}

			getAndScaleStatefulSet := func(scaleName string, scale int32) error {
				stsObj, err := frame.GetStatefulSet(scaleName, chaosNamespace)
				if err != nil {
					return err
				}
				_, err = frame.ScaleStatefulSet(stsObj, scale)
				if err != nil {
					return fmt.Errorf("failed to scale %s/%s replicas to %d, error: %v", chaosNamespace, scaleName, scale, err)
				}
				return nil
			}

			getAndScaleDeployment := func(scaleName string, scale int32) error {
				deployObj, err := frame.GetDeployment(scaleName, chaosNamespace)
				if err != nil {
					return err
				}
				_, err = frame.ScaleDeployment(deployObj, scale)
				if err != nil {
					return fmt.Errorf("failed to scale %s/%s replicas to %d, error: %v", chaosNamespace, scaleName, scale, err)
				}
				return nil
			}

			getAndScaleKruiseStatefulSet := func(scaleName string, scale int32) error {
				kruiseStsObj, err := common.GetKruiseStatefulSet(frame, chaosNamespace, scaleName)
				if err != nil {
					return err
				}
				_, err = common.ScaleKruiseStatefulSet(frame, kruiseStsObj, scale)
				if err != nil {
					return fmt.Errorf("failed to scale %s/%s replicas to %d, error: %v", chaosNamespace, scaleName, scale, err)
				}
				return nil
			}

			scaleFuncs := map[string]func(scale int32) error{
				stsNameOne:       func(scale int32) error { return getAndScaleStatefulSet(stsNameOne, scale) },
				stsNameTwo:       func(scale int32) error { return getAndScaleStatefulSet(stsNameTwo, scale) },
				deployNameOne:    func(scale int32) error { return getAndScaleDeployment(deployNameOne, scale) },
				deployNameTwo:    func(scale int32) error { return getAndScaleDeployment(deployNameTwo, scale) },
				kruiseStsNameOne: func(scale int32) error { return getAndScaleKruiseStatefulSet(kruiseStsNameOne, scale) },
				kruiseStsNameTwo: func(scale int32) error { return getAndScaleKruiseStatefulSet(kruiseStsNameTwo, scale) },
			}

			scaleWg := sync.WaitGroup{}
			scaleWg.Add(len(scaleFuncs) + 1)
			// This goroutine randomly selects a Spiderpool Controller Pod and restarts it to trigger a failover.
			// The process involves a random delay of up to 30 seconds before selecting and deleting the Pod.
			go func() {
				defer GinkgoRecover()
				defer scaleWg.Done()
				time.Sleep(time.Duration(rand.Intn(30)) * time.Second)

				podList, err := frame.GetPodListByLabel(map[string]string{"app.kubernetes.io/component": constant.SpiderpoolController})
				Expect(err).NotTo(HaveOccurred())

				if len(podList.Items) <= 1 {
					Skip("Only one replica of Spiderpool Controller is present, skipping test.")
				}

				// Randomly select a Pod from the list for deletion
				randomPod := podList.Items[rand.Intn(len(podList.Items))]
				err = frame.DeletePod(randomPod.Name, randomPod.Namespace)
				Expect(err).NotTo(HaveOccurred())

				// Log the details of the selected Pod for debugging and verification purposes
				GinkgoWriter.Printf("Randomly selected and deleted Spiderpool Controller Pod: %s/%s to trigger a restart.\n", randomPod.Namespace, randomPod.Name)
			}()

			i := 0
			for name, scaleFunc := range scaleFuncs {
				currentIndex := i
				i++
				go func(name string, currentIndex int) {
					defer GinkgoRecover()
					defer scaleWg.Done()
					err := scalePodSet(scaleFunc, name, int32(randNumbers[currentIndex]), 30*time.Second)
					Expect(err).NotTo(HaveOccurred())
					GinkgoWriter.Printf("Succeed to scale %s/%s replicas to %d \n", chaosNamespace, name, randNumbers[currentIndex])
				}(name, currentIndex)
			}
			scaleWg.Wait()

			// 5. Check the status of pods and ippools after scaling
			// We obtained the GC interval, but the GC execution still requires time.
			// We will proceed with the next steps after waiting for the GC to complete.
			GinkgoWriter.Println("scale: starting to check the pod running status and the sanity of the IPs in the IPPool.")
			waitForPodsAndCheckPoolSanity()

			// 6. restart all Pods
			restartPodSet := func(restartFunc func() error, restartName string, duration time.Duration) error {
				time.Sleep(time.Duration(rand.Intn(int(duration.Seconds()))) * time.Second)

				strartTime := time.Now()
				err := restartFunc()
				if err != nil {
					return fmt.Errorf("failed to restart %s/%s at time %v, error %v", chaosNamespace, restartName, strartTime, err)
				}

				GinkgoWriter.Printf("Succeeded in restarting %s/%s at time %v \n", chaosNamespace, restartName, strartTime)
				return nil
			}

			restartFuncs := map[string]func() error{
				stsNameOne: func() error {
					return common.RestartAndValidateStatefulSetPodIP(frame, map[string]string{"app": stsNameOne})
				},
				stsNameTwo: func() error {
					return common.RestartAndValidateStatefulSetPodIP(frame, map[string]string{"app": stsNameTwo})
				},
				deployNameOne: func() error {
					return frame.DeletePodListByLabel(map[string]string{"app": deployNameOne})
				},
				deployNameTwo: func() error {
					return frame.DeletePodListByLabel(map[string]string{"app": deployNameTwo})
				},
				kruiseStsNameOne: func() error {
					return frame.DeletePodListByLabel(map[string]string{"app": kruiseStsNameOne})
				},
				kruiseStsNameTwo: func() error {
					return frame.DeletePodListByLabel(map[string]string{"app": kruiseStsNameTwo})
				},
			}

			restartWg := sync.WaitGroup{}
			restartWg.Add(len(restartFuncs))
			for name, restartFunc := range restartFuncs {
				go func(name string, restartFunc func() error) {
					defer GinkgoRecover()
					defer restartWg.Done()
					err := restartPodSet(restartFunc, name, 30*time.Second)
					Expect(err).NotTo(HaveOccurred())
					GinkgoWriter.Printf("Succeed to restart %s/%s \n", chaosNamespace, name)
				}(name, restartFunc)
			}
			restartWg.Wait()

			// 7. Check the status of the pod and ippool after restarting the pod
			// We obtained the GC interval, but the GC execution still requires time.
			// We will proceed with the next steps after waiting for the GC to complete.
			GinkgoWriter.Println("restart: starting to check the pod running status and the sanity of the IPs in the IPPool.")
			waitForPodsAndCheckPoolSanity()

			// Get all Pods in advance to check if ippool and endpoint are deleted
			podList, err := frame.GetPodList(client.InNamespace(chaosNamespace))
			Expect(err).NotTo(HaveOccurred(), "before deleting, failed to get Pod list %v", err)

			// 8. Randomly delete Pods to verify that IPPool and endpoints can be recycled
			deletePodSet := func(deleteFun func() error, deleteName string, duration time.Duration) error {
				time.Sleep(time.Duration(rand.Intn(int(duration.Seconds()))) * time.Second)

				startTime := time.Now()
				err := deleteFun()
				if err != nil {
					GinkgoWriter.Printf("Failed to delete %s/%s at time %v, error %s \n", chaosNamespace, deleteName, startTime, err)
					return err
				}

				GinkgoWriter.Printf("Succeeded in deleting %s/%s at time %v \n", chaosNamespace, deleteName, startTime)
				return nil
			}

			deleteFuncs := map[string]func() error{
				stsNameOne: func() error {
					return frame.DeleteStatefulSet(stsNameOne, chaosNamespace)
				},
				stsNameTwo: func() error {
					return frame.DeleteStatefulSet(stsNameTwo, chaosNamespace)
				},
				deployNameOne: func() error {
					return frame.DeleteDeployment(deployNameOne, chaosNamespace)
				},
				deployNameTwo: func() error {
					return frame.DeleteDeployment(deployNameTwo, chaosNamespace)
				},
				kruiseStsNameOne: func() error {
					return common.DeleteKruiseStatefulSetByName(frame, kruiseStsNameOne, chaosNamespace)
				},
				kruiseStsNameTwo: func() error {
					return common.DeleteKruiseStatefulSetByName(frame, kruiseStsNameTwo, chaosNamespace)
				},
			}

			var deleteWg sync.WaitGroup
			for name, deleteFunc := range deleteFuncs {
				deleteWg.Add(1)
				go func(name string, deleteFunc func() error) {
					defer GinkgoRecover()
					defer deleteWg.Done()
					err := deletePodSet(deleteFunc, name, 30*time.Second)
					Expect(err).NotTo(HaveOccurred())
				}(name, deleteFunc)
			}
			deleteWg.Wait()

			// Check that spiderpool and endpoint resources are recycled
			Eventually(func() error {
				defer GinkgoRecover()
				if frame.Info.IpV4Enabled {
					ippool, err := common.GetIppoolByName(frame, v4PoolName)
					if err != nil {
						if api_errors.IsNotFound(err) {
							return fmt.Errorf("v4 ippool %s dose not exist", v4PoolName)
						}
						return fmt.Errorf("failed to get v4 ippool %s, error %v", v4PoolName, err)
					}
					if ippool.Status.AllocatedIPs != nil || *ippool.Status.AllocatedIPCount != int64(0) {
						return fmt.Errorf("The IP address %v in the v4 ippool %v is not completely released", *ippool.Status.AllocatedIPs, v4PoolName)
					}
				}
				if frame.Info.IpV6Enabled {
					ippool, err := common.GetIppoolByName(frame, v6PoolName)
					if err != nil {
						if api_errors.IsNotFound(err) {
							return fmt.Errorf("ippool %s dose not exist", v4PoolName)
						}
						return fmt.Errorf("failed to get ippool %s, error %v", v6PoolName, err)
					}
					if ippool.Status.AllocatedIPs != nil || *ippool.Status.AllocatedIPCount != int64(0) {
						return fmt.Errorf("The IP address %v in the v6 ippool %v is not completely released", *ippool.Status.AllocatedIPs, v6PoolName)
					}
				}

				for _, pod := range podList.Items {
					_, err := common.GetWorkloadByName(frame, pod.Namespace, pod.Name)
					if err != nil {
						if api_errors.IsNotFound(err) {
							return nil
						}
						return fmt.Errorf("failed to get endpoint %s/%s, error %v", pod.Namespace, pod.Name, err)
					}
				}
				return nil
			}).WithTimeout(common.ResourceDeleteTimeout).WithPolling(time.Second * 10).Should(BeNil())
		})
	})
})
