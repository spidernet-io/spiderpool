// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	"context"
	"errors"
	"fmt"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"k8s.io/utils/pointer"
	"time"

	. "github.com/onsi/ginkgo/v2"
	frame "github.com/spidernet-io/e2eframework/framework"
	"github.com/spidernet-io/e2eframework/tools"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GenerateExampleV4SubnetObject() (string, *spiderpool.SpiderSubnet) {
	subnetName := "v4-ss-" + tools.RandomName()
	randNum1 := GenerateRandomNumber(255)
	randNum2 := GenerateRandomNumber(255)
	subnetStr := fmt.Sprintf("%s.%s.0.0/16", randNum1, randNum2)
	gateway := fmt.Sprintf("%s.%s.0.1", randNum1, randNum2)
	ips := fmt.Sprintf("%s.%s.0.100-%s.%s.0.200", randNum1, randNum2, randNum1, randNum2)
	subnetObj := &spiderpool.SpiderSubnet{
		ObjectMeta: metav1.ObjectMeta{
			Name: subnetName,
		},
		Spec: spiderpool.SubnetSpec{
			IPVersion: pointer.Int64(4),
			Subnet:    subnetStr,
			Gateway:   &gateway,
			IPs:       []string{ips},
		},
	}
	return subnetName, subnetObj
}

func GenerateExampleV6SubnetObject() (string, *spiderpool.SpiderSubnet) {
	subnetName := "v6-ss-" + tools.RandomName()
	randNum1 := GenerateString(4, true)
	randNum2 := GenerateString(4, true)
	subnetStr := fmt.Sprintf("%s:%s::/112", randNum1, randNum2)
	gateway := fmt.Sprintf("%s:%s::1", randNum1, randNum2)
	ips := fmt.Sprintf("%s:%s::100-%s:%s::200", randNum1, randNum2, randNum1, randNum2)
	subnetObj := &spiderpool.SpiderSubnet{
		ObjectMeta: metav1.ObjectMeta{
			Name: subnetName,
		},
		Spec: spiderpool.SubnetSpec{
			IPVersion: pointer.Int64(6),
			Subnet:    subnetStr,
			Gateway:   &gateway,
			IPs:       []string{ips},
		},
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
