// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	e2e "github.com/spidernet-io/e2eframework/framework"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func GenerateExampleDeploymentYaml(dpmName, namespace string, replica int32) *appsv1.Deployment {
	Expect(dpmName).NotTo(BeEmpty())
	Expect(namespace).NotTo(BeEmpty())

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      dpmName,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(replica),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": dpmName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": dpmName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "samplepod",
							Image:           "alpine",
							ImagePullPolicy: "IfNotPresent",
							Command:         []string{"/bin/ash", "-c", "sleep infinity"},
						},
					},
				},
			},
		},
	}
}

func ScaleDeployUntilExpectedReplicas(frame *e2e.Framework, deploy *appsv1.Deployment, expectedReplicas int, ctx context.Context) (addedPod, removedPod []corev1.Pod, err error) {

	if frame == nil || deploy == nil || expectedReplicas <= 0 || int32(expectedReplicas) == *deploy.Spec.Replicas {
		return nil, nil, e2e.ErrWrongInput
	}

	var newPodList *corev1.PodList

	podList, err := frame.GetPodListByLabel(deploy.Spec.Selector.MatchLabels)
	Expect(err).NotTo(HaveOccurred())
	GinkgoWriter.Printf("deloyment %v/%v scale replicas from %v to %v \n", deploy.Namespace, deploy.Name, len(podList.Items), expectedReplicas)

	deploy, err = frame.ScaleDeployment(deploy, int32(expectedReplicas))
	Expect(err).NotTo(HaveOccurred())
	Expect(*deploy.Spec.Replicas).To(Equal(int32(expectedReplicas)))
	GinkgoWriter.Printf("Successful scale order to start waiting for expected replicas: %v \n", expectedReplicas)

	for {
		select {
		case <-ctx.Done():
			return nil, nil, fmt.Errorf("time out to wait expected replicas: %v ", expectedReplicas)
		default:
			newPodList, err = frame.GetPodListByLabel(deploy.Spec.Selector.MatchLabels)
			Expect(err).NotTo(HaveOccurred())
			if len(newPodList.Items) != expectedReplicas {
				break
			}
			// return the diff pod
			if expectedReplicas > len(podList.Items) {
				addedPod := GetAdditionalPods(podList, newPodList)
				return addedPod, nil, nil
			}
			if expectedReplicas < len(podList.Items) {
				removedPod := GetAdditionalPods(newPodList, podList)
				return nil, removedPod, nil
			}
		}
		time.Sleep(ForcedWaitingTime)
	}
}

func CreateDeployUntilExpectedReplicas(frame *e2e.Framework, deploy *appsv1.Deployment, ctx context.Context) (pods *corev1.PodList, err error) {

	if frame == nil || deploy == nil {
		return nil, e2e.ErrWrongInput
	}

	err = frame.CreateDeployment(deploy)
	Expect(err).NotTo(HaveOccurred())

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("time out to wait expected replicas: %v ", *deploy.Spec.Replicas)
		default:
			podList, err := frame.GetPodListByLabel(deploy.Spec.Selector.MatchLabels)
			Expect(err).NotTo(HaveOccurred())
			if int32(len(podList.Items)) != *deploy.Spec.Replicas {
				break
			}
			return podList, nil
		}
		time.Sleep(ForcedWaitingTime)
	}
}

// Create Deployment with types.AnnoPodIPPoolValue
func CreateDeployWithPodAnnoation(frame *e2e.Framework, name, namespace string, deployOriginialNum int, nic string, v4PoolNameList, v6PoolNameList []string) (deploy *appsv1.Deployment) {
	Expect(name).NotTo(BeEmpty(), "name is empty \n")
	Expect(namespace).NotTo(BeEmpty(), "namespace is empty \n")

	annoPodIPPoolValueStr := GeneratePodIPPoolAnnotations(frame, nic, v4PoolNameList, v6PoolNameList)

	deployYaml := GenerateExampleDeploymentYaml(name, namespace, int32(deployOriginialNum))
	deployYaml.Spec.Template.Annotations = map[string]string{constant.AnnoPodIPPool: annoPodIPPoolValueStr}
	Expect(deployYaml).NotTo(BeNil())

	deploy, err := frame.CreateDeploymentUntilReady(deployYaml, PodStartTimeout)
	Expect(err).NotTo(HaveOccurred())
	return deploy
}

// Create Deployment until the ip assignment is successful
func CreateDeployUnitlReadyCheckInIppool(frame *e2e.Framework, depName, namespaceName string, podNum int32, v4PoolNameList, v6PoolNameList []string) {
	deployYaml := GenerateExampleDeploymentYaml(depName, namespaceName, podNum)
	Expect(deployYaml).NotTo(BeNil())
	deploy, err := frame.CreateDeploymentUntilReady(deployYaml, PodStartTimeout)
	Expect(err).NotTo(HaveOccurred())

	// get pod list
	podlist, err := frame.GetDeploymentPodList(deploy)
	Expect(int32(len(podlist.Items))).Should(Equal(deploy.Status.ReadyReplicas))
	Expect(err).NotTo(HaveOccurred())

	// check pod ip record still in this ippool
	ok, _, _, err := CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podlist)
	Expect(err).NotTo(HaveOccurred())
	Expect(ok).To(BeTrue())
}
