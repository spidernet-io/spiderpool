// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	"context"
	"github.com/asaskevich/govalidator"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	e2e "github.com/spidernet-io/e2eframework/framework"
	"github.com/spidernet-io/e2eframework/tools"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

type GeneratePodYamlFunc func(string, string) *corev1.Pod

func GenerateExamplePodYaml(podName, namespace string) *corev1.Pod {
	Expect(podName).NotTo(BeEmpty(), "podName is a empty string")
	Expect(namespace).NotTo(BeEmpty(), "namespace is a empty string")

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      podName,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "samplepod",
					Image:           "alpine",
					ImagePullPolicy: "IfNotPresent",
					Command:         []string{"/bin/ash", "-c", "trap : TERM INT; sleep infinity & wait"},
				},
			},
		},
	}
}

func GenerateLongPodYaml(podName, namespace string) *corev1.Pod {
	Expect(podName).NotTo(BeEmpty(), "podName is a empty string")
	Expect(namespace).NotTo(BeEmpty(), "namespace is a empty string")
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "samplepod",
					Image:           "alpine",
					ImagePullPolicy: "IfNotPresent",
					Command:         []string{"/bin/ash", "-c", "trap : TERM INT; sleep infinity & wait"},
				},
			},
			HostAliases: []corev1.HostAlias{
				{
					Hostnames: []string{
						"test-02.com",
						"test-02",
					},
					IP: "178.9.0.2",
				}, {
					Hostnames: []string{
						"test-03.com",
						"test-03",
					},
					IP: "178.9.0.3",
				}, {
					Hostnames: []string{
						"test-04.com",
						"test-04",
					},
					IP: "178.9.0.4",
				}, {
					Hostnames: []string{
						"test-05.com",
						"test-05",
					},
					IP: "178.9.0.5",
				}, {
					Hostnames: []string{
						"test-06.com",
						"test-06",
					},
					IP: "178.9.0.6",
				}, {
					Hostnames: []string{
						"test-07.com",
						"test-07",
					},
					IP: "178.9.0.7",
				}, {
					Hostnames: []string{
						"test-08.com",
						"test-08",
					},
					IP: "178.9.0.8",
				}, {
					Hostnames: []string{
						"test-09.com",
						"test-09",
					},
					IP: "178.9.0.9",
				}, {
					Hostnames: []string{
						"test-10.com",
						"test-10",
					},
					IP: "178.9.0.10",
				}, {
					Hostnames: []string{
						"test-11.com",
						"test-11",
					},
					IP: "178.9.0.11",
				}, {
					Hostnames: []string{
						"test-12.com",
						"test-12",
					},
					IP: "178.9.0.12",
				}, {
					Hostnames: []string{
						"test-13.com",
						"test-13",
					},
					IP: "178.9.0.13",
				}, {
					Hostnames: []string{
						"test-14.com",
						"test-14",
					},
					IP: "178.9.0.14",
				}, {
					Hostnames: []string{
						"test-15.com",
						"test-15",
					},
					IP: "178.9.0.15",
				}, {
					Hostnames: []string{
						"test-16.com",
						"test-16",
					},
					IP: "178.9.0.16",
				}, {
					Hostnames: []string{
						"test-17.com",
						"test-17",
					},
					IP: "178.9.0.17",
				}, {
					Hostnames: []string{
						"test-18.com",
						"test-18",
					},
					IP: "178.9.0.18",
				}, {
					Hostnames: []string{
						"test-19.com",
						"test-19",
					},
					IP: "178.9.0.19",
				}, {
					Hostnames: []string{
						"test-20.com",
						"test-20",
					},
					IP: "178.9.0.20",
				}, {
					Hostnames: []string{
						"test-21.com",
						"test-21",
					},
					IP: "178.9.0.21",
				}, {
					Hostnames: []string{
						"test-22.com",
						"test-22",
					},
					IP: "178.9.0.22",
				}, {
					Hostnames: []string{
						"test-23.com",
						"test-23",
					},
					IP: "178.9.0.23",
				}, {
					Hostnames: []string{
						"test-24.com",
						"test-24",
					},
					IP: "178.9.0.24",
				}, {
					Hostnames: []string{
						"test-25.com",
						"test-25",
					},
					IP: "178.9.0.25",
				}, {
					Hostnames: []string{
						"test-26.com",
						"test-26",
					},
					IP: "178.9.0.26",
				}, {
					Hostnames: []string{
						"test-27.com",
						"test-27",
					},
					IP: "178.9.0.27",
				}, {
					Hostnames: []string{
						"test-28.com",
						"test-28",
					},
					IP: "178.9.0.28",
				}, {
					Hostnames: []string{
						"test-29.com",
						"test-29",
					},
					IP: "178.9.0.29",
				}, {
					Hostnames: []string{
						"test-30.com",
						"test-30",
					},
					IP: "178.9.0.30",
				}, {
					Hostnames: []string{
						"test-31.com",
						"test-31",
					},
					IP: "178.9.0.31",
				}, {
					Hostnames: []string{
						"test-32.com",
						"test-32",
					},
					IP: "178.9.0.32",
				}, {
					Hostnames: []string{
						"test-33.com",
						"test-33",
					},
					IP: "178.9.0.33",
				}, {
					Hostnames: []string{
						"test-34.com",
						"test-34",
					},
					IP: "178.9.0.34",
				}, {
					Hostnames: []string{
						"test-35.com",
						"test-35",
					},
					IP: "178.9.0.35",
				}, {
					Hostnames: []string{
						"test-36.com",
						"test-36",
					},
					IP: "178.9.0.36",
				}, {
					Hostnames: []string{
						"test-37.com",
						"test-37",
					},
					IP: "178.9.0.37",
				}, {
					Hostnames: []string{
						"test-38.com",
						"test-38",
					},
					IP: "178.9.0.38",
				}, {
					Hostnames: []string{
						"test-39.com",
						"test-39",
					},
					IP: "178.9.0.39",
				}, {
					Hostnames: []string{
						"test-40.com",
						"test-40",
					},
					IP: "178.9.0.40",
				}, {
					Hostnames: []string{
						"test-41.com",
						"test-41",
					},
					IP: "178.9.0.41",
				}, {
					Hostnames: []string{
						"test-42.com",
						"test-42",
					},
					IP: "178.9.0.42",
				}, {
					Hostnames: []string{
						"test-43.com",
						"test-43",
					},
					IP: "178.9.0.43",
				}, {
					Hostnames: []string{
						"test-44.com",
						"test-44",
					},
					IP: "178.9.0.44",
				}, {
					Hostnames: []string{
						"test-45.com",
						"test-45",
					},
					IP: "178.9.0.45",
				}, {
					Hostnames: []string{
						"test-46.com",
						"test-46",
					},
					IP: "178.9.0.46",
				}, {
					Hostnames: []string{
						"test-47.com",
						"test-47",
					},
					IP: "178.9.0.47",
				}, {
					Hostnames: []string{
						"test-48.com",
						"test-48",
					},
					IP: "178.9.0.48",
				}, {
					Hostnames: []string{
						"test-49.com",
						"test-49",
					},
					IP: "178.9.0.49",
				}, {
					Hostnames: []string{
						"test-50.com",
						"test-50",
					},
					IP: "178.9.0.50",
				}, {
					Hostnames: []string{
						"test-51.com",
						"test-51",
					},
					IP: "178.9.0.51",
				}, {
					Hostnames: []string{
						"test-52.com",
						"test-52",
					},
					IP: "178.9.0.52",
				}, {
					Hostnames: []string{
						"test-53.com",
						"test-53",
					},
					IP: "178.9.0.53",
				}, {
					Hostnames: []string{
						"test-54.com",
						"test-54",
					},
					IP: "178.9.0.54",
				}, {
					Hostnames: []string{
						"test-55.com",
						"test-55",
					},
					IP: "178.9.0.55",
				}, {
					Hostnames: []string{
						"test-56.com",
						"test-56",
					},
					IP: "178.9.0.56",
				}, {
					Hostnames: []string{
						"test-57.com",
						"test-57",
					},
					IP: "178.9.0.57",
				}, {
					Hostnames: []string{
						"test-58.com",
						"test-58",
					},
					IP: "178.9.0.58",
				}, {
					Hostnames: []string{
						"test-59.com",
						"test-59",
					},
					IP: "178.9.0.59",
				}, {
					Hostnames: []string{
						"test-60.com",
						"test-60",
					},
					IP: "178.9.0.60",
				}, {
					Hostnames: []string{
						"test-61.com",
						"test-61",
					},
					IP: "178.9.0.61",
				}, {
					Hostnames: []string{
						"test-62.com",
						"test-62",
					},
					IP: "178.9.0.62",
				}, {
					Hostnames: []string{
						"test-63.com",
						"test-63",
					},
					IP: "178.9.0.63",
				}, {
					Hostnames: []string{
						"test-64.com",
						"test-64",
					},
					IP: "178.9.0.64",
				}, {
					Hostnames: []string{
						"test-65.com",
						"test-65",
					},
					IP: "178.9.0.65",
				}, {
					Hostnames: []string{
						"test-66.com",
						"test-66",
					},
					IP: "178.9.0.66",
				}, {
					Hostnames: []string{
						"test-67.com",
						"test-67",
					},
					IP: "178.9.0.67",
				}, {
					Hostnames: []string{
						"test-68.com",
						"test-68",
					},
					IP: "178.9.0.68",
				}, {
					Hostnames: []string{
						"test-69.com",
						"test-69",
					},
					IP: "178.9.0.69",
				}, {
					Hostnames: []string{
						"test-70.com",
						"test-70",
					},
					IP: "178.9.0.70",
				}, {
					Hostnames: []string{
						"test-71.com",
						"test-71",
					},
					IP: "178.9.0.71",
				}, {
					Hostnames: []string{
						"test-72.com",
						"test-72",
					},
					IP: "178.9.0.72",
				}, {
					Hostnames: []string{
						"test-73.com",
						"test-73",
					},
					IP: "178.9.0.73",
				}, {
					Hostnames: []string{
						"test-74.com",
						"test-74",
					},
					IP: "178.9.0.74",
				}, {
					Hostnames: []string{
						"test-75.com",
						"test-75",
					},
					IP: "178.9.0.75",
				}, {
					Hostnames: []string{
						"test-76.com",
						"test-76",
					},
					IP: "178.9.0.76",
				}, {
					Hostnames: []string{
						"test-77.com",
						"test-77",
					},
					IP: "178.9.0.77",
				}, {
					Hostnames: []string{
						"test-78.com",
						"test-78",
					},
					IP: "178.9.0.78",
				}, {
					Hostnames: []string{
						"test-79.com",
						"test-79",
					},
					IP: "178.9.0.79",
				}, {
					Hostnames: []string{
						"test-80.com",
						"test-80",
					},
					IP: "178.9.0.80",
				}, {
					Hostnames: []string{
						"test-81.com",
						"test-81",
					},
					IP: "178.9.0.81",
				}, {
					Hostnames: []string{
						"test-82.com",
						"test-82",
					},
					IP: "178.9.0.82",
				}, {
					Hostnames: []string{
						"test-83.com",
						"test-83",
					},
					IP: "178.9.0.83",
				}, {
					Hostnames: []string{
						"test-84.com",
						"test-84",
					},
					IP: "178.9.0.84",
				}, {
					Hostnames: []string{
						"test-85.com",
						"test-85",
					},
					IP: "178.9.0.85",
				}, {
					Hostnames: []string{
						"test-86.com",
						"test-86",
					},
					IP: "178.9.0.86",
				}, {
					Hostnames: []string{
						"test-87.com",
						"test-87",
					},
					IP: "178.9.0.87",
				}, {
					Hostnames: []string{
						"test-88.com",
						"test-88",
					},
					IP: "178.9.0.88",
				}, {
					Hostnames: []string{
						"test-89.com",
						"test-89",
					},
					IP: "178.9.0.89",
				}, {
					Hostnames: []string{
						"test-90.com",
						"test-90",
					},
					IP: "178.9.0.90",
				}, {
					Hostnames: []string{
						"test-91.com",
						"test-91",
					},
					IP: "178.9.0.91",
				}, {
					Hostnames: []string{
						"test-92.com",
						"test-92",
					},
					IP: "178.9.0.92",
				}, {
					Hostnames: []string{
						"test-93.com",
						"test-93",
					},
					IP: "178.9.0.93",
				}, {
					Hostnames: []string{
						"test-94.com",
						"test-94",
					},
					IP: "178.9.0.94",
				}, {
					Hostnames: []string{
						"test-95.com",
						"test-95",
					},
					IP: "178.9.0.95",
				}, {
					Hostnames: []string{
						"test-96.com",
						"test-96",
					},
					IP: "178.9.0.96",
				}, {
					Hostnames: []string{
						"test-97.com",
						"test-97",
					},
					IP: "178.9.0.97",
				}, {
					Hostnames: []string{
						"test-98.com",
						"test-98",
					},
					IP: "178.9.0.98",
				}, {
					Hostnames: []string{
						"test-99.com",
						"test-99",
					},
					IP: "178.9.0.99",
				}, {
					Hostnames: []string{
						"test-100.com",
						"test-100",
					},
					IP: "178.9.0.100",
				}, {
					Hostnames: []string{
						"test-101.com",
						"test-101",
					},
					IP: "178.9.0.101",
				}, {
					Hostnames: []string{
						"test-102.com",
						"test-102",
					},
					IP: "178.9.0.102",
				}, {
					Hostnames: []string{
						"test-103.com",
						"test-103",
					},
					IP: "178.9.0.103",
				}, {
					Hostnames: []string{
						"test-104.com",
						"test-104",
					},
					IP: "178.9.0.104",
				}, {
					Hostnames: []string{
						"test-105.com",
						"test-105",
					},
					IP: "178.9.0.105",
				}, {
					Hostnames: []string{
						"test-106.com",
						"test-106",
					},
					IP: "178.9.0.106",
				}, {
					Hostnames: []string{
						"test-107.com",
						"test-107",
					},
					IP: "178.9.0.107",
				}, {
					Hostnames: []string{
						"test-108.com",
						"test-108",
					},
					IP: "178.9.0.108",
				}, {
					Hostnames: []string{
						"test-109.com",
						"test-109",
					},
					IP: "178.9.0.109",
				}, {
					Hostnames: []string{
						"test-110.com",
						"test-110",
					},
					IP: "178.9.0.110",
				}, {
					Hostnames: []string{
						"test-111.com",
						"test-111",
					},
					IP: "178.9.0.111",
				}, {
					Hostnames: []string{
						"test-112.com",
						"test-112",
					},
					IP: "178.9.0.112",
				}, {
					Hostnames: []string{
						"test-113.com",
						"test-113",
					},
					IP: "178.9.0.113",
				}, {
					Hostnames: []string{
						"test-114.com",
						"test-114",
					},
					IP: "178.9.0.114",
				}, {
					Hostnames: []string{
						"test-115.com",
						"test-115",
					},
					IP: "178.9.0.115",
				}, {
					Hostnames: []string{
						"test-116.com",
						"test-116",
					},
					IP: "178.9.0.116",
				}, {
					Hostnames: []string{
						"test-117.com",
						"test-117",
					},
					IP: "178.9.0.117",
				}, {
					Hostnames: []string{
						"test-118.com",
						"test-118",
					},
					IP: "178.9.0.118",
				}, {
					Hostnames: []string{
						"test-119.com",
						"test-119",
					},
					IP: "178.9.0.119",
				}, {
					Hostnames: []string{
						"test-120.com",
						"test-120",
					},
					IP: "178.9.0.120",
				}, {
					Hostnames: []string{
						"test-121.com",
						"test-121",
					},
					IP: "178.9.0.121",
				}, {
					Hostnames: []string{
						"test-122.com",
						"test-122",
					},
					IP: "178.9.0.122",
				}, {
					Hostnames: []string{
						"test-123.com",
						"test-123",
					},
					IP: "178.9.0.123",
				}, {
					Hostnames: []string{
						"test-124.com",
						"test-124",
					},
					IP: "178.9.0.124",
				}, {
					Hostnames: []string{
						"test-125.com",
						"test-125",
					},
					IP: "178.9.0.125",
				}, {
					Hostnames: []string{
						"test-126.com",
						"test-126",
					},
					IP: "178.9.0.126",
				}, {
					Hostnames: []string{
						"test-127.com",
						"test-127",
					},
					IP: "178.9.0.127",
				}, {
					Hostnames: []string{
						"test-128.com",
						"test-128",
					},
					IP: "178.9.0.128",
				}, {
					Hostnames: []string{
						"test-129.com",
						"test-129",
					},
					IP: "178.9.0.129",
				}, {
					Hostnames: []string{
						"test-130.com",
						"test-130",
					},
					IP: "178.9.0.130",
				}, {
					Hostnames: []string{
						"test-131.com",
						"test-131",
					},
					IP: "178.9.0.131",
				}, {
					Hostnames: []string{
						"test-132.com",
						"test-132",
					},
					IP: "178.9.0.132",
				}, {
					Hostnames: []string{
						"test-133.com",
						"test-133",
					},
					IP: "178.9.0.133",
				}, {
					Hostnames: []string{
						"test-134.com",
						"test-134",
					},
					IP: "178.9.0.134",
				}, {
					Hostnames: []string{
						"test-135.com",
						"test-135",
					},
					IP: "178.9.0.135",
				}, {
					Hostnames: []string{
						"test-136.com",
						"test-136",
					},
					IP: "178.9.0.136",
				}, {
					Hostnames: []string{
						"test-137.com",
						"test-137",
					},
					IP: "178.9.0.137",
				}, {
					Hostnames: []string{
						"test-138.com",
						"test-138",
					},
					IP: "178.9.0.138",
				}, {
					Hostnames: []string{
						"test-139.com",
						"test-139",
					},
					IP: "178.9.0.139",
				}, {
					Hostnames: []string{
						"test-140.com",
						"test-140",
					},
					IP: "178.9.0.140",
				}, {
					Hostnames: []string{
						"test-141.com",
						"test-141",
					},
					IP: "178.9.0.141",
				}, {
					Hostnames: []string{
						"test-142.com",
						"test-142",
					},
					IP: "178.9.0.142",
				}, {
					Hostnames: []string{
						"test-143.com",
						"test-143",
					},
					IP: "178.9.0.143",
				}, {
					Hostnames: []string{
						"test-144.com",
						"test-144",
					},
					IP: "178.9.0.144",
				}, {
					Hostnames: []string{
						"test-145.com",
						"test-145",
					},
					IP: "178.9.0.145",
				}, {
					Hostnames: []string{
						"test-146.com",
						"test-146",
					},
					IP: "178.9.0.146",
				}, {
					Hostnames: []string{
						"test-147.com",
						"test-147",
					},
					IP: "178.9.0.147",
				}, {
					Hostnames: []string{
						"test-148.com",
						"test-148",
					},
					IP: "178.9.0.148",
				}, {
					Hostnames: []string{
						"test-149.com",
						"test-149",
					},
					IP: "178.9.0.149",
				}, {
					Hostnames: []string{
						"test-150.com",
						"test-150",
					},
					IP: "178.9.0.150",
				}, {
					Hostnames: []string{
						"test-151.com",
						"test-151",
					},
					IP: "178.9.0.151",
				},
			},
		},
	}
}

