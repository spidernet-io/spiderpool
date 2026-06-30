// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networkresourceplugin

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/spidernet-io/spiderpool/pkg/constant"
)

type fakeInterfaceDiscoverer struct {
	interfaces     []string
	err            error
	includeVirtual bool
}

func (f *fakeInterfaceDiscoverer) Interfaces(includeVirtual bool) ([]string, error) {
	f.includeVirtual = includeVirtual
	if f.err != nil {
		return nil, f.err
	}
	return append([]string(nil), f.interfaces...), nil
}

var _ = Describe("network resource plugin manager", Label("networkresourceplugin_manager_test"), func() {
	It("initializes default discoverer and logger", func() {
		manager := NewManager(Config{}, false, nil, nil, nil)

		Expect(manager.discoverer).To(BeAssignableToTypeOf(NetInterfaceDiscoverer{}))
		Expect(manager.logger).NotTo(BeNil())
	})

	It("does not start workers when disabled", func() {
		manager := NewManager(Config{Enabled: false}, false, nil, &fakeInterfaceDiscoverer{}, nil)

		manager.Start(context.Background())

		Expect(manager.servers).To(BeNil())
	})

	It("normalizes nil start contexts", func() {
		manager := NewManager(Config{Enabled: false}, false, nil, &fakeInterfaceDiscoverer{}, nil)
		var startCtx context.Context

		manager.Start(startCtx)

		Expect(manager.servers).To(BeNil())
	})

	It("stops running servers and clears state", func() {
		registrar := NewRegistrarWithKubeletRootDir("spidernet.io/sub-eni", GinkgoT().TempDir(), nil)
		manager := NewManager(Config{}, false, nil, nil, nil)
		manager.servers = []*runningServer{{registrar: registrar}}

		manager.Stop()

		Expect(manager.servers).To(BeNil())
	})

	It("uses the provided discoverer and computes desired resources", func() {
		discoverer := &fakeInterfaceDiscoverer{interfaces: []string{"nrpdm0", "eth0"}}
		manager := NewManager(Config{
			Enabled: true,
			ResourceAdvertisement: ResourceAdvertisementConfig{
				MasterNIC: MasterNICAdvertisementConfig{
					Rules: []MasterNICRuleConfig{{
						IncludeInterfaces: []string{"nrpdm*"},
					}},
				},
			},
		}, false, nil, discoverer, nil)

		resources, err := manager.desiredResources(context.Background())

		Expect(err).NotTo(HaveOccurred())
		Expect(discoverer.includeVirtual).To(BeTrue())
		Expect(resources).To(Equal([]DesiredResource{{
			ResourceName: "spidernet.io/nrpdm0-nic",
			Devices:      DefaultMasterNICMaxCount,
			Interface:    "nrpdm0",
		}}))
	})

	It("returns discoverer errors", func() {
		manager := NewManager(Config{Enabled: true}, false, nil, &fakeInterfaceDiscoverer{err: errors.New("discover failed")}, nil)

		_, err := manager.desiredResources(context.Background())

		Expect(err).To(MatchError("discover failed"))
	})

	It("refreshes the node with nodeGetter", func() {
		getterNode := &corev1.Node{}
		manager := NewManagerWithNodeGetter(Config{
			Enabled: true,
			ResourceAdvertisement: ResourceAdvertisementConfig{
				SubENI: SubENIAdvertisementConfig{
					Rules: []SubENIRuleConfig{{
						ResourceName:    constant.DefaultENISlotResourceName,
						DefaultMaxCount: 1,
					}},
				},
			},
		}, true, func(context.Context) (*corev1.Node, error) {
			return getterNode, nil
		}, &fakeInterfaceDiscoverer{}, nil)

		resources, err := manager.desiredResources(context.Background())

		Expect(err).NotTo(HaveOccurred())
		Expect(manager.node).To(BeIdenticalTo(getterNode))
		Expect(resources).To(Equal([]DesiredResource{{
			ResourceName: constant.DefaultENISlotResourceName,
			Devices:      1,
		}}))
	})

	It("uses cached node metadata when nodeGetter fails", func() {
		cachedNode := &corev1.Node{}
		manager := NewManagerWithNodeGetter(Config{
			Enabled: true,
			ResourceAdvertisement: ResourceAdvertisementConfig{
				SubENI: SubENIAdvertisementConfig{
					Rules: []SubENIRuleConfig{{
						ResourceName:    constant.DefaultENISlotResourceName,
						DefaultMaxCount: 1,
					}},
				},
			},
		}, true, func(context.Context) (*corev1.Node, error) {
			return nil, errors.New("node unavailable")
		}, &fakeInterfaceDiscoverer{}, nil)
		manager.node = cachedNode

		resources, err := manager.desiredResources(context.Background())

		Expect(err).NotTo(HaveOccurred())
		Expect(resources).To(Equal([]DesiredResource{{
			ResourceName: constant.DefaultENISlotResourceName,
			Devices:      1,
		}}))
	})

	It("compares running server resource names as a set", func() {
		servers := []*runningServer{
			{desired: DesiredResource{ResourceName: "spidernet.io/eth0-nic"}},
			{desired: DesiredResource{ResourceName: constant.DefaultENISlotResourceName}},
		}

		Expect(sameResourceSet(servers, []DesiredResource{
			{ResourceName: constant.DefaultENISlotResourceName},
			{ResourceName: "spidernet.io/eth0-nic"},
		})).To(BeTrue())
		Expect(sameResourceSet(servers, []DesiredResource{{ResourceName: constant.DefaultENISlotResourceName}})).To(BeFalse())
		Expect(sameResourceSet(servers, []DesiredResource{
			{ResourceName: constant.DefaultENISlotResourceName},
			{ResourceName: "spidernet.io/eth1-nic"},
		})).To(BeFalse())
	})

	It("updates existing servers when resource device counts change", func() {
		server := NewServer(constant.DefaultENISlotResourceName, healthySubENIDevices(1), nil)
		servers := []*runningServer{{
			deviceServer: server,
			desired:      DesiredResource{ResourceName: constant.DefaultENISlotResourceName, Devices: 1},
		}}

		updateServers(servers, []DesiredResource{{
			ResourceName: constant.DefaultENISlotResourceName,
			Devices:      2,
		}})

		Expect(servers[0].desired.Devices).To(Equal(2))
		Expect(server.snapshotDevices()).To(Equal([]*pluginapi.Device{
			{ID: "sub-eni-0", Health: pluginapi.Healthy},
			{ID: "sub-eni-1", Health: pluginapi.Healthy},
		}))
	})

	It("skips server updates when desired devices and interface are unchanged", func() {
		server := NewServer(constant.DefaultENISlotResourceName, healthySubENIDevices(1), nil)
		servers := []*runningServer{{
			deviceServer: server,
			desired:      DesiredResource{ResourceName: constant.DefaultENISlotResourceName, Devices: 1},
		}}

		updateServers(servers, []DesiredResource{{
			ResourceName: constant.DefaultENISlotResourceName,
			Devices:      1,
		}})

		Expect(server.updateCh).NotTo(Receive())
		Expect(servers[0].desired.Devices).To(Equal(1))
	})

	It("updates master NIC server devices when the selected interface changes", func() {
		server := NewServer("spidernet.io/eth0-nic", healthyMasterNICDevices("eth0", 2), nil)
		servers := []*runningServer{{
			deviceServer: server,
			desired:      DesiredResource{ResourceName: "spidernet.io/eth0-nic", Devices: 2, Interface: "eth0"},
		}}

		updateServers(servers, []DesiredResource{{
			ResourceName: "spidernet.io/eth0-nic",
			Devices:      3,
			Interface:    "ens1",
		}})

		devices := server.snapshotDevices()
		Expect(servers[0].desired.Interface).To(Equal("ens1"))
		Expect(servers[0].desired.Devices).To(Equal(3))
		Expect(devices).To(HaveLen(3))
		Expect(devices[0]).To(Equal(&pluginapi.Device{ID: "ens1-nic-0", Health: pluginapi.Healthy}))
	})

	It("indexes desired resources by name", func() {
		index := desiredResourceByName([]DesiredResource{
			{ResourceName: "a", Devices: 1},
			{ResourceName: "b", Devices: 2},
		})

		Expect(index).To(Equal(map[string]DesiredResource{
			"a": {ResourceName: "a", Devices: 1},
			"b": {ResourceName: "b", Devices: 2},
		}))
	})
})
