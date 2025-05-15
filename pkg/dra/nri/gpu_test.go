package nri

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	resourcev1beta1 "k8s.io/api/resource/v1beta1"
	"k8s.io/utils/ptr"
)

var _ = Describe("filterPfToCniConfigsWithGpuRdmaAffinity", func() {

	Context("empty gpus", func() {
		It("should return nil", func() {
			got := filterPfToCniConfigsWithGpuRdmaAffinity([]string{}, &resourcev1beta1.ResourceSlice{
				Spec: resourcev1beta1.ResourceSliceSpec{
					Devices: []resourcev1beta1.Device{},
				},
			})
			Expect(got).To(BeNil())
		})
	})

	Context("no matching devices", func() {
		It("should return nil", func() {
			got := filterPfToCniConfigsWithGpuRdmaAffinity([]string{"0000:04:00.0"}, &resourcev1beta1.ResourceSlice{
				Spec: resourcev1beta1.ResourceSliceSpec{
					Devices: []resourcev1beta1.Device{
						{
							Name: "ens33",
							Basic: &resourcev1beta1.BasicDevice{
								Attributes: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceAttribute{
									"state":           {StringValue: ptr.To("up")},
									"rdma":            {BoolValue: ptr.To(true)},
									"cniConfigs":      {StringValue: ptr.To("ens33-sriov1")},
									"gdrAffinityGpus": {StringValue: ptr.To("0000:06:00.0")},
								},
							},
						},
					},
				},
			})
			Expect(len(got)).To(Equal(0))
		})

		It("no rdma network nic found", func() {
			got := filterPfToCniConfigsWithGpuRdmaAffinity([]string{"0000:06:00.0"}, &resourcev1beta1.ResourceSlice{
				Spec: resourcev1beta1.ResourceSliceSpec{
					Devices: []resourcev1beta1.Device{
						{
							Name: "ens33",
							Basic: &resourcev1beta1.BasicDevice{
								Attributes: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceAttribute{
									"state":           {StringValue: ptr.To("up")},
									"rdma":            {BoolValue: ptr.To(false)},
									"cniConfigs":      {StringValue: ptr.To("ens33-sriov1")},
									"gdrAffinityGpus": {StringValue: ptr.To("0000:06:00.0")},
								},
							},
						},
					},
				},
			})
			Expect(len(got)).To(Equal(0))
		})
	})

	Context("single gpu matched with single network", func() {
		It("should return the one network", func() {
			got := filterPfToCniConfigsWithGpuRdmaAffinity([]string{"0000:06:00.0"}, &resourcev1beta1.ResourceSlice{
				Spec: resourcev1beta1.ResourceSliceSpec{
					Devices: []resourcev1beta1.Device{
						{
							Name: "ens33",
							Basic: &resourcev1beta1.BasicDevice{
								Attributes: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceAttribute{
									"state":           {StringValue: ptr.To("up")},
									"rdma":            {BoolValue: ptr.To(true)},
									"cniConfigs":      {StringValue: ptr.To("ens33-sriov1")},
									"gdrAffinityGpus": {StringValue: ptr.To("0000:06:00.0")},
								},
							},
						},
						{
							Name: "ens34",
							Basic: &resourcev1beta1.BasicDevice{
								Attributes: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceAttribute{
									"state":           {StringValue: ptr.To("up")},
									"rdma":            {BoolValue: ptr.To(true)},
									"cniConfigs":      {StringValue: ptr.To("ens34-sriov1")},
									"gdrAffinityGpus": {StringValue: ptr.To("0000:08:00.0")},
								},
							},
						},
					},
				},
			})
			Expect(got).To(HaveLen(1))
			Expect(got).To(HaveKeyWithValue("ens33", "ens33-sriov1"))
		})
	})

	Context("single gpu matched multi networks nic", func() {
		It("should return only first network", func() {
			got := filterPfToCniConfigsWithGpuRdmaAffinity([]string{"0000:06:00.0"}, &resourcev1beta1.ResourceSlice{
				Spec: resourcev1beta1.ResourceSliceSpec{
					Devices: []resourcev1beta1.Device{
						{
							Name: "ens33",
							Basic: &resourcev1beta1.BasicDevice{
								Attributes: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceAttribute{
									"state":           {StringValue: ptr.To("up")},
									"rdma":            {BoolValue: ptr.To(true)},
									"cniConfigs":      {StringValue: ptr.To("ens33-sriov1")},
									"gdrAffinityGpus": {StringValue: ptr.To("0000:06:00.0")},
								},
							},
						},
						{
							Name: "ens34",
							Basic: &resourcev1beta1.BasicDevice{
								Attributes: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceAttribute{
									"state":           {StringValue: ptr.To("up")},
									"rdma":            {BoolValue: ptr.To(true)},
									"cniConfigs":      {StringValue: ptr.To("ens34-sriov1")},
									"gdrAffinityGpus": {StringValue: ptr.To("0000:06:00.0")},
								},
							},
						},
					},
				},
			})
			Expect(got).To(HaveLen(1))
			Expect(got).To(HaveKeyWithValue("ens33", "ens33-sriov1"))
		})
	})

	Context("multiple gpus only matched with single networks", func() {
		It("should return the matched network", func() {
			got := filterPfToCniConfigsWithGpuRdmaAffinity([]string{"0000:06:00.0", "0000:08:00.0"}, &resourcev1beta1.ResourceSlice{
				Spec: resourcev1beta1.ResourceSliceSpec{
					Devices: []resourcev1beta1.Device{
						{
							Name: "ens33",
							Basic: &resourcev1beta1.BasicDevice{
								Attributes: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceAttribute{
									"state":           {StringValue: ptr.To("up")},
									"rdma":            {BoolValue: ptr.To(true)},
									"cniConfigs":      {StringValue: ptr.To("ens33-sriov1")},
									"gdrAffinityGpus": {StringValue: ptr.To("0000:06:00.0,0000:08:00.0")},
								},
							},
						},
						{
							Name: "ens34",
							Basic: &resourcev1beta1.BasicDevice{
								Attributes: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceAttribute{
									"state":           {StringValue: ptr.To("up")},
									"rdma":            {BoolValue: ptr.To(true)},
									"cniConfigs":      {StringValue: ptr.To("ens34-sriov1")},
									"gdrAffinityGpus": {StringValue: ptr.To("0000:10:00.0,0000:12:00.0")},
								},
							},
						},
					},
				},
			})
			Expect(got).To(HaveLen(1))
			Expect(got).To(HaveKeyWithValue("ens33", "ens33-sriov1"))
		})
	})

	Context("multiple gpus with shared network", func() {
		It("if all gpus is matched one network, should return the network", func() {
			got := filterPfToCniConfigsWithGpuRdmaAffinity([]string{"0000:06:00.0", "0000:08:00.0"}, &resourcev1beta1.ResourceSlice{
				Spec: resourcev1beta1.ResourceSliceSpec{
					Devices: []resourcev1beta1.Device{
						{
							Name: "ens33",
							Basic: &resourcev1beta1.BasicDevice{
								Attributes: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceAttribute{
									"state":           {StringValue: ptr.To("up")},
									"rdma":            {BoolValue: ptr.To(true)},
									"cniConfigs":      {StringValue: ptr.To("ens33-sriov1")},
									"gdrAffinityGpus": {StringValue: ptr.To("0000:06:00.0")},
								},
							},
						},
						{
							Name: "ens34",
							Basic: &resourcev1beta1.BasicDevice{
								Attributes: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceAttribute{
									"state":           {StringValue: ptr.To("up")},
									"rdma":            {BoolValue: ptr.To(true)},
									"cniConfigs":      {StringValue: ptr.To("ens34-sriov1")},
									"gdrAffinityGpus": {StringValue: ptr.To("0000:06:00.0,0000:08:00.0")},
								},
							},
						},
						{
							Name: "ens35",
							Basic: &resourcev1beta1.BasicDevice{
								Attributes: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceAttribute{
									"state":           {StringValue: ptr.To("up")},
									"rdma":            {BoolValue: ptr.To(true)},
									"cniConfigs":      {StringValue: ptr.To("ens35-sriov1")},
									"gdrAffinityGpus": {StringValue: ptr.To("0000:08:00.0")},
								},
							},
						},
					},
				},
			})
			Expect(got).To(HaveLen(1))
			Expect(got).To(HaveKeyWithValue("ens34", "ens34-sriov1"))
		})

		It("two gpus matched with one networks, one gpu matched with another network", func() {
			got := filterPfToCniConfigsWithGpuRdmaAffinity([]string{"0000:06:00.0", "0000:08:00.0", "0000:10:00.0"}, &resourcev1beta1.ResourceSlice{
				Spec: resourcev1beta1.ResourceSliceSpec{
					Devices: []resourcev1beta1.Device{
						{
							Name: "ens33",
							Basic: &resourcev1beta1.BasicDevice{
								Attributes: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceAttribute{
									"state":           {StringValue: ptr.To("up")},
									"rdma":            {BoolValue: ptr.To(true)},
									"cniConfigs":      {StringValue: ptr.To("ens33-sriov1")},
									"gdrAffinityGpus": {StringValue: ptr.To("0000:06:00.0")},
								},
							},
						},
						{
							Name: "ens34",
							Basic: &resourcev1beta1.BasicDevice{
								Attributes: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceAttribute{
									"state":           {StringValue: ptr.To("up")},
									"rdma":            {BoolValue: ptr.To(true)},
									"cniConfigs":      {StringValue: ptr.To("ens34-sriov1")},
									"gdrAffinityGpus": {StringValue: ptr.To("0000:06:00.0,0000:08:00.0")},
								},
							},
						},
						{
							Name: "ens35",
							Basic: &resourcev1beta1.BasicDevice{
								Attributes: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceAttribute{
									"state":           {StringValue: ptr.To("up")},
									"rdma":            {BoolValue: ptr.To(true)},
									"cniConfigs":      {StringValue: ptr.To("ens35-sriov1")},
									"gdrAffinityGpus": {StringValue: ptr.To("0000:08:00.0")},
								},
							},
						},
						{
							Name: "ens36",
							Basic: &resourcev1beta1.BasicDevice{
								Attributes: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceAttribute{
									"state":           {StringValue: ptr.To("up")},
									"rdma":            {BoolValue: ptr.To(true)},
									"cniConfigs":      {StringValue: ptr.To("ens36-sriov1")},
									"gdrAffinityGpus": {StringValue: ptr.To("0000:10:00.0")},
								},
							},
						},
					},
				},
			})
			Expect(got).To(HaveLen(2))
			Expect(got).To(HaveKeyWithValue("ens34", "ens34-sriov1"))
			Expect(got).To(HaveKeyWithValue("ens36", "ens36-sriov1"))
		})

		It("every gpu matched with one network", func() {
			got := filterPfToCniConfigsWithGpuRdmaAffinity([]string{"0000:06:00.0", "0000:08:00.0"}, &resourcev1beta1.ResourceSlice{
				Spec: resourcev1beta1.ResourceSliceSpec{
					Devices: []resourcev1beta1.Device{
						{
							Name: "ens33",
							Basic: &resourcev1beta1.BasicDevice{
								Attributes: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceAttribute{
									"state":           {StringValue: ptr.To("up")},
									"rdma":            {BoolValue: ptr.To(true)},
									"cniConfigs":      {StringValue: ptr.To("ens33-sriov1")},
									"gdrAffinityGpus": {StringValue: ptr.To("0000:06:00.0")},
								},
							},
						},
						{
							Name: "ens34",
							Basic: &resourcev1beta1.BasicDevice{
								Attributes: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceAttribute{
									"state":           {StringValue: ptr.To("up")},
									"rdma":            {BoolValue: ptr.To(true)},
									"cniConfigs":      {StringValue: ptr.To("ens34-sriov1")},
									"gdrAffinityGpus": {StringValue: ptr.To("0000:08:00.0")},
								},
							},
						},
						{
							Name: "ens35",
							Basic: &resourcev1beta1.BasicDevice{
								Attributes: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceAttribute{
									"state":           {StringValue: ptr.To("up")},
									"rdma":            {BoolValue: ptr.To(true)},
									"cniConfigs":      {StringValue: ptr.To("ens35-sriov1")},
									"gdrAffinityGpus": {StringValue: ptr.To("0000:10:00.0")},
								},
							},
						},
					},
				},
			})
			Expect(got).To(HaveLen(2))
			Expect(got).To(HaveKeyWithValue("ens34", "ens34-sriov1"))
			Expect(got).To(HaveKeyWithValue("ens33", "ens33-sriov1"))
		})
	})

})
