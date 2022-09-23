// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	v1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"

	"github.com/asaskevich/govalidator"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	frame "github.com/spidernet-io/e2eframework/framework"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/cmd/spiderpool-agent/cmd"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	ip "github.com/spidernet-io/spiderpool/pkg/ip"
	"github.com/spidernet-io/spiderpool/pkg/types"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateIppool(f *frame.Framework, ippool *v1.SpiderIPPool, opts ...client.CreateOption) error {
	if f == nil || ippool == nil {
		return errors.New("wrong input")
	}
	// try to wait for finish last deleting
	fake := &v1.SpiderIPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: ippool.ObjectMeta.Name,
		},
	}
	key := client.ObjectKeyFromObject(fake)
	existing := &v1.SpiderIPPool{}
	e := f.GetResource(key, existing)
	if e == nil && existing.ObjectMeta.DeletionTimestamp == nil {
		return errors.New("failed to create , a same Ippool exists")
	} else {
		t := func() bool {
			existing := &v1.SpiderIPPool{}
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

func BatchCreateIppoolWithSpecifiedIPNumber(frame *frame.Framework, ippoolNumber, ipNum int, isV4orv6Pool bool) (ipPoolNameList []string, err error) {
	if frame == nil || ippoolNumber < 0 || ipNum < 0 {
		return nil, errors.New("wrong input")
	}

	var ipPoolName string
	var ipPoolObj *v1.SpiderIPPool
	var iPPoolNameList []string
	ipMap := make(map[string]string)

OUTER_FOR:
	// cycle create ippool
	for i := 1; i <= ippoolNumber; i++ {
		if isV4orv6Pool {
			ipPoolName, ipPoolObj = GenerateExampleIpv4poolObject(ipNum)
		} else {
			ipPoolName, ipPoolObj = GenerateExampleIpv6poolObject(ipNum)
		}
		Expect(ipPoolObj.Spec.IPs).NotTo(BeNil())
		GinkgoWriter.Printf("ipPoolObj.Spec.IPs : %v\n", ipPoolObj.Spec.IPs)
		GinkgoWriter.Printf("ipPoolObj.Spec.IPVersion : %v\n", *ipPoolObj.Spec.IPVersion)

		ipslice, err := ip.ParseIPRanges(*ipPoolObj.Spec.IPVersion, ipPoolObj.Spec.IPs)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("ip segment is : %v\n", ipslice)
		tempIPs := []string{}
		// traversal of ips in ip segment
		for _, ips := range ipslice {
			// check whether the ip exists in ipMap
			if d, isPresent := ipMap[string(ips)]; isPresent {
				GinkgoWriter.Printf("ippool objects %v and %v have conflicted ip: %v \n", d, ipPoolName, ips)
				i--
				// If there is duplication in the middle, delete the dirty data
				for _, ip := range tempIPs {
					delete(ipMap, ip)
				}
				// continue back to OUTER_FOR to continue creating ippool
				continue OUTER_FOR
			}
			tempIPs = append(tempIPs, string(ips))
			ipMap[string(ips)] = ipPoolName
		}
		errpool := CreateIppool(frame, ipPoolObj)
		// if the created ippool is not nil ,then return err
		if errpool != nil {
			return nil, errpool
		}
		GinkgoWriter.Printf("%v-th ippool %v successfully created \n", i, ipPoolName)
		iPPoolNameList = append(iPPoolNameList, ipPoolName)
	}
	GinkgoWriter.Printf("iPPool List name is: %v \n", iPPoolNameList)
	return iPPoolNameList, nil
}

func DeleteIPPoolByName(f *frame.Framework, poolName string, opts ...client.DeleteOption) error {
	if poolName == "" || f == nil {
		return errors.New("wrong input")
	}
	pool := &v1.SpiderIPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: poolName,
		},
	}
	return f.DeleteResource(pool, opts...)
}

