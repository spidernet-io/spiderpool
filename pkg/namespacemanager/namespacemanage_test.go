package namespacemanager_test

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Namespacemanager", Label("unitest", "Namespacemanager_test"), func() {
	var nsName string
	var nsDefaultV4Pool types.AnnoNSDefautlV4PoolValue
	var nsDefaultV6Pool types.AnnoNSDefautlV6PoolValue
	// init namespace name and create
	nsName = "ns" + tools.RandomName()
	GinkgoWriter.Printf("create namespace %v \n", nsName)

	Context("when we get default pools", func() {
		It("not exit nsPool", func() {
			ctx := context.TODO()
			nsDefaultV4Pool, nsDefaultV6Pool, err = npManager.GetNSDefaultPools(ctx, nsName)
			Expect(err).To(HaveOccurred())
			Expect(nsDefaultV4Pool).To(BeNil())
			Expect(nsDefaultV6Pool).To(BeNil())
		})
		It(" exit nsPool", func() {
			ctx := context.TODO()
			nsDefaultV4Pool, nsDefaultV6Pool, err = npManager.GetNSDefaultPools(ctx, nsName)
			Expect(err).To(HaveOccurred()) ////有问题
			Expect(nsDefaultV4Pool).To(BeNil())
			Expect(nsDefaultV6Pool).To(BeNil())
		})
	})
	Context("match label", func() {
		var labelSelector *metav1.LabelSelector
		It("return false", func() {
			b, err := npManager.MatchLabelSelector(ctx, nsName, labelSelector)
			Expect(err).NotTo(HaveOccurred())
			Expect(b).To(BeFalse())
		})
	})
})