func CreatePod(frame *e2e.Framework, getYaml GeneratePodYamlFunc, podName, namespace string) (pod *corev1.Pod, podIPv4, podIPv6 string) {
	// create pod
	GinkgoWriter.Printf("try to create pod %v/%v \n", namespace, podName)
	Expect(podName).NotTo(BeEmpty(), "pod name is a empty string")
	Expect(namespace).NotTo(BeEmpty(), "namespace is a empty string")
	pod = getYaml(podName, namespace)

	err := frame.CreatePod(pod)
	Expect(err).NotTo(HaveOccurred(), "failed to create pod")

	// wait for pod ip
	GinkgoWriter.Println("wait for pod ready")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pod, err = frame.WaitPodStarted(podName, namespace, ctx)
	Expect(err).NotTo(HaveOccurred(), "time out to wait pod ready")
	Expect(pod).NotTo(BeNil(), "pod is nil")
	Expect(pod.Status.PodIPs).NotTo(BeEmpty(), "pod failed to assign ip")

	GinkgoWriter.Printf("pod: %v/%v, ips: %+v \n", namespace, podName, pod.Status.PodIPs)

	if frame.Info.IpV4Enabled {
		Expect(tools.CheckPodIpv4IPReady(pod)).To(BeTrue(), "pod failed to get ipv4 ip")
		GinkgoWriter.Println("succeeded to check pod ipv4 ip")
		// get ipv4
		GinkgoWriter.Println("get IPv4")
		podIPv4 = GetPodIPv4(pod)
		GinkgoWriter.Printf("pod IPv4: %+v \n", podIPv4)
		Expect(podIPv4).NotTo(BeEmpty(), "podIPv4 is a empty string")
	}
	if frame.Info.IpV6Enabled {
		Expect(tools.CheckPodIpv6IPReady(pod)).To(BeTrue(), "pod failed to get ipv6 ip")
		GinkgoWriter.Println("succeeded to check pod ipv6 ip")
		// get ipv6
		GinkgoWriter.Println("get IPv6")
		podIPv6 = GetPodIPv6(pod)
		GinkgoWriter.Printf("pod IPv6: %+v", podIPv6)
		Expect(podIPv6).NotTo(BeEmpty(), "podIPv6 is a empty string")
	}
	return
}

func GetPodIPv4(pod *corev1.Pod) string {
	podIPs := pod.Status.PodIPs
	for _, v := range podIPs {
		if govalidator.IsIPv4(v.IP) {
			return v.IP
		}
	}
	return ""
}

func GetPodIPv6(pod *corev1.Pod) string {
	podIPs := pod.Status.PodIPs
	for _, v := range podIPs {
		if govalidator.IsIPv6(v.IP) {
			return v.IP
		}
	}
	return ""
}
