// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/lock"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"

	ip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/types"

	. "github.com/onsi/ginkgo/v2"
	frame "github.com/spidernet-io/e2eframework/framework"
	"github.com/spidernet-io/e2eframework/tools"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var usedSubnetsLock = new(lock.Mutex)

func GenerateExampleV4SubnetObject(f *frame.Framework, ipNum int) (string, *spiderpool.SpiderSubnet) {
	usedSubnetsLock.Lock()
	defer usedSubnetsLock.Unlock()

	if ipNum < 1 || ipNum > 65533 {
		GinkgoWriter.Println("the IP range should be between 1 and 65533")
		Fail("the IP range should be between 1 and 65533")
	}
	subnetName := "v4-ss-" + GenerateString(15, true)
	newSubnetObj := &spiderpool.SpiderSubnet{
		ObjectMeta: metav1.ObjectMeta{
			Name: subnetName,
		},
		Spec: spiderpool.SubnetSpec{
			IPVersion: ptr.To(int64(4)),
		},
	}

	for i := 0; i < 5; i++ {
		randNum1 := GenerateRandomNumber(255)
		randNum2 := GenerateRandomNumber(255)
		if ipNum <= 253 {
			newSubnetObj.Spec.Subnet = fmt.Sprintf("10.%s.%s.0/24", randNum1, randNum2)
		} else {
			newSubnetObj.Spec.Subnet = fmt.Sprintf("10.%s.0.0/16", randNum1)
		}
		oldSubnets, _ := GetAllSubnet(f)
		for _, oldSubnet := range oldSubnets.Items {
			if newSubnetObj.Spec.Subnet == oldSubnet.Spec.Subnet {
				GinkgoWriter.Printf("Subnet %s overlaps with subnet %s, the overlapping subnet is: %v \n", newSubnetObj.Name, oldSubnet.Name, newSubnetObj.Spec.Subnet)
				break
			}
		}
	}
	ips, err := GenerateIPs(newSubnetObj.Spec.Subnet, ipNum+1)
	Expect(err).NotTo(HaveOccurred())
	gateway := ips[0]
	newSubnetObj.Spec.Gateway = &gateway
	newSubnetObj.Spec.IPs = ips[1:]
	return subnetName, newSubnetObj
}

func GenerateExampleV6SubnetObject(f *frame.Framework, ipNum int) (string, *spiderpool.SpiderSubnet) {
	usedSubnetsLock.Lock()
	defer usedSubnetsLock.Unlock()

	if ipNum < 1 || ipNum > 65533 {
		GinkgoWriter.Println("the IP range should be between 1 and 65533")
		Fail("the IP range should be between 1 and 65533")
	}

	subnetName := "v6-ss-" + GenerateString(15, true)
	newSubnetObj := &spiderpool.SpiderSubnet{
		ObjectMeta: metav1.ObjectMeta{
			Name: subnetName,
		},
		Spec: spiderpool.SubnetSpec{
			IPVersion: ptr.To(int64(6)),
		},
	}
	for i := 0; i < 5; i++ {
		randNum := GenerateString(4, true)
		if ipNum <= 253 {
			newSubnetObj.Spec.Subnet = fmt.Sprintf("fd00:%s::/120", randNum)
		} else {
			newSubnetObj.Spec.Subnet = fmt.Sprintf("fd00:%s::/112", randNum)
		}
		oldSubnets, _ := GetAllSubnet(f)
		for _, oldSubnet := range oldSubnets.Items {
			if newSubnetObj.Spec.Subnet == oldSubnet.Spec.Subnet {
				GinkgoWriter.Printf("Subnet %s overlaps with subnet %s, the overlapping subnet is: %v \n", newSubnetObj.Name, oldSubnet.Name, newSubnetObj.Spec.Subnet)
				break
			}
		}
	}
	ips, err := GenerateIPs(newSubnetObj.Spec.Subnet, ipNum+1)
	Expect(err).NotTo(HaveOccurred())
	gateway := ips[0]
	newSubnetObj.Spec.Gateway = &gateway
	newSubnetObj.Spec.IPs = ips[1:]
	return subnetName, newSubnetObj
}

func CreateSubnet(f *frame.Framework, subnet *spiderpool.SpiderSubnet, opts ...client.CreateOption) error {
	if f == nil || subnet == nil {
		return frame.ErrWrongInput
	}
	// Try to wait for finish last deleting
	fake := &spiderpool.SpiderSubnet{
		ObjectMeta: metav1.ObjectMeta{
			Name: subnet.Name,
		},
	}
	key := client.ObjectKeyFromObject(fake)
	existing := &spiderpool.SpiderSubnet{}
	e := f.GetResource(key, existing)
	if e == nil && existing.ObjectMeta.DeletionTimestamp == nil {
		return errors.New("failed to create , a same subnet exists")
	}

	t := func() bool {
		existing := &spiderpool.SpiderSubnet{}
		e := f.GetResource(key, existing)
		b := api_errors.IsNotFound(e)
		if !b {
			GinkgoWriter.Printf("waiting for a same subnet %v to finish deleting \n", subnet.Name)
			return false
		}
		return true
	}
	if !tools.Eventually(t, f.Config.ResourceDeleteTimeout, time.Second) {
		return errors.New("failed to create , a same subnet exists")
	}

	return f.CreateResource(subnet, opts...)
}

