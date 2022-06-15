// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	"errors"
	"github.com/asaskevich/govalidator"
	. "github.com/onsi/ginkgo/v2"
	// . "github.com/onsi/gomega"
	frame "github.com/spidernet-io/e2eframework/framework"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	corev1 "k8s.io/api/core/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetIppoolByName(f *frame.Framework, poolName string) *spiderpool.IPPool {
	if poolName == "" || f == nil {
		return nil
	}

	v := apitypes.NamespacedName{Name: poolName}
	existing := &spiderpool.IPPool{}
	e := f.GetResource(v, existing)
	if e != nil {
		return nil
	}
	return existing
}

func GetAllIppool(f *frame.Framework, opts ...client.ListOption) *spiderpool.IPPoolList {
	if f == nil {
		return nil
	}

	v := &spiderpool.IPPoolList{}
	e := f.ListResource(v, opts...)
	if e != nil {
		return nil
	}
	return v
}

func CheckIppoolForUsedIP(f *frame.Framework, ippool *spiderpool.IPPool, PodName, PodNamespace string, ipAddrress *corev1.PodIP) (bool, error) {
	if f == nil || ippool == nil || PodName == "" || PodNamespace == "" || ipAddrress.String() == "" {
		return false, errors.New("wrong input")
	}

	t, ok := ippool.Status.AllocatedIPs[ipAddrress.IP]
	if ok == false {
		return false, nil
	}
	if t.Pod != PodName || t.Namespace != PodNamespace {
		return false, nil
	}
	return true, nil
}

func GetPodIPv4Address(pod *corev1.Pod) *corev1.PodIP {
	for _, v := range pod.Status.PodIPs {
		if govalidator.IsIPv4(v.IP) {
			return &v
		}
	}
	return nil
}

func GetPodIPv6Address(pod *corev1.Pod) *corev1.PodIP {
	for _, v := range pod.Status.PodIPs {
		if govalidator.IsIPv6(v.IP) {
			return &v
		}
	}
	return nil
}

func CheckPodIpRecordInIppool(f *frame.Framework, v4IppoolName string, v6IppoolName string, podList *corev1.PodList) (bool, error) {
	if f == nil || podList == nil {
		return false, errors.New("wrong input")
	}

	var v4ippool, v6ippool *spiderpool.IPPool
	if len(v4IppoolName) != 0 {
		v4ippool = GetIppoolByName(f, v4IppoolName)
		if v4ippool == nil {
			GinkgoWriter.Printf("ippool %v not existed \n", v4IppoolName)
			return false, errors.New("ippool not existed")
		}
		GinkgoWriter.Printf("succeeded to get ippool %v \n", v4IppoolName)
	}

	if len(v6IppoolName) != 0 {
		v6ippool = GetIppoolByName(f, v6IppoolName)
		if v6ippool == nil {
			GinkgoWriter.Printf("ippool %v not existed \n", v6IppoolName)
			return false, errors.New("ippool not existed")
		}
		GinkgoWriter.Printf("succeeded to get ippool %v \n", v6IppoolName)
	}

	for _, v := range podList.Items {
		GinkgoWriter.Printf("checking ippool record for pod %v/%v \n", v.Namespace, v.Name)
		if f.Info.IpV4Enabled == true {
			ip := GetPodIPv4Address(&v)
			if ip == nil {
				return false, errors.New("failed to get pod ipv4 address")
			}
			ok, e := CheckIppoolForUsedIP(f, v4ippool, v.Name, v.Namespace, ip)
			if e != nil {
				return false, e
			}
			if !ok {
				GinkgoWriter.Printf("pod %v/%v : ip %v not recorded in ippool %v\n", v.Namespace, v.Name, ip.String(), v4IppoolName)
				GinkgoWriter.Printf("ippool status: %+v \n", v4ippool.Status.AllocatedIPs)
				return false, errors.New("ip not recorded in ippool")
			}
			GinkgoWriter.Printf("succeeded to check pod %v/%v with ip %v in ippool %v\n", v.Namespace, v.Name, ip.String(), v4IppoolName)
		}

		if f.Info.IpV6Enabled == true {
			ip := GetPodIPv6Address(&v)
			if ip == nil {
				return false, errors.New("failed to get pod ipv6 address")
			}
			ok, e := CheckIppoolForUsedIP(f, v6ippool, v.Name, v.Namespace, ip)
			if e != nil {
				return false, e
			}
			if !ok {
				GinkgoWriter.Printf("pod %v/%v : ip %v not recorded in ippool %v\n", v.Namespace, v.Name, ip.String(), v6IppoolName)
				GinkgoWriter.Printf("ippool status: %+v \n", v6ippool.Status.AllocatedIPs)
				return false, errors.New("ip not recorded in ippool")
			}
			GinkgoWriter.Printf("succeeded to check pod %v/%v with ip %v in ippool %v\n", v.Namespace, v.Name, ip.String(), v6IppoolName)

		}
	}
	return true, nil
}

func GetWorkloadByName(f *frame.Framework, namespace, name string) *spiderpool.WorkloadEndpoint {
	if name == "" || namespace == "" {
		return nil
	}

	v := apitypes.NamespacedName{Name: name, Namespace: namespace}
	existing := &spiderpool.WorkloadEndpoint{}
	e := f.GetResource(v, existing)
	if e != nil {
		return nil
	}
	return existing
}
