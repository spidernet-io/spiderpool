package subnetmanager_test

import (
	"context"
	"github.com/agiledragon/gomonkey/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

type subnetController struct {
	*subnetmanager.SubnetAppController
}

func newController() (*subnetController, error) {
	subnetAppController, err := subnetmanager.NewSubnetAppController(fakeClient, mockSubnetMgr, subnetmanager.SubnetAppControllerConfig{})
	if nil != err {
		return nil, err
	}

	return &subnetController{
		SubnetAppController: subnetAppController,
	}, nil
}

var _ = Describe("AppController", func() {
	var subnetControl *subnetController
	var clientSet *fake.Clientset
	BeforeEach(func() {
		c, err := newController()
		Expect(err).NotTo(HaveOccurred())
		subnetControl = c

		clientSet = fake.NewSimpleClientset()

		patches := gomonkey.ApplyFuncReturn(cache.WaitForNamedCacheSync, true)
		DeferCleanup(patches.Reset)

	})

	It("test set up informer", func() {
		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()
		mockLeaderElector.EXPECT().IsElected().Return(false).AnyTimes()
		err := subnetControl.SetupInformer(ctx, clientSet, mockLeaderElector)
		Eventually(func(g Gomega) {
			g.Expect(subnetControl.SubnetAppController.DeploymentInformer.GetIndexer()).NotTo(BeNil())
		})

		Expect(err).NotTo(HaveOccurred())
	})

	It("test deployment creating event", func() {
		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()

		mockLeaderElector.EXPECT().IsElected().Return(true).AnyTimes()
		err := subnetControl.SetupInformer(ctx, clientSet, mockLeaderElector)
		Eventually(func(g Gomega) {
			g.Expect(subnetControl.SubnetAppController.DeploymentInformer.GetIndexer()).NotTo(BeNil())
		})

		Expect(err).NotTo(HaveOccurred())

		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment",
				Namespace: "default",
			},
		}
		addOrUpdateHandler := subnetControl.ControllerAddOrUpdateHandler()
		err = addOrUpdateHandler(ctx, nil, deployment)
		Expect(err).NotTo(HaveOccurred())
	})

})
