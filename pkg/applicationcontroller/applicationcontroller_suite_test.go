// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package applicationcontroller

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kruiseapi "github.com/openkruise/kruise-api"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta2"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager"
)

func TestApplicationcontroller(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Applicationcontroller Suite", Label("ApplicationController", "unittest"))
}

var scheme *runtime.Scheme
var subnetAppControllerConfig SubnetAppControllerConfig

type subnetApplicationController struct {
	*SubnetAppController
	deploymentStore  cache.Store
	replicasSetStore cache.Store
	daemonSetStore   cache.Store
	statefulSetStore cache.Store
	jobStore         cache.Store
	cronJobStore     cache.Store
}

var _ = BeforeSuite(func() {
	scheme = runtime.NewScheme()
	err := clientgoscheme.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = spiderpoolv2beta1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = kruiseapi.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	subnetAppControllerConfig = SubnetAppControllerConfig{
		EnableIPv4:                    true,
		EnableIPv6:                    true,
		AppControllerWorkers:          3,
		MaxWorkqueueLength:            5000,
		WorkQueueMaxRetries:           10,
		WorkQueueRequeueDelayDuration: -1 * time.Second,
		LeaderRetryElectGap:           0,
	}
})

func newController() (*subnetApplicationController, error) {
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	apiReader := fake.NewClientBuilder().WithScheme(scheme).Build()

	reservedIPManager, err := reservedipmanager.NewReservedIPManager(fakeClient, apiReader)
	if nil != err {
		return nil, err
	}
	subnetManager, err := subnetmanager.NewSubnetManager(fakeClient, apiReader, reservedIPManager)
	if nil != err {
		return nil, err
	}

	appController, err := NewSubnetAppController(fakeClient, apiReader, subnetManager, subnetAppControllerConfig)
	if nil != err {
		return nil, err
	}
	return &subnetApplicationController{
		SubnetAppController: appController,
	}, nil
}