func WaitCreateSubnetUntilFinish(ctx context.Context, f *frame.Framework, subnet *spiderpool.SpiderSubnet, opts ...client.CreateOption) error {
	if f == nil || subnet == nil {
		return frame.ErrWrongInput
	}
	err := CreateSubnet(f, subnet, opts...)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return frame.ErrTimeOut
		default:
			newSubnetObj, err := GetSubnetByName(f, subnet.ObjectMeta.Name)
			if err != nil {
				return err
			}
			if newSubnetObj.Name == subnet.Name {
				return nil
			}
			time.Sleep(time.Second)
		}
	}
}

func DeleteSubnetByName(f *frame.Framework, subnetName string, opts ...client.DeleteOption) error {
	if subnetName == "" || f == nil {
		return frame.ErrWrongInput
	}
	subnetObj := &spiderpool.SpiderSubnet{
		ObjectMeta: metav1.ObjectMeta{
			Name: subnetName,
		},
	}
	return f.DeleteResource(subnetObj, opts...)
}

func GetSubnetByName(f *frame.Framework, subnetName string) (*spiderpool.SpiderSubnet, error) {
	if subnetName == "" || f == nil {
		return nil, errors.New("wrong input")
	}
	key := apitypes.NamespacedName{Name: subnetName}
	subnetObj := &spiderpool.SpiderSubnet{}
	e := f.GetResource(key, subnetObj)
	if e != nil {
		return nil, e
	}
	return subnetObj, e
}

func DeleteSubnetUntilFinish(ctx context.Context, f *frame.Framework, subnetName string, opts ...client.DeleteOption) error {
	if f == nil || subnetName == "" {
		return frame.ErrWrongInput
	}
	err := DeleteSubnetByName(f, subnetName, opts...)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return frame.ErrTimeOut
		default:
			_, err := GetSubnetByName(f, subnetName)
			if err != nil {
				GinkgoWriter.Printf("Subnet '%s' has been removed，error: %v", subnetName, err)
				return nil
			}
			time.Sleep(time.Second)
		}
	}
}

func WaitValidateSubnetAllocatedIPCount(ctx context.Context, f *frame.Framework, subnetName string, allocatedIPCount int64) error {
	if f == nil || subnetName == "" {
		return frame.ErrWrongInput
	}

	for {
		select {
		case <-ctx.Done():
			return frame.ErrTimeOut
		default:
			subnetObject, err := GetSubnetByName(f, subnetName)
			if err != nil {
				return err
			}

			// The informer of SpiderSubnet will delay synchronizing its own state information
			// which may cause failure 'runtime error: invalid memory address or nil pointer dereference'
			if subnetObject.Status.AllocatedIPCount == nil {
				continue
			}

			if *subnetObject.Status.AllocatedIPCount == allocatedIPCount {
				return nil
			}
			time.Sleep(ForcedWaitingTime)
		}
	}
}

func PatchSpiderSubnet(f *frame.Framework, desiredSubnet, originalSubnet *spiderpool.SpiderSubnet, opts ...client.PatchOption) error {
	if desiredSubnet == nil || f == nil || originalSubnet == nil {
		return frame.ErrWrongInput
	}

	mergePatch := client.MergeFrom(originalSubnet)
	d, err := mergePatch.Data(desiredSubnet)
	GinkgoWriter.Printf("the patch is: %v. \n", string(d))
	if err != nil {
		return fmt.Errorf("failed to generate patch, err is %v", err)
	}

	return f.PatchResource(desiredSubnet, mergePatch, opts...)
}

func WaitIppoolNumberInSubnet(ctx context.Context, f *frame.Framework, subnetName string, poolNums int) error {
	if f == nil || subnetName == "" || poolNums < 0 {
		return frame.ErrWrongInput
	}

LOOP:
	for {
		select {
		case <-ctx.Done():
			return frame.ErrTimeOut
		default:
			poolList, err := GetIppoolsInSubnet(f, subnetName)
			if err != nil {
				return err
			}
			if len(poolList.Items) != poolNums {
				time.Sleep(ForcedWaitingTime)
				continue LOOP
			}
			return nil
		}
	}
}

func GetAvailableIpsInSubnet(f *frame.Framework, subnetName string) ([]net.IP, error) {
	if f == nil || subnetName == "" {
		return nil, frame.ErrWrongInput
	}

	subnetObj, err := GetSubnetByName(f, subnetName)
	if err != nil {
		return nil, fmt.Errorf("failed to get subnet '%s', error:'%v' ", subnetName, err)
	}

	ips1, err := ip.ParseIPRanges(*subnetObj.Spec.IPVersion, subnetObj.Spec.IPs)
	if err != nil {
		return nil, err
	}

	controlledIPPools, err := convert.UnmarshalSubnetAllocatedIPPools(subnetObj.Status.ControlledIPPools)
	if err != nil {
		return nil, err
	}

	ipArray := []string{}
	for _, preAllocation := range controlledIPPools {
		ipArray = append(ipArray, preAllocation.IPs...)
	}

	newArray, err := ip.MergeIPRanges(*subnetObj.Spec.IPVersion, ipArray)
	if err != nil {
		return nil, err
	}

	ips2, err := ip.ParseIPRanges(*subnetObj.Spec.IPVersion, newArray)
	if err != nil {
		return nil, err
	}

	ips := ip.IPsDiffSet(ips1, ips2, false)
	return ips, nil
}

