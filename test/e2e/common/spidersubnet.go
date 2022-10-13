// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	v1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"k8s.io/utils/pointer"

	. "github.com/onsi/ginkgo/v2"
	frame "github.com/spidernet-io/e2eframework/framework"
	"github.com/spidernet-io/e2eframework/tools"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GenerateExampleV4SubnetObject(ipNum int) (string, *spiderpool.SpiderSubnet) {
	if ipNum < 1 || ipNum > 65533 {
		GinkgoWriter.Println("the IP range should be between 1 and 65533")
		Fail("the IP range should be between 1 and 65533")
	}
	subnetName := "v4-ss-" + tools.RandomName()
	randNum1 := GenerateRandomNumber(255)
	randNum2 := GenerateRandomNumber(255)

	subnetObj := &spiderpool.SpiderSubnet{
		ObjectMeta: metav1.ObjectMeta{
			Name: subnetName,
		},
		Spec: spiderpool.SubnetSpec{
			IPVersion: pointer.Int64(4),
		},
	}
	if ipNum <= 253 {
		gateway := fmt.Sprintf("10.%s.%s.1", randNum1, randNum2)
		subnetObj.Spec.Gateway = &gateway
		subnetObj.Spec.Subnet = fmt.Sprintf("10.%s.%s.0/24", randNum1, randNum2)
		if ipNum == 1 {
			subnetObj.Spec.IPs = []string{fmt.Sprintf("10.%s.%s.2", randNum1, randNum2)}
		} else {
			a := strconv.Itoa(ipNum + 1)
			subnetObj.Spec.IPs = []string{fmt.Sprintf("10.%s.%s.2-10.%s.%s.%s", randNum1, randNum2, randNum1, randNum2, a)}
		}
	} else {
		gateway := fmt.Sprintf("10.%s.0.1", randNum1)
		subnetObj.Spec.Gateway = &gateway
		subnetObj.Spec.Subnet = fmt.Sprintf("10.%s.0.0/16", randNum1)
		a := fmt.Sprintf("%.0f", float64((ipNum+1)/256))
		b := strconv.Itoa((ipNum + 1) % 256)
		subnetObj.Spec.IPs = []string{fmt.Sprintf("10.%s.0.2-10.%s.%s.%s", randNum1, randNum2, a, b)}
	}
	return subnetName, subnetObj
}

func GenerateExampleV6SubnetObject(ipNum int) (string, *spiderpool.SpiderSubnet) {
	if ipNum < 1 || ipNum > 65533 {
		GinkgoWriter.Println("the IP range should be between 1 and 65533")
		Fail("the IP range should be between 1 and 65533")
	}

	subnetName := "v6-ss-" + tools.RandomName()
	randNum := GenerateString(4, true)
	subnetObj := &spiderpool.SpiderSubnet{
		ObjectMeta: metav1.ObjectMeta{
			Name: subnetName,
		},
		Spec: spiderpool.SubnetSpec{
			IPVersion: pointer.Int64(6),
		},
	}

	if ipNum <= 253 {
		gateway := fmt.Sprintf("fd00:%s::1", randNum)
		subnetObj.Spec.Gateway = &gateway
		subnetObj.Spec.Subnet = fmt.Sprintf("fd00:%s::/120", randNum)
	} else {
		gateway := fmt.Sprintf("fd00:%s::1", randNum)
		subnetObj.Spec.Gateway = &gateway
		subnetObj.Spec.Subnet = fmt.Sprintf("fd00:%s::/112", randNum)
	}

	if ipNum == 1 {
		subnetObj.Spec.IPs = []string{fmt.Sprintf("fd00:%s::2", randNum)}
	} else {
		bStr := strconv.FormatInt(int64(ipNum+1), 16)
		subnetObj.Spec.IPs = []string{fmt.Sprintf("fd00:%s::2-fd00:%s::%s", randNum, randNum, bStr)}
	}
	return subnetName, subnetObj
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
	} else {
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
	}
	return f.CreateResource(subnet, opts...)
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

func GetSubnetByName(f *frame.Framework, subnetName string) *spiderpool.SpiderSubnet {
	if subnetName == "" || f == nil {
		return nil
	}
	key := apitypes.NamespacedName{Name: subnetName}
	subnetObj := &spiderpool.SpiderSubnet{}
	e := f.GetResource(key, subnetObj)
	if e != nil {
		return nil
	}
	return subnetObj
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
			subnet := GetSubnetByName(f, subnetName)
			if subnet == nil {
				return nil
			}
			time.Sleep(time.Second)
		}
	}
}

func WaitValidateSubnetFreeIPs(ctx context.Context, f *frame.Framework, subnetName string, freeIPsNum int64) error {
	if f == nil || subnetName == "" {
		return frame.ErrWrongInput
	}
	for {
		select {
		case <-ctx.Done():
			return frame.ErrTimeOut
		default:
			subnetObject := GetSubnetByName(f, subnetName)
			if *subnetObject.Status.FreeIPCount == freeIPsNum {
				return nil
			}
			time.Sleep(ForcedWaitingTime)
		}
	}
}

func PatchSpiderSubnet(f *frame.Framework, desiredSubnet, originalSubnet *v1.SpiderSubnet, opts ...client.PatchOption) error {
	if desiredSubnet == nil || f == nil || originalSubnet == nil {
		return frame.ErrWrongInput
	}

	mergePatch := client.MergeFrom(originalSubnet)
	return f.PatchResource(desiredSubnet, mergePatch, opts...)
}

func WaitValidatePoolsCreatedAutomatically(ctx context.Context, f *frame.Framework, subnetName string, poolNums int) error {
	if f == nil || subnetName == "" || poolNums < 0 {
		return frame.ErrWrongInput
	}
LOOP:
	for {
		select {
		case <-ctx.Done():
			return errors.New("waiting for the expected number of ippool timeouts to be created automatically")
		default:
			BingoNum := int(0)
			subnetObject := GetSubnetByName(f, subnetName)
			poolList := GetAllIppool(f)
			for _, v := range poolList.Items {
				if v.Spec.Subnet == subnetObject.Spec.Subnet {
					BingoNum++
				}
			}
			if BingoNum != poolNums {
				continue LOOP
			}
			return nil
		}
	}
}
