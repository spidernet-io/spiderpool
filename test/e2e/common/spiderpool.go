// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/asaskevich/govalidator"
	. "github.com/onsi/ginkgo/v2"

	// . "github.com/onsi/gomega"
	frame "github.com/spidernet-io/e2eframework/framework"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/cmd/spiderpool-agent/cmd"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateIppool(f *frame.Framework, ippool *spiderpool.IPPool, opts ...client.CreateOption) error {
	if f == nil || ippool == nil {
		return errors.New("wrong input")
	}
	// try to wait for finish last deleting
	fake := &spiderpool.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: ippool.ObjectMeta.Name,
		},
	}
	key := client.ObjectKeyFromObject(fake)
	existing := &spiderpool.IPPool{}
	e := f.GetResource(key, existing)
	if e == nil && existing.ObjectMeta.DeletionTimestamp == nil {
		return errors.New("failed to create , a same Ippool exists")
	} else {
		t := func() bool {
			existing := &spiderpool.IPPool{}
			e := f.GetResource(key, existing)
			b := api_errors.IsNotFound(e)
			if !b {
				GinkgoWriter.Printf("waiting for a same Ippool %v to finish deleting \n", ippool.ObjectMeta.Name)
				return false
			}
			return true
		}
		if !tools.Eventually(t, f.Config.ResourceDeleteTimeout, time.Second) {
			return errors.New("failed to create , a same Ippool exists")
		}
	}
	return f.CreateResource(ippool, opts...)
}

func DeleteIPPoolByName(f *frame.Framework, poolName string, opts ...client.DeleteOption) error {
	if poolName == "" || f == nil {
		return errors.New("wrong input")
	}
	pool := &spiderpool.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: poolName,
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

func CheckPodIpRecordInIppool(f *frame.Framework, v4IppoolNameList, v6IppoolNameList []string, podList *corev1.PodList) (allIPRecorded, noneIPRecorded, partialIPRecorded bool, err error) {
	if f == nil || podList == nil {
		return false, false, false, errors.New("wrong input")
	}

	var v4ippoolList, v6ippoolList []*spiderpool.IPPool
	if len(v4IppoolNameList) != 0 {
		for _, v := range v4IppoolNameList {
			v4ippool := GetIppoolByName(f, v)
			if v4ippool == nil {
				GinkgoWriter.Printf("ippool %v not existed \n", v)
				return false, false, false, errors.New("ippool not existed")
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
				return false, false, false, errors.New("ippool not existed")
			}
			v6ippoolList = append(v6ippoolList, v6ippool)
		}
		GinkgoWriter.Printf("succeeded to get all ippool %v \n", v6IppoolNameList)
	}

	v4BingoNum := 0
	v6BingoNum := 0
	podNum := len(podList.Items)

	for _, v := range podList.Items {

		if f.Info.IpV4Enabled {
			GinkgoWriter.Printf("checking ippool %v , for pod %v/%v \n", v4IppoolNameList, v.Namespace, v.Name)
			ip := GetPodIPv4Address(&v)
			if ip == nil {
				return false, false, false, errors.New("failed to get pod ipv4 address")
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
				v4BingoNum++
				GinkgoWriter.Printf("pod %v/%v : ip %v recorded in ippool %v\n", v.Namespace, v.Name, ip.String(), m.Name)
				break
			}
			if !bingo {
				GinkgoWriter.Printf(" pod %v/%v  is not assigned v4 ip %v in ippool %v\n", v.Namespace, v.Name, ip.IP, v4IppoolNameList)
			} else {
				GinkgoWriter.Printf("succeeded to check pod %v/%v with ip %v in ippool %v\n", v.Namespace, v.Name, ip.IP, v4IppoolNameList)
			}
		}

		if f.Info.IpV6Enabled {
			GinkgoWriter.Printf("checking ippool %v , for pod %v/%v \n", v6IppoolNameList, v.Namespace, v.Name)
			ip := GetPodIPv6Address(&v)
			if ip == nil {
				return false, false, false, errors.New("failed to get pod ipv6 address")
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
				v6BingoNum++
				GinkgoWriter.Printf("pod %v/%v : ip %v recorded in ippool %v\n", v.Namespace, v.Name, ip.String(), m.Name)
				break
			}
			if !bingo {
				GinkgoWriter.Printf(" pod %v/%v  is not assigned v6 ip %v in ippool %v\n", v.Namespace, v.Name, ip.IP, v6IppoolNameList)
			} else {
				GinkgoWriter.Printf("succeeded to check pod %v/%v with ip %v in ippool %v\n", v.Namespace, v.Name, ip.String(), v6IppoolNameList)
			}
		}
	}

	if f.Info.IpV4Enabled && f.Info.IpV6Enabled {
		if v4BingoNum == podNum && v6BingoNum == podNum {
			return true, false, false, nil
		}
		if v4BingoNum == 0 && v6BingoNum == 0 {
			return false, true, false, nil
		}
		return false, false, true, nil
	} else if f.Info.IpV4Enabled {
		if v4BingoNum == podNum {
			return true, false, false, nil
		}
		if v4BingoNum == 0 {
			return false, true, false, nil
		}
		return false, false, true, nil
	} else {
		if v6BingoNum == podNum {
			return true, false, false, nil
		}
		if v6BingoNum == 0 {
			return false, true, false, nil
		}
		return false, false, true, nil
	}
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

func GetNamespaceDefaultIppool(f *frame.Framework, namespace string) (v4IppoolList, v6IppoolList []string, e error) {
	ns, err := f.GetNamespace(namespace)
	if err != nil {
		return nil, nil, err
	}
	GinkgoWriter.Printf("Get DefaultIppool for namespace: %v\n", ns)

	annoNSIPv4 := types.AnnoNSDefautlV4PoolValue{}
	annoNSIPv6 := types.AnnoNSDefautlV6PoolValue{}

	v4Data, v4OK := ns.Annotations[constant.AnnoNSDefautlV4Pool]
	v6Data, v6OK := ns.Annotations[constant.AnnoNSDefautlV6Pool]

	if v4OK && len(v4Data) > 0 {
		if err := json.Unmarshal([]byte(v4Data), &annoNSIPv4); err != nil {
			GinkgoWriter.Printf("fail to decode namespace annotation v4 value: %v\n", v4Data)
			return nil, nil, errors.New("invalid namespace annotation")
		}
		v4IppoolList = annoNSIPv4
	}
	if v6OK && len(v6Data) > 0 {
		if err := json.Unmarshal([]byte(v6Data), &annoNSIPv6); err != nil {
			GinkgoWriter.Printf("fail to decode namespace annotation v6 value: %v\n", v6Data)
			return nil, nil, errors.New("invalid namespace annotation")
		}
		v6IppoolList = annoNSIPv6
	}
	return v4IppoolList, v6IppoolList, nil
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

func WaitIPReclaimedFinish(f *frame.Framework, v4IppoolNameList, v6IppoolNameList []string, podList *corev1.PodList, timeOut time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeOut)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return errors.New("time out to wait ip reclaimed finish")
		default:
			_, ok, _, err := CheckPodIpRecordInIppool(f, v4IppoolNameList, v6IppoolNameList, podList)
			if err != nil {
				return err
			}
			if ok {
				return nil
			}
			time.Sleep(time.Second)
		}
	}
}

