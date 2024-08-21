// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	v1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/lock"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
	"k8s.io/client-go/tools/cache"

	"github.com/asaskevich/govalidator"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	frame "github.com/spidernet-io/e2eframework/framework"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	ip "github.com/spidernet-io/spiderpool/pkg/ip"
	"github.com/spidernet-io/spiderpool/pkg/types"

	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	}

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

func GetIppoolByName(f *frame.Framework, poolName string) (*v1.SpiderIPPool, error) {
	if poolName == "" || f == nil {
		return nil, errors.New("wrong input")
	}

	v := apitypes.NamespacedName{Name: poolName}
	existing := &v1.SpiderIPPool{}
	e := f.GetResource(v, existing)
	if e != nil {
		return nil, e
	}
	return existing, nil
}

func GetAllIppool(f *frame.Framework, opts ...client.ListOption) (*v1.SpiderIPPoolList, error) {
	if f == nil {
		return nil, errors.New("wrong input")
	}

	v := &v1.SpiderIPPoolList{}
	e := f.ListResource(v, opts...)
	if e != nil {
		return nil, e
	}
	return v, nil
}

func CheckIppoolForUsedIP(f *frame.Framework, ippool *v1.SpiderIPPool, podName, podNamespace string, ipAddrress *corev1.PodIP) (bool, error) {
	if f == nil || ippool == nil || podName == "" || podNamespace == "" || ipAddrress.String() == "" {
		return false, errors.New("wrong input")
	}

	allocatedRecords, err := convert.UnmarshalIPPoolAllocatedIPs(ippool.Status.AllocatedIPs)
	if err != nil {
		return false, err
	}
	namespacedName := fmt.Sprintf("%s/%s", podNamespace, podName)
	t, ok := allocatedRecords[ipAddrress.IP]
	if !ok {
		return false, nil
	}

	if t.NamespacedName != namespacedName {
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
			v4ippool, err := GetIppoolByName(f, v)
			if err != nil {
				return false, false, false, err
			}
			v4ippoolList = append(v4ippoolList, v4ippool)
		}
		f.Log("succeeded to get all v4 pool %v \n", v4IppoolNameList)
	}

	if len(v6IppoolNameList) != 0 {
		for _, v := range v6IppoolNameList {
			v6ippool, err := GetIppoolByName(f, v)
			if err != nil {
				return false, false, false, err
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
		return nil, e
	}
	return existing, nil
}

func CheckIppoolForPodName(f *frame.Framework, ippool *v1.SpiderIPPool, podName, podNamespace string) (bool, error) {
	if f == nil || ippool == nil || podName == "" || podNamespace == "" {
		return false, errors.New("wrong input")
	}

	allocatedRecords, err := convert.UnmarshalIPPoolAllocatedIPs(ippool.Status.AllocatedIPs)
	if err != nil {
		return false, err
	}

	namespacedName := fmt.Sprintf("%s/%s", podNamespace, podName)
	for _, v := range allocatedRecords {
		if v.NamespacedName == namespacedName {
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
	var v4PoolName string = "v4pool-" + GenerateString(15, true)
	var randomNumber1 string = GenerateRandomNumber(255)
	var randomNumber2 string = GenerateRandomNumber(255)

	iPv4PoolObj := &v1.SpiderIPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: v4PoolName,
		},
		Spec: v1.IPPoolSpec{
			IPVersion: v4Ipversion,
		},
	}
	if ipNum <= 253 {
		iPv4PoolObj.Spec.Subnet = fmt.Sprintf("10.%s.%s.0/24", randomNumber1, randomNumber2)
	} else {
		iPv4PoolObj.Spec.Subnet = fmt.Sprintf("10.%s.0.0/16", randomNumber1)
	}
	ips, err := GenerateIPs(iPv4PoolObj.Spec.Subnet, ipNum+1)
	Expect(err).NotTo(HaveOccurred())
	iPv4PoolObj.Spec.IPs = ips[1:]
	return v4PoolName, iPv4PoolObj
}

func GenerateExampleIpv6poolObject(ipNum int) (string, *v1.SpiderIPPool) {
	if ipNum < 1 || ipNum > 65533 {
		GinkgoWriter.Println("the IP range should be between 1 and 65533")
		Fail("the IP range should be between 1 and 65533")
	}

	var v6Ipversion = new(types.IPVersion)
	*v6Ipversion = constant.IPv6
	var v6PoolName string = "v6pool-" + GenerateString(15, true)
	var randomNumber string = GenerateString(4, true)

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

	ips, err := GenerateIPs(iPv6PoolObj.Spec.Subnet, ipNum+1)
	Expect(err).NotTo(HaveOccurred())
	iPv6PoolObj.Spec.IPs = ips[1:]
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
			_, err := GetIppoolByName(f, poolName)
			if err != nil {
				GinkgoWriter.Printf("IPPool '%s' has been removed，error: %v", poolName, err)
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

func GenerateRandomIPV4() string {
	a, b, c, d := r.Intn(255), r.Intn(255), r.Intn(255), r.Intn(255)
	return fmt.Sprintf("%d:%d:%d:%d", a, b, c, d)
}

func GenerateRandomIPV6() string {
	n := make([]byte, 3)
	r.Read(n)
	return fmt.Sprintf("%x:%x::%x", n[0], n[1], n[2])
}

// Waiting for Ippool Status Condition By Allocated IPs meets expectations
// can be used to detect dirty IPs recorded in ippool to be reclaimed automatically
func WaitIppoolStatusConditionByAllocatedIPs(ctx context.Context, f *frame.Framework, poolName, checkIPs string, isRecord bool) error {
	for {
		select {
		case <-ctx.Done():
			return frame.ErrTimeOut
		default:
			poolObj, err := GetIppoolByName(f, poolName)
			if err != nil {
				return err
			}

			allocatedRecords, err := convert.UnmarshalIPPoolAllocatedIPs(poolObj.Status.AllocatedIPs)
			if err != nil {
				return err
			}

			_, ok := allocatedRecords[checkIPs]
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
			return fmt.Errorf("time out to wait workload %v/%v delete until finish", namespace, name)
		default:
			_, err := GetWorkloadByName(f, namespace, name)
			if err != nil {
				if api_errors.IsNotFound(err) {
					GinkgoWriter.Printf("workload '%s/%s' has been removed，error: %v", namespace, name, err)
					return nil
				}
				return err
			}
			time.Sleep(ForcedWaitingTime)
		}
	}
}

func CheckUniqueUuidInSpiderPool(f *frame.Framework, poolName string) error {
	if f == nil || poolName == "" {
		return frame.ErrWrongInput
	}
	ippool, err := GetIppoolByName(f, poolName)
	if err != nil {
		return err
	}
	ipAndUuidMap := map[string]string{}
	allocatedRecords, err := convert.UnmarshalIPPoolAllocatedIPs(ippool.Status.AllocatedIPs)
	if err != nil {
		return err
	}

	for _, v := range allocatedRecords {
		if d, ok := ipAndUuidMap[v.PodUID]; ok {
			return fmt.Errorf("pod %v uuid %v is not unique", d, v.PodUID)
		}
		ipAndUuidMap[v.PodUID] = v.PodUID
	}
	return nil
}

func GetIppoolsInSubnet(f *frame.Framework, subnetName string) (*v1.SpiderIPPoolList, error) {
	if f == nil || subnetName == "" {
		return nil, frame.ErrWrongInput
	}

	opt := []client.ListOption{
		client.MatchingLabelsSelector{
			Selector: labels.SelectorFromSet(map[string]string{
				constant.LabelIPPoolOwnerSpiderSubnet: subnetName,
			}),
		},
	}

	poolList, err := GetAllIppool(f, opt...)
	if poolList == nil || err != nil {
		return nil, err
	}

	return poolList, nil
}

func GetPoolNameListInSubnet(f *frame.Framework, subnetName string) ([]string, error) {
	if f == nil || subnetName == "" {
		return nil, frame.ErrWrongInput
	}
	var poolNameList []string

	poolList, err := GetIppoolsInSubnet(f, subnetName)
	if nil != err {
		return nil, err
	}
	for _, v := range poolList.Items {
		poolNameList = append(poolNameList, v.Name)
	}
	return poolNameList, nil
}

func CreateIppoolInSpiderSubnet(ctx context.Context, f *frame.Framework, subnetName string, pool *v1.SpiderIPPool, ipNum int) error {
	if f == nil || subnetName == "" || pool == nil || ipNum <= 0 {
		return frame.ErrWrongInput
	}

	subnetObj, err := GetSubnetByName(f, subnetName)
	if err != nil {
		return fmt.Errorf("failed to get subnet '%s', error: '%v' ", subnetName, err)
	}
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("exhausted retries, failed to create ippool '%s' in subnet '%s'. ", pool.Name, subnetName)
		default:
			ips, err := GetAvailableIpsInSubnet(f, subnetName)
			if err != nil {
				return err
			}

			if len(ips) < ipNum {
				f.Log("insufficient subnet ip, wait for a second and get a retry.")
				time.Sleep(ForcedWaitingTime)
				continue
			}

			selectIpRanges, err := SelectIpFromIps(*subnetObj.Spec.IPVersion, ips, ipNum)
			if selectIpRanges == nil || err != nil {
				return err
			}

			pool.Spec.Subnet = subnetObj.Spec.Subnet
			pool.Spec.IPs = selectIpRanges
			pool.Spec.Gateway = subnetObj.Spec.Gateway
			err = CreateIppool(f, pool)
			if err != nil {
				// The informer of SpiderSubnet will delay synchronizing its own state information,
				// and build SpiderIPPool concurrently to add a retry mechanism to handle dirty reads.
				f.Log("failed to create ippool '%s' in subnet '%s', wait for a second and get a retry, error: %v", pool.Name, subnetName, err)
				time.Sleep(ForcedWaitingTime)
				continue
			}
			return nil
		}
	}
}

// BatchCreateIPPoolsInSpiderSubnet will create a set of identical versions of ippools for you under the desired subnet.
// subnet and subnetIPRanges must belong to the same IP version.
func BatchCreateIPPoolsInSpiderSubnet(f *frame.Framework, version types.IPVersion, subnet string, subnetIPRanges []string, poolNum, ipNum int) (poolNameList []string, err error) {
	if f == nil || subnet == "" || poolNum <= 0 || ipNum <= 0 {
		return nil, frame.ErrWrongInput
	}

	ipList, err := ip.ParseIPRanges(version, subnetIPRanges)
	if err != nil {
		return nil, errors.New("failed to parse ip")
	}
	if len(ipList) < (poolNum * ipNum) {
		return nil, errors.New("insufficient ip in subnet")
	}

	var poolNames []string
	lock := lock.Mutex{}
	wg := sync.WaitGroup{}
	wg.Add(poolNum)
	for i := 1; i <= poolNum; i++ {
		j := i
		var poolObj *v1.SpiderIPPool
		var poolName string

		go func() {
			defer GinkgoRecover()
			defer wg.Done()
			if version == constant.IPv4 {
				poolName, poolObj = GenerateExampleIpv4poolObject(ipNum)
			} else {
				poolName, poolObj = GenerateExampleIpv6poolObject(ipNum)
			}
			Expect(poolObj.Spec.IPs).NotTo(BeNil())
			ips, _ := ip.ConvertIPsToIPRanges(version, ipList[ipNum*(j-1):ipNum*j])
			poolObj.Spec.Subnet = subnet
			poolObj.Spec.IPs = ips
			Expect(CreateIppool(f, poolObj)).NotTo(HaveOccurred())

			lock.Lock()
			poolNames = append(poolNames, poolName)
			lock.Unlock()
		}()
	}
	wg.Wait()
	if len(poolNames) != poolNum {
		return nil, errors.New("failed to generate the specified number of pools")
	}
	return poolNames, nil
}

/*
GetPodIPAddressFromIppool is to get the IP from the ippool by name and namespace.
when the application has multiple IPs, but only one is displayed in the application's status,
it is necessary to get the one from the ippool to compare with the actual one.
*/
func GetPodIPAddressFromIppool(f *frame.Framework, poolName, namespace, name string) (string, error) {
	poolObj, err := GetIppoolByName(f, poolName)
	if err != nil {
		return "", err
	}
	allocatedRecords, err := convert.UnmarshalIPPoolAllocatedIPs(poolObj.Status.AllocatedIPs)
	if err != nil {
		return "", err
	}
	namespacedName := fmt.Sprintf("%s/%s", namespace, name)
	for ip, v := range allocatedRecords {
		if v.NamespacedName == namespacedName {
			return ip, nil
		}
	}

	return "", fmt.Errorf(" '%s/%s' does not exist in the pool '%s'", namespace, name, poolName)
}

func WaitWebhookReady(ctx context.Context, f *frame.Framework, webhookPort string) error {
	const webhookMutateRoute = "/webhook-health-check"

	nodeList, err := f.GetNodeList()
	if err != nil {
		return fmt.Errorf("failed to get node information")
	}

	serviceObj, err := f.GetService(constant.SpiderpoolController, SpiderPoolConfigmapNameSpace)
	if err != nil {
		return fmt.Errorf("failed to obtain service information, unable to obtain cluster IP")
	}

	var webhookHealthyCheck string
	if f.Info.IpV6Enabled && !f.Info.IpV4Enabled {
		webhookHealthyCheck = fmt.Sprintf("curl -I -m 1 -g https://[%s]:%s%s --insecure", serviceObj.Spec.ClusterIP, webhookPort, webhookMutateRoute)
	} else {
		webhookHealthyCheck = fmt.Sprintf("curl -I -m 1 https://%s:%s%s --insecure", serviceObj.Spec.ClusterIP, webhookPort, webhookMutateRoute)
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for webhookhealthy to be ready")
		default:
			out, err := f.DockerExecCommand(ctx, nodeList.Items[0].Name, webhookHealthyCheck)
			if err != nil {
				time.Sleep(ForcedWaitingTime)
				f.Log("failed to check webhook healthy, error: %v, output log is: %v ", err, string(out))
				continue
			}
			return nil
		}
	}
}