func GetIppoolByName(f *frame.Framework, poolName string) *v1.SpiderIPPool {
	if poolName == "" || f == nil {
		return nil
	}

	v := apitypes.NamespacedName{Name: poolName}
	existing := &v1.SpiderIPPool{}
	e := f.GetResource(v, existing)
	if e != nil {
		return nil
	}
	return existing
}

func GetAllIppool(f *frame.Framework, opts ...client.ListOption) *v1.SpiderIPPoolList {
	if f == nil {
		return nil
	}

	v := &v1.SpiderIPPoolList{}
	e := f.ListResource(v, opts...)
	if e != nil {
		return nil
	}
	return v
}

func CheckIppoolForUsedIP(f *frame.Framework, ippool *v1.SpiderIPPool, PodName, PodNamespace string, ipAddrress *corev1.PodIP) (bool, error) {
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
	if f == nil {
		return false, false, false, errors.New("wrong input: framework is nil")
	}
	if len(v4IppoolNameList) == 0 && len(v6IppoolNameList) == 0 {
		return false, false, false, fmt.Errorf("wrong input, v4IppoolNameList: '%v', v6IppoolNameList: '%v'", v4IppoolNameList, v6IppoolNameList)
	}
	if podList == nil || len(podList.Items) == 0 {
		return false, false, false, errors.New("wrong input: PodList in empty")
	}
	f.Log("cluster IPv4Enabled: '%v', IPv6Enabled: '%v'", f.Info.IpV4Enabled, f.Info.IpV6Enabled)

	var v4ippoolList, v6ippoolList []*v1.SpiderIPPool
	if len(v4IppoolNameList) != 0 {
		for _, v := range v4IppoolNameList {
			v4ippool := GetIppoolByName(f, v)
			if v4ippool == nil {
				f.Log("v4 pool %v not existed \n", v)
				return false, false, false, errors.New("v4 pool not existed")
			}
			v4ippoolList = append(v4ippoolList, v4ippool)
		}
		f.Log("succeeded to get all v4 pool %v \n", v4IppoolNameList)
	}

	if len(v6IppoolNameList) != 0 {
		for _, v := range v6IppoolNameList {
			v6ippool := GetIppoolByName(f, v)
			if v6ippool == nil {
				f.Log("v6 pool %v not existed \n", v)
				return false, false, false, errors.New("v6 pool not existed")
			}
			v6ippoolList = append(v6ippoolList, v6ippool)
		}
		f.Log("succeeded to get all v6 pool %v \n", v6IppoolNameList)
	}

	v4BingoNum := 0
	v6BingoNum := 0
	podNum := len(podList.Items)
	for _, v := range podList.Items {

		if f.Info.IpV4Enabled {
			bingo := false

			for _, m := range v4ippoolList {
				ok, e := CheckIppoolForPodName(f, m, v.Name, v.Namespace)
				if e != nil || !ok {
					f.Log("pod %v/%v not recorded in v4 pool %v \n", v.Namespace, v.Name, m.Name)
					f.Log("v4 pool %v status : %v \n", v.Name, m.Status.AllocatedIPs)
					continue
				}
				bingo = true
				v4BingoNum++
				f.Log("pod %v/%v recorded in v4 pool %v \n", v.Namespace, v.Name, m.Name)
				break
			}
			if !bingo {
				f.Log("pod %v/%v  is not assigned v4 ip in v4 pool %v \n", v.Namespace, v.Name, v4IppoolNameList)
			} else {
				f.Log("succeeded to check pod %v/%v in v4 pool %v \n", v.Namespace, v.Name, v4IppoolNameList)
			}
		}

		if f.Info.IpV6Enabled {
			bingo := false

			for _, m := range v6ippoolList {
				ok, e := CheckIppoolForPodName(f, m, v.Name, v.Namespace)
				if e != nil || !ok {
					f.Log("pod %v/%v not recorded in v6 pool %v \n", v.Namespace, v.Name, m.Name)
					f.Log("v6 pool %v status : %v \n", v.Name, m.Status.AllocatedIPs)
					continue
				}
				bingo = true
				v6BingoNum++
				f.Log("pod %v/%v recorded in v6 pool %v \n", v.Namespace, v.Name, m.Name)
				break
			}
			if !bingo {
				f.Log("pod %v/%v is not assigned v6 ip in v6 pool %v \n", v.Namespace, v.Name, v6IppoolNameList)
			} else {
				f.Log("succeeded to check pod %v/%v in v6 pool %v \n", v.Namespace, v.Name, v6IppoolNameList)
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

	configMap, e := f.GetConfigmap(SpiderPoolConfigmapName, SpiderPoolConfigmapNameSpace)
	if e != nil {
		return nil, nil, e
	}
	GinkgoWriter.Printf("configmap: %+v \n", configMap.Data)

	data, ok := configMap.Data["conf.yml"]
	if !ok || len(data) == 0 {
		return nil, nil, errors.New("failed to find cluster default ippool")
	}

	conf := cmd.Config{}
	if err := yaml.Unmarshal([]byte(data), &conf); nil != err {
		GinkgoWriter.Printf("failed to decode yaml config: %v \n", data)
		return nil, nil, errors.New("failed to find cluster default ippool")
	}
	GinkgoWriter.Printf("yaml config: %v \n", conf)

	if conf.EnableIPv4 && len(conf.ClusterDefaultIPv4IPPool) == 0 {
		return nil, nil, fmt.Errorf("IPv4 pool is not specified when IPv4 is enabled: %w", constant.ErrWrongInput)
	}
	if !conf.EnableIPv4 && len(conf.ClusterDefaultIPv4IPPool) != 0 {
		return nil, nil, fmt.Errorf("IPv4 pool is specified when IPv4 is disabled: %w", constant.ErrWrongInput)
	}
	if conf.EnableIPv6 && len(conf.ClusterDefaultIPv6IPPool) == 0 {
		return nil, nil, fmt.Errorf("IPv6 pool is not specified when IPv6 is enabled: %w", constant.ErrWrongInput)
	}
	if !conf.EnableIPv6 && len(conf.ClusterDefaultIPv6IPPool) != 0 {
		return nil, nil, fmt.Errorf("IPv6 pool is specified when IPv6 is disabled: %w", constant.ErrWrongInput)
	}

	return conf.ClusterDefaultIPv4IPPool, conf.ClusterDefaultIPv6IPPool, nil
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

func GetWorkloadByName(f *frame.Framework, namespace, name string) (*v1.SpiderEndpoint, error) {
	if name == "" || namespace == "" {
		return nil, frame.ErrWrongInput
	}

	v := apitypes.NamespacedName{Name: name, Namespace: namespace}
	existing := &v1.SpiderEndpoint{}
	e := f.GetResource(v, existing)
	if e != nil {
		return nil, errors.New("workload not existed")
	}
	return existing, nil
}

func CheckIppoolForPodName(f *frame.Framework, ippool *v1.SpiderIPPool, podName, podNamespace string) (bool, error) {
	if f == nil || ippool == nil || podName == "" || podNamespace == "" {
		return false, errors.New("wrong input")
	}

	for _, v := range ippool.Status.AllocatedIPs {
		if v.Pod == podName && v.Namespace == podNamespace {
			return true, nil
		}
	}
	return false, errors.New("pod name is not recorded in ippool")
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
			time.Sleep(ForcedWaitingTime)
		}
	}

}

func GenerateExampleIpv4poolObject(ipNum int) (string, *v1.SpiderIPPool) {
	if ipNum < 1 || ipNum > 65533 {
		GinkgoWriter.Println("the IP range should be between 1 and 65533")
		Fail("the IP range should be between 1 and 65533")
	}
	var v4Ipversion = new(types.IPVersion)
	*v4Ipversion = constant.IPv4

	var iPv4PoolObj *v1.SpiderIPPool
	// Generate ipv4pool name
	var v4PoolName string = "v4pool-" + tools.RandomName()
	// Generate random number
	var randomNumber1 string = GenerateRandomNumber(255)
	var randomNumber2 string = GenerateRandomNumber(255)

	iPv4PoolObj = &v1.SpiderIPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: v4PoolName,
		},
		Spec: v1.IPPoolSpec{
			IPVersion: v4Ipversion,
		},
	}

	if ipNum <= 253 {
		iPv4PoolObj.Spec.Subnet = fmt.Sprintf("10.%s.%s.0/24", randomNumber1, randomNumber2)
		if ipNum == 1 {
			iPv4PoolObj.Spec.IPs = []string{fmt.Sprintf("10.%s.%s.2", randomNumber1, randomNumber2)}
		} else {
			a := strconv.Itoa(ipNum + 1)
			iPv4PoolObj.Spec.IPs = []string{fmt.Sprintf("10.%s.%s.2-10.%s.%s.%s", randomNumber1, randomNumber2, randomNumber1, randomNumber2, a)}
		}
	} else {
		iPv4PoolObj.Spec.Subnet = fmt.Sprintf("10.%s.0.0/16", randomNumber1)
		a := fmt.Sprintf("%.0f", float64((ipNum+1)/256))
		b := strconv.Itoa((ipNum + 1) % 256)
		iPv4PoolObj.Spec.IPs = []string{fmt.Sprintf("10.%s.0.2-10.%s.%s.%s", randomNumber1, randomNumber1, a, b)}
	}
	return v4PoolName, iPv4PoolObj
}