func GenerateExampleIpv4poolObject() (string, *spiderpool.IPPool) {

	var v4Ipversion = new(spiderpool.IPVersion)
	*v4Ipversion = spiderpool.IPv4

	var iPv4PoolObj *spiderpool.IPPool
	// Generate ipv4pool name
	var v4PoolName string = "v4pool-" + tools.RandomName()
	// Generate random number
	var randomNumber1 string = GenerateRandomNumber(255)
	var randomNumber2 string = GenerateRandomNumber(255)
	iPv4PoolObj = &spiderpool.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: v4PoolName,
		},
		Spec: spiderpool.IPPoolSpec{
			IPVersion: v4Ipversion,
			Subnet:    fmt.Sprintf("10.%s.%s.0/24", randomNumber1, randomNumber2),
			IPs:       []string{fmt.Sprintf("10.%s.%s.10-10.%s.%s.250", randomNumber1, randomNumber2, randomNumber1, randomNumber2)},
			ExcludeIPs: []string{fmt.Sprintf("10.%s.%s.2", randomNumber1, randomNumber2),
				fmt.Sprintf("10.%s.%s.254", randomNumber1, randomNumber2)},
		},
	}
	return v4PoolName, iPv4PoolObj
}

func GenerateExampleIpv6poolObject() (string, *spiderpool.IPPool) {

	var v6Ipversion = new(spiderpool.IPVersion)
	*v6Ipversion = spiderpool.IPv6

	// Generate ipv6pool name
	var v6PoolName string = "v6pool-" + tools.RandomName()
	// Generate random number
	var randomNumber string = GenerateRandomNumber(9999)

	iPv6PoolObj := &spiderpool.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: v6PoolName,
		},
		Spec: spiderpool.IPPoolSpec{
			IPVersion: v6Ipversion,
			Subnet:    fmt.Sprintf("fd00:%s::/120", randomNumber),
			IPs:       []string{fmt.Sprintf("fd00:%s::10-fd00:%s::250", randomNumber, randomNumber)},
			ExcludeIPs: []string{fmt.Sprintf("fd00:%s::2", randomNumber),
				fmt.Sprintf("fd00:%s::254", randomNumber)},
		},
	}
	return v6PoolName, iPv6PoolObj
}

func UpdateIppool(f *frame.Framework, ippool *spiderpool.IPPool, opts ...client.UpdateOption) error {
	if ippool == nil || f == nil {
		return errors.New("wrong input")
	}
	return f.UpdateResource(ippool, opts...)
}
