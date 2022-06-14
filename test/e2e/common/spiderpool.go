// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	"errors"
	"fmt"
	"time"

	"github.com/asaskevich/govalidator"
	. "github.com/onsi/ginkgo/v2"

	// . "github.com/onsi/gomega"
	frame "github.com/spidernet-io/e2eframework/framework"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/cmd/spiderpool-agent/cmd"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GenerateExampleIppoolYaml(Name string) *spiderpool.IPPool {
	var disable bool = false
	var IPVersion spiderpool.IPVersion = "IPv4"
	return &spiderpool.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: Name,
		},
		Spec: spiderpool.IPPoolSpec{
			IPVersion: &IPVersion,
			// IPs:       []string{"172.21.40.2-172.21.40.254"},
			// Subnet:    "172.21.0.0/16",
			Disable: &disable,
		},
	}
}

func CreateIppool(f *frame.Framework, pool *spiderpool.IPPool, opts ...client.CreateOption) error {
	// try to wait for finish last deleting
	fake := &spiderpool.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: pool.ObjectMeta.Name,
		},
	}

	key := client.ObjectKeyFromObject(fake)
	existing := &spiderpool.IPPool{}
	e := f.GetResource(key, existing)
	if e == nil && existing.ObjectMeta.DeletionTimestamp == nil {
		return fmt.Errorf("failed to create , a same IPPool %v exists", pool.ObjectMeta.Name)
	} else {
		t := func() bool {
			existing := &spiderpool.IPPool{}
			e := f.GetResource(key, existing)
			b := api_errors.IsNotFound(e)
			if !b {
				f.Log("waiting for a same IPPool %v to finish deleting \n", pool.ObjectMeta.Name)
				return false
			}
			return true
		}
		if !tools.Eventually(t, f.Config.ResourceDeleteTimeout, time.Second) {
			return fmt.Errorf("time out")
		}
	}
	return f.CreateResource(pool, opts...)
}

func DeleteIppool(f *frame.Framework, name string, opts ...client.DeleteOption) error {

	pool := &spiderpool.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	return f.DeleteResource(pool, opts...)
}

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
	if !ok {
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

func CheckPodIpRecordInIppool(f *frame.Framework, v4IppoolNameList, v6IppoolNameList []string, podList *corev1.PodList) (bool, error) {
	if f == nil || podList == nil {
		return false, errors.New("wrong input")
	}

	var v4ippoolList, v6ippoolList []*spiderpool.IPPool
	if len(v4IppoolNameList) != 0 {
		for _, v := range v4IppoolNameList {
			v4ippool := GetIppoolByName(f, v)
			if v4ippool == nil {
				GinkgoWriter.Printf("ippool %v not existed \n", v)
				return false, errors.New("ippool not existed")
			}
			v4ippoolList = append(v4ippoolList, v4ippool)
		}
		GinkgoWriter.Printf("succeeded to get all ippool %v \n", v4IppoolNameList)
	}

	if len(v6IppoolNameList) != 0 {
		for _, v := range v6IppoolNameList {
			v6ippool := GetIppoolByName(f, v)
			if v6ippool == nil {
				GinkgoWriter.Printf("ippool %v not existed \n", v)
				return false, errors.New("ippool not existed")
			}
			v6ippoolList = append(v6ippoolList, v6ippool)
		}
		GinkgoWriter.Printf("succeeded to get all ippool %v \n", v6IppoolNameList)
	}

	for _, v := range podList.Items {

		if f.Info.IpV4Enabled {
			GinkgoWriter.Printf("checking ippool %v , for pod %v/%v \n", v4IppoolNameList, v.Namespace, v.Name)
			ip := GetPodIPv4Address(&v)
			if ip == nil {
				return false, errors.New("failed to get pod ipv4 address")
			}
			bingo := false
			for _, m := range v4ippoolList {
				ok, e := CheckIppoolForUsedIP(f, m, v.Name, v.Namespace, ip)
				if e != nil || !ok {
					GinkgoWriter.Printf("pod %v/%v : ip %v not recorded in ippool %v\n", v.Namespace, v.Name, ip.IP, m.Name)
					GinkgoWriter.Printf("ippool %v status : %v \n", v.Name, m.Status.AllocatedIPs)
					continue
				}
				bingo = true
				GinkgoWriter.Printf("pod %v/%v : ip %v recorded in ippool %v\n", v.Namespace, v.Name, ip.String(), m.Name)
				break
			}
			if !bingo {
				return false, nil
			}
			GinkgoWriter.Printf("succeeded to check pod %v/%v with ip %v in ippool %v\n", v.Namespace, v.Name, ip.IP, v4IppoolNameList)
		}

		if f.Info.IpV6Enabled {
			GinkgoWriter.Printf("checking ippool %v , for pod %v/%v \n", v6IppoolNameList, v.Namespace, v.Name)
			ip := GetPodIPv6Address(&v)
			if ip == nil {
				return false, errors.New("failed to get pod ipv6 address")
			}
			bingo := false
			for _, m := range v6ippoolList {
				ok, e := CheckIppoolForUsedIP(f, m, v.Name, v.Namespace, ip)
				if e != nil || !ok {
					GinkgoWriter.Printf("pod %v/%v : ip %v not recorded in ippool %v\n", v.Namespace, v.Name, ip.String(), m.Name)
					GinkgoWriter.Printf("ippool %v status : %v \n", v.Name, m.Status.AllocatedIPs)
					continue
				}
				bingo = true
				GinkgoWriter.Printf("pod %v/%v : ip %v recorded in ippool %v\n", v.Namespace, v.Name, ip.String(), m.Name)
				break
			}
			if !bingo {
				return false, nil
			}
			GinkgoWriter.Printf("succeeded to check pod %v/%v with ip %v in ippool %v\n", v.Namespace, v.Name, ip.String(), v6IppoolNameList)
		}
	}
	GinkgoWriter.Printf("succeeded to check ippool for all ip of pods \n")
	return true, nil
}

func GetClusterDefaultIppool(f *frame.Framework) (v4IppoolList, v6IppoolList []string, e error) {
	if f == nil {
		return nil, nil, errors.New("wrong input")
	}

	t, e := f.GetConfigmap(SpiderPoolConfigmapName, SpiderPoolConfigmapNameSpace)
	if e != nil {
		return nil, nil, e
	}
	GinkgoWriter.Printf("configmap: %+v \n", t.Data)

	data, ok := t.Data["conf.yml"]
	if !ok || len(data) == 0 {
		return nil, nil, errors.New("failed to find cluster default ippool")
	}

	d := cmd.Config{}
	if err := yaml.Unmarshal([]byte(data), &d); nil != err {
		GinkgoWriter.Printf("failed to decode yaml config: %v \n", data)
		return nil, nil, errors.New("failed to find cluster default ippool")
	}
	GinkgoWriter.Printf("yaml config: %v \n", d)

	return d.ClusterDefaultIPv4IPPool, d.ClusterDefaultIPv6IPPool, nil
}

func GetNamespaceDefaultIppool(f *frame.Framework) (v4IppoolList, v6IppoolList []string, e error) {
	// TODO (binzeSun)
	return nil, nil, nil
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