func GenerateExampleIpv6poolObject(ipNum int) (string, *v1.SpiderIPPool) {
	if ipNum < 1 || ipNum > 65533 {
		GinkgoWriter.Println("the IP range should be between 1 and 65533")
		Fail("the IP range should be between 1 and 65533")
	}

	var v6Ipversion = new(types.IPVersion)
	*v6Ipversion = constant.IPv6

	// Generate ipv6pool name
	var v6PoolName string = "v6pool-" + tools.RandomName()
	// Generate random number
	var randomNumber string = GenerateRandomNumber(9999)

	iPv6PoolObj := &v1.SpiderIPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: v6PoolName,
		},
		Spec: v1.IPPoolSpec{
			IPVersion: v6Ipversion,
		},
	}

	if ipNum <= 253 {
		iPv6PoolObj.Spec.Subnet = fmt.Sprintf("fd00:%s::/120", randomNumber)
	} else {
		iPv6PoolObj.Spec.Subnet = fmt.Sprintf("fd00:%s::/112", randomNumber)
	}

	if ipNum == 1 {
		iPv6PoolObj.Spec.IPs = []string{fmt.Sprintf("fd00:%s::2", randomNumber)}
	} else {
		bStr := fmt.Sprintf("%x", ipNum+1)
		iPv6PoolObj.Spec.IPs = []string{fmt.Sprintf("fd00:%s::2-fd00:%s::%s", randomNumber, randomNumber, bStr)}
	}
	return v6PoolName, iPv6PoolObj
}