func WaitValidateSubnetAndPoolIpConsistency(ctx context.Context, f *frame.Framework, subnetName string) error {
	if f == nil || subnetName == "" {
		return frame.ErrWrongInput
	}

LOOP:
	for {
		select {
		case <-ctx.Done():
			return frame.ErrTimeOut
		default:
			subnetObject, err := GetSubnetByName(f, subnetName)
			if err != nil {
				return fmt.Errorf("failed to get subnet '%s', error:'%v' ", subnetName, err)
			}

			poolList, err := GetIppoolsInSubnet(f, subnetName)
			if err != nil || len(poolList.Items) == 0 {
				return fmt.Errorf("failed to get ippool in subnet %v", subnetName)
			}

			var poolInSubentList []string
			ipMap := make(map[string]string)

			controlledIPPools, err := convert.UnmarshalSubnetAllocatedIPPools(subnetObject.Status.ControlledIPPools)
			if err != nil {
				return err
			}

			for poolInSubnet, ipsInSubnet := range controlledIPPools {
				poolInSubentList = append(poolInSubentList, poolInSubnet)
				for _, pool := range poolList.Items {
					if pool.Name == poolInSubnet {
						ips1, err := ip.AssembleTotalIPs(*pool.Spec.IPVersion, pool.Spec.IPs, pool.Spec.ExcludeIPs)
						if err != nil {
							return fmt.Errorf("failed to calculate SpiderIPPool '%s' total IP count, error: %v", pool.Name, err)
						}

						for _, v := range ips1 {
							if d, ok := ipMap[string(v)]; ok {
								return fmt.Errorf("ippool objects %v and %v have conflicting ip: %v", d, pool.Name, v)
							}
							ipMap[string(v)] = pool.Name
						}

						ips2, err := ip.ParseIPRanges(*subnetObject.Spec.IPVersion, ipsInSubnet.IPs)
						if err != nil {
							return err
						}

						if ip.IsDiffIPSet(ips1, ips2) {
							GinkgoWriter.Printf("inconsistent ip records in subnet %v/%v and pool %v/%v \n", subnetName, ips2, pool.Name, ips1)
							continue LOOP
						}
						break
					}
				}
			}
			if len(poolInSubentList) != len(poolList.Items) {
				continue LOOP
			}
			return nil
		}
	}
}

func BatchCreateSubnet(f *frame.Framework, version types.IPVersion, subnetNums, subnetIpNums int) ([]string, error) {
	if f == nil || subnetNums <= 0 || subnetIpNums <= 0 {
		return nil, frame.ErrWrongInput
	}

	var subnetName string
	var subnetObject *spiderpool.SpiderSubnet
	var subnetNameList []string
	var subnetObjectList []*spiderpool.SpiderSubnet
	CirdMap := make(map[string]string)

OUTER_FOR:
	for i := 1; i <= subnetNums; i++ {
		if version == constant.IPv4 {
			subnetName, subnetObject = GenerateExampleV4SubnetObject(f, subnetIpNums)
		} else {
			subnetName, subnetObject = GenerateExampleV6SubnetObject(f, subnetIpNums)
		}

		if d, ok := CirdMap[subnetObject.Spec.Subnet]; ok {
			GinkgoWriter.Printf("subnet objects %v and %v have conflicted subnet: %v \n", d, subnetName, subnetObject.Spec.Subnet)
			i--
			continue OUTER_FOR
		}
		CirdMap[string(subnetObject.Spec.Subnet)] = subnetName
		subnetObjectList = append(subnetObjectList, subnetObject)
	}

	lock := lock.Mutex{}
	wg := sync.WaitGroup{}
	wg.Add(len(subnetObjectList))
	for _, subentObj := range subnetObjectList {
		s := subentObj
		go func() {
			defer GinkgoRecover()
			defer wg.Done()
			Expect(CreateSubnet(f, s)).NotTo(HaveOccurred())

			lock.Lock()
			subnetNameList = append(subnetNameList, s.Name)
			lock.Unlock()
		}()
	}
	wg.Wait()
	Expect(len(subnetNameList)).To(Equal(subnetNums))
	return subnetNameList, nil
}

func GetAllSubnet(f *frame.Framework, opts ...client.ListOption) (*spiderpool.SpiderSubnetList, error) {
	if f == nil {
		return nil, errors.New("wrong input")
	}

	v := &spiderpool.SpiderSubnetList{}
	e := f.ListResource(v, opts...)
	if e != nil {
		return nil, e
	}
	return v, nil
}
