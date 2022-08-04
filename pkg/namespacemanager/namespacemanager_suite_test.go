package namespacemanager_test

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager/mocks"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNamespacemanager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Namespacemanager Suite")
}

var npManager namespacemanager.NamespaceManager
var ctx context.Context
var cancel context.CancelFunc
var err error
var k8sClient client.Client
var scheme *runtime.Scheme

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.Background())
	scheme = runtime.NewScheme()
	err = corev1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	mgr := &mocks.Manager{}
	k8sClient = createFakeClient()
	mgr.On("GetClient").Return(k8sClient).Maybe()
	mgr.On("SetFields", mock.Anything).Return(nil).Maybe()
	mgr.On("Elected").Return(nil).Maybe()
	npManager, err = namespacemanager.NewNamespaceManager(mgr)
	Expect(err).NotTo(HaveOccurred())
	Expect(npManager).NotTo(BeNil())
})

func createFakeClient() client.Client {
	return fakeclient.NewClientBuilder().WithScheme(scheme).Build()
}

//
//var npManager namespacemanager.NamespaceManager
//var testenv *envtest.Environment
//var mgr manager.Manager
//var cfg *rest.Config
//var err error
//var ctx, cancel = context.WithCancel(context.Background())
//
//var _ = BeforeSuite(func() {
//	testenv = &envtest.Environment{
//		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "charts", "spiderpool", "crds")},
//		ErrorIfCRDPathMissing: false,
//	}
//	cfg, err = testenv.Start()
//	Expect(err).NotTo(HaveOccurred())
//	Expect(cfg).NotTo(BeNil())
//
//	scheme := runtime.NewScheme()
//	err = spiderpoolv1.AddToScheme(scheme)
//	Expect(err).NotTo(HaveOccurred())
//
//	err = admissionv1.AddToScheme(scheme)
//	Expect(err).NotTo(HaveOccurred())
//
//	err = corev1.AddToScheme(scheme)
//	Expect(err).NotTo(HaveOccurred())
//
//	mgr, err = manager.New(cfg, manager.Options{
//		Scheme:                 scheme,
//		MetricsBindAddress:     "0",
//		HealthProbeBindAddress: "0",
//	})
//	npManager, err = namespacemanager.NewNamespaceManager(mgr)
//	Expect(err).NotTo(HaveOccurred())
//	Expect(npManager).NotTo(BeNil())
//
//	go func() {
//		defer GinkgoRecover()
//
//		err = mgr.Start(ctx)
//		Expect(err).NotTo(HaveOccurred())
//	}()
//})
//
//var _ = AfterSuite(func() {
//	cancel()
//	err = testenv.Stop()
//	Expect(err).NotTo(HaveOccurred())
//
//})