func DeleteIPPoolUntilFinish(f *frame.Framework, poolName string, ctx context.Context, opts ...client.DeleteOption) error {
	if poolName == "" {
		return frame.ErrWrongInput
	}
	err := DeleteIPPoolByName(f, poolName, opts...)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return frame.ErrTimeOut
		default:
			pool := GetIppoolByName(f, poolName)
			if pool == nil {
				return nil
			}
			time.Sleep(ForcedWaitingTime)
		}
	}
}

func UpdateIppool(f *frame.Framework, ippool *v1.SpiderIPPool, opts ...client.UpdateOption) error {
	if ippool == nil || f == nil {
		return errors.New("wrong input")
	}
	return f.UpdateResource(ippool, opts...)
}

func PatchIppool(f *frame.Framework, desiredPool, originalPool *v1.SpiderIPPool, opts ...client.PatchOption) error {
	if desiredPool == nil || f == nil || originalPool == nil {
		return errors.New("wrong input")
	}
	mergePatch := client.MergeFrom(originalPool)
	return f.PatchResource(desiredPool, mergePatch, opts...)
}

func BatchDeletePoolUntilFinish(f *frame.Framework, iPPoolNameList []string, ctx context.Context) error {
	if f == nil || iPPoolNameList == nil {
		return frame.ErrWrongInput
	}
	for _, iPPool := range iPPoolNameList {
		err := DeleteIPPoolUntilFinish(f, iPPool, ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func GenerateExampleIpv4Gateway() (v4Gateway string) {
	num1 := GenerateRandomNumber(255)
	num2 := GenerateRandomNumber(255)
	num3 := GenerateRandomNumber(255)
	num4 := GenerateRandomNumber(255)
	v4Gateway = fmt.Sprintf("%s.%s.%s.%s", num1, num2, num3, num4)
	return v4Gateway
}

func GenerateExampleIpv6Gateway() (v6Gateway string) {
	num1 := GenerateRandomNumber(9999)
	num2 := GenerateRandomNumber(9999)
	num3 := GenerateRandomNumber(9999)
	num4 := GenerateRandomNumber(9999)
	v6Gateway = fmt.Sprintf("%s:%s:%s::%s", num1, num2, num3, num4)
	return v6Gateway
}

func GenerateExampleIpv4Address() (ipv4Address string) {
	randomNum1 := GenerateRandomNumber(255)
	randomNum2 := GenerateRandomNumber(255)
	randomNum3 := GenerateRandomNumber(255)
	randomNum4 := GenerateRandomNumber(255)
	ipv4Address = fmt.Sprintf("%s.%s.%s.%s", randomNum1, randomNum2, randomNum3, randomNum4)
	return ipv4Address
}

func GenerateExampleIpv6Address() (ipv6Address string) {
	randomNum1 := GenerateRandomNumber(9999)
	randomNum2 := GenerateRandomNumber(9999)
	randomNum3 := GenerateRandomNumber(9999)
	ipv6Address = fmt.Sprintf("%s:%s::%s", randomNum1, randomNum2, randomNum3)
	return ipv6Address
}

// Waiting for Ippool Status Condition By Allocated IPs meets expectations
// can be used to detect dirty IPs recorded in ippool to be reclaimed automatically
func WaitIppoolStatusConditionByAllocatedIPs(ctx context.Context, f *frame.Framework, poolName, checkIPs string, isRecord bool) error {
	for {
		select {
		case <-ctx.Done():
			return frame.ErrTimeOut
		default:
			poolObj := GetIppoolByName(f, poolName)
			_, ok := poolObj.Status.AllocatedIPs[checkIPs]
			if isRecord && ok {
				GinkgoWriter.Printf("the IP %v recorded in IPPool %v \n", checkIPs, poolName)
				return nil
			}
			if !isRecord && !ok {
				GinkgoWriter.Printf("the IP %v reclaimed from IPPool %v \n", checkIPs, poolName)
				return nil
			}
			time.Sleep(ForcedWaitingTime)
		}
	}
}

// When the Pod IP resource is reclaimed, wait for the corresponding workload deletion to complete
func WaitWorkloadDeleteUntilFinish(ctx context.Context, f *frame.Framework, namespace, name string) error {
	if f == nil || namespace == "" || name == "" {
		return frame.ErrWrongInput
	}
	for {
		select {
		case <-ctx.Done():
			return errors.New("time out to wait Workload delete until finish")
		default:
			workload, err := GetWorkloadByName(f, namespace, name)
			if workload == nil && err != nil {
				return nil
			}
			time.Sleep(ForcedWaitingTime)
		}
	}
}