func GetSpiderControllerEnvValue(f *frame.Framework, envName string) (string, error) {
	deployment, err := f.GetDeployment(constant.SpiderpoolController, MultusNs)
	if err != nil {
		return "", fmt.Errorf("failed to get deployment: %v", err)
	}

	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		return "", fmt.Errorf("expected 1 container in the deployment, found %d", len(deployment.Spec.Template.Spec.Containers))
	}

	for _, env := range deployment.Spec.Template.Spec.Containers[0].Env {
		if env.Name == envName {
			return env.Value, nil
		}
	}

	return "", fmt.Errorf("environment variable %s not found in Spiderpool Controller", envName)
}

type NetworkStatus struct {
	Name      string   `json:"name"`
	Interface string   `json:"interface"`
	IPs       []string `json:"ips"`
	MAC       string   `json:"mac"`
	Default   bool     `json:"default"`
}

// ParsePodNetworkAnnotation parses the 'PodMultusNetworksStatus' annotation from the given Pod
// and extracts all IP addresses associated with the Pod's network interfaces.
func ParsePodNetworkAnnotation(f *frame.Framework, pod *corev1.Pod) ([]string, error) {
	var podIPs []string

	if pod.Annotations[PodMultusNetworksStatus] != "" {
		var networksStatus []NetworkStatus

		// Unmarshal the JSON from the annotation into a slice of NetworkStatus
		err := json.Unmarshal([]byte(pod.Annotations[PodMultusNetworksStatus]), &networksStatus)
		if err != nil {
			return nil, fmt.Errorf("failed to parse IP(s) from the annotation for Multus CNI: %v", err)
		}

		for _, net := range networksStatus {
			podIPs = append(podIPs, net.IPs...)
		}
	} else {
		return nil, fmt.Errorf("network status annotation %v does not exist in pod %s/%s", PodMultusNetworksStatus, pod.Namespace, pod.Name)
	}

	return podIPs, nil
}

