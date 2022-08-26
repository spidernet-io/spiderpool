// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	"context"
	"errors"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"time"

	. "github.com/onsi/ginkgo/v2"
	frame "github.com/spidernet-io/e2eframework/framework"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/types"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateReservedIP(f *frame.Framework, ReservedIP *spiderpool.SpiderReservedIP, opts ...client.CreateOption) error {
	if f == nil || ReservedIP == nil {
		return frame.ErrWrongInput
	}
	// Try to wait for finish last deleting
	fake := &spiderpool.SpiderReservedIP{
		ObjectMeta: metav1.ObjectMeta{
			Name: ReservedIP.ObjectMeta.Name,
		},
	}
	key := client.ObjectKeyFromObject(fake)
	existing := &spiderpool.SpiderReservedIP{}
	e := f.GetResource(key, existing)
	if e == nil && existing.ObjectMeta.DeletionTimestamp == nil {
		return errors.New("failed to create , a same reservedip exists")
	} else {
		t := func() bool {
			existing := &spiderpool.SpiderReservedIP{}
			e := f.GetResource(key, existing)
			b := api_errors.IsNotFound(e)
			if !b {
				GinkgoWriter.Printf("waiting for a same reservedIP %v to finish deleting \n", ReservedIP.ObjectMeta.Name)
				return false
			}
			return true
		}
		if !tools.Eventually(t, f.Config.ResourceDeleteTimeout, time.Second) {
			return errors.New("failed to create , a same reservedip exists")
		}
	}
	return f.CreateResource(ReservedIP, opts...)
}

func DeleteReservedIPByName(f *frame.Framework, reservedIPName string, opts ...client.DeleteOption) error {
	if reservedIPName == "" || f == nil {
		return frame.ErrWrongInput
	}
	reservedIP := &spiderpool.SpiderReservedIP{
		ObjectMeta: metav1.ObjectMeta{
			Name: reservedIPName,
		},
	}
	return f.DeleteResource(reservedIP, opts...)
}

func GetReservedIPByName(f *frame.Framework, reservedIPName string) *spiderpool.SpiderReservedIP {
	if reservedIPName == "" || f == nil {
		return nil
	}

	v := apitypes.NamespacedName{Name: reservedIPName}
	existing := &spiderpool.SpiderReservedIP{}
	e := f.GetResource(v, existing)
	if e != nil {
		return nil
	}
	return existing
}

func DeleteResverdIPUntilFinish(ctx context.Context, f *frame.Framework, reservedIPName string, opts ...client.DeleteOption) error {
	if f == nil || reservedIPName == "" {
		return frame.ErrWrongInput
	}
	err := DeleteReservedIPByName(f, reservedIPName, opts...)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return frame.ErrTimeOut
		default:
			pool := GetReservedIPByName(f, reservedIPName)
			if pool == nil {
				return nil
			}
			time.Sleep(time.Second)
		}
	}
}

func GenerateExampleV4ReservedIpObject(ips []string) (string, *spiderpool.SpiderReservedIP) {
	var v4Ipversion = new(types.IPVersion)
	var iPv4ReservedIpObj *spiderpool.SpiderReservedIP
	var v4ReservedIpName string

	*v4Ipversion = constant.IPv4
	v4ReservedIpName = "v4-sr-" + tools.RandomName()

	iPv4ReservedIpObj = &spiderpool.SpiderReservedIP{
		ObjectMeta: metav1.ObjectMeta{
			Name: v4ReservedIpName,
		},
		Spec: spiderpool.ReservedIPSpec{
			IPVersion: v4Ipversion,
			IPs:       ips,
		},
	}
	return v4ReservedIpName, iPv4ReservedIpObj
}

func GenerateExampleV6ReservedIpObject(ips []string) (string, *spiderpool.SpiderReservedIP) {
	var v6Ipversion = new(types.IPVersion)
	var iPv6ReservedIpObj *spiderpool.SpiderReservedIP
	var v6ReservedIpName string

	*v6Ipversion = constant.IPv6
	v6ReservedIpName = "v6-sr-" + tools.RandomName()

	iPv6ReservedIpObj = &spiderpool.SpiderReservedIP{
		ObjectMeta: metav1.ObjectMeta{
			Name: v6ReservedIpName,
		},
		Spec: spiderpool.ReservedIPSpec{
			IPVersion: v6Ipversion,
			IPs:       ips,
		},
	}
	return v6ReservedIpName, iPv6ReservedIpObj
}
