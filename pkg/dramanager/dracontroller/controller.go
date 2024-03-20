package dracontroller

import (
	"context"

	"github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/dynamic-resource-allocation/controller"
)

func StartController(ctx context.Context, spiderClientset clientset.Interface, kubeClient kubernetes.Interface, informerFactory informers.SharedInformerFactory) error {
	driver := NewDriver()
	controller := controller.New(ctx, "todo", driver, kubeClient, informerFactory)
	informerFactory.Start(ctx.Done())
	controller.Run(1)
	return nil
}