// CheckIppoolSanity checks the integrity and correctness of the IP pool's allocation status.
// It ensures that each IP in the pool is:
// 1. Allocated to a single Pod, whose UID matches the record in the IP pool.
// 2. Correctly tracked by its associated endpoint, confirming that the Pod's UID matches the endpoint's UID.
// 3. The actual number of IPs in use is compared against the IP pool's reported usage count to ensure consistency.
func CheckIppoolSanity(f *frame.Framework, poolName string) error {
	// Retrieve the IPPool by name
	ippool, err := GetIppoolByName(f, poolName)
	if err != nil {
		if api_errors.IsNotFound(err) {
			return fmt.Errorf("ippool %s does not exist", poolName)
		}
		return fmt.Errorf("failed to get ippool %s, error %v", poolName, err)
	}

	// Parse the allocated IPs from the IP pool status
	poolAllocatedIPs, err := convert.UnmarshalIPPoolAllocatedIPs(ippool.Status.AllocatedIPs)
	if err != nil {
		return fmt.Errorf("failed to parse IPPool '%v' status AllocatedIPs, error: %v", ippool, err)
	}

	// Track the actual number of IPs in use
	actualIPUsageCount := 0
	isSanity := true
	for poolIP, poolIPAllocation := range poolAllocatedIPs {
		// The total number of assigned IP addresses
		actualIPUsageCount++

		// Split the pod NamespacedName to get the namespace and pod name
		podNS, podName, err := cache.SplitMetaNamespaceKey(poolIPAllocation.NamespacedName)
		if err != nil {
			return fmt.Errorf("failed to split pod NamespacedName %s, error: %v", poolIPAllocation.NamespacedName, err)
		}

		// Retrieve the Pod object by its name and namespace
		podYaml, err := f.GetPod(podName, podNS)
		if err != nil {
			if api_errors.IsNotFound(err) {
				GinkgoLogr.Error(fmt.Errorf("pod %s/%s does not exist", podNS, podName), "Failed")
			} else {
				return fmt.Errorf("failed to get pod %s/%s, error: %v", podNS, podName, err)
			}
		}

		podNetworkIPs, err := ParsePodNetworkAnnotation(f, podYaml)
		if nil != err {
			return fmt.Errorf("failed to parse pod %s/%s network annotation \n pod yaml %v, \n error: %v ", podNS, podName, podYaml, err)
		}

		ipINPodCounts := 0
		isIPExistedPod := false
		for _, podNetworkIP := range podNetworkIPs {
			if poolIP == podNetworkIP {
				isIPExistedPod = true
				ipINPodCounts++
			}
		}

		if !isIPExistedPod {
			isSanity = false
			GinkgoLogr.Error(fmt.Errorf("the IP %s in ippool %s does not exist in the pod %s/%s NetworkAnnotation %v", poolIP, poolName, podNS, podName, podNetworkIPs), "Failed")
		} else if ipINPodCounts > 1 {
			isSanity = false
			GinkgoLogr.Error(fmt.Errorf("the IP %s from the IPPool %s appears multiple times in the NetworkAnnotation %+v of the Pod %s/%s", poolIP, poolName, podNetworkIPs, podNS, podName), "Failed")
		} else {
			GinkgoWriter.Printf("The IP %s in the IPPool %s exist in the Pods %s/%s NetworkAnnotation %v and is unique. \n", poolIP, poolName, podNS, podName, podNetworkIPs)
		}

		if string(podYaml.UID) == poolIPAllocation.PodUID {
			GinkgoWriter.Printf("Succeed: Pod %s/%s UID %s matches IPPool %s UID %s \n", podNS, podName, string(podYaml.UID), poolName, poolIPAllocation.PodUID)
		} else {
			isSanity = false
			GinkgoLogr.Error(fmt.Errorf("pod %s/%s UID %s does not match the IPPool %s UID %s", podNS, podName, string(podYaml.UID), poolName, poolIPAllocation.PodUID), "Failed")
		}

		wep, err := GetWorkloadByName(f, podYaml.Namespace, podYaml.Name)
		if err != nil {
			if api_errors.IsNotFound(err) {
				GinkgoLogr.Error(fmt.Errorf("endpoint %s/%s dose not exist", podYaml.Namespace, podYaml.Name), "Failed")
			}
			return fmt.Errorf("failed to get endpoint %s/%s, error %v", podYaml.Namespace, podYaml.Name, err)
		}

		podUsedIPs := convert.GroupIPAllocationDetails(wep.Status.Current.UID, wep.Status.Current.IPs)
		for tmpPoolName, tmpIPs := range podUsedIPs {
			if tmpPoolName == poolName {
				for idx := range tmpIPs {
					if tmpIPs[idx].UID != poolIPAllocation.PodUID {
						isSanity = false
						GinkgoLogr.Error(fmt.Errorf("the UID %s recorded in IPPool %s for the Pod does not match the UID %s in the endpoint %s/%s", poolIPAllocation.PodUID, poolName, tmpIPs[idx].UID, podNS, podName), "Failed")
					} else if tmpIPs[idx].IP != poolIP {
						isSanity = false
						GinkgoLogr.Error(fmt.Errorf("the IP %s recorded in IPPool %s for the Pod %s/%s does not match the IP %s in the endpoint %s/%s", poolIP, poolName, podNS, podName, tmpIPs[idx].IP, podNS, podName), "Failed")
					} else {
						GinkgoWriter.Printf("Succeed: The IP %s and UID %s recorded for the Pod %s/%s in IPPool %s are the same as the UID %s and IP %s in the endpoint %s/%s \n",
							poolIP, poolIPAllocation.PodUID, podNS, podName, poolName, tmpIPs[idx].UID, tmpIPs[idx].IP, podNS, podName)
					}
				}
			}
		}
	}

	if *ippool.Status.AllocatedIPCount > *ippool.Status.TotalIPCount {
		GinkgoWriter.Printf(
			"allocated IP count (%v) exceeds total IP count (%v) \n",
			*ippool.Status.AllocatedIPCount, *ippool.Status.TotalIPCount,
		)
		isSanity = false
	}

	// Ensure that the IP pool's reported usage matches the actual usage
	if actualIPUsageCount != int(*ippool.Status.AllocatedIPCount) {
		GinkgoWriter.Printf("IPPool %s usage count mismatch: expected %d, got %d \n", poolName, actualIPUsageCount, *ippool.Status.AllocatedIPCount)
		isSanity = false
	}

	if !isSanity {
		return fmt.Errorf("IPPool %s sanity check failed", poolName)
	}

	GinkgoWriter.Printf("Successfully checked IPPool %s sanity, IPPool record information is correct \n", poolName)
	return nil
}
